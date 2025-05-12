package main

import (
	"fishyinhe/backend/internal/api" // 确保此导入路径与您的 go.mod 模块名一致
	"log"
)

func main() {
	log.Println("Starting backend server on port 5679...")

	// 调用 SetupRouter 并将其返回的 Gin 引擎赋值给 router 变量
	router := api.SetupRouter() // 正确的调用方式

	// 使用获取到的 router 启动 HTTP 服务
	err := router.Run(":5679")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
