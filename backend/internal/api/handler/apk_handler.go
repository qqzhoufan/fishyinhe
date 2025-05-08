// D:\fishyinhe\backend\internal\api\handler\apk_handler.go
package handler

import (
	"fishyinhe/backend/internal/adb" // 确保模块路径正确
	"io"                             // 导入 io 包
	"log"
	"net/http"
	"os"            // 导入 os 包
	"path/filepath" // 导入 filepath 包
	// "net/url" // 这个 Handler 不需要
	// "strings" // 这个 Handler 不需要
	// "os/exec" // 这个 Handler 不需要
	"github.com/gin-gonic/gin"
)

// InstallLocalAPKHandler 处理从本地上传并直接安装 APK 的请求
func InstallLocalAPKHandler(c *gin.Context) {
	// 从 URL 路径参数中获取设备 ID
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供设备 ID (Device ID is required)"})
		return
	}

	// 从 multipart/form-data 中获取名为 "apkFile" 的文件
	file, header, err := c.Request.FormFile("apkFile")
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 从表单获取文件时出错: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "检索上传的 APK 文件时出错: " + err.Error()})
		return
	}
	// 确保在函数结束时关闭上传的文件句柄
	defer file.Close()

	originalFilename := header.Filename
	log.Printf("InstallLocalAPKHandler: 收到直接安装 APK 文件的请求。设备: %s, 文件名: %s, 大小: %d",
		deviceId, originalFilename, header.Size)

	// 1. 将上传的 APK 保存到服务器的临时位置
	//    确保这个临时目录是可写的，并且会定期清理 (或在此处确保删除)
	tempServerDir := filepath.Join(os.TempDir(), "adb_install_temp") // 例如 C:\Users\YourUser\AppData\Local\Temp\adb_install_temp
	if err := os.MkdirAll(tempServerDir, 0755); err != nil {
		log.Printf("InstallLocalAPKHandler: 创建服务器临时目录 %s 失败: %v", tempServerDir, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法创建临时目录"})
		return
	}
	// 使用原始文件名（或生成唯一文件名以避免冲突）
	localTempApkPath := filepath.Join(tempServerDir, originalFilename)

	// 在服务器上创建临时文件用于写入
	tempFileOnServer, err := os.Create(localTempApkPath)
	if err != nil {
		log.Printf("InstallLocalAPKHandler: 在服务器上创建临时文件 %s 失败: %v", localTempApkPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法创建临时文件"})
		return
	}

	// 将上传文件的内容复制到服务器上的临时文件
	_, err = io.Copy(tempFileOnServer, file)
	// 必须在 Copy 后立即关闭文件，否则后续 adb install 可能无法读取
	closeErr := tempFileOnServer.Close()
	if err != nil { // 检查 io.Copy 的错误
		log.Printf("InstallLocalAPKHandler: 将上传的文件内容复制到 %s 失败: %v", localTempApkPath, err)
		os.Remove(localTempApkPath) // 尝试删除部分写入的文件
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误：无法保存上传的 APK 文件"})
		return
	}
	if closeErr != nil { // 检查文件关闭的错误
		log.Printf("InstallLocalAPKHandler: 关闭服务器上的临时文件 %s 失败: %v", localTempApkPath, closeErr)
		// 即使关闭失败，文件内容可能已写入，继续尝试安装，但记录日志
	}

	// 确保在函数结束时删除服务器上的临时 APK 文件
	defer func() {
		log.Printf("InstallLocalAPKHandler: 尝试移除服务器临时文件: %s", localTempApkPath)
		if rmErr := os.Remove(localTempApkPath); rmErr != nil {
			log.Printf("InstallLocalAPKHandler: 移除服务器临时文件 %s 失败: %v", localTempApkPath, rmErr)
		} else {
			log.Printf("InstallLocalAPKHandler: 成功移除服务器临时文件: %s", localTempApkPath)
		}
	}()

	// 2. 调用 adb.InstallAPK，传入服务器上的临时文件路径
	//    adb.InstallAPK 内部会执行 adb install <localTempApkPath>
	output, err := adb.InstallAPK(deviceId, localTempApkPath)
	if err != nil {
		// adb.InstallAPK 内部已经记录了详细错误
		log.Printf("InstallLocalAPKHandler: 在设备 %s 上从路径 %s 安装 APK 失败: %v", deviceId, localTempApkPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "安装 APK 失败 (Failed to install APK)",
			"details":  output,      // 将 ADB 的输出返回给前端
			"rawError": err.Error(), // 可以包含更底层的错误信息
		})
		return
	}

	log.Printf("InstallLocalAPKHandler: 设备 %s 上的 APK 安装过程（文件 %s）产生的输出: %s", deviceId, originalFilename, output)
	c.JSON(http.StatusOK, gin.H{
		"message":  "APK 安装命令已执行 (APK installation command executed)",
		"details":  output, // 将 ADB 的输出返回给前端
		"filename": originalFilename,
	})
}

// 注意：这里没有第二个 InstallLocalAPKHandler 函数的定义了。
