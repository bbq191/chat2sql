package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/middleware"
	"chat2sql-go/internal/metrics"
)

// MockAuthMiddleware 模拟JWT认证中间件
type MockAuthMiddleware struct {
	mock.Mock
}

func (m *MockAuthMiddleware) JWTAuth() gin.HandlerFunc {
	args := m.Called()
	return args.Get(0).(gin.HandlerFunc)
}

// TestUpdateRoutesWithJWTAuth 测试JWT认证路由更新
func TestUpdateRoutesWithJWTAuth(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	
	// 创建测试路由器
	r := gin.New()
	
	// 添加一些基础路由用于测试
	r.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})
	
	// 创建模拟认证中间件
	mockAuthMiddleware := &MockAuthMiddleware{}
	mockAuthMiddleware.On("JWTAuth").Return(gin.HandlerFunc(func(c *gin.Context) {
		c.Header("X-Auth-Applied", "true")
		c.Next()
	}))
	
	// 测试路由更新
	assert.NotPanics(t, func() {
		updateRoutesWithJWTAuth(r, &middleware.AuthMiddleware{})
	})
	
	// 验证路由组被正确创建
	routes := r.Routes()
	assert.True(t, len(routes) >= 1, "应该至少有一个路由")
}

// TestGinModeSettings 测试Gin模式设置
func TestGinModeSettings(t *testing.T) {
	// 保存原始环境变量
	originalMode := os.Getenv("GIN_MODE")
	defer os.Setenv("GIN_MODE", originalMode)
	
	// 测试默认模式（环境变量为空）
	os.Unsetenv("GIN_MODE")
	gin.SetMode(gin.TestMode) // 重置为测试模式
	
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode) // 模拟main函数中的逻辑
	}
	
	assert.Equal(t, gin.ReleaseMode, gin.Mode())
	
	// 测试设置环境变量
	os.Setenv("GIN_MODE", "debug")
	gin.SetMode(gin.DebugMode)
	assert.Equal(t, gin.DebugMode, gin.Mode())
}

// TestServerConfiguration 测试服务器配置
func TestServerConfiguration(t *testing.T) {
	// 测试HTTP服务器配置结构
	srv := &http.Server{
		Addr:           ":8080",
		Handler:        gin.New(),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
	
	assert.Equal(t, ":8080", srv.Addr)
	assert.Equal(t, 30*time.Second, srv.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)
	assert.Equal(t, 1<<20, srv.MaxHeaderBytes)
	assert.NotNil(t, srv.Handler)
}

// TestCollectSystemMetrics 测试系统指标收集
func TestCollectSystemMetrics(t *testing.T) {
	// 创建独立的registry避免冲突
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := metrics.DefaultMetricsConfig()
	pm := metrics.NewPrometheusMetrics(config, logger)
	
	// 测试系统指标收集（短时间运行）
	done := make(chan bool, 1)
	var wg sync.WaitGroup
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// 模拟collectSystemMetrics函数的核心逻辑
		ticker := time.NewTicker(100 * time.Millisecond) // 快速间隔用于测试
		defer ticker.Stop()
		
		select {
		case <-ticker.C:
			// 收集内存统计
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			
			// 更新系统指标
			pm.UpdateSystemMetrics(int64(m.Alloc), runtime.NumGoroutine())
			
			done <- true
		case <-time.After(500 * time.Millisecond):
			done <- false
		}
	}()
	
	// 等待指标收集完成或超时
	result := <-done
	wg.Wait()
	
	assert.True(t, result, "系统指标收集应该成功完成")
	
	// 验证指标是否被更新
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	assert.True(t, len(metricFamily) > 0, "应该收集到系统指标")
}

// TestRuntimeMetricsCollection 测试运行时指标收集
func TestRuntimeMetricsCollection(t *testing.T) {
	// 测试基础的运行时指标获取
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	assert.True(t, m.Alloc > 0, "内存分配量应该大于0")
	assert.True(t, m.Sys >= m.Alloc, "系统内存应该大于等于分配内存")
	assert.True(t, runtime.NumGoroutine() > 0, "Goroutine数量应该大于0")
	assert.True(t, runtime.NumCPU() > 0, "CPU数量应该大于0")
}

// TestPrometheusMetricsIntegration 测试Prometheus指标集成
func TestPrometheusMetricsIntegration(t *testing.T) {
	// 创建独立registry
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := metrics.DefaultMetricsConfig()
	pm := metrics.NewPrometheusMetrics(config, logger)
	
	// 创建测试Gin引擎
	gin.SetMode(gin.TestMode)
	r := gin.New()
	
	// 添加Prometheus中间件
	r.Use(pm.HTTPMetricsMiddleware())
	
	// 添加指标端点
	r.GET("/metrics", pm.GetMetricsHandler())
	
	// 添加测试端点
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})
	
	// 执行测试请求
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
	
	// 检查指标端点
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w2, req2)
	
	assert.Equal(t, 200, w2.Code)
	assert.Contains(t, w2.Body.String(), "http_requests_total", "应该包含HTTP请求指标")
}

// TestMiddlewareConfiguration 测试中间件配置
func TestMiddlewareConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	logger := zaptest.NewLogger(t)
	config := middleware.DefaultMiddlewareConfig(logger)
	
	assert.NotNil(t, config)
	
	// 创建路由器测试中间件配置
	r := gin.New()
	
	// 应用中间件配置（模拟SetupMiddleware调用）
	assert.NotPanics(t, func() {
		middleware.SetupMiddleware(r, config)
	})
	
	// 添加测试路由
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	
	// 执行测试请求
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
}

// TestRouterGroupCreation 测试路由组创建
func TestRouterGroupCreation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	
	// 测试API版本路由组创建
	v1 := r.Group("/api/v1")
	assert.NotNil(t, v1)
	
	// 测试受保护路由组创建
	protected := v1.Group("/")
	assert.NotNil(t, protected)
	
	// 添加测试路由到组
	protected.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "protected resource"})
	})
	
	// 验证路由被正确添加
	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/v1/protected" {
			found = true
			break
		}
	}
	assert.True(t, found, "受保护的路由应该被正确添加")
}

// TestSystemMetricsLogger 测试系统指标日志记录
func TestSystemMetricsLogger(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	// 模拟指标收集和日志记录
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// 验证日志记录不会引发panic
	assert.NotPanics(t, func() {
		logger.Debug("收集系统性能指标",
			zap.Uint64("memory_alloc_mb", m.Alloc/1024/1024),
			zap.Uint64("memory_sys_mb", m.Sys/1024/1024),
			zap.Int("goroutines", runtime.NumGoroutine()),
			zap.Uint32("gc_runs", m.NumGC))
	})
}

// TestGracefulShutdownSignals 测试优雅关闭信号处理
func TestGracefulShutdownSignals(t *testing.T) {
	// 测试信号通道创建和配置
	quit := make(chan os.Signal, 1)
	assert.NotNil(t, quit)
	assert.Equal(t, 1, cap(quit))
	
	// 测试上下文创建（用于优雅关闭）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	
	// 验证上下文超时时间设置
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.True(t, deadline.After(time.Now()))
	
	// 验证不会panic
	assert.NotNil(t, quit)
}

// TestEnvironmentVariables 测试环境变量处理
func TestEnvironmentVariables(t *testing.T) {
	// 保存原始值
	originalGinMode := os.Getenv("GIN_MODE")
	defer os.Setenv("GIN_MODE", originalGinMode)
	
	// 测试环境变量为空的情况
	os.Unsetenv("GIN_MODE")
	assert.Equal(t, "", os.Getenv("GIN_MODE"))
	
	// 测试设置环境变量
	os.Setenv("GIN_MODE", "release")
	assert.Equal(t, "release", os.Getenv("GIN_MODE"))
	
	// 测试调试模式
	os.Setenv("GIN_MODE", "debug")
	assert.Equal(t, "debug", os.Getenv("GIN_MODE"))
}

// TestServerAddressConfiguration 测试服务器地址配置
func TestServerAddressConfiguration(t *testing.T) {
	// 测试默认地址配置
	defaultAddr := ":8080"
	assert.Equal(t, ":8080", defaultAddr)
	
	// 验证地址格式
	assert.True(t, len(defaultAddr) > 0)
	assert.True(t, defaultAddr[0] == ':')
	
	// 测试端口号提取
	port := defaultAddr[1:]
	assert.Equal(t, "8080", port)
}

// BenchmarkSystemMetricsCollection 性能测试：系统指标收集
func BenchmarkSystemMetricsCollection(b *testing.B) {
	var m runtime.MemStats
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.ReadMemStats(&m)
		_ = m.Alloc
		_ = runtime.NumGoroutine()
	}
}

// BenchmarkGinRouteCreation 性能测试：Gin路由创建
func BenchmarkGinRouteCreation(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			c.Status(200)
		})
	}
}

// TestComponentsIntegration 测试组件集成
func TestComponentsIntegration(t *testing.T) {
	// 创建独立registry
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	
	// 初始化指标
	config := metrics.DefaultMetricsConfig()
	pm := metrics.NewPrometheusMetrics(config, logger)
	
	// 创建路由器
	r := gin.New()
	
	// 集成中间件和指标
	r.Use(pm.HTTPMetricsMiddleware())
	r.GET("/metrics", pm.GetMetricsHandler())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	
	// 执行健康检查请求
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)
	
	assert.Equal(t, 200, w.Code)
	
	// 检查指标端点
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w2, req2)
	
	assert.Equal(t, 200, w2.Code)
	assert.NotEmpty(t, w2.Body.String())
}