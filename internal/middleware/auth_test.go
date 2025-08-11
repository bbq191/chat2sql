package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"chat2sql-go/internal/auth"
)

// AuthMiddlewareTestSuite 认证中间件测试套件
type AuthMiddlewareTestSuite struct {
	suite.Suite
	authMiddleware *AuthMiddleware
	jwtService     *auth.JWTService
	logger         *zap.Logger
	router         *gin.Engine
}

func (suite *AuthMiddlewareTestSuite) SetupSuite() {
	// 设置gin为测试模式
	gin.SetMode(gin.TestMode)
	
	// 创建测试日志器
	suite.logger = zap.NewNop()
	
	// 创建JWT服务用于测试
	config := &auth.JWTConfig{
		AutoGenerateKeys: true,
		Issuer:           "test-issuer",
		Audience:         "test-audience",
		AccessTokenTTL:   time.Hour,
		RefreshTokenTTL:  24 * time.Hour,
	}
	
	// 创建一个简单的Redis客户端模拟
	// 在实际测试中应该使用mock
	var err error
	suite.jwtService, err = auth.NewJWTService(config, suite.logger, nil)
	require.NoError(suite.T(), err)
}

func (suite *AuthMiddlewareTestSuite) SetupTest() {
	// 创建认证中间件
	suite.authMiddleware = NewAuthMiddleware(suite.jwtService, suite.logger)
	
	// 设置路由
	suite.router = gin.New()
	suite.router.Use(gin.Recovery())
}

func (suite *AuthMiddlewareTestSuite) TestNewAuthMiddleware() {
	middleware := NewAuthMiddleware(suite.jwtService, suite.logger)
	assert.NotNil(suite.T(), middleware)
	assert.Equal(suite.T(), suite.jwtService, middleware.jwtService)
	assert.Equal(suite.T(), suite.logger, middleware.logger)
}

func (suite *AuthMiddlewareTestSuite) TestJWTAuth_Success() {
	// 生成有效token
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	// 设置受保护的路由
	suite.router.GET("/protected", suite.authMiddleware.JWTAuth(), func(c *gin.Context) {
		// 从上下文中获取用户信息
		userIDValue, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found"})
			return
		}
		
		usernameValue, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "username not found"})
			return
		}
		
		roleValue, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "role not found"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userIDValue,
			"username": usernameValue,
			"role":     roleValue,
			"message":  "success",
		})
	})
	
	// 创建带有有效token的请求
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "success", response["message"])
	assert.Equal(suite.T(), float64(123), response["user_id"]) // JSON中数字是float64
	assert.Equal(suite.T(), "testuser", response["username"])
	assert.Equal(suite.T(), "user", response["role"])
}

func (suite *AuthMiddlewareTestSuite) TestJWTAuth_MissingHeader() {
	// 设置受保护的路由
	suite.router.GET("/protected", suite.authMiddleware.JWTAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// 创建不带Authorization头的请求
	req, _ := http.NewRequest("GET", "/protected", nil)
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "MISSING_AUTH_HEADER", response["code"])
	assert.Contains(suite.T(), response["message"], "缺少授权头")
}

func (suite *AuthMiddlewareTestSuite) TestJWTAuth_InvalidToken() {
	// 设置受保护的路由
	suite.router.GET("/protected", suite.authMiddleware.JWTAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// 创建带有无效token的请求
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_TOKEN", response["code"])
}

func (suite *AuthMiddlewareTestSuite) TestJWTAuth_InvalidHeaderFormat() {
	// 设置受保护的路由
	suite.router.GET("/protected", suite.authMiddleware.JWTAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// 创建带有格式错误Authorization头的请求
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token")
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_TOKEN", response["code"])
}

// 测试中间件辅助函数
func TestGetUserIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	
	// 测试未设置的情况
	userID, exists := GetUserIDFromContext(c)
	assert.False(t, exists)
	assert.Equal(t, int64(0), userID)
	
	// 测试设置后的情况
	expectedUserID := int64(123)
	c.Set("user_id", expectedUserID)
	userID, exists = GetUserIDFromContext(c)
	assert.True(t, exists)
	assert.Equal(t, expectedUserID, userID)
}

func TestGetUsernameFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	
	// 测试未设置的情况
	username, exists := GetUsernameFromContext(c)
	assert.False(t, exists)
	assert.Equal(t, "", username)
	
	// 测试设置后的情况
	expectedUsername := "testuser"
	c.Set("username", expectedUsername)
	username, exists = GetUsernameFromContext(c)
	assert.True(t, exists)
	assert.Equal(t, expectedUsername, username)
}

func TestGetUserRoleFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	
	// 测试未设置的情况
	role, exists := GetUserRoleFromContext(c)
	assert.False(t, exists)
	assert.Equal(t, "", role)
	
	// 测试设置后的情况
	expectedRole := "admin"
	c.Set("user_role", expectedRole)
	role, exists = GetUserRoleFromContext(c)
	assert.True(t, exists)
	assert.Equal(t, expectedRole, role)
}

func TestUserIDFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	// 为避免nil指针错误，设置一个空的Request
	req, _ := http.NewRequest("GET", "/test", nil)
	c.Request = req
	
	// 测试未认证的情况
	userID := UserIDFromRequest(c)
	assert.Equal(t, int64(0), userID)
	
	// 测试已认证的情况（从context）
	c.Set("user_id", int64(123))
	userID = UserIDFromRequest(c)
	assert.Equal(t, int64(123), userID)
	
	// 测试从Header获取
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-User-ID", "456")
	c2.Request = req2
	
	userID = UserIDFromRequest(c2)
	assert.Equal(t, int64(456), userID)
}

// 运行测试套件
func TestAuthMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AuthMiddlewareTestSuite))
}

// ==============================================
// 权限越权安全测试套件
// ==============================================

// AuthorizationSecurityTestSuite 权限安全测试套件
type AuthorizationSecurityTestSuite struct {
	suite.Suite
	authMiddleware *AuthMiddleware
	jwtService     *auth.JWTService
	logger         *zap.Logger
	router         *gin.Engine
}

func (suite *AuthorizationSecurityTestSuite) SetupSuite() {
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
	require.NoError(suite.T(), err)
	
	suite.authMiddleware = NewAuthMiddleware(suite.jwtService, suite.logger)
}

func (suite *AuthorizationSecurityTestSuite) SetupTest() {
	suite.router = gin.New()
	suite.router.Use(gin.Recovery())
}

// TestRoleEscalationAttack 测试角色提升攻击防护
func (suite *AuthorizationSecurityTestSuite) TestRoleEscalationAttack() {
	// 生成普通user角色的token
	userTokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	// 设置需要admin权限的接口
	suite.router.GET("/admin/users",
		suite.authMiddleware.JWTAuth(),
		RequireRole("admin"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "admin data"})
		},
	)
	
	// 使用user token尝试访问admin接口
	req, _ := http.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+userTokenPair.AccessToken)
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	// 应该被拒绝访问
	assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INSUFFICIENT_PERMISSIONS", response["code"])
	assert.Equal(suite.T(), "user", response["user_role"])
}

// TestCrossUserDataAccess 测试跨用户数据访问防护
func (suite *AuthorizationSecurityTestSuite) TestCrossUserDataAccess() {
	// 生成两个不同用户的token
	user1Token, err := suite.jwtService.GenerateTokenPair(123, "user1", "user")
	require.NoError(suite.T(), err)
	
	user2Token, err := suite.jwtService.GenerateTokenPair(456, "user2", "user")
	require.NoError(suite.T(), err)
	
	// 设置用户专属数据接口
	suite.router.GET("/user/:id/profile",
		suite.authMiddleware.JWTAuth(),
		func(c *gin.Context) {
			requestUserID := c.Param("id")
			currentUserID, exists := GetUserIDFromContext(c)
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
				return
			}
			
			// 检查用户是否尝试访问他人的数据
			if requestUserID != strconv.FormatInt(currentUserID, 10) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "cannot access other user's data",
					"requested_user": requestUserID,
					"current_user": currentUserID,
				})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"user_id": currentUserID,
				"profile": "sensitive data",
			})
		},
	)
	
	// user1尝试访问user2的数据
	req, _ := http.NewRequest("GET", "/user/456/profile", nil)
	req.Header.Set("Authorization", "Bearer "+user1Token.AccessToken)
	
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	
	// 应该被拒绝
	assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	
	// user1访问自己的数据应该成功
	req2, _ := http.NewRequest("GET", "/user/123/profile", nil)
	req2.Header.Set("Authorization", "Bearer "+user1Token.AccessToken)
	
	w2 := httptest.NewRecorder()
	suite.router.ServeHTTP(w2, req2)
	
	assert.Equal(suite.T(), http.StatusOK, w2.Code)

	// user2访问自己的数据也应该成功
	req3, _ := http.NewRequest("GET", "/user/456/profile", nil)
	req3.Header.Set("Authorization", "Bearer "+user2Token.AccessToken)

	w3 := httptest.NewRecorder()
	suite.router.ServeHTTP(w3, req3)

	assert.Equal(suite.T(), http.StatusOK, w3.Code)
}

// TestPermissionEscalation 测试权限提升攻击
func (suite *AuthorizationSecurityTestSuite) TestPermissionEscalation() {
	// 生成不同角色的token
	userToken, err := suite.jwtService.GenerateTokenPair(123, "user", "user")
	require.NoError(suite.T(), err)
	
	managerToken, err := suite.jwtService.GenerateTokenPair(456, "manager", "manager")
	require.NoError(suite.T(), err)
	
	adminToken, err := suite.jwtService.GenerateTokenPair(789, "admin", "admin")
	require.NoError(suite.T(), err)
	
	// 设置不同权限级别的接口
	suite.router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "public data"})
	})
	
	suite.router.GET("/user/data",
		suite.authMiddleware.JWTAuth(),
		RequirePermission("query:execute"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"data": "user data"})
		},
	)
	
	suite.router.GET("/manager/team",
		suite.authMiddleware.JWTAuth(),
		RequirePermission("history:view_team"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"data": "team data"})
		},
	)
	
	suite.router.GET("/admin/system",
		suite.authMiddleware.JWTAuth(),
		RequireRole("admin"),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"data": "system data"})
		},
	)
	
	// 测试各角色的权限边界
	testCases := []struct {
		name       string
		path       string
		token      string
		expectedStatus int
	}{
		{"user访问public", "/public", "", http.StatusOK},
		{"user访问user/data", "/user/data", userToken.AccessToken, http.StatusOK},
		{"user访问manager/team", "/manager/team", userToken.AccessToken, http.StatusForbidden},
		{"user访问admin/system", "/admin/system", userToken.AccessToken, http.StatusForbidden},
		
		{"manager访问user/data", "/user/data", managerToken.AccessToken, http.StatusOK},
		{"manager访问manager/team", "/manager/team", managerToken.AccessToken, http.StatusOK},
		{"manager访问admin/system", "/admin/system", managerToken.AccessToken, http.StatusForbidden},
		
		{"admin访问user/data", "/user/data", adminToken.AccessToken, http.StatusOK},
		{"admin访问manager/team", "/manager/team", adminToken.AccessToken, http.StatusOK},
		{"admin访问admin/system", "/admin/system", adminToken.AccessToken, http.StatusOK},
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

// TestTokenForgedClaims 测试伪造Claims防护
func (suite *AuthorizationSecurityTestSuite) TestTokenForgedClaims() {
	// 尝试手动构造假的JWT token
	forgedTokens := []string{
		// 无效的base64编码
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.FORGED_PAYLOAD.FORGED_SIGNATURE",
		// 空的token部分
		"...",
		// 只有header
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9",
		// 无效的JSON格式
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.aW52YWxpZF9qc29u.invalid_signature",
	}
	
	suite.router.GET("/protected",
		suite.authMiddleware.JWTAuth(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		},
	)
	
	for _, forgedToken := range forgedTokens {
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+forgedToken)
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		// 所有伪造token都应该被拒绝
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code,
			"伪造token应该被拒绝: %s", forgedToken)
	}
}

// TestConcurrentAuthRequests 测试并发认证请求
func (suite *AuthorizationSecurityTestSuite) TestConcurrentAuthRequests() {
	// 生成有效token
	validToken, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	suite.router.GET("/concurrent",
		suite.authMiddleware.JWTAuth(),
		func(c *gin.Context) {
			// 模拟一些处理时间
			time.Sleep(10 * time.Millisecond)
			c.JSON(http.StatusOK, gin.H{
				"user_id": c.GetInt64("user_id"),
				"timestamp": time.Now().Unix(),
			})
		},
	)
	
	// 并发发送多个请求
	const numRequests = 10
	results := make(chan int, numRequests)
	
	for i := 0; i < numRequests; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/concurrent", nil)
			req.Header.Set("Authorization", "Bearer "+validToken.AccessToken)
			
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)
			
			results <- w.Code
		}()
	}
	
	// 收集结果
	for i := 0; i < numRequests; i++ {
		status := <-results
		assert.Equal(suite.T(), http.StatusOK, status, "并发请求%d应该成功", i+1)
	}
}

// TestAuthorizationHeaderVariations 测试各种授权头变形
func (suite *AuthorizationSecurityTestSuite) TestAuthorizationHeaderVariations() {
	validToken, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	suite.router.GET("/auth-test",
		suite.authMiddleware.JWTAuth(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		},
	)
	
	// 测试不同的授权头格式
	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{"正常Bearer", "Bearer " + validToken.AccessToken, http.StatusOK},
		{"小bearer", "bearer " + validToken.AccessToken, http.StatusUnauthorized},
		{"多余空格", "Bearer  " + validToken.AccessToken, http.StatusUnauthorized},
		{"缺少空格", "Bearer" + validToken.AccessToken, http.StatusUnauthorized},
		{"错误前缀", "Token " + validToken.AccessToken, http.StatusUnauthorized},
		{"空字符串", "", http.StatusUnauthorized},
		{"只有Bearer", "Bearer", http.StatusUnauthorized},
		{"只有Bearer ", "Bearer ", http.StatusUnauthorized},
	}
	
	for _, tc := range testCases {
		req, _ := http.NewRequest("GET", "/auth-test", nil)
		if tc.authHeader != "" {
			req.Header.Set("Authorization", tc.authHeader)
		}
		
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		
		assert.Equal(suite.T(), tc.expectedStatus, w.Code,
			"Test case: %s, header: %s", tc.name, tc.authHeader)
	}
}

// 运行权限安全测试套件
func TestAuthorizationSecurityTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationSecurityTestSuite))
}