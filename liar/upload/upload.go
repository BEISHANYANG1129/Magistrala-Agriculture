package upload

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// 上传配置参数结构体
type UploadConfig struct {
	BaseURL      string
	DomainID     string
	ChannelID    string
	Subtopic     string
	ClientSecret string
	CACertPath   string
	BaseName     string
	Timeout      int
}

// 普通 SenML 消息结构体（数值型）
type SenMLValueRecord struct {
	Bn string  `json:"bn,omitempty"` // Base Name
	N  string  `json:"n"`            // Name
	U  string  `json:"u,omitempty"`  // Unit
	V  float64 `json:"v,omitempty"`  // Value
}

// 图片 SenML 消息结构体（图片路径字符串）
type SenMLImageRecord struct {
	Bn string `json:"bn,omitempty"` // Base Name
	N  string `json:"n"`            // Name
	U  string `json:"u,omitempty"`  // Unit
	Vs string `json:"vs,omitempty"` // String Value（图片路径或URL）
}

// 生成普通 SenML 数据
func generateSenMLData(baseName string) []SenMLValueRecord {
	return []SenMLValueRecord{
		{Bn: baseName, N: "lumen", U: "CD", V: 53.1},
	}
}

// 生成图片 SenML 数据（此处 imagePath 应为图片URL）
func generateSenMLImageData(baseName, imageURL string) []SenMLImageRecord {
	return []SenMLImageRecord{
		{
			Bn: baseName,
			N:  "image",
			U:  "path",
			Vs: imageURL,
		},
	}
}

// 加载CA证书
func loadCACert(caCertPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return caCertPool, nil
}

// 上传图片到 image_upload_server，返回图片访问 URL
func UploadImageToServer(imagePath string, uploadURL string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("无法打开图片: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("创建表单失败: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", fmt.Errorf("写入文件内容失败: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("上传失败，状态码: %d，响应: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil // 返回图片访问 URL
}

// 上传数据到 HTTP 服务
func uploadData(cfg UploadConfig, data []byte, contentType string) bool {
	url := fmt.Sprintf("%s/http/m/%s/c/%s/%s", cfg.BaseURL, cfg.DomainID, cfg.ChannelID, cfg.Subtopic)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return false
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Client "+cfg.ClientSecret)

	// 加载CA证书
	caCertPool, err := loadCACert(cfg.CACertPath)
	if err != nil {
		log.Printf("加载CA证书失败: %v", err)
		return false
	}
	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            caCertPool,
				InsecureSkipVerify: true, // 跳过证书校验，适合本地测试
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("上传请求失败: %v", err)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("响应状态码: %d", resp.StatusCode)
	log.Printf("响应内容: %s", string(body))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// 上传入口函数
func Upload(cfg UploadConfig, uploadType, imagePath, imageUploadURL string) bool {
	var success bool
	switch uploadType {
	case "senml":
		senmlData := generateSenMLData(cfg.BaseName)
		data, _ := json.Marshal(senmlData)
		success = uploadData(cfg, data, "application/senml+json")
	case "senml_image":
		imgURL, err := UploadImageToServer(imagePath, imageUploadURL)
		if err != nil {
			log.Printf("图片上传失败: %v", err)
			return false
		}
		log.Printf("图片访问 URL: %s", imgURL)
		senmlImgData := generateSenMLImageData(cfg.BaseName, imgURL)
		data, _ := json.Marshal(senmlImgData)
		success = uploadData(cfg, data, "application/senml+json")
	default:
		log.Printf("未知上传类型: %s", uploadType)
		return false
	}
	log.Printf("上传结果: %v", success)
	return success
}
