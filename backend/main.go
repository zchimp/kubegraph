package main

import (
	"log"
	"net/http"
)

func main() {
	// 启动
	log.Printf("server start ...")

	// 监听8080，无路由，阻塞不退出
	_ = http.ListenAndServe(":8080", nil)
}
