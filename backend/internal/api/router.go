// D:\fishyinhe\backend\internal\api\router.go
package api

import (
	"fishyinhe/backend/internal/api/handler"    // 确保模块路径正确
	"fishyinhe/backend/internal/api/middleware" // 确保模块路径正确
	"github.com/gin-gonic/gin"
)

// SetupRouter 创建、配置并返回 Gin 引擎实例 (用于分离部署)
func SetupRouter() *gin.Engine { // 函数签名：无参数，返回 *gin.Engine
	router := gin.Default() // 在这里创建 Gin 引擎

	// 应用 CORS 中间件 (可以全局应用，也可以只对 API 组应用)
	router.Use(middleware.CORSMiddleware())

	// API V1 路由组
	apiV1 := router.Group("/api")
	// 如果上面全局应用了 CORS，这里就不需要再 Use 了
	// apiV1.Use(middleware.CORSMiddleware())
	{
		apiV1.GET("/health", handler.HealthCheck)
		apiV1.GET("/devices", handler.GetDevices)
		apiV1.GET("/screen/:deviceId", handler.ScreenMirrorWS)

		// 文件相关路由组 (保持之前的结构)
		deviceFiles := apiV1.Group("/files")
		{
			deviceFiles.GET("/list/:deviceId", handler.ListFilesHandler)
			deviceFiles.GET("/download/:deviceId", handler.DownloadFileHandler)
			deviceFiles.POST("/upload/:deviceId", handler.UploadFileHandler)
		}

		// APK 相关路由组 (保持之前的结构)
		apkRoutes := apiV1.Group("/apk")
		{
			apkRoutes.POST("/install/:deviceId", handler.InstallLocalAPKHandler)
		}
	}
	return router // 返回创建并配置好的引擎
}
