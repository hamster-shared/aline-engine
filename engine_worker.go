package engine

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/executor"
	"github.com/hamster-shared/aline-engine/grpc/api"
	grpcClient "github.com/hamster-shared/aline-engine/grpc/client"
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
)

type workerEngine struct {
	name, address string
	masterAddress string
	executeClient *executor.ExecutorClient
	rpcClient     *grpcClient.AlineGrpcClient
	doneJobList   sync.Map
}

func newWorkerEngine(masterAddress string) (*workerEngine, error) {
	e := &workerEngine{}
	e.name, _ = utils.GetMyHostname()
	if masterAddress[:9] == "127.0.0.1" {
		e.address = "127.0.0.1"
	} else {
		e.address, _ = utils.GetMyIP()
	}
	e.masterAddress = masterAddress
	e.executeClient = executor.NewExecutorClient()

	rpcClient, err := grpcClient.GrpcClientStart(masterAddress)
	if err != nil {
		return nil, err
	}
	e.rpcClient = rpcClient

	e.handleGrpcMessage()
	e.register()
	e.keepAlive()

	e.executeClient.Main()
	e.handleDoneJob()

	return e, nil
}

// 处理 grpc client 收到的消息，这里只处理与任务执行有关的消息
func (e *workerEngine) handleGrpcMessage() {
	logger.Debug("worker engine start handle grpc message")
	// 接收消息
	go func() {
		for {
			// 这里只需要处理与任务执行有关的消息
			switch msg := <-e.rpcClient.RecvMsgChan; msg.Type {
			case api.MessageType_EXECUTE:
				// 4 接收到 master 节点的执行任务
				logger.Tracef("worker engine receive execute job message: %v", msg)
				e.executeClient.QueueChan <- model.NewStartQueueMsg(msg.ExecReq.Name, msg.ExecReq.PipelineFile, int(msg.ExecReq.JobDetailId))
				e.sendLogJobDetail(msg)

			case api.MessageType_CANCEL:
				// 5 接收到 master 节点的取消任务
				logger.Tracef("worker engine receive cancel job message: %v", msg)
				e.executeClient.QueueChan <- model.NewStopQueueMsg(msg.ExecReq.Name, msg.ExecReq.PipelineFile, int(msg.ExecReq.JobDetailId))

			case api.MessageType_STATUS:
				// master 询问 job 状态
				logger.Tracef("worker engine receive status job message: %v", msg)
				e.sendJobStatus(msg)
			}
		}
	}()
}

// 向 master 注册自己
func (e *workerEngine) register() {
	e.rpcClient.SendMsgChan <- &api.AlineMessage{
		Type:    api.MessageType_REGISTER,
		Name:    e.name,
		Address: e.address,
	}
	logger.Trace("worker engine register success")
}

// 向 master 定时发送心跳
func (e *workerEngine) keepAlive() {
	go func() {
		for {
			time.Sleep(time.Second * 30)
			e.rpcClient.SendMsgChan <- &api.AlineMessage{
				Type:    api.MessageType_HEARTBEAT,
				Name:    e.name,
				Address: e.address,
			}
			logger.Trace("worker engine send ping message")
			logger.Tracef("length of send message channel: %d", len(e.rpcClient.SendMsgChan))
		}
	}()
}

func (e *workerEngine) handleDoneJob() {
	go func() {
		for {
			statusChan := e.executeClient.GetStatusChangeChan()
			jobResultStatus := <-statusChan
			e.doneJobList.Store(utils.FormatJobToString(jobResultStatus.JobName, jobResultStatus.JobId), struct{}{})
			logger.Debugf("job %s-%d done, status: %d", jobResultStatus.JobName, jobResultStatus.JobId, jobResultStatus.Status.ToString())
			if e.address != "127.0.0.1" {
				// 回传日志
				logMsg, _ := e.getLogAndJobDetailMessage(jobResultStatus.JobName, jobResultStatus.JobId)
				e.rpcClient.SendMsgChan <- logMsg

				// 回传 report
				reports, err := jober.GetJobCheckFilesData(jobResultStatus.JobName, strconv.Itoa(jobResultStatus.JobId))
				if err != nil {
					logger.Warnf("get job %s-%d report error: %s, this may not be needed", jobResultStatus.JobName, jobResultStatus.JobId, err.Error())
				}
				for _, report := range reports {
					e.rpcClient.SendMsgChan <- &api.AlineMessage{
						Type:    api.MessageType_FILE,
						Name:    e.name,
						Address: e.address,
						File:    report,
					}
				}

				// 回传构建物
				artifactorys, err := jober.GetJobArtifactoryFilesData(jobResultStatus.JobName, strconv.Itoa(jobResultStatus.JobId))
				if err != nil {
					logger.Warnf("get job %s-%d artifactory error: %s, this may not be needed", jobResultStatus.JobName, jobResultStatus.JobId, err.Error())
				}
				for _, artifactory := range artifactorys {
					e.rpcClient.SendMsgChan <- &api.AlineMessage{
						Type:    api.MessageType_FILE,
						Name:    e.name,
						Address: e.address,
						File:    artifactory,
					}
				}
			}
			// 告诉 master 任务执行完成
			e.rpcClient.SendMsgChan <- &api.AlineMessage{
				Type:    api.MessageType_RESULT,
				Name:    e.name,
				Address: e.address,
				Result: &api.ExecuteResult{
					JobName:   jobResultStatus.JobName,
					JobID:     int64(jobResultStatus.JobId),
					JobStatus: int64(jobResultStatus.Status),
				},
			}
			logger.Debugf("job %s-%d result send to master", jobResultStatus.JobName, jobResultStatus.JobId)
		}
	}()
}

// 回传日志和 job detail
func (e *workerEngine) sendLogJobDetail(msg *api.AlineMessage) {
	if e.address == "127.0.0.1" {
		return
	}
	go func() {
		errorCounter := 0
		for {
			// 检查是否已经完成
			doneJobKey := utils.FormatJobToString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
			if _, ok := e.doneJobList.Load(doneJobKey); ok {
				e.doneJobList.Delete(doneJobKey)
				return
			}

			logMsg, err := e.getLogAndJobDetailMessage(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
			if err != nil {
				if errorCounter > 10 {
					logger.Errorf("get job log string error: %v", err)
					return
				}
				// 刚建立任务的时候，可能日志还没出来，错误是正常的，先等一会儿
				errorCounter++
				time.Sleep(time.Millisecond * 500)
				continue
			}
			e.rpcClient.SendMsgChan <- logMsg

			// 0.5s 发送一次日志
			time.Sleep(time.Millisecond * 500)
		}
	}()
}

func (e *workerEngine) getLogAndJobDetailMessage(jobName string, jobID int) (*api.AlineMessage, error) {
	logString, err := jober.GetJobLogString(jobName, jobID)
	if err != nil {
		logger.Errorf("get job log string error: %v", err)
		return nil, fmt.Errorf("get job log string error: %v", err)
	}
	jobDetailString, err := jober.ReadStringJobDetail(jobName, jobID)
	if err != nil {
		logger.Errorf("get job detail string failed: %s", err)
		return nil, fmt.Errorf("get job detail string failed: %s", err)
	}
	return &api.AlineMessage{
		Type:    api.MessageType_LOG,
		Name:    e.name,
		Address: e.address,
		ExecReq: &api.ExecuteReq{
			Name:         jobName,
			JobDetailId:  int64(jobID),
			PipelineFile: jobDetailString,
		},
		Log: logString,
	}, nil
}

func (e *workerEngine) GetJobStatus(jobName string, jobID int) (model.Status, error) {
	return e.executeClient.GetJobStatus(jobName, jobID)
}

func (e *workerEngine) sendJobStatus(msg *api.AlineMessage) {
	status, err := e.GetJobStatus(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
	if err != nil {
		logger.Errorf("get job %s-%d status error: %s", msg.ExecReq.Name, msg.ExecReq.JobDetailId, err.Error())
		status = model.STATUS_NOTRUN
	}
	e.rpcClient.SendMsgChan <- &api.AlineMessage{
		Type:    api.MessageType_STATUS,
		Name:    e.name,
		Address: e.address,
		Status:  convertStatus(status),
		ExecReq: &api.ExecuteReq{
			Name:        msg.ExecReq.Name,
			JobDetailId: msg.ExecReq.JobDetailId,
		},
	}
}

func convertStatus(status model.Status) api.JobStatus {
	switch status {
	case model.STATUS_NOTRUN:
		return api.JobStatus_NOTRUN
	case model.STATUS_RUNNING:
		return api.JobStatus_RUNNING
	case model.STATUS_FAIL:
		return api.JobStatus_FAIL
	case model.STATUS_SUCCESS:
		return api.JobStatus_SUCCESS
	case model.STATUS_STOP:
		return api.JobStatus_STOP
	}
	return api.JobStatus_NOTRUN
}
