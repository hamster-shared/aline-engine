package engine

import (
	"fmt"
	"os"
	"testing"

	"github.com/hamster-shared/aline-engine/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestEngine(t *testing.T) {

	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)

	engine := NewEngine(AsMaster("0.0.0.0:50051"))

	jobName := "hello-world"
	data, err := os.ReadFile("test.yml")
	assert.NoError(t, err)

	yaml := string(data)
	fmt.Println(yaml)

	err = engine.CreateJob(jobName, yaml)
	assert.NoError(t, err)
	logger.Info("create job success")

	job := engine.GetJob(jobName)
	_, err = job.StageSort()
	assert.NoError(t, err)

	go engine.Start()
	detail, err := engine.ExecuteJob(jobName)
	assert.NoError(t, err)
	t.Log(detail)

	// for {
	// 	time.Sleep(1 * time.Second)
	// 	jobDetail := engine.GetJobHistory(jobName, detail.Id)
	// 	if jobDetail.Status > model.STATUS_RUNNING {
	// 		break
	// 	}
	// }

}

// func TestURL(t *testing.T) {

// 	URL, err := url.Parse("file:///tmp/test/dist.zip")
// 	assert.NoError(t, err)
// 	fmt.Println(URL.RequestURI())

// }

// func TestEngineWork(t *testing.T) {
// 	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
// 	e := NewEngine(AsMaster("0.0.0.0:50051"))
// 	e.Start()
// }
