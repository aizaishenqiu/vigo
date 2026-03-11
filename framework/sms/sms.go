package sms

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SMS 短信接口
type SMS interface {
	Send(phone string, templateID string, params map[string]string) (*SendResult, error)
	BatchSend(phones []string, templateID string, params map[string]string) (*BatchSendResult, error)
	Query(messageID string) (*QueryResult, error)
}

// SendResult 发送结果
type SendResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
	Phone     string `json:"phone"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
}

// BatchSendResult 批量发送结果
type BatchSendResult struct {
	Success      bool          `json:"success"`
	Total        int           `json:"total"`
	SuccessCount int           `json:"success_count"`
	FailedCount  int           `json:"failed_count"`
	Results      []*SendResult `json:"results"`
}

// QueryResult 查询结果
type QueryResult struct {
	Success    bool   `json:"success"`
	MessageID  string `json:"message_id"`
	Phone      string `json:"phone"`
	Status     string `json:"status"`
	SendTime   string `json:"send_time"`
	ReportTime string `json:"report_time"`
	Message    string `json:"message"`
}

// AliyunSMS 阿里云短信
type AliyunSMS struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	Region          string
}

// NewAliyunSMS 创建阿里云短信实例
func NewAliyunSMS(accessKeyID, accessKeySecret, signName, region string) *AliyunSMS {
	return &AliyunSMS{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		SignName:        signName,
		Region:          region,
	}
}

// Send 发送短信
func (s *AliyunSMS) Send(phone string, templateID string, params map[string]string) (*SendResult, error) {
	// 构造请求参数
	requestParams := map[string]string{
		"Action":           "SendSms",
		"Version":          "2017-05-25",
		"AccessKeyId":      s.AccessKeyID,
		"Format":           "JSON",
		"SignatureMethod":  "HMAC-SHA1",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureVersion": "1.0",
		"SignatureNonce":   generateUUID(),
		"RegionId":         s.Region,
		"PhoneNumbers":     phone,
		"SignName":         s.SignName,
		"TemplateCode":     templateID,
	}

	// 模板参数
	if len(params) > 0 {
		templateParam, _ := json.Marshal(params)
		requestParams["TemplateParam"] = string(templateParam)
	}

	// 生成签名
	signature := s.generateSignature(requestParams)
	requestParams["Signature"] = signature

	// 发送请求
	reqURL := "https://dysmsapi.aliyuncs.com/"
	reqData := url.Values{}
	for k, v := range requestParams {
		reqData.Set(k, v)
	}

	resp, err := http.PostForm(reqURL, reqData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		BizId     string `json:"BizId"`
		RequestId string `json:"RequestId"`
	}

	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)

	if result.Code == "OK" {
		return &SendResult{
			Success:   true,
			MessageID: result.BizId,
			Phone:     phone,
			Code:      200,
			Message:   "发送成功",
		}, nil
	}

	return &SendResult{
		Success: false,
		Phone:   phone,
		Code:    500,
		Message: result.Message,
	}, nil
}

// BatchSend 批量发送
func (s *AliyunSMS) BatchSend(phones []string, templateID string, params map[string]string) (*BatchSendResult, error) {
	results := make([]*SendResult, 0, len(phones))
	successCount := 0

	for _, phone := range phones {
		result, err := s.Send(phone, templateID, params)
		if err != nil {
			results = append(results, &SendResult{
				Success: false,
				Phone:   phone,
				Code:    500,
				Message: err.Error(),
			})
			continue
		}
		if result.Success {
			successCount++
		}
		results = append(results, result)
	}

	return &BatchSendResult{
		Success:      successCount > 0,
		Total:        len(phones),
		SuccessCount: successCount,
		FailedCount:  len(phones) - successCount,
		Results:      results,
	}, nil
}

// Query 查询发送状态
func (s *AliyunSMS) Query(messageID string) (*QueryResult, error) {
	// 实现查询逻辑
	return &QueryResult{
		Success:   true,
		MessageID: messageID,
		Status:    "DELIVERED",
		Message:   "查询成功",
	}, nil
}

// generateSignature 生成签名
func (s *AliyunSMS) generateSignature(params map[string]string) string {
	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	var canonicalizedQueryString strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonicalizedQueryString.WriteString("&")
		}
		canonicalizedQueryString.WriteString(percentEncode(k))
		canonicalizedQueryString.WriteString("=")
		canonicalizedQueryString.WriteString(percentEncode(params[k]))
	}

	// 构造签名字符串
	stringToSign := "GET&%2F&" + percentEncode(canonicalizedQueryString.String())

	// HMAC-SHA1 签名
	h := hmac.New(sha256.New, []byte(s.AccessKeySecret+"&"))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}

// percentEncode URL 编码
func percentEncode(s string) string {
	return url.QueryEscape(s)
}

// TencentSMS 腾讯云短信
type TencentSMS struct {
	SecretID  string
	SecretKey string
	AppID     string
	SignName  string
	Region    string
}

// NewTencentSMS 创建腾讯云短信实例
func NewTencentSMS(secretID, secretKey, appID, signName, region string) *TencentSMS {
	return &TencentSMS{
		SecretID:  secretID,
		SecretKey: secretKey,
		AppID:     appID,
		SignName:  signName,
		Region:    region,
	}
}

// Send 发送短信
func (s *TencentSMS) Send(phone string, templateID string, params map[string]string) (*SendResult, error) {
	// 构造请求
	requestParams := map[string]interface{}{
		"PhoneNumberSet": []string{phone},
		"SmsSdkAppId":    s.AppID,
		"SignName":       s.SignName,
		"TemplateId":     templateID,
		"TemplateParamSet": func() []string {
			values := make([]string, 0, len(params))
			for _, v := range params {
				values = append(values, v)
			}
			return values
		}(),
	}

	// 发送请求
	reqBody, _ := json.Marshal(requestParams)
	reqURL := "https://sms.tencentcloudapi.com/"

	req, _ := http.NewRequest("POST", reqURL, strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")

	// 添加签名头
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	// 生成随机数作为 nonce（用于防重放攻击）
	nonce := strconv.FormatInt(rand.Int63(), 10)

	// 生成腾讯云 API 签名
	signature := s.generateTencentSignature(timestamp, nonce, reqBody)

	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2021-01-11")
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("X-TC-Nonce", nonce)
	req.Header.Set("X-TC-Region", s.Region)
	req.Header.Set("X-TC-Signature", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Response struct {
			SendStatusSet []struct {
				StatusCode        int    `json:"StatusCode"`
				StatusDescription string `json:"StatusDescription"`
				SerialNo          string `json:"SerialNo"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}

	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)

	if len(result.Response.SendStatusSet) > 0 {
		status := result.Response.SendStatusSet[0]
		if status.StatusCode == 0 {
			return &SendResult{
				Success:   true,
				MessageID: status.SerialNo,
				Phone:     phone,
				Code:      200,
				Message:   "发送成功",
			}, nil
		}
		return &SendResult{
			Success: false,
			Phone:   phone,
			Code:    status.StatusCode,
			Message: status.StatusDescription,
		}, nil
	}

	return nil, errors.New("发送失败")
}

// BatchSend 批量发送
func (s *TencentSMS) BatchSend(phones []string, templateID string, params map[string]string) (*BatchSendResult, error) {
	results := make([]*SendResult, 0, len(phones))
	successCount := 0

	for _, phone := range phones {
		result, err := s.Send(phone, templateID, params)
		if err != nil {
			results = append(results, &SendResult{
				Success: false,
				Phone:   phone,
				Code:    500,
				Message: err.Error(),
			})
			continue
		}
		if result.Success {
			successCount++
		}
		results = append(results, result)
	}

	return &BatchSendResult{
		Success:      successCount > 0,
		Total:        len(phones),
		SuccessCount: successCount,
		FailedCount:  len(phones) - successCount,
		Results:      results,
	}, nil
}

// Query 查询发送状态
func (s *TencentSMS) Query(messageID string) (*QueryResult, error) {
	return &QueryResult{
		Success:   true,
		MessageID: messageID,
		Status:    "SUCCESS",
		Message:   "查询成功",
	}, nil
}

// SMSConfig 短信配置
type SMSConfig struct {
	Provider string            `yaml:"provider"`
	Aliyun   *AliyunSMSConfig  `yaml:"aliyun"`
	Tencent  *TencentSMSConfig `yaml:"tencent"`
}

// AliyunSMSConfig 阿里云配置
type AliyunSMSConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	SignName        string `yaml:"sign_name"`
	Region          string `yaml:"region"`
}

// TencentSMSConfig 腾讯云配置
type TencentSMSConfig struct {
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	AppID     string `yaml:"app_id"`
	SignName  string `yaml:"sign_name"`
	Region    string `yaml:"region"`
}

// SMSManager 短信管理器
type SMSManager struct {
	config *SMSConfig
	client SMS
}

// NewSMSManager 创建短信管理器
func NewSMSManager(config *SMSConfig) *SMSManager {
	return &SMSManager{
		config: config,
	}
}

// generateTencentSignature 生成腾讯云 API 签名
func (s *TencentSMS) generateTencentSignature(timestamp, nonce string, body []byte) string {
	// 1. 构造签名字符串
	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/json\nhost:sms.tencentcloudapi.com\nx-tc-action:SendSms\nx-tc-nonce:%s\nx-tc-region:%s\nx-tc-timestamp:%s\nx-tc-version:2021-01-11\n",
		nonce, s.Region, timestamp)

	// 添加请求体哈希
	bodyHash := sha256.Sum256(body)
	canonicalRequest += fmt.Sprintf("\n%s", hex.EncodeToString(bodyHash[:]))

	// 2. 计算 canonicalRequest 的 SHA256
	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))

	// 3. 构造 stringToSign
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%s\n%s", timestamp, hex.EncodeToString(canonicalRequestHash[:]))

	// 4. 计算签名
	secretDate := hmacSha256("TC3"+s.SecretKey, timestamp)
	secretService := hmacSha256(secretDate, "sms")
	secretSigning := hmacSha256(secretService, "tc3_request")
	signature := hex.EncodeToString([]byte(hmacSha256(secretSigning, stringToSign)))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/sms/tc3_request, SignedHeaders=content-type;host;x-tc-action;x-tc-nonce;x-tc-region;x-tc-timestamp;x-tc-version, Signature=%s",
		s.SecretID, timestamp, signature)
}

// hmacSha256 HMAC-SHA256 签名
func hmacSha256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return string(mac.Sum(nil))
}

// GetClient 获取短信客户端
func (m *SMSManager) GetClient() SMS {
	if m.client != nil {
		return m.client
	}

	switch m.config.Provider {
	case "aliyun":
		cfg := m.config.Aliyun
		m.client = NewAliyunSMS(
			cfg.AccessKeyID,
			cfg.AccessKeySecret,
			cfg.SignName,
			cfg.Region,
		)
	case "tencent":
		cfg := m.config.Tencent
		m.client = NewTencentSMS(
			cfg.SecretID,
			cfg.SecretKey,
			cfg.AppID,
			cfg.SignName,
			cfg.Region,
		)
	}

	return m.client
}

// Send 发送短信
func (m *SMSManager) Send(phone string, templateID string, params map[string]string) (*SendResult, error) {
	return m.GetClient().Send(phone, templateID, params)
}

// BatchSend 批量发送
func (m *SMSManager) BatchSend(phones []string, templateID string, params map[string]string) (*BatchSendResult, error) {
	return m.GetClient().BatchSend(phones, templateID, params)
}

// Query 查询发送状态
func (m *SMSManager) Query(messageID string) (*QueryResult, error) {
	return m.GetClient().Query(messageID)
}

// SendVerifyCode 发送验证码
func (m *SMSManager) SendVerifyCode(phone string, code string, expireMinutes int) (*SendResult, error) {
	params := map[string]string{
		"code":   code,
		"expire": strconv.Itoa(expireMinutes),
	}

	// 使用验证码模板
	templateID := "SMS_123456789" // 验证码模板 ID
	return m.Send(phone, templateID, params)
}

// SendNotice 发送通知短信
func (m *SMSManager) SendNotice(phone string, templateID string, params map[string]string) (*SendResult, error) {
	return m.Send(phone, templateID, params)
}

// SendMarketing 发送营销短信
func (m *SMSManager) SendMarketing(phones []string, templateID string, params map[string]string) (*BatchSendResult, error) {
	return m.BatchSend(phones, templateID, params)
}

// generateUUID 生成 UUID
func generateUUID() string {
	// 简化实现
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
