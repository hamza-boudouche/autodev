package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetK8sClient() (*kubernetes.Clientset, error) {
	_, inKubernetes := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	if inKubernetes {
		fmt.Println("Running inside a Kubernetes cluster.")
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		return clientset, nil
	} else {
		fmt.Println("Not running inside a Kubernetes cluster.")
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		return clientset, nil
	}
}

func CreatePV(cs *kubernetes.Clientset, name string, capacity string) error {
	pvPath := "/root"
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse(capacity),
			},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			AccessModes:                   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: pvPath,
					Type: new(v1.HostPathType),
				},
			},
		},
	}
	_, err := cs.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1.CreateOptions{})
	return err
}

func CreatePVC(cs *kubernetes.Clientset, name string, capacity string) error {
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse(capacity),
				},
			},
		},
	}
	_, err := cs.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), pvc, metav1.CreateOptions{})
	return err
}

func ExposeSession(cs *kubernetes.Clientset, sessionID string, components []Component, ingressName string) ([]Component, error) {
	// create all the port that need to be exposed
	ports := make([]v1.ServicePort, 0, len(components))
	for _, component := range components {
		if component.ExposeComponent {
			ports = append(ports, v1.ServicePort{
				Port:       int32(component.GetPublicPort()),
				TargetPort: intstr.FromInt(component.GetPublicPort()),
				Name:       component.ComponentID,
			})
		}
	}
	// creating the service
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: sessionID,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": sessionID,
			},
			Ports: ports,
			Type:  v1.ServiceTypeClusterIP,
		},
	}
	_, err := cs.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create service for session %s", sessionID))
	}

	// update ingress with a new entry
	ingress, err := cs.NetworkingV1().Ingresses("default").Get(context.TODO(), ingressName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get ingress %s", ingressName))
	}
	pathType := networkingv1.PathTypePrefix
	rules := make([]networkingv1.IngressRule, 0, len(ports))
	for i, component := range components {
		if component.ExposeComponent {
			url := fmt.Sprintf("%s.%s.hamzaboudouche.tech", sessionID, component.ComponentID)
			components[i].ComponentMetadata.Url = url
			rules = append(rules, networkingv1.IngressRule{
				Host: url,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: sessionID,
										Port: networkingv1.ServiceBackendPort{
											Number: int32(component.GetPublicPort()),
										},
									},
								},
							},
						},
					},
				},
			})
		}
	}

	ingress.Spec.Rules = append(ingress.Spec.Rules, rules...)
	_, err = cs.NetworkingV1().Ingresses("default").Update(context.TODO(), ingress, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update the ingress %s with a new rule for the session %s", ingressName, sessionID)
	}
	return components, nil
}

func InitSession(rc *redis.Client, kcs *kubernetes.Clientset, sessionID string) error {
	_, err := rc.Get(context.TODO(), sessionID).Result()
	if err == redis.Nil {
		// session has not been initialized before, proceed to initialize
		rc.Set(context.TODO(), sessionID, 1, 0)
		// if err := CreatePV(kcs, sessionID, "10Mi"); err != nil {
		// 	return err
		// }
		return CreatePVC(kcs, sessionID, "10Mi")
	} else if err != nil {
		// some error other than key not found occured, abort
		return err
	}
	// session was found, no need to initialize again
	return nil
}

type ComponentType string

const (
	Undefined ComponentType = "undefined"
	Code      ComponentType = "code"
	Redis     ComponentType = "redis"
	Mongo     ComponentType = "mongo"
)

type ComponentState string

const (
	Initializing ComponentState = "initializing"
	Ready        ComponentState = "ready"
	Terminated   ComponentState = "terminated"
)

type ComponentMetadata struct {
	Password string
	Url      string
}

type Component struct {
	ComponentType     ComponentType     `json:"componentType"`
	ExposeComponent   bool              `json:"exposeComponent"`
	ComponentID       string            `json:"componentID"`
	ComponentMetadata ComponentMetadata `json:"componentMetadata"`
}

func (c Component) GetPublicPort() int {
	switch c.ComponentType {
	case Code:
		return 8443
	case Redis:
		return 6379
	case Mongo:
		return 27017
	default:
		return 8080
	}
}

func (c Component) ToContainer(sessionID string) (*v1.Container, *v1.Volume, error) {
	if c.ComponentType == Code {
		return &v1.Container{
				Name:  c.ComponentID,
				Image: "linuxserver/code-server",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: int32(c.GetPublicPort()),
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "PUID",
						Value: "1000",
					},
					{
						Name:  "PGID",
						Value: "1000",
					},
					{
						Name:  "TZ",
						Value: "Etc/UTC",
					},
					{
						Name:  "PASSWORD",
						Value: c.ComponentMetadata.Password,
					},
					{
						Name:  "SUDO_PASSWORD",
						Value: "password",
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      sessionID,
						MountPath: "/config/workspace",
					},
				},
			}, &v1.Volume{
				Name: sessionID,
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: sessionID,
					},
				},
			}, nil
	} else if c.ComponentType == Redis {
		return &v1.Container{
				Name:  c.ComponentID,
				Image: "redis:latest",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: int32(c.GetPublicPort()),
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "redis-data",
						MountPath: "/data",
					},
				},
			}, &v1.Volume{
				Name: "redis-data",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "redis-data",
					},
				},
			}, nil
	} else if c.ComponentType == Mongo {
		return &v1.Container{
				Name:  c.ComponentID,
				Image: "mongo:latest",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: int32(c.GetPublicPort()),
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "mongodb-data",
						MountPath: "/data/db",
					},
				},
			}, &v1.Volume{
				Name: "mongodb-data",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mongodb-data",
					},
				},
			}, nil
	}
	return nil, nil, fmt.Errorf("unsupported component %s", c.ComponentType)
}

func ParseComponents(components []Component, sessionID string) ([]*v1.Container, []*v1.Volume, error) {
	containers := make([]*v1.Container, len(components))
	var volumes []*v1.Volume
	for i, component := range components {
		container, volume, err := component.ToContainer(sessionID)
		if err != nil {
			return nil, nil, err
		}
		containers[i] = container
		if volume != nil {
			volumes = append(volumes, volume)
		}
	}
	return containers, volumes, nil
}

type SessionState string

const (
	Initialized SessionState = "initialized"
	Running SessionState = "running"
	Stopped SessionState = "stopped"
)

type SessionInfo struct {
	SessionState SessionState `json:"sessionState"`
	Components   []Component  `json:"components"`
}

func CreateDeploy(cs *kubernetes.Clientset, rc *redis.Client, sessionID string, components []Component) error {
	sessionState, err := rc.Get(context.TODO(), sessionID).Result()
	if err != nil {
		return err
	}
	if sessionState != "1" {
		// session hasn't been just created
		return errors.New(fmt.Sprintf("session %s is already populated, delete and reinitialize first", sessionID))
	}
	var replicas *int32
	replicas = new(int32)
	*replicas = 1
	containers, volumes, err := ParseComponents(components, sessionID)
	if err != nil {
		return err
	}

	// create persistant volumes and persistant volume claims for each volume
	for _, volume := range volumes {
		if volume.Name == sessionID {
			continue
		}
		// err := CreatePV(cs, volume.Name, "20Mi")
		// if err != nil {
		// 	return err
		// }
		err = CreatePVC(cs, volume.Name, "20Mi")
		if err != nil {
			return err
		}
	}

	containerValues := make([]v1.Container, len(containers))
	volumeValues := make([]v1.Volume, len(volumes))

	for i, container := range containers {
		containerValues[i] = *container
	}

	for i, volume := range volumes {
		volumeValues[i] = *volume
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: sessionID,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": sessionID,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": sessionID,
					},
				},
				Spec: v1.PodSpec{
					Containers: containerValues,
					Volumes:    volumeValues,
				},
			},
		},
	}

	// Create the Deployment
	_, err = cs.AppsV1().Deployments("default").Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// expose the deployment
	components, err = ExposeSession(cs, sessionID, components, "minimal-ingress")
	if err != nil {
		return err
	}

	sessionJSON, _ := json.Marshal(SessionInfo{
		SessionState: Initialized,
		Components:   components,
	})
	_, err = rc.Set(
		context.TODO(),
		sessionID,
		string(sessionJSON),
		0).Result()
	return err
}

func ContainerStatus(cs *kubernetes.Clientset, sessionID string) (map[string]ComponentState, error) {
	_, err := cs.AppsV1().Deployments("default").Get(context.TODO(), sessionID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment for session: %s", sessionID)
	}
	pods, err := cs.CoreV1().Pods("default").List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", sessionID),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get pods for session: %s", sessionID)
	}

	if len(pods.Items) == 0 {
		return nil, nil
	}

	res := make(map[string]ComponentState, len(pods.Items[0].Status.ContainerStatuses))

	for _, containerStatus := range pods.Items[0].Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			// container is running
			res[containerStatus.Name] = Ready
		} else if containerStatus.State.Terminated != nil {
			// container is waiting
			res[containerStatus.Name] = Terminated
		} else {
			// container is not ready yet
			res[containerStatus.Name] = Initializing
		}
	}
	return res, nil
}

func RefreshDeploy(cs *kubernetes.Clientset, rc *redis.Client, sessionID string) (*SessionInfo, error) {
	// get stored SessionInfo
	sessionJSON, err := rc.Get(context.TODO(), sessionID).Result()
	if err != nil {
		return nil, err
	}
	var session SessionInfo
	err = json.Unmarshal([]byte(sessionJSON), &session)
	if err != nil {
		return nil, err
	}
	if session.SessionState == Initialized {
		deployment, err := cs.AppsV1().Deployments("default").Get(context.TODO(), sessionID, metav1.GetOptions{})
		if err != nil {
			// deployment not found
			// TODO: check the type of this error to see if it concerns anything another than the deployment not being found
			return nil, err
		}
		if deployment.Status.ReadyReplicas == 1 {
			// deployment is ready
			session.SessionState = Running
			sessionJSON, _ := json.Marshal(session)
			_, err = rc.Set(
				context.TODO(),
				sessionID,
				string(sessionJSON),
				0).Result()
			return &session, err
		} else {
			return &session, nil
		}
	} else if session.SessionState == Running {
		deployment, err := cs.AppsV1().Deployments("default").Get(context.TODO(), sessionID, metav1.GetOptions{})
		if err != nil {
			// deployment not found
			// TODO: check the type of this error to see if it concerns anything another than the deployment not being found
			return nil, err
		}
		if deployment.Status.ReadyReplicas == 1 {
			// deployment is still ready
			return &session, err
		} else {
			_, volumes, _ := ParseComponents(session.Components, sessionID)
			for _, volume := range volumes {
				_, err := cs.CoreV1().PersistentVolumeClaims("default").Get(context.TODO(), volume.Name, metav1.GetOptions{})
				if err != nil {
					// the pvc was not found
					// the session was deleted
					// delete from cache
					_, err = rc.Del(
						context.TODO(),
						sessionID).Result()
					return nil, err
				}
			}
			return &session, nil
		}
	} else if session.SessionState == Stopped {
		_, volumes, _ := ParseComponents(session.Components, sessionID)
		for _, volume := range volumes {
			_, err := cs.CoreV1().PersistentVolumeClaims("default").Get(context.TODO(), volume.Name, metav1.GetOptions{})
			if err != nil {
				// the pvc was not found
				// the session was deleted
				// delete from cache
				_, err = rc.Del(
					context.TODO(),
					sessionID).Result()
				return nil, err
			}
		}
		return &session, nil
	}
	return nil, nil
}

func GetSessionLogs(ctx context.Context, cs *kubernetes.Clientset, sessionID string, componentID string) (io.ReadCloser, error) {
	pods, err := cs.CoreV1().Pods("default").List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", sessionID),
		},
	)

	if err != nil || len(pods.Items) == 0 {
		// either pods have not been created yet or deployment doesn't exist
		return nil, fmt.Errorf("failed to fetch pods for session %s", sessionID)
	}

	for _, container := range pods.Items[0].Spec.Containers {
		if container.Name == componentID {
			podLogOptions := v1.PodLogOptions{
				Container: container.Name,
				Follow:    true,
			}
			podLogRequest := cs.CoreV1().
				Pods("default").
				GetLogs(pods.Items[0].Name, &podLogOptions)
			stream, err := podLogRequest.Stream(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get logs for container %s in session %s", container.Name, sessionID)
			}
			return stream, err
		}
	}

	return nil, fmt.Errorf("failed to find component %s", componentID)
}

func ToggleDeploy(cs *kubernetes.Clientset, rc *redis.Client, sessionID string) error {
	sessionJSON, err := rc.Get(context.TODO(), sessionID).Result()
	if err != nil {
		return err
	}
	var session SessionInfo
	err = json.Unmarshal([]byte(sessionJSON), &session)
	if err != nil {
		return err
	}

	if session.SessionState == Running {
		// toggle off
		err = cs.AppsV1().Deployments("default").Delete(context.TODO(), sessionID, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		session.SessionState = Stopped
		sessionJSON, _ := json.Marshal(session)
		_, err = rc.Set(
			context.TODO(),
			sessionID,
			string(sessionJSON),
			0).Result()
		return err
	} else if session.SessionState == Stopped {
		// toggle on
		var replicas *int32
		replicas = new(int32)
		*replicas = 1
		containers, volumes, err := ParseComponents(session.Components, sessionID)
		if err != nil {
			return err
		}

		containerValues := make([]v1.Container, len(containers))
		volumeValues := make([]v1.Volume, len(volumes))

		for i, container := range containers {
			containerValues[i] = *container
		}

		for i, volume := range volumes {
			volumeValues[i] = *volume
		}

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: sessionID,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": sessionID,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": sessionID,
						},
					},
					Spec: v1.PodSpec{
						Containers: containerValues,
						Volumes:    volumeValues,
					},
				},
			},
		}

		// Create the Deployment
		_, err = cs.AppsV1().Deployments("default").Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		session.SessionState = Running
		sessionJSON, _ := json.Marshal(session)
		_, err = rc.Set(
			context.TODO(),
			sessionID,
			string(sessionJSON),
			0).Result()
		return err
	} else {
		// session is neither Running nor Stopped
		// in this case it is still Initializing
		// session cannot be toggled ON or  OFF while Initializing
		return fmt.Errorf("session %s is still Initializing", sessionID)
	}
}

func DeleteDeploy(cs *kubernetes.Clientset, rc *redis.Client, sessionID string) error {
	cs.AppsV1().Deployments("default").Delete(context.TODO(), sessionID, metav1.DeleteOptions{})
	sessionJSON, err := rc.Get(context.TODO(), sessionID).Result()
	if err != nil {
		return err
	}
	var session SessionInfo
	err = json.Unmarshal([]byte(sessionJSON), &session)
	_, volumes, err := ParseComponents(session.Components, sessionID)
	if err != nil {
		return err
	}
	for _, volume := range volumes {
		err = cs.CoreV1().PersistentVolumeClaims("default").Delete(
			context.TODO(),
			volume.VolumeSource.PersistentVolumeClaim.ClaimName,
			metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	_ = cs.CoreV1().Services("default").Delete(context.TODO(), sessionID, metav1.DeleteOptions{})

	ingress, err := cs.NetworkingV1().Ingresses("default").Get(context.TODO(), "minimal-ingress", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the ingress %s", "minimal-ingress")
	}
	var filteredRules []networkingv1.IngressRule
	for _, rule := range ingress.Spec.Rules {
		if !strings.HasPrefix(rule.Host, fmt.Sprintf("%s.", sessionID)) {
			filteredRules = append(filteredRules, rule)
		}
	}
	ingress.Spec.Rules = filteredRules

	_, err = cs.NetworkingV1().Ingresses("default").Update(context.TODO(), ingress, metav1.UpdateOptions{})

	if err != nil {
		return fmt.Errorf("failed to update ingress %s", "minimal-ingress")
	}

	_, err = rc.Del(
		context.TODO(),
		sessionID).Result()
	return err
}
