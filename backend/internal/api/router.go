package api

import (
	"fishyinhe/backend/internal/api/handler"    // 确保模块路径正确
	"fishyinhe/backend/internal/api/middleware" // 确保模块路径正确
	"github.com/gin-gonic/gin"
)

// SetupRouter 创建、配置并返回 Gin 引擎实例
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
		apiV1.POST("/devices/:deviceId/gohome", handler.DeviceGoHomeHandler)
		apiV1.GET("/screen/:deviceId", handler.ScreenMirrorWS)

		// 文件相关路由组
		deviceFiles := apiV1.Group("/files")
		{
			// 确保这里的路由与您之前的设计一致
			deviceFiles.GET("/list/:deviceId", handler.ListFilesHandler)
			deviceFiles.GET("/download/:deviceId", handler.DownloadFileHandler)
			deviceFiles.POST("/upload/:deviceId", handler.UploadFileHandler)
		}

		// APK 相关路由组
		apkRoutes := apiV1.Group("/apk")
		{
			apkRoutes.POST("/install/:deviceId", handler.InstallLocalAPKHandler)
		}

		// 应用管理相关路由 (如果已添加)
		appRoutes := apiV1.Group("/apps")
		{
			appRoutes.GET("/list/:deviceId", handler.ListInstalledAppsHandler)
		}
	}
	return router // 返回创建并配置好的引擎
}
