package client

import (
	"context"
	"io"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AlineGrpcClient struct {
	rpc         api.AlineRPCClient
	RecvMsgChan chan *api.AlineMessage
	SendMsgChan chan *api.AlineMessage
	ErrorChan   chan error
}

func GrpcClientStart(masterAddress string) (*AlineGrpcClient, error) {

	conn, err := grpc.Dial(masterAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Errorf("grpc did not connect: %v", err)
		return nil, err
	}

	// 不关闭，在程序运行期间，一直保持连接
	// defer conn.Close()

	alineGrpcClient := &AlineGrpcClient{
		rpc:         api.NewAlineRPCClient(conn),
		RecvMsgChan: make(chan *api.AlineMessage, 100),
		SendMsgChan: make(chan *api.AlineMessage, 100),
		ErrorChan:   make(chan error, 100),
	}

	err = alineGrpcClient.handleMessage()
	if err != nil {
		return nil, err
	}
	return alineGrpcClient, nil
}

func (c *AlineGrpcClient) handleMessage() error {
	stream, err := c.rpc.AlineChat(context.Background())
	if err != nil {
		logger.Errorf("gprc client send message failed: %v", err)
		return err
	}

	go func() {
		for {
			msg := <-c.SendMsgChan
			if err := stream.Send(msg); err != nil {
				logger.Errorf("gprc client send message failed: %v", err)
				c.ErrorChan <- err
				panic(err)
			} else {
				logger.Tracef("gprc client send message success: %v", msg)
			}
		}
	}()

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					logger.Warnf("gprc client recv message failed: %v", err)
					stream.CloseSend()
					return
				}
				logger.Errorf("gprc client recv message failed: %v", err)
				c.ErrorChan <- err
				return
			}
			logger.Tracef("gprc client recv message success: %v", msg)
			c.RecvMsgChan <- msg
			logger.Tracef("len(c.RecvMsgChan): %v", len(c.RecvMsgChan))
		}
	}()
	return nil
}
