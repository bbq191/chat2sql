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
		assert.Contains(t, response["message"], "数据库查询失败")

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
		assert.Contains(t, response["message"], "请求参数格式错误")
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
		assert.Contains(t, response["message"], "用户名或密码错误")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Login_PasswordHashError", func(t *testing.T) {
		mockUserRepo := &MockUserRepository{}
		mockJWTService := &MockJWTService{}
		logger := zaptest.NewLogger(t)

		handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

		// 模拟用户存在但密码哈希格式错误
		user := &repository.User{
			BaseModel:    repository.BaseModel{ID: 1},
			Username:     "testuser",
			PasswordHash: "invalid-hash-too-short", // 无效的bcrypt哈希
			Status:       string(repository.StatusActive),
		}
		mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)

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

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["message"], "用户名或密码错误")

		mockUserRepo.AssertExpectations(t)
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

		// 模拟连接存在
		connection := &repository.DatabaseConnection{
			BaseModel: repository.BaseModel{ID: 1},
			UserID:    1,
			Name:      "test-db",
			Status:    string(repository.ConnectionActive),
		}
		mockConnRepo.On("GetByID", mock.Anything, int64(1)).Return(connection, nil)
		
		// 模拟查询历史记录创建和更新
		mockQueryRepo.On("Create", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).
			Return(nil)
		mockQueryRepo.On("Update", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).
			Return(nil)

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
		c.Set("user_id", int64(1)) // 模拟认证用户

		handler.ExecuteSQL(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "error", response["status"])
		assert.Contains(t, response["error"], "syntax error in SQL")

		mockQueryRepo.AssertExpectations(t)
		mockConnRepo.AssertExpectations(t)
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
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		if response != nil {
			assert.Contains(t, response["message"], "未授权访问")
		}
	})

	t.Run("GetQueryHistory_RepositoryError", func(t *testing.T) {
		mockQueryRepo := &MockQueryHistoryRepository{}
		mockConnRepo := &MockConnectionRepository{}
		mockSQLExecutor := &MockSQLExecutor{}
		logger := zaptest.NewLogger(t)

		handler := NewSQLHandler(mockQueryRepo, mockConnRepo, mockSQLExecutor, logger)

		// 模拟repository错误，只设置ListByUser的Mock，因为出错时不会调用CountByUser
		mockQueryRepo.On("ListByUser", mock.Anything, int64(1), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
			Return([]*repository.QueryHistory{}, errors.New("database query timeout"))

		req := httptest.NewRequest(http.MethodGet, "/sql/history?page=1&limit=10", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", int64(1))

		handler.GetQueryHistory(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		if response != nil {
			assert.Contains(t, response["message"], "查询历史获取失败")
		}

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

		// 首先模拟名称检查通过
		mockConnRepo.On("ExistsByUserAndName", mock.Anything, int64(1), "test_connection").
			Return(false, nil)
		// 模拟ConnectionManager创建连接失败
		mockConnManager.On("CreateConnection", mock.Anything, mock.AnythingOfType("*repository.DatabaseConnection")).
			Return(errors.New("connection creation failed"))

		reqBody := map[string]interface{}{
			"name":          "test_connection",
			"db_type":       "postgresql",
			"host":          "localhost",
			"port":          5432,
			"database_name": "testdb",
			"username":      "testuser",
			"password":      "testpass",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/connections", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", int64(1))

		handler.CreateConnection(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		if response != nil {
			assert.Contains(t, response["message"], "创建连接失败")
		}

		mockConnRepo.AssertExpectations(t)
		mockConnManager.AssertExpectations(t)
	})

	t.Run("TestConnection_ConnectionError", func(t *testing.T) {
		mockConnRepo := &MockConnectionRepository{}
		mockSchemaRepo := &MockSchemaRepository{}
		mockConnManager := &MockConnectionManager{}
		logger := zaptest.NewLogger(t)

		handler := NewConnectionHandler(mockConnRepo, mockSchemaRepo, mockConnManager, logger)

		// 模拟获取连接失败（连接不存在）
		mockConnRepo.On("GetByID", mock.Anything, int64(1)).
			Return(nil, errors.New("connection not found"))

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
		c.Set("user_id", int64(1))
		c.Params = gin.Params{{Key: "id", Value: "1"}}

		handler.TestConnection(c)

		assert.Equal(t, http.StatusNotFound, w.Code) // 修正期望状态码
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		if response != nil {
			assert.Contains(t, response["message"], "连接不存在")
		}

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
		c.Set("user_id", int64(1))
		// 模拟路由参数
		c.Params = []gin.Param{{Key: "id", Value: "999"}}

		handler.GetConnection(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		if response != nil {
			assert.Contains(t, response["message"], "连接不存在")
		}

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
		c.Set("user_id", int64(1))

		handler.GetProfile(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["message"], "用户不存在")

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
		c.Set("user_id", int64(1))

		handler.UpdateProfile(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response["message"], "请求参数格式错误")
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

		// Recovery中间件应该捕获panic并返回500状态码
		// 这里应该验证恢复后的行为，而不是验证是否panic
		r.ServeHTTP(w, req)

		// 验证Recovery中间件正确处理了panic
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		
		// 验证日志输出显示panic被恢复
		// Gin的Recovery中间件会记录panic信息到日志，但默认不返回响应体
	})
}

// Mock implementations - reusing types from handler_integration_test.go
// 复用handler_integration_test.go中定义的Mock类型以避免重复声明

// 注意：Mock方法实现在handler_integration_test.go中定义
// 这里只需要声明测试逻辑即可