package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zchimp/kubegraph/backend/config"
	"github.com/zchimp/kubegraph/backend/handler"
	"github.com/zchimp/kubegraph/backend/service"
)

// Server 服务器结构体
type Server struct {
	config    *config.ServerConfig
	engine    *gin.Engine
	healthSvc *service.HealthService
	healthHdl *handler.HealthHandler
}

// NewServer 创建新的服务器实例
func NewServer() *Server {
	cfg := config.NewServerConfig()
	engine := gin.Default()

	healthSvc := service.NewHealthService()
	healthHdl := handler.NewHealthHandler()

	return &Server{
		config:    cfg,
		engine:    engine,
		healthSvc: healthSvc,
		healthHdl: healthHdl,
	}
}

// Init 初始化服务器
func (s *Server) Init() {
	// 注册路由
	s.registerRoutes()
}

// registerRoutes 注册路由
func (s *Server) registerRoutes() {
	// 健康检查路由
	s.engine.GET(s.config.HealthCheckPath, s.healthHdl.HealthCheck)
	s.engine.GET(s.config.ReadyCheckPath, s.healthHdl.ReadinessCheck)
}

// Start 启动服务器
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         s.config.Addr,
		Handler:      s.engine,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	log.Printf("server starting on %s", s.config.Addr)

	// 启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号进行优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server...")

	// 设置优雅关闭的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	log.Println("server exiting")
	return nil
}
