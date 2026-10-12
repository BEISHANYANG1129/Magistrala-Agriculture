package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	fmt.Println("代理服务启动，监听端口: 8090")
	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("收到请求: %s %s\n", r.Method, r.URL.String())
		// 处理预检请求
		if r.Method == http.MethodOptions {
			fmt.Println("处理预检请求（OPTIONS）")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
			w.WriteHeader(http.StatusOK)
			return
		}

		// 正常 GET 请求代理
		url := "http://localhost:9011" + r.URL.Path[len("/api"):] + "?" + r.URL.RawQuery
		fmt.Printf("代理到后端URL: %s\n", url)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("代理请求失败: %v\n", err)
			http.Error(w, "代理失败", 500)
			return
		}
		defer resp.Body.Close()
		fmt.Printf("后端响应状态: %d\n", resp.StatusCode)
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
		w.WriteHeader(resp.StatusCode)
		n, copyErr := io.Copy(w, resp.Body)
		fmt.Printf("已转发响应字节数: %d, copyErr: %v\n", n, copyErr)
	})
	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		fmt.Printf("代理服务启动失败: %v\n", err)
	}
}
