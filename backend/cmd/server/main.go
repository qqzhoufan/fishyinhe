package main

import (
	"fishyinhe/backend/internal/api" // 确保这里的模块路径与 go.mod 一致
	"log"
)

func main() {
	log.Println("Starting backend server on port 5679...")

	router := api.SetupRouter()

	err := router.Run(":5679")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
