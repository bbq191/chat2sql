package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	Logger      *zap.Logger
	RateLimit   *RateLimitConfig
	CORS        *CORSConfig
	Security    *SecurityConfig
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerSecond int           // 每秒请求数限制
	Burst             int           // 突发请求数
	CleanupInterval   time.Duration // 清理间隔
}

// CORSConfig CORS配置
type CORSConfig struct {
	AllowOrigins     []string // 允许的源
	AllowMethods     []string // 允许的HTTP方法
	AllowHeaders     []string // 允许的请求头
	AllowCredentials bool     // 是否允许凭据
	MaxAge           int      // 预检请求缓存时间
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableCSP        bool     // 是否启用内容安全策略
	EnableHSTS       bool     // 是否启用HSTS
	TrustedProxies   []string // 受信任的代理IP
}

// DefaultMiddlewareConfig 默认中间件配置
func DefaultMiddlewareConfig(logger *zap.Logger) *MiddlewareConfig {
	return &MiddlewareConfig{
		Logger: logger,
		RateLimit: &RateLimitConfig{
			RequestsPerSecond: 100,           // 每秒100个请求
			Burst:             200,           // 允许突发200个请求
			CleanupInterval:   5 * time.Minute, // 5分钟清理一次
		},
		CORS: &CORSConfig{
			AllowOrigins:     []string{"*"}, // 开发环境允许所有源
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "X-User-ID"},
			AllowCredentials: true,
			MaxAge:           86400, // 24小时
		},
		Security: &SecurityConfig{
			EnableCSP:      true,
			EnableHSTS:     true,
			TrustedProxies: []string{"127.0.0.1", "::1"},
		},
	}
}

// SetupMiddleware 配置所有中间件
func SetupMiddleware(r *gin.Engine, config *MiddlewareConfig) {
	// 1. 恢复中间件 - 防止panic导致服务崩溃
	r.Use(RecoveryMiddleware(config.Logger))
	
	// 2. 结构化日志中间件
	r.Use(StructuredLogger(config.Logger))
	
	// 3. 安全头中间件
	r.Use(SecurityHeaders(config.Security))
	
	// 4. CORS跨域中间件
	r.Use(CORSMiddleware(config.CORS))
	
	// 5. 请求限流中间件
	r.Use(RateLimitMiddleware(config.RateLimit))
	
	// 6. 指标收集中间件
	r.Use(MetricsMiddleware())
	
	// 7. 请求ID中间件
	r.Use(RequestIDMiddleware())
}

// RecoveryMiddleware 恢复中间件
// 捕获panic并记录详细错误日志，防止服务崩溃
func RecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		if logger != nil {
			logger.Error("Request panic recovered",
				zap.Any("panic", recovered),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			)
		}
		
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "服务器内部错误",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})
}

// StructuredLogger 结构化日志中间件
// 记录每个HTTP请求的详细信息，包括响应时间、状态码等
func StructuredLogger(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			if logger != nil {
				logger.Info("HTTP Request",
					zap.String("method", param.Method),
					zap.String("path", param.Path),
					zap.Int("status", param.StatusCode),
					zap.Duration("latency", param.Latency),
					zap.String("remote_addr", param.ClientIP),
					zap.String("user_agent", param.Request.UserAgent()),
					zap.Int("body_size", param.BodySize),
				)
			}
			return ""
		},
		Output: nil, // 不输出到标准输出，只记录到zap logger
	})
}

// SecurityHeaders 安全头中间件
// 设置各种安全相关的HTTP头，提高应用安全性
func SecurityHeaders(config *SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 基础安全头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// HSTS头（仅HTTPS）
		if config.EnableHSTS && c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		
		// 内容安全策略
		if config.EnableCSP {
			c.Header("Content-Security-Policy", 
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		}
		
		c.Next()
	}
}

// CORSMiddleware CORS跨域中间件
// 处理跨域请求，支持预检请求和实际请求
func CORSMiddleware(config *CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// 设置CORS头
		if len(config.AllowOrigins) > 0 && (config.AllowOrigins[0] == "*" || contains(config.AllowOrigins, origin)) {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		if len(config.AllowMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", joinStrings(config.AllowMethods, ", "))
		}
		
		if len(config.AllowHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", joinStrings(config.AllowHeaders, ", "))
		}
		
		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		
		if config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}
		
		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// RateLimiter 限流器结构
type RateLimiter struct {
	limiters        sync.Map
	rate            rate.Limit
	burst           int
	cleanupInterval time.Duration
	lastCleanup     time.Time
	mu              sync.RWMutex
}

// NewRateLimiter 创建限流器实例
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		rate:            rate.Limit(config.RequestsPerSecond),
		burst:           config.Burst,
		cleanupInterval: config.CleanupInterval,
		lastCleanup:     time.Now(),
	}
	
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	// 定期清理过期的限流器
	rl.cleanup()
	
	// 获取或创建用户专用限流器
	limiterInterface, _ := rl.limiters.LoadOrStore(key, rate.NewLimiter(rl.rate, rl.burst))
	limiter := limiterInterface.(*rate.Limiter)
	
	return limiter.Allow()
}

// cleanup 清理过期的限流器
func (rl *RateLimiter) cleanup() {
	rl.mu.RLock()
	if time.Since(rl.lastCleanup) < rl.cleanupInterval {
		rl.mu.RUnlock()
		return
	}
	rl.mu.RUnlock()
	
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if time.Since(rl.lastCleanup) < rl.cleanupInterval {
		return
	}
	
	// 注意: 未来可实现更智能的清理逻辑，移除长时间未使用的限流器
	rl.lastCleanup = time.Now()
}

// rateLimiter 全局限流器实例
var globalRateLimiter *RateLimiter

// RateLimitMiddleware 请求限流中间件
// 基于用户ID或IP地址进行限流，防止API滥用
func RateLimitMiddleware(config *RateLimitConfig) gin.HandlerFunc {
	if globalRateLimiter == nil {
		globalRateLimiter = NewRateLimiter(config)
	}
	
	return func(c *gin.Context) {
		// 获取限流键值：优先使用用户ID，否则使用IP地址
		var limitKey string
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			limitKey = "user:" + userID
		} else {
			limitKey = "ip:" + c.ClientIP()
		}
		
		// 检查是否允许请求
		if !globalRateLimiter.Allow(limitKey) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "请求频率超过限制，请稍后重试",
				"retry_after": 60, // 建议60秒后重试
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// RequestMetrics 请求指标统计
type RequestMetrics struct {
	requestTotal     sync.Map // key: method_endpoint_status, value: count
	requestDuration  sync.Map // key: method_endpoint, value: []duration
	mu               sync.RWMutex
}

// globalMetrics 全局指标收集器
var globalMetrics = &RequestMetrics{}

// MetricsMiddleware 指标收集中间件
// 收集HTTP请求的性能指标，用于监控和分析
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// 处理请求
		c.Next()
		
		// 计算处理时间
		duration := time.Since(start)
		
		// 生成指标键
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := strconv.Itoa(c.Writer.Status())
		
		// 记录请求总数
		totalKey := method + "_" + path + "_" + status
		currentCount, _ := globalMetrics.requestTotal.LoadOrStore(totalKey, int64(0))
		globalMetrics.requestTotal.Store(totalKey, currentCount.(int64)+1)
		
		// 记录响应时间
		durationKey := method + "_" + path
		durationList, _ := globalMetrics.requestDuration.LoadOrStore(durationKey, []time.Duration{})
		newDurationList := append(durationList.([]time.Duration), duration)
		
		// 限制存储的时间记录数量，避免内存泄漏
		if len(newDurationList) > 1000 {
			newDurationList = newDurationList[len(newDurationList)-1000:]
		}
		globalMetrics.requestDuration.Store(durationKey, newDurationList)
	}
}

// RequestIDMiddleware 请求ID中间件
// 为每个请求生成唯一ID，用于日志追踪和调试
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否已有请求ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// 生成新的请求ID
			requestID = generateRequestID()
		}
		
		// 设置请求ID到上下文和响应头
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		
		c.Next()
	}
}

// generateRequestID 生成请求ID
// 基于时间戳和随机数生成唯一标识符
func generateRequestID() string {
	now := time.Now()
	timestamp := now.UnixNano()
	return "req_" + strconv.FormatInt(timestamp, 36)
}

// GetMetrics 获取指标数据（用于/metrics端点）
func GetMetrics() map[string]any {
	result := make(map[string]any)
	
	// 请求总数指标
	totalRequests := make(map[string]int64)
	globalMetrics.requestTotal.Range(func(key, value any) bool {
		totalRequests[key.(string)] = value.(int64)
		return true
	})
	
	// 响应时间指标
	avgDurations := make(map[string]float64)
	globalMetrics.requestDuration.Range(func(key, value any) bool {
		durations := value.([]time.Duration)
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[key.(string)] = float64(total.Nanoseconds()) / float64(len(durations)) / 1000000 // 转换为毫秒
		}
		return true
	})
	
	result["http_requests_total"] = totalRequests
	result["http_request_duration_avg_ms"] = avgDurations
	result["timestamp"] = time.Now().Format(time.RFC3339)
	
	return result
}

// 辅助函数

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// joinStrings 连接字符串切片
func joinStrings(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += sep + slice[i]
	}
	return result
}