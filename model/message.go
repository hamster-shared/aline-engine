package model

type Command int

const (
	Command_Start Command = iota
	Command_Stop
)

type QueueMessage struct {
	JobName    string
	JobId      int
	JobContent string
	Command    Command
}

func NewStartQueueMsg(name, content string, id int) *QueueMessage {
	return &QueueMessage{
		JobName:    name,
		JobId:      id,
		JobContent: content,
		Command:    Command_Start,
	}

}

func NewStopQueueMsg(name, content string, id int) *QueueMessage {
	return &QueueMessage{
		JobName:    name,
		JobId:      id,
		JobContent: content,
		Command:    Command_Stop,
	}

}

type StatusChangeMessage struct {
	JobName string
	JobId   int
	Status  Status
}

func NewStatusChangeMsg(name string, id int, status Status) StatusChangeMessage {
	return StatusChangeMessage{
		name,
		id,
		status,
	}
}
