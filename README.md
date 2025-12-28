# Go-Storage

[![Go Reference](https://pkg.go.dev/badge/github.com/wdcbot/go-storage.svg)](https://pkg.go.dev/github.com/wdcbot/go-storage)
[![Go Report Card](https://goreportcard.com/badge/github.com/wdcbot/go-storage)](https://goreportcard.com/report/github.com/wdcbot/go-storage)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> ⚠️ **Alpha 版本** - 欢迎试用，反馈问题！

配置驱动的 Go 文件存储库。一行代码初始化，统一 API 操作多种存储后端。

## 安装

```bash
go get github.com/wdcbot/go-storage

# 云存储按需安装
go get github.com/wdcbot/go-storage/drivers/aliyun   # 阿里云 OSS
go get github.com/wdcbot/go-storage/drivers/tencent  # 腾讯云 COS
go get github.com/wdcbot/go-storage/drivers/s3       # AWS S3 / MinIO
go get github.com/wdcbot/go-storage/drivers/qiniu    # 七牛云
```

## 快速开始

```go
package main

import (
    "github.com/wdcbot/go-storage"
    _ "github.com/wdcbot/go-storage/drivers/aliyun" // 按需 import
)

func main() {
    // 初始化（配合 viper 使用）
    // storage.MustSetup(viper.GetStringMap("storage"))
    
    // 或直接传 map
    storage.MustSetup(map[string]any{
        "default": "local",
        "disks": map[string]any{
            "local": map[string]any{
                "driver": "local",
                "root":   "./uploads",
            },
        },
    })

    // 上传
    storage.PutString("hello.txt", "Hello World")
    storage.PutFile("photo.jpg", "/path/to/photo.jpg")
    
    // 下载
    content, _ := storage.GetString("hello.txt")
    
    // 其他操作
    storage.Exists("hello.txt")
    storage.Delete("hello.txt")
    storage.URL("hello.txt")
    
    // 指定 disk
    storage.Disk("aliyun").PutString("cloud.txt", "Hello Cloud")
}
```

## 配置示例

```yaml
# config.yaml
storage:
  default: local
  disks:
    local:
      driver: local
      root: ./uploads
      base_url: http://localhost:8080/files

    aliyun:
      driver: aliyun
      endpoint: oss-cn-hangzhou.aliyuncs.com
      bucket: my-bucket
      access_key_id: ${ALIYUN_ACCESS_KEY_ID}
      access_key_secret: ${ALIYUN_ACCESS_KEY_SECRET}

    tencent:
      driver: tencent
      region: ap-guangzhou
      bucket: my-bucket-1234567890
      secret_id: ${TENCENT_SECRET_ID}
      secret_key: ${TENCENT_SECRET_KEY}

    s3:
      driver: s3
      region: us-east-1
      bucket: my-bucket
      access_key_id: ${AWS_ACCESS_KEY_ID}
      secret_access_key: ${AWS_SECRET_ACCESS_KEY}

    minio:
      driver: s3
      endpoint: http://localhost:9000
      bucket: my-bucket
      access_key_id: minioadmin
      secret_access_key: minioadmin
      force_path_style: true

    qiniu:
      driver: qiniu
      bucket: my-bucket
      access_key: ${QINIU_ACCESS_KEY}
      secret_key: ${QINIU_SECRET_KEY}
      domain: https://cdn.example.com
      region: z0
```

## API

```go
// 基础操作
storage.Put(key, reader)              // 上传
storage.PutFile(key, filePath)        // 上传文件（自动检测 Content-Type）
storage.PutString(key, content)       // 上传字符串
storage.PutBytes(key, data)           // 上传 bytes

storage.Get(key)                      // 下载 -> io.ReadCloser
storage.GetString(key)                // 下载 -> string
storage.GetBytes(key)                 // 下载 -> []byte

storage.Delete(key)                   // 删除
storage.Exists(key)                   // 检查存在
storage.URL(key)                      // 获取 URL

// 指定 disk
storage.Disk("aliyun").PutString(key, content)

// 上传选项
storage.Put(key, reader,
    storage.WithContentType("image/jpeg"),
    storage.WithACL("public-read"),
    storage.WithMetadata(map[string]string{"author": "test"}),
)
```

## 支持的存储

| Driver | 状态 | 说明 |
|--------|------|------|
| `local` | ✅ 内置 | 本地文件系统 |
| `aliyun` | ✅ | 阿里云 OSS |
| `tencent` | ✅ | 腾讯云 COS |
| `s3` | ✅ | AWS S3 / MinIO |
| `qiniu` | ✅ | 七牛云 |

## License

MIT
