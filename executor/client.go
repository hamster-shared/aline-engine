package executor

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/grpc/client"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/service"
)

func NewExecutorClient(channel chan model.QueueMessage, callbackChannel chan model.StatusChangeMessage, jobService service.Jober) *ExecutorClient {
	name, err := os.Hostname()
	if err != nil {
		logger.Errorf("get hostname error: %v", err)
		name = "unknown"
	}
	address, err := getClientIP()
	if err != nil {
		logger.Errorf("get client ip error: %v", err)
		address = "unknown"
	}

	return &ExecutorClient{
		name:    name,
		address: address,
		executor: &Executor{
			cancelMap:       make(map[string]func()),
			jobService:      jobService,
			callbackChannel: callbackChannel,
		},
		channel:         channel,
		callbackChannel: callbackChannel,
		msgChan:         make(chan *api.AlineMessage, 100),
	}
}

type ExecutorClient struct {
	name            string
	address         string
	executor        *Executor
	channel         chan model.QueueMessage
	callbackChannel chan model.StatusChangeMessage
	msgChan         chan *api.AlineMessage
}

func (c *ExecutorClient) Main() {
	// 向 master 节点的 grpc server 注册自己
	c.register()

	// 发送定时心跳，避免被 master 节点的 grpc server 踢出
	c.keepAlive()

	// 持续监听消息
	c.handleGrpcMessage()

	// for {
	// 	// 监听队列
	// 	queueMessage, ok := <-c.channel
	// 	if !ok {
	// 		logger.Error("executor client channel closed")
	// 		return
	// 	}
	// 	logger.Infof("executor client receive message: %v", queueMessage)

	// 	//4.TODO...，获取 job 信息

	// 	// TODO ... 计算 jobId
	// 	jobName := queueMessage.JobName
	// 	jobId := queueMessage.JobId

	// 	pipelineReader, err := c.executor.FetchJob(jobName)

	// 	if err != nil {
	// 		logger.Error(err)
	// 		continue
	// 	}

	// 	//5. 解析 pipeline
	// 	job, err := pipeline.GetJobFromReader(pipelineReader)

	// 	//6. 异步执行 pipeline
	// 	go func() {
	// 		var err error
	// 		if queueMessage.Command == model.Command_Start {
	// 			err = c.executor.Execute(jobId, job)
	// 		} else if queueMessage.Command == model.Command_Stop {
	// 			err = c.executor.Cancel(jobId, job)
	// 		}

	// 		if err != nil {

	// 		}
	// 	}()

	// }
}

func (c *ExecutorClient) Execute(jobId int, job *model.Job) error {
	return c.executor.Execute(jobId, job)
}

func (c *ExecutorClient) register() {
	client.GrpcClientStart("0.0.0.0:50051")
	msg := &api.AlineMessage{
		Type:    1,
		Name:    c.name,
		Address: c.address,
	}
	err := client.SendMessage(context.Background(), msg)
	if err != nil {
		logger.Errorf("register error: %v", err)
	}
}

func (c *ExecutorClient) keepAlive() {
	go func() {
		for {
			time.Sleep(time.Second * 30)
			err := client.SendMessage(context.Background(), &api.AlineMessage{
				Type:    3,
				Name:    c.name,
				Address: c.address,
			})
			if err != nil {
				logger.Errorf("ping error: %v", err)
				break
			}
		}
	}()
}

func (c *ExecutorClient) saveJob(msg *api.AlineMessage) error {
	name := msg.ExecReq.Name
	pipelineFileString := msg.ExecReq.PipelineFile
	// jobDetailID := msg.ExecReq.JobDetailId

	// 保存 job yaml 文件
	err := c.executor.jobService.SaveJob(name, pipelineFileString)
	if err != nil {
		logger.Errorf("save job error: %v", err)
		return err
	}
	return nil
}

func (c *ExecutorClient) received(msg *api.AlineMessage) error {
	err := client.SendMessage(context.Background(), &api.AlineMessage{
		Type:         8,
		Name:         c.name,
		Address:      c.address,
		ReceivedType: msg.Type,
		ReceivedName: msg.ExecReq.Name,
	})
	if err != nil {
		logger.Errorf("send message error: %v", err)
		return err
	}
	return nil
}

func (c *ExecutorClient) execJob(msg *api.AlineMessage) {
	// 先回复收到
	c.received(msg)
	// 保存 job yaml 文件
	c.saveJob(msg)

	// 然后呢
	job := c.executor.jobService.GetJobObject(msg.ExecReq.Name)
	if job == nil {
		// nil 表示 job 不存在，需要告诉 master 节点
	}
	c.Execute(int(msg.ExecReq.JobDetailId), job)

}

func (c *ExecutorClient) handleGrpcMessage() {
	logger.Debug("executor client start handle grpc message")
	client.RecvMessage(c.msgChan)
	go func() {
		for {
			msg := <-c.msgChan
			logger.Tracef("executor client receive message: %v", msg)
			// 这里只需要处理与任务执行有关的消息
			switch msg.Type {
			case 4:
				// 接收到 master 节点的执行任务
				c.execJob(msg)

			case 5:
				// 4. 接收到 master 节点的取消任务
			case 6:
			case 7:
			}
		}
	}()
}

func getClientIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("can not get ip")
}
