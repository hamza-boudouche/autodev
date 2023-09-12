package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func ExposeSession(cs *kubernetes.Clientset, sessionID string, ingressName string) (string, error) {
	// creating the service
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: sessionID,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": sessionID,
			},
			Ports: []v1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
	_, err := cs.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to create service for session %s", sessionID))
	}

    // update ingress with a new entry
	ingress, err := cs.NetworkingV1().Ingresses("default").Get(context.TODO(), ingressName, metav1.GetOptions{})
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to get ingress %s", ingressName))
	}
    pathType := networkingv1.PathTypePrefix
	newPath := networkingv1.HTTPIngressPath{
		Path:     fmt.Sprintf("/session/%s", sessionID), // Replace with the desired path
		PathType: &pathType,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: sessionID,
				Port: networkingv1.ServiceBackendPort{
					Number: 80,
				},
			},
		},
	}
    ingress.Spec.Rules[0].HTTP.Paths = append(ingress.Spec.Rules[0].HTTP.Paths, newPath)
    _, err = cs.NetworkingV1().Ingresses("default").Update(context.TODO(), ingress, metav1.UpdateOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to update the ingress %s with a new path for the session %s", ingressName, sessionID)
    }
    return fmt.Sprintf("www.hamzaboudouche.tech/session/%s", sessionID), nil
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

type ComponentType int64

const (
	Undefined ComponentType = iota
	Code
	Redis
	Mongo
)

type ComponentState int64

const (
	UndefinedComponent ComponentState = iota
	Initializing
	Ready
)

type ComponentMetadata struct {
	Password string
	Url      string
}

type Component struct {
	ComponentType     ComponentType     `json:"componentType"`
	ComponentID       string            `json:"componentID"`
	ComponentMetadata ComponentMetadata `json:"componentMetadata"`
	ComponentState    ComponentState    `json:"componentState"`
}

func (c Component) ToContainer(sessionID string) (*v1.Container, *v1.Volume, error) {
	if c.ComponentType == Code {
		return &v1.Container{
				Name:  fmt.Sprintf("%s-code-server", c.ComponentID),
				Image: "linuxserver/code-server",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: 8443,
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
				Name:  "redis",
				Image: "redis:latest",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: 6379,
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
				Name:  "mongodb",
				Image: "mongo:latest",
				Ports: []v1.ContainerPort{
					{
						ContainerPort: 27017,
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
	return nil, nil, fmt.Errorf("unsupported component %d", c.ComponentType)
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

type SessionState int64

const (
	Initialized SessionState = iota
	Running
	Stopped
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
    ideUrl, err := ExposeSession(cs, sessionID, "minimal-ingress")
    if err != nil {
        return err
    }
    components[0].ComponentMetadata.Url = ideUrl

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

    ingress, err := cs.NetworkingV1().Ingresses("default").Get(context.TODO(), "minimal-ingress", metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get the ingress %s", "minimal-ingress")
    }
    var filteredPaths []networkingv1.HTTPIngressPath
    for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
        if ! strings.HasSuffix(path.Path, sessionID) {
            filteredPaths = append(filteredPaths, path)
        }
    }
    ingress.Spec.Rules[0].HTTP.Paths = filteredPaths

    _, err = cs.NetworkingV1().Ingresses("default").Update(context.TODO(), ingress, metav1.UpdateOptions{})

    if err != nil {
        return fmt.Errorf("failed to update ingress %s", "minimal-ingress")
    }

	_, err = rc.Del(
		context.TODO(),
		sessionID).Result()
	return err
}
