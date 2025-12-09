package api

import (
	"net/http"
	"sync"

	"xia_adpter/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server API 服务器
type Server struct {
	cfg      *config.Config
	configPath string
	logger   *zap.Logger
	mu       sync.RWMutex
}

// NewServer 创建新的 API 服务器
func NewServer(cfg *config.Config, configPath string, logger *zap.Logger) *Server {
	return &Server{
		cfg:        cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// SetupRoutes 设置路由
func (s *Server) SetupRoutes(router *gin.Engine) {
	// 静态文件服务（使用绝对路径）
	router.Static("/static", "web/static")
	router.LoadHTMLGlob("web/templates/*")

	// 首页
	router.GET("/", s.handleIndex)

	// API 路由
	api := router.Group("/api/v1")
	{
		api.GET("/config", s.getConfig)
		api.PUT("/config", s.updateConfig)
		api.GET("/status", s.getStatus)
	}
}

// handleIndex 处理首页
func (s *Server) handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Xia Adapter 管理面板",
	})
}

// getConfig 获取配置
func (s *Server) getConfig(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    s.cfg,
	})
}

// updateConfig 更新配置
func (s *Server) updateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	s.mu.Lock()
	*s.cfg = newCfg
	s.mu.Unlock()

	// 保存到文件
	if err := config.Save(&newCfg, s.configPath); err != nil {
		s.logger.Error("Failed to save config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "保存配置失败: " + err.Error(),
		})
		return
	}

	s.logger.Info("Config updated successfully")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已保存",
	})
}

// getStatus 获取服务状态
func (s *Server) getStatus(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"lark": gin.H{
				"enabled": s.cfg.Platform.Lark.Enabled,
			},
			"wecom": gin.H{
				"enabled": s.cfg.Platform.WeCom.Enabled,
			},
			"dify": gin.H{
				"enabled": s.cfg.Agent.Dify.Enabled,
			},
			"coze": gin.H{
				"enabled": s.cfg.Agent.Coze.Enabled,
			},
		},
	})
}

