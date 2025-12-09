# 消息格式转换系统

## 概述

消息格式转换系统负责在不同格式之间转换消息：
- **平台消息格式**：飞书、企微等平台的原生消息格式
- **统一消息格式**：系统内部使用的统一消息格式
- **Agent 格式**：Dify、Coze 等 Agent 平台的消息格式

## 转换流程

```
平台消息 → 统一消息格式 → Agent 请求格式 → Agent 响应格式 → 统一消息格式 → 平台消息格式
```

## 核心组件

### 1. Converter（转换器）

`converter.go` 提供了完整的消息格式转换功能：

- **ToAgentRequest**: 将统一消息格式转换为 Agent 请求格式
- **FromAgentResponse**: 将 Agent 响应格式转换为统一消息格式
- **ToPlatformMessage**: 将统一消息格式转换为平台消息格式
- **BuildDifyRequest**: 构建 Dify API 请求
- **BuildCozeRequest**: 构建 Coze API 请求
- **ParseDifyResponse**: 解析 Dify API 响应
- **ParseCozeResponse**: 解析 Coze API 响应
- **FormatForLark**: 格式化飞书消息
- **FormatForWeCom**: 格式化企微消息
- **SplitLongText**: 分割长文本（用于平台限制）
- **MergeMessages**: 合并多个消息（用于流式响应）

### 2. Message（统一消息格式）

定义在 `queue.go` 中：

```go
type Message struct {
    Platform    string            // 平台标识
    SessionID   string            // 会话ID
    UserID      string            // 用户ID
    Content     string            // 消息内容
    MessageType string            // 消息类型
    Metadata    map[string]string // 元数据
    Timestamp   int64             // 时间戳
}
```

### 3. AgentRequest（Agent 请求格式）

定义在 `converter.go` 中：

```go
type AgentRequest struct {
    Query        string                   // 文本查询
    ImageURLs    []string                 // 图片 URL 列表
    SessionID    string                   // 会话 ID
    UserID       string                   // 用户 ID
    SystemPrompt string                   // 系统提示词
    Contexts     []map[string]interface{} // 历史上下文
    Metadata     map[string]string        // 元数据
}
```

### 4. AgentResponse（Agent 响应格式）

定义在 `converter.go` 中：

```go
type AgentResponse struct {
    Content   string            // 文本内容
    ImageURLs []string          // 图片 URL 列表
    Metadata  map[string]string // 元数据
}
```

## 使用示例

### 平台消息 → 统一消息格式

```go
// 在平台适配器中
msg := &message.Message{
    Platform:    "lark",
    SessionID:   sessionID,
    UserID:      userID,
    Content:     textContent,
    MessageType: "text",
    Metadata:    map[string]string{...},
}
```

### 统一消息格式 → Agent 请求格式

```go
converter := message.NewConverter()
agentReq := converter.ToAgentRequest(msg)
```

### Agent 响应格式 → 统一消息格式

```go
agentResp := &message.AgentResponse{
    Content: "回复内容",
}
responseMsg := converter.FromAgentResponse(agentResp, originalMsg)
```

### 统一消息格式 → 平台消息格式

```go
// 在平台适配器中
platformMsg := converter.ToPlatformMessage(msg)
// 然后使用平台特定的发送方法
```

## 支持的消息类型

- **文本消息** (text)
- **图片消息** (image) - 支持 base64 和 URL
- **语音消息** (voice)
- **文件消息** (file)
- **视频消息** (video)

## 平台特定处理

### 飞书
- 使用富文本格式（post）
- 支持 @ 用户
- 图片需要上传获取 image_key

### 企微
- 文本消息限制 2048 字符，自动分割
- 图片需要上传获取 media_id
- 支持语音消息（AMR 格式）

## 注意事项

1. **图片处理**：base64 图片需要先上传到平台获取 media_id/image_key
2. **长文本分割**：企微等平台有文本长度限制，需要自动分割
3. **流式响应**：Agent 返回的流式响应需要合并后再发送
4. **元数据保留**：转换过程中保留平台特定的元数据

