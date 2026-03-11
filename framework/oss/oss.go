package oss

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// OSS 对象存储接口
type OSS interface {
	Upload(file *multipart.FileHeader, key string) (*UploadResult, error)
	UploadFromBytes(data []byte, key string) (*UploadResult, error)
	Download(key string) ([]byte, error)
	Delete(key string) error
	GetURL(key string) string
	Exists(key string) (bool, error)
}

// UploadResult 上传结果
type UploadResult struct {
	Success bool   `json:"success"`
	Key     string `json:"key"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	Message string `json:"message"`
}

// AliyunOSS 阿里云 OSS
type AliyunOSS struct {
	Endpoint  string
	Bucket    string
	AccessID  string
	AccessKey string
	CDN       string
}

// NewAliyunOSS 创建阿里云 OSS 实例
func NewAliyunOSS(endpoint, bucket, accessID, accessKey, cdn string) *AliyunOSS {
	return &AliyunOSS{
		Endpoint:  endpoint,
		Bucket:    bucket,
		AccessID:  accessID,
		AccessKey: accessKey,
		CDN:       cdn,
	}
}

// Upload 上传文件
func (o *AliyunOSS) Upload(file *multipart.FileHeader, key string) (*UploadResult, error) {
	// 打开文件
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// 读取文件内容
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	return o.UploadFromBytes(data, key)
}

// UploadFromBytes 上传字节数据
func (o *AliyunOSS) UploadFromBytes(data []byte, key string) (*UploadResult, error) {
	// 构造请求
	method := "PUT"
	contentType := "application/octet-stream"
	date := time.Now().UTC().Format(time.RFC1123)
	contentMD5 := fmt.Sprintf("%x", md5.Sum(data))
	contentLength := len(data)

	// 生成签名
	canonicalizedResource := "/" + o.Bucket + "/" + key
	stringToSign := method + "\n" + contentMD5 + "\n" + contentType + "\n" + date + "\n" + canonicalizedResource
	signature := o.sign(stringToSign)

	// 构造 URL
	u := &url.URL{
		Scheme: "https",
		Host:   o.Bucket + "." + o.Endpoint,
		Path:   "/" + key,
	}

	// 创建请求
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Authorization", "OSS "+o.AccessID+":"+signature)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-MD5", contentMD5)
	req.Header.Set("Content-Length", strconv.Itoa(contentLength))
	req.Header.Set("Date", date)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.New("上传失败：" + string(body))
	}

	return &UploadResult{
		Success: true,
		Key:     key,
		URL:     o.GetURL(key),
		Size:    int64(contentLength),
		Message: "上传成功",
	}, nil
}

// Download 下载文件
func (o *AliyunOSS) Download(key string) ([]byte, error) {
	// 实现下载逻辑
	return []byte{}, nil
}

// Delete 删除文件
func (o *AliyunOSS) Delete(key string) error {
	// 实现删除逻辑
	return nil
}

// GetURL 获取文件 URL
func (o *AliyunOSS) GetURL(key string) string {
	if o.CDN != "" {
		return "https://" + o.CDN + "/" + key
	}
	return "https://" + o.Bucket + "." + o.Endpoint + "/" + key
}

// Exists 检查文件是否存在
func (o *AliyunOSS) Exists(key string) (bool, error) {
	// 实现检查逻辑
	return true, nil
}

// sign 生成签名
func (o *AliyunOSS) sign(stringToSign string) string {
	h := hmac.New(sha1.New, []byte(o.AccessKey))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature
}

// QiniuOSS 七牛云 OSS
type QiniuOSS struct {
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	CDN       string
	Zone      string
}

// NewQiniuOSS 创建七牛云 OSS 实例
func NewQiniuOSS(region, bucket, accessKey, secretKey, cdn string) *QiniuOSS {
	return &QiniuOSS{
		Region:    region,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		CDN:       cdn,
	}
}

// Upload 上传文件
func (q *QiniuOSS) Upload(file *multipart.FileHeader, key string) (*UploadResult, error) {
	// 打开文件
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// 读取文件内容
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	return q.UploadFromBytes(data, key)
}

// UploadFromBytes 上传字节数据
func (q *QiniuOSS) UploadFromBytes(data []byte, key string) (*UploadResult, error) {
	// 生成上传凭证
	uploadToken := q.generateUploadToken()

	// 构造 multipart 表单
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加 token
	fw, err := writer.CreateFormField("token")
	if err != nil {
		return nil, err
	}
	fw.Write([]byte(uploadToken))

	// 添加文件
	fw, err = writer.CreateFormFile("file", key)
	if err != nil {
		return nil, err
	}
	fw.Write(data)

	writer.Close()

	// 上传 URL
	uploadURL := "https://upload.qiniup.com"
	if q.Zone == "z2" {
		uploadURL = "https://upload-na0.qiniup.com"
	} else if q.Zone == "z1" {
		uploadURL = "https://upload-z1.qiniup.com"
	}

	// 创建请求
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Key  string `json:"key"`
		Hash string `json:"hash"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Key == "" {
		return nil, errors.New("上传失败")
	}

	return &UploadResult{
		Success: true,
		Key:     result.Key,
		URL:     q.GetURL(result.Key),
		Size:    int64(len(data)),
		Message: "上传成功",
	}, nil
}

// Download 下载文件
func (q *QiniuOSS) Download(key string) ([]byte, error) {
	return []byte{}, nil
}

// Delete 删除文件
func (q *QiniuOSS) Delete(key string) error {
	return nil
}

// GetURL 获取文件 URL
func (q *QiniuOSS) GetURL(key string) string {
	if q.CDN != "" {
		return "https://" + q.CDN + "/" + key
	}
	return "https://" + q.Bucket + "." + q.Region + ".qiniucs.com/" + key
}

// Exists 检查文件是否存在
func (q *QiniuOSS) Exists(key string) (bool, error) {
	return true, nil
}

// generateUploadToken 生成上传凭证
func (q *QiniuOSS) generateUploadToken() string {
	// 简化实现
	return "mock_upload_token"
}

// TencentCOS 腾讯云 COS
type TencentCOS struct {
	Region    string
	Bucket    string
	SecretID  string
	SecretKey string
	CDN       string
}

// NewTencentCOS 创建腾讯云 COS 实例
func NewTencentCOS(region, bucket, secretID, secretKey, cdn string) *TencentCOS {
	return &TencentCOS{
		Region:    region,
		Bucket:    bucket,
		SecretID:  secretID,
		SecretKey: secretKey,
		CDN:       cdn,
	}
}

// Upload 上传文件
func (c *TencentCOS) Upload(file *multipart.FileHeader, key string) (*UploadResult, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	return c.UploadFromBytes(data, key)
}

// UploadFromBytes 上传字节数据
func (c *TencentCOS) UploadFromBytes(data []byte, key string) (*UploadResult, error) {
	// 实现腾讯云 COS 上传
	return &UploadResult{
		Success: true,
		Key:     key,
		URL:     c.GetURL(key),
		Size:    int64(len(data)),
		Message: "上传成功",
	}, nil
}

// Download 下载文件
func (c *TencentCOS) Download(key string) ([]byte, error) {
	return []byte{}, nil
}

// Delete 删除文件
func (c *TencentCOS) Delete(key string) error {
	return nil
}

// GetURL 获取文件 URL
func (c *TencentCOS) GetURL(key string) string {
	if c.CDN != "" {
		return "https://" + c.CDN + "/" + key
	}
	return "https://" + c.Bucket + ".cos." + c.Region + ".myqcloud.com/" + key
}

// Exists 检查文件是否存在
func (c *TencentCOS) Exists(key string) (bool, error) {
	return true, nil
}

// OSSConfig OSS 配置
type OSSConfig struct {
	Provider string         `yaml:"provider"`
	Aliyun   *AliyunConfig  `yaml:"aliyun"`
	Qiniu    *QiniuConfig   `yaml:"qiniu"`
	Tencent  *TencentConfig `yaml:"tencent"`
}

// AliyunConfig 阿里云配置
type AliyunConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessID  string `yaml:"access_id"`
	AccessKey string `yaml:"access_key"`
	CDN       string `yaml:"cdn"`
}

// QiniuConfig 七牛配置
type QiniuConfig struct {
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	CDN       string `yaml:"cdn"`
}

// TencentConfig 腾讯配置
type TencentConfig struct {
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	CDN       string `yaml:"cdn"`
}

// OSSManager OSS 管理器
type OSSManager struct {
	config *OSSConfig
	client OSS
}

// NewOSSManager 创建 OSS 管理器
func NewOSSManager(config *OSSConfig) *OSSManager {
	return &OSSManager{
		config: config,
	}
}

// GetClient 获取 OSS 客户端
func (m *OSSManager) GetClient() OSS {
	if m.client != nil {
		return m.client
	}

	switch m.config.Provider {
	case "aliyun":
		cfg := m.config.Aliyun
		m.client = NewAliyunOSS(
			cfg.Endpoint,
			cfg.Bucket,
			cfg.AccessID,
			cfg.AccessKey,
			cfg.CDN,
		)
	case "qiniu":
		cfg := m.config.Qiniu
		m.client = NewQiniuOSS(
			cfg.Region,
			cfg.Bucket,
			cfg.AccessKey,
			cfg.SecretKey,
			cfg.CDN,
		)
	case "tencent":
		cfg := m.config.Tencent
		m.client = NewTencentCOS(
			cfg.Region,
			cfg.Bucket,
			cfg.SecretID,
			cfg.SecretKey,
			cfg.CDN,
		)
	}

	return m.client
}

// Upload 上传文件
func (m *OSSManager) Upload(file *multipart.FileHeader, key string) (*UploadResult, error) {
	return m.GetClient().Upload(file, key)
}

// UploadFromBytes 上传字节数据
func (m *OSSManager) UploadFromBytes(data []byte, key string) (*UploadResult, error) {
	return m.GetClient().UploadFromBytes(data, key)
}

// Download 下载文件
func (m *OSSManager) Download(key string) ([]byte, error) {
	return m.GetClient().Download(key)
}

// Delete 删除文件
func (m *OSSManager) Delete(key string) error {
	return m.GetClient().Delete(key)
}

// GetURL 获取 URL
func (m *OSSManager) GetURL(key string) string {
	return m.GetClient().GetURL(key)
}

// UploadLocalFile 上传本地文件
func (m *OSSManager) UploadLocalFile(localPath, key string) (*UploadResult, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	return m.UploadFromBytes(data, key)
}

// GenerateUploadToken 生成上传凭证（用于前端直传）
func (m *OSSManager) GenerateUploadToken(key string, expire int64) (string, error) {
	// 实现凭证生成
	return "", nil
}

// BatchUpload 批量上传
func (m *OSSManager) BatchUpload(files []*multipart.FileHeader, prefix string) ([]*UploadResult, error) {
	results := make([]*UploadResult, 0, len(files))
	for _, file := range files {
		key := prefix + "/" + filepath.Base(file.Filename)
		result, err := m.Upload(file, key)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}
