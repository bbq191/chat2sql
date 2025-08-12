package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"chat2sql-go/internal/middleware"
	"chat2sql-go/internal/repository"
)

// UserHandler 用户管理处理器
// 处理用户资料管理、密码修改等用户相关操作
type UserHandler struct {
	userRepo         repository.UserRepository
	queryHistoryRepo repository.QueryHistoryRepository
	connectionRepo   repository.ConnectionRepository
	logger           *zap.Logger
}

// NewUserHandler 创建用户处理器实例
func NewUserHandler(
	userRepo repository.UserRepository,
	queryHistoryRepo repository.QueryHistoryRepository,
	connectionRepo repository.ConnectionRepository,
	logger *zap.Logger,
) *UserHandler {
	return &UserHandler{
		userRepo:         userRepo,
		queryHistoryRepo: queryHistoryRepo,
		connectionRepo:   connectionRepo,
		logger:           logger,
	}
}

// UpdateProfileRequest 更新用户资料请求结构
type UpdateProfileRequest struct {
	Email string `json:"email" binding:"omitempty,email,max=100" example:"new_email@example.com"`
}

// ChangePasswordRequest 修改密码请求结构
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required" example:"OldPass123!"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=100" example:"NewPass123!"`
}

// UserProfileResponse 用户资料响应结构
type UserProfileResponse struct {
	User      *UserInfo           `json:"user"`
	Stats     *UserStats          `json:"stats"`
	LastLogin *time.Time          `json:"last_login,omitempty"`
}

// UserStats 用户统计信息
type UserStats struct {
	TotalQueries    int64 `json:"total_queries" example:"156"`
	SuccessfulQueries int64 `json:"successful_queries" example:"142"`
	TotalConnections int64 `json:"total_connections" example:"3"`
}

// GetProfile 获取用户资料
// @Summary 获取当前用户资料
// @Description 获取当前登录用户的详细资料和统计信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserProfileResponse "获取成功"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/users/profile [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	// 从JWT Token中获取用户ID
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	// 获取用户信息
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user profile",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "USER_NOT_FOUND",
			Message: "用户不存在",
		})
		return
	}
	
	// 获取用户统计信息
	stats, err := h.getUserStatistics(c.Request.Context(), userID)
	if err != nil {
		h.logger.Warn("Failed to get user statistics",
			zap.Error(err),
			zap.Int64("user_id", userID))
		// 统计信息失败不影响主要功能，使用默认值
		stats = &UserStats{
			TotalQueries:      0,
			SuccessfulQueries: 0,
			TotalConnections:  0,
		}
	}
	
	response := &UserProfileResponse{
		User: &UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
			Status:   user.Status,
		},
		Stats: stats,
		LastLogin: user.LastLoginTime,
	}
	
	c.JSON(http.StatusOK, response)
}

// UpdateProfile 更新用户资料
// @Summary 更新用户资料
// @Description 更新当前登录用户的资料信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "更新信息"
// @Success 200 {object} UserInfo "更新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 409 {object} ErrorResponse "邮箱已存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/users/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 获取当前用户信息
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user for update",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "USER_NOT_FOUND",
			Message: "用户不存在",
		})
		return
	}
	
	// 检查邮箱是否已被其他用户使用
	if req.Email != "" && req.Email != user.Email {
		exists, err := h.userRepo.ExistsByEmail(c.Request.Context(), req.Email)
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
				Message: "邮箱地址已被使用",
			})
			return
		}
		
		user.Email = req.Email
	}
	
	// 更新用户信息
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		h.logger.Error("Failed to update user profile",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "UPDATE_FAILED",
			Message: "更新用户资料失败",
		})
		return
	}
	
	h.logger.Info("User profile updated successfully",
		zap.Int64("user_id", userID),
		zap.String("remote_addr", c.ClientIP()))
	
	response := &UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		Status:   user.Status,
	}
	
	c.JSON(http.StatusOK, response)
}

// ChangePassword 修改密码
// @Summary 修改用户密码
// @Description 验证当前密码并设置新密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "密码修改信息"
// @Success 200 {object} SuccessResponse "密码修改成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "当前密码错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/users/change-password [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 获取用户信息验证当前密码
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user for password change",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "数据库查询失败",
		})
		return
	}
	
	// 验证当前密码
	passwordValid, err := verifyPassword(req.CurrentPassword, user.PasswordHash)
	if err != nil {
		h.logger.Error("Failed to verify current password",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "PASSWORD_VERIFICATION_FAILED",
			Message: "密码验证失败",
		})
		return
	}
	
	if !passwordValid {
		h.logger.Warn("Invalid current password for password change",
			zap.Int64("user_id", userID),
			zap.String("remote_addr", c.ClientIP()))
		
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "INVALID_CURRENT_PASSWORD",
			Message: "当前密码错误",
		})
		return
	}
	
	// 加密新密码
	newPasswordHash, err := hashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("Failed to hash new password",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "PASSWORD_HASH_FAILED",
			Message: "新密码加密失败",
		})
		return
	}
	
	// 更新密码
	if err := h.userRepo.UpdatePassword(c.Request.Context(), userID, newPasswordHash); err != nil {
		h.logger.Error("Failed to update password",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "UPDATE_PASSWORD_FAILED",
			Message: "密码更新失败",
		})
		return
	}
	
	h.logger.Info("Password changed successfully",
		zap.Int64("user_id", userID),
		zap.String("remote_addr", c.ClientIP()))
	
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    "PASSWORD_CHANGED",
		Message: "密码修改成功",
	})
}

// getUserIDFromContext 从JWT中间件上下文获取用户ID
func (h *UserHandler) getUserIDFromContext(c *gin.Context) int64 {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		return 0
	}
	return userID
}

// SuccessResponse 成功响应结构
type SuccessResponse struct {
	Code      string `json:"code" example:"SUCCESS"`
	Message   string `json:"message" example:"操作成功"`
	Timestamp string `json:"timestamp" example:"2024-01-08T12:00:00Z"`
}

// NewSuccessResponse 创建标准成功响应
func NewSuccessResponse(code, message string) *SuccessResponse {
	return &SuccessResponse{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// getUserStatistics 获取用户统计信息
func (h *UserHandler) getUserStatistics(ctx context.Context, userID int64) (*UserStats, error) {
	stats := &UserStats{}

	// 获取查询总数
	if h.queryHistoryRepo != nil {
		totalQueries, err := h.queryHistoryRepo.CountByUser(ctx, userID)
		if err != nil {
			h.logger.Warn("Failed to get total queries count",
				zap.Error(err),
				zap.Int64("user_id", userID))
		} else {
			stats.TotalQueries = totalQueries
		}

		// 获取查询执行统计（包含成功查询数）
		executionStats, err := h.queryHistoryRepo.GetExecutionStats(ctx, userID, 365) // 近一年统计
		if err != nil {
			h.logger.Warn("Failed to get execution stats",
				zap.Error(err),
				zap.Int64("user_id", userID))
		} else if executionStats != nil {
			stats.SuccessfulQueries = executionStats.SuccessfulQueries
		}
	}

	// 获取连接总数
	if h.connectionRepo != nil {
		totalConnections, err := h.connectionRepo.CountByUser(ctx, userID)
		if err != nil {
			h.logger.Warn("Failed to get connections count",
				zap.Error(err),
				zap.Int64("user_id", userID))
		} else {
			stats.TotalConnections = totalConnections
		}
	}

	return stats, nil
}

