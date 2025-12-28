package main

import (
	"fmt"
	"log"
	"os"

	"github.com/wdcbot/go-storage"
	// 使用云存储时取消注释对应的 import
	// _ "github.com/wdcbot/go-storage/drivers/aliyun"
	// _ "github.com/wdcbot/go-storage/drivers/tencent"
	// _ "github.com/wdcbot/go-storage/drivers/s3"
	// _ "github.com/wdcbot/go-storage/drivers/qiniu"
)

func main() {
	// 方式一：直接用 map 配置
	storage.MustSetup(map[string]any{
		"default": "local",
		"disks": map[string]any{
			"local": map[string]any{
				"driver":   "local",
				"root":     "./uploads",
				"base_url": "http://localhost:8080/files",
			},
			// 阿里云配置示例
			// "aliyun": map[string]any{
			// 	"driver":            "aliyun",
			// 	"endpoint":          "oss-cn-hangzhou.aliyuncs.com",
			// 	"bucket":            "my-bucket",
			// 	"access_key_id":     os.Getenv("ALIYUN_ACCESS_KEY_ID"),
			// 	"access_key_secret": os.Getenv("ALIYUN_ACCESS_KEY_SECRET"),
			// },
		},
	})

	// 方式二：使用 viper（推荐）
	// viper.SetConfigFile("config.yaml")
	// viper.ReadInConfig()
	// storage.MustSetup(viper.GetStringMap("storage"))

	// === 基本操作 ===

	// 上传字符串
	result, err := storage.PutString("hello.txt", "Hello, Go-Storage!")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("上传成功: %s\n", result.URL)

	// 上传文件
	if err := createTestFile(); err != nil {
		log.Fatal(err)
	}
	result, err = storage.PutFile("images/test.txt", "test.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("文件上传成功: %s\n", result.Key)

	// 检查文件是否存在
	exists, _ := storage.Exists("hello.txt")
	fmt.Printf("文件存在: %v\n", exists)

	// 下载文件
	content, err := storage.GetString("hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("文件内容: %s\n", content)

	// 获取 URL
	url, _ := storage.URL("hello.txt")
	fmt.Printf("文件 URL: %s\n", url)

	// 删除文件
	storage.Delete("hello.txt")
	storage.Delete("images/test.txt")
	fmt.Println("文件已删除")

	// === 使用指定 disk ===
	// storage.Disk("aliyun").PutString("cloud.txt", "Hello Cloud!")

	// === 高级操作（AdvancedStorage）===
	// 云存储驱动支持更多功能，通过 Storage() 方法获取底层接口：
	//
	// s, err := storage.Disk("aliyun").Storage()
	// if err != nil {
	//     log.Fatal(err)
	// }
	// if adv, ok := s.(storage.AdvancedStorage); ok {
	//     ctx := context.Background()
	//
	//     // 生成临时签名 URL（私有文件访问）
	//     signedURL, _ := adv.SignedURL(ctx, "private.txt", time.Hour)
	//
	//     // 列出目录下的文件
	//     list, _ := adv.List(ctx, "images/")
	//
	//     // 复制/移动文件
	//     adv.Copy(ctx, "a.txt", "b.txt")
	//     adv.Move(ctx, "old.txt", "new.txt")
	//
	//     // 获取文件信息
	//     size, _ := adv.Size(ctx, "file.txt")
	//     meta, _ := adv.Metadata(ctx, "file.txt")
	// }

	// 清理测试文件
	os.Remove("test.txt")
	os.RemoveAll("uploads")
}

func createTestFile() error {
	return os.WriteFile("test.txt", []byte("test content"), 0644)
}
