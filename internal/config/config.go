package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server" json:"server"`
	Platform PlatformConfig `mapstructure:"platform" json:"platform"`
	Agent    AgentConfig    `mapstructure:"agent" json:"agent"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

// PlatformConfig 平台配置
type PlatformConfig struct {
	Lark  LarkConfig  `mapstructure:"lark" json:"lark"`
	WeCom WeComConfig `mapstructure:"wecom" json:"wecom"`
}

// LarkConfig 飞书配置
type LarkConfig struct {
	Enabled   bool   `mapstructure:"enabled" json:"enabled"`
	AppID     string `mapstructure:"app_id" json:"app_id"`
	AppSecret string `mapstructure:"app_secret" json:"app_secret"`
	Domain    string `mapstructure:"domain" json:"domain"` // feishu.cn 或 larksuite.com
	BotName   string `mapstructure:"bot_name" json:"bot_name"`
}

// WeComConfig 企微配置
type WeComConfig struct {
	Enabled       bool   `mapstructure:"enabled" json:"enabled"`
	CorpID        string `mapstructure:"corp_id" json:"corp_id"`
	Secret        string `mapstructure:"secret" json:"secret"`
	Token         string `mapstructure:"token" json:"token"`
	EncodingAESKey string `mapstructure:"encoding_aes_key" json:"encoding_aes_key"`
	Port          int    `mapstructure:"port" json:"port"`
	Host          string `mapstructure:"host" json:"host"`
	AgentID       int    `mapstructure:"agent_id" json:"agent_id"` // 应用 AgentID
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Dify DifyConfig `mapstructure:"dify" json:"dify"`
	Coze CozeConfig `mapstructure:"coze" json:"coze"`
}

// DifyConfig Dify 配置
type DifyConfig struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled"`
	APIKey   string `mapstructure:"api_key" json:"api_key"`
	APIBase  string `mapstructure:"api_base" json:"api_base"`
	AppID    string `mapstructure:"app_id" json:"app_id"` // Dify 应用 ID
	UserID   string `mapstructure:"user_id" json:"user_id"`
}

// CozeConfig Coze 配置
type CozeConfig struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	APIKey  string `mapstructure:"api_key" json:"api_key"`
	APIBase string `mapstructure:"api_base" json:"api_base"`
	BotID   string `mapstructure:"bot_id" json:"bot_id"`
	UserID  string `mapstructure:"user_id" json:"user_id"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 支持环境变量
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 从环境变量覆盖配置
	overrideFromEnv(&cfg)

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("platform.lark.domain", "feishu.cn")
	viper.SetDefault("platform.wecom.host", "0.0.0.0")
	viper.SetDefault("platform.wecom.port", 8888)
	viper.SetDefault("agent.dify.api_base", "https://api.dify.ai/v1")
	viper.SetDefault("agent.coze.api_base", "https://api.coze.cn")
}

func overrideFromEnv(cfg *Config) {
	if appID := os.Getenv("LARK_APP_ID"); appID != "" {
		cfg.Platform.Lark.AppID = appID
	}
	if appSecret := os.Getenv("LARK_APP_SECRET"); appSecret != "" {
		cfg.Platform.Lark.AppSecret = appSecret
	}
	if corpID := os.Getenv("WECOM_CORP_ID"); corpID != "" {
		cfg.Platform.WeCom.CorpID = corpID
	}
	if secret := os.Getenv("WECOM_SECRET"); secret != "" {
		cfg.Platform.WeCom.Secret = secret
	}
	if difyKey := os.Getenv("DIFY_API_KEY"); difyKey != "" {
		cfg.Agent.Dify.APIKey = difyKey
	}
	if cozeKey := os.Getenv("COZE_API_KEY"); cozeKey != "" {
		cfg.Agent.Coze.APIKey = cozeKey
	}
}

// Save 保存配置到文件
func Save(cfg *Config, configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置配置值
	viper.Set("server.host", cfg.Server.Host)
	viper.Set("server.port", cfg.Server.Port)

	// 平台配置
	viper.Set("platform.lark.enabled", cfg.Platform.Lark.Enabled)
	viper.Set("platform.lark.app_id", cfg.Platform.Lark.AppID)
	viper.Set("platform.lark.app_secret", cfg.Platform.Lark.AppSecret)
	viper.Set("platform.lark.domain", cfg.Platform.Lark.Domain)
	viper.Set("platform.lark.bot_name", cfg.Platform.Lark.BotName)

	viper.Set("platform.wecom.enabled", cfg.Platform.WeCom.Enabled)
	viper.Set("platform.wecom.corp_id", cfg.Platform.WeCom.CorpID)
	viper.Set("platform.wecom.secret", cfg.Platform.WeCom.Secret)
	viper.Set("platform.wecom.token", cfg.Platform.WeCom.Token)
	viper.Set("platform.wecom.encoding_aes_key", cfg.Platform.WeCom.EncodingAESKey)
	viper.Set("platform.wecom.host", cfg.Platform.WeCom.Host)
	viper.Set("platform.wecom.port", cfg.Platform.WeCom.Port)
	viper.Set("platform.wecom.agent_id", cfg.Platform.WeCom.AgentID)

	// Agent 配置
	viper.Set("agent.dify.enabled", cfg.Agent.Dify.Enabled)
	viper.Set("agent.dify.api_key", cfg.Agent.Dify.APIKey)
	viper.Set("agent.dify.api_base", cfg.Agent.Dify.APIBase)
	viper.Set("agent.dify.app_id", cfg.Agent.Dify.AppID)
	viper.Set("agent.dify.user_id", cfg.Agent.Dify.UserID)

	viper.Set("agent.coze.enabled", cfg.Agent.Coze.Enabled)
	viper.Set("agent.coze.api_key", cfg.Agent.Coze.APIKey)
	viper.Set("agent.coze.api_base", cfg.Agent.Coze.APIBase)
	viper.Set("agent.coze.bot_id", cfg.Agent.Coze.BotID)
	viper.Set("agent.coze.user_id", cfg.Agent.Coze.UserID)

	// 写入文件
	return viper.WriteConfig()
}

