// D:\fishyinhe\backend\internal\api\handler\file_handler.go
package handler

import (
	"fishyinhe/backend/internal/adb" // 导入 adb 包
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ListFilesHandler 处理列出设备文件的请求
func ListFilesHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	remotePath := c.Query("path") // 从查询参数获取路径，例如 /api/devices/emulator-5554/files?path=/sdcard/

	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}
	if remotePath == "" {
		remotePath = "/sdcard/" // 默认路径，安卓的 SD 卡通常挂载点
	}
	// 简单的路径规范化，防止一些基本问题
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}
	if !strings.HasSuffix(remotePath, "/") && remotePath != "/" { // 确保路径以 / 结尾，除非是根目录
		remotePath += "/"
	}

	log.Printf("Listing files for device: %s, path: %s", deviceId, remotePath)

	files, err := adb.ListFiles(deviceId, remotePath)
	if err != nil {
		// 检查错误是否表示路径不存在
		// stderr 的具体内容可能因 ADB 版本和设备而异
		errMsg := "Failed to list files"
		if exitErr, ok := err.(*exec.ExitError); ok { // exec.ExitError 没有直接导出，所以类型断言可能失败
			stderrStr := string(exitErr.Stderr)
			if strings.Contains(stderrStr, "No such file or directory") || strings.Contains(stderrStr, "Not a directory") {
				errMsg = "Path not found or is not a directory"
				c.JSON(http.StatusNotFound, gin.H{"error": errMsg, "path": remotePath})
				return
			}
		}
		log.Printf("Error in adb.ListFiles for device %s, path %s: %v", deviceId, remotePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg, "details": err.Error()})
		return
	}

	if files == nil {
		files = []adb.AdbFileItem{} // 确保返回空数组而不是 null
	}

	c.JSON(http.StatusOK, gin.H{
		"path":  remotePath,
		"files": files,
	})
}

// DownloadFileHandler 处理下载设备文件的请求
func DownloadFileHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	// remoteFilePath 通常包含特殊字符，需要进行 URL 解码
	remoteFilePathQuery := c.Query("filePath") // 例如 /sdcard/Download/my file.txt

	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}
	if remoteFilePathQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path is required"})
		return
	}

	// 对查询参数进行 URL 解码
	remoteFilePath, err := url.QueryUnescape(remoteFilePathQuery)
	if err != nil {
		log.Printf("Error unescaping file path '%s': %v", remoteFilePathQuery, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path encoding"})
		return
	}

	log.Printf("Request to download file. Device: %s, Remote Path: %s", deviceId, remoteFilePath)

	// 定义服务器上的临时存储目录，你可以根据你的服务器环境修改
	// 确保这个目录是可写的
	tempDir := filepath.Join(os.TempDir(), "adb_pulled_files") // 例如 C:\Users\YourUser\AppData\Local\Temp\adb_pulled_files

	localFilePath, err := adb.PullFile(deviceId, remoteFilePath, tempDir)
	if err != nil {
		log.Printf("Failed to pull file for download. Device: %s, Path: %s, Error: %v", deviceId, remoteFilePath, err)
		// 这里可以根据 adb.PullFile 返回的错误类型来决定 HTTP 状态码
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "No such file or directory") {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found on device", "path": remoteFilePath})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file from device"})
		}
		return
	}

	// 确保文件在函数结束时被删除
	defer func() {
		log.Printf("Attempting to remove temporary file: %s", localFilePath)
		err := os.Remove(localFilePath)
		if err != nil {
			log.Printf("Failed to remove temporary file %s: %v", localFilePath, err)
		} else {
			log.Printf("Successfully removed temporary file: %s", localFilePath)
		}
	}()

	// 让浏览器下载文件，而不是直接显示
	// 第一个参数是本地文件路径，第二个参数是希望在浏览器中显示的文件名
	originalFileName := filepath.Base(remoteFilePath)
	c.FileAttachment(localFilePath, originalFileName)
	// c.File(localFilePath) // 如果只是想让浏览器尝试打开（例如图片），而不是强制下载

	log.Printf("Sent file %s (original: %s) to client for download.", localFilePath, originalFileName)
}

func UploadFileHandler(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	// 从表单数据中获取目标远程目录路径
	// 前端应确保 remoteDirPath 是一个目录路径，并且以 '/' 结尾 (除非是根目录)
	remoteDirPath := c.PostForm("remoteDirPath")
	if remoteDirPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Remote directory path (remoteDirPath) is required in form data"})
		return
	}
	// 简单的规范化，确保以斜杠结尾
	if !strings.HasSuffix(remoteDirPath, "/") {
		remoteDirPath += "/"
	}

	// 从表单数据中获取上传的文件
	file, header, err := c.Request.FormFile("file") // "file" 是前端 <input type="file" name="file"> 的 name
	if err != nil {
		log.Printf("Error getting file from form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving uploaded file: " + err.Error()})
		return
	}
	defer file.Close()

	originalFilename := header.Filename
	log.Printf("Received file upload request. Device: %s, Target Dir: %s, Filename: %s, Size: %d",
		deviceId, remoteDirPath, originalFilename, header.Size)

	// 1. 将上传的文件保存到服务器的临时位置
	//    确保这个临时目录是可写的，并且会定期清理 (或在此处确保删除)
	tempServerDir := filepath.Join(os.TempDir(), "adb_uploads_temp")
	if err := os.MkdirAll(tempServerDir, 0755); err != nil {
		log.Printf("Failed to create server temp directory %s: %v", tempServerDir, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error: could not create temp directory"})
		return
	}
	localTempFilePath := filepath.Join(tempServerDir, originalFilename)

	// 创建临时文件用于写入
	tempFile, err := os.Create(localTempFilePath)
	if err != nil {
		log.Printf("Failed to create temp file %s on server: %v", localTempFilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error: could not create temp file"})
		return
	}
	// 将上传文件的内容复制到临时文件
	_, err = io.Copy(tempFile, file) // io 包需要导入: "io"
	tempFile.Close()                 // 确保关闭文件，即使复制失败
	if err != nil {
		log.Printf("Failed to copy uploaded file content to %s: %v", localTempFilePath, err)
		os.Remove(localTempFilePath) // 尝试删除部分写入的文件
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error: could not save uploaded file"})
		return
	}

	// 函数结束时删除服务器上的临时文件
	defer func() {
		log.Printf("Attempting to remove server temporary file: %s", localTempFilePath)
		errRemove := os.Remove(localTempFilePath)
		if errRemove != nil {
			log.Printf("Failed to remove server temporary file %s: %v", localTempFilePath, errRemove)
		} else {
			log.Printf("Successfully removed server temporary file: %s", localTempFilePath)
		}
	}()

	// 2. 将服务器上的临时文件推送到设备的指定远程路径
	//    远程路径是目录 +原始文件名
	fullRemoteDevicePath := remoteDirPath + originalFilename // 假设 remoteDirPath 以 '/' 结尾

	err = adb.PushFile(deviceId, localTempFilePath, fullRemoteDevicePath)
	if err != nil {
		log.Printf("Failed to push file to device. Device: %s, Local: %s, Remote: %s, Error: %v",
			deviceId, localTempFilePath, fullRemoteDevicePath, err)
		// 具体的错误信息已在 adb.PushFile 中记录，这里可以返回一个通用错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to push file to device", "details": err.Error()})
		return
	}

	log.Printf("File %s uploaded successfully to device %s at %s", originalFilename, deviceId, fullRemoteDevicePath)
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully to device",
		"filePath": fullRemoteDevicePath,
		"filename": originalFilename,
	})
}
