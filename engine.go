package engine

import (
	"fmt"

	jober "github.com/hamster-shared/aline-engine/job"
	"github.com/hamster-shared/aline-engine/model"
)

type Engine interface {
	CreateJob(name string, yaml string) error
	SaveJobParams(name string, params map[string]string) error
	DeleteJob(name string) error
	UpdateJob(name, newName, jobYaml string) error
	GetJob(name string) (*model.Job, error)
	GetJobs(keyword string, page, size int) (*model.JobPage, error)
	GetCodeInfo(name string, historyId int) (string, error)
	ExecuteJob(name string, id int) error
	GetJobHistory(name string, id int) (*model.JobDetail, error)
	CreateJobDetail(name string) (*model.JobDetail, error)
	RegisterStatusChangeHook(ch chan model.StatusChangeMessage)
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
	e := &engine{}
	e.role = RoleMaster

	var err error
	e.master, err = newMasterEngine(fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		return nil, err
	}

	// e.worker, err = newWorkerEngine(fmt.Sprintf("127.0.0.1:%d", listenPort))
	// if err != nil {
	// 	return nil, err
	// }

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

func (e *engine) ExecuteJob(name string, id int) error {
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

func (e *engine) CreateJobDetail(name string) (*model.JobDetail, error) {
	return jober.CreateJobDetail(name)
}

func (e *engine) RegisterStatusChangeHook(ch chan model.StatusChangeMessage) {
	if e.role != RoleMaster {
		return
	}
	e.master.registerStatusChangeHook(ch)
}