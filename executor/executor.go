package executor

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/action"
	"github.com/hamster-shared/aline-engine/consts"
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
)

type IExecutor interface {
	// Execute 执行任务
	Execute(id int, job *model.Job) error
	//SendResultToQueue 发送结果到队列
	// SendResultToQueue(job *model.JobDetail)
	Cancel(id int, job *model.Job) error
}

type Executor struct {
	cancelMap    map[string]func() // key: jobName/jobID, value: cancelFunc
	StatusChan   chan model.StatusChangeMessage
	stepTimerMap sync.Map // key: jobName/jobID, value: stepTimer
}

// Execute 执行任务
func (e *Executor) Execute(id int, job *model.Job) error {

	// 1. 解析对 pipeline 进行任务排序
	stages, err := job.StageSort()
	jobWrapper := &model.JobDetail{
		Id:     id,
		Job:    *job,
		Status: model.STATUS_NOTRUN,
		Stages: stages,
		ActionResult: model.ActionResult{
			Artifactorys: make([]model.Artifactory, 0),
			Reports:      make([]model.Report, 0),
		},
	}

	// 分支太多，不确定会从哪个分支 return，所以使用 defer，保证一定会将最终结果发送到 StatusChan
	defer func() {
		// 将执行结果发送到 StatusChan，worker 会监听该 chan，将结果发送到 grpc server
		e.StatusChan <- model.NewStatusChangeMsg(jobWrapper.Name, jobWrapper.Id, jobWrapper.Status)
		logger.Infof("send status change message to chan, job name: %s, job id: %d, status: %d", jobWrapper.Name, jobWrapper.Id, jobWrapper.Status)
		// step 定时器也需要删除，避免出现意料之外的报错
		e.stepTimerMap.Delete(utils.FormatJobToString(jobWrapper.Name, jobWrapper.Id))
	}()

	if err != nil {
		return err
	}
	go e.handleTimerListener()

	// 2. 初始化 执行器的上下文

	env := make([]string, 0)
	env = append(env, "PIPELINE_NAME="+job.Name)
	env = append(env, "PIPELINE_ID="+strconv.Itoa(id))

	homeDir, _ := os.UserHomeDir()

	engineContext := make(map[string]any)
	engineContext["hamsterRoot"] = path.Join(homeDir, "workdir")
	workdir := path.Join(engineContext["hamsterRoot"].(string), job.Name)
	engineContext["workdir"] = workdir

	err = os.MkdirAll(workdir, os.ModePerm)

	engineContext["name"] = job.Name
	engineContext["id"] = fmt.Sprintf("%d", id)
	engineContext["env"] = env

	if job.Parameter == nil {
		job.Parameter = make(map[string]string)
	}

	engineContext["parameter"] = job.Parameter

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "stack", engineContext))

	// 将取消 hook 记录到内存中，用于中断程序
	e.cancelMap[strings.Join([]string{job.Name, strconv.Itoa(id)}, "/")] = cancel

	// 队列堆栈
	var stack utils.Stack[action.ActionHandler]

	jobWrapper.Status = model.STATUS_RUNNING
	jobWrapper.StartTime = time.Now()

	executeAction := func(ah action.ActionHandler, job *model.JobDetail) (err error) {
		// 延迟处理的函数
		defer func() {
			// 发生宕机时，获取 panic 传递的上下文并打印
			rErr := recover()
			switch rErr.(type) {
			case runtime.Error: // 运行时错误
				fmt.Println("runtime error:", rErr)
				logger.Errorf("runtime error: %s", rErr)
				err = fmt.Errorf("runtime error: %s", rErr)
			default: // 非运行时错误
				// do nothing
			}
		}()
		if jobWrapper.Status != model.STATUS_RUNNING {
			return nil
		}
		if ah == nil {
			logger.Errorf("action handler is nil, job name: %s, job id: %d", job.Name, job.Id)
			return nil
		}
		err = ah.Pre()
		if err != nil {
			job.Status = model.STATUS_FAIL
			logger.Errorf("action pre hook error, job name: %s, job id: %d, error: %s", job.Name, job.Id, err.Error())
			fmt.Println(err)
			return err
		}
		logger.Infof("action pre hook success, job name: %s, job id: %d", job.Name, job.Id)
		stack.Push(ah)
		actionResult, err := ah.Hook()
		if actionResult != nil && len(actionResult.Artifactorys) > 0 {
			jobWrapper.Artifactorys = append(jobWrapper.Artifactorys, actionResult.Artifactorys...)
		}
		if actionResult != nil && len(actionResult.Reports) > 0 {
			jobWrapper.Reports = append(jobWrapper.Reports, actionResult.Reports...)
		}
		if actionResult != nil && actionResult.CodeInfo != "" {
			jobWrapper.CodeInfo = actionResult.CodeInfo
		}
		if actionResult != nil && len(actionResult.Deploys) > 0 {
			jobWrapper.Deploys = append(jobWrapper.Deploys, actionResult.Deploys...)
		}
		if actionResult != nil && len(actionResult.BuildData) > 0 {
			jobWrapper.BuildData = append(jobWrapper.BuildData, actionResult.BuildData...)
		}
		if err != nil {
			job.Status = model.STATUS_FAIL
			return err
		}
		return err
	}

	jobWrapper.Output = output.New(job.Name, jobWrapper.Id)

	var jobDone = make(chan struct{})
	defer close(jobDone)

	// 定时保存运行状态到 job detail，以更新 step 的运行时间
	go func(jobW *model.JobDetail) {
		for {
			select {
			case <-jobDone:
				return
			default:
				for i := range jobW.Stages {
					for j := range jobW.Stages[i].Stage.Steps {
						if jobW.Stages[i].Stage.Steps[j].Status == model.STATUS_RUNNING {
							jobW.Stages[i].Stage.Steps[j].Duration = int64(time.Since(jobW.Stages[i].Stage.Steps[j].StartTime).Milliseconds())
							logger.Tracef("job: %s, step: %s, duration: %d", jobW.Name, jobW.Stages[i].Stage.Steps[j].Name, jobW.Stages[i].Stage.Steps[j].Duration)
						}
					}
				}
				jober.SaveJobDetail(jobW.Name, jobW)
				time.Sleep(time.Second * 2)
			}
		}
	}(jobWrapper)

	for index, stageWapper := range jobWrapper.Stages {
		//TODO ... stage 的输出也需要换成堆栈方式
		logger.Info("stage: {")
		logger.Infof("   // %s", stageWapper.Name)
		stageWapper.Status = model.STATUS_RUNNING
		stageWapper.StartTime = time.Now()
		jobWrapper.Stages[index] = stageWapper
		jobWrapper.Output.NewStage(stageWapper.Name)
		jober.SaveJobDetail(jobWrapper.Name, jobWrapper)

		for index, step := range stageWapper.Stage.Steps {
			var ah action.ActionHandler
			if step.RunsOn != "" {
				ah = action.NewDockerEnv(step, ctx, jobWrapper.Output)
				err = executeAction(ah, jobWrapper)
				if err != nil {
					break
				}
			}
			stageWapper.Stage.Steps[index].StartTime = time.Now()
			stageWapper.Stage.Steps[index].Status = model.STATUS_RUNNING
			jober.SaveJobDetail(jobWrapper.Name, jobWrapper)
			// 如果 step 超时，则调用 cancel，在这里存储该 job 的计时器
			// 每次新 step 时，都会重新设置该计时器，所以不需要存储到底是哪个 step
			e.stepTimerMap.Store(utils.FormatJobToString(jobWrapper.Name, jobWrapper.Id), newStepTimer())
			if step.Uses == "" || step.Uses == "shell" {
				ah = action.NewShellAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "git-checkout" {
				ah = action.NewGitAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "hamster-ipfs" {
				ah = action.NewIpfsAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "hamster-pinata-ipfs" {
				ah = action.NewPinataIpfsAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "hamster-artifactory" {
				ah = action.NewArtifactoryAction(step, ctx, jobWrapper.Output)
				//} else if step.Uses == "deploy-contract" {
				//	ah = action.NewTruffleDeployAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "image-build" {
				ah = action.NewImageBuildAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "image-push" {
				ah = action.NewImagePushAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "k8s-frontend-deploy" {
				ah = action.NewK8sDeployAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "k8s-assign-domain" {
				ah = action.NewK8sIngressAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "sol-profiler-check" {
				ah = action.NewSolProfilerAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "solhint-check" {
				ah = action.NewSolHintAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "mythril-check" {
				ah = action.NewMythRilAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "slither-check" {
				ah = action.NewSlitherAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "check-aggregation" {
				ah = action.NewCheckAggregationAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "deploy-ink-contract" {
				ah = action.NewInkAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "frontend-check" {
				ah = action.NewEslintAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "eth-gas-reporter" {
				ah = action.NewEthGasReporterAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "workdir" {
				ah = action.NewWorkdirAction(step, ctx, jobWrapper.Output)
			} else if step.Uses == "openai" {
				ah = action.NewOpenaiAction(step, ctx, jobWrapper.Output)
			} else if strings.Contains(step.Uses, "/") {
				ah = action.NewRemoteAction(step, ctx)
			}
			jobWrapper.Output.NewStep(step.Name)
			err = executeAction(ah, jobWrapper)
			dataTime := time.Since(stageWapper.Stage.Steps[index].StartTime)
			stageWapper.Stage.Steps[index].Duration = dataTime.Milliseconds()
			if err != nil {
				stageWapper.Stage.Steps[index].Status = model.STATUS_FAIL
				break
			}
			stageWapper.Stage.Steps[index].Status = model.STATUS_SUCCESS
			jober.SaveJobDetail(jobWrapper.Name, jobWrapper)
		}

		for !stack.IsEmpty() {
			ah, _ := stack.Pop()
			_ = ah.Post()
		}

		if err != nil {
			stageWapper.Status = model.STATUS_FAIL
		} else {
			stageWapper.Status = model.STATUS_SUCCESS
		}
		dataTime := time.Since(stageWapper.StartTime)
		stageWapper.Duration = dataTime.Milliseconds()
		jobWrapper.Stages[index] = stageWapper
		jober.SaveJobDetail(jobWrapper.Name, jobWrapper)
		logger.Info("}")
		if err != nil {
			cancel()
			break
		}
	}
	jobWrapper.Output.Done()

	delete(e.cancelMap, job.Name)
	if err == nil {
		jobWrapper.Status = model.STATUS_SUCCESS
	} else {
		jobWrapper.Status = model.STATUS_FAIL
		jobWrapper.Error = err.Error()
	}

	dataTime := time.Since(jobWrapper.StartTime)
	jobWrapper.Duration = dataTime.Milliseconds()
	jober.SaveJobDetail(jobWrapper.Name, jobWrapper)

	return err
}

// Cancel 取消
func (e *Executor) Cancel(jobName string, id int) error {
	cancel, ok := e.cancelMap[strings.Join([]string{jobName, strconv.Itoa(id)}, "/")]
	if ok {
		cancel()
		// 删除
		delete(e.cancelMap, strings.Join([]string{jobName, strconv.Itoa(id)}, "/"))
	} else {
		logger.Errorf("job cancel function not found: %s/%d", jobName, id)
	}
	e.StatusChan <- model.NewStatusChangeMsg(jobName, id, model.STATUS_STOP)
	return nil
}

func (e *Executor) GetJobStatus(jobName string, jobID int) (model.Status, error) {
	_, ok := e.cancelMap[strings.Join([]string{jobName, strconv.Itoa(jobID)}, "/")]
	if ok {
		return model.STATUS_RUNNING, nil
	}
	return model.STATUS_NOTRUN, fmt.Errorf("job not found")
}

// 定时监听，以在任务超时时将其取消
func (e *Executor) handleTimerListener() {
	for {
		e.stepTimerMap.Range(func(key, value any) bool {
			timer := value.(*stepTimer)
			if timer.isTimeout() {
				name, id, err := utils.GetJobNameAndIDFromFormatString(key.(string))
				if err != nil {
					logger.Errorf("get job name and id from format string error: %v, key: %s", err, key.(string))
					return true
				}
				err = e.Cancel(name, id)
				if err != nil {
					logger.Errorf("cancel job error: %v, key: %s", err, key.(string))
				}
				e.stepTimerMap.Delete(key)
			}
			return true
		})
		time.Sleep(time.Minute)
	}
}

type stepTimer struct {
	startTime time.Time
}

func newStepTimer() *stepTimer {
	return &stepTimer{
		startTime: time.Now(),
	}
}

// 如果单个步骤超时了，就取消，超时时间暂定为 30 分钟
func (t *stepTimer) isTimeout() bool {
	return time.Since(t.startTime) > time.Minute*consts.STEP_TIMEOUT_MINUTE
}
