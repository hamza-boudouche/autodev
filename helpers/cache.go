package helpers

import (
	"os"
	"github.com/redis/go-redis/v9"
)

func CreateRedisClient() *redis.Client {
    if os.Getenv("AUTODEV_ENV") == "production" {
        return redis.NewClient(&redis.Options{
            Addr: os.Getenv("AUTODEV_REDIS_ADDR"),
            Password: os.Getenv("AUTODEV_REDIS_PASSWORD"),
            DB: 0,
        })
    }
    return redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
        Password: "",
        DB: 0,
    })
}

