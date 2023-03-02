package utils

import (
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"log"
	"os/exec"
	"strings"
	"testing"
)

func Test_k8s(t *testing.T) {
	client, err := InitK8sClient()
	if err != nil {
		log.Println(err.Error())
		return
	}
	err = CreateNamespace(client, "jian-guo-s")
	if err != nil {
		log.Println("create namespace failed ------", err.Error())
		return
	}
	var containers []corev1.Container
	var envs []corev1.EnvVar
	env1 := corev1.EnvVar{
		Name:  "DB_HOST",
		Value: "mysql",
	}
	env2 := corev1.EnvVar{
		Name: "DB_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "jian-guo-s-test",
				},
				Key: "db_password",
			},
		},
	}
	envs = append(envs, env1, env2)
	var ports []corev1.ContainerPort
	port := corev1.ContainerPort{
		ContainerPort: 8080,
	}
	ports = append(ports, port)
	container1 := corev1.Container{
		Name:  "jian-guo-s-test",
		Image: "hamstershare/hamster-develop:8",
		Env:   envs,
		Ports: ports,
	}
	containers = append(containers, container1)
	err = CreateDeployment(client, "jian-guo-s", "jian-guo-s-test", containers)
	if err != nil {
		log.Println("create deployment failed +++++++", err.Error())
		return
	}
	var servicePorts []corev1.ServicePort
	servicePort := corev1.ServicePort{
		Protocol:   corev1.ProtocolTCP,
		Port:       8081,
		TargetPort: intstr.FromInt(8080),
		NodePort:   30317,
	}
	servicePorts = append(servicePorts, servicePort)
	err = CreateService(client, "jian-guo-s", "jian-guo-s-test", servicePorts)
	if err != nil {
		log.Println("create service failed: ", err.Error())
		return
	}
}

type Container struct {
	Name  string          `json:"name"`
	Image string          `json:"image"`
	Ports []ContainerPort `json:"ports"`
}

type ContainerPort struct {
	ContainerPort int32 `json:"containerPort"`
}

type ServicePort struct {
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	NodePort   int32  `json:"nodePort"`
}

func Test_Data(t *testing.T) {
	var servicePorts []corev1.ServicePort
	var serPorts []ServicePort
	ser := ServicePort{
		Protocol:   "TCP",
		Port:       8081,
		TargetPort: 8080,
		NodePort:   30317,
	}
	serPorts = append(serPorts, ser)
	jsonString, err := json.Marshal(serPorts)
	if err != nil {
		log.Println("--------------")
		log.Println(err.Error())
		log.Println("---------------")
		return
	}
	err = json.Unmarshal(jsonString, &servicePorts)
	if err != nil {
		log.Println("++++++++++++")
		log.Println(err.Error())
		log.Println("++++++++++++")
	}
	log.Println(servicePorts[0].Port)
	log.Println(servicePorts[0].Protocol)
	log.Println(servicePorts[0].NodePort)
	log.Println(servicePorts[0].TargetPort.String())
}

func Test_docker(t *testing.T) {
	cmd := exec.Command("docker", "search", "hamstershare/dfafafafasf")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("sfsfsfsfsfsfsfs")
		log.Println(err.Error())
		log.Println("-----------")
	}
	res := string(out)
	dd := strings.Fields(res)
	log.Println("------------")
	log.Println(len(dd))
	log.Println("----------")
	log.Println(string(out))
}
