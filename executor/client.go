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
	executor   *Executor
	QueueChan  chan *model.QueueMessage
	StatusChan chan model.StatusChangeMessage
}

func (c *ExecutorClient) Main() {
	// 持续监听任务队列
	go c.handleJobQueue()
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

		// 保存 job
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
			var err error
			if queueMessage.Command == model.Command_Start {
				err = c.executor.Execute(jobId, job)
			} else if queueMessage.Command == model.Command_Stop {
				err = c.executor.Cancel(jobId, job)
			}
			if err != nil {
				logger.Errorf("execute job error: %v", err)
			}
		}()
	}
}
