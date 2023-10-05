package main

import (
	"fmt"
	"time"

	"github.com/hamza-boudouche/autodev/pkg/helpers/cache"
	"github.com/hamza-boudouche/autodev/pkg/helpers/locking"
)

func main() {
	client := cache.CreateEtcdClient()

	defer client.Close()

	lock := "my-lock"

    _, release, err := locking.AcquireLock(client, lock)
    if err != nil {
        panic(err)
    }

	for i := 0; i < 10; i++ {
		fmt.Println("doing work ...")
		time.Sleep(time.Second)
	}

	err = release()
	if err != nil {
		panic(err)
	}
}

