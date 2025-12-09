package coze

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

// Agent Coze Agent
type Agent struct {
	cfg    config.CozeConfig
	logger *zap.Logger
	client *http.Client
}

// NewAgent 创建新的 Coze Agent
func NewAgent(cfg config.CozeConfig, logger *zap.Logger) *Agent {
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
	
	// 构建 Coze 请求
	payload := converter.BuildCozeRequest(req, a.cfg.BotID)
	payload["user_id"] = req.UserID
	if a.cfg.UserID != "" {
		payload["user_id"] = a.cfg.UserID
	}
	
	url := fmt.Sprintf("%s/v3/chat", a.cfg.APIBase)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.cfg.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("Coze API authentication failed, please check API key")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Coze API error: %d, %s", resp.StatusCode, string(body))
	}

	// 处理 SSE 流式响应
	var fullResponse strings.Builder
	var conversationID string
	var messageID string
	scanner := bufio.NewScanner(resp.Body)
	
	var eventData string
	
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		
		if line == "" {
			// 空行表示一个事件结束
			if eventData != "" {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(eventData), &data); err == nil {
					// 解析响应
					agentResp := converter.ParseCozeResponse(data)
					if agentResp.Content != "" {
						fullResponse.WriteString(agentResp.Content)
					}
					
					// 提取会话 ID
					if cid, ok := agentResp.Metadata["conversation_id"]; ok {
						conversationID = cid
					}
					
					// 提取消息 ID
					if mid, ok := agentResp.Metadata["message_id"]; ok {
						messageID = mid
					}
				}
			}
			eventData = ""
			continue
		}
		
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data != "[DONE]" {
				eventData = data
			} else {
				break
			}
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

