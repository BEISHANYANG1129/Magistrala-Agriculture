package image_upload_server

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func uploadImage(filePath string, uploadURL string, uploadName string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", uploadName)
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

	respBody := &bytes.Buffer{}
	_, err = io.Copy(respBody, resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("上传失败，状态码: %d，响应: %s", resp.StatusCode, respBody.String())
	}

	return respBody.String(), nil
}

func TestImageUpload(t *testing.T) {
	imagePath := "static/pictures/test.png" // 路径相对于 image_upload_server 目录
	absImagePath, err := filepath.Abs(imagePath)
	if err != nil {
		t.Fatalf("获取源图片绝对路径失败: %v", err)
	}
	t.Logf("准备上传的源图片绝对路径: %s", absImagePath)

	uploadURL := "http://localhost:18080/upload"
	// 使用唯一文件名，避免覆盖
	uploadName := fmt.Sprintf("test_%d.png", time.Now().UnixNano())
	url, err := uploadImage(absImagePath, uploadURL, uploadName)
	if err != nil {
		t.Fatalf("图片上传失败: %v", err)
	}
	t.Logf("上传返回的 URL: %s", url)

	expectedPrefix := "http://localhost:18080/uploaded/"
	if len(url) == 0 || url[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("返回的 URL 不正确: %s", url)
	} else {
		t.Logf("返回的 URL 校验通过: %s", url)
	}
}
