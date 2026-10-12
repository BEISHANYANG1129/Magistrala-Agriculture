package image_upload_server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 图片上传服务配置结构体
// 包含本地存储目录、服务监听地址、URL 基础路径
type ImageUploadConfig struct {
	UploadDir  string // 本地存储目录
	ServerAddr string // 服务地址（如 :18080）
	BaseURL    string // 返回的 URL 基础路径（如 http://localhost:18080）
}

// 启动图片上传服务
func StartImageUploadServer(cfg ImageUploadConfig) {
	// 1. 确保上传目录存在（递归创建）
	err := os.MkdirAll(cfg.UploadDir, 0755)
	if err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	// 2. 注册静态文件服务路由
	// 访问 /uploaded/xxx.png 时，实际读取本地上传目录下的文件
	http.Handle("/uploaded/", http.StripPrefix("/uploaded/", http.FileServer(http.Dir(cfg.UploadDir))))

	// 3. 注册图片上传接口路由
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		// 只允许 POST 请求
		if r.Method != http.MethodPost {
			http.Error(w, "仅支持 POST 请求", http.StatusMethodNotAllowed)
			return
		}

		// 解析上传的文件（表单字段名为 "file"）
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "文件上传失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 检查文件类型（仅允许图片类型）
		if !isValidImage(header.Filename) {
			http.Error(w, "仅支持图片文件上传", http.StatusUnsupportedMediaType)
			return
		}

		// 自动生成唯一文件名，准备保存图片到本地目录
		originalName := header.Filename
		ext := filepath.Ext(originalName)
		uniqueName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext) // 时间戳+扩展名，保证唯一
		filePath := filepath.Join(cfg.UploadDir, uniqueName)

		// 检查目标目录是否存在，不存在则创建
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "创建目标目录失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 创建文件并写入上传内容（真正的保存操作）
		outFile, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "保存文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, file)
		if err != nil {
			http.Error(w, "写入文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 日志输出实际保存路径，便于调试和定位
		absPath, _ := filepath.Abs(filePath)
		log.Printf("图片实际保存路径: %s", absPath)

		// 构造图片访问 URL 并返回给客户端
		fileURL := fmt.Sprintf("%s/uploaded/%s", cfg.BaseURL, uniqueName)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileURL))
	})

	// 4. 启动 HTTP 服务，监听指定端口
	log.Printf("图片上传服务启动，地址: %s，上传目录: %s", cfg.ServerAddr, cfg.UploadDir)
	log.Fatal(http.ListenAndServe(cfg.ServerAddr, nil))
}

// 检查文件是否为有效图片类型（只允许常见图片扩展名）
func isValidImage(fileName string) bool {
	allowedExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp"}
	ext := strings.ToLower(filepath.Ext(fileName))
	for _, allowed := range allowedExtensions {
		if ext == allowed {
			return true
		}
	}
	return false
}
