package api

import (
	"fishyinhe/backend/internal/api/handler"    // 确保模块路径正确
	"fishyinhe/backend/internal/api/middleware" // 确保模块路径正确
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	router := gin.Default()

	// 应用CORS中间件
	router.Use(middleware.CORSMiddleware())

	// API V1 路由组
	apiV1 := router.Group("/api")
	{
		apiV1.GET("/health", handler.HealthCheck)
		apiV1.GET("/devices", handler.GetDevices)
		apiV1.GET("/screen/:deviceId", handler.ScreenMirrorWS)

		// 文件相关路由
		deviceFiles := apiV1.Group("/devices/:deviceId/files") // 使用路由组来组织
		{
			deviceFiles.GET("", handler.ListFilesHandler)             // <--- 新增这行，路径为 /api/devices/:deviceId/files
			deviceFiles.GET("/download", handler.DownloadFileHandler) // <--- 新增这行，路径为 /api/devices/:deviceId/files/download
			deviceFiles.POST("/upload", handler.UploadFileHandler)    // <--- 新增这行，用于文件上传
			// 未来可以添加 POST (上传), GET (下载特定文件), DELETE 等
		}

		//apkRoutes := apiV1.Group("/devices/:deviceId/apk")
		//{
		//	apkRoutes.POST("/install", handler.InstallAPKHandler) // <--- 新增这行
		//}
	}

	return router
}
