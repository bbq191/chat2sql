package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	
	"chat2sql-go/internal/auth"
)

// AuthMiddleware JWT认证中间件
// 验证请求中的JWT Token并设置用户上下文信息
type AuthMiddleware struct {
	jwtService *auth.JWTService
	logger     *zap.Logger
}

// NewAuthMiddleware 创建认证中间件实例
func NewAuthMiddleware(jwtService *auth.JWTService, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		logger:     logger,
	}
}

// JWTAuth JWT认证中间件函数
func (am *AuthMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			am.logger.Warn("Missing authorization header",
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.ClientIP()))
			
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "MISSING_AUTH_HEADER",
				"message": "缺少授权头",
			})
			c.Abort()
			return
		}
		
		// 验证Token
		claims, err := am.jwtService.ValidateTokenFromRequest(authHeader)
		if err != nil {
			am.logger.Warn("JWT validation failed",
				zap.Error(err),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.ClientIP()))
			
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "INVALID_TOKEN",
				"message": "无效的访问令牌",
				"details": err.Error(),
			})
			c.Abort()
			return
		}
		
		// TODO: 检查Token是否被撤销（黑名单机制）
		isRevoked, err := am.jwtService.IsTokenRevoked(authHeader)
		if err != nil {
			am.logger.Error("Failed to check token revocation status",
				zap.Error(err),
				zap.Int64("user_id", claims.UserID))
		} else if isRevoked {
			am.logger.Warn("Revoked token used",
				zap.String("jti", claims.RegisteredClaims.ID),
				zap.Int64("user_id", claims.UserID))
			
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "TOKEN_REVOKED",
				"message": "令牌已被撤销",
			})
			c.Abort()
			return
		}
		
		// 检查Token是否即将过期（可选的刷新提醒）
		if am.jwtService.IsTokenExpiringSoon(claims, 300*time.Second) { // 5分钟内过期
			c.Header("X-Token-Expiring", "true")
		}
		
		// 设置用户上下文信息
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("user_role", claims.Role)
		c.Set("jwt_claims", claims)
		
		am.logger.Debug("JWT authentication successful",
			zap.Int64("user_id", claims.UserID),
			zap.String("username", claims.Username),
			zap.String("role", claims.Role))
		
		c.Next()
	}
}

// RequireRole 角色权限中间件
// 要求用户具有特定角色才能访问接口
func RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "MISSING_ROLE",
				"message": "用户角色信息缺失",
			})
			c.Abort()
			return
		}
		
		roleStr := userRole.(string)
		
		// 检查用户是否具有所需角色
		hasPermission := false
		for _, role := range requiredRoles {
			if roleStr == role {
				hasPermission = true
				break
			}
		}
		
		// 管理员拥有所有权限
		if roleStr == "admin" {
			hasPermission = true
		}
		
		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    "INSUFFICIENT_PERMISSIONS",
				"message": "权限不足",
				"required_roles": requiredRoles,
				"user_role": roleStr,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// RequirePermission 权限检查中间件
// 基于更细粒度的权限控制
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "MISSING_ROLE",
				"message": "用户角色信息缺失",
			})
			c.Abort()
			return
		}
		
		roleStr := userRole.(string)
		
		// 权限检查逻辑
		hasPermission := checkPermission(roleStr, permission)
		
		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    "INSUFFICIENT_PERMISSIONS",
				"message": "权限不足",
				"required_permission": permission,
				"user_role": roleStr,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// OptionalAuth 可选认证中间件
// 如果提供了Token则验证，但不强制要求认证
func (am *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 没有提供Token，继续处理但不设置用户信息
			c.Next()
			return
		}
		
		// 有Token时进行验证
		claims, err := am.jwtService.ValidateTokenFromRequest(authHeader)
		if err != nil {
			am.logger.Warn("Optional JWT validation failed",
				zap.Error(err),
				zap.String("path", c.Request.URL.Path))
			
			// 验证失败但不阻止请求
			c.Next()
			return
		}
		
		// 设置用户上下文信息
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("user_role", claims.Role)
		c.Set("jwt_claims", claims)
		
		c.Next()
	}
}

// GetUserIDFromContext 从Gin上下文获取用户ID
func GetUserIDFromContext(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	
	id, ok := userID.(int64)
	return id, ok
}

// GetUsernameFromContext 从Gin上下文获取用户名
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	
	name, ok := username.(string)
	return name, ok
}

// GetUserRoleFromContext 从Gin上下文获取用户角色
func GetUserRoleFromContext(c *gin.Context) (string, bool) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", false
	}
	
	roleStr, ok := role.(string)
	return roleStr, ok
}

// GetJWTClaimsFromContext 从Gin上下文获取JWT Claims
func GetJWTClaimsFromContext(c *gin.Context) (*auth.CustomClaims, bool) {
	claims, exists := c.Get("jwt_claims")
	if !exists {
		return nil, false
	}
	
	jwtClaims, ok := claims.(*auth.CustomClaims)
	return jwtClaims, ok
}

// checkPermission 检查角色是否具有指定权限
func checkPermission(role, permission string) bool {
	// 管理员拥有所有权限
	if role == "admin" {
		return true
	}
	
	// 基于角色的权限映射
	rolePermissions := map[string][]string{
		"user": {
			"query:execute",
			"connection:manage",
			"profile:read",
			"profile:update",
		},
		"manager": {
			"query:execute",
			"connection:manage",
			"profile:read",
			"profile:update",
			"history:view_team",
			"connection:view_team",
		},
		"admin": {
			// 管理员拥有所有权限，在上面已处理
		},
	}
	
	permissions, exists := rolePermissions[role]
	if !exists {
		return false
	}
	
	for _, perm := range permissions {
		if perm == permission {
			return true
		}
	}
	
	return false
}

// UserIDFromRequest 从请求获取用户ID（用于兼容性）
// 用于不使用中间件的场景
func UserIDFromRequest(c *gin.Context) int64 {
	// 首先尝试从上下文获取（JWT中间件设置）
	if userID, exists := GetUserIDFromContext(c); exists {
		return userID
	}
	
	// 备用方案：从Header获取（开发阶段）
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		return 0
	}
	
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0
	}
	
	return userID
}