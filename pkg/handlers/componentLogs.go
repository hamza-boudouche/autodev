package handlers

import (
	"fmt"
	"net/http"
	"strings"
    "io"
    "time"

	"github.com/gin-gonic/gin"
	ss "github.com/hamza-boudouche/autodev/pkg/sessions"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/client-go/kubernetes"
)

func ComponentLogsHandler(cc *clientv3.Client, kcs *kubernetes.Clientset) gin.HandlerFunc {
    return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		sessionName := strings.ReplaceAll(c.Param("sessionID"), "/", "")
        sessionID := fmt.Sprintf("session-%s", sessionName)
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
	}
}

