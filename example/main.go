package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	storage "github.com/wdcbot/go-storage"
	_ "github.com/wdcbot/go-storage/drivers/local" // Import local driver
	// _ "github.com/wdcbot/go-storage/drivers/aliyun"  // Uncomment to use Aliyun
	// _ "github.com/wdcbot/go-storage/drivers/tencent" // Uncomment to use Tencent
	// _ "github.com/wdcbot/go-storage/drivers/s3"      // Uncomment to use S3
	// _ "github.com/wdcbot/go-storage/drivers/qiniu"   // Uncomment to use Qiniu
)

func main() {
	// Initialize from config file
	if err := storage.Init("storage.yaml"); err != nil {
		log.Fatalf("Failed to init storage: %v", err)
	}

	ctx := context.Background()

	// Get default storage
	disk, err := storage.Default()
	if err != nil {
		log.Fatalf("Failed to get default storage: %v", err)
	}

	// Upload a file
	content := "Hello, Go-Storage!"
	result, err := disk.Upload(ctx, "test/hello.txt", strings.NewReader(content),
		storage.WithContentType("text/plain"),
	)
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Printf("Uploaded: %s\n", result.Key)
	fmt.Printf("URL: %s\n", result.URL)

	// Check if file exists
	exists, err := disk.Exists(ctx, "test/hello.txt")
	if err != nil {
		log.Fatalf("Exists check failed: %v", err)
	}
	fmt.Printf("Exists: %v\n", exists)

	// Download the file
	reader, err := disk.Download(ctx, "test/hello.txt")
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	defer reader.Close()

	buf := new(strings.Builder)
	if _, err := buf.ReadFrom(reader); err != nil {
		log.Fatalf("Read failed: %v", err)
	}
	fmt.Printf("Content: %s\n", buf.String())

	// Delete the file
	if err := disk.Delete(ctx, "test/hello.txt"); err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("Deleted successfully")

	// Use a specific disk
	// aliyunDisk, _ := storage.Disk("aliyun")
	// aliyunDisk.Upload(ctx, "images/photo.jpg", file)
}
