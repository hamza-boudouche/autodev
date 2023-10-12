package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	cmp "github.com/hamza-boudouche/autodev/pkg/components"
	lck "github.com/hamza-boudouche/autodev/pkg/helpers/locking"
	"github.com/hamza-boudouche/autodev/pkg/helpers/logging"
	ss "github.com/hamza-boudouche/autodev/pkg/sessions"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/kubernetes"
)

type createEnv struct {
	Components []cmp.Component `json:"components"`
}

func CreateSessionHandler(cc *clientv3.Client, kcs *kubernetes.Clientset) gin.HandlerFunc {
    return func(c *gin.Context) {
		sessionName := strings.ReplaceAll(c.Param("sessionID"), "/", "")
        sessionID := fmt.Sprintf("session-%s", sessionName)

        logging.Logger.Info("trying to acquire lock", "session", sessionID)
        _, releaseSession, errSessionLock := lck.AcquireLock(cc, sessionID)
        defer releaseSession()
        if errSessionLock != nil {
            logging.Logger.Error("failed to acquire lock", "session", sessionID)
            c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to initialize session %s", sessionName),
			})
			return
        }
        logging.Logger.Info("acquired lock successfully", "session", sessionID)

        logging.Logger.Info("trying to acquire ingress lock", "session", sessionID)
        _, releaseIngress, errIngressLock := lck.AcquireLock(cc, "ingress")
        defer releaseIngress()
        if errIngressLock != nil {
            logging.Logger.Error("failed to acquire ingress lock", "session", sessionID)
            c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to initialize session %s", sessionName),
			})
			return
        }
        logging.Logger.Info("acquired ingress lock successfully", "session", sessionID)

		var body createEnv
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		err := ss.CreateDeploy(c.Request.Context(),kcs, cc, sessionID, body.Components)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to create components for session %s", sessionName),
			})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"message": fmt.Sprintf("components for session %s have been created successfully", sessionName),
		})
    }
}
