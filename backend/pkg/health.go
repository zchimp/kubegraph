package pkg

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status string `json:"status"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HealthCheckHandler 健康检查处理器
func HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ok",
	})
}

// ReadinessCheckHandler 就绪检查处理器
func ReadinessCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ready",
	})
}

// Success 响应成功
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Message: "success",
		Data:    data,
	})
}

// Error 响应错误
func Error(c *gin.Context, statusCode int, err error, message string) {
	c.JSON(statusCode, ErrorResponse{
		Error:   err.Error(),
		Message: message,
	})
}
