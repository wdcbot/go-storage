# Go-Storage

[![Go Reference](https://pkg.go.dev/badge/github.com/wdcbot/go-storage.svg)](https://pkg.go.dev/github.com/wdcbot/go-storage)
[![Go Report Card](https://goreportcard.com/badge/github.com/wdcbot/go-storage)](https://goreportcard.com/report/github.com/wdcbot/go-storage)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> ⚠️ **Alpha 版本** - 首次发布，仍在测试中，可能存在 bug。欢迎试用体验，反馈问题！

一个配置驱动的 Go 文件存储库。告别繁琐的初始化代码，只需一个配置文件即可使用多种存储后端。

## 特性

- 🔌 **可插拔设计** - 支持多种存储后端，按需引入
- 📝 **配置驱动** - YAML/JSON 配置，零代码初始化
- 🔄 **统一接口** - 一套 API 操作所有存储
- 🌍 **环境变量支持** - 敏感信息可通过环境变量配置
- 🚀 **开箱即用** - 内置常用云存储驱动

## 支持的存储后端

| 驱动 | 说明 | 状态 |
|------|------|------|
| `local` | 本地文件系统 | ✅ 内置 |
| `aliyun` / `oss` | 阿里云 OSS | ✅ 可用 |
| `tencent` / `cos` | 腾讯云 COS | ✅ 可用 |
| `s3` / `minio` | AWS S3 / MinIO | ✅ 可用 |
| `qiniu` | 七牛云 | ✅ 可用 |
| `huawei` | 华为云 OBS | 🚧 开发中 |
| `upyun` | 又拍云 | 🚧 开发中 |
| `azure` | Azure Blob | 🚧 开发中 |
| `gcs` | Google Cloud Storage | 🚧 开发中 |

## 安装

```bash
go get github.com/wdcbot/go-storage
```

云存储 driver 按需安装：
```bash
go get github.com/wdcbot/go-storage/drivers/aliyun   # 阿里云 OSS
go get github.com/wdcbot/go-storage/drivers/tencent  # 腾讯云 COS
go get github.com/wdcbot/go-storage/drivers/s3       # AWS S3 / MinIO
go get github.com/wdcbot/go-storage/drivers/qiniu    # 七牛云
```

## 快速开始

在你现有的 `config.yaml` 中添加 storage 配置：

```yaml
app:
  name: myapp
  port: 8080

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
```

```go
package main

import (
    "fmt"
    
    "github.com/spf13/viper"
    "github.com/wdcbot/go-storage"
    _ "github.com/wdcbot/go-storage/drivers/aliyun" // 使用阿里云时 import
)

func main() {
    // 加载你的配置
    viper.SetConfigFile("config.yaml")
    viper.ReadInConfig()
    
    // 一行初始化
    storage.MustSetup(viper.GetStringMap("storage"))
    
    // 上传
    storage.PutString("hello.txt", "Hello World")
    
    // 下载
    content, _ := storage.GetString("hello.txt")
    fmt.Println(content)
    
    // 使用指定 disk
    storage.Disk("aliyun").PutFile("images/photo.jpg", "/path/to/photo.jpg")
    
    // 删除
    storage.Delete("hello.txt")
}
```

### 不用 viper？直接传 map

```go
storage.MustSetup(map[string]any{
    "default": "local",
    "disks": map[string]any{
        "local": map[string]any{
            "driver": "local",
            "root":   "./uploads",
        },
    },
})

storage.Put("test.txt", strings.NewReader("hello"))
```

```go
package main

import (
    "context"
    "strings"

    storage "github.com/wdcbot/go-storage"
    _ "github.com/wdcbot/go-storage/drivers/local"  // 引入本地驱动
    _ "github.com/wdcbot/go-storage/drivers/aliyun" // 引入阿里云驱动
)

func main() {
    // 初始化（只需一次）
    if err := storage.Init("storage.yaml"); err != nil {
        panic(err)
    }

    ctx := context.Background()

    // 使用默认存储
    disk, _ := storage.Default()
    result, _ := disk.Upload(ctx, "hello.txt", strings.NewReader("Hello World"))
    println(result.URL)

    // 使用指定存储
    aliyun, _ := storage.Disk("aliyun")
    aliyun.Upload(ctx, "images/photo.jpg", file)
}
```

## API

### 简化 API（推荐）

```go
// 默认 disk
storage.Put(key, reader)           // 上传 io.Reader
storage.PutFile(key, "/path/to/file")  // 上传本地文件（自动检测 Content-Type）
storage.PutBytes(key, []byte{...})     // 上传 bytes
storage.PutString(key, "hello")        // 上传字符串

storage.Get(key)                   // 下载，返回 io.ReadCloser
storage.GetBytes(key)              // 下载，返回 []byte
storage.GetString(key)             // 下载，返回 string

storage.Delete(key)                // 删除
storage.Exists(key)                // 检查存在
storage.URL(key)                   // 获取 URL

// 指定 disk
storage.Disk("aliyun").Put(key, reader)
storage.Disk("aliyun").PutFile(key, "/path/to/file")
```

### 带 Context（需要超时控制时）

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

storage.Disk("aliyun").PutWithContext(ctx, key, reader)
storage.Disk("aliyun").GetWithContext(ctx, key)
```

### 上传选项

```go
// 设置 Content-Type
storage.Put(key, reader, storage.WithContentType("image/jpeg"))

// 设置元数据
storage.Put(key, reader, storage.WithMetadata(map[string]string{"author": "test"}))

// 设置访问权限
storage.Put(key, reader, storage.WithACL("public-read"))

// 上传进度回调
storage.Put(key, reader, storage.WithProgress(func(uploaded, total int64) {
    fmt.Printf("Progress: %d/%d\n", uploaded, total)
}))
```

### 辅助函数

```go
// 从文件路径上传（自动检测 Content-Type）
storage.UploadFile(ctx, disk, "images/photo.jpg", "/path/to/photo.jpg")

// 下载到文件
storage.DownloadToFile(ctx, disk, "images/photo.jpg", "/path/to/save.jpg")

// 生成唯一 key: prefix/2006/01/02/uuid.ext
key := storage.GenerateKey("images", "photo.jpg")

// 带重试的操作
storage.Retry(ctx, 3, func() error {
    _, err := disk.Upload(ctx, key, reader)
    return err
})
```

### 批量操作

```go
// 批量上传（并发数 5）
items := []storage.BatchUploadItem{
    {Key: "a.txt", Reader: strings.NewReader("a")},
    {Key: "b.txt", Reader: strings.NewReader("b")},
}
result := storage.BatchUpload(ctx, disk, items, 5)
fmt.Printf("成功: %d, 失败: %d\n", len(result.Succeeded), len(result.Failed))

// 批量删除
keys := []string{"a.txt", "b.txt", "c.txt"}
storage.BatchDelete(ctx, disk, keys, 10)

// 删除整个目录
storage.DeleteAll(ctx, disk, "uploads/2024/", 10)
```

### 日志调试

```go
// 方式一：设置环境变量
// STORAGE_DEBUG=1 go run main.go

// 方式二：代码开启
storage.EnableDebugLog()

// 方式三：自定义 logger（支持 slog）
storage.SetLogger(storage.NewSlogAdapter(slog.Default()))

// 方式四：包装单个 storage
disk = storage.WrapWithLogging(disk, "aliyun", myLogger)
```

## 配置说明

配置直接放在你的 `config.yaml` 里，所有参数平铺（不需要 `options` 嵌套）：

### 本地存储 (local)

```yaml
local:
  driver: local
  root: ./uploads        # 存储根目录
  base_url: http://...   # 访问 URL 前缀
```

### 阿里云 OSS (aliyun)

```yaml
aliyun:
  driver: aliyun
  endpoint: oss-cn-hangzhou.aliyuncs.com
  access_key_id: xxx       # 或用环境变量 ${ALIYUN_ACCESS_KEY_ID}
  access_key_secret: xxx
  bucket: my-bucket
  domain: https://cdn.example.com  # 可选：自定义域名
```

### 腾讯云 COS (tencent)

```yaml
tencent:
  driver: tencent
  secret_id: xxx
  secret_key: xxx
  region: ap-guangzhou
  bucket: my-bucket-1234567890
```

### AWS S3 / MinIO (s3)

```yaml
s3:
  driver: s3
  region: us-east-1
  bucket: my-bucket
  access_key_id: xxx
  secret_access_key: xxx
  endpoint: http://localhost:9000  # MinIO 需要
  force_path_style: true           # MinIO 需要
```

### 七牛云 (qiniu)

```yaml
qiniu:
  driver: qiniu
  access_key: xxx
  secret_key: xxx
  bucket: my-bucket
  domain: https://cdn.example.com
  region: z0  # z0=华东, z1=华北, z2=华南
```

## 环境变量

配置文件支持 `${VAR}` 和 `$VAR` 格式的环境变量，会自动展开：

```yaml
aliyun:
  driver: aliyun
  options:
    access_key_id: ${ALIYUN_ACCESS_KEY_ID}
    access_key_secret: $ALIYUN_ACCESS_KEY_SECRET
```

驱动也会自动读取对应的环境变量：

- `ALIYUN_ACCESS_KEY_ID`, `ALIYUN_ACCESS_KEY_SECRET`, `ALIYUN_OSS_ENDPOINT`, `ALIYUN_OSS_BUCKET`
- `TENCENT_SECRET_ID`, `TENCENT_SECRET_KEY`, `TENCENT_COS_REGION`, `TENCENT_COS_BUCKET`
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_S3_BUCKET`
- `QINIU_ACCESS_KEY`, `QINIU_SECRET_KEY`, `QINIU_BUCKET`, `QINIU_DOMAIN`
- `HUAWEI_ACCESS_KEY`, `HUAWEI_SECRET_KEY`, `HUAWEI_OBS_ENDPOINT`, `HUAWEI_OBS_BUCKET`
- `BAIDU_ACCESS_KEY`, `BAIDU_SECRET_KEY`, `BAIDU_BOS_ENDPOINT`, `BAIDU_BOS_BUCKET`
- `UPYUN_BUCKET`, `UPYUN_OPERATOR`, `UPYUN_PASSWORD`, `UPYUN_DOMAIN`
- `AZURE_STORAGE_ACCOUNT`, `AZURE_STORAGE_KEY`, `AZURE_STORAGE_CONTAINER`
- `GCS_BUCKET`, `GOOGLE_APPLICATION_CREDENTIALS`

## 自定义驱动

```go
package mydriver

import storage "github.com/wdcbot/go-storage"

func init() {
    storage.Register("mydriver", func(cfg map[string]any) (storage.Storage, error) {
        // 返回你的 Storage 实现
    })
}
```

## License

MIT
