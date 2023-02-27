package dispatcher

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/executor"
	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/grpc/server"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
)

type IDispatcher interface {
	// DispatchNode 选择节点
	DispatchNode() (*model.Node, error)
	// Register 节点注册
	Register(node *model.Node) error
	// UnRegister 节点注销
	UnRegister(node *model.Node) error
	// Ping
	Ping(node *model.Node) error

	// HealthcheckNode 节点心跳
	HealthcheckNode(node *model.Node)

	// SendJob 发送任务
	SendJob(name, yameString string, jobDetailID int, node *model.Node)

	// CancelJob 取消任务
	CancelJob(job *model.JobDetail, node *model.Node)

	// GetExecutor 根据节点获取执行器
	// TODO ... 这个方法设计的不好，分布式机构后应当用 api 代替
	GetExecutor(node *model.Node) executor.IExecutor
	Received(ReceivedInfo)
	IsReceived(ReceivedInfo) bool
}

type Dispatcher struct {
	Channel         chan model.QueueMessage
	CallbackChannel chan model.StatusChangeMessage
	nodes           []*model.Node
}

func NewDispatcher(channel chan model.QueueMessage, callbackChannel chan model.StatusChangeMessage) *Dispatcher {
	return &Dispatcher{
		Channel:         channel,
		CallbackChannel: callbackChannel,
		nodes:           make([]*model.Node, 0),
	}
}

// DispatchNode 选择节点
func (d *Dispatcher) DispatchNode(job *model.Job) *model.Node {

	//TODO ... 单机情况直接返回 本地
	if len(d.nodes) > 0 {
		return d.nodes[0]
	}
	return nil
}

// Register 节点注册
func (d *Dispatcher) Register(node *model.Node) {
	d.nodes = append(d.nodes, node)
	return
}

// UnRegister 节点注销
func (d *Dispatcher) UnRegister(node *model.Node) {
	return
}

// HealthcheckNode 节点心跳
func (d *Dispatcher) HealthcheckNode(*model.Node) {
	// TODO  ... 检查注册的心跳信息，超过 3 分钟没有更新的节点，踢掉
	return
}

// SendJob 发送任务
func (d *Dispatcher) SendJob(job *model.JobDetail, node *model.Node) {

	// TODO ... 单机情况下 不考虑节点，直接发送本地
	// TODO ... 集群情况下 通过注册的 ip 地址进行 api 接口调用

	d.Channel <- model.NewStartQueueMsg(job.Name, job.Id)

	return
}

// CancelJob 取消任务
func (d *Dispatcher) CancelJob(job *model.JobDetail, node *model.Node) {

	d.Channel <- model.NewStopQueueMsg(job.Name, job.Id)
	return
}

// GetExecutor 根据节点获取执行器
// TODO ... 这个方法设计的不好，分布式机构后应当用 api 代替
func (d *Dispatcher) GetExecutor(node *model.Node) executor.IExecutor {
	return nil
}

type HttpDispatcher struct {
	// Channel         chan model.QueueMessage
	// CallbackChannel chan model.StatusChangeMessage
	msgChan     chan *api.AlineMessage
	nodes       sync.Map // key: node.Name + "@" + node.Address, value: NodeInfo
	poller      *Poller
	receivedMap sync.Map
}

type NodeInfo struct {
	node         *model.Node
	lastPingTime int64
}

// Poller 轮询器
type Poller struct {
	index   int64
	keyList []string
}

func NewHttpDispatcher(msgChan chan *api.AlineMessage) IDispatcher {
	return &HttpDispatcher{
		msgChan: msgChan,
		poller: &Poller{
			index:   0,
			keyList: make([]string, 0),
		},
	}
}

// DispatchNode 选择节点
func (d *HttpDispatcher) DispatchNode() (*model.Node, error) {
	if len(d.poller.keyList) == 0 {
		return nil, errors.New("no node available")
	}
	key := d.poller.keyList[d.poller.index]
	d.poller.index = (d.poller.index + 1) % int64(len(d.poller.keyList))
	if value, ok := d.nodes.Load(key); ok {
		return value.(NodeInfo).node, nil
	}
	return nil, errors.New("no node available")
}

// Register 节点注册
func (d *HttpDispatcher) Register(node *model.Node) error {
	key := node.Name + "@" + node.Address
	if _, ok := d.nodes.Load(key); !ok {
		d.nodes.Store(key, NodeInfo{
			node:         node,
			lastPingTime: time.Now().Unix(),
		})
		d.poller.keyList = append(d.poller.keyList, key)
		return nil
	}
	return errors.New("node already exists")
}

// UnRegister 节点注销
func (d *HttpDispatcher) UnRegister(node *model.Node) error {
	key := node.Name + "@" + node.Address
	if _, ok := d.nodes.Load(key); ok {
		d.nodes.Delete(key)
		d.poller.keyList = append(d.poller.keyList[:d.poller.index], d.poller.keyList[d.poller.index+1:]...)
		logger.Tracef("UnRegister node: %s", key)
		return nil
	}
	return errors.New("node not exists")
}

// Ping 节点心跳
func (d *HttpDispatcher) Ping(node *model.Node) error {
	key := node.Name + "@" + node.Address
	if _, ok := d.nodes.Load(key); ok {
		d.nodes.Store(key, NodeInfo{
			node:         node,
			lastPingTime: time.Now().Unix(),
		})
		return nil
	}
	return errors.New("node not exists")
}

// HealthcheckNode 检查节点心跳
func (d *HttpDispatcher) HealthcheckNode(node *model.Node) {
	d.nodes.Range(func(_, value any) bool {
		nodeInfo := value.(NodeInfo)
		if time.Now().Unix()-nodeInfo.lastPingTime > 3*60 {
			d.UnRegister(nodeInfo.node)
		}
		return true
	})
}

// SendJob 发送任务
func (d *HttpDispatcher) SendJob(name, yamlString string, jobDetailID int, node *model.Node) {
	logger.Tracef("SendJob: %v to %s@%s", name, node.Name, node.Address)
	msg := &api.AlineMessage{
		Type: 4,
		ExecReq: &api.ExecuteReq{
			Name:         name,
			PipelineFile: yamlString,
			JobDetailId:  int64(jobDetailID),
		},
	}
	server.SendMessage(msg)
}

// CancelJob 取消任务
func (d *HttpDispatcher) CancelJob(job *model.JobDetail, node *model.Node) {
	logger.Tracef("CancelJob: %v to %s@%s", job.Name, node.Name, node.Address)
	msg := &api.AlineMessage{
		Type: 5,
		ExecReq: &api.ExecuteReq{
			Name:         job.Name,
			PipelineFile: job.ToString(),
			JobDetailId:  int64(job.Id),
		},
	}
	d.msgChan <- msg
}

// GetExecutor 根据节点获取执行器
// TODO ... 这个方法设计的不好，分布式机构后应当用 api 代替
func (d *HttpDispatcher) GetExecutor(node *model.Node) executor.IExecutor {

	return nil
}

type ReceivedInfo struct {
	AlineMessageType int
	JobName          string
	Node             string
}

func (d *HttpDispatcher) IsReceived(receivedInfo ReceivedInfo) bool {
	logger.Tracef("IsReceived: %v", receivedInfo)
	k := fmt.Sprintf("%d@%s@%s", receivedInfo.AlineMessageType, receivedInfo.JobName, receivedInfo.Node)
	if _, ok := d.receivedMap.Load(k); ok {
		return true
	}
	return false
}

func (d *HttpDispatcher) Received(receivedInfo ReceivedInfo) {
	logger.Tracef("Received: %v", receivedInfo)
	k := fmt.Sprintf("%d@%s@%s", receivedInfo.AlineMessageType, receivedInfo.JobName, receivedInfo.Node)
	d.receivedMap.Store(k, receivedInfo)
}
