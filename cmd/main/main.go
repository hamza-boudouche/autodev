package main

import (
	"github.com/gin-gonic/gin"
	"github.com/hamza-boudouche/autodev/pkg/helpers/cache"
	"github.com/hamza-boudouche/autodev/pkg/helpers/k8s"
	"github.com/hamza-boudouche/autodev/pkg/handlers"
)


func main() {
	kcs, err := k8s.GetK8sClient()
	if err != nil {
		panic(err)
	}

	cc := cache.CreateEtcdClient()

	r := gin.Default()

	r.GET("/healthcheck", handlers.HealthcheckHandler())

	r.POST("/init/:sessionID", handlers.InitSessionHandler(cc, kcs))

	r.POST("/create/:sessionID", handlers.CreateSessionHandler(cc, kcs))

	r.GET("/statuses/:sessionID", handlers.SessionStatusHandler(cc, kcs))

	r.GET("/logs/:sessionID/:componentID", handlers.ComponentLogsHandler(cc, kcs))

	r.POST("/refresh/:sessionID", handlers.RefreshSessionHandler(cc, kcs))

	r.PATCH("/toggle/:sessionID", handlers.ToggleSessionHandler(cc, kcs))

	r.DELETE("/:sessionID", handlers.DeleteSessionHandler(cc, kcs))

	r.Run()
}
