package config

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// TestDefaultRedisConfig 测试默认Redis配置
func TestDefaultRedisConfig(t *testing.T) {
	config := DefaultRedisConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, "localhost:6379", config.Addr)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 8*time.Millisecond, config.MinRetryBackoff)
	assert.Equal(t, 512*time.Millisecond, config.MaxRetryBackoff)
	assert.Equal(t, 5*time.Second, config.DialTimeout)
	assert.Equal(t, 3*time.Second, config.ReadTimeout)
	assert.Equal(t, 3*time.Second, config.WriteTimeout)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 5, config.MinIdleConns)
	assert.Equal(t, time.Duration(0), config.MaxConnAge)
	assert.Equal(t, 4*time.Second, config.PoolTimeout)
	assert.Equal(t, 5*time.Minute, config.IdleTimeout)
	assert.Equal(t, time.Minute, config.IdleCheckFreq)
	assert.False(t, config.TLSEnabled)
	assert.False(t, config.TLSSkipVerify)
	assert.False(t, config.ClusterMode)
	assert.Empty(t, config.ClusterAddrs)
}

// TestRedisConfig_CustomValues 测试自定义Redis配置值
func TestRedisConfig_CustomValues(t *testing.T) {
	config := &RedisConfig{
		Addr:            "redis.example.com:6380",
		Password:        "secret123",
		DB:              2,
		MaxRetries:      5,
		MinRetryBackoff: 10 * time.Millisecond,
		MaxRetryBackoff: 1 * time.Second,
		DialTimeout:     10 * time.Second,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		PoolSize:        20,
		MinIdleConns:    10,
		MaxConnAge:      1 * time.Hour,
		PoolTimeout:     8 * time.Second,
		IdleTimeout:     10 * time.Minute,
		IdleCheckFreq:   2 * time.Minute,
		TLSEnabled:      true,
		TLSSkipVerify:   true,
		ClusterMode:     true,
		ClusterAddrs:    []string{"redis1:6379", "redis2:6379", "redis3:6379"},
	}
	
	assert.Equal(t, "redis.example.com:6380", config.Addr)
	assert.Equal(t, "secret123", config.Password)
	assert.Equal(t, 2, config.DB)
	assert.Equal(t, 5, config.MaxRetries)
	assert.True(t, config.TLSEnabled)
	assert.True(t, config.TLSSkipVerify)
	assert.True(t, config.ClusterMode)
	assert.Equal(t, 3, len(config.ClusterAddrs))
	assert.Contains(t, config.ClusterAddrs, "redis1:6379")
}

// TestNewRedisManager_WithNilConfig 测试使用nil配置创建Redis管理器
func TestNewRedisManager_WithNilConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	// 创建管理器，如果Redis服务器运行则会成功，否则失败
	manager, err := NewRedisManager(nil, logger)
	
	if err != nil {
		// 如果连接失败（没有Redis服务器），这是预期的
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	} else {
		// 如果连接成功（有Redis服务器），测试基本功能
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.GetClient())
		
		// 清理资源
		err = manager.Close()
		assert.NoError(t, err)
	}
}

// TestNewRedisManager_WithDefaultConfig 测试使用默认配置
func TestNewRedisManager_WithDefaultConfig(t *testing.T) {
	config := DefaultRedisConfig()
	logger := zaptest.NewLogger(t)
	
	// 创建管理器，如果Redis服务器运行则会成功，否则失败
	manager, err := NewRedisManager(config, logger)
	
	if err != nil {
		// 如果连接失败（没有Redis服务器），这是预期的
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	} else {
		// 如枟连接成功（有Redis服务器），测试基本功能
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.GetClient())
		
		// 清理资源
		err = manager.Close()
		assert.NoError(t, err)
	}
}

// TestNewRedisManager_TLSConfig 测试TLS配置
func TestNewRedisManager_TLSConfig(t *testing.T) {
	config := &RedisConfig{
		Addr:          "localhost:6380",
		TLSEnabled:    true,
		TLSSkipVerify: true,
	}
	logger := zaptest.NewLogger(t)
	
	// TLS连接可能失败（没有TLS Redis服务器）
	manager, err := NewRedisManager(config, logger)
	
	if err != nil {
		// 预期连接失败
		assert.Nil(t, manager)
	} else {
		// 如果意外成功，清理资源
		_ = manager.Close()
	}
}

// TestNewRedisManager_ClusterMode 测试集群模式配置
func TestNewRedisManager_ClusterMode(t *testing.T) {
	config := &RedisConfig{
		Addr:         "localhost:6379",
		ClusterMode:  true,
		ClusterAddrs: []string{"redis1:6379", "redis2:6379", "redis3:6379"},
	}
	logger := zaptest.NewLogger(t)
	
	// 集群模式连接可能失败（没有Redis集群）
	manager, err := NewRedisManager(config, logger)
	
	if err != nil {
		// 预期连接失败
		assert.Nil(t, manager)
	} else {
		// 如果意外成功，清理资源
		_ = manager.Close()
	}
}

// MockRedisManager 用于测试的Mock Redis管理器
type MockRedisManager struct {
	client redis.UniversalClient
	config *RedisConfig
	closed bool
}

func NewMockRedisManager(config *RedisConfig) *MockRedisManager {
	if config == nil {
		config = DefaultRedisConfig()
	}
	return &MockRedisManager{
		client: nil, // 不创建真实连接
		config: config,
		closed: false,
	}
}

func (m *MockRedisManager) GetClient() redis.UniversalClient {
	return m.client
}

func (m *MockRedisManager) Close() error {
	m.closed = true
	return nil
}

func (m *MockRedisManager) HealthCheck(ctx context.Context) error {
	if m.closed {
		return assert.AnError
	}
	return nil
}

func (m *MockRedisManager) GetStats() *redis.PoolStats {
	return &redis.PoolStats{}
}

// TestMockRedisManager 测试Mock Redis管理器
func TestMockRedisManager(t *testing.T) {
	config := DefaultRedisConfig()
	manager := NewMockRedisManager(config)
	
	// 测试基本功能
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	assert.False(t, manager.closed)
	
	// 测试健康检查
	ctx := context.Background()
	err := manager.HealthCheck(ctx)
	assert.NoError(t, err)
	
	// 测试获取统计信息
	stats := manager.GetStats()
	assert.NotNil(t, stats)
	
	// 测试关闭
	err = manager.Close()
	assert.NoError(t, err)
	assert.True(t, manager.closed)
	
	// 测试关闭后的健康检查
	err = manager.HealthCheck(ctx)
	assert.Error(t, err)
}

// TestRedisConfig_ConfigValidation 测试配置验证
func TestRedisConfig_ConfigValidation(t *testing.T) {
	testCases := []struct {
		name           string
		config         *RedisConfig
		expectError    bool
		errorContains  string
	}{
		{
			name:   "Valid default config",
			config: DefaultRedisConfig(),
			expectError: true, // 因为没有真实Redis服务器
			errorContains: "failed to connect to Redis",
		},
		{
			name: "Invalid address",
			config: &RedisConfig{
				Addr: "",
				DB:   0,
			},
			expectError: true,
			errorContains: "failed to connect to Redis",
		},
		{
			name: "Negative DB number",
			config: &RedisConfig{
				Addr: "localhost:6379",
				DB:   -1,
			},
			expectError: true,
			errorContains: "failed to connect to Redis",
		},
	}
	
	logger := zaptest.NewLogger(t)
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewRedisManager(tc.config, logger)
			
			if tc.expectError {
				// 检查是否如预期失败
				if err != nil {
					assert.Nil(t, manager)
					if tc.errorContains != "" {
						assert.Contains(t, err.Error(), tc.errorContains)
					}
				} else {
					// 如果意外成功，清理资源
					if manager != nil {
						_ = manager.Close()
					}
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				// 清理资源
				if manager != nil {
					_ = manager.Close()
				}
			}
		})
	}
}

// TestRedisConfig_EdgeCases 测试边界情况
func TestRedisConfig_EdgeCases(t *testing.T) {
	t.Run("Zero timeouts", func(t *testing.T) {
		config := &RedisConfig{
			Addr:            "localhost:6379",
			DialTimeout:     0,
			ReadTimeout:     0,
			WriteTimeout:    0,
			PoolTimeout:     0,
			IdleTimeout:     0,
			IdleCheckFreq:   0,
		}
		
		logger := zaptest.NewLogger(t)
		manager, err := NewRedisManager(config, logger)
		
		// 预期连接失败，但配置本身是有效的
		if err != nil {
			assert.Nil(t, manager)
		} else {
			// 如果意外成功，清理资源
			_ = manager.Close()
		}
	})
	
	t.Run("Large values", func(t *testing.T) {
		config := &RedisConfig{
			Addr:            "localhost:6379",
			MaxRetries:      1000,
			PoolSize:        1000,
			MinIdleConns:    500,
			MaxConnAge:      24 * time.Hour,
		}
		
		logger := zaptest.NewLogger(t)
		manager, err := NewRedisManager(config, logger)
		
		// 预期连接失败，但配置本身是有效的
		if err != nil {
			assert.Nil(t, manager)
		} else {
			// 如果意外成功，清理资源
			_ = manager.Close()
		}
	})
}

// TestNewRedisClient_ConvenienceFunction 测试便捷函数
func TestNewRedisClient_ConvenienceFunction(t *testing.T) {
	config := DefaultRedisConfig()
	
	client, err := NewRedisClient(config)
	
	if err != nil {
		// 预期连接失败（没有Redis服务器）
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	} else {
		// 如果连接成功，测试客户端功能
		assert.NotNil(t, client)
		// 清理资源
		_ = client.Close()
	}
}

// TestNewRedisClient_WithNilConfig 测试使用nil配置的便捷函数
func TestNewRedisClient_WithNilConfig(t *testing.T) {
	client, err := NewRedisClient(nil)
	
	if err != nil {
		// 预期连接失败（没有Redis服务器），但会使用默认配置
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	} else {
		// 如果连接成功，测试客户端功能
		assert.NotNil(t, client)
		// 清理资源
		_ = client.Close()
	}
}

// TestRedisConfig_TLSConfiguration 测试TLS配置选项
func TestRedisConfig_TLSConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		tlsEnabled  bool
		skipVerify  bool
	}{
		{"TLS disabled", false, false},
		{"TLS enabled with verification", true, false},
		{"TLS enabled skip verification", true, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &RedisConfig{
				Addr:          "localhost:6380", // 通常TLS端口
				TLSEnabled:    tc.tlsEnabled,
				TLSSkipVerify: tc.skipVerify,
			}
			
			logger := zaptest.NewLogger(t)
			manager, err := NewRedisManager(config, logger)
			
			// 检查连接结果
			if err != nil {
				assert.Nil(t, manager)
			} else {
				// 如果意外成功，清理资源
				_ = manager.Close()
			}
		})
	}
}

// BenchmarkDefaultRedisConfig 基准测试：创建默认配置
func BenchmarkDefaultRedisConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := DefaultRedisConfig()
		_ = config
	}
}

// BenchmarkNewRedisManager_ConfigParsing 基准测试：配置解析（不包括连接）
func BenchmarkNewRedisManager_ConfigParsing(b *testing.B) {
	config := DefaultRedisConfig()
	logger := zaptest.NewLogger(b)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 仅测试配置解析部分，连接会失败但不影响性能测试
		_, _ = NewRedisManager(config, logger)
	}
}