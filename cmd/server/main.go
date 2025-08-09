package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"chat2sql-go/internal/auth"
	"chat2sql-go/internal/config"
	"chat2sql-go/internal/database"
	"chat2sql-go/internal/handler"
	"chat2sql-go/internal/middleware"
	"chat2sql-go/internal/metrics"
	"chat2sql-go/internal/repository/postgres"
)

func main() {
	// 初始化日志
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting Chat2SQL Server", 
		zap.String("version", "0.1.0"),
		zap.String("go_version", runtime.Version()))

	// 初始化配置
	dbConfig := config.DefaultDatabaseConfig()
	jwtConfig := auth.DefaultJWTConfig()
	metricsConfig := metrics.DefaultMetricsConfig()
	
	// 初始化数据库连接
	dbManager, err := database.NewManager(dbConfig, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer dbManager.Close()

	// 验证数据库连接
	if err := dbManager.HealthCheck(context.Background()); err != nil {
		logger.Fatal("Database health check failed", zap.Error(err))
	}
	logger.Info("Database connection established successfully")

	// 初始化Repository层
	repo := postgres.NewPostgreSQLRepository(dbManager.GetPool(), logger)
	
	// 初始化JWT服务
	jwtService, err := auth.NewJWTService(jwtConfig, logger)
	if err != nil {
		logger.Fatal("Failed to initialize JWT service", zap.Error(err))
	}
	
	// 保存JWT密钥（开发阶段）
	if err := jwtService.SaveKeysToFile(jwtConfig.PrivateKeyPath, jwtConfig.PublicKeyPath); err != nil {
		logger.Warn("Failed to save JWT keys", zap.Error(err))
	}

	// 初始化Prometheus指标
	prometheusMetrics := metrics.NewPrometheusMetrics(metricsConfig, logger)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(repo.UserRepo(), jwtService, logger)
	userHandler := handler.NewUserHandler(repo.UserRepo(), logger)
	sqlHandler := handler.NewSQLHandler(repo.QueryHistoryRepo(), repo.ConnectionRepo(), logger)
	connectionHandler := handler.NewConnectionHandler(repo.ConnectionRepo(), repo.SchemaRepo(), logger)

	// 初始化中间件
	authMiddleware := middleware.NewAuthMiddleware(jwtService, logger)
	middlewareConfig := middleware.DefaultMiddlewareConfig(logger)

	// 初始化Gin路由器
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode) // 生产模式
	}
	
	r := gin.New()

	// 配置全局中间件
	middleware.SetupMiddleware(r, middlewareConfig)
	
	// 添加Prometheus指标中间件
	r.Use(prometheusMetrics.HTTPMetricsMiddleware())

	// 配置路由
	routerConfig := &handler.RouterConfig{
		AuthHandler:       authHandler,
		UserHandler:       userHandler,
		SQLHandler:        sqlHandler,
		ConnectionHandler: connectionHandler,
		AuthMiddleware:    authMiddleware,
	}
	
	handler.SetupRoutes(r, routerConfig)

	// 添加Prometheus指标端点
	r.GET("/metrics", prometheusMetrics.GetMetricsHandler())

	// 更新路由中的JWT认证中间件
	updateRoutesWithJWTAuth(r, authMiddleware)

	// 启动系统指标收集
	go collectSystemMetrics(prometheusMetrics, logger)

	// 启动HTTP服务器
	srv := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// 启动服务器
	go func() {
		logger.Info("Chat2SQL server starting",
			zap.String("addr", srv.Addr),
			zap.String("mode", gin.Mode()))
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 优雅关闭处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 设置关闭超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	} else {
		logger.Info("Server gracefully stopped")
	}

	// 关闭数据库连接
	dbManager.Close()
	logger.Info("Database connections closed")

	logger.Info("Chat2SQL server exited")
}

// updateRoutesWithJWTAuth 更新路由以使用JWT认证中间件
func updateRoutesWithJWTAuth(r *gin.Engine, authMiddleware *middleware.AuthMiddleware) {
	// 这是一个临时方案，理想情况下应该在router.go中直接配置
	// TODO: 重构路由配置以支持依赖注入
	
	// 为受保护的路由组添加JWT认证
	v1 := r.Group("/api/v1")
	protected := v1.Group("/")
	protected.Use(authMiddleware.JWTAuth())
	// 注意：这里的路由已经在handler.SetupRoutes中配置，
	// 这个函数主要是为了演示如何集成JWT中间件
}

// collectSystemMetrics 收集系统指标
func collectSystemMetrics(pm *metrics.PrometheusMetrics, logger *zap.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 收集内存统计
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// 更新系统指标
		pm.UpdateSystemMetrics(int64(m.Alloc), runtime.NumGoroutine())
		
		logger.Debug("System metrics updated",
			zap.Uint64("memory_alloc", m.Alloc),
			zap.Int("goroutines", runtime.NumGoroutine()))
	}
}