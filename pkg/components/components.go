package components

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

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

