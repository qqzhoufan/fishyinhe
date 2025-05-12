package handler

import (
	"fishyinhe/backend/internal/adb" // 确保模块路径正确
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ClearLogcatHandler 处理清除设备 Logcat 缓存的请求
func ClearLogcatHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	log.Printf("ClearLogcatHandler: Received request for device %s", deviceId)
	err := adb.ClearLogcatBuffer(deviceId)
	if err != nil {
		log.Printf("ClearLogcatHandler: Error for device %s: %v", deviceId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear logcat buffer", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logcat buffer cleared successfully for device " + deviceId})
}

// DownloadLogcatHandler 处理下载设备 Logcat 文件的请求
func DownloadLogcatHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	log.Printf("DownloadLogcatHandler: Received request for device %s", deviceId)

	// 定义服务器上的临时存储目录
	tempStorageDir := filepath.Join(os.TempDir(), "adb_logcat_dumps")

	localTempLogFilePath, err := adb.DumpLogcatToFile(deviceId, tempStorageDir)
	if err != nil {
		log.Printf("DownloadLogcatHandler: Failed to dump logcat for device %s: %v", deviceId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logcat data", "details": err.Error()})
		return
	}

	// 确保文件在函数结束时被删除
	defer func() {
		log.Printf("DownloadLogcatHandler: Attempting to remove temporary log file: %s", localTempLogFilePath)
		if rmErr := os.Remove(localTempLogFilePath); rmErr != nil {
			log.Printf("DownloadLogcatHandler: Failed to remove temporary log file %s: %v", localTempLogFilePath, rmErr)
		} else {
			log.Printf("DownloadLogcatHandler: Successfully removed temporary log file: %s", localTempLogFilePath)
		}
	}()

	// 构造下载时的文件名
	downloadFileName := fmt.Sprintf("logcat_%s_%s.txt",
		strings.ReplaceAll(deviceId, ":", "_"),
		time.Now().Format("20060102_150405"))

	// 让浏览器下载文件
	c.FileAttachment(localTempLogFilePath, downloadFileName)
	log.Printf("DownloadLogcatHandler: Sent log file %s (as %s) to client for download.", localTempLogFilePath, downloadFileName)
}
