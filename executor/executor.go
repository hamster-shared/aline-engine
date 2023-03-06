package executor

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hamster-shared/aline-engine/action"
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
)

type IExecutor interface {
	// Execute 执行任务
	Execute(id int, job *model.Job) error
	// HandlerLog 处理日志
	HandlerLog(jobId int)
	//SendResultToQueue 发送结果到队列
	SendResultToQueue(channel chan model.StatusChangeMessage, jobWrapper *model.JobDetail)

	Cancel(id int, job *model.Job) error
}

type Executor struct {
	cancelMap  map[string]func()
	StatusChan chan model.StatusChangeMessage
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

	if err != nil {
		return err
	}

	// 2. 初始化 执行器的上下文

	env := make([]string, 0)
	env = append(env, "NAME="+job.Name)

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

	executeAction := func(ah action.ActionHandler, job *model.JobDetail) error {
		if jobWrapper.Status != model.STATUS_RUNNING {
			return nil
		}
		err := ah.Pre()
		if err != nil {
			job.Status = model.STATUS_FAIL
			fmt.Println(err)
			return err
		}
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
		if err != nil {
			job.Status = model.STATUS_FAIL
			return err
		}
		return nil
	}

	jobWrapper.Output = output.New(job.Name, jobWrapper.Id)

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
			} else if step.Uses == "workdir" {
				ah = action.NewWorkdirAction(step, ctx, jobWrapper.Output)
			} else if strings.Contains(step.Uses, "/") {
				ah = action.NewRemoteAction(step, ctx)
			}
			err = executeAction(ah, jobWrapper)
			dataTime := time.Since(stageWapper.Stage.Steps[index].StartTime)
			stageWapper.Stage.Steps[index].Duration = dataTime.Milliseconds()
			if err != nil {
				stageWapper.Stage.Steps[index].Status = model.STATUS_FAIL
				break
			}
			stageWapper.Stage.Steps[index].Status = model.STATUS_SUCCESS
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

	//TODO ... 发送结果到队列
	e.SendResultToQueue(jobWrapper)
	//_ = os.RemoveAll(path.Join(engineContext["hamsterRoot"].(string), job.Name))

	return err

}

// SendResultToQueue 发送结果到队列
func (e *Executor) SendResultToQueue(job *model.JobDetail) {
	//TODO ...
	e.StatusChan <- model.NewStatusChangeMsg(job.Name, job.Id, job.Status)
}

// Cancel 取消
func (e *Executor) Cancel(id int, job *model.Job) error {
	cancel, ok := e.cancelMap[strings.Join([]string{job.Name, strconv.Itoa(id)}, "/")]
	if ok {
		cancel()
	}
	return nil
}
