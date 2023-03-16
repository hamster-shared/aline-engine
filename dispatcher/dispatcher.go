package dispatcher

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
)

type IDispatcher interface {
	// DispatchNode 选择节点
	DispatchNode() (*model.Node, error)
	// Register 节点注册
	Register(node *model.Node) error
	// UnRegister 节点注销
	UnRegister(node *model.Node) error
	UnRegisterWithKey(key string) error
	// Ping 节点 ping
	Ping(node *model.Node) error
	// HealthcheckNode 检查节点心跳
	HealthcheckNode(node *model.Node)
	// SendJob 发送任务
	SendJob(name, yamlString string, jobDetailID int, node *model.Node) *api.AlineMessage
	// CancelJob 取消任务
	CancelJob(name string, jobDetailID int) (*api.AlineMessage, error)
	// CancelJobWithNode 通过指定节点取消任务
	CancelJobWithNode(name string, jobDetailID int, node *model.Node) *api.AlineMessage
	GetJobStatus(name string, jobDetailID int) (*api.AlineMessage, error)
	// IsValidNode 判断有没有这个节点
	IsValidNode(n string) bool
}

type GrpcDispatcher struct {
	nodes      sync.Map // key: node.Name + "@" + node.Address, value: NodeInfo // 记录有哪些节点
	poller     *Poller  // 轮询器，用来选择节点
	mu         sync.Mutex
	JobNodeMap sync.Map // key: jobname(id), value: []*node // 记录任务和节点的对应关系，用来取消任务，value 是一个数组，用来记录任务在哪些节点上执行过
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

func NewGrpcDispatcher() IDispatcher {
	return &GrpcDispatcher{
		poller: &Poller{
			index:   0,
			keyList: make([]string, 0),
		},
	}
}

// DispatchNode 选择节点
func (d *GrpcDispatcher) DispatchNode() (*model.Node, error) {
	// 加锁，防止多个 goroutine 同时修改 poller.index
	d.mu.Lock()
	if len(d.poller.keyList) == 0 {
		d.mu.Unlock()
		return nil, errors.New("no node available, len key list is 0")
	}
	key := d.poller.keyList[d.poller.index]
	d.poller.index = (d.poller.index + 1) % int64(len(d.poller.keyList))
	d.mu.Unlock()
	if value, ok := d.nodes.Load(key); ok {
		return value.(NodeInfo).node, nil
	}
	logger.Errorf("DispatchNode failed, node not exists: %s, index is %d", key, d.poller.index)
	logger.Tracef("list: %v", d.poller.keyList)
	return nil, errors.New("no node available")
}

// Register 节点注册
func (d *GrpcDispatcher) Register(node *model.Node) error {
	key := utils.GetNodeKey(node.Name, node.Address)
	if _, ok := d.nodes.Load(key); !ok {
		d.nodes.Store(key, NodeInfo{
			node:         node,
			lastPingTime: time.Now().Unix(),
		})
		d.mu.Lock()
		d.poller.keyList = append(d.poller.keyList, key)
		d.mu.Unlock()
		// 看看现在有几个节点
		logger.Tracef("Register node: %s, now have %d nodes", key, len(d.poller.keyList))
		return nil
	}
	logger.Tracef("Register node failed, node already exists: %s, now have %d nodes", key, len(d.poller.keyList))
	return errors.New("node already exists")
}

// UnRegister 节点注销
func (d *GrpcDispatcher) UnRegister(node *model.Node) error {
	key := utils.GetNodeKey(node.Name, node.Address)
	return d.unRegister(key)
}

func (d *GrpcDispatcher) UnRegisterWithKey(key string) error {
	return d.unRegister(key)
}

func (d *GrpcDispatcher) unRegister(key string) error {
	if _, ok := d.nodes.Load(key); ok {
		d.nodes.Delete(key)
		d.mu.Lock()
		if d.poller.index != 0 {
			d.poller.keyList = append(d.poller.keyList[:d.poller.index-1], d.poller.keyList[d.poller.index:]...)
			d.poller.index = (d.poller.index - 1) % int64(len(d.poller.keyList))
		} else {
			d.poller.keyList = d.poller.keyList[:len(d.poller.keyList)-1]
			d.poller.index = 0
		}
		d.mu.Unlock()
		logger.Tracef("UnRegister node: %s", key)
		return nil
	}
	return errors.New("node not exists")
}

// Ping 节点心跳
func (d *GrpcDispatcher) Ping(node *model.Node) error {
	key := utils.GetNodeKey(node.Name, node.Address)
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
func (d *GrpcDispatcher) HealthcheckNode(node *model.Node) {
	d.nodes.Range(func(_, value any) bool {
		nodeInfo := value.(NodeInfo)
		if time.Now().Unix()-nodeInfo.lastPingTime > 3*60 {
			d.UnRegister(nodeInfo.node)
		}
		return true
	})
}

// SendJob 发送任务
func (d *GrpcDispatcher) SendJob(name, yamlString string, jobDetailID int, node *model.Node) *api.AlineMessage {
	logger.Tracef("SendJob: %v to %s@%s", name, node.Name, node.Address)
	msg := &api.AlineMessage{
		Name:    node.Name,
		Address: node.Address,
		Type:    api.MessageType_EXECUTE,
		ExecReq: &api.ExecuteReq{
			Name:         name,
			PipelineFile: yamlString,
			JobDetailId:  int64(jobDetailID),
		},
	}
	if nodes, ok := d.JobNodeMap.Load(utils.FormatJobToString(name, jobDetailID)); ok {
		d.JobNodeMap.Store(utils.FormatJobToString(name, jobDetailID), append(nodes.([]*model.Node), node))
	} else {
		d.JobNodeMap.Store(utils.FormatJobToString(name, jobDetailID), []*model.Node{node})
	}
	return msg
}

// CancelJobWithNode 取消任务通过指定节点
func (d *GrpcDispatcher) CancelJobWithNode(name string, jobDetailID int, node *model.Node) *api.AlineMessage {
	logger.Tracef("CancelJob: %s(%d) to %s@%s", name, jobDetailID, node.Name, node.Address)
	msg := &api.AlineMessage{
		Name:    node.Name,
		Address: node.Address,
		Type:    api.MessageType_CANCEL,
		ExecReq: &api.ExecuteReq{
			Name:        name,
			JobDetailId: int64(jobDetailID),
		},
	}
	return msg
}

func (d *GrpcDispatcher) CancelJob(name string, jobDetailID int) (*api.AlineMessage, error) {
	node, err := d.GetJobLatestNode(name, jobDetailID)
	if err != nil {
		return nil, fmt.Errorf("job %s(%d) not found execute node", name, jobDetailID)
	}
	return d.CancelJobWithNode(name, jobDetailID, node), nil
}

// GetJobNode 获取任务执行节点
func (d *GrpcDispatcher) GetJobNode(name string, jobDetailID int) []*model.Node {
	if nodes, ok := d.JobNodeMap.Load(utils.FormatJobToString(name, jobDetailID)); ok {
		return nodes.([]*model.Node)
	}
	return nil
}

func (d *GrpcDispatcher) GetJobLatestNode(name string, id int) (*model.Node, error) {
	nodes := d.GetJobNode(name, id)
	if nil == nodes || len(nodes) == 0 {
		return nil, errors.New("job node not found")
	}
	return nodes[len(nodes)-1], nil
}

func (d *GrpcDispatcher) GetJobStatus(name string, id int) (*api.AlineMessage, error) {
	node, err := d.GetJobLatestNode(name, id)
	if err != nil {
		return nil, err
	}
	return &api.AlineMessage{
		Name:    node.Name,
		Address: node.Address,
		Type:    api.MessageType_STATUS,
		ExecReq: &api.ExecuteReq{
			Name:        name,
			JobDetailId: int64(id),
		},
	}, nil
}

func (d *GrpcDispatcher) IsValidNode(n string) bool {
	_, ok := d.nodes.Load(n)
	return ok
}
