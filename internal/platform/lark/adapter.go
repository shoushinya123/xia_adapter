package lark

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"xia_adpter/internal/config"
	"xia_adpter/internal/message"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"go.uber.org/zap"
)

// Adapter 飞书适配器
type Adapter struct {
	cfg      config.LarkConfig
	queue    *message.Queue
	logger   *zap.Logger
	client   *lark.Client
	wsClient *larkws.Client
	botName  string
	mu       sync.RWMutex
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewAdapter 创建新的飞书适配器
func NewAdapter(cfg config.LarkConfig, queue *message.Queue, logger *zap.Logger) *Adapter {
	botName := cfg.BotName
	if botName == "" {
		botName = "astrbot"
	}

	// 创建飞书 API 客户端（用于发送消息）
	baseURL := lark.FeishuBaseUrl
	if cfg.Domain == "larksuite.com" {
		baseURL = lark.LarkBaseUrl
	}

	client := lark.NewClient(
		cfg.AppID,
		cfg.AppSecret,
		lark.WithOpenBaseUrl(baseURL),
		lark.WithLogLevel(larkcore.LogLevelError),
	)

	return &Adapter{
		cfg:     cfg,
		queue:   queue,
		logger:  logger,
		client:  client,
		botName: botName,
	}
}

// Start 启动适配器
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("adapter is already running")
	}
	a.running = true
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.mu.Unlock()

	a.logger.Info("Starting Lark adapter",
		zap.String("app_id", a.cfg.AppID),
		zap.String("domain", a.cfg.Domain),
		zap.String("bot_name", a.botName),
	)

	// 创建事件分发器（按照官方示例）
	eventDispatcher := larkdispatcher.NewEventDispatcher("", "")
	
	// 注册消息接收事件处理器（使用 P2 版本）
	// 注意：WebSocket 长连接使用 P2 版本事件，事件类型为 "im.message.receive_v1"
	eventDispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		return a.handleMessageEvent(ctx, event)
	})

	// 创建 WebSocket 客户端选项（按照官方示例，简化配置）
	opts := []larkws.ClientOption{
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithLogLevel(larkcore.LogLevelError),
		// 注意：不显式设置 WithAutoReconnect，SDK 默认开启
		// 不显式设置 WithDomain，使用 SDK 默认值（https://open.feishu.cn）
	}

	// 只有在使用 larksuite.com 时才指定域名
	if a.cfg.Domain == "larksuite.com" {
		// 使用完整的 URL，包含协议前缀
		opts = append(opts, larkws.WithDomain("https://open.larksuite.com"))
	}

	// 创建 WebSocket 客户端（按照官方示例）
	wsClient := larkws.NewClient(a.cfg.AppID, a.cfg.AppSecret, opts...)
	a.wsClient = wsClient

	// 在协程中启动 WebSocket 连接（Start 会阻塞）
	// 按照官方示例，直接调用 Start，但我们需要在 goroutine 中运行以避免阻塞
	go func() {
		a.logger.Info("Starting WebSocket connection...")
		if err := wsClient.Start(a.ctx); err != nil {
			if err == context.Canceled {
				a.logger.Info("WebSocket client stopped by context")
			} else {
				a.logger.Error("WebSocket client error", zap.Error(err))
			}
		}
	}()

	// 等待一小段时间确保连接启动
	time.Sleep(500 * time.Millisecond)
	a.logger.Info("Lark WebSocket client started")

	// 等待上下文取消
	<-a.ctx.Done()
	return a.Stop()
}

// Stop 停止适配器
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cancel != nil {
		a.cancel()
	}

	// WebSocket 客户端会在上下文取消时自动关闭
	a.running = false
	a.logger.Info("Lark adapter stopped")
	return nil
}

// handleMessageEvent 处理消息接收事件（P2 版本）
func (a *Adapter) handleMessageEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil {
		return nil
	}

	data := event.Event
	if data.Message == nil {
		return nil
	}

	msg := data.Message
	sender := data.Sender

	// 获取会话 ID
	sessionID := ""
	if msg.ChatId != nil {
		sessionID = *msg.ChatId
	} else if sender != nil && sender.SenderId != nil {
		// 如果是私聊，使用发送者 ID
		if sender.SenderId.OpenId != nil {
			sessionID = *sender.SenderId.OpenId
		} else if sender.SenderId.UserId != nil {
			sessionID = *sender.SenderId.UserId
		}
	}

	// 获取用户 ID
	userID := ""
	if sender != nil && sender.SenderId != nil {
		if sender.SenderId.OpenId != nil {
			userID = *sender.SenderId.OpenId
		} else if sender.SenderId.UserId != nil {
			userID = *sender.SenderId.UserId
		}
	}

	// 获取消息类型
	messageType := "text"
	if msg.MessageType != nil {
		messageType = a.getMessageType(*msg.MessageType)
	}

	// 提取消息内容
	content := ""
	if msg.Content != nil {
		content = a.extractTextContentFromP2Message(*msg.Content, messageType)
	}

	// 构建统一消息格式
	msgObj := &message.Message{
		Platform:    "lark",
		SessionID:   sessionID,
		UserID:      userID,
		Content:     content,
		MessageType: messageType,
		Metadata:    make(map[string]string),
	}

	// 添加元数据
	if msg.MessageId != nil {
		msgObj.Metadata["message_id"] = *msg.MessageId
	}
	if msg.ChatId != nil {
		msgObj.Metadata["chat_id"] = *msg.ChatId
	}
	if msg.ChatType != nil {
		msgObj.Metadata["chat_type"] = *msg.ChatType
	}

	// 处理图片消息
	if messageType == "image" && msg.MessageId != nil {
		// 从 content 中提取 image_key
		if imageKey := a.extractImageKeyFromContent(*msg.Content); imageKey != "" {
			msgObj.Metadata["image_key"] = imageKey
			// 下载图片并转换为 base64
			if imageData, err := a.downloadImage(*msg.MessageId, imageKey); err == nil {
				msgObj.Content = base64.StdEncoding.EncodeToString(imageData)
			} else {
				a.logger.Warn("Failed to download image", zap.Error(err))
			}
		}
	}

	a.logger.Info("Received Lark message",
		zap.String("message_id", func() string {
			if msg.MessageId != nil {
				return *msg.MessageId
			}
			return ""
		}()),
		zap.String("session_id", msgObj.SessionID),
		zap.String("user_id", msgObj.UserID),
		zap.String("type", msgObj.MessageType),
		zap.String("content", func() string {
			if len(msgObj.Content) > 50 {
				return msgObj.Content[:50] + "..."
			}
			return msgObj.Content
		}()),
	)

	// 推送到消息队列
	a.queue.Push(msgObj)
	return nil
}

// extractTextContentFromP2Message 从 P2 消息的 Content 字段提取文本内容
func (a *Adapter) extractTextContentFromP2Message(contentJSON string, messageType string) string {
	// Content 是 JSON 字符串，需要解析
	var contentMap map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &contentMap); err != nil {
		return ""
	}

	switch messageType {
	case "text":
		if text, ok := contentMap["text"].(string); ok {
			text = a.removeAtMentions(text)
			return strings.TrimSpace(text)
		}
	case "post":
		// 处理富文本消息
		return a.extractTextContent(contentMap, "post")
	case "image":
		// 图片消息，返回空字符串，后续会下载图片
		return ""
	default:
		// 尝试提取文本
		if text, ok := contentMap["text"].(string); ok {
			return strings.TrimSpace(text)
		}
	}

	return ""
}

// extractImageKeyFromContent 从消息内容中提取图片 key
func (a *Adapter) extractImageKeyFromContent(contentJSON string) string {
	var contentMap map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &contentMap); err != nil {
		return ""
	}

	if imageKey, ok := contentMap["image_key"].(string); ok {
		return imageKey
	}

	return ""
}


// extractTextContent 提取文本内容
func (a *Adapter) extractTextContent(contentJSON map[string]interface{}, msgType string) string {
	switch msgType {
	case "text":
		if text, ok := contentJSON["text"].(string); ok {
			// 移除 @ 用户标记
			text = a.removeAtMentions(text)
			return strings.TrimSpace(text)
		}
	case "post":
		// 处理富文本消息
		if content, ok := contentJSON["content"].([]interface{}); ok {
			var textParts []string
			for _, item := range content {
				if items, ok := item.([]interface{}); ok {
					for _, subItem := range items {
						if subItemMap, ok := subItem.(map[string]interface{}); ok {
							if tag, ok := subItemMap["tag"].(string); ok {
								if tag == "text" {
									if text, ok := subItemMap["text"].(string); ok {
										textParts = append(textParts, strings.TrimSpace(text))
									}
								}
							}
						}
					}
				}
			}
			return strings.Join(textParts, "\n")
		}
	}
	return ""
}

// removeAtMentions 移除 @ 用户标记
func (a *Adapter) removeAtMentions(text string) string {
	// 移除 @_user_xxx 格式的标记
	re := strings.NewReplacer("@_user_", "")
	return re.Replace(text)
}

// getMessageType 获取消息类型
func (a *Adapter) getMessageType(larkType string) string {
	switch larkType {
	case "text", "post":
		return "text"
	case "image":
		return "image"
	default:
		return "text"
	}
}

// downloadImage 下载图片
func (a *Adapter) downloadImage(messageID, imageKey string) ([]byte, error) {
	// 调用飞书 API 下载图片
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(imageKey).
		Type("image").
		Build()

	resp, err := a.client.Im.V1.MessageResource.Get(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to get image resource: %w", err)
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to get image resource: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return imageData, nil
}

// SendMessage 发送消息
func (a *Adapter) SendMessage(sessionID string, content string) error {
	return a.sendTextMessage(sessionID, content)
}

// sendTextMessage 发送文本消息
func (a *Adapter) sendTextMessage(sessionID string, content string) error {
	// 判断是群聊还是私聊
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.Contains(sessionID, "%") {
		// 群聊：sessionID 格式可能是 "user_id%chat_id"
		parts := strings.Split(sessionID, "%")
		if len(parts) > 1 {
			sessionID = parts[1]
			receiveIDType = larkim.ReceiveIdTypeChatId
		}
	} else {
		// 检查是否是 chat_id（通常 chat_id 更长）
		if len(sessionID) > 20 {
			receiveIDType = larkim.ReceiveIdTypeChatId
		}
	}

	// 构建消息内容（使用富文本格式）
	messageContent := map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"title": "",
			"content": [][]map[string]interface{}{
				{
					{
						"tag":  "text",
						"text": content,
					},
				},
			},
		},
	}

	contentJSON, err := json.Marshal(messageContent)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}

	// 创建消息请求
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(sessionID).
			Content(string(contentJSON)).
			MsgType("post").
			Uuid(fmt.Sprintf("%d", time.Now().UnixNano())).
			Build()).
		Build()

	// 发送消息
	resp, err := a.client.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	messageID := ""
	if resp.Data.MessageId != nil {
		messageID = *resp.Data.MessageId
	}
	a.logger.Debug("Sent message to Lark",
		zap.String("session_id", sessionID),
		zap.String("receive_id_type", string(receiveIDType)),
		zap.String("message_id", messageID),
	)

	return nil
}

// SendImageMessage 发送图片消息
func (a *Adapter) SendImageMessage(sessionID string, imageData []byte) error {
	// 先上传图片
	imageKey, err := a.uploadImage(imageData)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// 判断是群聊还是私聊
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.Contains(sessionID, "%") {
		parts := strings.Split(sessionID, "%")
		if len(parts) > 1 {
			sessionID = parts[1]
			receiveIDType = larkim.ReceiveIdTypeChatId
		}
	} else if len(sessionID) > 20 {
		receiveIDType = larkim.ReceiveIdTypeChatId
	}

	// 构建消息内容
	messageContent := map[string]interface{}{
		"zh_cn": map[string]interface{}{
			"title": "",
			"content": [][]map[string]interface{}{
				{
					{
						"tag":       "img",
						"image_key": imageKey,
					},
				},
			},
		},
	}

	contentJSON, err := json.Marshal(messageContent)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}

	// 创建消息请求
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(sessionID).
			Content(string(contentJSON)).
			MsgType("post").
			Uuid(fmt.Sprintf("%d", time.Now().UnixNano())).
			Build()).
		Build()

	// 发送消息
	resp, err := a.client.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// uploadImage 上传图片
func (a *Adapter) uploadImage(imageData []byte) (string, error) {
	// 调用飞书 API 上传图片
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.ImageTypeMessage).
			Image(strings.NewReader(string(imageData))).
			Build()).
		Build()

	resp, err := a.client.Im.V1.Image.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to upload image: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data.ImageKey == nil {
		return "", fmt.Errorf("image key is nil")
	}

	return *resp.Data.ImageKey, nil
}
