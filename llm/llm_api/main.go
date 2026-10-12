package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// 配置和知识库路径相对于 llm 模块目录。兼容从 llm_api 目录执行 go run。
	if _, err := os.Stat("config/config.json"); os.IsNotExist(err) {
		if _, parentErr := os.Stat("../config/config.json"); parentErr == nil {
			if err := os.Chdir(".."); err != nil {
				log.Fatalf("切换到 llm 目录失败: %v", err)
			}
		}
	}

	mux := http.NewServeMux()
	// orch, err := NewDefaultOrchestratorFromConfig()
	// if err != nil {
	// 	log.Fatalf("加载配置失败: %v", err)
	// }
	// mux.HandleFunc("/llm/plan-and-execute", func(w http.ResponseWriter, r *http.Request) {
	// 	PlanAndExecuteHandler(w, r, orch)
	// })
	ctrlAdapter, err := NewControlAdapterFromConfig()
	if err != nil {
		log.Fatalf("加载控制服务配置失败: %v", err)
	}
	mux.HandleFunc("/llm/plan-and-send", func(w http.ResponseWriter, r *http.Request) { // 新增路由
		PlanAndSendToControlHandler(w, r, ctrlAdapter)
	})
	log.Println("llm_api listening on :9000")
	if err := http.ListenAndServe(":9000", mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
