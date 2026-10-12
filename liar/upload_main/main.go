package main

import (
	"liar/upload"
	"os"
)

func main() {
	cfg := upload.UploadConfig{
		BaseURL:      "https://localhost",
		DomainID:     os.Getenv("MG_WORKSPACE_ID"),
		ChannelID:    os.Getenv("MG_CHANNEL_ID"),
		Subtopic:     "temperature",
		ClientSecret: os.Getenv("MG_DEVICE_SECRET"),
		CACertPath:   "../CA/ca.crt",
		BaseName:     "ljp",
		Timeout:      10,
	}
	uploadType := "senml_image" // 或 "senml"
	imagePath := "test.jpg"
	imageUploadURL := "http://localhost:18080/upload"

	upload.Upload(cfg, uploadType, imagePath, imageUploadURL)
}
