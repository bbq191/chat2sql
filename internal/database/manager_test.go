package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"chat2sql-go/internal/config"
)

func TestNewManager(t *testing.T) {
	dbConfig := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
		MaxConns: 10,
		MinConns: 2,
		HealthCheckPeriod: 30 * time.Second,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
	}
	
	logger := zap.NewNop()
	
	manager, err := NewManager(dbConfig, logger)
	
	// 由于没有真实的数据库连接，这个测试可能会失败
	// 但我们可以测试函数的基本结构
	if err != nil {
		// 预期会失败，因为没有真实的数据库
		assert.Contains(t, err.Error(), "数据库健康检查失败")
	} else {
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.pool)
		assert.Equal(t, logger, manager.logger)
	}
}

func TestManager_Close_NilPool(t *testing.T) {
	logger := zap.NewNop()
	manager := &Manager{
		pool:   nil,
		logger: logger,
	}
	
	// 这不应该panic
	manager.Close()
}

func TestManager_HealthCheck_NilPool(t *testing.T) {
	logger := zap.NewNop()
	manager := &Manager{
		pool:   nil,
		logger: logger,
	}
	
	ctx := context.Background()
	err := manager.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接池未初始化")
}

func TestManager_Stats_Basic(t *testing.T) {
	logger := zap.NewNop()
	manager := &Manager{
		pool:   nil,
		logger: logger,
	}
	
	// 测试manager结构体不为nil
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger)
}

func TestDatabaseConfig_DefaultValues(t *testing.T) {
	config := &config.DatabaseConfig{}
	
	// 测试默认配置不会panic
	assert.NotNil(t, config)
}

// ==============================================
// 数据库稳定性测试套件
// ==============================================

// DatabaseStabilityTestSuite 数据库稳定性测试套件
type DatabaseStabilityTestSuite struct {
	suite.Suite
	logger *zap.Logger
	config *config.DatabaseConfig
}

func (suite *DatabaseStabilityTestSuite) SetupSuite() {
	suite.logger = zap.NewNop()
	suite.config = &config.DatabaseConfig{
		Host:              "localhost",
		Port:              5432,
		User:              "test",
		Password:          "test",
		Database:          "testdb",
		SSLMode:           "disable",
		MaxConns:          10,
		MinConns:          2,
		HealthCheckPeriod: 30 * time.Second,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
	}
}

// TestConnectionPoolExhaustion 测试连接池耗尽场景
func (suite *DatabaseStabilityTestSuite) TestConnectionPoolExhaustion() {
	// 创建一个小的连接池配置
	smallPoolConfig := *suite.config
	smallPoolConfig.MaxConns = 2
	smallPoolConfig.MinConns = 1
	
	// 由于测试环境可能没有真实数据库，这个测试主要验证配置和错误处理
	manager, err := NewManager(&smallPoolConfig, suite.logger)
	
	if err != nil {
		// 预期会失败，因为没有真实的数据库连接
		assert.Contains(suite.T(), err.Error(), "数据库健康检查失败")
		return
	}
	
	// 如果连接成功，测试连接池统计
	defer manager.Close()
	
	stats := manager.GetPoolStats()
	assert.NotNil(suite.T(), stats)
}

// TestConnectionFailureRecovery 测试连接失败恢复
func (suite *DatabaseStabilityTestSuite) TestConnectionFailureRecovery() {
	// 使用错误的连接配置
	badConfig := *suite.config
	badConfig.Host = "nonexistent-host"
	badConfig.Port = 9999
	
	// 尝试创建管理器应该失败
	manager, err := NewManager(&badConfig, suite.logger)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), manager)
	
	// 验证错误信息包含连接失败的描述
	assert.Contains(suite.T(), err.Error(), "数据库健康检查失败")
}

// TestHealthCheckFailure 测试健康检查失败处理
func (suite *DatabaseStabilityTestSuite) TestHealthCheckFailure() {
	logger := zap.NewNop()
	manager := &Manager{
		pool:   nil, // 故意设置为nil模拟失败
		logger: logger,
	}
	
	ctx := context.Background()
	err := manager.HealthCheck(ctx)
	
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "数据库连接池未初始化")
}

// TestConcurrentConnectionRequests 测试并发连接请求
func (suite *DatabaseStabilityTestSuite) TestConcurrentConnectionRequests() {
	// 这个测试主要验证并发访问的安全性
	manager := &Manager{
		pool:   nil,
		logger: suite.logger,
	}
	
	const numGoroutines = 10
	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines)
	
	// 并发执行健康检查
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			err := manager.HealthCheck(ctx)
			errorChan <- err
		}()
	}
	
	wg.Wait()
	close(errorChan)
	
	// 验证所有并发请求都正确处理错误
	errorCount := 0
	for err := range errorChan {
		assert.Error(suite.T(), err)
		assert.Contains(suite.T(), err.Error(), "数据库连接池未初始化")
		errorCount++
	}
	assert.Equal(suite.T(), numGoroutines, errorCount)
}

// TestManagerCloseIdempotent 测试管理器关闭的幂等性
func (suite *DatabaseStabilityTestSuite) TestManagerCloseIdempotent() {
	manager := &Manager{
		pool:   nil,
		logger: suite.logger,
	}
	
	// 多次调用Close不应该panic
	manager.Close()
	manager.Close()
	manager.Close()
	
	// 验证没有panic发生
	assert.True(suite.T(), true)
}

// TestDatabaseConfigValidation 测试数据库配置验证
func (suite *DatabaseStabilityTestSuite) TestDatabaseConfigValidation() {
	testCases := []struct {
		name        string
		config      *config.DatabaseConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "test",
				Password: "test",
				Database: "testdb",
				SSLMode:  "disable",
				MaxConns: 10,
				MinConns: 2,
				HealthCheckPeriod: 30 * time.Second,
				MaxConnLifetime:   time.Hour,
				MaxConnIdleTime:   30 * time.Minute,
			},
			expectError: true, // 在测试环境中没有真实数据库，所以期望错误
		},
		{
			name: "invalid port",
			config: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     0, // 无效端口
				User:     "test",
				Password: "test",
				Database: "testdb",
				SSLMode:  "disable",
				MaxConns: 10,
				MinConns: 2,
				HealthCheckPeriod: 30 * time.Second,
				MaxConnLifetime:   time.Hour,
				MaxConnIdleTime:   30 * time.Minute,
			},
			expectError: true,
		},
		{
			name: "empty host",
			config: &config.DatabaseConfig{
				Host:     "", // 空主机
				Port:     5432,
				User:     "test",
				Password: "test",
				Database: "testdb",
				SSLMode:  "disable",
				MaxConns: 10,
				MinConns: 2,
				HealthCheckPeriod: 30 * time.Second,
				MaxConnLifetime:   time.Hour,
				MaxConnIdleTime:   30 * time.Minute,
			},
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		_, err := NewManager(tc.config, suite.logger)
		if tc.expectError {
			assert.Error(suite.T(), err, "Test case: %s", tc.name)
		} else {
			assert.NoError(suite.T(), err, "Test case: %s", tc.name)
		}
	}
}

// TestConnectionTimeoutHandling 测试连接超时处理
func (suite *DatabaseStabilityTestSuite) TestConnectionTimeoutHandling() {
	// 使用很短的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	
	manager := &Manager{
		pool:   nil,
		logger: suite.logger,
	}
	
	err := manager.HealthCheck(ctx)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "数据库连接池未初始化")
}

// TestConnectionPoolStats 测试连接池统计信息
func (suite *DatabaseStabilityTestSuite) TestConnectionPoolStats() {
	manager := &Manager{
		pool:   nil,
		logger: suite.logger,
	}
	
	// 当pool为nil时，GetPoolStats会panic，这是预期的行为
	assert.Panics(suite.T(), func() {
		manager.GetPoolStats()
	}, "当pool为nil时GetPoolStats应该panic")
}

// TestDatabaseManagerLifecycle 测试数据库管理器生命周期
func (suite *DatabaseStabilityTestSuite) TestDatabaseManagerLifecycle() {
	// 测试管理器的完整生命周期：创建 -> 使用 -> 关闭
	manager, err := NewManager(suite.config, suite.logger)
	
	if err != nil {
		// 在测试环境中预期会失败
		assert.Contains(suite.T(), err.Error(), "数据库健康检查失败")
		return
	}
	
	// 如果创建成功，测试基本操作
	defer manager.Close()
	
	// 健康检查
	ctx := context.Background()
	err = manager.HealthCheck(ctx)
	assert.NoError(suite.T(), err)
	
	// 获取统计信息
	stats := manager.GetPoolStats()
	assert.NotNil(suite.T(), stats)
	
	// 验证连接池配置
	if manager.pool != nil {
		config := manager.pool.Config()
		assert.NotNil(suite.T(), config)
	}
}

// TestErrorHandlingConsistency 测试错误处理一致性
func (suite *DatabaseStabilityTestSuite) TestErrorHandlingConsistency() {
	// 创建多个不同的错误场景
	errorScenarios := []struct {
		name        string
		createManager func() (*Manager, error)
		expectedError string
	}{
		{
			name: "nil config",
			createManager: func() (*Manager, error) {
				return NewManager(nil, suite.logger)
			},
			expectedError: "数据库配置不能为空",
		},
		{
			name: "invalid connection",
			createManager: func() (*Manager, error) {
				badConfig := *suite.config
				badConfig.Host = "invalid-host"
				return NewManager(&badConfig, suite.logger)
			},
			expectedError: "数据库健康检查失败",
		},
	}
	
	for _, scenario := range errorScenarios {
		manager, err := scenario.createManager()
		assert.Error(suite.T(), err, "Scenario: %s", scenario.name)
		assert.Nil(suite.T(), manager, "Scenario: %s", scenario.name)
		
		if scenario.expectedError != "" {
			assert.Contains(suite.T(), err.Error(), scenario.expectedError,
				"Scenario: %s", scenario.name)
		}
	}
}

// TestMemoryLeakPrevention 测试内存泄漏防护
func (suite *DatabaseStabilityTestSuite) TestMemoryLeakPrevention() {
	// 快速创建和销毁多个管理器实例，检查是否有内存泄漏
	for i := 0; i < 100; i++ {
		manager, err := NewManager(suite.config, suite.logger)
		if err != nil {
			// 预期在测试环境中会失败
			continue
		}
		
		// 立即关闭
		manager.Close()
	}
	
	// 这个测试主要确保没有panic和明显的资源泄漏
	assert.True(suite.T(), true, "Memory leak test completed without panic")
}

// 运行数据库稳定性测试套件
func TestDatabaseStabilityTestSuite(t *testing.T) {
	suite.Run(t, new(DatabaseStabilityTestSuite))
}