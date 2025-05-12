// D:\fishyinhe\backend\internal\api\handler\apk_handler.go
package handler

import (
	"fishyinhe/backend/internal/adb" // 确保模块路径正确
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// InstallLocalAPKHandler 处理从本地上传并直接安装 APK 的请求
func InstallLocalAPKHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供设备 ID (Device ID is required)"})
		return
	}

	file, header, err := c.Request.FormFile("apkFile")
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 从表单获取文件时出错: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "检索上传的 APK 文件时出错: " + err.Error()})
		return
	}
	defer file.Close()

	originalFilename := header.Filename
	log.Printf("InstallLocalAPKHandler: 收到直接安装 APK 文件的请求。设备: %s, 原始文件名: %s, 大小: %d",
		deviceId, originalFilename, header.Size)

	// 1. 将上传的 APK 保存到服务器的临时位置
	//    获取当前工作目录
	currentWorkDir, err := os.Getwd()
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 无法获取当前工作目录: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法确定应用工作路径"})
		return
	}

	// 在当前工作目录旁边创建一个名为 "temp_apk_storage" 的目录
	tempServerDir := filepath.Join(currentWorkDir, "temp_apk_storage_server") // 改个名字以区分之前的尝试
	log.Printf("InstallLocalAPKHandler: 将使用服务器临时目录: %s", tempServerDir)

	if err := os.MkdirAll(tempServerDir, 0755); err != nil {
		log.Printf("InstallLocalAPKHandler: 创建服务器临时目录 %s 失败: %v", tempServerDir, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法创建临时目录"})
		return
	}

	tempFileNameForServer := fmt.Sprintf("install_%d.apk", time.Now().UnixNano())
	localTempApkPath := filepath.Join(tempServerDir, tempFileNameForServer)

	log.Printf("InstallLocalAPKHandler: 将上传的 APK 保存到服务器临时路径: %s", localTempApkPath)

	tempFileOnServer, err := os.Create(localTempApkPath)
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 在服务器上创建临时文件 %s 失败: %v", localTempApkPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法创建临时文件"})
		return
	}

	_, err = io.Copy(tempFileOnServer, file)
	closeErr := tempFileOnServer.Close()
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 将上传的文件内容复制到 %s 失败: %v", localTempApkPath, err)
		os.Remove(localTempApkPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法保存上传的 APK"})
		return
	}
	if closeErr != nil {
		log.Printf("InstallLocalAPKHandler: 关闭服务器上的临时文件 %s 失败: %v", localTempApkPath, closeErr)
	}

	defer func() {
		log.Printf("InstallLocalAPKHandler: 尝试移除服务器临时文件: %s", localTempApkPath)
		if rmErr := os.Remove(localTempApkPath); rmErr != nil {
			log.Printf("InstallLocalAPKHandler: 移除服务器临时文件 %s 失败: %v", localTempApkPath, rmErr)
		} else {
			log.Printf("InstallLocalAPKHandler: 成功移除服务器临时文件: %s", localTempApkPath)
		}
	}()

	// 在调用 adb.InstallAPK 之前，再次确认文件确实存在于服务器的 localTempApkPath
	if _, statErr := os.Stat(localTempApkPath); os.IsNotExist(statErr) {
		log.Printf("InstallLocalAPKHandler: 严重错误 - 临时文件 %s 在调用 InstallAPK 前不存在!", localTempApkPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误：临时文件丢失"})
		return
	}
	log.Printf("InstallLocalAPKHandler: 确认临时文件 %s 存在，准备调用 adb.InstallAPK", localTempApkPath)

	output, err := adb.InstallAPK(deviceId, localTempApkPath)
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 在设备 %s 上从服务器路径 %s (原始文件名: %s) 安装 APK 失败: %v", deviceId, localTempApkPath, originalFilename, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "安装 APK 失败 (Failed to install APK)",
			"details":  output,
			"rawError": err.Error(),
		})
		return
	}

	log.Printf("InstallLocalAPKHandler: 设备 %s 上的 APK 安装过程（原始文件 %s）产生的输出: %s", deviceId, originalFilename, output)
	c.JSON(http.StatusOK, gin.H{
		"message":  "APK 安装命令已执行 (APK installation command executed)",
		"details":  output,
		"filename": originalFilename,
	})
}
