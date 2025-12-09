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

## License

本项目采用 [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0) 许可证。

### 许可证说明

- 本项目参考 [AstrBot](https://github.com/AstrBotDevs/AstrBot) 的实现思路
- 由于本项目是网络服务（适配器服务），采用 AGPL-3.0 许可证以确保所有用户（包括通过网络访问的用户）都能获得源代码
- AGPL-3.0 是 GPL-3.0 的扩展版本，专门针对网络服务器软件设计

### 商业使用

AGPL-3.0 允许商业使用，但需要遵守以下要求：
- 必须提供源代码（包括通过网络提供服务的源代码）
- 必须保留版权声明和许可证
- 衍生作品也必须使用 AGPL-3.0 许可证
- **通过网络提供服务时，也必须向用户提供源代码**

### AGPL-3.0 vs GPL-3.0

AGPL-3.0 与 GPL-3.0 的主要区别：
- **GPL-3.0**: 只有分发软件时才需要开源
- **AGPL-3.0**: 分发软件**或通过网络提供服务**时都需要开源

对于网络服务项目，AGPL-3.0 确保所有用户（包括通过网络访问的用户）都能获得源代码。

更多信息请参阅 [LICENSE](LICENSE) 文件。

