package model

type Command int

const (
	Command_Start Command = iota
	Command_Stop
)

type QueueMessage struct {
	JobName string
	JobId   int
	Command Command
	Node    *Node
}

func NewStartQueueMsg(name string, id int) QueueMessage {
	return QueueMessage{
		JobName: name,
		JobId:   id,
		Command: Command_Start,
	}

}

func NewStopQueueMsg(name string, id int) QueueMessage {
	return QueueMessage{
		JobName: name,
		JobId:   id,
		Command: Command_Stop,
	}

}

func NewStartQueueMsgToNode(name string, id int, node *Node) QueueMessage {
	return QueueMessage{
		JobName: name,
		JobId:   id,
		Command: Command_Start,
		Node:    node,
	}
}

func NewStopQueueMsgToNode(name string, id int, node *Node) QueueMessage {
	return QueueMessage{
		JobName: name,
		JobId:   id,
		Command: Command_Stop,
		Node:    node,
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
