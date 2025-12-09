package message

// MessageType 消息类型常量
const (
	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeVoice = "voice"
	MessageTypeFile  = "file"
	MessageTypeVideo = "video"
)

// Platform 平台标识常量
const (
	PlatformLark  = "lark"
	PlatformWeCom = "wecom"
)

// NewTextMessage 创建文本消息
func NewTextMessage(platform, sessionID, userID, content string) *Message {
	return &Message{
		Platform:    platform,
		SessionID:   sessionID,
		UserID:      userID,
		Content:     content,
		MessageType: MessageTypeText,
		Metadata:    make(map[string]string),
	}
}

// NewImageMessage 创建图片消息
func NewImageMessage(platform, sessionID, userID, imageData string) *Message {
	return &Message{
		Platform:    platform,
		SessionID:   sessionID,
		UserID:      userID,
		Content:     imageData,
		MessageType: MessageTypeImage,
		Metadata:    make(map[string]string),
	}
}

// IsText 检查是否是文本消息
func (m *Message) IsText() bool {
	return m.MessageType == MessageTypeText
}

// IsImage 检查是否是图片消息
func (m *Message) IsImage() bool {
	return m.MessageType == MessageTypeImage
}

// IsVoice 检查是否是语音消息
func (m *Message) IsVoice() bool {
	return m.MessageType == MessageTypeVoice
}

// HasImage 检查是否包含图片
func (m *Message) HasImage() bool {
	return m.IsImage() || m.Metadata["image_key"] != "" || m.Metadata["media_id"] != ""
}

// GetImageData 获取图片数据（base64 或 URL）
func (m *Message) GetImageData() string {
	if m.IsImage() {
		return m.Content
	}
	if imageKey, ok := m.Metadata["image_key"]; ok {
		return imageKey
	}
	if mediaID, ok := m.Metadata["media_id"]; ok {
		return mediaID
	}
	return ""
}

