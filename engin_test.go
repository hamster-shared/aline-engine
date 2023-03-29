package engine

import (
	"os"
	"testing"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

// func TestEngine(t *testing.T) {

// 	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)

// 	_ = NewEngine(AsMaster("0.0.0.0:50051"))
// 	time.Sleep(1 * time.Hour)

// jobName := "hello-world"
// data, err := os.ReadFile("test.yml")
// assert.NoError(t, err)

// yaml := string(data)
// fmt.Println(yaml)

// err = e.CreateJob(jobName, yaml)
// assert.NoError(t, err)
// logger.Info("create job success")

// job := e.GetJob(jobName)
// _, err = job.StageSort()
// assert.NoError(t, err)

// detail, err := e.ExecuteJob(jobName)
// assert.NoError(t, err)
// t.Log(detail)

// for {
// 	time.Sleep(1 * time.Second)
// 	jobDetail := engine.GetJobHistory(jobName, detail.Id)
// 	if jobDetail.Status > model.STATUS_RUNNING {
// 		break
// 	}
// }

// }

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

// func TestWorkerEngine(t *testing.T) {

// 	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)

// 	e := NewEngine()

// 	jobName := "hello-world"
// 	data, err := os.ReadFile("test.yml")
// 	assert.NoError(t, err)

// 	yaml := string(data)
// 	fmt.Println(yaml)

// 	err = e.CreateJob(jobName, yaml)
// 	assert.NoError(t, err)
// 	logger.Info("create job success")

// 	job := e.GetJob(jobName)
// 	_, err = job.StageSort()
// 	assert.NoError(t, err)

// 	detail, err := e.ExecuteJob(jobName)
// 	assert.NoError(t, err)
// 	t.Log(detail)

// 	// for {
// 	// 	time.Sleep(1 * time.Second)
// 	// 	jobDetail := engine.GetJobHistory(jobName, detail.Id)
// 	// 	if jobDetail.Status > model.STATUS_RUNNING {
// 	// 		break
// 	// 	}
// 	// }

//		time.Sleep(1000 * time.Second)
//	}
func TestEngineWork(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	e, err := NewMasterEngine(50001)
	assert.NilError(t, err)
	createJob(e)
	go func() {
		_, err = e.ExecuteJob("aptos-check", 22)
		if err != nil {
			t.Errorf("execute job error: %v", err)
		}
		// time.Sleep(120 * time.Second)
	}()
	http.ListenAndServe("0.0.0.0:6060", nil)
}

func TestEngineStepTimeoutCancel(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	e, err := NewMasterEngine(50001)
	assert.NilError(t, err)
	createJob(e)
	go func() {
		_, err = e.ExecuteJob("hello1", 2)
		if err != nil {
			t.Errorf("execute job error: %v", err)
		}
		// time.Sleep(120 * time.Second)
	}()
	http.ListenAndServe("0.0.0.0:6060", nil)
}

func TestWorkerEngineWork(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	_, err := NewWorkerEngine("0.0.0.0:50001")
	assert.NilError(t, err)
	time.Sleep(1000 * time.Second)
}

func createJob(e Engine) {
	data, err := os.ReadFile("aptos-check.yml")
	if err != nil {
		panic(err)
	}
	err = e.CreateJob("aptos-check", string(data))
	if err != nil {
		panic(err)
	}
}

func TestEngineGetJobStatus(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	e, err := NewMasterEngine(50001)
	assert.NilError(t, err)
	createJob(e)
	var jobDetail *model.JobDetail
	go func() {
		jobDetail, err = e.ExecuteJob("hello1", 1)
		if err != nil {
			t.Errorf("execute job error: %v", err)
		}
	}()
	go func() {
		for {
			time.Sleep(10 * time.Second)
			status, err := e.GetCurrentJobStatus("hello1", jobDetail.Id)
			if err != nil {
				t.Errorf("get job status error: %v", err)
			}
			t.Logf("job status: %v", status)
		}
	}()
	http.ListenAndServe("0.0.0.0:6060", nil)
}

func TestEngineGetJobStatusWithCustom(t *testing.T) {
	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)
	e, err := NewMasterEngine(50001)
	assert.NilError(t, err)
	time.Sleep(time.Second)

	status, err := e.GetCurrentJobStatus("hello1", 39)
	if err != nil {
		t.Errorf("get job status error: %v", err)
	}
	t.Logf("job status: %v", status)
}
