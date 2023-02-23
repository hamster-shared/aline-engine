package engine

import (
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestEngine(t *testing.T) {

	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)

	engine := NewEngine()

	jobName := "2e70fb35-f8b3-403e-ae0f-a31d5a945c05_75"
	data, _ := os.ReadFile("eth-gas-reporter.yml")
	yaml := string(data)
	fmt.Println(yaml)
	err := engine.CreateJob(jobName, yaml)
	assert.NoError(t, err)

	params := make(map[string]string)
	params["baseDir"] = "dist"
	params["ArtifactUrl"] = "file:///Users/mohaijiang/pipelines/jobs/2e70fb35-f8b3-403e-ae0f-a31d5a945c05_74/artifactory/2/frontend.zip"

	engine.SaveJobParams("2e70fb35-f8b3-403e-ae0f-a31d5a945c05_75", params)

	job := engine.GetJob(jobName)
	_, err = job.StageSort()
	assert.NoError(t, err)

	go engine.Start()
	detail, err := engine.ExecuteJob(jobName)

	assert.NoError(t, err)
	fmt.Println(detail.Id)

	for {
		time.Sleep(1 * time.Second)
		jobDetail := engine.GetJobHistory(jobName, detail.Id)
		if jobDetail.Status > model.STATUS_RUNNING {
			break
		}
	}

}

func TestURL(t *testing.T) {

	URL, err := url.Parse("file:///tmp/test/dist.zip")
	assert.NoError(t, err)
	fmt.Println(URL.RequestURI())

}
