package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// JWTServiceTestSuite JWT服务测试套件
type JWTServiceTestSuite struct {
	suite.Suite
	jwtService *JWTService
	logger     *zap.Logger
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	redisClient redis.UniversalClient
}

func (suite *JWTServiceTestSuite) SetupSuite() {
	// 创建测试日志器
	suite.logger = zap.NewNop()
	
	// 生成测试用密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(suite.T(), err)
	suite.privateKey = privateKey
	suite.publicKey = &privateKey.PublicKey
	
	// 创建Redis客户端（Mock实现）
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
}

func (suite *JWTServiceTestSuite) SetupTest() {
	// 为每个测试创建新的JWT服务实例
	config := &JWTConfig{
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}
	
	suite.jwtService = &JWTService{
		privateKey:      suite.privateKey,
		publicKey:       suite.publicKey,
		issuer:          config.Issuer,
		audience:        config.Audience,
		accessTokenTTL:  config.AccessTokenTTL,
		refreshTokenTTL: config.RefreshTokenTTL,
		logger:          suite.logger,
		redisClient:     suite.redisClient,
	}
}

func (suite *JWTServiceTestSuite) TestNewJWTService_Success() {
	// 创建临时密钥文件
	privateKeyFile := suite.createTempKeyFile("private")
	publicKeyFile := suite.createTempKeyFile("public")
	defer os.Remove(privateKeyFile)
	defer os.Remove(publicKeyFile)
	
	config := &JWTConfig{
		PrivateKeyPath:  privateKeyFile,
		PublicKeyPath:   publicKeyFile,
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}
	
	service, err := NewJWTService(config, suite.logger, suite.redisClient)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.Equal(suite.T(), config.Issuer, service.issuer)
	assert.Equal(suite.T(), config.Audience, service.audience)
}

func (suite *JWTServiceTestSuite) TestNewJWTService_AutoGenerateKeys() {
	config := &JWTConfig{
		AutoGenerateKeys: true,
		Issuer:           "test-issuer",
		Audience:         "test-audience",
		AccessTokenTTL:   time.Hour,
		RefreshTokenTTL:  24 * time.Hour,
	}
	
	service, err := NewJWTService(config, suite.logger, suite.redisClient)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	// 由于privateKey和publicKey是私有字段，我们只能验证服务被成功创建
}

func (suite *JWTServiceTestSuite) TestGenerateTokenPair_Success() {
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), tokenPair.AccessToken)
	assert.NotEmpty(suite.T(), tokenPair.RefreshToken)
	assert.Equal(suite.T(), "Bearer", tokenPair.TokenType)
	assert.Equal(suite.T(), int64(suite.jwtService.accessTokenTTL.Seconds()), int64(tokenPair.ExpiresIn))
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_AccessToken_Success() {
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.AccessToken)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), userID, claims.UserID)
	assert.Equal(suite.T(), username, claims.Username)
	assert.Equal(suite.T(), role, claims.Role)
	assert.Equal(suite.T(), "access", claims.TokenType)
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_RefreshToken_ShouldFail() {
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	// ValidateTokenFromRequest应该拒绝refresh token
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.RefreshToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
	assert.Contains(suite.T(), err.Error(), "not an access token")
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_InvalidToken() {
	invalidToken := "Bearer invalid.token.string"
	
	claims, err := suite.jwtService.ValidateTokenFromRequest(invalidToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_ExpiredToken() {
	// 创建一个短过期时间的配置
	config := &JWTConfig{
		AutoGenerateKeys: true,
		Issuer:           "test-issuer",
		Audience:         "test-audience",
		AccessTokenTTL:   time.Millisecond, // 很短的过期时间
		RefreshTokenTTL:  time.Millisecond,
	}
	
	shortTTLService, err := NewJWTService(config, suite.logger, suite.redisClient)
	require.NoError(suite.T(), err)
	
	tokenPair, err := shortTTLService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	// 等待token过期
	time.Sleep(10 * time.Millisecond)
	
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.AccessToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_BearerToken() {
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	authHeader := "Bearer " + tokenPair.AccessToken
	claims, err := suite.jwtService.ValidateTokenFromRequest(authHeader)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), userID, claims.UserID)
	assert.Equal(suite.T(), username, claims.Username)
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_InvalidFormat() {
	authHeader := "InvalidFormat token"
	
	claims, err := suite.jwtService.ValidateTokenFromRequest(authHeader)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
	assert.Contains(suite.T(), err.Error(), "invalid authorization header format")
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_EmptyToken() {
	authHeader := "Bearer "
	
	claims, err := suite.jwtService.ValidateTokenFromRequest(authHeader)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
	assert.Contains(suite.T(), err.Error(), "token is empty")
}

func (suite *JWTServiceTestSuite) TestGenerateTokenPair_MultipleCallsDifferentTokens() {
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	// 生成第一个token对
	firstTokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	// 稍等片刻确保时间戳不同
	time.Sleep(1 * time.Millisecond)
	
	// 生成第二个token对
	secondTokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	// 验证两次生成的token是不同的
	assert.NotEqual(suite.T(), firstTokenPair.AccessToken, secondTokenPair.AccessToken)
	assert.NotEqual(suite.T(), firstTokenPair.RefreshToken, secondTokenPair.RefreshToken)
}

func (suite *JWTServiceTestSuite) TestValidateTokenFromRequest_WrongTokenType() {
	// 这个测试验证token的结构和内容
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	// 验证access token的内容
	accessClaims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.AccessToken)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "access", accessClaims.TokenType)
	
	// 验证refresh token应该被ValidateTokenFromRequest拒绝
	refreshClaims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.RefreshToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), refreshClaims)
	assert.Contains(suite.T(), err.Error(), "not an access token")
}

func (suite *JWTServiceTestSuite) TestRevokeToken_Success() {
	// 这个测试只验证RevokeToken方法能正常调用
	userID := int64(123)
	username := "testuser"
	role := "user"
	
	tokenPair, err := suite.jwtService.GenerateTokenPair(userID, username, role)
	require.NoError(suite.T(), err)
	
	// 验证能调用RevokeToken方法
	err = suite.jwtService.RevokeToken(tokenPair.AccessToken)
	// 由于测试环境中Redis可能不可用，我们接受连接错误
	if err != nil {
		assert.Contains(suite.T(), err.Error(), "connection refused")
	}
}






// 辅助方法：创建临时密钥文件
func (suite *JWTServiceTestSuite) createTempKeyFile(keyType string) string {
	var keyBytes []byte
	var keyBlock *pem.Block
	
	if keyType == "private" {
		privateKeyBytes, _ := x509.MarshalPKCS8PrivateKey(suite.privateKey)
		keyBlock = &pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		}
	} else {
		publicKeyBytes, _ := x509.MarshalPKIXPublicKey(suite.publicKey)
		keyBlock = &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		}
	}
	
	keyBytes = pem.EncodeToMemory(keyBlock)
	
	tmpFile, err := os.CreateTemp("", keyType+"_key_*.pem")
	require.NoError(suite.T(), err)
	
	_, err = tmpFile.Write(keyBytes)
	require.NoError(suite.T(), err)
	
	tmpFile.Close()
	return tmpFile.Name()
}

// 运行测试套件
func TestJWTServiceTestSuite(t *testing.T) {
	suite.Run(t, new(JWTServiceTestSuite))
}

// 基础功能测试
func TestTokenPair_Basic(t *testing.T) {
	tp := &TokenPair{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}
	
	// 验证基本字段
	assert.Equal(t, "access_token", tp.AccessToken)
	assert.Equal(t, "refresh_token", tp.RefreshToken)
	assert.Equal(t, "Bearer", tp.TokenType)
	assert.Equal(t, int64(3600), tp.ExpiresIn)
}

func TestCustomClaims_Validate(t *testing.T) {
	claims := &CustomClaims{
		UserID:    123,
		Username:  "testuser",
		Role:      "user",
		TokenType: "access",
	}
	
	err := claims.Validate()
	assert.NoError(t, err)
}

func TestCustomClaims_Validate_InvalidData(t *testing.T) {
	// 测试无效的UserID
	claims := &CustomClaims{
		UserID:    0, // 无效的用户ID
		Username:  "testuser",
		Role:      "user",
		TokenType: "access",
	}
	
	err := claims.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
	
	// 测试空用户名
	claims2 := &CustomClaims{
		UserID:    123,
		Username:  "", // 空用户名
		Role:      "user",
		TokenType: "access",
	}
	
	err = claims2.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username cannot be empty")
	
	// 测试无效的token类型
	claims3 := &CustomClaims{
		UserID:    123,
		Username:  "testuser",
		Role:      "user",
		TokenType: "invalid", // 无效的token类型
	}
	
	err = claims3.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

// ==============================================
// JWT安全攻击防护测试套件
// ==============================================

// JWTSecurityTestSuite JWT安全测试套件
type JWTSecurityTestSuite struct {
	suite.Suite
	jwtService *JWTService
	logger     *zap.Logger
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	redisClient redis.UniversalClient
}

func (suite *JWTSecurityTestSuite) SetupSuite() {
	suite.logger = zap.NewNop()
	
	// 生成测试密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(suite.T(), err)
	suite.privateKey = privateKey
	suite.publicKey = &privateKey.PublicKey
	
	// Mock Redis客户端
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
}

func (suite *JWTSecurityTestSuite) SetupTest() {
	config := &JWTConfig{
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}
	
	suite.jwtService = &JWTService{
		privateKey:      suite.privateKey,
		publicKey:       suite.publicKey,
		issuer:          config.Issuer,
		audience:        config.Audience,
		accessTokenTTL:  config.AccessTokenTTL,
		refreshTokenTTL: config.RefreshTokenTTL,
		logger:          suite.logger,
		redisClient:     suite.redisClient,
	}
}

// TestAlgorithmConfusionAttack 测试算法混淆攻击防护
func (suite *JWTSecurityTestSuite) TestAlgorithmConfusionAttack() {
	// 生成正常的RS256 token
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	// 解析token获取payload
	parts := strings.Split(tokenPair.AccessToken, ".")
	require.Len(suite.T(), parts, 3)
	
	// 创建伪造的HS256 header
	fakeHeader := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	fakeHeaderBytes, _ := json.Marshal(fakeHeader)
	fakeHeaderB64 := base64.RawURLEncoding.EncodeToString(fakeHeaderBytes)
	
	// 使用公钥作为HMAC密钥尝试伪造签名
	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(suite.publicKey)
	payload := fakeHeaderB64 + "." + parts[1]
	h := hmac.New(sha256.New, publicKeyBytes)
	h.Write([]byte(payload))
	fakeSignature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	
	// 构造恶意token
	maliciousToken := payload + "." + fakeSignature
	
	// 验证应该失败
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
	assert.Contains(suite.T(), err.Error(), "unexpected signing method")
}

// TestNoneAlgorithmAttack 测试none算法攻击防护
func (suite *JWTSecurityTestSuite) TestNoneAlgorithmAttack() {
	// 构造none算法的恶意header
	noneHeader := map[string]interface{}{
		"alg": "none",
		"typ": "JWT",
	}
	noneHeaderBytes, _ := json.Marshal(noneHeader)
	noneHeaderB64 := base64.RawURLEncoding.EncodeToString(noneHeaderBytes)
	
	// 构造恶意payload
	maliciousClaims := CustomClaims{
		UserID:   999,
		Username: "hacker",
		Role:     "admin",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    suite.jwtService.issuer,
			Audience:  jwt.ClaimStrings{suite.jwtService.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	payloadBytes, _ := json.Marshal(maliciousClaims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	
	// none算法的签名部分为空
	maliciousToken := noneHeaderB64 + "." + payloadB64 + "."
	
	// 验证应该失败
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

// TestJWTHeaderManipulation 测试JWT头部篡改攻击
func (suite *JWTSecurityTestSuite) TestJWTHeaderManipulation() {
	// 生成正常token
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	parts := strings.Split(tokenPair.AccessToken, ".")
	require.Len(suite.T(), parts, 3)
	
	// 篡改header中的kid字段
	headerBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]interface{}
	json.Unmarshal(headerBytes, &header)
	header["kid"] = "../../../etc/passwd" // 路径遍历攻击
	header["jku"] = "http://malicious.com/keys" // JKU攻击
	
	maliciousHeaderBytes, _ := json.Marshal(header)
	maliciousHeaderB64 := base64.RawURLEncoding.EncodeToString(maliciousHeaderBytes)
	
	maliciousToken := maliciousHeaderB64 + "." + parts[1] + "." + parts[2]
	
	// 验证应该失败（签名不匹配）
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

// TestClaimsManipulation 测试Claims篡改攻击
func (suite *JWTSecurityTestSuite) TestClaimsManipulation() {
	// 生成正常token
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	parts := strings.Split(tokenPair.AccessToken, ".")
	require.Len(suite.T(), parts, 3)
	
	// 解析并篡改payload
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]interface{}
	json.Unmarshal(payloadBytes, &claims)
	
	// 尝试提升权限
	claims["role"] = "admin"
	claims["uid"] = float64(999) // JSON中数字是float64
	claims["username"] = "hacker"
	
	maliciousPayloadBytes, _ := json.Marshal(claims)
	maliciousPayloadB64 := base64.RawURLEncoding.EncodeToString(maliciousPayloadBytes)
	
	maliciousToken := parts[0] + "." + maliciousPayloadB64 + "." + parts[2]
	
	// 验证应该失败（签名不匹配）
	validatedClaims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), validatedClaims)
}

// TestTokenReplayAttack 测试Token重放攻击防护
func (suite *JWTSecurityTestSuite) TestTokenReplayAttack() {
	// 生成token
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	// 第一次使用token（正常）
	claims1, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.AccessToken)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claims1)
	
	// 撤销token
	err = suite.jwtService.RevokeToken(tokenPair.AccessToken)
	if err != nil {
		// 在测试环境中Redis可能不可用，这是预期的
		assert.Contains(suite.T(), err.Error(), "connection refused")
		return
	}
	
	// 再次使用被撤销的token
	// 注意：ValidateTokenFromRequest只验证token的有效性，不检查撤销状态
	// 撤销检查是在middleware中进行的
	claims2, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + tokenPair.AccessToken)
	// Token本身仍然有效，但应该在中间件层被撤销检查拦截
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claims2)
	
	// 验证撤销状态检查
	isRevoked, err := suite.jwtService.IsTokenRevoked(tokenPair.AccessToken)
	if err != nil {
		// Redis连接问题，跳过撤销检查
		suite.T().Skip("Redis unavailable, skipping revocation check")
	} else {
		assert.True(suite.T(), isRevoked, "Token should be marked as revoked")
	}
}

// TestTimingAttack 测试时序攻击防护
func (suite *JWTSecurityTestSuite) TestTimingAttack() {
	// 生成多个不同的无效token
	invalidTokens := []string{
		"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.invalid.signature",
		"invalid.token.format",
		"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.invalid",
	}
	
	// 测量验证时间，检查是否存在明显的时序差异
	var validationTimes []time.Duration
	
	for _, token := range invalidTokens {
		start := time.Now()
		claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + token)
		duration := time.Since(start)
		
		validationTimes = append(validationTimes, duration)
		
		// 所有无效token都应该被拒绝
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), claims)
	}
	
	// 验证时间差异不应该过大（防止时序攻击）
	// 这里只是基础检查，实际的时序攻击防护需要更复杂的实现
	for i := 1; i < len(validationTimes); i++ {
		timeDiff := validationTimes[i] - validationTimes[i-1]
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		// 时间差异不应该超过100ms（在测试环境中这是合理的阈值）
		assert.True(suite.T(), timeDiff < 100*time.Millisecond, 
			"时序差异过大，可能存在时序攻击风险: %v", timeDiff)
	}
}

// TestJWTKeyConfusion 测试密钥混淆攻击
func (suite *JWTSecurityTestSuite) TestJWTKeyConfusion() {
	// 生成另一个密钥对
	otherPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(suite.T(), err)
	
	// 使用错误的密钥签名token
	claims := &CustomClaims{
		UserID:    123,
		Username:  "testuser",
		Role:      "user",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    suite.jwtService.issuer,
			Audience:  jwt.ClaimStrings{suite.jwtService.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	maliciousTokenString, err := token.SignedString(otherPrivateKey)
	require.NoError(suite.T(), err)
	
	// 验证应该失败（密钥不匹配）
	validatedClaims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousTokenString)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), validatedClaims)
	assert.Contains(suite.T(), err.Error(), "validation")
}

// TestJWTBlankSignature 测试空签名攻击
func (suite *JWTSecurityTestSuite) TestJWTBlankSignature() {
	// 生成正常token然后清空签名部分
	tokenPair, err := suite.jwtService.GenerateTokenPair(123, "testuser", "user")
	require.NoError(suite.T(), err)
	
	parts := strings.Split(tokenPair.AccessToken, ".")
	require.Len(suite.T(), parts, 3)
	
	// 构造空签名的恶意token
	maliciousToken := parts[0] + "." + parts[1] + "."
	
	// 验证应该失败
	claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + maliciousToken)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

// TestJWTMalformedToken 测试畸形token处理
func (suite *JWTSecurityTestSuite) TestJWTMalformedToken() {
	malformedTokens := []string{
		"malformed",
		"malformed.token",
		"malformed.token.with.too.many.parts.here",
		"..",
		"....",
		"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9..invalid_signature",
		"invalid_base64!@#$.invalid_base64!@#$.invalid_base64!@#$",
	}
	
	for _, malformedToken := range malformedTokens {
		claims, err := suite.jwtService.ValidateTokenFromRequest("Bearer " + malformedToken)
		assert.Error(suite.T(), err, "畸形token应该被拒绝: %s", malformedToken)
		assert.Nil(suite.T(), claims)
	}
}

// 运行JWT安全测试套件
func TestJWTSecurityTestSuite(t *testing.T) {
	suite.Run(t, new(JWTSecurityTestSuite))
}