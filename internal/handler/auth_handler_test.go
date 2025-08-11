package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/auth"
	"chat2sql-go/internal/repository"
)

// MockJWTServiceSimple Mock JWT服务（简化版，避免重复）
type MockJWTServiceSimple struct {
	mock.Mock
}

func (m *MockJWTServiceSimple) GenerateTokenPair(userID int64, username, role string) (*auth.TokenPair, error) {
	args := m.Called(userID, username, role)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.TokenPair), args.Error(1)
}

func (m *MockJWTServiceSimple) ValidateRefreshToken(tokenString string) (*auth.CustomClaims, error) {
	args := m.Called(tokenString)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.CustomClaims), args.Error(1)
}

func (m *MockJWTServiceSimple) ValidateTokenFromRequest(authHeader string) (*auth.CustomClaims, error) {
	args := m.Called(authHeader)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.CustomClaims), args.Error(1)
}

func (m *MockJWTServiceSimple) RefreshTokenPair(refreshTokenString string) (*auth.TokenPair, error) {
	args := m.Called(refreshTokenString)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.TokenPair), args.Error(1)
}

// TestNewAuthHandler 测试创建认证处理器
func TestNewAuthHandler(t *testing.T) {
	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)

	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, mockUserRepo, handler.userRepo)
	assert.Equal(t, mockJWTService, handler.jwtService)
	assert.Equal(t, logger, handler.logger)
}

// TestAuthHandler_Register_Success 测试用户注册成功
func TestAuthHandler_Register_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	// 设置mock期望
	mockUserRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, nil)
	mockUserRepo.On("ExistsByEmail", mock.Anything, "test@example.com").Return(false, nil)
	mockUserRepo.On("Create", mock.Anything, mock.MatchedBy(func(user *repository.User) bool {
		return user.Username == "testuser" && 
			   user.Email == "test@example.com" &&
			   user.Role == string(repository.RoleUser) &&
			   user.Status == string(repository.StatusActive) &&
			   user.PasswordHash != ""
	})).Return(nil).Run(func(args mock.Arguments) {
		user := args.Get(1).(*repository.User)
		user.BaseModel.ID = 1 // 模拟数据库生成的ID
	})

	expectedTokenPair := &auth.TokenPair{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	mockJWTService.On("GenerateTokenPair", int64(1), "testuser", string(repository.RoleUser)).
		Return(expectedTokenPair, nil)

	// 准备请求
	reqBody := RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// 创建Gin上下文
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// 执行测试
	handler.Register(c)

	// 验证结果
	assert.Equal(t, http.StatusCreated, w.Code)
	
	var response AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, expectedTokenPair.AccessToken, response.AccessToken)
	assert.Equal(t, expectedTokenPair.RefreshToken, response.RefreshToken)
	assert.Equal(t, "testuser", response.User.Username)
	assert.Equal(t, "test@example.com", response.User.Email)

	mockUserRepo.AssertExpectations(t)
	mockJWTService.AssertExpectations(t)
}

// TestAuthHandler_Register_InvalidRequest 测试注册请求参数错误
func TestAuthHandler_Register_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	// 准备无效请求（缺少必需字段）
	reqBody := map[string]interface{}{
		"username": "testuser",
		// 缺少email和password
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", response.Code)
	assert.Equal(t, "请求参数格式错误", response.Message)
}

// TestAuthHandler_Register_UsernameExists 测试用户名已存在
func TestAuthHandler_Register_UsernameExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	mockUserRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(true, nil)

	reqBody := RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Register(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "USERNAME_EXISTS", response.Code)
	assert.Equal(t, "用户名已存在", response.Message)

	mockUserRepo.AssertExpectations(t)
}

// TestAuthHandler_Login_Success 测试用户登录成功
func TestAuthHandler_Login_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	// 创建测试用户
	hashedPassword, _ := hashPassword("password123")
	testUser := &repository.User{
		BaseModel: repository.BaseModel{
			ID:         1,
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		},
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}

	mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(testUser, nil)
	mockUserRepo.On("UpdateLastLogin", mock.Anything, int64(1), mock.AnythingOfType("time.Time")).Return(nil)

	expectedTokenPair := &auth.TokenPair{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	mockJWTService.On("GenerateTokenPair", int64(1), "testuser", string(repository.RoleUser)).
		Return(expectedTokenPair, nil)

	reqBody := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Login(c)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, expectedTokenPair.AccessToken, response.AccessToken)
	assert.Equal(t, "testuser", response.User.Username)

	mockUserRepo.AssertExpectations(t)
	mockJWTService.AssertExpectations(t)
}

// TestAuthHandler_Login_InvalidCredentials 测试登录凭据错误
func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(nil, errors.New("user not found"))

	reqBody := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Login(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_CREDENTIALS", response.Code)
	assert.Equal(t, "用户名或密码错误", response.Message)

	mockUserRepo.AssertExpectations(t)
}

// TestAuthHandler_RefreshToken_Success 测试刷新Token成功
func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	// 模拟有效的refresh token claims
	claims := &auth.CustomClaims{
		UserID:   1,
		Username: "testuser",
		Role:     string(repository.RoleUser),
	}

	testUser := &repository.User{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		Username: "testuser",
		Email:    "test@example.com",
		Role:     string(repository.RoleUser),
		Status:   string(repository.StatusActive),
	}

	mockJWTService.On("ValidateRefreshToken", "valid_refresh_token").Return(claims, nil)
	mockUserRepo.On("GetByID", mock.Anything, int64(1)).Return(testUser, nil)

	expectedTokenPair := &auth.TokenPair{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	mockJWTService.On("GenerateTokenPair", int64(1), "testuser", string(repository.RoleUser)).
		Return(expectedTokenPair, nil)

	reqBody := RefreshTokenRequest{
		RefreshToken: "valid_refresh_token",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.RefreshToken(c)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, expectedTokenPair.AccessToken, response.AccessToken)
	assert.Equal(t, expectedTokenPair.RefreshToken, response.RefreshToken)

	mockUserRepo.AssertExpectations(t)
	mockJWTService.AssertExpectations(t)
}

// TestAuthHandler_RefreshToken_InvalidToken 测试无效刷新Token
func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	mockJWTService.On("ValidateRefreshToken", "invalid_token").Return(nil, errors.New("invalid token"))

	reqBody := RefreshTokenRequest{
		RefreshToken: "invalid_token",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.RefreshToken(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_REFRESH_TOKEN", response.Code)

	mockJWTService.AssertExpectations(t)
}

// TestNewErrorResponse 测试错误响应创建
func TestNewErrorResponse(t *testing.T) {
	response := NewErrorResponse("TEST_CODE", "测试消息")
	
	assert.Equal(t, "TEST_CODE", response.Code)
	assert.Equal(t, "测试消息", response.Message)
	assert.NotEmpty(t, response.Timestamp)
	assert.Empty(t, response.Details)
	assert.Empty(t, response.RequestID)
}

// TestAuthHandler_Login_AccountLocked 测试账户锁定状态
func TestAuthHandler_Login_AccountLocked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	hashedPassword, _ := hashPassword("password123")
	testUser := &repository.User{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusLocked), // 账户已锁定
	}

	mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(testUser, nil)

	reqBody := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Login(c)

	assert.Equal(t, http.StatusLocked, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ACCOUNT_LOCKED", response.Code)
	assert.Contains(t, response.Message, "账户已被锁定")

	mockUserRepo.AssertExpectations(t)
}

// TestAuthHandler_Register_EmailExists 测试邮箱已存在
func TestAuthHandler_Register_EmailExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	mockUserRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, nil)
	mockUserRepo.On("ExistsByEmail", mock.Anything, "test@example.com").Return(true, nil)

	reqBody := RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Register(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "EMAIL_EXISTS", response.Code)
	assert.Equal(t, "邮箱地址已存在", response.Message)

	mockUserRepo.AssertExpectations(t)
}

// TestAuthHandler_Register_DatabaseError 测试数据库错误
func TestAuthHandler_Register_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockUserRepo := &MockUserRepository{}
	mockJWTService := &MockJWTServiceSimple{}
	logger := zaptest.NewLogger(t)
	handler := NewAuthHandler(mockUserRepo, mockJWTService, logger)

	mockUserRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, errors.New("database error"))

	reqBody := RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Register(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "DATABASE_ERROR", response.Code)

	mockUserRepo.AssertExpectations(t)
}