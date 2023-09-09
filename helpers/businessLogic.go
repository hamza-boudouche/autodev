package helpers

import (
	"context"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
)

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

func CreateDevEnv(rc *redis.Client, sessionID string) error {
	return nil
}

func GetDevEnv(rc *redis.Client, sessionID string) error {
	return nil
}

func DeleteDevEnv(sessionID string) error {
	return nil
}
