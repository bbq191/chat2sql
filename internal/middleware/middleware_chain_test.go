package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"chat2sql-go/internal/auth"
)

// ==============================================
// Middleware链路安全测试套件
// ==============================================

// MiddlewareChainTestSuite Middleware链路测试套件
type MiddlewareChainTestSuite struct {
	suite.Suite
	router         *gin.Engine
	logger         *zap.Logger
	jwtService     *auth.JWTService
	authMiddleware *AuthMiddleware
}

func (suite *MiddlewareChainTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.logger = zap.NewNop()

	config := &auth.JWTConfig{
		AutoGenerateKeys: true,
		Issuer:           "test-issuer",
		Audience:         "test-audience",
		AccessTokenTTL:   time.Hour,
		RefreshTokenTTL:  24 * time.Hour,
	}

	var err error
	suite.jwtService, err = auth.NewJWTService(config, suite.logger, nil)
	suite.Require().NoError(err)

	suite.authMiddleware = NewAuthMiddleware(suite.jwtService, suite.logger)
}

func (suite *MiddlewareChainTestSuite) SetupTest() {
	suite.router = gin.New()
	suite.router.Use(gin.Recovery())
}

// TestCORSSecurityHeaders 测试CORS安全头部设置
func (suite *MiddlewareChainTestSuite) TestCORSSecurityHeaders() {
	// 设置CORS中间件
	suite.router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		
		// 限制允许的域名
		allowedOrigins := []string{
			"https://chat2sql.com",
			"https://app.chat2sql.com", 
			"http://localhost:3000", // 开发环境
		}
		
		isAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				isAllowed = true
				break
			}
		}
		
		if isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		// 安全头部设置
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "X-Token-Expiring")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24小时
		
		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	})
	
	suite.router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	testCases := []struct {
		name           string
		origin         string
		method         string
		expectedStatus int
		expectOrigin   bool
	}{
		{"允许的域名", "https://chat2sql.com", "GET", http.StatusOK, true},
		{"开发环境域名", "http://localhost:3000", "GET", http.StatusOK, true},
		{"恶意域名", "https://malicious.com", "GET", http.StatusOK, false},
		{"预检请求-允许", "https://chat2sql.com", "OPTIONS", http.StatusNoContent, true},
		{"预检请求-拒绝", "https://malicious.com", "OPTIONS", http.StatusNoContent, false},
	}

	for _, tc := range testCases {
		req, _ := http.NewRequest(tc.method, "/api/test", nil)
		req.Header.Set("Origin", tc.origin)
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		assert.Equal(suite.T(), tc.expectedStatus, w.Code, "Test case: %s", tc.name)
		
		if tc.expectOrigin {
			assert.Equal(suite.T(), tc.origin, w.Header().Get("Access-Control-Allow-Origin"), 
				"Test case: %s", tc.name)
		} else {
			assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"), 
				"Test case: %s", tc.name)
		}
		
		// 验证其他安全头部
		assert.NotEmpty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	}
}

// TestSecurityHeadersMiddleware 测试安全头部中间件
func (suite *MiddlewareChainTestSuite) TestSecurityHeadersMiddleware() {
	// 安全头部中间件
	suite.router.Use(func(c *gin.Context) {
		// 防止点击劫持
		c.Header("X-Frame-Options", "DENY")
		// 防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")
		// XSS保护
		c.Header("X-XSS-Protection", "1; mode=block")
		// 强制HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// 内容安全策略
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		// 推荐策略
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// 权限策略
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		c.Next()
	})
	
	suite.router.GET("/secure", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "secure"})
	})

	req, _ := http.NewRequest("GET", "/secure", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	
	// 验证所有安全头部都被正确设置
	securityHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff", 
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	
	for header, expected := range securityHeaders {
		assert.Equal(suite.T(), expected, w.Header().Get(header), 
			"Security header %s should be set", header)
	}
	
	// 验证CSP头部存在
	assert.Contains(suite.T(), w.Header().Get("Content-Security-Policy"), "default-src 'self'")
	assert.Contains(suite.T(), w.Header().Get("Permissions-Policy"), "geolocation=")
}

// TestRateLimitingMiddleware 测试请求限流中间件
func (suite *MiddlewareChainTestSuite) TestRateLimitingMiddleware() {
	// 简单的内存限流器
	rateLimiter := make(map[string][]time.Time)
	var rateLimiterMutex sync.RWMutex
	
	// 限流中间件 - 每IP每分钟最多5个请求
	const maxRequests = 5
	const timeWindow = time.Minute
	
	suite.router.Use(func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		rateLimiterMutex.Lock()
		defer rateLimiterMutex.Unlock()
		
		// 获取客户端请求历史
		requests, exists := rateLimiter[clientIP]
		if !exists {
			requests = make([]time.Time, 0)
		}
		
		// 清理过期请求
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if now.Sub(reqTime) < timeWindow {
				validRequests = append(validRequests, reqTime)
			}
		}
		
		// 检查是否超过限制
		if len(validRequests) >= maxRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": int(timeWindow.Seconds()),
			})
			c.Abort()
			return
		}
		
		// 添加当前请求
		validRequests = append(validRequests, now)
		rateLimiter[clientIP] = validRequests
		
		c.Next()
	})
	
	suite.router.GET("/limited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 测试正常请求
	for i := 0; i < maxRequests; i++ {
		req, _ := http.NewRequest("GET", "/limited", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		assert.Equal(suite.T(), http.StatusOK, w.Code, 
			"Request %d should succeed", i+1)
	}
	
	// 测试超出限制的请求
	req, _ := http.NewRequest("GET", "/limited", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusTooManyRequests, w.Code)
	
	// 验证响应内容
	assert.Contains(suite.T(), w.Body.String(), "Rate limit exceeded")
}

// TestMiddlewareChainOrder 测试中间件执行顺序
func (suite *MiddlewareChainTestSuite) TestMiddlewareChainOrder() {
	var executionOrder []string
	var orderMutex sync.Mutex
	
	// 第一个中间件
	suite.router.Use(func(c *gin.Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, "middleware1_before")
		orderMutex.Unlock()
		
		c.Next()
		
		orderMutex.Lock()
		executionOrder = append(executionOrder, "middleware1_after")
		orderMutex.Unlock()
	})
	
	// 第二个中间件
	suite.router.Use(func(c *gin.Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, "middleware2_before")
		orderMutex.Unlock()
		
		c.Next()
		
		orderMutex.Lock()
		executionOrder = append(executionOrder, "middleware2_after")
		orderMutex.Unlock()
	})
	
	suite.router.GET("/order", func(c *gin.Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, "handler")
		orderMutex.Unlock()
		
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/order", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	
	// 验证中间件执行顺序
	expectedOrder := []string{
		"middleware1_before",
		"middleware2_before", 
		"handler",
		"middleware2_after",
		"middleware1_after",
	}
	
	orderMutex.Lock()
	assert.Equal(suite.T(), expectedOrder, executionOrder)
	orderMutex.Unlock()
}

// TestMiddlewareErrorHandling 测试中间件错误处理
func (suite *MiddlewareChainTestSuite) TestMiddlewareErrorHandling() {
	// 错误处理中间件
	suite.router.Use(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				suite.logger.Error("Middleware panic recovered", zap.Any("error", err))
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		
		c.Next()
	})
	
	// 会发生panic的中间件
	suite.router.Use(func(c *gin.Context) {
		if c.Query("panic") == "true" {
			panic("Test panic")
		}
		c.Next()
	})
	
	suite.router.GET("/error-test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 测试正常请求
	req, _ := http.NewRequest("GET", "/error-test", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	
	// 测试panic恢复
	req2, _ := http.NewRequest("GET", "/error-test?panic=true", nil)
	w2 := httptest.NewRecorder()
	suite.router.ServeHTTP(w2, req2)
	
	assert.Equal(suite.T(), http.StatusInternalServerError, w2.Code)
	assert.Contains(suite.T(), w2.Body.String(), "Internal server error")
}

// TestRequestValidationMiddleware 测试请求验证中间件
func (suite *MiddlewareChainTestSuite) TestRequestValidationMiddleware() {
	// 请求验证中间件
	suite.router.Use(func(c *gin.Context) {
		// 检查Content-Type
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") && 
			   !strings.Contains(contentType, "application/x-www-form-urlencoded") &&
			   !strings.Contains(contentType, "multipart/form-data") {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Unsupported content type",
				})
				c.Abort()
				return
			}
		}
		
		// 检查User-Agent
		userAgent := c.GetHeader("User-Agent")
		if userAgent == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "User-Agent header is required",
			})
			c.Abort()
			return
		}
		
		// 阻止已知的恶意User-Agent
		maliciousAgents := []string{
			"sqlmap", "nikto", "nmap", "masscan",
		}
		
		userAgentLower := strings.ToLower(userAgent)
		for _, malicious := range maliciousAgents {
			if strings.Contains(userAgentLower, malicious) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Blocked user agent",
				})
				c.Abort()
				return
			}
		}
		
		c.Next()
	})
	
	suite.router.POST("/validate", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "valid request"})
	})
	
	suite.router.GET("/validate", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "valid request"})
	})

	testCases := []struct {
		name           string
		method         string
		contentType    string
		userAgent      string
		expectedStatus int
	}{
		{"正常GET请求", "GET", "", "Mozilla/5.0", http.StatusOK},
		{"正常POST请求", "POST", "application/json", "Mozilla/5.0", http.StatusOK},
		{"错误Content-Type", "POST", "text/plain", "Mozilla/5.0", http.StatusUnsupportedMediaType},
		{"缺少User-Agent", "GET", "", "", http.StatusBadRequest},
		{"恶意User-Agent", "GET", "", "sqlmap/1.0", http.StatusForbidden},
	}

	for _, tc := range testCases {
		req, _ := http.NewRequest(tc.method, "/validate", nil)
		if tc.contentType != "" {
			req.Header.Set("Content-Type", tc.contentType)
		}
		if tc.userAgent != "" {
			req.Header.Set("User-Agent", tc.userAgent)
		}
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		assert.Equal(suite.T(), tc.expectedStatus, w.Code, "Test case: %s", tc.name)
	}
}

// TestAuthenticationAndAuthorizationChain 测试认证授权中间件链
func (suite *MiddlewareChainTestSuite) TestAuthenticationAndAuthorizationChain() {
	// 生成测试token
	userToken, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	suite.Require().NoError(err)
	
	adminToken, err := suite.jwtService.GenerateTokenPair(456, "admin", "admin")
	suite.Require().NoError(err)

	// 设置路由和中间件链
	api := suite.router.Group("/api")
	
	// 公开端点
	api.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})
	
	// 需要认证的端点
	authenticated := api.Group("/")
	authenticated.Use(suite.authMiddleware.JWTAuth())
	authenticated.GET("/user/profile", func(c *gin.Context) {
		userID, _ := GetUserIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})
	
	// 需要特定角色的端点
	adminOnly := authenticated.Group("/")
	adminOnly.Use(RequireRole("admin"))
	adminOnly.GET("/admin/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin data"})
	})

	testCases := []struct {
		name           string
		path           string
		token          string
		expectedStatus int
	}{
		{"公开端点-无token", "/api/public", "", http.StatusOK},
		{"用户端点-有效token", "/api/user/profile", userToken.AccessToken, http.StatusOK},
		{"用户端点-无token", "/api/user/profile", "", http.StatusUnauthorized},
		{"管理端点-用户token", "/api/admin/users", userToken.AccessToken, http.StatusForbidden},
		{"管理端点-管理员token", "/api/admin/users", adminToken.AccessToken, http.StatusOK},
		{"管理端点-无token", "/api/admin/users", "", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		req, _ := http.NewRequest("GET", tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		assert.Equal(suite.T(), tc.expectedStatus, w.Code, "Test case: %s", tc.name)
	}
}

// 运行Middleware链路测试套件
func TestMiddlewareChainTestSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareChainTestSuite))
}