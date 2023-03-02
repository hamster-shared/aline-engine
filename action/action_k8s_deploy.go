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
	"os/exec"
)

type K8sDeployAction struct {
	gateway      string
	namespace    string
	containers   string
	image        string
	projectName  string
	servicePorts string
	ctx          context.Context
	output       *output.Output
}

func NewK8sDeployAction(step model.Step, ctx context.Context, output *output.Output) *K8sDeployAction {
	return &K8sDeployAction{
		gateway:      step.With["gateway"],
		namespace:    step.With["namespace"],
		containers:   step.With["containers"],
		image:        step.With["image"],
		projectName:  step.With["project_name"],
		servicePorts: step.With["service_ports"],
		ctx:          ctx,
		output:       output,
	}
}

func (k *K8sDeployAction) Pre() error {
	stack := k.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	k.namespace = utils.ReplaceWithParam(k.namespace, params)
	logger.Debugf("k8s namespace : %s", k.namespace)
	k.containers = utils.ReplaceWithParam(k.containers, params)
	logger.Debugf("k8s containers : %s", k.containers)
	k.projectName = utils.ReplaceWithParam(k.projectName, params)
	logger.Debugf("k8s deploy project name is : %s", k.projectName)
	k.image = utils.ReplaceWithParam(k.image, params)
	logger.Debugf("k8s deploy image is : %s", k.image)
	k.servicePorts = utils.ReplaceWithParam(k.servicePorts, params)
	logger.Debugf("k8s deploy service ports is : %s", k.servicePorts)
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return errors.New("get workdir error")
	}
	dockerBuildCmd := exec.Command("docker", "build", "-t", k.image, ".")
	dockerBuildCmd.Dir = workdir
	output, err := dockerBuildCmd.CombinedOutput()
	k.output.WriteLine(string(output))
	if err != nil {
		logger.Errorf("execute docker build command error: %s", err.Error())
		return err
	}
	dockerPushCmd := exec.Command("docker", "push", k.image)
	dockerPushCmd.Dir = workdir
	pushOut, err := dockerPushCmd.CombinedOutput()
	k.output.WriteLine(string(pushOut))
	if err != nil {
		logger.Errorf("execute docker push command error: %s", err.Error())
		return err
	}
	return nil
}

func (k *K8sDeployAction) Hook() (*model.ActionResult, error) {
	client, err := utils.InitK8sClient()
	if err != nil {
		logger.Errorf("init k8s client failed: %s", err.Error())
		return nil, err
	}
	err = utils.CreateNamespace(client, k.namespace)
	if err != nil {
		logger.Errorf("k8s create namespace failed: %s", err.Error())
		return nil, err
	}
	var containers []corev1.Container
	err = json.Unmarshal([]byte(k.containers), &containers)
	if err != nil {
		logger.Errorf("k8s containers format failed: %s", err.Error())
		return nil, err
	}
	containers[0].Image = k.image
	name := fmt.Sprintf("%s-%s", k.namespace, k.projectName)
	deploymentRes, err := utils.CreateDeployment(client, k.namespace, name, containers)
	if err != nil {
		logger.Errorf("k8s create deployment failed: %s", err.Error())
		return nil, err
	}
	logger.Debugf("k8s deployment replicas is :%d", deploymentRes.Status.Replicas)
	if deploymentRes.Status.Replicas == 0 {
		logger.Errorf("k8s container start failed,replicas is :%d", deploymentRes.Status.ReadyReplicas)
		return nil, errors.New("k8s container start failed")
	}
	var servicePorts []corev1.ServicePort
	err = json.Unmarshal([]byte(k.servicePorts), &servicePorts)
	if err != nil {
		logger.Errorf("k8s service ports format failed: %s", err.Error())
		return nil, err
	}
	err = utils.CreateService(client, k.namespace, name, servicePorts)
	if err != nil {
		logger.Errorf("k8s create service failed: %s", err.Error())
		return nil, err
	}
	actionResult := &model.ActionResult{}
	deployInfo := model.DeployInfo{
		Url: fmt.Sprintf("%s:%d", k.gateway, servicePorts[0].NodePort),
	}
	actionResult.Deploys = append(actionResult.Deploys, deployInfo)
	return actionResult, nil
}

func (k *K8sDeployAction) Post() error {
	return nil
}
