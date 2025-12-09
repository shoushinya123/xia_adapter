# Xia Adapter

一个轻量级的 Golang 项目，用于对接飞书、企微平台，并集成 Dify 和 Coze Agent 能力。

本项目参考 [AstrBot](https://github.com/AstrBotDevs/AstrBot) 的实现思路，使用 Go 语言独立重新实现，作为独立的平台适配器服务。

## 功能特性

- ✅ 飞书（Lark）平台对接：支持 WebSocket 连接和消息收发
- ✅ 企微（WeCom）平台对接：支持 HTTP Webhook 回调和消息加解密
- ✅ Dify Agent 集成：支持流式响应和消息处理
- ✅ Coze Agent 集成：支持流式响应和消息处理
- ✅ 统一的消息处理管道

## 项目结构

```
xia_adpter/
├── cmd/                    # 主程序入口
│   └── server/
│       └── main.go
├── internal/                # 内部包
│   ├── platform/           # 平台适配器
│   │   ├── lark/           # 飞书适配器
│   │   └── wecom/          # 企微适配器
│   ├── agent/              # Agent 集成
│   │   ├── dify/           # Dify Agent
│   │   └── coze/           # Coze Agent
│   ├── message/            # 消息处理
│   ├── config/             # 配置管理
│   └── pipeline/          # 消息处理管道
├── configs/                # 配置文件
│   └── config.example.yaml
├── go.mod
└── README.md
```

## 快速开始

### 1. 安装依赖

```bash
go mod download
```

### 2. 配置

复制 `configs/config.example.yaml` 为 `configs/config.yaml` 并填写配置信息。

### 3. 运行

```bash
# 方式1: 直接运行
go run cmd/server/main.go

# 方式2: 构建后运行
go build -o xia_adpter ./cmd/server
./xia_adpter
```

## 配置说明

配置文件位于 `configs/config.yaml`，包含以下配置项：

- `platform`: 平台配置（飞书、企微）
- `agent`: Agent 配置（Dify、Coze）
- `server`: 服务器配置

## 平台和 Agent 验证状态

### ✅ 已验证
- **Dify Agent**: 已验证通过，支持流式响应和会话连续性
- **飞书 (Lark) 平台**: 已验证通过，支持 WebSocket 长连接和消息收发

### ⚠️ 未验证
- **Coze Agent**: 未验证，代码已实现但未测试
- **企微 (WeCom) 平台**: 未验证，代码已实现但未测试

## License

本项目采用 [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0) 许可证。


