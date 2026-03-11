package upload

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vigo/framework/mvc"
)

// FileUploader 文件上传处理器
type FileUploader struct {
	uploadDir      string   // 上传目录
	maxSize        int64    // 最大文件大小（字节）
	allowedExts    []string // 允许的扩展名
	dangerousMIMEs []string // 危险的 MIME 类型
	dangerousChars []string // 危险字符
}

// UploadResult 上传结果
type UploadResult struct {
	Filename    string `json:"filename"`     // 原始文件名
	SavePath    string `json:"save_path"`    // 保存路径
	URL         string `json:"url"`          // 访问 URL
	Size        int64  `json:"size"`         // 文件大小
	Hash        string `json:"hash"`         // 文件哈希
	ContentType string `json:"content_type"` // MIME 类型
}

// NewUploader 创建文件上传处理器
func NewUploader(uploadDir string, maxSize int64, allowedExts []string) *FileUploader {
	return &FileUploader{
		uploadDir:   uploadDir,
		maxSize:     maxSize,
		allowedExts: allowedExts,
		dangerousMIMEs: []string{
			"application/x-msdownload",
			"application/x-executable",
			"text/x-php",
			"text/x-python",
			"application/x-perl",
			"application/x-sh",
			"application/x-shellscript",
			"application/x-php",
			"application/php",
			"text/x-php",
		},
		dangerousChars: []string{"<", ">", ":", "\"", "|", "?", "*", "&", ";", "$", "`", "\\", "/"},
	}
}

// ValidateAndUpload 验证并上传文件
// 用法：result, err := uploader.ValidateAndUpload(c, "file")
func (u *FileUploader) ValidateAndUpload(c *mvc.Context, fieldName string) (*UploadResult, error) {
	// 获取文件
	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("获取文件失败：%v", err)
	}
	defer file.Close()

	// 验证文件
	if err := u.validateFile(header); err != nil {
		return nil, err
	}

	// 生成安全的文件名
	safeName := u.generateSafeFilename(header.Filename)

	// 创建上传目录
	if err := os.MkdirAll(u.uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败：%v", err)
	}

	// 保存文件路径
	savePath := filepath.Join(u.uploadDir, safeName)

	// 创建文件
	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败：%v", err)
	}
	defer dst.Close()

	// 复制文件内容
	written, err := io.Copy(dst, file)
	if err != nil {
		return nil, fmt.Errorf("保存文件失败：%v", err)
	}

	// 计算文件哈希
	hash, err := u.calculateFileHash(file)
	if err != nil {
		return nil, fmt.Errorf("计算哈希失败：%v", err)
	}

	// 返回结果
	return &UploadResult{
		Filename:    header.Filename,
		SavePath:    savePath,
		URL:         "/uploads/" + safeName,
		Size:        written,
		Hash:        hash,
		ContentType: header.Header.Get("Content-Type"),
	}, nil
}

// validateFile 验证文件安全性
func (u *FileUploader) validateFile(header *multipart.FileHeader) error {
	// 1. 验证文件大小
	if header.Size > u.maxSize {
		return fmt.Errorf("文件大小超过限制 (%.2f MB)", float64(u.maxSize)/1024/1024)
	}

	// 2. 验证文件扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := false
	for _, allowedExt := range u.allowedExts {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("不支持的文件类型：%s", ext)
	}

	// 3. 验证 MIME 类型
	fileType := header.Header.Get("Content-Type")
	if fileType != "" {
		for _, dangerous := range u.dangerousMIMEs {
			if fileType == dangerous {
				return fmt.Errorf("危险的文件类型：%s", fileType)
			}
		}
	}

	// 4. 验证文件名（防止路径穿越）
	filename := header.Filename
	if strings.Contains(filename, "..") {
		return fmt.Errorf("非法的文件名：包含路径穿越字符")
	}

	// 5. 验证文件名不包含危险字符
	for _, char := range u.dangerousChars {
		if strings.Contains(filename, char) {
			return fmt.Errorf("文件名包含非法字符：%s", char)
		}
	}

	// 6. 文件名长度限制
	if len(filename) > 255 {
		return fmt.Errorf("文件名过长")
	}

	return nil
}

// generateSafeFilename 生成安全的文件名
func (u *FileUploader) generateSafeFilename(originalName string) string {
	// 获取扩展名
	ext := strings.ToLower(filepath.Ext(originalName))

	// 生成时间戳 + 随机字符串
	timestamp := time.Now().Format("20060102150405")
	random := fmt.Sprintf("%d", time.Now().UnixNano())

	// 生成哈希
	hash := sha256.Sum256([]byte(timestamp + random + originalName))
	hashStr := hex.EncodeToString(hash[:8])

	// 组合：时间戳_哈希。扩展名
	safeName := fmt.Sprintf("%s_%s%s", timestamp, hashStr, ext)

	return safeName
}

// calculateFileHash 计算文件 SHA256 哈希
func (u *FileUploader) calculateFileHash(file multipart.File) (string, error) {
	// 重置文件指针
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	defer file.Seek(0, 0)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ValidateOnly 仅验证文件（不上传）
// 用法：err := uploader.ValidateOnly(c, "file")
func (u *FileUploader) ValidateOnly(c *mvc.Context, fieldName string) error {
	_, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return fmt.Errorf("获取文件失败：%v", err)
	}

	return u.validateFile(header)
}

// UploadMultiple 上传多个文件
// 用法：results, err := uploader.UploadMultiple(c, "files")
func (u *FileUploader) UploadMultiple(c *mvc.Context, fieldName string) ([]*UploadResult, error) {
	// 获取所有文件
	form, err := c.Request.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("读取表单失败：%v", err)
	}

	var results []*UploadResult

	for {
		part, err := form.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取文件失败：%v", err)
		}

		// 跳过非文件字段
		if part.FileName() == "" {
			continue
		}

		// 只处理指定字段名的文件
		if part.FormName() != fieldName {
			continue
		}

		// 读取文件内容到临时缓冲区
		fileContent, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败：%v", err)
		}

		// 创建临时 FileHeader
		header := &multipart.FileHeader{
			Filename: part.FileName(),
			Size:     int64(len(fileContent)),
			Header:   part.Header,
		}

		// 验证文件
		if err := u.validateFile(header); err != nil {
			return nil, err
		}

		// 生成安全的文件名
		safeName := u.generateSafeFilename(header.Filename)

		// 保存文件
		savePath := filepath.Join(u.uploadDir, safeName)
		dst, err := os.Create(savePath)
		if err != nil {
			return nil, fmt.Errorf("创建文件失败：%v", err)
		}

		written, err := dst.Write(fileContent)
		dst.Close()
		if err != nil {
			os.Remove(savePath)
			return nil, fmt.Errorf("保存文件失败：%v", err)
		}

		// 计算哈希
		hash := sha256.Sum256(fileContent)
		hashStr := hex.EncodeToString(hash[:])

		results = append(results, &UploadResult{
			Filename:    header.Filename,
			SavePath:    savePath,
			URL:         "/uploads/" + safeName,
			Size:        int64(written),
			Hash:        hashStr,
			ContentType: part.Header.Get("Content-Type"),
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("没有找到上传的文件")
	}

	return results, nil
}

// ==================== 助手函数 ====================

// Upload 快速上传单个文件
// 用法：result, err := upload.Upload(c, "file", "./uploads", 5*1024*1024, []string{".jpg", ".png"})
func Upload(c *mvc.Context, fieldName string, uploadDir string, maxSize int64, allowedExts []string) (*UploadResult, error) {
	uploader := NewUploader(uploadDir, maxSize, allowedExts)
	return uploader.ValidateAndUpload(c, fieldName)
}

// UploadMultipleFiles 快速上传多个文件
func UploadMultipleFiles(c *mvc.Context, fieldName string, uploadDir string, maxSize int64, allowedExts []string) ([]*UploadResult, error) {
	uploader := NewUploader(uploadDir, maxSize, allowedExts)
	return uploader.UploadMultiple(c, fieldName)
}

// ValidateFile 快速验证文件（不上传）
func ValidateFile(c *mvc.Context, fieldName string, maxSize int64, allowedExts []string) error {
	uploader := NewUploader("", maxSize, allowedExts)
	return uploader.ValidateOnly(c, fieldName)
}
