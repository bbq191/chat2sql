package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_GetConnectionString(t *testing.T) {
	config := &DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "testuser",
		Password:        "testpass",
		Database:        "testdb",
		SSLMode:         "disable",
		ApplicationName: "chat2sql",
		SearchPath:      "public",
		ConnectTimeout:  30 * time.Second,
	}
	
	connStr := config.GetConnectionString()
	// 验证连接字符串包含所有必要的组件
	assert.Contains(t, connStr, "host=localhost")
	assert.Contains(t, connStr, "port=5432")
	assert.Contains(t, connStr, "user=testuser")
	assert.Contains(t, connStr, "password=testpass")
	assert.Contains(t, connStr, "dbname=testdb")
	assert.Contains(t, connStr, "sslmode=disable")
	assert.Contains(t, connStr, "application_name=chat2sql")
	assert.Contains(t, connStr, "search_path=public")
	assert.Contains(t, connStr, "connect_timeout=30")
}

func TestDatabaseConfig_GetConnectionString_WithSSL(t *testing.T) {
	config := &DatabaseConfig{
		Host:            "prod-db.example.com",
		Port:            5432,
		User:            "produser",
		Password:        "prodpass",
		Database:        "proddb",
		SSLMode:         "require",
		SSLCert:         "/path/to/client.crt",
		SSLKey:          "/path/to/client.key",
		ApplicationName: "chat2sql-prod",
		SearchPath:      "public",
		ConnectTimeout:  30 * time.Second,
	}
	
	connStr := config.GetConnectionString()
	// 验证连接字符串包含SSL相关配置
	assert.Contains(t, connStr, "host=prod-db.example.com")
	assert.Contains(t, connStr, "sslmode=require")
	assert.Contains(t, connStr, "sslcert=/path/to/client.crt")
	assert.Contains(t, connStr, "sslkey=/path/to/client.key")
	assert.Contains(t, connStr, "application_name=chat2sql-prod")
}

func TestDatabaseConfig_Validate_Success(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 10,
		MinConns: 1,
	}
	
	err := config.Validate()
	assert.NoError(t, err)
}

func TestDatabaseConfig_Validate_EmptyHost(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库主机地址不能为空")
}

func TestDatabaseConfig_Validate_InvalidPort(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     0,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库端口必须在1-65535范围内")
}

func TestDatabaseConfig_Validate_EmptyUser(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库用户名不能为空")
}

func TestDatabaseConfig_Validate_EmptyDatabase(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "",
		SSLMode:  "disable",
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库名称不能为空")
}

func TestDatabaseConfig_Validate_InvalidSSLMode(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "invalid",
		MaxConns: 10, // 设置有效的MaxConns
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的SSL模式")
}

func TestDatabaseConfig_GetPoolConfigWithLogger(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 10, // 设置有效的MaxConns
	}
	
	// 测试获取连接池配置不会panic
	poolConfig, err := config.GetPoolConfigWithLogger(nil)
	assert.NoError(t, err)
	assert.NotNil(t, poolConfig)
}

func TestDatabaseConfig_DefaultValues(t *testing.T) {
	config := DefaultDatabaseConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "prefer", config.SSLMode) // 从实际代码中看默认是prefer
	assert.Equal(t, int32(100), config.MaxConns)
	assert.Equal(t, int32(10), config.MinConns)
	assert.Equal(t, "chat2sql", config.ApplicationName)
}

// TestDatabaseConfig_ProductionValues 测试生产环境配置
func TestDatabaseConfig_ProductionValues(t *testing.T) {
	config := ProductionDatabaseConfig()
	assert.NotNil(t, config)
	assert.Equal(t, int32(200), config.MaxConns)
	assert.Equal(t, int32(20), config.MinConns)
	assert.Equal(t, "error", config.LogLevel)
	assert.Equal(t, 500*time.Millisecond, config.SlowQueryThreshold)
}

// TestDatabaseConfig_GetPoolConfig_ValidatesFirst 测试获取连接池配置时先验证
func TestDatabaseConfig_GetPoolConfig_ValidatesFirst(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "", // 空主机应该导致验证失败
		Port:     5432,
		User:     "testuser",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 10,
		MinConns: 1,
	}
	
	poolConfig, err := config.GetPoolConfig()
	assert.Error(t, err)
	assert.Nil(t, poolConfig)
	assert.Contains(t, err.Error(), "数据库配置验证失败")
}

// TestDatabaseConfig_Validate_MaxConnsLessThanOrEqualToZero 测试最大连接数验证
func TestDatabaseConfig_Validate_MaxConnsLessThanOrEqualToZero(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 0, // 无效值
		MinConns: 1,
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大连接数必须大于0")
}

// TestDatabaseConfig_Validate_MinConnsNegative 测试最小连接数验证
func TestDatabaseConfig_Validate_MinConnsNegative(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 10,
		MinConns: -1, // 无效值
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最小连接数不能小于0")
}

// TestDatabaseConfig_Validate_MinConnsGreaterThanMaxConns 测试连接数范围验证
func TestDatabaseConfig_Validate_MinConnsGreaterThanMaxConns(t *testing.T) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 5,
		MinConns: 10, // 比最大连接数还大
	}
	
	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最小连接数不能大于最大连接数")
}

// TestDatabaseConfig_Validate_PortOutOfRange 测试端口范围验证
func TestDatabaseConfig_Validate_PortOutOfRange(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"Port too high", 65536},
		{"Port negative", -1},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &DatabaseConfig{
				Host:     "localhost",
				Port:     tc.port,
				User:     "testuser",
				Database: "testdb",
				SSLMode:  "disable",
				MaxConns: 10,
				MinConns: 1,
			}
			
			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "数据库端口必须在1-65535范围内")
		})
	}
}

// TestDatabaseConfig_Validate_AllSSLModes 测试所有SSL模式
func TestDatabaseConfig_Validate_AllSSLModes(t *testing.T) {
	validModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	
	for _, mode := range validModes {
		t.Run("SSL_Mode_"+mode, func(t *testing.T) {
			config := &DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Database: "testdb",
				SSLMode:  mode,
				MaxConns: 10,
				MinConns: 1,
			}
			
			err := config.Validate()
			assert.NoError(t, err)
		})
	}
}

// TestDatabaseConfig_GetConnectionString_WithAllSSLOptions 测试完整SSL配置的连接字符串
func TestDatabaseConfig_GetConnectionString_WithAllSSLOptions(t *testing.T) {
	config := &DatabaseConfig{
		Host:            "ssl-db.example.com",
		Port:            5432,
		User:            "ssluser",
		Password:        "sslpass",
		Database:        "ssldb",
		SSLMode:         "verify-full",
		SSLCert:         "/path/to/client.crt",
		SSLKey:          "/path/to/client.key",
		SSLRootCert:     "/path/to/ca.crt",
		ApplicationName: "ssl-app",
		SearchPath:      "secure,public",
		ConnectTimeout:  60 * time.Second,
	}
	
	connStr := config.GetConnectionString()
	
	// 验证所有SSL相关参数都包含在连接字符串中
	expectedParams := []string{
		"host=ssl-db.example.com",
		"port=5432",
		"user=ssluser",
		"password=sslpass",
		"dbname=ssldb",
		"sslmode=verify-full",
		"sslcert=/path/to/client.crt",
		"sslkey=/path/to/client.key",
		"sslrootcert=/path/to/ca.crt",
		"application_name=ssl-app",
		"search_path=secure,public",
		"connect_timeout=60",
	}
	
	for _, param := range expectedParams {
		assert.Contains(t, connStr, param, "连接字符串应包含: %s", param)
	}
}

// TestDatabaseConfig_GetPoolConfigWithLogger_Success 测试带日志的连接池配置
func TestDatabaseConfig_GetPoolConfigWithLogger_Success(t *testing.T) {
	config := DefaultDatabaseConfig()
	config.LogLevel = "info"
	
	// 创建一个简单的logger进行测试
	// 这里我们只测试方法不会panic，因为创建真实的zap logger比较复杂
	poolConfig, err := config.GetPoolConfigWithLogger(nil)
	assert.NoError(t, err)
	assert.NotNil(t, poolConfig)
	assert.Equal(t, config.MaxConns, poolConfig.MaxConns)
	assert.Equal(t, config.MinConns, poolConfig.MinConns)
}



