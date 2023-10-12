package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	lck "github.com/hamza-boudouche/autodev/pkg/helpers/locking"
	"github.com/hamza-boudouche/autodev/pkg/helpers/logging"
	ss "github.com/hamza-boudouche/autodev/pkg/sessions"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/kubernetes"
)

func SessionStatusHandler(cc *clientv3.Client, kcs *kubernetes.Clientset) gin.HandlerFunc {
    return func(c *gin.Context) {
		sessionName := strings.ReplaceAll(c.Param("sessionID"), "/", "")
        sessionID := fmt.Sprintf("session-%s", sessionName)

        logging.Logger.Info("trying to acquire lock", "session", sessionID)
        _, release, errLock := lck.AcquireLock(cc, sessionID)
        defer release()
        if errLock != nil {
            logging.Logger.Error("failed to acquire lock", "session", sessionID)
            c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to fetch container statuses of session %s", sessionName),
			})
			return
        }
        logging.Logger.Info("acquired lock successfully", "session", sessionID)

		containerStatuses, err := ss.ContainerStatus(c.Request.Context(),kcs, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("session %s container statuses fetched successfully", sessionName),
			"result":  containerStatuses,
		})
	}}

