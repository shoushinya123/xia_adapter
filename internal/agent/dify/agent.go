package dify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"xia_adpter/internal/config"
	"xia_adpter/internal/message"

	"go.uber.org/zap"
)

// Agent Dify Agent
type Agent struct {
	cfg    config.DifyConfig
	logger *zap.Logger
	client *http.Client
}

// NewAgent 创建新的 Dify Agent
func NewAgent(cfg config.DifyConfig, logger *zap.Logger) *Agent {
	return &Agent{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat 发送聊天消息（使用 AgentRequest 格式）
func (a *Agent) Chat(ctx context.Context, req *message.AgentRequest) (*message.AgentResponse, error) {
	converter := message.NewConverter()
	
	// 构建 Dify 请求
	payload := converter.BuildDifyRequest(req, map[string]interface{}{})
	payload["user"] = req.SessionID // 使用 session_id 作为 user
	if a.cfg.UserID != "" {
		payload["user"] = a.cfg.UserID
	}
	
	// 调试日志：检查 payload 中的 conversation_id
	if cid, ok := payload["conversation_id"].(string); ok {
		a.logger.Info("Dify request payload contains conversation_id",
			zap.String("conversation_id", cid),
			zap.Bool("is_uuid", len(cid) == 36 && strings.Count(cid, "-") == 4),
		)
	} else {
		a.logger.Info("Dify request payload does not contain conversation_id, will create new conversation")
	}
	
	url := fmt.Sprintf("%s/chat-messages", a.cfg.APIBase)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 检查 API Key 是否为空
	if a.cfg.APIKey == "" {
		return nil, fmt.Errorf("Dify API key is empty")
	}

	// 设置 Authorization header
	authHeader := fmt.Sprintf("Bearer %s", a.cfg.APIKey)
	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Content-Type", "application/json")
	
	// 调试日志
	a.logger.Debug("Dify API request",
		zap.String("url", url),
		zap.String("app_id", a.cfg.AppID),
		zap.Bool("has_api_key", a.cfg.APIKey != ""),
		zap.String("auth_header_prefix", func() string {
			if len(authHeader) > 20 {
				return authHeader[:20] + "..."
			}
			return authHeader
		}()),
	)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Dify API error: %d, %s", resp.StatusCode, string(body))
	}

	// 处理 SSE 流式响应
	var fullResponse strings.Builder
	var conversationID string
	var messageID string
	scanner := bufio.NewScanner(resp.Body)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				a.logger.Warn("Failed to parse SSE event", zap.Error(err))
				continue
			}

			// 提取消息内容
			if answer, ok := event["answer"].(string); ok {
				fullResponse.WriteString(answer)
			}
			
			// 提取会话 ID
			if cid, ok := event["conversation_id"].(string); ok && cid != "" {
				conversationID = cid
			}
			
			// 提取消息 ID
			if mid, ok := event["message_id"].(string); ok && mid != "" {
				messageID = mid
			}
			
			// 处理文件（图片等）- 文件信息会在最终响应中返回
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	response := fullResponse.String()
	if response == "" {
		response = "抱歉，我没有理解您的问题。"
	}

	// 构建 AgentResponse
	agentResp := &message.AgentResponse{
		Content:   response,
		ImageURLs: []string{},
		Metadata:  make(map[string]string),
	}
	
	if conversationID != "" {
		agentResp.Metadata["conversation_id"] = conversationID
	}
	if messageID != "" {
		agentResp.Metadata["message_id"] = messageID
	}

	return agentResp, nil
}

