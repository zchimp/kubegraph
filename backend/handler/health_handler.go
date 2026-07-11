package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zchimp/kubegraph/backend/pkg"
)

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建新的健康检查处理器
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheck 处理健康检查请求
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	pkg.HealthCheckHandler(c)
}

// ReadinessCheck 处理就绪检查请求
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	pkg.ReadinessCheckHandler(c)
}
