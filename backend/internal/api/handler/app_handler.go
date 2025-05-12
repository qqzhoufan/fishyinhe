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

// UninstallAppRequest 定义了卸载应用请求的 JSON 结构体
// 这个结构体必须在这里定义，或者在同一个包的其他 .go 文件中定义并被正确导出（如果首字母大写）
type UninstallAppRequest struct {
	PackageName string `json:"packageName" binding:"required"`
	KeepData    bool   `json:"keepData"` // 可选，是否保留应用数据和缓存 (-k 选项)
}

// UninstallAppHandler 处理卸载设备上应用的请求
func UninstallAppHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	var req UninstallAppRequest // 使用上面定义的结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UninstallAppHandler: Error binding JSON for device %s: %v", deviceId, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	packageName := req.PackageName
	if packageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "packageName in request body is required"})
		return
	}

	adbOptions := ""
	if req.KeepData {
		adbOptions = "-k" // 保留数据和缓存目录
	}

	log.Printf("UninstallAppHandler: Request to uninstall package. Device: %s, Package: %s, KeepData: %t", deviceId, packageName, req.KeepData)

	output, err := adb.UninstallPackage(deviceId, packageName, adbOptions)
	if err != nil {
		log.Printf("UninstallAppHandler: Failed to uninstall package on device %s, package %s: %v", deviceId, packageName, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Failed to uninstall package",
			"details":  output,
			"rawError": err.Error(),
		})
		return
	}

	log.Printf("UninstallAppHandler: Uninstallation process for device %s, package %s, resulted in output: %s", deviceId, packageName, output)
	c.JSON(http.StatusOK, gin.H{
		"message":     "Uninstallation command executed",
		"details":     output,
		"packageName": packageName,
	})
}
