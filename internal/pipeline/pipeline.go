package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"xia_adpter/internal/agent/coze"
	"xia_adpter/internal/agent/dify"
	"xia_adpter/internal/config"
	"xia_adpter/internal/message"

	"go.uber.org/zap"
)

// PlatformSender 平台消息发送接口
type PlatformSender interface {
	SendMessage(sessionID string, content string) error
}

// Pipeline 消息处理管道
type Pipeline struct {
	cfg       *config.Config
	logger    *zap.Logger
	difyAgent *dify.Agent
	cozeAgent *coze.Agent
	converter *message.Converter
	
	// 平台发送器映射
	senders map[string]PlatformSender
	mu      sync.RWMutex
}

// New 创建新的消息处理管道
func New(cfg *config.Config, logger *zap.Logger) *Pipeline {
	p := &Pipeline{
		cfg:       cfg,
		logger:    logger,
		senders:   make(map[string]PlatformSender),
		converter: message.NewConverter(),
	}

	// 初始化 Dify Agent
	if cfg.Agent.Dify.Enabled {
		p.difyAgent = dify.NewAgent(cfg.Agent.Dify, logger)
	}

	// 初始化 Coze Agent
	if cfg.Agent.Coze.Enabled {
		p.cozeAgent = coze.NewAgent(cfg.Agent.Coze, logger)
	}

	return p
}

// RegisterSender 注册平台发送器
func (p *Pipeline) RegisterSender(platform string, sender PlatformSender) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.senders[platform] = sender
}

// Start 启动消息处理管道
func (p *Pipeline) Start(ctx context.Context, queue *message.Queue) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := queue.Pop(ctx)
			if err != nil {
				return err
			}

			// 处理消息
			go p.processMessage(ctx, msg)
		}
	}
}

// processMessage 处理单个消息
func (p *Pipeline) processMessage(ctx context.Context, msg *message.Message) {
	p.logger.Info("Processing message",
		zap.String("platform", msg.Platform),
		zap.String("session_id", msg.SessionID),
		zap.String("type", msg.MessageType),
		zap.String("content", func() string {
			if len(msg.Content) > 100 {
				return msg.Content[:100] + "..."
			}
			return msg.Content
		}()),
	)

	// 规范化消息内容
	p.converter.NormalizeContent(msg)

	// 转换为 Agent 请求格式
	agentReq := p.converter.ToAgentRequest(msg)

	// 根据配置选择 Agent
	var agentResp *message.AgentResponse
	var err error

	if p.cfg.Agent.Dify.Enabled {
		agentResp, err = p.difyAgent.Chat(ctx, agentReq)
		if err != nil {
			p.logger.Error("Dify agent error", zap.Error(err))
			// 如果 Dify 失败，尝试 Coze
			if p.cfg.Agent.Coze.Enabled {
				agentResp, err = p.cozeAgent.Chat(ctx, agentReq)
			}
		}
	} else if p.cfg.Agent.Coze.Enabled {
		agentResp, err = p.cozeAgent.Chat(ctx, agentReq)
	}

	if err != nil {
		p.logger.Error("Failed to get agent response", zap.Error(err))
		// 创建错误响应
		agentResp = &message.AgentResponse{
			Content: fmt.Sprintf("处理消息时出错: %v", err),
		}
	}

	// 将 Agent 响应转换为统一消息格式
	responseMsg := p.converter.FromAgentResponse(agentResp, msg)
	
	// 如果 Agent 返回了 conversation_id，保存到原始消息的 Metadata 中
	// 这样下次请求时可以使用相同的 conversation_id
	// 注意：只保存有效的 UUID 格式的 conversation_id
	if agentResp != nil && agentResp.Metadata != nil {
		if cid, ok := agentResp.Metadata["conversation_id"]; ok && cid != "" {
			// 验证 conversation_id 是否是有效的 UUID
			uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
			if uuidRegex.MatchString(strings.ToLower(cid)) {
				if msg.Metadata == nil {
					msg.Metadata = make(map[string]string)
				}
				msg.Metadata["conversation_id"] = cid
				// 同时保存到响应消息的 Metadata 中
				if responseMsg.Metadata == nil {
					responseMsg.Metadata = make(map[string]string)
				}
				responseMsg.Metadata["conversation_id"] = cid
			} else {
				// 如果不是有效的 UUID，清除之前可能错误保存的 conversation_id
				if msg.Metadata != nil {
					delete(msg.Metadata, "conversation_id")
				}
				if responseMsg.Metadata != nil {
					delete(responseMsg.Metadata, "conversation_id")
				}
			}
		}
	}

	// 发送回复到平台
	p.mu.RLock()
	sender, ok := p.senders[msg.Platform]
	p.mu.RUnlock()

	if ok && sender != nil {
		// 根据平台格式化消息
		if err := p.sendToPlatform(sender, msg.Platform, responseMsg); err != nil {
			p.logger.Error("Failed to send message to platform",
				zap.String("platform", msg.Platform),
				zap.String("session_id", msg.SessionID),
				zap.Error(err),
			)
		} else {
			p.logger.Info("Message sent successfully",
				zap.String("platform", msg.Platform),
				zap.String("session_id", msg.SessionID),
			)
		}
	} else {
		p.logger.Warn("No sender registered for platform",
			zap.String("platform", msg.Platform),
		)
	}
}

// sendToPlatform 发送消息到平台
func (p *Pipeline) sendToPlatform(sender PlatformSender, platform string, msg *message.Message) error {
	// 根据平台类型格式化消息
	switch platform {
	case message.PlatformLark:
		// 飞书需要特殊格式，由适配器处理
		return sender.SendMessage(msg.SessionID, msg.Content)
	case message.PlatformWeCom:
		// 企微需要分割长文本
		if msg.IsText() {
			chunks := p.converter.SplitLongText(msg.Content, 2048)
			for _, chunk := range chunks {
				if err := sender.SendMessage(msg.SessionID, chunk); err != nil {
					return err
				}
			}
			return nil
		}
		return sender.SendMessage(msg.SessionID, msg.Content)
	default:
		return sender.SendMessage(msg.SessionID, msg.Content)
	}
}

