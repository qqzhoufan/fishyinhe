// D:\fishyinhe\backend\internal\api\handler\device_handler.go
package handler

import (
	"fishyinhe/backend/internal/adb" // 导入我们上面创建的 adb 包
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// GetDevices 处理获取设备列表的请求
func GetDevices(c *gin.Context) {
	devices, err := adb.ListConnectedDevices()
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		// 在实际应用中，你可能想根据错误类型返回更具体的 HTTP 状态码
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve device list"})
		return
	}

	if devices == nil { // 确保在没有设备时返回一个空数组而不是 null
		devices = []adb.DeviceInfo{}
	}

	c.JSON(http.StatusOK, devices)
}
