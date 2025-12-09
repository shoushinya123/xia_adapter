# 管理面板使用说明

## 访问管理面板

启动服务后，在浏览器中访问：

```
http://localhost:8081
```

（如果配置了不同的端口，请使用相应的端口号）

## 功能说明

### 1. 状态栏
页面顶部显示各个平台和 Agent 的启用状态：
- **飞书 (Lark)**: 显示是否已启用
- **企微 (WeCom)**: 显示是否已启用
- **Dify**: 显示是否已启用
- **Coze**: 显示是否已启用

状态每 5 秒自动刷新。

### 2. 平台配置
在"平台配置"标签页中，可以配置：

#### 飞书 (Lark)
- **启用飞书平台**: 勾选以启用
- **App ID**: 飞书应用的 App ID
- **App Secret**: 飞书应用的 App Secret
- **域名**: 选择 `feishu.cn` 或 `larksuite.com`
- **机器人名称**: 机器人显示名称

#### 企微 (WeCom)
- **启用企微平台**: 勾选以启用
- **Corp ID**: 企业微信的 Corp ID
- **Secret**: 企业微信应用的 Secret
- **Token**: 回调验证 Token
- **Encoding AES Key**: 消息加解密的 AES Key
- **Host**: Webhook 服务器监听地址（默认: 0.0.0.0）
- **Port**: Webhook 服务器监听端口（默认: 8888）
- **Agent ID**: 企业微信应用的 Agent ID

### 3. Agent 配置
在"Agent 配置"标签页中，可以配置：

#### Dify Agent
- **启用 Dify Agent**: 勾选以启用
- **API Key**: Dify 的 API Key
- **API Base**: Dify API 地址（默认: https://api.dify.ai/v1）
- **App ID**: Dify 应用 ID
- **User ID**: 用户 ID（默认: default_user）

#### Coze Agent
- **启用 Coze Agent**: 勾选以启用
- **API Key**: Coze 的 API Key
- **API Base**: Coze API 地址（默认: https://api.coze.cn）
- **Bot ID**: Coze Bot ID
- **User ID**: 用户 ID（默认: default_user）

### 4. 服务器配置
在"服务器配置"标签页中，可以配置：
- **Host**: API 服务器监听地址（默认: 0.0.0.0）
- **Port**: API 服务器监听端口（默认: 8080）

## 操作说明

1. **查看配置**: 页面加载时自动从服务器获取当前配置
2. **修改配置**: 在各个标签页中修改配置项
3. **保存配置**: 点击"保存配置"按钮保存修改
4. **重新加载**: 点击"重新加载"按钮从服务器重新获取配置

## 注意事项

1. **保存后需重启**: 修改配置并保存后，需要重启服务才能使配置生效
2. **敏感信息**: API Key、Secret 等敏感信息会以密码框形式显示，输入时注意安全
3. **配置验证**: 保存配置时，系统会验证配置格式，如有错误会显示错误信息

## API 接口

管理面板使用以下 API 接口：

- `GET /api/v1/config` - 获取配置
- `PUT /api/v1/config` - 更新配置
- `GET /api/v1/status` - 获取服务状态

