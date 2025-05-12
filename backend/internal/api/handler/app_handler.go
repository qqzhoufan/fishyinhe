package handler

import (
	"fishyinhe/backend/internal/adb" // 确保模块路径正确
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// ListInstalledAppsHandler 处理列出设备上已安装应用的请求
func ListInstalledAppsHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	// 从查询参数获取选项，例如 ?filter=third_party
	filterOption := c.Query("filter")
	adbOptions := ""
	if filterOption == "third_party" {
		adbOptions = "-3" // 只列出第三方应用
	}
	// 未来可以添加其他过滤选项，如 "system" (-s) 等

	log.Printf("ListInstalledAppsHandler: Received request for device %s, filter: %s (adb options: '%s')", deviceId, filterOption, adbOptions)
	packages, err := adb.ListInstalledPackages(deviceId, adbOptions)
	if err != nil {
		log.Printf("ListInstalledAppsHandler: Error listing packages for device %s: %v", deviceId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list installed applications", "details": err.Error()})
		return
	}

	if packages == nil {
		packages = []string{} // 确保返回空数组而不是 null
	}

	c.JSON(http.StatusOK, gin.H{"packages": packages})
}

// 如果未来要添加卸载功能，可以在这里添加 UninstallAppHandler 等
