package engine

import (
	"fmt"
	"time"

	"github.com/hamster-shared/aline-engine/dispatcher"
	"github.com/hamster-shared/aline-engine/executor"
	"github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
)

type workerEngine struct {
	jober         job.Jober
	executeClient *executor.ExecutorClient
}

func newWorkerEngine() *workerEngine {
	e := &workerEngine{}
	channel := make(chan model.QueueMessage)
	callbackChannel := make(chan model.StatusChangeMessage)
	jobService := job.NewJober()
	executeClient := executor.NewExecutorClient(channel, callbackChannel, jobService)
	e.jober = jobService
	e.executeClient = executeClient
	return e
}

func (e *workerEngine) CreateJob(name string, yaml string) error {
	return e.jober.SaveJob(name, yaml)
}

func (e *workerEngine) SaveJobParams(name string, params map[string]string) error {
	return e.jober.SaveJobParams(name, params)
}

func (e *workerEngine) DeleteJob(name string) error {
	return e.jober.DeleteJob(name)
}

func (e *workerEngine) UpdateJob(name, newName, jobYaml string) error {

	return e.jober.UpdateJob(name, newName, jobYaml)
}

func (e *workerEngine) GetJob(name string) *model.Job {
	return e.jober.GetJobObject(name)
}

func (e *workerEngine) GetJobs(keyword string, page int, size int) *model.JobPage {
	return e.jober.JobList(keyword, page, size)
}

func (e *workerEngine) ExecuteJob(name string) (*model.JobDetail, error) {
	logger.Debugf("execute job:%s", name)

	job := e.jober.GetJobObject(name)
	jobDetail, err := e.jober.ExecuteJob(name)
	if err != nil {
		return nil, err
	}

	return jobDetail, nil
}

func (e *workerEngine) ReExecuteJob(name string, historyId int) error {
	err := e.jober.ReExecuteJob(name, historyId)
	if err != nil {
		logger.Error(fmt.Sprintf("re execute job error:%s", err.Error()))
		return err
	}
	job := e.jober.GetJobObject(name)
	jobDetail := e.jober.GetJobDetail(name, historyId)
	node, err := e.dispatch.DispatchNode(job)
	if err != nil {
		logger.Error(fmt.Sprintf("dispatch node error:%s", err.Error()))
		return err
	}
	e.dispatch.SendJob(jobDetail, node)
	return err
}

func (e *workerEngine) TerminalJob(name string, historyId int) error {

	err := e.jober.StopJobDetail(name, historyId)
	if err != nil {
		return err
	}
	job := e.jober.GetJobObject(name)
	jobDetail := e.jober.GetJobDetail(name, historyId)
	node, err := e.dispatch.DispatchNode(job)
	if err != nil {
		logger.Error(fmt.Sprintf("dispatch node error:%s", err.Error()))
		return err
	}
	e.dispatch.CancelJob(jobDetail, node)
	return nil
}

func (e *workerEngine) GetJobHistory(name string, historyId int) *model.JobDetail {
	return e.jober.GetJobDetail(name, historyId)
}

func (e *workerEngine) GetJobHistorys(name string, page, size int) *model.JobDetailPage {
	return e.jober.JobDetailList(name, page, size)
}

func (e *workerEngine) DeleteJobHistory(name string, historyId int) error {
	return e.jober.DeleteJobDetail(name, historyId)
}

func (e *workerEngine) GetJobHistoryLog(name string, historyId int) *model.JobLog {
	return e.jober.GetJobLog(name, historyId)
}

func (e *workerEngine) GetJobHistoryStageLog(name string, historyId int, stageName string, start int) *model.JobStageLog {
	return e.jober.GetJobStageLog(name, historyId, stageName, start)
}

func (e *workerEngine) GetCodeInfo(name string, historyId int) string {
	jobDetail := e.jober.GetJobDetail(name, historyId)
	if jobDetail != nil {
		return jobDetail.CodeInfo
	}
	return ""
}

// func (e *workerEngine) RegisterStatusChangeHook(hookResult func(message model.StatusChangeMessage)) {
// 	for { //

// 		//3. 监听队列
// 		statusMsg, ok := <-e.callbackChannel
// 		if !ok {
// 			return
// 		}

// 		fmt.Println("=======[status callback]=========")
// 		fmt.Println(statusMsg)
// 		fmt.Println("=======[status callback]=========")

// 		if hookResult != nil {
// 			hookResult(statusMsg)
// 		}
// 	}
// }
