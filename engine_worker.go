package engine

import (
	"fmt"
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
			case 4:
				// 接收到 master 节点的执行任务
				logger.Tracef("worker engine receive execute job message: %v", msg)
				e.executeClient.QueueChan <- model.NewStartQueueMsg(msg.ExecReq.Name, msg.ExecReq.PipelineFile, int(msg.ExecReq.JobDetailId))
				e.sendLogJobDetail(msg)

			case 5:
				// 4. 接收到 master 节点的取消任务
				logger.Tracef("worker engine receive cancel job message: %v", msg)
				e.executeClient.QueueChan <- model.NewStopQueueMsg(msg.ExecReq.Name, msg.ExecReq.PipelineFile, int(msg.ExecReq.JobDetailId))
			case 6:
			case 7:
			}
		}
	}()
}

// 向 master 注册自己
func (e *workerEngine) register() {
	e.rpcClient.SendMsgChan <- &api.AlineMessage{
		Type:    1,
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
				Type:    3,
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
			doneJob := <-e.executeClient.DoneJobChan
			e.doneJobList.Store(utils.FormatJobToString(doneJob.Name, doneJob.ID), struct{}{})
		}
	}()
}

// 回传日志和 job detail
func (e *workerEngine) sendLogJobDetail(msg *api.AlineMessage) {
	go func() {
		errorCounter := 0
		for {
			logMsg, err := getLogAndJobDetailMsg(msg)
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

			// 检查是否已经完成
			doneJobKey := utils.FormatJobToString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
			if _, ok := e.doneJobList.Load(doneJobKey); ok {
				e.doneJobList.Delete(doneJobKey)
				// 任务完成后再传一次，免得日志不完整
				logMsg, _ := getLogAndJobDetailMsg(msg)
				e.rpcClient.SendMsgChan <- logMsg
				return
			}

			// 0.5s 发送一次日志
			time.Sleep(time.Millisecond * 500)
		}
	}()
}

func getLogAndJobDetailMsg(msg *api.AlineMessage) (*api.AlineMessage, error) {
	logString, err := jober.GetJobLogString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
	if err != nil {
		logger.Errorf("get job log string error: %v", err)
		return nil, fmt.Errorf("get job log string error: %v", err)
	}
	jobDetailString, err := jober.ReadStringJobDetail(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId))
	if err != nil {
		logger.Errorf("get job detail string failed: %s", err)
		return nil, fmt.Errorf("get job detail string failed: %s", err)
	}
	return &api.AlineMessage{
		Type:    7,
		Name:    msg.Name,
		Address: msg.Address,
		ExecReq: &api.ExecuteReq{
			Name:         msg.ExecReq.Name,
			JobDetailId:  msg.ExecReq.JobDetailId,
			PipelineFile: jobDetailString,
		},
		Log: logString,
	}, nil
}
