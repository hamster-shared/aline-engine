package engine

import (
	"github.com/hamster-shared/aline-engine/dispatcher"
	"github.com/hamster-shared/aline-engine/grpc/server"
	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
)

type masterEngine struct {
	dispatch  dispatcher.IDispatcher
	rpcServer *server.AlineGrpcServer
}

func newMasterEngine(listenAddress string) (*masterEngine, error) {
	e := &masterEngine{}
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

			logger.Tracef("grpc server recv message: %v", msg)
			switch msg.Type {
			case 1:
				// 注册
				err := e.dispatch.Register(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("register node error: %v", err)
				} else {
					logger.Debugf("register node success: %v", msg)
				}

			case 2:
				// 注销
				err := e.dispatch.UnRegister(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("unregister node error: %v", err)
				} else {
					logger.Debugf("unregister node success: %v", msg)
				}

			case 3:
				// 心跳
				err := e.dispatch.Ping(&model.Node{
					Name:    msg.Name,
					Address: msg.Address,
				})
				if err != nil {
					logger.Errorf("node ping error: %v", err)
				} else {
					logger.Tracef("node ping success: %v", msg)
				}
			case 4:
				// 这是属于 worker 节点的消息，不需要处理
			case 5:
				// 执行状态

			case 7:
				// 接收到任务的执行日志，保存起来
				// 如果是本机 worker 节点的日志，就不需要保存了
				if msg.Address == "127.0.0.1" {
					break
				}
				err := jober.SaveJobLogString(msg.ExecReq.Name, int(msg.ExecReq.JobDetailId), msg.Log)
				if err != nil {
					logger.Errorf("save job log error: %v", err)
				}

			default:
				logger.Warnf("grpc server recv unknown message: %v", msg)
			}
		}
	}()
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

func (e *masterEngine) registerStatusChangeHook(ch chan model.StatusChangeMessage) {
	// TODO

}

func (e *masterEngine) terminalJob(name string, id int) error {
	// TODO
	return nil
}
