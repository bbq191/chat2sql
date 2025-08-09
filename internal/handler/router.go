package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"net/http"
)

// RouterConfig 路由配置结构
type RouterConfig struct {
	AuthHandler       *AuthHandler
	UserHandler       *UserHandler
	SQLHandler        *SQLHandler
	ConnectionHandler *ConnectionHandler
	AuthMiddleware    AuthMiddleware // JWT认证中间件接口
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
	setupSystemRoutes(r)
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
	}
}

// setupSystemRoutes 配置系统级路由
func setupSystemRoutes(r *gin.Engine) {
	// 健康检查端点
	r.GET("/health", healthCheck)
	r.GET("/ready", readinessCheck)
	
	// 系统信息端点
	r.GET("/version", versionInfo)
	
	// Prometheus指标端点（由中间件提供）
	// r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// healthCheck 健康检查处理器
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": "2024-01-08T12:00:00Z", // TODO: 使用实际时间
		"service":   "chat2sql-api",
		"version":   "0.1.0", // TODO: 从配置获取版本号
	})
}

// readinessCheck 就绪状态检查
func readinessCheck(c *gin.Context) {
	// TODO: 检查数据库连接、依赖服务状态
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"checks": gin.H{
			"database": "ok",
			"cache":    "ok",
		},
	})
}

// versionInfo 版本信息
func versionInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":  "chat2sql-api",
		"version":  "0.1.0",
		"go_version": "1.24+",
		"build_time": "2024-01-08T12:00:00Z", // TODO: 编译时注入
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