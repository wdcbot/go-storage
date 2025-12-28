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

| 驱动 | 说明 |
|------|------|
| `local` | 本地文件系统 |
| `aliyun` / `alioss` | 阿里云 OSS |
| `tencent` / `cos` | 腾讯云 COS |
| `s3` | AWS S3 |
| `minio` | MinIO (S3 兼容) |
| `qiniu` | 七牛云 |
| `huawei` / `obs` | 华为云 OBS |
| `baidu` / `bos` | 百度云 BOS |
| `upyun` | 又拍云 |
| `azure` / `azblob` | Azure Blob Storage |
| `gcs` / `google` | Google Cloud Storage |

## 安装

```bash
go get github.com/wdcbot/go-storage

# 按需引入驱动
go get github.com/wdcbot/go-storage/drivers/local
go get github.com/wdcbot/go-storage/drivers/aliyun
go get github.com/wdcbot/go-storage/drivers/tencent
go get github.com/wdcbot/go-storage/drivers/s3
go get github.com/wdcbot/go-storage/drivers/qiniu
go get github.com/wdcbot/go-storage/drivers/huawei
go get github.com/wdcbot/go-storage/drivers/baidu
go get github.com/wdcbot/go-storage/drivers/upyun
go get github.com/wdcbot/go-storage/drivers/azure
go get github.com/wdcbot/go-storage/drivers/gcs
```

## 快速开始

### 方式一：独立配置文件

创建 `storage.yaml`：

```yaml
default: local

storages:
  local:
    driver: local
    options:
      root: ./uploads
      base_url: http://localhost:8080/files
```

```go
storage.Init("storage.yaml")
```

### 方式二：嵌入现有配置文件

在你的 `config.yaml` 中添加 storage 配置：

```yaml
app:
  name: myapp
  port: 8080

database:
  host: localhost

storage:                    # <-- 嵌入这里
  default: local
  storages:
    local:
      driver: local
      options:
        root: ./uploads
```

```go
storage.InitEmbedded("config.yaml")
```

### 方式三：自定义配置 key

```yaml
# config.yaml
oss:                        # <-- 自定义 key
  default: aliyun
  storages:
    aliyun:
      driver: aliyun
      options:
        bucket: my-bucket
```

```go
storage.InitEmbeddedWithKey("config.yaml", "oss")
```

### 方式四：与 viper/koanf 等配置库集成

```go
import "github.com/spf13/viper"

viper.SetConfigFile("config.yaml")
viper.ReadInConfig()

cfg, _ := storage.NewConfigFromMap(viper.GetStringMap("storage"))
storage.InitFromConfig(cfg)
```

### 使用

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

### Storage 接口（基础）

```go
type Storage interface {
    Upload(ctx, key, reader, opts...) (*UploadResult, error)
    Download(ctx, key) (io.ReadCloser, error)
    Delete(ctx, key) error
    Exists(ctx, key) (bool, error)
    URL(ctx, key) (string, error)
    Close() error
}
```

### AdvancedStorage 接口（扩展）

```go
type AdvancedStorage interface {
    Storage
    SignedURL(ctx, key, expires) (string, error)  // 签名 URL
    List(ctx, prefix, opts...) (*ListResult, error) // 文件列表
    Copy(ctx, src, dst) error                      // 复制
    Move(ctx, src, dst) error                      // 移动
    Size(ctx, key) (int64, error)                  // 文件大小
    Metadata(ctx, key) (*FileInfo, error)          // 元数据
}
```

### 上传选项

```go
// 设置 Content-Type
storage.WithContentType("image/jpeg")

// 设置元数据
storage.WithMetadata(map[string]string{"author": "test"})

// 设置访问权限
storage.WithACL("public-read")

// 上传进度回调
storage.WithProgress(func(uploaded, total int64) {
    fmt.Printf("Progress: %d/%d\n", uploaded, total)
})
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

### 本地存储 (local)

```yaml
local:
  driver: local
  options:
    root: ./uploads        # 存储根目录
    base_url: http://...   # 访问 URL 前缀
    perm: 0644             # 文件权限
```

### 阿里云 OSS (aliyun)

```yaml
aliyun:
  driver: aliyun
  options:
    endpoint: oss-cn-hangzhou.aliyuncs.com
    access_key_id: xxx
    access_key_secret: xxx
    bucket: my-bucket
    domain: https://cdn.example.com  # 可选：自定义域名
```

### 腾讯云 COS (tencent)

```yaml
tencent:
  driver: tencent
  options:
    secret_id: xxx
    secret_key: xxx
    region: ap-guangzhou
    bucket: my-bucket-1234567890
```

### AWS S3 / MinIO (s3)

```yaml
s3:
  driver: s3
  options:
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
  options:
    access_key: xxx
    secret_key: xxx
    bucket: my-bucket
    domain: https://cdn.example.com
    region: z0  # z0=华东, z1=华北, z2=华南
```

### 华为云 OBS (huawei)

```yaml
huawei:
  driver: huawei
  options:
    endpoint: obs.cn-north-4.myhuaweicloud.com
    access_key: xxx
    secret_key: xxx
    bucket: my-bucket
```

### 百度云 BOS (baidu)

```yaml
baidu:
  driver: baidu
  options:
    endpoint: bj.bcebos.com
    access_key: xxx
    secret_key: xxx
    bucket: my-bucket
```

### 又拍云 (upyun)

```yaml
upyun:
  driver: upyun
  options:
    bucket: my-bucket
    operator: xxx
    password: xxx
    domain: https://cdn.example.com
```

### Azure Blob Storage (azure)

```yaml
azure:
  driver: azure
  options:
    account_name: xxx
    account_key: xxx
    container: my-container
```

### Google Cloud Storage (gcs)

```yaml
gcs:
  driver: gcs
  options:
    bucket: my-bucket
    credentials_file: /path/to/service-account.json
    # Or use GOOGLE_APPLICATION_CREDENTIALS env var
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
