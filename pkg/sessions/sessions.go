package sessions


import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
    cmp "github.com/hamza-boudouche/autodev/pkg/components"
    "github.com/hamza-boudouche/autodev/pkg/helpers/k8s"
)

type SessionState string

const (
	Initialized SessionState = "initialized"
	Running     SessionState = "running"
	Stopped     SessionState = "stopped"
)

type SessionInfo struct {
	SessionState SessionState `json:"sessionState"`
	Components   []cmp.Component  `json:"components"`
}

func InitSession(rc *redis.Client, kcs *kubernetes.Clientset, sessionID string) error {
	_, err := rc.Get(context.TODO(), sessionID).Result()
	if err == redis.Nil {
		// session has not been initialized before, proceed to initialize
		rc.Set(context.TODO(), sessionID, 1, 0)
		// if err := CreatePV(kcs, sessionID, "10Mi"); err != nil {
		// 	return err
		// }
		return k8s.CreatePVC(kcs, sessionID, "10Mi")
	} else if err != nil {
		// some error other than key not found occured, abort
		return err
	}
	// session was found, no need to initialize again
	return nil
}

func ExposeSession(cs *kubernetes.Clientset, sessionID string, components []cmp.Component, ingressName string) ([]cmp.Component, error) {
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

func CreateDeploy(cs *kubernetes.Clientset, rc *redis.Client, sessionID string, components []cmp.Component) error {
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
	containers, volumes, err := cmp.ParseComponents(components, sessionID)
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
		err = k8s.CreatePVC(cs, volume.Name, "20Mi")
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

func ContainerStatus(cs *kubernetes.Clientset, sessionID string) (map[string]cmp.ComponentState, error) {
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

	res := make(map[string]cmp.ComponentState, len(pods.Items[0].Status.ContainerStatuses))

	for _, containerStatus := range pods.Items[0].Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			// container is running
			res[containerStatus.Name] = cmp.Ready
		} else if containerStatus.State.Terminated != nil {
			// container is waiting
			res[containerStatus.Name] = cmp.Terminated
		} else {
			// container is not ready yet
			res[containerStatus.Name] = cmp.Initializing
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
			_, volumes, _ := cmp.ParseComponents(session.Components, sessionID)
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
		_, volumes, _ := cmp.ParseComponents(session.Components, sessionID)
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
		containers, volumes, err := cmp.ParseComponents(session.Components, sessionID)
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
	_, volumes, err := cmp.ParseComponents(session.Components, sessionID)
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
