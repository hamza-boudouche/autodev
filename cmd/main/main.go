package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamza-boudouche/autodev/pkg/helpers/cache"
	"github.com/hamza-boudouche/autodev/pkg/helpers/k8s"
    cmp "github.com/hamza-boudouche/autodev/pkg/components"
    ss "github.com/hamza-boudouche/autodev/pkg/sessions"
)

type CreateEnv struct {
	Components []cmp.Component `json:"components"`
}

func main() {
	kcs, err := k8s.GetK8sClient()
	if err != nil {
		panic(err)
	}

	cc := cache.CreateEtcdClient()

	r := gin.Default()

	r.GET("/healthcheck", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
            "message": "server is running",
		})
	})

	r.POST("/init/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		err := ss.InitSession(c.Request.Context(),cc, kcs, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to initialize session %s", sessionID),
			})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"message": fmt.Sprintf("session %s created successfully", sessionID),
		})
	})

	r.POST("/create/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		var body CreateEnv
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		err := ss.CreateDeploy(c.Request.Context(),kcs, cc, sessionID, body.Components)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to create components for session %s", sessionID),
			})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"message": fmt.Sprintf("components for session %s have been created successfully", sessionID),
		})
	})

	r.GET("/statuses/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		containerStatuses, err := ss.ContainerStatus(c.Request.Context(),kcs, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("session %s container statuses fetched successfully", sessionID),
			"result":  containerStatuses,
		})

	})

	r.GET("/logs/:sessionID/:componentID", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		componentID := strings.ReplaceAll(c.Param("componentID"), "/", "")

		logStream, err := ss.GetSessionLogs(c.Request.Context(), kcs, sessionID, componentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		defer logStream.Close()

		c.Stream(func(w io.Writer) bool {
			buf := make([]byte, 2000)
			numBytes, err := logStream.Read(buf)
			// fmt.Println("read", numBytes, "from", containerName)
			if numBytes == 0 {
				return true
			}
			if err != nil {
				return true
			}
			c.SSEvent("logs", string(buf[:numBytes]))
			time.Sleep(time.Second)
			return true
		})
	})

	r.POST("/refresh/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		sessionInfo, err := ss.RefreshDeploy(c.Request.Context(),kcs, cc, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to refresh session %s", sessionID),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("session %s refreshed successfully", sessionID),
			"result":  sessionInfo,
		})
	})

	r.PATCH("/toggle/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		err := ss.ToggleDeploy(c.Request.Context(),kcs, cc, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to toggle session %s", sessionID),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("session %s toggled successfully", sessionID),
		})
	})

	r.DELETE("/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		err := ss.DeleteDeploy(c.Request.Context(),kcs, cc, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to delete session %s", sessionID),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("session %s deleted successfully", sessionID),
		})
	})

	r.Run()
}
