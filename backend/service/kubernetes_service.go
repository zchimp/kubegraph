package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zchimp/kubegraph/backend/internal/queue"
	coreV1 "k8s.io/api/core/v1"
	apimacherrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	// 实时调用 APIServer
	dynamicClient dynamic.Interface
	clientSet     kubernetes.Interface

	// 本地内存缓存
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	watchGVRList    []schema.GroupVersionResource

	// 内部事件队列
	eventQueue *queue.RingDropOldQueue[KubeEvent]

	ClusterHost string

	// 展示字段，目前无用
	Mode              string
	KubeVersion       string
	UseEndpointSlices bool
}
type KubeEvent struct {
	// 事件类型
	EventType EventTypeEnum
	// Object 触发事件的Kubernetes资源对象，使用 unstructured.Unstructured 兼容任意原生资源与CRD，无需预先定义结构体
	Object *unstructured.Unstructured
}

type EventTypeEnum string

const (
	AddEvent    EventTypeEnum = "add"
	UpdateEvent EventTypeEnum = "update"
	DeleteEvent EventTypeEnum = "delete"
	PingEvent   EventTypeEnum = "ping"
	// 最早丢弃队列数量，事件满，会自动删除最老的
	defaultEventQueueCap = 1000
)

func NewKubernetes(singleNamespace string) (*Kubernetes, error) {
	var kubeConfig *rest.Config

	var err error

	eventQueue := queue.NewRingDropOldQueue[KubeEvent](defaultEventQueueCap)

	mode := "out-of-cluster"

	// 优先尝试集群内配置
	kubeConfig, err = rest.InClusterConfig()
	if err == nil {
		mode = "in-cluster"
		log.Println("[K8S-SERVICE-INIT] Running in cluster, use serviceaccount token")
	} else if errors.Is(err, rest.ErrNotInCluster) {
		// 不在集群，加载kubeconfig文件
		kubeconfigFile := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		if envCfg := os.Getenv("KUBECONFIG"); envCfg != "" {
			kubeconfigFile = envCfg
		}
		log.Println("[K8S-SERVICE-INIT] Running outside cluster, will use config file:", kubeconfigFile)
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	}

	if err != nil {
		return nil, fmt.Errorf("load k8s config failed: %w", err)
	}

	log.Println("[K8S-SERVICE-INIT] Kubernetes host:", kubeConfig.Host)

	//  API 发现客户端，查询集群开放了哪些 API 资源
	discClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	// 测试连通性并且获取版本
	serverVersion, err := discClient.ServerVersion()
	if err != nil {
		log.Println("[K8S-SERVICE-INIT] Failed to connect to Kubernetes API", err)
		return nil, err
	} else {
		log.Println("[K8S-SERVICE-INIT] Connected to Kubernetes API, version:", serverVersion.String())
	}

	useEndpointSlices := false

	// 1.33版本标记Endpoints API deprecated（弃用），使用EndpointSlices替代
	if serverVersion.Major == "1" && serverVersion.Minor >= "33" {
		log.Println("[K8S-SERVICE-INIT] Kubernetes version > 1.32 Using EndpointSlices for service endpoints")

		useEndpointSlices = true
	}

	// 初始化DynamicClient
	// 动态获取原生资源和CRD
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	// 初始化ClientSet，一般用于读取日志
	clientSet, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	namespace := coreV1.NamespaceAll // Work in all namespaces
	if singleNamespace != "" {
		namespace = singleNamespace
		log.Println("[K8S-SERVICE-INIT] Authorised for a single namespace:", namespace)
	}

	log.Println("[K8S-SERVICE-INIT] Setting up resource watchers...")

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient, time.Minute, namespace, nil)

	rawGVRs, err := getAllGVRs(discClient)
	if err != nil {
		return nil, err
	}
	eventHandler := getHandlerFuncs(eventQueue)

	// GVR清洗流水线
	watchGVRs := deduplicateGVRs(rawGVRs)
	watchGVRs = filterBlockListGVRs(watchGVRs)
	// 根据版本互斥保留endpoints/endpointslices
	watchGVRs = filterEndpointMutualExclusive(watchGVRs, useEndpointSlices)
	// 新增权限预检过滤
	ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCheck()
	watchGVRs = filterNoPermissionGVRs(ctxCheck, dynamicClient, watchGVRs, namespace)

	kube := &Kubernetes{
		dynamicClient: dynamicClient,
		clientSet:     clientSet,

		informerFactory:   factory,
		watchGVRList:      watchGVRs,
		eventQueue:        eventQueue,
		ClusterHost:       kubeConfig.Host,
		Mode:              mode,
		UseEndpointSlices: useEndpointSlices,
		KubeVersion:       serverVersion.String(),
	}

	for _, gvr := range watchGVRs {
		kube.registerInformer(gvr, eventHandler)
	}

	factory.Start(context.Background().Done())
	factory.WaitForCacheSync(context.Background().Done())

	return kube, nil
}

// ListAllResources 拉取所有已监听GVR下的全部资源（所有Group/Version/Resource）
// ns = "" 查询全部命名空间
func (k *Kubernetes) ListAllResources(ns string) ([]*unstructured.Unstructured, error) {
	var allItems []*unstructured.Unstructured

	for _, gvr := range k.watchGVRList {
		items, err := k.ListResourceWithNS(gvr, ns)
		if err != nil {
			log.Printf("[K8S—SVC] warn: list %s failed, skip. err=%v", gvr.String(), err)
			continue
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

// ListResourceWithNS 根据GVR和指定命名空间查询资源
// ns 为空字符串 "" 代表查询所有命名空间
func (k *Kubernetes) ListResourceWithNS(gvr schema.GroupVersionResource, ns string) ([]*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	listOpt := metav1.ListOptions{}

	listObj, err := k.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, listOpt)
	if err != nil {
		return nil, fmt.Errorf("list resource %s namespace[%s] failed: %w", gvr.String(), ns, err)
	}

	result := make([]*unstructured.Unstructured, 0, len(listObj.Items))
	for i := range listObj.Items {
		item := &listObj.Items[i]
		// 清除冗余字段，减少传输体积
		item.SetManagedFields(nil)
		result = append(result, item)
	}

	return result, nil
}

// getAllGVRs 查询集群所有支持 List & Watch 的GVR，排除子资源
func getAllGVRs(disc discovery.DiscoveryInterface) ([]schema.GroupVersionResource, error) {
	_, resourceLists, err := disc.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}

	var gvrs []schema.GroupVersionResource

	for _, list := range resourceLists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			log.Printf("[GET-GVRs] parse groupversion %s err: %v", list.GroupVersion, err)
			continue
		}

		for _, ar := range list.APIResources {
			// 1. 过滤子资源 pods/status deployments/scale
			if strings.Contains(ar.Name, "/") {
				continue
			}

			hasList, hasWatch := false, false
			for _, verb := range ar.Verbs {
				switch verb {
				case "list":
					hasList = true
				case "watch":
					hasWatch = true
				}
			}

			// 只保留同时具备 list + watch
			if hasList && hasWatch {
				gvr := schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: ar.Name,
				}
				gvrs = append(gvrs, gvr)
			}
		}
	}
	return gvrs, nil
}

// checkGVRCanList 预检当前客户端是否拥有该GVR list权限
func checkGVRCanList(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, ns string) bool {
	// 使用轻量 list，limit=1 减少数据传输
	opt := metav1.ListOptions{Limit: 1}
	_, err := dyn.Resource(gvr).Namespace(ns).List(ctx, opt)
	if err == nil {
		return true
	}
	// 判断是否权限禁止
	if apimacherrors.IsForbidden(err) {
		log.Printf("[SKIP GVR] permission forbidden, skip watch %s", gvr.String())
		return false
	}
	// 其他错误（例如资源不存在、连接异常）也直接跳过，避免无效informer
	log.Printf("[WARN GVR] list test failed %s, err=%v", gvr.String(), err)
	return false
}

// filterNoPermissionGVRs 批量过滤无权限资源
func filterNoPermissionGVRs(ctx context.Context, dyn dynamic.Interface, gvrs []schema.GroupVersionResource, targetNS string) []schema.GroupVersionResource {
	var allowed []schema.GroupVersionResource
	for _, gvr := range gvrs {
		if checkGVRCanList(ctx, dyn, gvr, targetNS) {
			allowed = append(allowed, gvr)
		}
	}
	return allowed
}

// deduplicateGVRs GVR去重
// 高可用集群多台 kube-apiserver，开启聚合发现后，可能会返回重复的 GVR
func deduplicateGVRs(gvrs []schema.GroupVersionResource) []schema.GroupVersionResource {
	set := make(map[string]struct{})
	var res []schema.GroupVersionResource
	for _, g := range gvrs {
		key := g.String()
		if _, ok := set[key]; !ok {
			set[key] = struct{}{}
			res = append(res, g)
		}
	}
	return res
}

// filterBlockListGVRs 黑名单过滤不需要监听的资源
func filterBlockListGVRs(gvrs []schema.GroupVersionResource) []schema.GroupVersionResource {
	block := map[string]bool{
		"events":               true,
		"tokenreviews":         true,
		"selfsubjectreviews":   true,
		"subjectaccessreviews": true,
	}
	var res []schema.GroupVersionResource
	for _, gvr := range gvrs {
		if block[gvr.Resource] {
			continue
		}
		res = append(res, gvr)
	}
	return res
}

// filterEndpointMutualExclusive 二选一：endpoints / endpointslices 互斥过滤
func filterEndpointMutualExclusive(gvrs []schema.GroupVersionResource, useEPslice bool) []schema.GroupVersionResource {
	var result []schema.GroupVersionResource
	for _, gvr := range gvrs {
		switch gvr.Resource {
		case "endpoints":
			if useEPslice {
				// 启用EndpointSlice，则剔除原生endpoints
				continue
			}
		case "endpointslices":
			if !useEPslice {
				// 不启用EndpointSlice，则剔除endpointslices
				continue
			}
		}
		result = append(result, gvr)
	}
	return result
}

// registerInformer 抽离：为单个GVR注册informer并绑定事件处理器
func (k *Kubernetes) registerInformer(gvr schema.GroupVersionResource, handler cache.ResourceEventHandlerFuncs) {
	inf := k.informerFactory.ForResource(gvr).Informer()
	inf.AddEventHandler(handler)
	log.Printf("success register informer for %s", gvr.String())
}

func getHandlerFuncs(eventQueue *queue.RingDropOldQueue[KubeEvent]) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			namespace := u.GetNamespace()
			if namespace == "" {
				return
			}

			u.SetManagedFields(nil)
			event := KubeEvent{EventType: AddEvent, Object: u}
			eventQueue.Push(event)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			namespace := u.GetNamespace()
			if namespace == "" {
				return
			}

			u.SetManagedFields(nil)
			event := KubeEvent{EventType: UpdateEvent, Object: u}
			eventQueue.Push(event)
		},

		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			namespace := u.GetNamespace()
			if namespace == "" {
				return
			}

			u.SetManagedFields(nil)
			event := KubeEvent{EventType: DeleteEvent, Object: u}
			eventQueue.Push(event)
		},
	}

}
