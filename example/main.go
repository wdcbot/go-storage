package main

import (
	"fmt"
	"io"
	"log"
	"strings"

	storage "github.com/wdcbot/go-storage"
	// local driver 已内置，无需 import
)

func main() {
	// 直接用 map 配置（实际项目中用 viper.GetStringMap("storage")）
	storage.MustSetup(map[string]any{
		"default": "local",
		"disks": map[string]any{
			"local": map[string]any{
				"driver":   "local",
				"root":     "./uploads",
				"base_url": "http://localhost:8080/files",
			},
		},
	})

	// 上传
	result, err := storage.Put("test/hello.txt", strings.NewReader("Hello, Go-Storage!"),
		storage.WithContentType("text/plain"),
	)
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Printf("Uploaded: %s\n", result.Key)
	fmt.Printf("URL: %s\n", result.URL)

	// 检查存在
	exists, err := storage.Exists("test/hello.txt")
	if err != nil {
		log.Fatalf("Exists check failed: %v", err)
	}
	fmt.Printf("Exists: %v\n", exists)

	// 下载
	reader, err := storage.Get("test/hello.txt")
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("Read failed: %v", err)
	}
	fmt.Printf("Content: %s\n", string(data))

	// 删除
	if err := storage.Delete("test/hello.txt"); err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("Deleted successfully")

	// 使用指定 disk
	// storage.Disk("aliyun").Put("images/photo.jpg", file)
}
