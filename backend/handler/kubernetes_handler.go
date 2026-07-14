package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zchimp/kubegraph/backend/pkg/response"
	"github.com/zchimp/kubegraph/backend/service"
)

// KubernetesHandler k8s相关业务处理器
type KubernetesHandler struct {
	kubeService *service.Kubernetes
}

type ListResourceQuery struct {
	Namespace string `form:"namespace"`
}

// NewKubernetesHandler 创建新的k8s相关业务处理器
func NewKubernetesHandler(kubeService *service.Kubernetes) *KubernetesHandler {
	return &KubernetesHandler{kubeService: kubeService}
}

func (h *KubernetesHandler) ListAllResources(c *gin.Context) {
	var req ListResourceQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		// 参数错误
		response.Fail(c, response.CodeParamErr, "参数解析失败", err)
		return
	}

	k8sSvc := h.kubeService
	if k8sSvc == nil {
		response.Fail(c, response.CodeServiceUnavailable, "K8s服务尚未初始化")
		return
	}

	list, err := k8sSvc.ListAllResources(req.Namespace)
	if err != nil {
		response.Fail(c, response.CodeNotFound, "资源类型不存在或未监听")
		return
	}

	// 成功返回，data为资源数组
	response.Success(c, list)
}
