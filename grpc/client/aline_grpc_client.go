package client

import (
	"context"
	"io"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpcClient api.AlineRPCClient
)

func GrpcClientStart(address string) error {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Errorf("grpc did not connect: %v", err)
		return err
	}
	// 不关闭，在程序运行期间，一直保持连接
	// defer conn.Close()
	grpcClient = api.NewAlineRPCClient(conn)
	return nil
}

func RecvMessage(msgChan chan *api.AlineMessage) error {
	stream, err := grpcClient.AlineChat(context.Background())
	if err != nil {
		logger.Errorf("gprc client recv message failed: %v", err)
		return err
	} else {
		logger.Trace("gprc client create recv message success")
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					// 什么都不做
					continue
				}
				logger.Errorf("gprc client recv message failed: %v", err)
				return
			}
			logger.Tracef("gprc client recv message success: %v", msg)
			msgChan <- msg
		}
	}()
	return nil
}

func SendMessage(ctx context.Context, msg *api.AlineMessage) error {
	stream, err := grpcClient.AlineChat(ctx)
	if err != nil {
		logger.Errorf("gprc client send message failed: %v", err)
		return err
	}

	if err := stream.Send(msg); err != nil {
		logger.Errorf("gprc client send message failed: %v", err)
		return err
	}
	stream.CloseSend()
	return nil
}
