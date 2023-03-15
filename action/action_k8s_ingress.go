package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
)

type K8sIngressAction struct {
	gateway      string
	namespace    string
	projectName  string
	servicePorts string
	output       *output.Output
	ctx          context.Context
}

func NewK8sIngressAction(step model.Step, ctx context.Context, output *output.Output) *K8sIngressAction {
	return &K8sIngressAction{
		gateway:      step.With["gateway"],
		namespace:    step.With["namespace"],
		projectName:  step.With["project_name"],
		servicePorts: step.With["service_ports"],
		ctx:          ctx,
		output:       output,
	}
}

func (k *K8sIngressAction) Pre() error {
	stack := k.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	k.gateway = utils.ReplaceWithParam(k.gateway, params)
	logger.Debugf("k8s gateway : %s", k.gateway)
	k.namespace = utils.ReplaceWithParam(k.namespace, params)
	logger.Debugf("k8s namespace : %s", k.namespace)
	k.projectName = utils.ReplaceWithParam(k.projectName, params)
	logger.Debugf("k8s deploy project name is : %s", k.projectName)
	k.servicePorts = utils.ReplaceWithParam(k.servicePorts, params)
	logger.Debugf("k8s deploy service ports is : %s", k.servicePorts)
	return nil
}

func (k *K8sIngressAction) Hook() (*model.ActionResult, error) {
	client, err := utils.InitK8sClient()
	if err != nil {
		k.output.WriteLine(fmt.Sprintf("[ERROR]: k8s client init failed, %s", err.Error()))
		logger.Errorf("init k8s client failed: %s", err.Error())
		return nil, err
	}
	var servicePorts []corev1.ServicePort
	err = json.Unmarshal([]byte(k.servicePorts), &servicePorts)
	if err != nil {
		k.output.WriteLine(fmt.Sprintf("[ERROR]: k8s service ports format failed, %s", err.Error()))
		logger.Errorf("k8s service ports format failed: %s", err.Error())
		return nil, err
	}
	serviceName := fmt.Sprintf("%s-%s", k.namespace, k.projectName)
	_, err = utils.CreateIngress(client, k.namespace, serviceName, k.gateway, servicePorts)
	if err != nil {
		k.output.WriteLine(fmt.Sprintf("[ERROR]: k8s create ingress failed, %s", err.Error()))
		logger.Errorf("k8s create ingress  failed: %s", err.Error())
		return nil, err
	}
	name := fmt.Sprintf("%s-%s", k.namespace, k.projectName)
	for {
		log.Println("------")
		service, _ := client.CoreV1().Services(k.namespace).Get(context.Background(), name, metav1.GetOptions{})
		log.Println("**************************")
		pods, _ := client.CoreV1().Pods(k.namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", service.ObjectMeta.Name),
		})
		log.Println("-------------------------------")
		log.Println(len(pods.Items))
		log.Println("-------------------------------")
		if len(pods.Items) > 0 {
			log.Println("=++++++++++++++++++++")
			log.Println(pods.Items[0].Status.Phase)
			log.Println("=++++++++++++++++++++")
			if pods.Items[0].Status.Phase == corev1.PodRunning {
				break
			}
			if pods.Items[0].Status.Phase == corev1.PodFailed {
				return nil, errors.New("container deploy failed")
			}
		}
	}
	actionResult := &model.ActionResult{}
	deployInfo := model.DeployInfo{
		Url: fmt.Sprintf("http://%s.%s", serviceName, k.gateway),
	}
	actionResult.Deploys = append(actionResult.Deploys, deployInfo)
	log.Println("=======================")
	log.Println(actionResult)
	log.Println("=======================")
	return actionResult, nil
}
func (k *K8sIngressAction) Post() error {
	return nil
}
