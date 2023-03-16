package job

import (
	"fmt"
	"log"
	"os/exec"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func Test_SaveJob(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	step1 := model.Step{
		Name: "sun",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	job := model.Job{
		Version: "1",
		Name:    "mysql",
		Stages: map[string]model.Stage{
			"node": {
				Steps: steps,
				Needs: strs,
			},
		},
	}
	data, _ := yaml.Marshal(job)
	err := SaveJob("jian1", string(data))
	assert.NoError(t, err)
}

func Test_SaveJobDetail(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	step1 := model.Step{
		Name: "sun",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	stageDetail := model.StageDetail{
		Name: "string",
		Stage: model.Stage{
			Steps: steps,
			Needs: strs,
		},
		Status: model.STATUS_FAIL,
	}
	var stageDetails []model.StageDetail
	stageDetails = append(stageDetails, stageDetail)
	jobDetail := model.JobDetail{
		Id: 6,
		Job: model.Job{
			Version: "2",
			Name:    "mysql",
			Stages: map[string]model.Stage{
				"node": {
					Steps: steps,
					Needs: strs,
				},
			},
		},
		Status: model.STATUS_NOTRUN,
		Stages: stageDetails,
	}
	SaveJobDetail("sun", &jobDetail)
}

func Test_GetJob(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	data, err := GetJob("qiao")
	assert.Nil(t, err)
	log.Println(data)
	assert.NotNil(t, data)
}

func Test_UpdateJob(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	step1 := model.Step{
		Name: "jian",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	job := model.Job{
		Version: "1",
		Name:    "mysql",
		Stages: map[string]model.Stage{
			"node": {
				Steps: steps,
				Needs: strs,
			},
		},
	}
	data, _ := yaml.Marshal(job)
	err := UpdateJob("jian", "jian1", string(data))
	assert.NoError(t, err)
}

func Test_GetJobDetail(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	_, err := GetJobDetail("test1", 1)
	assert.Nil(t, err)
}

func Test_DeleteJob(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	err := DeleteJob("jian1")
	assert.NoError(t, err)
}

func Test_DeleteJobDetail(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	err := DeleteJobDetail("test1", 1)
	assert.NoError(t, err)
}

func Test_JobList(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	data, err := JobList("s", 1, 10)
	assert.Nil(t, err)
	assert.NotNil(t, data)
	t.Log(spew.Sdump(data))
}

func Test_JobDetailList(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	data, err := JobDetailList("hello-world", 2, 10)
	assert.Nil(t, err)
	assert.NotNil(t, data)
	t.Log(spew.Sdump(data))
}

func Test_CreateJobDetail(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	detail, err := CreateJobDetail("hello-world")
	assert.Nil(t, err)
	t.Log(spew.Sdump(detail))
}

func TestGetJobLog(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	log, err := GetJobLog("hello-world", 1000)
	assert.Nil(t, err)
	if log == nil {
		t.Error("log is nil")
	}
	spew.Dump(log)
}

func TestGetStageLog(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	log, err := GetJobStageLog("hello-world", 100, "say-hello", 0)
	assert.Nil(t, err)
	if log == nil {
		t.Error("log is nil")
	}
	spew.Dump(log)
}

func TestOpenFile(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	cmd := exec.Command("open", "/Users/sunjianguo/Desktop/miner")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}

func TestGetJobArtifactoryFilesData(t *testing.T) {
	files, err := GetJobArtifactoryFilesData("4c6d8534-df46-4dd1-9786-2b5f9a587143_507", "2")
	assert.Nil(t, err)
	for _, f := range files {
		t.Logf(f.Path)
	}
}

func TestGetJobArtifactoryFiles(t *testing.T) {
	files, err := GetJobArtifactoryFiles("4c6d8534-df46-4dd1-9786-2b5f9a587143_507", "2")
	assert.Nil(t, err)
	for _, f := range files {
		t.Logf(f)
	}
}

func TestGetJobArtifactoryDir(t *testing.T) {
	dir := GetJobArtifactoryDir("4c6d8534-df46-4dd1-9786-2b5f9a587143_507", "2")
	fmt.Println(dir)
	t.Logf(dir)
}

func TestGetJobCheckFilesData(t *testing.T) {
	files, err := GetJobCheckFilesData("4c6d8534-df46-4dd1-9786-2b5f9a587143_507", "2")
	assert.Nil(t, err)
	for _, f := range files {
		t.Logf(f.Path)
	}
}

func TestGetJobStepLog(t *testing.T) {
	step, err := GetJobStepLog("test", 10009, "第一阶段", "步骤 1")
	assert.Nil(t, err)
	spew.Dump(step)
}
