package server

// APIPath 统一存放所有接口路径常量
const (
	HealthCheckPath = "/healthz"
	ReadyCheckPath  = "/ready"
	APIV1Root       = "/api/v1"
)

// 子路由
var SubRoutePath = struct {
	AllResources string
}{
	AllResources: "/all-resources",
}
