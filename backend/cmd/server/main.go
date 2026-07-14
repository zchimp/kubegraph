package main

import (
	"log"

	server "github.com/zchimp/kubegraph/backend/server"
)

func main() {
	// 创建服务器实例
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("New Server failed: %v", err)
	}

	// 初始化服务器
	srv.Init()

	// 启动服务器
	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
