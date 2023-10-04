package k8s

import (
	"context"
	"os"
	"path/filepath"

    "github.com/hamza-boudouche/autodev/pkg/helpers/logging"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetK8sClient() (*kubernetes.Clientset, error) {
    logging.Logger.Info("constructing the k8s clientset")
	_, inKubernetes := os.LookupEnv("KUBERNETES_SERVICE_HOST")
    logging.Logger.Info("checking for env var", "KUBERNETES_SERVICE_HOST", inKubernetes)
	if inKubernetes {
		logging.Logger.Info("detected running inside a Kubernetes cluster.")
		config, err := rest.InClusterConfig()
		if err != nil {
		    logging.Logger.Error("failed to get in-cluster config")
			return nil, err
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
		    logging.Logger.Error("failed to construct clientset from config")
			return nil, err
		}
        logging.Logger.Info("k8s clientset created successfully")
		return clientset, nil
	} else {
		logging.Logger.Info("not running inside a Kubernetes cluster.")
        pathToConfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", pathToConfig)
		if err != nil {
		    logging.Logger.Error("failed to get config from file", "filePath", pathToConfig)
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
		    logging.Logger.Error("failed to construct clientset from config")
			return nil, err
		}
        logging.Logger.Info("k8s clientset created successfully")
		return clientset, nil
	}
}

func CreatePV(ctx context.Context, cs *kubernetes.Clientset, name string, capacity string) error {
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
	_, err := cs.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
	return err
}

func CreatePVC(ctx context.Context, cs *kubernetes.Clientset, name string, capacity string) error {
    logging.Logger.Info("creating PVC", "PVCName", name, "capacity", capacity)
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
	_, err := cs.CoreV1().PersistentVolumeClaims("default").Create(ctx, pvc, metav1.CreateOptions{})
    if err != nil {
        logging.Logger.Error("failed to create PVC", "PVCName", name, "capacity", capacity)
    }
	return err
}

