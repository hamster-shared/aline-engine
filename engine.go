package engine

type Engine interface{}
type MasterEngine interface{}
type SlaveEngine interface{}

type Role int

const (
	Master Role = iota
	Worker
)

type engine struct {
	// channel         chan model.QueueMessage
	// callbackChannel chan model.StatusChangeMessage
	role   Role
	master *masterEngine
	worker *workerEngine
}

type EngineOption func(*engine)

func AsMaster(listenAddress string) EngineOption {
	return func(e *engine) {
		e.master = newMasterEngine(listenAddress)
		e.worker = nil
	}
}

func NewEngine(opts ...EngineOption) *engine {
	e := &engine{}
	e.worker = newWorkerEngine()
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *engine) Start() {
	e.worker.executeClient.Main()
}
