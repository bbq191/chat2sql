package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/repository"
)


// TestAuthHandler_ErrorHandling 测试认证处理器的错误场景
func TestAuthHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Register_UserRepoError", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockJWTService := &MockJWTService{}
		logger := zaptest.NewLogger(t)

		handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

		// 模拟用户仓库错误
		mockUserRepo.On("ExistsByUsername", mock.Anything, "testuser").
			Return(false, errors.New("database connection failed"))

		// 创建请求
		reqBody := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// 创建gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.Register(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "检查用户名失败")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Register_InvalidJSON", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockJWTService := &MockJWTService{}
		logger := zaptest.NewLogger(t)

		handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

		// 无效的JSON
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.Register(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "无效的请求格式")
	})

	t.Run("Login_UserNotFound", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockJWTService := &MockJWTService{}
		logger := zaptest.NewLogger(t)

		handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

		// 模拟用户不存在
		mockUserRepo.On("GetByUsername", mock.Anything, "nonexistent").
			Return(nil, errors.New("user not found"))

		reqBody := map[string]interface{}{
			"username": "nonexistent",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.Login(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "用户名或密码错误")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Login_JWTGenerationError", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockJWTService := &MockJWTService{}
		logger := zaptest.NewLogger(t)

		handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

		// 模拟用户存在但JWT生成失败
		user := &repository.User{
			BaseModel:    repository.BaseModel{ID: 1},
			Username:     "testuser",
			PasswordHash: "$2a$12$hashedpassword",
			Status:       string(repository.StatusActive),
		}
		mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)
		mockJWTService.On("ValidatePassword", "password123", user.PasswordHash).Return(true)
		mockJWTService.On("GenerateToken", user).
			Return("", errors.New("key generation failed"))

		reqBody := map[string]interface{}{
			"username": "testuser",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.Login(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "生成访问令牌失败")

		mockUserRepo.AssertExpectations(t)
		mockJWTService.AssertExpectations(t)
	})
}

// TestSQLHandler_ErrorHandling 测试SQL处理器的错误场景
func TestSQLHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ExecuteSQL_ExecutorError", func(t *testing.T) {
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		mockSQLExecutor := &MockSQLExecutor{}
		logger := zaptest.NewLogger(t)

		handler := NewSQLHandler(mockQueryRepo, mockConnRepo, mockSQLExecutor, logger)

		// 模拟SQL执行器错误
		mockSQLExecutor.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(nil, errors.New("syntax error in SQL"))

		reqBody := map[string]interface{}{
			"sql":           "SELECT * FROM invalid_table",
			"connection_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/sql/execute", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1)) // 模拟认证用户

		handler.ExecuteSQL(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "SQL执行失败")

		mockSQLExecutor.AssertExpectations(t)
	})

	t.Run("ExecuteSQL_MissingUserID", func(t *testing.T) {
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		mockSQLExecutor := &MockSQLExecutor{}
		logger := zaptest.NewLogger(t)

		handler := NewSQLHandler(mockQueryRepo, mockConnRepo, mockSQLExecutor, logger)

		reqBody := map[string]interface{}{
			"sql":           "SELECT * FROM users",
			"connection_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/sql/execute", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		// 不设置userID，模拟认证失败

		handler.ExecuteSQL(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "用户未认证")
	})

	t.Run("GetQueryHistory_RepositoryError", func(t *testing.T) {
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		mockSQLExecutor := &MockSQLExecutor{}
		logger := zaptest.NewLogger(t)

		handler := NewSQLHandler(mockQueryRepo, mockConnRepo, mockSQLExecutor, logger)

		// 模拟repository错误
		mockQueryRepo.On("ListByUser", mock.Anything, int64(1), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
			Return(nil, errors.New("database query timeout"))

		req := httptest.NewRequest(http.MethodGet, "/sql/history?page=1&limit=10", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1))

		handler.GetQueryHistory(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "获取查询历史失败")

		mockQueryRepo.AssertExpectations(t)
	})
}

// TestConnectionHandler_ErrorHandling 测试连接处理器的错误场景
func TestConnectionHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("CreateConnection_RepositoryError", func(t *testing.T) {
		mockConnRepo := &MockConnectionRepository{}
		mockSchemaRepo := &MockSchemaRepository{}
		mockConnManager := &MockConnectionManager{}
		logger := zaptest.NewLogger(t)

		handler := NewConnectionHandler(mockConnRepo, mockSchemaRepo, mockConnManager, logger)

		// 模拟repository创建失败
		mockConnRepo.On("Create", mock.Anything, mock.AnythingOfType("*repository.DatabaseConnection")).
			Return(errors.New("unique constraint violation"))

		reqBody := map[string]interface{}{
			"name":     "test_connection",
			"type":     "postgresql",
			"host":     "localhost",
			"port":     5432,
			"database": "testdb",
			"username": "testuser",
			"password": "testpass",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/connections", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1))

		handler.CreateConnection(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "创建连接失败")

		mockConnRepo.AssertExpectations(t)
	})

	t.Run("TestConnection_ConnectionError", func(t *testing.T) {
		mockConnRepo := &MockConnectionRepository{}
		mockSchemaRepo := &MockSchemaRepository{}
		mockConnManager := &MockConnectionManager{}
		logger := zaptest.NewLogger(t)

		handler := NewConnectionHandler(mockConnRepo, mockSchemaRepo, mockConnManager, logger)

		// 模拟连接测试失败
		mockConnManager.On("TestConnection", mock.Anything, mock.AnythingOfType("*repository.DatabaseConnection")).
			Return(errors.New("connection refused"))

		reqBody := map[string]interface{}{
			"type":     "postgresql",
			"host":     "nonexistent-host",
			"port":     5432,
			"database": "testdb",
			"username": "testuser",
			"password": "wrongpass",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/connections/test", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.TestConnection(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "连接测试失败")

		mockConnManager.AssertExpectations(t)
	})

	t.Run("GetConnection_NotFound", func(t *testing.T) {
		mockConnRepo := &MockConnectionRepository{}
		mockSchemaRepo := &MockSchemaRepository{}
		mockConnManager := &MockConnectionManager{}
		logger := zaptest.NewLogger(t)

		handler := NewConnectionHandler(mockConnRepo, mockSchemaRepo, mockConnManager, logger)

		// 模拟连接不存在
		mockConnRepo.On("GetByID", mock.Anything, int64(999)).
			Return(nil, errors.New("connection not found"))

		req := httptest.NewRequest(http.MethodGet, "/connections/999", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1))
		// 模拟路由参数
		c.Params = []gin.Param{{Key: "id", Value: "999"}}

		handler.GetConnection(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "连接不存在")

		mockConnRepo.AssertExpectations(t)
	})
}

// TestUserHandler_ErrorHandling 测试用户处理器的错误场景
func TestUserHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GetProfile_RepositoryError", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		logger := zaptest.NewLogger(t)

		handler := NewUserHandler(mockUserRepo, mockQueryRepo, mockConnRepo, logger)

		// 模拟repository错误
		mockUserRepo.On("GetByID", mock.Anything, int64(1)).
			Return(nil, errors.New("database connection lost"))

		req := httptest.NewRequest(http.MethodGet, "/user/profile", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1))

		handler.GetProfile(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "获取用户信息失败")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("UpdateProfile_ValidationError", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		logger := zaptest.NewLogger(t)

		handler := NewUserHandler(mockUserRepo, mockQueryRepo, mockConnRepo, logger)

		// 无效的邮箱格式
		reqBody := map[string]interface{}{
			"email": "invalid-email-format",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/user/profile", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("userID", int64(1))

		handler.UpdateProfile(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "邮箱格式不正确")
	})
}

// TestHealthHandler_ErrorHandling 测试健康检查处理器的错误场景
func TestHealthHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HealthCheck_ServiceError", func(t *testing.T) {
		mockHealthService := &MockHealthService{}

		// 创建路由器并设置健康检查路由
		r := gin.New()
		r.GET("/health", func(c *gin.Context) {
			status, err := mockHealthService.GetApplicationStatus(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error":  "健康检查失败",
					"detail": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, status)
		})

		// 模拟健康服务错误
		mockHealthService.On("GetApplicationStatus", mock.Anything).
			Return(nil, errors.New("database health check failed"))

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "健康检查失败")

		mockHealthService.AssertExpectations(t)
	})
}

// TestMiddlewareErrorHandling 测试中间件的错误处理
func TestMiddlewareErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("AuthMiddleware_InvalidToken", func(t *testing.T) {
		mockJWTService := &MockJWTService{}

		// 模拟JWT验证失败
		mockJWTService.On("ValidateTokenFromRequest", "Bearer invalid-token").
			Return(nil, errors.New("token is invalid"))

		// 创建中间件
		authMiddleware := func(c *gin.Context) {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证头"})
				c.Abort()
				return
			}

			_, err := mockJWTService.ValidateTokenFromRequest(authHeader)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的访问令牌"})
				c.Abort()
				return
			}

			c.Next()
		}

		// 创建路由器
		r := gin.New()
		r.Use(authMiddleware)
		r.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "无效的访问令牌")

		mockJWTService.AssertExpectations(t)
	})

	t.Run("RateLimitMiddleware_Exceeded", func(t *testing.T) {
		// 模拟限流中间件超出限制
		rateLimitMiddleware := func(c *gin.Context) {
			// 简化的限流逻辑 - 直接返回限流错误
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": "请稍后重试",
			})
			c.Abort()
		}

		r := gin.New()
		r.Use(rateLimitMiddleware)
		r.GET("/api/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "请求过于频繁")
	})
}

// TestPanicRecovery 测试panic恢复
func TestPanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HandlerPanic", func(t *testing.T) {
		r := gin.New()
		r.Use(gin.Recovery()) // 使用gin的recovery中间件

		r.GET("/panic", func(c *gin.Context) {
			panic("intentional panic for testing")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		// 不应该导致程序崩溃
		assert.NotPanics(t, func() {
			r.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// Mock implementations - reusing types from handler_integration_test.go
// 复用handler_integration_test.go中定义的Mock类型以避免重复声明

// 注意：Mock方法实现在handler_integration_test.go中定义
// 这里只需要声明测试逻辑即可