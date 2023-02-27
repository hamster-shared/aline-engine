package client

import (
	"context"
	"testing"
	"time"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestClient(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	err := GrpcClientStart("localhost:50051")
	assert.NilError(t, err)
	logger.Infof("grpc client start success, connected to %s", "localhost:50051")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	assert.NilError(t, err)
	err = SendMessage(ctx, &api.AlineMessage{Type: 1})
	assert.NilError(t, err)

	msgChan := make(chan *api.AlineMessage)
	err = RecvMessage(msgChan)
	assert.NilError(t, err)

	for {
		msg := <-msgChan
		t.Logf("msg: %v", msg)
		assert.Equal(t, msg.Type, int64(1))
		break
	}
}
