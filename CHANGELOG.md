# Changelog

## v0.3.0-alpha (2025-12-28)

### Added
- `DiskWrapper.Storage()` 公开方法，用于访问 AdvancedStorage 接口

### Fixed
- 修复测试中 mockStorage 的 data race 问题
- 修复 example 中引用私有方法的注释

## v0.2.0-alpha (2025-12-28)

### Added
- 阿里云 OSS driver (`drivers/aliyun`)
- 腾讯云 COS driver (`drivers/tencent`)
- AWS S3 / MinIO driver (`drivers/s3`)
- 七牛云 driver (`drivers/qiniu`)
- `SignedURL` 签名 URL 支持
- `List` 文件列表
- `Copy` / `Move` 文件操作
- `Size` / `Metadata` 文件信息

## v0.1.0-alpha (2025-12-28)

### Added
- 核心 Storage 接口
- 内置 local driver
- 简化 API: `Put`, `Get`, `Delete`, `Exists`, `URL`
- 便捷方法: `PutFile`, `PutString`, `PutBytes`, `GetString`, `GetBytes`
- 配置支持: viper/koanf 集成, 环境变量展开
- 批量操作: `BatchUpload`, `BatchDelete`
- 日志支持: `SetLogger`, `EnableDebugLog`
