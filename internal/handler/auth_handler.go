package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	
	"chat2sql-go/internal/repository"
)

// AuthHandler 认证处理器
// 处理用户注册、登录、Token刷新等认证相关操作
type AuthHandler struct {
	userRepo repository.UserRepository
	// jwtService JWT服务，后续实现
	logger   *zap.Logger
}

// NewAuthHandler 创建认证处理器实例
func NewAuthHandler(userRepo repository.UserRepository, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo,
		logger:   logger,
	}
}

// RegisterRequest 用户注册请求结构
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"john_doe"`
	Email    string `json:"email" binding:"required,email,max=100" example:"john@example.com"`
	Password string `json:"password" binding:"required,min=8,max=100" example:"SecurePass123!"`
}

// LoginRequest 用户登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"john_doe"`
	Password string `json:"password" binding:"required" example:"SecurePass123!"`
}

// RefreshTokenRequest Token刷新请求结构
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"eyJhbGciOiJSUzI1NiI..."`
}

// AuthResponse 认证响应结构
type AuthResponse struct {
	AccessToken  string    `json:"access_token" example:"eyJhbGciOiJSUzI1NiI..."`
	RefreshToken string    `json:"refresh_token" example:"eyJhbGciOiJSUzI1NiI..."`
	TokenType    string    `json:"token_type" example:"Bearer"`
	ExpiresIn    int64     `json:"expires_in" example:"3600"`
	ExpiresAt    time.Time `json:"expires_at" example:"2024-01-08T13:00:00Z"`
	User         *UserInfo `json:"user"`
}

// UserInfo 用户信息结构（不包含敏感信息）
type UserInfo struct {
	ID       int64  `json:"id" example:"1"`
	Username string `json:"username" example:"john_doe"`
	Email    string `json:"email" example:"john@example.com"`
	Role     string `json:"role" example:"user"`
	Status   string `json:"status" example:"active"`
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账户，需要唯一的用户名和邮箱
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "注册信息"
// @Success 201 {object} AuthResponse "注册成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 409 {object} ErrorResponse "用户名或邮箱已存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	
	// 绑定和验证请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid register request", 
			zap.Error(err),
			zap.String("remote_addr", c.ClientIP()))
		
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 检查用户名是否已存在
	exists, err := h.userRepo.ExistsByUsername(c.Request.Context(), req.Username)
	if err != nil {
		h.logger.Error("Failed to check username existence",
			zap.Error(err),
			zap.String("username", req.Username))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "数据库查询失败",
		})
		return
	}
	
	if exists {
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:    "USERNAME_EXISTS",
			Message: "用户名已存在",
		})
		return
	}
	
	// 检查邮箱是否已存在
	exists, err = h.userRepo.ExistsByEmail(c.Request.Context(), req.Email)
	if err != nil {
		h.logger.Error("Failed to check email existence",
			zap.Error(err),
			zap.String("email", req.Email))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "数据库查询失败",
		})
		return
	}
	
	if exists {
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:    "EMAIL_EXISTS",
			Message: "邮箱地址已存在",
		})
		return
	}
	
	// TODO: 密码哈希加密（后续集成bcrypt）
	passwordHash := hashPassword(req.Password)
	
	// 创建新用户
	user := &repository.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	
	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		h.logger.Error("Failed to create user",
			zap.Error(err),
			zap.String("username", req.Username))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "CREATE_USER_FAILED",
			Message: "用户创建失败",
		})
		return
	}
	
	// TODO: 生成JWT Token（后续实现）
	response := h.generateAuthResponse(user)
	
	h.logger.Info("User registered successfully",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("remote_addr", c.ClientIP()))
	
	c.JSON(http.StatusCreated, response)
}

// Login 用户登录
// @Summary 用户登录
// @Description 验证用户凭据并返回JWT Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} AuthResponse "登录成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "用户名或密码错误"
// @Failure 423 {object} ErrorResponse "账户被锁定"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	
	// 绑定和验证请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid login request",
			zap.Error(err),
			zap.String("remote_addr", c.ClientIP()))
		
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 密码哈希验证
	passwordHash := hashPassword(req.Password)
	
	// 验证用户凭据
	user, err := h.userRepo.ValidateCredentials(c.Request.Context(), req.Username, passwordHash)
	if err != nil {
		h.logger.Warn("Invalid login credentials",
			zap.Error(err),
			zap.String("username", req.Username),
			zap.String("remote_addr", c.ClientIP()))
		
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "INVALID_CREDENTIALS",
			Message: "用户名或密码错误",
		})
		return
	}
	
	// 检查用户状态
	if !user.IsActive() {
		status := ""
		switch user.Status {
		case string(repository.StatusLocked):
			status = "账户已被锁定"
		case string(repository.StatusInactive):
			status = "账户未激活"
		default:
			status = "账户状态异常"
		}
		
		c.JSON(http.StatusLocked, ErrorResponse{
			Code:    "ACCOUNT_LOCKED",
			Message: status,
		})
		return
	}
	
	// 更新最后登录时间
	if err := h.userRepo.UpdateLastLogin(c.Request.Context(), user.ID, time.Now()); err != nil {
		h.logger.Warn("Failed to update last login time",
			zap.Error(err),
			zap.Int64("user_id", user.ID))
	}
	
	// TODO: 生成JWT Token（后续实现）
	response := h.generateAuthResponse(user)
	
	h.logger.Info("User logged in successfully",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("remote_addr", c.ClientIP()))
	
	c.JSON(http.StatusOK, response)
}

// RefreshToken Token刷新
// @Summary 刷新访问Token
// @Description 使用刷新Token获取新的访问Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "刷新Token请求"
// @Success 200 {object} AuthResponse "Token刷新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "刷新Token无效或已过期"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	
	// 绑定和验证请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// TODO: 验证刷新Token（后续实现JWT验证）
	// claims, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
	
	// 临时实现 - 模拟Token验证成功
	userID := int64(1) // TODO: 从Token解析用户ID
	
	// 获取用户信息
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user for token refresh",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "INVALID_REFRESH_TOKEN",
			Message: "刷新Token无效",
		})
		return
	}
	
	// 检查用户状态
	if !user.IsActive() {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "ACCOUNT_INACTIVE",
			Message: "账户状态异常，无法刷新Token",
		})
		return
	}
	
	// TODO: 生成新的JWT Token对（后续实现）
	response := h.generateAuthResponse(user)
	
	h.logger.Info("Token refreshed successfully",
		zap.Int64("user_id", user.ID),
		zap.String("remote_addr", c.ClientIP()))
	
	c.JSON(http.StatusOK, response)
}

// generateAuthResponse 生成认证响应
// TODO: 集成JWT服务后完善Token生成逻辑
func (h *AuthHandler) generateAuthResponse(user *repository.User) *AuthResponse {
	// 临时实现 - 模拟Token生成
	accessToken := "mock_access_token_" + strings.ReplaceAll(time.Now().Format(time.RFC3339Nano), ":", "_")
	refreshToken := "mock_refresh_token_" + strings.ReplaceAll(time.Now().Format(time.RFC3339Nano), ":", "_")
	
	expiresIn := int64(3600) // 1小时
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	
	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		ExpiresAt:    expiresAt,
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
			Status:   user.Status,
		},
	}
}

// hashPassword 密码哈希加密
// TODO: 集成bcrypt库进行密码加密
func hashPassword(password string) string {
	// 临时实现 - 后续集成bcrypt
	return "hashed_" + password
}

// ErrorResponse 统一错误响应结构
type ErrorResponse struct {
	Code      string `json:"code" example:"INVALID_REQUEST"`
	Message   string `json:"message" example:"请求参数格式错误"`
	Details   string `json:"details,omitempty" example:"validation failed"`
	Timestamp string `json:"timestamp" example:"2024-01-08T12:00:00Z"`
	RequestID string `json:"request_id,omitempty" example:"req_123456"`
}

// NewErrorResponse 创建标准错误响应
func NewErrorResponse(code, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}