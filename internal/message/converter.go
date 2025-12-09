package message

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Converter 消息格式转换器
type Converter struct{}

// NewConverter 创建消息转换器
func NewConverter() *Converter {
	return &Converter{}
}

// PlatformMessage 平台消息接口（各平台适配器实现）
type PlatformMessage interface {
	GetPlatform() string
	GetSessionID() string
	GetUserID() string
	GetContent() string
	GetMessageType() string
	GetMetadata() map[string]string
}

// AgentRequest Agent 请求格式
type AgentRequest struct {
	Query       string                   `json:"query"`        // 文本查询
	ImageURLs   []string                 `json:"image_urls"`   // 图片 URL 列表（base64 或 URL）
	SessionID   string                   `json:"session_id"`   // 会话 ID
	UserID      string                   `json:"user_id"`      // 用户 ID
	SystemPrompt string                  `json:"system_prompt,omitempty"` // 系统提示词
	Contexts    []map[string]interface{} `json:"contexts,omitempty"`      // 历史上下文
	Metadata    map[string]string        `json:"metadata,omitempty"`      // 元数据
}

// AgentResponse Agent 响应格式
type AgentResponse struct {
	Content   string            `json:"content"`    // 文本内容
	ImageURLs []string          `json:"image_urls"` // 图片 URL 列表
	Metadata  map[string]string `json:"metadata"`   // 元数据
}

// ToAgentRequest 将统一消息格式转换为 Agent 请求格式
func (c *Converter) ToAgentRequest(msg *Message) *AgentRequest {
	req := &AgentRequest{
		Query:     msg.Content,
		SessionID: msg.SessionID,
		UserID:    msg.UserID,
		ImageURLs: []string{},
		Metadata:  make(map[string]string),
	}
	
	// 复制原始消息的 Metadata，以便保存和复用 conversation_id
	// 注意：只复制有效的 UUID 格式的 conversation_id，清除错误的格式
	if msg.Metadata != nil {
		for k, v := range msg.Metadata {
			// 如果是 conversation_id，验证是否为有效的 UUID
			if k == "conversation_id" {
				if isUUID(v) {
					req.Metadata[k] = v
				}
				// 如果不是 UUID，不复制，让 Dify 创建新的会话
			} else {
				req.Metadata[k] = v
			}
		}
	}

	// 处理图片消息
	if msg.MessageType == "image" {
		// 检查 Content 是否是 base64
		if strings.HasPrefix(msg.Content, "data:image/") || 
		   (len(msg.Content) > 100 && !strings.HasPrefix(msg.Content, "http")) {
			// 可能是 base64 图片
			if imageData, err := c.extractBase64Image(msg.Content); err == nil {
				req.ImageURLs = append(req.ImageURLs, imageData)
			}
		} else if strings.HasPrefix(msg.Content, "http") {
			// URL 图片
			req.ImageURLs = append(req.ImageURLs, msg.Content)
		}

		// 从 Metadata 中获取图片信息
		if mediaID, ok := msg.Metadata["media_id"]; ok {
			// 企微的 media_id，需要下载后转换为 base64
			// 这里先保存 media_id，后续需要下载
			if req.Metadata == nil {
				req.Metadata = make(map[string]string)
			}
			req.Metadata["media_id"] = mediaID
			req.Metadata["platform"] = msg.Platform
		}
	}

	// 处理混合消息（文本 + 图片）
	if msg.MessageType == "text" {
		// 检查是否有图片元数据
		if imageKey, ok := msg.Metadata["image_key"]; ok && imageKey != "" {
			// 飞书图片，Content 中应该已经有 base64
			if strings.HasPrefix(msg.Content, "data:image/") || 
			   (len(msg.Content) > 100 && !strings.HasPrefix(msg.Content, "http")) {
				req.ImageURLs = append(req.ImageURLs, msg.Content)
			}
		}
	}

	return req
}

// FromAgentResponse 将 Agent 响应格式转换为统一消息格式
func (c *Converter) FromAgentResponse(resp *AgentResponse, originalMsg *Message) *Message {
	msg := &Message{
		Platform:    originalMsg.Platform,
		SessionID:   originalMsg.SessionID,
		UserID:      originalMsg.UserID,
		Content:     resp.Content,
		MessageType: "text",
		Metadata:    resp.Metadata,
	}

	// 如果有图片，设置为图片消息
	if len(resp.ImageURLs) > 0 {
		msg.MessageType = "image"
		// 将第一张图片作为 Content
		msg.Content = resp.ImageURLs[0]
		// 其他图片保存在 Metadata 中
		if len(resp.ImageURLs) > 1 {
			msg.Metadata["additional_images"] = strings.Join(resp.ImageURLs[1:], ",")
		}
	}

	return msg
}

// ToPlatformMessage 将统一消息格式转换为平台消息格式
// 这个方法返回平台特定的消息结构，由各平台适配器实现具体的发送逻辑
func (c *Converter) ToPlatformMessage(msg *Message) PlatformMessage {
	return &PlatformMessageImpl{
		platform:    msg.Platform,
		sessionID:   msg.SessionID,
		userID:      msg.UserID,
		content:     msg.Content,
		messageType: msg.MessageType,
		metadata:    msg.Metadata,
	}
}

// PlatformMessageImpl 平台消息实现
type PlatformMessageImpl struct {
	platform    string
	sessionID   string
	userID      string
	content     string
	messageType string
	metadata    map[string]string
}

func (p *PlatformMessageImpl) GetPlatform() string    { return p.platform }
func (p *PlatformMessageImpl) GetSessionID() string   { return p.sessionID }
func (p *PlatformMessageImpl) GetUserID() string      { return p.userID }
func (p *PlatformMessageImpl) GetContent() string     { return p.content }
func (p *PlatformMessageImpl) GetMessageType() string { return p.messageType }
func (p *PlatformMessageImpl) GetMetadata() map[string]string { return p.metadata }

// extractBase64Image 提取 base64 图片数据
func (c *Converter) extractBase64Image(content string) (string, error) {
	// 检查是否是 data URI 格式
	if strings.HasPrefix(content, "data:image/") {
		parts := strings.Split(content, ",")
		if len(parts) == 2 {
			return parts[1], nil
		}
	}

	// 检查是否是纯 base64 字符串
	if _, err := base64.StdEncoding.DecodeString(content); err == nil {
		// 可能是 base64，但需要验证是否是图片
		if len(content) > 100 {
			return content, nil
		}
	}

	return "", fmt.Errorf("invalid image format")
}

// NormalizeContent 规范化消息内容
func (c *Converter) NormalizeContent(msg *Message) {
	// 清理文本内容
	if msg.MessageType == "text" {
		msg.Content = strings.TrimSpace(msg.Content)
		// 移除多余的空白字符
		msg.Content = strings.ReplaceAll(msg.Content, "\r\n", "\n")
		msg.Content = strings.ReplaceAll(msg.Content, "\r", "\n")
	}

	// 处理图片内容
	if msg.MessageType == "image" {
		// 确保图片内容是有效的 base64 或 URL
		if !strings.HasPrefix(msg.Content, "http") && 
		   !strings.HasPrefix(msg.Content, "data:image/") &&
		   len(msg.Content) > 100 {
			// 可能是纯 base64，添加 data URI 前缀
			msg.Content = "data:image/png;base64," + msg.Content
		}
	}
}

// SplitLongText 分割长文本（用于平台限制）
func (c *Converter) SplitLongText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 2048 // 默认 2048 字符
	}

	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + maxLen
		if end > len(text) {
			end = len(text)
		}

			// 尝试在标点符号处分割
			if end < len(text) {
				// 向前查找合适的分割点
				cutPos := end
				for i := end; i > start+maxLen/2; i-- {
					if i < len(text) {
						char := rune(text[i-1])
						if char == '\n' || char == '。' || char == '！' || 
						   char == '？' || char == '.' || char == '!' || char == '?' {
							cutPos = i
							break
						}
					}
				}
				end = cutPos
			}

		chunks = append(chunks, text[start:end])
		start = end
	}

	return chunks
}

// isUUID 检查字符串是否是有效的 UUID 格式
func isUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(s))
}

// BuildDifyRequest 构建 Dify 请求
func (c *Converter) BuildDifyRequest(req *AgentRequest, variables map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"query":         req.Query,
		"user":          req.SessionID,
		"response_mode": "streaming",
	}

	// 检查是否有有效的 conversation_id（UUID 格式）
	// 优先从 Metadata 中获取之前保存的 conversation_id
	var conversationID string
	if req.Metadata != nil {
		if cid, ok := req.Metadata["conversation_id"]; ok && cid != "" {
			// 验证 conversation_id 是否是有效的 UUID
			// 如果之前错误地保存了非 UUID 格式的 ID（如飞书的 chat_id），则忽略并清除
			if isUUID(cid) {
				conversationID = cid
			} else {
				// 如果不是 UUID，清除错误的 conversation_id，防止下次继续使用
				delete(req.Metadata, "conversation_id")
			}
		}
	}
	
	// 如果没有从 Metadata 获取到有效的 UUID，尝试使用 SessionID（但必须是 UUID 格式）
	// 注意：SessionID 通常是飞书的 chat_id（oc_xxx），不是 UUID，所以这里通常不会匹配
	if conversationID == "" && isUUID(req.SessionID) {
		conversationID = req.SessionID
	}

	// 只有当 conversation_id 是有效的 UUID 时才传递
	// 否则让 Dify 自动创建新的会话
	// 重要：这里必须再次验证，确保不会传递错误的 conversation_id
	if conversationID != "" {
		if isUUID(conversationID) {
			payload["conversation_id"] = conversationID
		}
		// 如果不是 UUID，不设置 conversation_id，让 Dify 自动创建
	}

	// 添加变量
	inputs := make(map[string]interface{})
	for k, v := range variables {
		inputs[k] = v
	}
	payload["inputs"] = inputs

	// 处理图片
	if len(req.ImageURLs) > 0 {
		files := []map[string]interface{}{}
		for _, imgURL := range req.ImageURLs {
			// 如果是 base64，需要先上传
			if strings.HasPrefix(imgURL, "data:image/") || 
			   (len(imgURL) > 100 && !strings.HasPrefix(imgURL, "http")) {
				// base64 图片，需要上传后获取 file_id
				// 这里先保存，由 Agent 客户端处理上传
				files = append(files, map[string]interface{}{
					"type":            "image",
					"transfer_method": "local_file",
					"base64_data":     imgURL,
				})
			} else {
				// URL 图片
				files = append(files, map[string]interface{}{
					"type":            "image",
					"transfer_method": "remote_url",
					"url":             imgURL,
				})
			}
		}
		if len(files) > 0 {
			payload["files"] = files
		}
	}

	return payload
}

// BuildCozeRequest 构建 Coze 请求
func (c *Converter) BuildCozeRequest(req *AgentRequest, botID string) map[string]interface{} {
	payload := map[string]interface{}{
		"bot_id":            botID,
		"user_id":           req.UserID,
		"stream":            true,
		"auto_save_history": true,
	}

	if req.SessionID != "" {
		payload["conversation_id"] = req.SessionID
	}

	// 构建消息列表
	messages := []map[string]interface{}{}

	// 处理多模态消息（文本 + 图片）
	if len(req.ImageURLs) > 0 {
		// 构建 object_string 格式
		content := []map[string]interface{}{}
		
		// 添加文本
		if req.Query != "" {
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": req.Query,
			})
		}

		// 添加图片（base64 需要先上传）
		for _, imgURL := range req.ImageURLs {
			if strings.HasPrefix(imgURL, "data:image/") || 
			   (len(imgURL) > 100 && !strings.HasPrefix(imgURL, "http")) {
				// base64 图片，标记需要上传
				content = append(content, map[string]interface{}{
					"type":      "image",
					"base64":    imgURL,
					"need_upload": true,
				})
			} else {
				// URL 图片
				content = append(content, map[string]interface{}{
					"type": "image",
					"url":  imgURL,
				})
			}
		}

		// 转换为 JSON 字符串
		contentJSON, _ := json.Marshal(content)
		messages = append(messages, map[string]interface{}{
			"role":        "user",
			"content":     string(contentJSON),
			"content_type": "object_string",
		})
	} else if req.Query != "" {
		// 纯文本消息
		messages = append(messages, map[string]interface{}{
			"role":        "user",
			"content":     req.Query,
			"content_type": "text",
		})
	}

	// 添加历史上下文
	if len(req.Contexts) > 0 {
		messages = append(req.Contexts, messages...)
	}

	payload["additional_messages"] = messages

	return payload
}

// ParseDifyResponse 解析 Dify 响应
func (c *Converter) ParseDifyResponse(data map[string]interface{}) *AgentResponse {
	resp := &AgentResponse{
		Content:  "",
		ImageURLs: []string{},
		Metadata: make(map[string]string),
	}

	// 提取文本内容
	if answer, ok := data["answer"].(string); ok {
		resp.Content = answer
	}

	// 提取文件（图片等）
	if files, ok := data["files"].([]interface{}); ok {
		for _, file := range files {
			if fileMap, ok := file.(map[string]interface{}); ok {
				if fileType, ok := fileMap["type"].(string); ok && fileType == "image" {
					if url, ok := fileMap["url"].(string); ok {
						resp.ImageURLs = append(resp.ImageURLs, url)
					}
				}
			}
		}
	}

	// 提取元数据
	if conversationID, ok := data["conversation_id"].(string); ok {
		resp.Metadata["conversation_id"] = conversationID
	}
	if messageID, ok := data["message_id"].(string); ok {
		resp.Metadata["message_id"] = messageID
	}

	return resp
}

// ParseCozeResponse 解析 Coze 响应
func (c *Converter) ParseCozeResponse(data map[string]interface{}) *AgentResponse {
	resp := &AgentResponse{
		Content:   "",
		ImageURLs: []string{},
		Metadata:  make(map[string]string),
	}

	// 提取内容
	if content, ok := data["content"].(string); ok {
		resp.Content = content
	}

	// 提取增量内容（流式响应）
	if delta, ok := data["delta"].(map[string]interface{}); ok {
		if deltaContent, ok := delta["content"].(string); ok {
			resp.Content = deltaContent
		}
	}

	// 提取元数据
	if conversationID, ok := data["conversation_id"].(string); ok {
		resp.Metadata["conversation_id"] = conversationID
	}
	if messageID, ok := data["message_id"].(string); ok {
		resp.Metadata["message_id"] = messageID
	}

	return resp
}

// FormatForLark 格式化消息为飞书格式
func (c *Converter) FormatForLark(msg *Message) map[string]interface{} {
	// 飞书使用富文本格式
	content := [][]map[string]interface{}{}
	row := []map[string]interface{}{}

	if msg.MessageType == "text" {
		// 文本消息
		row = append(row, map[string]interface{}{
			"tag":  "text",
			"text": msg.Content,
		})
	} else if msg.MessageType == "image" {
		// 图片消息
		if imageKey, ok := msg.Metadata["image_key"]; ok {
			row = append(row, map[string]interface{}{
				"tag":       "img",
				"image_key": imageKey,
			})
		} else {
			// 如果没有 image_key，需要先上传图片
			// 这里返回标记，由适配器处理上传
			row = append(row, map[string]interface{}{
				"tag":  "text",
				"text": "[图片]",
			})
		}
	}

	if len(row) > 0 {
		content = append(content, row)
	}

	return map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"title":   "",
			"content": content,
		},
	}
}

// FormatForWeCom 格式化消息为企微格式
func (c *Converter) FormatForWeCom(msg *Message) map[string]interface{} {
	// 企微消息格式
	result := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": msg.Content,
		},
	}

	// 处理图片消息
	if msg.MessageType == "image" {
		if mediaID, ok := msg.Metadata["media_id"]; ok {
			result["msgtype"] = "image"
			result["image"] = map[string]interface{}{
				"media_id": mediaID,
			}
		} else {
			// 如果没有 media_id，需要先上传
			// 返回文本提示
			result["msgtype"] = "text"
			result["text"] = map[string]interface{}{
				"content": "[图片]",
			}
		}
	}

	return result
}

// MergeMessages 合并多个消息为一个（用于流式响应）
func (c *Converter) MergeMessages(messages []*Message) *Message {
	if len(messages) == 0 {
		return nil
	}

	if len(messages) == 1 {
		return messages[0]
	}

	// 合并文本内容
	var content strings.Builder
	for _, msg := range messages {
		if msg.MessageType == "text" {
			content.WriteString(msg.Content)
		}
	}

	// 使用第一个消息作为基础
	merged := &Message{
		Platform:    messages[0].Platform,
		SessionID:   messages[0].SessionID,
		UserID:      messages[0].UserID,
		Content:     content.String(),
		MessageType: "text",
		Metadata:    make(map[string]string),
	}

	// 合并元数据
	for _, msg := range messages {
		for k, v := range msg.Metadata {
			if _, exists := merged.Metadata[k]; !exists {
				merged.Metadata[k] = v
			}
		}
	}

	return merged
}

