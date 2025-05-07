package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Backend is healthy and running!",
		"version": "1.0.0",
	})
}
