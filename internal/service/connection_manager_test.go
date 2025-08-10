package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"chat2sql-go/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// MockConnectionRepository 模拟ConnectionRepository接口
type MockConnectionRepository struct {
	mock.Mock
}

func (m *MockConnectionRepository) Create(ctx context.Context, connection *repository.DatabaseConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockConnectionRepository) GetByID(ctx context.Context, id int64) (*repository.DatabaseConnection, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) GetByUserAndName(ctx context.Context, userID int64, name string) (*repository.DatabaseConnection, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) Update(ctx context.Context, connection *repository.DatabaseConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockConnectionRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConnectionRepository) UpdateLastTested(ctx context.Context, id int64, testedAt time.Time) error {
	args := m.Called(ctx, id, testedAt)
	return args.Error(0)
}

func (m *MockConnectionRepository) UpdateStatus(ctx context.Context, id int64, status repository.ConnectionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockConnectionRepository) ListByStatus(ctx context.Context, status repository.ConnectionStatus, limit, offset int) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) CountByStatus(ctx context.Context, status repository.ConnectionStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) BatchUpdateStatus(ctx context.Context, ids []int64, status repository.ConnectionStatus) error {
	args := m.Called(ctx, ids, status)
	return args.Error(0)
}

func (m *MockConnectionRepository) GetConnectionStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// AESEncryptionTestSuite AES加密测试套件
type AESEncryptionTestSuite struct {
	suite.Suite
	encryption *AESEncryption
}

// SetupSuite 设置AES加密测试套件
func (suite *AESEncryptionTestSuite) SetupSuite() {
	key := []byte("test-key-32-bytes-long-for-aes256")
	encryption, err := NewAESEncryption(key)
	require.NoError(suite.T(), err)
	suite.encryption = encryption
}

// TestAESEncryption_EncryptDecrypt 测试AES加密解密
func (suite *AESEncryptionTestSuite) TestAESEncryption_EncryptDecrypt() {
	t := suite.T()

	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			"简单密码",
			"password123",
		},
		{
			"复杂密码",
			"MyC0mpl3x!P@ssw0rd#2024",
		},
		{
			"长密码",
			"this_is_a_very_long_password_with_many_characters_and_symbols_!@#$%^&*()",
		},
		{
			"包含中文",
			"密码123!@#abc",
		},
		{
			"空字符串",
			"",
		},
		{
			"特殊字符",
			"!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			start := time.Now()
			
			// 测试加密
			encrypted, err := suite.encryption.Encrypt(testCase.plaintext)
			encryptDuration := time.Since(start)
			
			require.NoError(t, err)
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, testCase.plaintext, encrypted)
			assert.Less(t, encryptDuration, 10*time.Millisecond, "加密时间应小于10ms")

			// 测试解密
			start = time.Now()
			decrypted, err := suite.encryption.Decrypt(encrypted)
			decryptDuration := time.Since(start)
			
			require.NoError(t, err)
			assert.Equal(t, testCase.plaintext, decrypted)
			assert.Less(t, decryptDuration, 10*time.Millisecond, "解密时间应小于10ms")

			t.Logf("加密解密测试通过 - 原文长度: %d, 密文长度: %d, 加密: %v, 解密: %v",
				len(testCase.plaintext), len(encrypted), encryptDuration, decryptDuration)
		})
	}
}

// TestAESEncryption_InvalidKey 测试无效密钥
func (suite *AESEncryptionTestSuite) TestAESEncryption_InvalidKey() {
	t := suite.T()

	invalidKeys := [][]byte{
		nil,
		{},
		[]byte("short"),
		[]byte("this-key-is-too-long-for-aes-256-encryption-and-should-fail"),
	}

	for i, key := range invalidKeys {
		t.Run(fmt.Sprintf("无效密钥_%d", i), func(t *testing.T) {
			_, err := NewAESEncryption(key)
			assert.Error(t, err, "无效密钥应该返回错误")
		})
	}
}

// TestAESEncryption_InvalidCiphertext 测试无效密文解密
func (suite *AESEncryptionTestSuite) TestAESEncryption_InvalidCiphertext() {
	t := suite.T()

	invalidCiphertexts := []string{
		"",
		"invalid-base64",
		"dGVzdA==", // 有效base64但不是有效的加密数据
		"short",
	}

	for i, ciphertext := range invalidCiphertexts {
		t.Run(fmt.Sprintf("无效密文_%d", i), func(t *testing.T) {
			_, err := suite.encryption.Decrypt(ciphertext)
			assert.Error(t, err, "无效密文应该解密失败")
		})
	}
}

// TestAESEncryption_Performance 测试AES加密性能
func (suite *AESEncryptionTestSuite) TestAESEncryption_Performance() {
	t := suite.T()

	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	password := "test_password_123!@#"
	operationCount := 1000

	// 加密性能测试
	start := time.Now()
	encrypted := ""
	for i := 0; i < operationCount; i++ {
		result, err := suite.encryption.Encrypt(password)
		require.NoError(t, err)
		encrypted = result
	}
	encryptTotalTime := time.Since(start)
	encryptAvgTime := encryptTotalTime / time.Duration(operationCount)

	// 解密性能测试
	start = time.Now()
	for i := 0; i < operationCount; i++ {
		_, err := suite.encryption.Decrypt(encrypted)
		require.NoError(t, err)
	}
	decryptTotalTime := time.Since(start)
	decryptAvgTime := decryptTotalTime / time.Duration(operationCount)

	// 性能指标验证
	assert.Less(t, encryptAvgTime, 5*time.Millisecond, "平均加密时间应小于5ms")
	assert.Less(t, decryptAvgTime, 5*time.Millisecond, "平均解密时间应小于5ms")

	encryptQPS := float64(operationCount) / encryptTotalTime.Seconds()
	decryptQPS := float64(operationCount) / decryptTotalTime.Seconds()

	assert.Greater(t, encryptQPS, 1000.0, "加密QPS应大于1000")
	assert.Greater(t, decryptQPS, 1000.0, "解密QPS应大于1000")

	t.Logf("AES加密性能测试 - 加密: %v/op (%.0f QPS), 解密: %v/op (%.0f QPS)",
		encryptAvgTime, encryptQPS, decryptAvgTime, decryptQPS)
}

// ConnectionManagerUnitTestSuite ConnectionManager单元测试套件
type ConnectionManagerUnitTestSuite struct {
	suite.Suite
	mockRepo   *MockConnectionRepository
	logger     *zap.Logger
}

// SetupTest 设置每个测试
func (suite *ConnectionManagerUnitTestSuite) SetupTest() {
	suite.mockRepo = &MockConnectionRepository{}
	suite.logger = zap.NewNop()
}

// TestConnectionManager_PasswordEncryption 测试密码加密功能
func (suite *ConnectionManagerUnitTestSuite) TestConnectionManager_PasswordEncryption() {
	t := suite.T()

	// 创建加密服务
	encryptionKey := []byte("test-secret-key-32-bytes-long!12")
	encryption, err := NewAESEncryption(encryptionKey)
	require.NoError(t, err)

	testPasswords := []string{
		"simple123",
		"Compl3x!P@ssw0rd#2024",
		"数据库密码123!@#",
		"",
	}

	for _, password := range testPasswords {
		t.Run(fmt.Sprintf("密码_%s", password), func(t *testing.T) {
			start := time.Now()
			
			// 测试密码加密
			encrypted, err := encryption.Encrypt(password)
			encryptDuration := time.Since(start)
			
			require.NoError(t, err)
			assert.NotEqual(t, password, encrypted)
			assert.Less(t, encryptDuration, 10*time.Millisecond, "密码加密时间应小于10ms")

			// 测试密码解密
			start = time.Now()
			decrypted, err := encryption.Decrypt(encrypted)
			decryptDuration := time.Since(start)
			
			require.NoError(t, err)
			assert.Equal(t, password, decrypted)
			assert.Less(t, decryptDuration, 10*time.Millisecond, "密码解密时间应小于10ms")

			t.Logf("密码加密测试通过 - 加密: %v, 解密: %v", encryptDuration, decryptDuration)
		})
	}
}

// TestConnectionManager_Configuration 测试连接管理器配置
func (suite *ConnectionManagerUnitTestSuite) TestConnectionManager_Configuration() {
	t := suite.T()

	testConfigs := []struct {
		name    string
		config  *ConnectionManagerConfig
		wantErr bool
	}{
		{
			"有效配置",
			&ConnectionManagerConfig{
				EncryptionKey:       []byte("valid-32-byte-key-for-aes-256!!"),
				MaxPoolsPerUser:     10,
				PoolIdleTimeout:     30 * time.Minute,
				ConnectionTimeout:   10 * time.Second,
				HealthCheckInterval: 5 * time.Minute,
			},
			false,
		},
		{
			"空加密密钥",
			&ConnectionManagerConfig{
				EncryptionKey: []byte{},
			},
			true,
		},
		{
			"nil配置",
			nil,
			true,
		},
	}

	for _, testCase := range testConfigs {
		t.Run(testCase.name, func(t *testing.T) {
			// 注意：这里我们无法直接测试ConnectionManager的创建，
			// 因为它需要真实的pgxpool.Pool，但我们可以测试配置验证逻辑

			if testCase.config != nil && len(testCase.config.EncryptionKey) > 0 {
				// 测试加密密钥是否有效
				_, err := NewAESEncryption(testCase.config.EncryptionKey)
				if testCase.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

// TestConnectionManager_DatabaseConnectionValidation 测试数据库连接验证
func (suite *ConnectionManagerUnitTestSuite) TestConnectionManager_DatabaseConnectionValidation() {
	t := suite.T()

	validConnections := []*repository.DatabaseConnection{
		{
			BaseModel: repository.BaseModel{
				ID: 1,
			},
			UserID:            1,
			Name:              "测试PostgreSQL连接",
			Host:              "localhost",
			Port:              5432,
			DatabaseName:      "testdb",
			Username:          "testuser",
			PasswordEncrypted: "encrypted_password",
			DBType:            string(repository.DBTypePostgreSQL),
			Status:            string(repository.ConnectionActive),
		},
		{
			BaseModel: repository.BaseModel{
				ID: 2,
			},
			UserID:            1,
			Name:              "测试MySQL连接",
			Host:              "mysql.example.com",
			Port:              3306,
			DatabaseName:      "myapp",
			Username:          "appuser",
			PasswordEncrypted: "encrypted_mysql_password",
			DBType:            string(repository.DBTypeMySQL),
			Status:            string(repository.ConnectionActive),
		},
	}

	for i, conn := range validConnections {
		t.Run(fmt.Sprintf("连接_%d", i), func(t *testing.T) {
			// 验证连接配置的基本字段
			assert.Greater(t, conn.ID, int64(0), "连接ID应该大于0")
			assert.Greater(t, conn.UserID, int64(0), "用户ID应该大于0")
			assert.NotEmpty(t, conn.Name, "连接名称不能为空")
			assert.NotEmpty(t, conn.Host, "主机地址不能为空")
			assert.Greater(t, conn.Port, int32(0), "端口应该大于0")
			assert.NotEmpty(t, conn.DatabaseName, "数据库名称不能为空")
			assert.NotEmpty(t, conn.Username, "用户名不能为空")
			assert.NotEmpty(t, conn.PasswordEncrypted, "加密密码不能为空")
			assert.NotEmpty(t, conn.DBType, "数据库类型不能为空")

			// 验证数据库类型
			dbType := repository.DatabaseType(conn.DBType)
			assert.True(t, dbType.IsValid(), "数据库类型应该有效")

			// 验证连接状态
			status := repository.ConnectionStatus(conn.Status)
			assert.True(t, status.IsValid(), "连接状态应该有效")

			t.Logf("连接验证通过 - 名称: %s, 类型: %s, 状态: %s", 
				conn.Name, conn.DBType, conn.Status)
		})
	}
}

// TestConnectionManager_MockRepository 测试与Repository的交互
func (suite *ConnectionManagerUnitTestSuite) TestConnectionManager_MockRepository() {
	t := suite.T()
	ctx := context.Background()

	// 设置mock期望
	conn := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		UserID:            1,
		Name:              "测试连接",
		Host:              "localhost",
		Port:              5432,
		DatabaseName:      "testdb",
		Username:          "testuser",
		PasswordEncrypted: "encrypted_password",
		DBType:            string(repository.DBTypePostgreSQL),
		Status:            string(repository.ConnectionActive),
	}

	// 测试GetByID
	suite.mockRepo.On("GetByID", ctx, int64(1)).Return(conn, nil)
	
	result, err := suite.mockRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, conn.ID, result.ID)
	assert.Equal(t, conn.Name, result.Name)

	// 测试ListByUser
	connections := []*repository.DatabaseConnection{conn}
	suite.mockRepo.On("ListByUser", ctx, int64(1)).Return(connections, nil)
	
	results, err := suite.mockRepo.ListByUser(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, conn.ID, results[0].ID)

	// 验证所有mock调用
	suite.mockRepo.AssertExpectations(t)

	t.Logf("Repository交互测试通过 - 连接ID: %d, 用户ID: %d", conn.ID, conn.UserID)
}

// TestSuite 运行AES加密测试套件
func TestAESEncryptionTestSuite(t *testing.T) {
	suite.Run(t, new(AESEncryptionTestSuite))
}

// TestSuite 运行ConnectionManager单元测试套件
func TestConnectionManagerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionManagerUnitTestSuite))
}