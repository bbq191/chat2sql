package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// JWTService JWT认证服务
// 基于RS256算法实现企业级JWT Token管理
type JWTService struct {
	privateKey       *rsa.PrivateKey
	publicKey        *rsa.PublicKey
	issuer           string
	audience         string
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
	logger           *zap.Logger
}

// JWTConfig JWT配置
type JWTConfig struct {
	PrivateKeyPath   string        `env:"JWT_PRIVATE_KEY_PATH"`
	PublicKeyPath    string        `env:"JWT_PUBLIC_KEY_PATH"`
	Issuer           string        `env:"JWT_ISSUER" envDefault:"chat2sql-api"`
	Audience         string        `env:"JWT_AUDIENCE" envDefault:"chat2sql-users"`
	AccessTokenTTL   time.Duration `env:"JWT_ACCESS_TTL" envDefault:"1h"`
	RefreshTokenTTL  time.Duration `env:"JWT_REFRESH_TTL" envDefault:"24h"`
	AutoGenerateKeys bool          `env:"JWT_AUTO_GENERATE" envDefault:"true"`
}

// CustomClaims 自定义JWT Claims
// 包含标准Claims和应用特定字段
type CustomClaims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// Validate 实现ClaimsValidator接口的验证方法
func (c CustomClaims) Validate() error {
	// 验证自定义字段
	if c.UserID <= 0 {
		return errors.New("invalid user ID")
	}
	
	if c.Username == "" {
		return errors.New("username cannot be empty")
	}
	
	if c.TokenType != "access" && c.TokenType != "refresh" {
		return errors.New("invalid token type")
	}
	
	return nil
}

// TokenPair Token对结构
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// NewJWTService 创建JWT服务实例
func NewJWTService(config *JWTConfig, logger *zap.Logger) (*JWTService, error) {
	service := &JWTService{
		issuer:          config.Issuer,
		audience:        config.Audience,
		accessTokenTTL:  config.AccessTokenTTL,
		refreshTokenTTL: config.RefreshTokenTTL,
		logger:          logger,
	}
	
	// 加载或生成RSA密钥对
	if err := service.loadOrGenerateKeys(config); err != nil {
		return nil, fmt.Errorf("failed to initialize JWT keys: %w", err)
	}
	
	logger.Info("JWT service initialized successfully",
		zap.String("issuer", config.Issuer),
		zap.Duration("access_ttl", config.AccessTokenTTL),
		zap.Duration("refresh_ttl", config.RefreshTokenTTL))
	
	return service, nil
}

// GenerateTokenPair 生成Token对
func (j *JWTService) GenerateTokenPair(userID int64, username, role string) (*TokenPair, error) {
	now := time.Now()
	
	// 生成Access Token
	accessClaims := &CustomClaims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("user:%d", userID),
			Audience:  jwt.ClaimStrings{j.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(),
		},
	}
	
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(j.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}
	
	// 生成Refresh Token
	refreshClaims := &CustomClaims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("user:%d", userID),
			Audience:  jwt.ClaimStrings{j.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(j.refreshTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(),
		},
	}
	
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(j.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}
	
	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int64(j.accessTokenTTL.Seconds()),
		ExpiresAt:    now.Add(j.accessTokenTTL),
	}, nil
}

// ValidateToken 验证Token
func (j *JWTService) ValidateToken(tokenString string) (*CustomClaims, error) {
	// 解析和验证Token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	
	// 检查Token有效性
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}
	
	// 提取Claims
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}
	
	// 验证自定义Claims
	if err := claims.Validate(); err != nil {
		return nil, fmt.Errorf("claims validation failed: %w", err)
	}
	
	return claims, nil
}

// ValidateAccessToken 验证Access Token
func (j *JWTService) ValidateAccessToken(tokenString string) (*CustomClaims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	
	if claims.TokenType != "access" {
		return nil, errors.New("token is not an access token")
	}
	
	return claims, nil
}

// ValidateRefreshToken 验证Refresh Token
func (j *JWTService) ValidateRefreshToken(tokenString string) (*CustomClaims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	
	if claims.TokenType != "refresh" {
		return nil, errors.New("token is not a refresh token")
	}
	
	return claims, nil
}

// RefreshTokenPair 刷新Token对
func (j *JWTService) RefreshTokenPair(refreshTokenString string) (*TokenPair, error) {
	// 验证Refresh Token
	claims, err := j.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}
	
	// 生成新的Token对
	return j.GenerateTokenPair(claims.UserID, claims.Username, claims.Role)
}

// GetTokenClaims 获取Token Claims（不验证Token有效性）
func (j *JWTService) GetTokenClaims(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.publicKey, nil
	}, jwt.WithoutClaimsValidation())
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}
	
	return claims, nil
}

// loadOrGenerateKeys 加载或生成RSA密钥对
func (j *JWTService) loadOrGenerateKeys(config *JWTConfig) error {
	// 尝试从文件加载密钥
	if config.PrivateKeyPath != "" && config.PublicKeyPath != "" {
		if err := j.loadKeysFromFile(config.PrivateKeyPath, config.PublicKeyPath); err != nil {
			if !config.AutoGenerateKeys {
				return fmt.Errorf("failed to load keys from file: %w", err)
			}
			j.logger.Warn("Failed to load keys from file, generating new keys", zap.Error(err))
		} else {
			j.logger.Info("Successfully loaded RSA keys from files",
				zap.String("private_key", config.PrivateKeyPath),
				zap.String("public_key", config.PublicKeyPath))
			return nil
		}
	}
	
	// 自动生成密钥对
	if config.AutoGenerateKeys {
		return j.generateKeys()
	}
	
	return errors.New("no keys available and auto-generation disabled")
}

// loadKeysFromFile 从文件加载RSA密钥对
func (j *JWTService) loadKeysFromFile(privateKeyPath, publicKeyPath string) error {
	// 加载私钥
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %w", err)
	}
	
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	
	// 加载公钥
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key file: %w", err)
	}
	
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}
	
	j.privateKey = privateKey
	j.publicKey = publicKey
	
	return nil
}

// generateKeys 生成RSA密钥对
func (j *JWTService) generateKeys() error {
	j.logger.Info("Generating new RSA key pair for JWT signing")
	
	// 生成2048位RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA private key: %w", err)
	}
	
	j.privateKey = privateKey
	j.publicKey = &privateKey.PublicKey
	
	j.logger.Info("RSA key pair generated successfully")
	return nil
}

// SaveKeysToFile 保存密钥到文件
func (j *JWTService) SaveKeysToFile(privateKeyPath, publicKeyPath string) error {
	// 保存私钥
	privateKeyPEM, err := j.marshalPrivateKeyToPEM()
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}
	
	// 保存公钥
	publicKeyPEM, err := j.marshalPublicKeyToPEM()
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}
	
	j.logger.Info("JWT keys saved to files",
		zap.String("private_key", privateKeyPath),
		zap.String("public_key", publicKeyPath))
	
	return nil
}

// marshalPrivateKeyToPEM 将私钥编码为PEM格式
func (j *JWTService) marshalPrivateKeyToPEM() ([]byte, error) {
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(j.privateKey)
	if err != nil {
		return nil, err
	}
	
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	
	return privateKeyPEM, nil
}

// marshalPublicKeyToPEM 将公钥编码为PEM格式
func (j *JWTService) marshalPublicKeyToPEM() ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(j.publicKey)
	if err != nil {
		return nil, err
	}
	
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	
	return publicKeyPEM, nil
}

// GetPublicKeyPEM 获取公钥PEM格式（用于外部验证）
func (j *JWTService) GetPublicKeyPEM() ([]byte, error) {
	return j.marshalPublicKeyToPEM()
}

// DefaultJWTConfig 默认JWT配置
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		PrivateKeyPath:   "./configs/jwt_private.pem",
		PublicKeyPath:    "./configs/jwt_public.pem",
		Issuer:           "chat2sql-api",
		Audience:         "chat2sql-users",
		AccessTokenTTL:   1 * time.Hour,
		RefreshTokenTTL:  24 * time.Hour,
		AutoGenerateKeys: true,
	}
}

// generateJTI 生成唯一的JWT ID
func generateJTI() string {
	now := time.Now()
	timestamp := now.UnixNano()
	return fmt.Sprintf("jti_%d", timestamp)
}

// ExtractTokenFromHeader 从Authorization Header提取Token
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is empty")
	}
	
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", errors.New("invalid authorization header format")
	}
	
	token := authHeader[len(bearerPrefix):]
	if token == "" {
		return "", errors.New("token is empty")
	}
	
	return token, nil
}

// ValidateTokenFromRequest 从HTTP请求验证Token
func (j *JWTService) ValidateTokenFromRequest(authHeader string) (*CustomClaims, error) {
	tokenString, err := ExtractTokenFromHeader(authHeader)
	if err != nil {
		return nil, err
	}
	
	return j.ValidateAccessToken(tokenString)
}

// IsTokenExpiringSoon 检查Token是否即将过期
func (j *JWTService) IsTokenExpiringSoon(claims *CustomClaims, threshold time.Duration) bool {
	expTime, err := claims.GetExpirationTime()
	if err != nil || expTime == nil {
		return false
	}
	
	timeUntilExpiration := time.Until(expTime.Time)
	
	return timeUntilExpiration <= threshold
}

// RevokeToken Token撤销（黑名单实现）
// TODO: 实现Token黑名单机制，可以使用Redis存储
func (j *JWTService) RevokeToken(tokenString string) error {
	// 解析Token获取JTI
	claims, err := j.GetTokenClaims(tokenString)
	if err != nil {
		return fmt.Errorf("failed to parse token for revocation: %w", err)
	}
	
	// TODO: 将JTI添加到黑名单（Redis实现）
	j.logger.Info("Token revoked",
		zap.String("jti", claims.RegisteredClaims.ID),
		zap.Int64("user_id", claims.UserID))
	
	return nil
}

// IsTokenRevoked 检查Token是否已被撤销
// TODO: 从Redis黑名单检查
func (j *JWTService) IsTokenRevoked(tokenString string) (bool, error) {
	// TODO: 从Redis黑名单检查JTI
	return false, nil
}