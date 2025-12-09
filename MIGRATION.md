# 项目迁移说明

## 迁移信息

本项目已从 AstrBot 项目中独立出来，迁移到 `/Users/shoushinya/xia_adpter` 目录。

## 迁移内容

### 1. 代码迁移
- ✅ 所有 Go 源代码文件
- ✅ 配置文件示例
- ✅ 文档文件（README.md, MESSAGE_CONVERSION.md）
- ✅ Go 模块文件（go.mod, go.sum）

### 2. 模块名更新
- **旧模块名**: `agent-gateway`
- **新模块名**: `xia_adpter`
- ✅ 已更新所有导入路径

### 3. 依赖关系
- ✅ 已完全剥离与 AstrBot 的依赖关系
- ✅ 所有依赖已通过 `go mod tidy` 验证
- ✅ 项目可独立编译和运行

## 项目结构

```
xia_adpter/
├── cmd/
│   └── server/          # 主程序入口
├── internal/
│   ├── agent/           # Agent 客户端（Dify, Coze）
│   ├── config/          # 配置管理
│   ├── message/         # 消息格式转换
│   ├── pipeline/        # 消息处理管道
│   └── platform/        # 平台适配器（Lark, WeCom）
├── configs/             # 配置文件
├── go.mod
├── go.sum
└── README.md
```

## 验证

### 编译验证
```bash
cd /Users/shoushinya/xia_adpter
go build ./...
```

### 运行验证
```bash
go run cmd/server/main.go
```

## 与原项目的区别

1. **独立模块**: 不再依赖 AstrBot 项目
2. **模块名**: 从 `agent-gateway` 改为 `xia_adpter`
3. **独立运行**: 可以独立部署和运行
4. **配置独立**: 使用自己的配置文件

## 后续工作

- [ ] 添加单元测试
- [ ] 添加集成测试
- [ ] 完善文档
- [ ] 添加 CI/CD 配置

