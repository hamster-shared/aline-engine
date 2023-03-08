package server

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type AlineGrpcServer struct {
	api.UnimplementedAlineRPCServer
	RecvMsgChan chan *api.AlineMessage
	SendMsgChan chan *api.AlineMessage
	conns       sync.Map   // 保存 node 节点和 stream 连接的映射关系
	ErrorChan   chan error // 用来接收 grpc server 发送消息的错误
}

type streamConnection struct {
	stream api.AlineRPC_AlineChatServer
}

// AlineChat 每当有新的连接建立时，都会调用这个函数，不同的连接会有不同的 stream
func (s *AlineGrpcServer) AlineChat(stream api.AlineRPC_AlineChatServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// 当客户端主动关闭连接时，从连接映射 map 中删除此项，退出此函数
			s.deleteStream(stream)
			break
		}
		if err != nil {
			logger.Errorf("grpc server recv message failed: %v", err)
			s.deleteStream(stream)
			break
		}

		// 保存 node 节点和 stream 的映射关系
		key := utils.GetNodeKey(msg.Name, msg.Address)
		s.conns.Store(key, streamConnection{
			stream: stream,
		})

		// 将收到的消息放入 channel 中，供 engine 处理
		s.RecvMsgChan <- msg
		logger.Tracef("len(s.RecvMsgChan): %v", len(s.RecvMsgChan))
	}
	return nil
}

// 删除保存的 stream 连接
func (s *AlineGrpcServer) deleteStream(stream api.AlineRPC_AlineChatServer) {
	s.conns.Range(func(key, value any) bool {
		if value.(streamConnection).stream == stream {
			s.conns.Delete(key)
			return false
		}
		return true
	})
}

// 获取保存的 stream 连接
func (s *AlineGrpcServer) getConn(key string) (streamConnection, bool) {
	value, ok := s.conns.Load(key)
	if !ok {
		return streamConnection{}, false
	}
	return value.(streamConnection), true
}

func GrpcServerStart(listenAddress string) (*AlineGrpcServer, error) {

	alineGrpcServer := &AlineGrpcServer{
		RecvMsgChan: make(chan *api.AlineMessage, 10000),
		SendMsgChan: make(chan *api.AlineMessage, 10000),
		ErrorChan:   make(chan error, 100),
	}

	// var opts []grpc.ServerOption
	grpcServer := grpc.NewServer()
	api.RegisterAlineRPCServer(grpcServer, alineGrpcServer)

	// Register reflection service on gRPC server. grpc 反射，方便使用 grpcurl 命令行工具调试
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		logger.Errorf("grpc server listen failed: %v", err)
		return nil, err
	}

	// 此处 goroutine 用来启动 grpc server
	go func() {
		logger.Infof("grpc server listen on %s", listenAddress)
		err := grpcServer.Serve(listener)
		if err != nil {
			logger.Errorf("grpc server serve failed: %v", err)
			// grpc server 监听失败，直接使程序 panic
			// goroutine 内的 panic，如果没有 recover，也会导致整个程序退出
			panic(err)
		}
	}()

	// 此处 goroutine 用来发送消息
	go func() {
		for {
			// 从 channel 中取出消息，发送给客户端
			msg := <-alineGrpcServer.SendMsgChan

			conn, ok := alineGrpcServer.getConn(utils.GetNodeKey(msg.Name, msg.Address))
			if !ok {
				logger.Errorf("grpc server send message failed: don't find stream connection")
				// 这个 error channel，当发送失败时，用来通知上层，做重试或其他操作
				if msg.Type == 4 {
					logger.Errorf("grpc server send message failed: %v", fmt.Errorf("don't find stream connection"))
					alineGrpcServer.ErrorChan <- &model.SendJobError{
						Err:       fmt.Errorf("don't find stream connection"),
						JobName:   msg.ExecReq.Name,
						JobID:     int(msg.ExecReq.JobDetailId),
						ErrorNode: utils.GetNodeKey(msg.Name, msg.Address),
					}
					continue
				}
				continue
			}

			err := conn.stream.Send(msg)
			if err != nil {
				if msg.Type == 4 {
					logger.Errorf("grpc server send message failed: %v", err)
					alineGrpcServer.ErrorChan <- &model.SendJobError{
						Err:       err,
						JobName:   msg.ExecReq.Name,
						JobID:     int(msg.ExecReq.JobDetailId),
						ErrorNode: utils.GetNodeKey(msg.Name, msg.Address),
					}
					continue
				}
				logger.Errorf("grpc server send message failed: %v", err)
				alineGrpcServer.ErrorChan <- err
			}
			logger.Tracef("len(alineGrpcServer.SendMsgChan): %d", len(alineGrpcServer.SendMsgChan))
		}
	}()

	return alineGrpcServer, nil
}
