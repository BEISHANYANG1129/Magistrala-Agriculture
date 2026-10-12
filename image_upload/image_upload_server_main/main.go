package main

import (
	"image_upload/image_upload_server"
	"path/filepath"
)

func main() {
	// 获取 image_upload_server 包目录，并拼接 test_uploads 作为图片存储目录
	serverDir := filepath.Join("../image_upload_server")
	uploadDir := filepath.Join(serverDir, "test_uploads")

	cfg := image_upload_server.ImageUploadConfig{
		UploadDir:  uploadDir,
		ServerAddr: ":18080",
		BaseURL:    "http://localhost:18080",
	}
	image_upload_server.StartImageUploadServer(cfg)
}
