package main

import (
	"agriDeviceExecutor/internal/api"
	"agriDeviceExecutor/internal/config"
	"agriDeviceExecutor/internal/data"
	"agriDeviceExecutor/internal/service"
	"log"
	"net/http"
	"time"
)

func main() {
	// 启动前先加载已有映射，避免重复注册
	if err := data.LoadMapping(""); err != nil {
		log.Printf("[startup] 加载映射失败: %v", err)
	} else {
		log.Printf("[startup] 已加载执行映射（节点级）")
	}

	mux := api.SetupMux()

	// 启动 HTTP 服务
	srv := &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}
	log.Println("HTTP server listening on :8090")
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// 启动后：先调用第三方平台登录（直接用 service.UserLogin），再执行同步
	go func() {
		// 稍等服务就绪（非必须，仅避免日志交错）
		time.Sleep(500 * time.Millisecond)

		// 1) 直接调用第三方登录逻辑（参考 global_service.go）
		//    成功后会把 token 持久化到 config.json（agriPlatform.userToken）
		log.Println("[startup] 执行第三方平台登录...")
		if _, err := service.UserLogin("", ""); err != nil {
			log.Printf("[startup] 登录失败，跳过首次同步: %v", err)
			return
		}
		log.Println("[startup] 登录成功")

		// 2) 读取严格版 token 与基础地址
		token, err := config.GetUserToken()
		if err != nil {
			log.Printf("[startup] 读取 token 失败，跳过首次同步: %v", err)
			return
		}
		baseURL, err := config.GetNormalizedAPIBaseURL()
		if err != nil {
			log.Printf("[startup] 获取基础地址失败，跳过首次同步: %v", err)
			return
		}

		// 3) 执行一次全量同步（发现设备与节点，并向 Magistrala 注册缺失的 client）
		log.Println("[startup] 开始首次设备/节点同步...")
		if err := service.SyncAll(token, baseURL); err != nil {
			log.Printf("[startup] 首次同步失败: %v", err)
		} else {
			log.Printf("[startup] 首次同步完成")
		}

		// 4) 可选：周期增量同步（如每 5 分钟）
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			// 每轮前读取最新 token（避免过期）
			token, err = config.GetUserToken()
			if err != nil {
				log.Printf("[sync] 读取 token 失败，跳过本轮: %v", err)
				continue
			}
			if err := service.SyncAll(token, baseURL); err != nil {
				log.Printf("[sync] 周期同步失败: %v", err)
			} else {
				log.Printf("[sync] 周期同步完成")
			}
		}
	}()

	// 阻塞主协程（也可实现优雅关闭）
	select {}
}
