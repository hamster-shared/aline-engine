package executor

import (
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
)

func NewExecutorClient() *ExecutorClient {
	statusChan := make(chan model.StatusChangeMessage, 100)
	return &ExecutorClient{
		executor: &Executor{
			cancelMap:  make(map[string]func()),
			StatusChan: statusChan,
		},
		QueueChan: make(chan *model.QueueMessage, 100),
	}
}

type ExecutorClient struct {
	executor  *Executor
	QueueChan chan *model.QueueMessage
}

func (c *ExecutorClient) Main() {
	// 持续监听任务队列
	go c.handleJobQueue()
}

func (c *ExecutorClient) GetStatusChangeChan() chan model.StatusChangeMessage {
	return c.executor.StatusChan
}

func (c *ExecutorClient) GetJobStatus(jobName string, jobID int) (model.Status, error) {
	return c.executor.GetJobStatus(jobName, jobID)
}

func (c *ExecutorClient) handleJobQueue() {
	for {
		// 监听队列
		queueMessage, ok := <-c.QueueChan
		if !ok {
			logger.Error("executor client channel closed")
			return
		}
		logger.Tracef("executor client receive message: %v", queueMessage)

		// 如果收到了停止任务的消息，那么就取消任务，结束本次循环
		if queueMessage.Command == model.Command_Stop {
			err := c.executor.Cancel(queueMessage.JobName, queueMessage.JobId)
			if err != nil {
				logger.Errorf("cancel job error: %v", err)
			}
			continue
		}

		// 否则，保存并执行 job
		err := jober.SaveJob(queueMessage.JobName, queueMessage.JobContent)
		if err != nil {
			logger.Errorf("save job error: %v", err)
			continue
		}

		jobName := queueMessage.JobName
		jobId := queueMessage.JobId

		job, err := jober.GetJobObject(jobName)
		if err != nil {
			logger.Errorf("get job error: %v", err)
			continue
		}

		//6. 异步执行 pipeline
		go func() {
			err := c.executor.Execute(jobId, job)
			if err != nil {
				logger.Errorf("execute job error: %v", err)
				// 在这里再次同步一次状态
				c.executor.StatusChan <- model.NewStatusChangeMsg(jobName, jobId, model.STATUS_FAIL)
			}
		}()
	}
}
