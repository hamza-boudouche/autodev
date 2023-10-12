package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HealthcheckHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "message": "server is running",
        })
    }
}

