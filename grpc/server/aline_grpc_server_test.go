package server

import (
	"testing"
	"time"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStartServer(t *testing.T) {
	// 初始化日志
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	// 创建一个接收消息的 channel
	msgChan := make(chan *api.AlineMessage)
	// 启动 grpc server
	err := GrpcServerStart("localhost:50051", msgChan)
	assert.Nil(t, err)
	// 接收注册消息
	msg := <-msgChan
	assert.Equal(t, msg.Type, int64(1))
	// 发送同样的消息
	err = SendMessage(msg)
	assert.Nil(t, err)
	// 不要关闭，因为客户端还在监听，如果这里关闭了，客户端的测试就会失败，晚一会儿
	time.Sleep(time.Second)
}
