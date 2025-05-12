package handler

import (
	"fishyinhe/backend/internal/adb" // 确保模块路径正确
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// GetDevices 处理获取已连接设备列表的请求
func GetDevices(c *gin.Context) {
	devices, err := adb.ListConnectedDevices()
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve device list"})
		return
	}
	if devices == nil {
		devices = []adb.DeviceInfo{}
	}
	c.JSON(http.StatusOK, devices)
}

// DeviceGoHomeHandler 处理让设备返回主屏幕的请求
func DeviceGoHomeHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	log.Printf("DeviceGoHomeHandler: Received request for device %s to go home", deviceId)
	err := adb.GoToHomeScreen(deviceId) // 调用 adb 包中的 GoToHomeScreen 函数
	if err != nil {
		log.Printf("DeviceGoHomeHandler: Error sending home command for device %s: %v", deviceId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send home command to device", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Home command sent successfully to device " + deviceId})
}
