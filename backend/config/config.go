package config

import (
	"time"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr            string        `yaml:"addr"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	HealthCheckPath string        `yaml:"health_check_path"`
	ReadyCheckPath  string        `yaml:"ready_check_path"`
}

// DefaultServerConfig 默认服务器配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Addr:            ":8080",
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		HealthCheckPath: "/healthz",
		ReadyCheckPath:  "/ready",
	}
}

// NewServerConfig 创建新的服务器配置
func NewServerConfig() *ServerConfig {
	return DefaultServerConfig()
}
