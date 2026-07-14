package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zchimp/kubegraph/backend/config"
	"github.com/zchimp/kubegraph/backend/handler"
	"github.com/zchimp/kubegraph/backend/service"
)

// Server 服务器结构体
type Server struct {
	config *config.ServerConfig
	engine *gin.Engine

	// 健康检查服务
	healthSvc *service.HealthService
	healthHdl *handler.HealthHandler

	// k8s资源服务（延迟初始化，http接口触发构建）
	mu     sync.RWMutex
	k8sSvc *service.Kubernetes
	k8sHdl *handler.KubernetesHandler
}

// NewServer 创建新的服务器实例
func NewServer() (*Server, error) {
	cfg := config.NewServerConfig()
	engine := gin.Default()

	healthSvc := service.NewHealthService()
	healthHdl := handler.NewHealthHandler()
	k8sSvc, err := service.NewKubernetes("")
	if err != nil {
		log.Fatal("Init k8s service failed")
		return nil, err
	}
	k8sHdl := handler.NewKubernetesHandler(k8sSvc)

	return &Server{
		config:    cfg,
		engine:    engine,
		healthSvc: healthSvc,
		healthHdl: healthHdl,
		k8sSvc:    k8sSvc,
		k8sHdl:    k8sHdl,
	}, nil
}

// Init 初始化服务器
func (s *Server) Init() {
	// 注册路由
	s.registerRoutes()
}

// registerRoutes 注册路由
func (s *Server) registerRoutes() {
	// 健康检查路由
	s.engine.GET(HealthCheckPath, s.healthHdl.HealthCheck)
	s.engine.GET(ReadyCheckPath, s.healthHdl.ReadinessCheck)

	s.engine.Group(APIV1Root).
		GET(SubRoutePath.AllResources, func(c *gin.Context) {
			s.k8sHdl.ListAllResources(c)
		})
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
