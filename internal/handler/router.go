package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"chat2sql-go/internal/service"
)

// RouterConfig 路由配置结构
type RouterConfig struct {
	AuthHandler       *AuthHandler
	UserHandler       *UserHandler
	SQLHandler        *SQLHandler
	ConnectionHandler *ConnectionHandler
	AIHandler         *AIHandler          // P1阶段新增: AI服务处理器
	AuthMiddleware    AuthMiddleware       // JWT认证中间件接口
	HealthService     service.HealthServiceInterface // 健康检查服务接口
}

// AuthMiddleware JWT认证中间件接口
type AuthMiddleware interface {
	JWTAuth() gin.HandlerFunc
}

// SetupRoutes 配置所有API路由
// 实现企业级RESTful API设计模式，支持版本管理和中间件链
func SetupRoutes(r *gin.Engine, config *RouterConfig) {
	// 全局中间件
	setupGlobalMiddleware(r)
	
	// API版本管理
	v1 := r.Group("/api/v1")
	{
		// 公开API - 无需认证
		setupPublicRoutes(v1, config)
		
		// 受保护API - 需要JWT认证
		setupProtectedRoutes(v1, config)
	}
	
	// 健康检查和系统监控端点
	setupSystemRoutes(r, config)
}

// setupGlobalMiddleware 配置全局中间件
func setupGlobalMiddleware(r *gin.Engine) {
	// 中间件顺序很重要，按照请求处理流程排列
	r.Use(gin.Recovery())     // 1. 恢复panic，防止服务崩溃
	r.Use(gin.Logger())       // 2. 请求日志记录
	r.Use(corsMiddleware())   // 3. 跨域处理
	r.Use(securityHeaders())  // 4. 安全头设置
}

// setupPublicRoutes 配置公开API路由
func setupPublicRoutes(rg *gin.RouterGroup, config *RouterConfig) {
	// 认证相关API - 不需要JWT Token
	auth := rg.Group("/auth")
	{
		auth.POST("/register", config.AuthHandler.Register)   // 用户注册
		auth.POST("/login", config.AuthHandler.Login)        // 用户登录
		auth.POST("/refresh", config.AuthHandler.RefreshToken) // Token刷新
	}
}

// setupProtectedRoutes 配置受保护的API路由
func setupProtectedRoutes(rg *gin.RouterGroup, config *RouterConfig) {
	// 需要JWT认证的API分组
	protected := rg.Group("/")
	if config.AuthMiddleware != nil {
		protected.Use(config.AuthMiddleware.JWTAuth())
	}
	{
		// 用户管理API
		users := protected.Group("/users")
		{
			users.GET("/profile", config.UserHandler.GetProfile)        // 获取用户资料
			users.PUT("/profile", config.UserHandler.UpdateProfile)     // 更新用户资料
			users.POST("/change-password", config.UserHandler.ChangePassword) // 修改密码
		}
		
		// SQL查询API
		sql := protected.Group("/sql")
		{
			sql.POST("/execute", config.SQLHandler.ExecuteSQL)          // 执行SQL查询
			sql.GET("/history", config.SQLHandler.GetQueryHistory)      // 查询历史
			sql.GET("/history/:id", config.SQLHandler.GetQueryById)     // 获取特定查询
			sql.POST("/validate", config.SQLHandler.ValidateSQL)        // SQL语法验证
		}
		
		// 数据库连接管理API
		connections := protected.Group("/connections")
		{
			connections.POST("/", config.ConnectionHandler.CreateConnection)         // 创建连接
			connections.GET("/", config.ConnectionHandler.ListConnections)          // 连接列表
			connections.GET("/:id", config.ConnectionHandler.GetConnection)         // 获取连接详情
			connections.PUT("/:id", config.ConnectionHandler.UpdateConnection)      // 更新连接
			connections.DELETE("/:id", config.ConnectionHandler.DeleteConnection)   // 删除连接
			connections.POST("/:id/test", config.ConnectionHandler.TestConnection)  // 测试连接
			connections.GET("/:id/schema", config.ConnectionHandler.GetSchema)      // 获取数据库结构
		}
		
		// P1阶段新增：AI智能查询API
		if config.AIHandler != nil {
			ai := protected.Group("/ai")
			{
				ai.POST("/chat2sql", config.AIHandler.Chat2SQL)           // 自然语言转SQL
				ai.POST("/feedback", config.AIHandler.SubmitFeedback)     // 提交用户反馈
				ai.GET("/stats", config.AIHandler.GetAIStats)             // 获取AI服务统计
			}
		}
	}
}

// setupSystemRoutes 配置系统级路由
func setupSystemRoutes(r *gin.Engine, config *RouterConfig) {
	if config.HealthService != nil {
		// 健康检查端点
		r.GET("/health", healthCheckHandler(config.HealthService))
		r.GET("/ready", readinessCheckHandler(config.HealthService))
		
		// 系统信息端点
		r.GET("/version", versionInfoHandler(config.HealthService))
	} else {
		// 降级到简单的健康检查
		r.GET("/health", simpleHealthCheck)
		r.GET("/ready", simpleReadinessCheck)
		r.GET("/version", simpleVersionInfo)
	}
	
	// Prometheus指标端点（由中间件提供）
	// r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// healthCheckHandler 健康检查处理器
func healthCheckHandler(healthService service.HealthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := healthService.CheckHealth(c.Request.Context())
		
		// 根据健康状态设置HTTP状态码
		httpStatus := http.StatusOK
		if result.Status == "unhealthy" {
			httpStatus = http.StatusServiceUnavailable
		} else if result.Status == "degraded" {
			httpStatus = http.StatusOK // 降级状态仍返回200，但在响应中标明
		}
		
		c.JSON(httpStatus, result)
	}
}

// readinessCheckHandler 就绪状态检查处理器
func readinessCheckHandler(healthService service.HealthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := healthService.CheckReadiness(c.Request.Context())
		
		// 就绪检查更严格，只有完全健康才返回200
		httpStatus := http.StatusOK
		if result.Status != "healthy" {
			httpStatus = http.StatusServiceUnavailable
		}
		
		c.JSON(httpStatus, result)
	}
}

// versionInfoHandler 版本信息处理器
func versionInfoHandler(healthService service.HealthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		versionInfo := healthService.GetVersionInfo()
		c.JSON(http.StatusOK, versionInfo)
	}
}

// 简单的健康检查处理器（降级版本）
func simpleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "chat2sql-api",
		"version":   "0.1.0",
	})
}

func simpleReadinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"components": gin.H{
			"service": "ok",
		},
	})
}

func simpleVersionInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":    "chat2sql-api",
		"version":    "0.1.0",
		"go_version": "1.24+",
		"build_time": "development",
	})
}

// corsMiddleware CORS跨域中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		
		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// securityHeaders 安全头中间件
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 安全相关HTTP头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		
		c.Next()
	}
}

// RequestBindingJSON 统一JSON绑定配置
func init() {
	// 配置JSON绑定选项，提高安全性
	binding.EnableDecoderUseNumber = true
	binding.EnableDecoderDisallowUnknownFields = true
}