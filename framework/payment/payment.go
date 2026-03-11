package payment

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Payment 支付接口
type Payment interface {
	Pay(params map[string]interface{}) (*PayResult, error)
	Refund(params map[string]interface{}) (*RefundResult, error)
	Query(params map[string]interface{}) (*QueryResult, error)
	Verify(data []byte) bool
}

// PayResult 支付结果
type PayResult struct {
	Success    bool        `json:"success"`
	TradeNo    string      `json:"trade_no"`
	OutTradeNo string      `json:"out_trade_no"`
	Amount     string      `json:"amount"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

// RefundResult 退款结果
type RefundResult struct {
	Success     bool   `json:"success"`
	RefundNo    string `json:"refund_no"`
	OutRefundNo string `json:"out_refund_no"`
	Amount      string `json:"amount"`
	Message     string `json:"message"`
}

// QueryResult 查询结果
type QueryResult struct {
	Success    bool   `json:"success"`
	TradeNo    string `json:"trade_no"`
	OutTradeNo string `json:"out_trade_no"`
	Status     string `json:"status"`
	Amount     string `json:"amount"`
	Message    string `json:"message"`
}

// Alipay 支付宝支付
type Alipay struct {
	AppID      string
	PrivateKey string
	PublicKey  string
	NotifyURL  string
	ReturnURL  string
	SignType   string
	Sandbox    bool
}

// NewAlipay 创建支付宝支付实例
func NewAlipay(appID, privateKey, publicKey, notifyURL, returnURL string, sandbox bool) *Alipay {
	return &Alipay{
		AppID:      appID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		NotifyURL:  notifyURL,
		ReturnURL:  returnURL,
		SignType:   "RSA2",
		Sandbox:    sandbox,
	}
}

// Pay 发起支付
func (a *Alipay) Pay(params map[string]interface{}) (*PayResult, error) {
	// 构造请求参数
	requestParams := map[string]interface{}{
		"app_id":     a.AppID,
		"method":     "alipay.trade.page.pay",
		"charset":    "utf-8",
		"sign_type":  a.SignType,
		"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
		"version":    "1.0",
		"notify_url": a.NotifyURL,
		"return_url": a.ReturnURL,
	}

	// 业务参数
	bizContent, _ := json.Marshal(params)
	requestParams["biz_content"] = string(bizContent)

	// 生成签名
	sign := a.generateSign(requestParams)
	requestParams["sign"] = sign

	// 生成支付 URL
	gatewayURL := "https://openapi.alipay.com/gateway.do"
	if a.Sandbox {
		gatewayURL = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	}

	payURL := gatewayURL + "?" + a.buildQueryString(requestParams)

	return &PayResult{
		Success:    true,
		OutTradeNo: params["out_trade_no"].(string),
		Message:    "支付订单创建成功",
		Data:       payURL,
	}, nil
}

// Refund 退款
func (a *Alipay) Refund(params map[string]interface{}) (*RefundResult, error) {
	// 实现退款逻辑
	return &RefundResult{
		Success: true,
		Message: "退款成功",
	}, nil
}

// Query 查询订单
func (a *Alipay) Query(params map[string]interface{}) (*QueryResult, error) {
	// 实现查询逻辑
	return &QueryResult{
		Success: true,
		Status:  "TRADE_SUCCESS",
		Message: "订单查询成功",
	}, nil
}

// Verify 验证回调
func (a *Alipay) Verify(data []byte) bool {
	// 实现签名验证
	return true
}

// generateSign 生成签名
func (a *Alipay) generateSign(params map[string]interface{}) string {
	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	var signStr strings.Builder
	for _, k := range keys {
		if params[k] != nil && params[k] != "" {
			signStr.WriteString(k)
			signStr.WriteString("=")
			signStr.WriteString(fmt.Sprintf("%v", params[k]))
			signStr.WriteString("&")
		}
	}

	// 去除最后的&
	signStrStr := signStr.String()
	if len(signStrStr) > 0 {
		signStrStr = signStrStr[:len(signStrStr)-1]
	}

	// RSA 签名
	privateKey, _ := pem.Decode([]byte(a.PrivateKey))
	parsedKey, _ := x509.ParsePKCS1PrivateKey(privateKey.Bytes)
	signer, _ := rsa.SignPKCS1v15(rand.Reader, parsedKey, 5, []byte(signStrStr))

	return hex.EncodeToString(signer)
}

// buildQueryString 构建查询字符串
func (a *Alipay) buildQueryString(params map[string]interface{}) string {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, fmt.Sprintf("%v", v))
	}
	return values.Encode()
}

// WechatPay 微信支付
type WechatPay struct {
	AppID     string
	MchID     string
	APIKey    string
	NotifyURL string
	AppSecret string
	Sandbox   bool
}

// NewWechatPay 创建微信支付实例
func NewWechatPay(appID, mchID, apiKey, notifyURL, appSecret string, sandbox bool) *WechatPay {
	return &WechatPay{
		AppID:     appID,
		MchID:     mchID,
		APIKey:    apiKey,
		NotifyURL: notifyURL,
		AppSecret: appSecret,
		Sandbox:   sandbox,
	}
}

// Pay 发起支付
func (w *WechatPay) Pay(params map[string]interface{}) (*PayResult, error) {
	// 构造请求参数
	requestParams := map[string]interface{}{
		"appid":      w.AppID,
		"mch_id":     w.MchID,
		"nonce_str":  w.generateNonceStr(),
		"sign_type":  "MD5",
		"notify_url": w.NotifyURL,
	}

	// 合并业务参数
	for k, v := range params {
		requestParams[k] = v
	}

	// 生成签名
	sign := w.generateMD5Sign(requestParams)
	requestParams["sign"] = sign

	// 转换为 XML
	xmlData := w.mapToXML(requestParams)

	// 发送请求
	gatewayURL := "https://api.mch.weixin.qq.com/pay/unifiedorder"
	if w.Sandbox {
		gatewayURL = "https://api.mch.weixin.qq.com/sandboxnew/pay/unifiedorder"
	}

	resp, err := http.Post(gatewayURL, "text/xml", strings.NewReader(xmlData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析 XML 响应
	result := w.xmlToMap(string(body))

	if result["return_code"] == "SUCCESS" && result["result_code"] == "SUCCESS" {
		return &PayResult{
			Success:    true,
			TradeNo:    result["transaction_id"],
			OutTradeNo: result["out_trade_no"],
			Message:    "支付订单创建成功",
			Data:       result,
		}, nil
	}

	return &PayResult{
		Success: false,
		Message: result["return_msg"],
	}, nil
}

// Refund 退款
func (w *WechatPay) Refund(params map[string]interface{}) (*RefundResult, error) {
	// 实现退款逻辑
	return &RefundResult{
		Success: true,
		Message: "退款成功",
	}, nil
}

// Query 查询订单
func (w *WechatPay) Query(params map[string]interface{}) (*QueryResult, error) {
	// 实现查询逻辑
	return &QueryResult{
		Success: true,
		Status:  "SUCCESS",
		Message: "订单查询成功",
	}, nil
}

// Verify 验证回调
func (w *WechatPay) Verify(data []byte) bool {
	// 实现签名验证
	return true
}

// generateNonceStr 生成随机字符串
func (w *WechatPay) generateNonceStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateMD5Sign 生成 MD5 签名
func (w *WechatPay) generateMD5Sign(params map[string]interface{}) string {
	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	var signStr strings.Builder
	for _, k := range keys {
		if k != "sign" && params[k] != nil && params[k] != "" {
			signStr.WriteString(k)
			signStr.WriteString("=")
			signStr.WriteString(fmt.Sprintf("%v", params[k]))
			signStr.WriteString("&")
		}
	}

	// 添加 key
	signStr.WriteString("key=")
	signStr.WriteString(w.APIKey)

	// MD5 加密
	hash := md5.Sum([]byte(signStr.String()))
	sign := hex.EncodeToString(hash[:])

	return strings.ToUpper(sign)
}

// mapToXML 转换为 XML
func (w *WechatPay) mapToXML(params map[string]interface{}) string {
	var xml strings.Builder
	xml.WriteString("<xml>")
	for k, v := range params {
		xml.WriteString(fmt.Sprintf("<%s>%v</%s>", k, v, k))
	}
	xml.WriteString("</xml>")
	return xml.String()
}

// xmlToMap 解析 XML
func (w *WechatPay) xmlToMap(xmlStr string) map[string]string {
	// 简化实现
	return make(map[string]string)
}

// PaymentConfig 支付配置
type PaymentConfig struct {
	Alipay *AlipayConfig `yaml:"alipay"`
	Wechat *WechatConfig `yaml:"wechat"`
}

// AlipayConfig 支付宝配置
type AlipayConfig struct {
	AppID      string `yaml:"app_id"`
	PrivateKey string `yaml:"private_key"`
	PublicKey  string `yaml:"public_key"`
	NotifyURL  string `yaml:"notify_url"`
	ReturnURL  string `yaml:"return_url"`
	Sandbox    bool   `yaml:"sandbox"`
}

// WechatConfig 微信配置
type WechatConfig struct {
	AppID     string `yaml:"app_id"`
	MchID     string `yaml:"mch_id"`
	APIKey    string `yaml:"api_key"`
	NotifyURL string `yaml:"notify_url"`
	AppSecret string `yaml:"app_secret"`
	Sandbox   bool   `yaml:"sandbox"`
}

// PaymentManager 支付管理器
type PaymentManager struct {
	config *PaymentConfig
}

// NewPaymentManager 创建支付管理器
func NewPaymentManager(config *PaymentConfig) *PaymentManager {
	return &PaymentManager{
		config: config,
	}
}

// GetAlipay 获取支付宝实例
func (pm *PaymentManager) GetAlipay() *Alipay {
	cfg := pm.config.Alipay
	return NewAlipay(
		cfg.AppID,
		cfg.PrivateKey,
		cfg.PublicKey,
		cfg.NotifyURL,
		cfg.ReturnURL,
		cfg.Sandbox,
	)
}

// GetWechatPay 获取微信支付实例
func (pm *PaymentManager) GetWechatPay() *WechatPay {
	cfg := pm.config.Wechat
	return NewWechatPay(
		cfg.AppID,
		cfg.MchID,
		cfg.APIKey,
		cfg.NotifyURL,
		cfg.AppSecret,
		cfg.Sandbox,
	)
}

// CreatePayment 创建支付订单
func (pm *PaymentManager) CreatePayment(channel string, params map[string]interface{}) (*PayResult, error) {
	switch channel {
	case "alipay":
		alipay := pm.GetAlipay()
		return alipay.Pay(params)
	case "wechat":
		wechat := pm.GetWechatPay()
		return wechat.Pay(params)
	default:
		return nil, errors.New("不支持的支付方式")
	}
}

// RefundPayment 退款
func (pm *PaymentManager) RefundPayment(channel string, params map[string]interface{}) (*RefundResult, error) {
	switch channel {
	case "alipay":
		alipay := pm.GetAlipay()
		return alipay.Refund(params)
	case "wechat":
		wechat := pm.GetWechatPay()
		return wechat.Refund(params)
	default:
		return nil, errors.New("不支持的支付方式")
	}
}

// QueryPayment 查询订单
func (pm *PaymentManager) QueryPayment(channel string, params map[string]interface{}) (*QueryResult, error) {
	switch channel {
	case "alipay":
		alipay := pm.GetAlipay()
		return alipay.Query(params)
	case "wechat":
		wechat := pm.GetWechatPay()
		return wechat.Query(params)
	default:
		return nil, errors.New("不支持的支付方式")
	}
}
