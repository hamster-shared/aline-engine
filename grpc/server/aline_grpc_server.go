package server

import (
	"fmt"
	"io"
	"net"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"google.golang.org/grpc"
)

var (
	alineGrpcServer *AlineGRPCServer
)

type AlineGRPCServer struct {
	api.UnimplementedAlineRPCServer
	msgChanRecv chan *api.AlineMessage
	msgChanSend chan *api.AlineMessage
	// 保存 node 节点和 stream 的映射关系
	// nodeStreamMap sync.Map
}
type NodeMessage struct {
	StreamID string
	Msg      *api.AlineMessage
}

func (s *AlineGRPCServer) AlineChat(stream api.AlineRPC_AlineChatServer) error {
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				continue
			}
			if err != nil {
				logger.Errorf("grpc server recv message failed: %v", err)
				break
			}
			// streamID := stream.Context().Value("streamID").(string)
			// s.msgChan <- &NodeMessage{
			// 	StreamID: streamID,
			// 	Msg:      msg,
			// }
			s.msgChanRecv <- msg
		}
	}()
	// go func() {
	for {
		msg := <-s.msgChanSend

		err := stream.Send(msg)

		if err != nil {
			logger.Errorf("grpc server send message failed: %v", err)
			break
		} else {
			logger.Tracef("grpc server send message success: %v", msg)
		}
	}
	// }()
	return nil
}

func GrpcServerStart(listenAddress string, msgChanRecv chan *api.AlineMessage, msgChanSend chan *api.AlineMessage) error {
	alineGrpcServer = &AlineGRPCServer{
		msgChanRecv: msgChanRecv,
		msgChanSend: msgChanSend,
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	api.RegisterAlineRPCServer(grpcServer, alineGrpcServer)
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		logger.Errorf("grpc server listen failed: %v", err)
		return err
	}
	logger.Infof("grpc server listen on %s", listenAddress)
	go func() {
		err := grpcServer.Serve(listener)
		if err != nil {
			logger.Errorf("grpc server serve failed: %v", err)
		}
	}()
	return nil
}

func SendMessage(msg *api.AlineMessage) error {
	if alineGrpcServer == nil {
		return fmt.Errorf("grpc server not start")
	}
	if alineGrpcServer.msgChanRecv == nil {
		return fmt.Errorf("grpc server msgChan not init")
	}
	alineGrpcServer.msgChanSend <- msg
	return nil
}
