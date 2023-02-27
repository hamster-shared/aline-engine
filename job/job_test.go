package job

import (
	"log"
	"os/exec"
	"testing"

	model2 "github.com/hamster-shared/aline-engine/model"
	"gopkg.in/yaml.v2"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	ass "gotest.tools/v3/assert"
)

func Test_SaveJob(t *testing.T) {
	step1 := model2.Step{
		Name: "sun",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model2.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	job := model2.Job{
		Version: "1",
		Name:    "mysql",
		Stages: map[string]model2.Stage{
			"node": {
				Steps: steps,
				Needs: strs,
			},
		},
	}
	jobService := NewJober()
	data, _ := yaml.Marshal(job)
	err := jobService.SaveJob("qiao", string(data))
	ass.NilError(t, err)
}

func Test_SaveJobDetail(t *testing.T) {
	step1 := model2.Step{
		Name: "sun",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model2.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	stageDetail := model2.StageDetail{
		Name: "string",
		Stage: model2.Stage{
			Steps: steps,
			Needs: strs,
		},
		Status: model2.STATUS_FAIL,
	}
	var stageDetails []model2.StageDetail
	stageDetails = append(stageDetails, stageDetail)
	jobDetail := model2.JobDetail{
		Id: 6,
		Job: model2.Job{
			Version: "2",
			Name:    "mysql",
			Stages: map[string]model2.Stage{
				"node": {
					Steps: steps,
					Needs: strs,
				},
			},
		},
		Status: model2.STATUS_NOTRUN,
		Stages: stageDetails,
	}
	jobService := NewJober()
	jobService.SaveJobDetail("sun", &jobDetail)
}

func Test_GetJob(t *testing.T) {
	jobService := NewJober()
	data := jobService.GetJob("guo")
	log.Println(data)
	assert.NotNil(t, data)
}

func Test_UpdateJob(t *testing.T) {
	jobService := NewJober()
	step1 := model2.Step{
		Name: "jian",
		Uses: "",
		With: map[string]string{
			"pipelie": "string",
			"data":    "data",
		},
		RunsOn: "open",
		Run:    "stage",
	}
	var steps []model2.Step
	var strs []string
	strs = append(strs, "strings")
	steps = append(steps, step1)
	job := model2.Job{
		Version: "1",
		Name:    "mysql",
		Stages: map[string]model2.Stage{
			"node": {
				Steps: steps,
				Needs: strs,
			},
		},
	}
	data, _ := yaml.Marshal(job)
	err := jobService.UpdateJob("guo", "jian", string(data))
	ass.NilError(t, err)
}

func Test_GetJobDetail(t *testing.T) {
	jobService := NewJober()
	data := jobService.GetJobDetail("sun", 3)
	assert.NotNil(t, data)
}

func Test_DeleteJob(t *testing.T) {
	jobService := NewJober()
	err := jobService.DeleteJob("sun")
	ass.NilError(t, err)
}

func Test_DeleteJobDetail(t *testing.T) {
	jobService := NewJober()
	err := jobService.DeleteJobDetail("cdqadqa92d3if4r9n8j0", 1)
	ass.NilError(t, err)
}

func Test_JobList(t *testing.T) {
	jobService := NewJober()
	data := jobService.JobList("cdqadqa92d3if4r9n8j0", 1, 10)
	assert.NotNil(t, data)
}

func Test_JobDetailList(t *testing.T) {
	jobService := NewJober()
	data := jobService.JobDetailList("sun", 2, 10)
	log.Println(data)
	assert.NotNil(t, data)
}

func Test_ExecuteJob(t *testing.T) {
	jobService := NewJober()
	jobService.ExecuteJob("sun")
}

func TestGetJobLog(t *testing.T) {
	jobService := NewJober()
	log := jobService.GetJobLog("test", 10001)
	if log == nil {
		t.Error("log is nil")
	}
	spew.Dump(log)
}

func TestGetStageLog(t *testing.T) {
	jobService := NewJober()
	log := jobService.GetJobStageLog("maven", 11, "code-compile", 0)
	if log == nil {
		t.Error("log is nil")
	}
	spew.Dump(log)
}

func TestOpenFile(t *testing.T) {
	cmd := exec.Command("open", "/Users/sunjianguo/Desktop/miner")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}
