package queue

import (
	"github.com/hamster-shared/aline-engine/model"
)

type IQueue interface {
	Push(job *model.Job, node *model.Node)
	Listener() chan *model.Job
}

type Queue struct {
}
