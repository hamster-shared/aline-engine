package utils

import (
	"context"
	"fmt"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"log"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func InitK8sClient() (*kubernetes.Clientset, error) {
	var kubeConfig string
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeConfig = ""
	}
	_, err := os.Stat(kubeConfig)
	if err != nil {
		if os.IsNotExist(err) {
			kubeConfig = ""
		}
	}
	logger.Debugf("kube config path is: %s", kubeConfig)
	// use the current context in kubeConfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		log.Println("get kubectl config failed", err.Error())
		return nil, err
	}
	// create the clientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println("get k8s client failed: ", err.Error())
		return nil, err
	}
	return clientSet, nil
}

// CreateDeployment create deployment
func CreateDeployment(client *kubernetes.Clientset, username, deploymentName string, container []corev1.Container) (*appsv1.Deployment, error) {
	//get deployments by namespace
	var deploymentRes *appsv1.Deployment
	deploymentsClient := client.AppsV1().Deployments(username)
	list, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("get deployment list failed: ", err.Error())
		return deploymentRes, err
	}
	namespaceExist := false
	for _, deployment := range list.Items {
		if deployment.Name == deploymentName {
			namespaceExist = true
			break
		}
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: container,
				},
			},
		},
	}
	if !namespaceExist {
		deploymentRes, err = deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			log.Println("create deployment failed", err.Error())
			return deploymentRes, err
		}
	} else {
		deploymentRes, err = deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			log.Println("updata deployment failed", err.Error())
			return deploymentRes, err
		}
	}
	return deploymentRes, nil
}

func CreateNamespace(client *kubernetes.Clientset, username string) error {
	namespaceList, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("get namespace list failed: ", err.Error())
		return err
	}
	namespaceExist := false
	for _, ns := range namespaceList.Items {
		if ns.Name == username {
			namespaceExist = true
			break
		}
	}
	if !namespaceExist {
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: username,
			},
		}
		_, err = client.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
		if err != nil {
			log.Println("create namespace failed: ", err.Error())
			return err
		}
	}
	return nil
}

func CreateService(client *kubernetes.Clientset, username, serviceName string, ports []corev1.ServicePort) error {
	//get services by namespace
	serviceClient := client.CoreV1().Services(username)
	list, err := serviceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("get services list failed: ", err.Error())
		return err
	}
	serviceExist := false
	for _, service := range list.Items {
		if service.Name == serviceName {
			serviceExist = true
			break
		}
	}
	// Create the service spec
	serviceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": serviceName},
			Type:     corev1.ServiceTypeNodePort,
			Ports:    ports,
		},
	}
	if !serviceExist {
		_, err = serviceClient.Create(context.TODO(), serviceSpec, metav1.CreateOptions{})
		if err != nil {
			log.Println("create service failed: ", err.Error())
			return err
		}
	} else {
		_, err = serviceClient.Update(context.TODO(), serviceSpec, metav1.UpdateOptions{})
		if err != nil {
			log.Println("update service failed: ", err.Error())
			return err
		}
	}
	return nil
}

func CreateIngress(client *kubernetes.Clientset, namespace, serviceName, gateway string, ports []corev1.ServicePort) (*networkingv1beta1.Ingress, error) {
	var in *networkingv1beta1.Ingress
	pathType := networkingv1beta1.PathTypePrefix
	var tlsHost []string
	tlsHost = append(tlsHost, gateway)
	ingress := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ingress", serviceName),
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
			},
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s", serviceName, gateway),
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1beta1.IngressBackend{
										Service: &networkingv1beta1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1beta1.ServiceBackendPort{
												Number: ports[0].Port,
											},
										},
									},
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1beta1.IngressTLS{
				{
					Hosts:      tlsHost,
					SecretName: consts.SecretName,
				},
			},
		},
	}
	ingressClient := client.NetworkingV1().Ingresses(namespace)
	list, err := ingressClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("get ingress list failed: ", err.Error())
		return in, err
	}
	ingressExist := false
	for _, ingressItem := range list.Items {
		if ingressItem.Name == fmt.Sprintf("%s-ingress", serviceName) {
			ingressExist = true
			break
		}
	}
	if !ingressExist {
		in, err = client.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
		if err != nil {
			log.Println("create service failed: ", err.Error())
			return in, err
		}
	} else {
		in, err = client.NetworkingV1().Ingresses(namespace).Update(context.Background(), ingress, metav1.UpdateOptions{})
		if err != nil {
			log.Println("update service failed: ", err.Error())
			return in, err
		}
	}
	return in, err
}

func CreateHttpsIngress(client *kubernetes.Clientset, namespace, serviceName, gateway string, ports []corev1.ServicePort) (*networkingv1beta1.Ingress, error) {
	var in *networkingv1beta1.Ingress
	pathType := networkingv1beta1.PathTypePrefix
	var tlsHost []string
	tlsHost = append(tlsHost, fmt.Sprintf("%s.%s", serviceName, gateway))
	ingress := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ingress", serviceName),
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer":                 "letsencrypt-prod",
				"kubernetes.io/ingress.class":                    "nginx",
				"nginx.ingress.kubernetes.io/proxy-read-timeout": "3600",
				"nginx.ingress.kubernetes.io/proxy-send-timeout": "3600",
			},
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s", serviceName, gateway),
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1beta1.IngressBackend{
										Service: &networkingv1beta1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1beta1.ServiceBackendPort{
												Number: ports[0].Port,
											},
										},
									},
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1beta1.IngressTLS{
				{
					Hosts:      tlsHost,
					SecretName: consts.SecretName,
				},
			},
		},
	}
	ingressClient := client.NetworkingV1().Ingresses(namespace)
	list, err := ingressClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("get ingress list failed: ", err.Error())
		return in, err
	}
	ingressExist := false
	for _, ingressItem := range list.Items {
		if ingressItem.Name == fmt.Sprintf("%s-ingress", serviceName) {
			ingressExist = true
			break
		}
	}
	if !ingressExist {
		in, err = client.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
		if err != nil {
			log.Println("create service failed: ", err.Error())
			return in, err
		}
	} else {
		in, err = client.NetworkingV1().Ingresses(namespace).Update(context.Background(), ingress, metav1.UpdateOptions{})
		if err != nil {
			log.Println("update service failed: ", err.Error())
			return in, err
		}
	}
	return in, err
}

func int32Ptr(i int32) *int32 { return &i }

func GetPodLogs(containerName, deploymentName, namespace string) (*restclient.Request, error) {
	var req *restclient.Request
	client, err := InitK8sClient()
	if err != nil {
		log.Println("get k8s client failed", err.Error())
		return req, err
	}
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		log.Println("get pods failed", err.Error())
		return req, err
	}
	if len(pods.Items) > 0 {
		req = client.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
			Container: containerName,
			Follow:    true,
		})
	}
	return req, nil
}
