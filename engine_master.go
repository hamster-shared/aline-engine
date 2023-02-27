package engine

import (
	"fmt"

	"github.com/hamster-shared/aline-engine/dispatcher"
	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/grpc/server"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
)

type masterEngine struct {
	dispatch    dispatcher.IDispatcher
	msgChanRecv chan *api.AlineMessage
	msgChanSend chan *api.AlineMessage
	*workerEngine
}

func newMasterEngine(listenAddress string) *masterEngine {
	e := &masterEngine{}
	e.msgChanRecv = make(chan *api.AlineMessage, 100)
	e.msgChanSend = make(chan *api.AlineMessage, 100)
	server.GrpcServerStart(listenAddress, e.msgChanRecv, e.msgChanSend)
	dispatch := dispatcher.NewHttpDispatcher(e.msgChanRecv)
	e.dispatch = dispatch
	e.GrpcServerHandleMessage()
	return e
}

// GrpcServerHandleMessage Master 节点处理 grpc server 收到的消息的地方
func (e *masterEngine) GrpcServerHandleMessage() {
	// 这个用来接收 grpc server 收到的消息
	go func() {
		logger.Debugf("grpc server start listen message")
		for {
			msg, ok := <-e.msgChanRecv
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
				// 接受执行

			case 5:
				// 接受取消执行

			case 6:
				// 执行结果通知

			case 7:
				// 执行日志
			case 8:
				// 收到了任务
				receivedInfo := dispatcher.ReceivedInfo{
					AlineMessageType: int(msg.ReceivedType),
					Node:             fmt.Sprintf("%s@%s", msg.Name, msg.Address),
					JobName:          msg.ReceivedName,
				}
				e.dispatch.Received(receivedInfo)

			default:
				logger.Warnf("grpc server recv unknown message: %v", msg)
			}
		}
	}()
}

// DispatchJob 分发任务
func (e *masterEngine) DispatchJob() {
	node, err := e.dispatch.DispatchNode()
	if err != nil {
		logger.Errorf("dispatch node error:%s", err.Error())
		return nil, err
	}
	yamlString := e.jober.GetJob(name)
	// 发送 job，如果收不到响应，就重试，10 次不成功，就放弃
	e.dispatch.SendJob(name, yamlString, node)
	receivedInfo := dispatcher.ReceivedInfo{
		AlineMessageType: 4,
		Node:             fmt.Sprintf("%s@%s", node.Name, node.Address),
		JobName:          name,
	}
	for i := 0; i < 10; i++ {
		if i == 9 {
			logger.Errorf("send job to node:%s failed, already retry 10 times", node)
			return nil, fmt.Errorf("send job to node:%s failed, already retry 10 times", node)
		}
		if e.dispatch.IsReceived(receivedInfo) {
			logger.Tracef("send job to node:%s success", node)
			break
		} else {
			logger.Warnf("send job failed, retry %d", i)
			e.dispatch.SendJob(jobDetail, node)
			time.Sleep(1 * time.Second)
		}
	}
}
