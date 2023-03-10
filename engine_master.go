package engine

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/dispatcher"
	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/grpc/server"
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
)

type masterEngine struct {
	dispatch         dispatcher.IDispatcher
	rpcServer        *server.AlineGrpcServer
	statusChangeChan chan model.StatusChangeMessage
	jobStatusMap     sync.Map // key: jobname(id), value: jobStatus
}

func newMasterEngine(listenAddress string) (*masterEngine, error) {
	e := &masterEngine{}
	e.statusChangeChan = make(chan model.StatusChangeMessage, 100)
	rpcServer, err := server.GrpcServerStart(listenAddress)
	if err != nil {
		logger.Errorf("grpc server start failed: %v", err)
		return nil, err
	}

	e.rpcServer = rpcServer
	e.dispatch = dispatcher.NewGrpcDispatcher()
	e.handleGrpcServerMessage()
	e.handleGrpcServerError()
	return e, nil
}

// GrpcServerHandleMessage Master 节点处理 grpc server 收到的消息的地方
func (e *masterEngine) handleGrpcServerMessage() {
	// 这个用来接收 grpc server 收到的消息
	go func() {
		logger.Debugf("grpc server start listen message")
		for {
			msg, ok := <-e.rpcServer.RecvMsgChan
			if !ok {
				logger.Error("grpc server message channel closed")
				return
			}

			logger.Tracef("grpc server recv message: msg.Type: %v", msg.Type)
			switch msg.Type {
			case api.MessageType_REGISTER:
				// 1 注册
				err := e.dispatch.Register(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("register node error: %v", err)
				} else {
					logger.Debugf("register node success: %v", msg)
				}

			case api.MessageType_UNREGISTER:
				// 2 注销
				err := e.dispatch.UnRegister(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("unregister node error: %v", err)
				} else {
					logger.Debugf("unregister node success: %v", msg)
				}

			case api.MessageType_HEARTBEAT:
				// 3 心跳
				err := e.dispatch.Ping(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("node ping error: %v", err)
				} else {
					logger.Tracef("node ping success: %v", msg)
				}
				logger.Tracef("len(e.statusChangeChan): %d", len(e.statusChangeChan))
			case api.MessageType_EXECUTE, api.MessageType_CANCEL:
				// 4，5 这里应该是 master 发给 worker 的，如果 worker 发来这个，是不对的
				logger.Warnf("grpc server recv job exec request: %v", msg)
			case api.MessageType_RESULT:
				// 6 执行结果
				logger.Debugf("grpc server recv job exec result: %v", msg)
				status, err := model.IntToStatus(int(msg.Result.JobStatus))
				if err != nil {
					logger.Errorf("IntToStatus error: %v", err)
				}
				e.statusChangeChan <- model.NewStatusChangeMsg(msg.Result.JobName, int(msg.Result.JobID), status)

			case api.MessageType_LOG:
				// 7 接收到任务的执行日志和修改了的 job detail，保存起来
				err := jober.SaveJobLogString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId), msg.Log)
				if err != nil {
					logger.Errorf("save job log error: %s", err)
				}
				err = jober.SaveStringJobDetail(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId), msg.ExecReq.PipelineFile)
				if err != nil {
					logger.Errorf("save job detail error: %s", err)
				}
			case api.MessageType_ERROR:
				// 8 接收到任务的执行错误信息
				logger.Debugf("grpc server recv message: %v", msg)
			case api.MessageType_FILE:
				// 9 接收到文件
				logger.Debugf("grpc server recv file message, file name: %s", msg.File.Path)
				err := e.saveFile(msg.File)
				if err != nil {
					logger.Errorf("save file error: %s", err)
				}
			case api.MessageType_STATUS:
				// 10 接收到 job 的状态
				e.jobStatusMap.Store(utils.FormatJobToString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId)), msg.Status)

			default:
				logger.Warnf("grpc server recv unknown message: %v", msg)
			}
		}
	}()
}

func (e *masterEngine) saveFile(msg *api.File) error {
	jobsDir := jober.GetJobsDir()
	return jober.SaveFile(filepath.Join(jobsDir, msg.Path), msg.Data)
}

func (e *masterEngine) handleGrpcServerError() {
	go func() {
		for {
			err, ok := <-e.rpcServer.ErrorChan
			if !ok {
				logger.Error("grpc server error channel closed")
				return
			}
			switch err := err.(type) {
			// 如果是发送任务出错，就重新分发任务
			case *model.SendJobError:
				logger.Tracef("grpc server send job error: %v", err)
				// 删掉出错的节点
				e.dispatch.UnRegisterWithKey(err.ErrorNode)
				// 重新分发任务
				e.dispatchJob(err.JobName, err.JobID)
			default:
				logger.Errorf("grpc server error: %v", err)
			}
		}
	}()
}

// dispatchJob 分发任务
func (e *masterEngine) dispatchJob(name string, id int) error {
	var node *model.Node
	var err error
	for retry := 0; retry < 3; retry++ {
		node, err = e.dispatch.DispatchNode()
		if err != nil {
			logger.Errorf("dispatch node error: %s, retry counter: %d", err.Error(), retry)
			time.Sleep(time.Second * 3)
			continue
		} else {
			break
		}
	}
	if err != nil {
		return err
	}
	jobYamlString, err := jober.GetJob(name)
	if err != nil {
		return err
	}
	e.rpcServer.SendMsgChan <- e.dispatch.SendJob(name, jobYamlString, id, node)
	return nil
}

// 取消任务
func (e *masterEngine) cancelJob(name string, id int) error {
	msg, err := e.dispatch.CancelJob(name, id)
	if err != nil {
		return err
	}
	e.rpcServer.SendMsgChan <- msg
	return nil
}

func (e *masterEngine) registerStatusChangeHook(hook func(message model.StatusChangeMessage)) {
	if hook != nil {
		logger.Debugf("register status change hook")
		go func() {
			for {
				msg := <-e.statusChangeChan
				logger.Infof("hook status change message: %v", msg)
				hook(msg)
			}
		}()
	}
}

func (e *masterEngine) getJobStatus(name string, id int) (model.Status, error) {
	msg, err := e.dispatch.GetJobStatus(name, id)
	if err != nil {
		return model.STATUS_NOTRUN, err
	}
	e.rpcServer.SendMsgChan <- msg
	for i := 0; i < 10; i++ {
		status, ok := e.jobStatusMap.Load(utils.FormatJobToString(name, id))
		if ok {
			e.jobStatusMap.Delete(utils.FormatJobToString(name, id))
			return convertJobStatus(status.(api.JobStatus)), nil
		}
		time.Sleep(time.Second)
	}
	return model.STATUS_NOTRUN, fmt.Errorf("get job status timeout")
}

func convertJobStatus(status api.JobStatus) model.Status {
	switch status {
	case api.JobStatus_NOTRUN:
		return model.STATUS_NOTRUN
	case api.JobStatus_RUNNING:
		return model.STATUS_RUNNING
	case api.JobStatus_FAIL:
		return model.STATUS_FAIL
	case api.JobStatus_SUCCESS:
		return model.STATUS_SUCCESS
	case api.JobStatus_STOP:
		return model.STATUS_STOP
	default:
		return model.STATUS_NOTRUN
	}
}
