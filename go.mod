module github.com/wdcbot/go-storage

go 1.21

require (
	github.com/wdcbot/go-storage/drivers/local v0.0.0-20251228015841-6dba4ad5fd88
	gopkg.in/yaml.v3 v3.0.1
)

// Driver dependencies (users import what they need):
// - local: no extra dependencies
// - aliyun: github.com/aliyun/aliyun-oss-go-sdk
// - tencent: github.com/tencentyun/cos-go-sdk-v5
// - s3/minio: github.com/aws/aws-sdk-go-v2
// - qiniu: github.com/qiniu/go-sdk/v7
// - huawei: github.com/huaweicloud/huaweicloud-sdk-go-obs
// - baidu: github.com/baidubce/bce-sdk-go
// - upyun: github.com/upyun/go-sdk/v3
// - azure: github.com/Azure/azure-sdk-for-go/sdk/storage/azblob
// - gcs: cloud.google.com/go/storage
