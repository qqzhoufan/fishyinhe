// D:\fishyinhe\backend\internal\api\handler\apk_handler.go
package handler

import (
	"fishyinhe/backend/internal/adb" // 导入 adb 包
	"log"
	"net/http"
	// "net/url" // 如果从查询参数获取路径则需要

	"github.com/gin-gonic/gin"
)

// InstallAPKRequest 定义了安装 APK 请求的 JSON 结构体
type InstallAPKRequest struct {
	ApkPath string `json:"apkPath" binding:"required"` // APK 在设备上的完整路径
}

// InstallAPKHandler 处理在设备上安装 APK 的请求
func InstallAPKHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	var req InstallAPKRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("InstallAPKHandler: Error binding JSON for device %s: %v", deviceId, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	remoteApkPathOnDevice := req.ApkPath
	if remoteApkPathOnDevice == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "apkPath in request body is required"})
		return
	}

	log.Printf("Request to install APK. Device: %s, Remote APK Path: %s", deviceId, remoteApkPathOnDevice)

	output, err := adb.InstallAPK(deviceId, remoteApkPathOnDevice)
	if err != nil {
		// adb.InstallAPK 内部已经记录了详细错误
		// output 可能也包含了一些有用的信息
		log.Printf("Failed to install APK on device %s, path %s: %v", deviceId, remoteApkPathOnDevice, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Failed to install APK",
			"details":  output, // 将ADB的输出返回给前端
			"rawError": err.Error(),
		})
		return
	}

	log.Printf("APK installation process for device %s, path %s, resulted in output: %s", deviceId, remoteApkPathOnDevice, output)
	c.JSON(http.StatusOK, gin.H{
		"message": "APK installation command executed",
		"details": output, // 将ADB的输出返回给前端，前端可以检查是否包含 "Success"
	})
}
