package message

import (
	"context"
)

// Message 统一消息结构
type Message struct {
	Platform    string            `json:"platform"`     // lark, wecom
	SessionID   string            `json:"session_id"`   // 会话ID
	UserID      string            `json:"user_id"`     // 用户ID
	Content     string            `json:"content"`      // 消息内容（文本或 base64 图片）
	MessageType string            `json:"message_type"` // text, image, voice, file
	Metadata    map[string]string `json:"metadata"`    // 平台特定元数据
	Timestamp   int64             `json:"timestamp,omitempty"` // 时间戳
}

// Queue 消息队列
type Queue struct {
	ch chan *Message
}

// NewQueue 创建新的消息队列
func NewQueue(size int) *Queue {
	return &Queue{
		ch: make(chan *Message, size),
	}
}

// Push 推送消息到队列
func (q *Queue) Push(msg *Message) {
	select {
	case q.ch <- msg:
	default:
		// 队列满了，丢弃消息或记录日志
	}
}

// Pop 从队列弹出消息（阻塞）
func (q *Queue) Pop(ctx context.Context) (*Message, error) {
	select {
	case msg := <-q.ch:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TryPop 尝试从队列弹出消息（非阻塞）
func (q *Queue) TryPop() (*Message, bool) {
	select {
	case msg := <-q.ch:
		return msg, true
	default:
		return nil, false
	}
}

