package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	agridataintegration "agridataintegration"
)

func main() {
	// 配置文件路径
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 加载配置（若不存在则直接报错退出）
	config, err := agridataintegration.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config %s: %v", configPath, err)
	}

	// 创建集成服务
	service, err := agridataintegration.NewIntegrationService(config)
	if err != nil {
		log.Fatalf("Failed to create integration service: %v", err)
	}

	// 启动 HTTP API 服务器
	go startHTTPServer(service, config)

	// 启动集成服务
	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start integration service: %v", err)
	}

	// 等待终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Agriculture data integration service started. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down service...")
	service.Stop()
	log.Println("Service stopped.")
}

// startHTTPServer 启动 HTTP API 服务器
func startHTTPServer(service *agridataintegration.IntegrationService, config *agridataintegration.Config) {
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		stats := service.GetStats()
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   stats,
		})
	})

	http.HandleFunc("/api/mappings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		mappings := service.GetMappings()
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   mappings,
		})
	})

	http.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if err := service.RefreshSensors(); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"status":  "success",
			"message": "Sensors refreshed successfully",
		})
	})

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   config,
		})
	})

	// 健康检查
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"service": "agri-data-integration",
		})
	})

	// CORS 处理
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		http.NotFound(w, r)
	})

	serverAddr := config.Server.Host + ":" + config.Server.Port
	log.Printf("HTTP API server starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}
