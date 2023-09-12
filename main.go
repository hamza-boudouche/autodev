package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamza-boudouche/autodev/helpers"
)

type CreateEnv struct {
	Components []helpers.Component `json:"components"`
}

func main() {
	kcs, err := helpers.GetK8sClient()
	if err != nil {
		panic(err)
	}

	rc := helpers.CreateRedisClient()

	r := gin.Default()

	r.POST("/init/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		err := helpers.InitSession(rc, kcs, sessionID)
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
		err := helpers.CreateDeploy(kcs, rc, sessionID, body.Components)
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

	r.POST("/refresh/:sessionID", func(c *gin.Context) {
		sessionID := strings.ReplaceAll(c.Param("sessionID"), "/", "")
		sessionInfo, err := helpers.RefreshDeploy(kcs, rc, sessionID)
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
		err := helpers.ToggleDeploy(kcs, rc, sessionID)
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
		err := helpers.DeleteDeploy(kcs, rc, sessionID)
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
