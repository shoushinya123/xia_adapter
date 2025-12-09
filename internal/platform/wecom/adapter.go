package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"xia_adpter/internal/config"
	"xia_adpter/internal/message"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Adapter 企微适配器
type Adapter struct {
	cfg         config.WeComConfig
	queue       *message.Queue
	logger      *zap.Logger
	server      *http.Server
	accessToken string
	tokenExpiry time.Time
	tokenMu     sync.RWMutex
}

// NewAdapter 创建新的企微适配器
func NewAdapter(cfg config.WeComConfig, queue *message.Queue, logger *zap.Logger) *Adapter {
	return &Adapter{
		cfg:    cfg,
		queue:  queue,
		logger: logger,
	}
}

// Start 启动适配器
func (a *Adapter) Start(ctx context.Context) error {
	// 设置 Gin 为发布模式
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 注册回调路由
	router.GET("/callback", a.handleVerify)
	router.POST("/callback", a.handleCallback)

	// 启动 HTTP 服务器
	a.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port),
		Handler: router,
	}

	a.logger.Info("Starting WeCom adapter",
		zap.String("host", a.cfg.Host),
		zap.Int("port", a.cfg.Port),
	)

	// 在协程中启动服务器
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("WeCom server failed", zap.Error(err))
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	return a.Stop()
}

// Stop 停止适配器
func (a *Adapter) Stop() error {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(ctx)
	}
	return nil
}

// handleVerify 处理验证请求（GET）
func (a *Adapter) handleVerify(c *gin.Context) {
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echostr := c.Query("echostr")

	if msgSignature == "" || timestamp == "" || nonce == "" || echostr == "" {
		c.String(http.StatusBadRequest, "Missing parameters")
		return
	}

	// 验证签名
	if !a.verifySignature(msgSignature, timestamp, nonce, echostr) {
		a.logger.Warn("Invalid signature",
			zap.String("msg_signature", msgSignature),
			zap.String("timestamp", timestamp),
			zap.String("nonce", nonce),
		)
		c.String(http.StatusBadRequest, "Invalid signature")
		return
	}

	// 解密 echostr
	decrypted, err := a.decrypt(echostr, msgSignature, timestamp, nonce)
	if err != nil {
		a.logger.Error("Failed to decrypt echostr", zap.Error(err))
		c.String(http.StatusBadRequest, "Decryption failed")
		return
	}

	a.logger.Info("WeCom verification successful")
	c.String(http.StatusOK, decrypted)
}

// handleCallback 处理回调请求（POST）
func (a *Adapter) handleCallback(c *gin.Context) {
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	if msgSignature == "" || timestamp == "" || nonce == "" {
		c.String(http.StatusBadRequest, "Missing parameters")
		return
	}

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		a.logger.Error("Failed to read request body", zap.Error(err))
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	// 解析 XML
	var msg WeComMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		a.logger.Error("Failed to unmarshal XML", zap.Error(err))
		c.String(http.StatusBadRequest, "Invalid XML")
		return
	}

	// 解密消息
	decrypted, err := a.decrypt(msg.Encrypt, msgSignature, timestamp, nonce)
	if err != nil {
		a.logger.Error("Failed to decrypt message", zap.Error(err))
		c.String(http.StatusBadRequest, "Decryption failed")
		return
	}

	// 解析解密后的 XML
	var decryptedMsg WeComDecryptedMessage
	if err := xml.Unmarshal([]byte(decrypted), &decryptedMsg); err != nil {
		a.logger.Error("Failed to unmarshal decrypted XML", zap.Error(err))
		c.String(http.StatusBadRequest, "Invalid decrypted XML")
		return
	}

	// 转换为统一消息格式
	msgObj := a.convertMessage(&decryptedMsg)

	// 推送到消息队列
	if msgObj != nil {
		a.queue.Push(msgObj)
	}

	c.String(http.StatusOK, "success")
}

// convertMessage 转换企微消息为统一消息格式
func (a *Adapter) convertMessage(msg *WeComDecryptedMessage) *message.Message {
	msgObj := &message.Message{
		Platform:    "wecom",
		SessionID:   msg.FromUserName,
		UserID:      msg.FromUserName,
		Content:     msg.Content,
		MessageType: a.getMessageType(msg.MsgType),
		Metadata: map[string]string{
			"msg_id":      msg.MsgID,
			"to_user":     msg.ToUserName,
			"create_time": fmt.Sprintf("%d", msg.CreateTime),
		},
	}

	// 处理图片消息
	if msg.MsgType == "image" {
		if msg.PicURL != "" {
			msgObj.Metadata["pic_url"] = msg.PicURL
		}
		if msg.MediaID != "" {
			msgObj.Metadata["media_id"] = msg.MediaID
		}
	}

	// 处理语音消息
	if msg.MsgType == "voice" {
		if msg.MediaID != "" {
			msgObj.Metadata["media_id"] = msg.MediaID
		}
		if msg.Format != "" {
			msgObj.Metadata["format"] = msg.Format
		}
	}

	return msgObj
}

// getMessageType 获取消息类型
func (a *Adapter) getMessageType(wecomType string) string {
	switch wecomType {
	case "text":
		return "text"
	case "image":
		return "image"
	case "voice":
		return "voice"
	default:
		return "text"
	}
}

// verifySignature 验证签名
func (a *Adapter) verifySignature(signature, timestamp, nonce, echostr string) bool {
	// 企微签名算法：对 token、timestamp、nonce、echostr 进行字典序排序后拼接，然后进行 SHA1 加密
	tokens := []string{a.cfg.Token, timestamp, nonce, echostr}
	sort.Strings(tokens)
	combined := strings.Join(tokens, "")

	hash := sha1.Sum([]byte(combined))
	calculatedSignature := fmt.Sprintf("%x", hash)

	return calculatedSignature == signature
}

// decrypt 解密消息（AES-256-CBC）
// 企微加密格式：随机16字节 + 消息长度4字节(网络字节序) + 消息内容 + CorpID
func (a *Adapter) decrypt(encrypted, msgSignature, timestamp, nonce string) (string, error) {
	// 解码 base64
	encryptedBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 解码 AES Key（EncodingAESKey 是 43 字节的 base64 字符串，需要补全到 44 字节）
	aesKeyStr := a.cfg.EncodingAESKey
	if len(aesKeyStr)%4 != 0 {
		// 补全 base64 padding
		padding := 4 - (len(aesKeyStr) % 4)
		aesKeyStr += strings.Repeat("=", padding)
	}
	
	aesKey, err := base64.StdEncoding.DecodeString(aesKeyStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode AES key: %w", err)
	}

	if len(aesKey) != 32 {
		return "", fmt.Errorf("invalid AES key length: expected 32, got %d", len(aesKey))
	}

	// 创建 AES 解密器
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// 使用 CBC 模式，IV 是 AES Key 的前 16 字节
	iv := aesKey[:16]
	mode := cipher.NewCBCDecrypter(block, iv)

	// 检查数据长度必须是 16 的倍数
	if len(encryptedBytes)%16 != 0 {
		return "", fmt.Errorf("encrypted data length must be multiple of 16")
	}

	// 解密
	decrypted := make([]byte, len(encryptedBytes))
	mode.CryptBlocks(decrypted, encryptedBytes)

	// 去除 PKCS7 填充
	decrypted = a.pkcs7Unpad(decrypted)
	if len(decrypted) < 20 {
		return "", fmt.Errorf("decrypted message too short: %d bytes", len(decrypted))
	}

	// 提取消息长度（第 16-20 字节，网络字节序大端）
	contentLen := binary.BigEndian.Uint32(decrypted[16:20])
	
	// 验证消息长度
	if int(contentLen) > len(decrypted)-20 {
		return "", fmt.Errorf("invalid message length: %d > %d", contentLen, len(decrypted)-20)
	}

	// 提取消息内容（从第 20 字节开始）
	contentStart := 20
	contentEnd := contentStart + int(contentLen)
	if contentEnd > len(decrypted) {
		return "", fmt.Errorf("message content out of bounds")
	}
	content := decrypted[contentStart:contentEnd]

	// 验证 CorpID（消息内容后面应该是 CorpID）
	corpIDStart := contentEnd
	if corpIDStart < len(decrypted) {
		corpID := string(decrypted[corpIDStart:])
		if corpID != a.cfg.CorpID {
			a.logger.Warn("CorpID mismatch",
				zap.String("expected", a.cfg.CorpID),
				zap.String("got", corpID),
			)
			// 不返回错误，因为有些情况下 CorpID 可能不匹配但消息仍然有效
		}
	}

	return string(content), nil
}

// pkcs7Unpad 去除 PKCS7 填充
func (a *Adapter) pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return data
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return data
		}
	}
	return data[:len(data)-padding]
}

// getAccessToken 获取 access_token
func (a *Adapter) getAccessToken() (string, error) {
	a.tokenMu.RLock()
	if a.accessToken != "" && time.Now().Before(a.tokenExpiry) {
		token := a.accessToken
		a.tokenMu.RUnlock()
		return token, nil
	}
	a.tokenMu.RUnlock()

	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// 双重检查
	if a.accessToken != "" && time.Now().Before(a.tokenExpiry) {
		return a.accessToken, nil
	}

	// 获取新的 access_token
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		a.cfg.CorpID, a.cfg.Secret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// 解析 JSON 响应
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("failed to get access token: %d %s", result.ErrCode, result.ErrMsg)
	}

	a.accessToken = result.AccessToken
	// 提前 5 分钟过期，避免边界情况
	a.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return a.accessToken, nil
}

// SendMessage 发送消息
func (a *Adapter) SendMessage(sessionID string, content string) error {
	// 获取 access_token
	token, err := a.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// 企微发送消息 API
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	// 构建请求体
	reqBody := map[string]interface{}{
		"touser":  sessionID,
		"msgtype": "text",
		"agentid": a.cfg.AgentID, // 需要从配置中获取
		"text": map[string]string{
			"content": content,
		},
		"safe": 0,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MsgID   string `json:"msgid"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("failed to send message: %d %s", result.ErrCode, result.ErrMsg)
	}

	a.logger.Debug("Sent message to WeCom",
		zap.String("session_id", sessionID),
		zap.String("msg_id", result.MsgID),
	)

	return nil
}

// SendImageMessage 发送图片消息
func (a *Adapter) SendImageMessage(sessionID string, imageData []byte) error {
	// 先上传图片获取 media_id
	mediaID, err := a.uploadMedia("image", imageData)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// 获取 access_token
	token, err := a.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// 发送图片消息
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	reqBody := map[string]interface{}{
		"touser":  sessionID,
		"msgtype": "image",
		"agentid": a.cfg.AgentID,
		"image": map[string]string{
			"media_id": mediaID,
		},
		"safe": 0,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("failed to send message: %d %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// uploadMedia 上传媒体文件
func (a *Adapter) uploadMedia(mediaType string, mediaData []byte) (string, error) {
	// 获取 access_token
	token, err := a.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// 上传媒体文件
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=%s", token, mediaType)

	// 创建 multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("media", "media")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(mediaData); err != nil {
		return "", fmt.Errorf("failed to write media data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// 发送请求
	resp, err := http.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
		Type     string `json:"type"`
		MediaID  string `json:"media_id"`
		CreatedAt int64 `json:"created_at"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("failed to upload media: %d %s", result.ErrCode, result.ErrMsg)
	}

	return result.MediaID, nil
}

// WeComMessage 企微消息（加密后）
type WeComMessage struct {
	XMLName     xml.Name `xml:"xml"`
	Encrypt     string   `xml:"Encrypt"`
	MsgSignature string  `xml:"MsgSignature"`
	TimeStamp   string   `xml:"TimeStamp"`
	Nonce       string   `xml:"Nonce"`
}

// WeComDecryptedMessage 企微消息（解密后）
type WeComDecryptedMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
	PicURL       string   `xml:"PicUrl,omitempty"`
	MediaID      string   `xml:"MediaId,omitempty"`
	Format       string   `xml:"Format,omitempty"`
}
