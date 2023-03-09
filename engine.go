package engine

import (
	"fmt"
	"os"

	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/sirupsen/logrus"
)

type Engine interface {
	CreateJob(name string, yaml string) error
	SaveJobParams(name string, params map[string]string) error
	DeleteJob(name string) error
	UpdateJob(name, newName, jobYaml string) error
	GetJob(name string) (*model.Job, error)
	GetJobs(keyword string, page, size int) (*model.JobPage, error)
	GetCodeInfo(name string, historyId int) (string, error)
	ExecuteJob(name string) (*model.JobDetail, error)
	ReExecuteJob(name string, id int) error
	GetJobHistory(name string, id int) (*model.JobDetail, error)
	GetJobHistorys(name string, page, size int) (*model.JobDetailPage, error)
	DeleteJobHistory(name string, id int) error
	CreateJobDetail(name string) (*model.JobDetail, error)
	RegisterStatusChangeHook(hook func(message model.StatusChangeMessage))
	GetJobHistoryLog(name string, id int) (*model.JobLog, error)
	GetJobHistoryStageLog(name string, id int, stageName string, start int) (*model.JobStageLog, error)
	TerminalJob(name string, id int) error
}

type Role int

const (
	RoleMaster Role = iota
	RoleWorker
)

type engine struct {
	role   Role
	master *masterEngine
	worker *workerEngine
}

func NewMasterEngine(listenPort int) (Engine, error) {
	logger.Init().ToStdoutAndFile().SetLevel(readLogLevelFromEnv())
	e := &engine{}
	e.role = RoleMaster

	var err error
	e.master, err = newMasterEngine(fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		return nil, err
	}

	e.worker, err = newWorkerEngine(fmt.Sprintf("127.0.0.1:%d", listenPort))
	if err != nil {
		return nil, err
	}

	return e, nil
}

func NewWorkerEngine(masterAddress string) (Engine, error) {
	e := &engine{}
	e.role = RoleWorker
	var err error
	e.worker, err = newWorkerEngine(masterAddress)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *engine) CreateJob(name string, yaml string) error {
	return jober.SaveJob(name, yaml)
}

func (e *engine) SaveJobParams(name string, params map[string]string) error {
	return jober.SaveJobParams(name, params)
}

func (e *engine) DeleteJob(name string) error {
	return jober.DeleteJob(name)
}

func (e *engine) UpdateJob(name, newName, jobYaml string) error {
	return jober.UpdateJob(name, newName, jobYaml)
}

func (e *engine) GetJob(name string) (*model.Job, error) {
	return jober.GetJobObject(name)
}

func (e *engine) GetJobs(keyword string, page, size int) (*model.JobPage, error) {
	return jober.JobList(keyword, page, size)
}

func (e *engine) GetCodeInfo(name string, historyId int) (string, error) {
	jobDetail, err := jober.GetJobDetail(name, historyId)
	if err != nil {
		return "", err
	}
	return jobDetail.CodeInfo, nil
}

func (e *engine) ExecuteJob(name string) (*model.JobDetail, error) {
	if e.role != RoleMaster {
		return nil, fmt.Errorf("only master can execute job")
	}
	jobDetail, err := e.CreateJobDetail(name)
	if err != nil {
		return nil, err
	}
	return jobDetail, e.master.dispatchJob(name, jobDetail.Id)
}

func (e *engine) ReExecuteJob(name string, id int) error {
	if e.role != RoleMaster {
		return fmt.Errorf("only master can execute job")
	}
	return e.master.dispatchJob(name, id)
}

func (e *engine) CancelJob(name string, id int) error {
	if e.role != RoleMaster {
		return fmt.Errorf("only master can cancel job")
	}
	return e.master.cancelJob(name, id)
}

func (e *engine) GetJobHistory(name string, id int) (*model.JobDetail, error) {
	return jober.GetJobDetail(name, id)
}

func (e *engine) DeleteJobHistory(name string, id int) error {
	return jober.DeleteJobDetail(name, id)
}

func (e *engine) CreateJobDetail(name string) (*model.JobDetail, error) {
	return jober.CreateJobDetail(name)
}

func (e *engine) RegisterStatusChangeHook(hook func(message model.StatusChangeMessage)) {
	if e.role != RoleMaster {
		return
	}
	logger.Infof("register status change hook")
	e.master.registerStatusChangeHook(hook)
}

func (e *engine) GetJobHistorys(name string, page, size int) (*model.JobDetailPage, error) {
	return jober.JobDetailList(name, page, size)
}

func (e *engine) GetJobHistoryLog(name string, id int) (*model.JobLog, error) {
	return jober.GetJobLog(name, id)
}

func (e *engine) GetJobHistoryStageLog(name string, id int, stageName string, start int) (*model.JobStageLog, error) {
	return jober.GetJobStageLog(name, id, stageName, start)
}

func (e *engine) TerminalJob(name string, id int) error {
	if e.role != RoleMaster {
		return fmt.Errorf("only master can terminal job")
	}
	return e.master.cancelJob(name, id)
}

func readLogLevelFromEnv() logrus.Level {
	levelStr := os.Getenv("ALINE_LOG_LEVEL")
	if levelStr == "" {
		return logrus.InfoLevel
	}
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		return logrus.InfoLevel
	}
	return level
}
