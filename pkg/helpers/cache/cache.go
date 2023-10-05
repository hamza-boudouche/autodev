package cache

import (
	"os"
	"time"

	"github.com/hamza-boudouche/autodev/pkg/helpers/logging"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func CreateEtcdClient() *clientv3.Client {
	logging.Logger.Info("constructing etcd client")
	if os.Getenv("AUTODEV_ENV") == "production" {
		logging.Logger.Error("production env etcd client connection not implemented yet")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logging.Logger.Error("failed to create etcd client ")
        panic(err)
	}
	logging.Logger.Info("etcd client constructed successfully")
    return cli
}
