package main

import (
	"flag"
	"log"
	"net/http"

	"agri-control-service/internal/api"
	"agri-control-service/internal/logstore"
	"agri-control-service/internal/registry"
	"agri-control-service/internal/service"
)

// 程序入口：加载任务配置、初始化日志存储与控制服务，并启动 HTTP 接口。
func main() {
	// 支持通过参数指定任务注册表文件和并发 worker 数量。
	registryPath := flag.String("registry", "configs/scenarios.yaml", "registry config file (yaml/json)")
	workers := flag.Int("workers", 4, "number of concurrent worker goroutines")
	flag.Parse()

	// 优先加载外部任务场景配置，失败则回退到内置默认配置。
	if err := registry.LoadFromFile(*registryPath); err != nil {
		log.Printf("registry: load %s failed, fallback to built-in: %v", *registryPath, err)
	} else {
		log.Printf("registry: loaded from %s", *registryPath)
	}

	// 初始化执行日志存储；失败时仅禁用落盘，不影响主流程。
	store, err := logstore.NewLogStore("data/execution.log")
	if err != nil {
		log.Printf("execution log disabled: %v", err)
	}

	// 组装控制服务和 HTTP 处理器。
	ctrl := service.NewControlService(store, *workers)
	handler := api.NewHandler(ctrl)

	// 注册 API 路由。
	http.HandleFunc("/control/task", handler.HandleTask)

	log.Println("Agri Control Service running on :8280")
	log.Fatal(http.ListenAndServe(":8280", nil))
}
