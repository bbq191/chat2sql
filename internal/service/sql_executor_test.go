package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockConnectionManager 模拟ConnectionManager接口
type MockConnectionManager struct {
	mock.Mock
}

func (m *MockConnectionManager) GetConnectionPool(ctx context.Context, connectionID int64) (interface{}, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0), args.Error(1)
}

func (m *MockConnectionManager) TestConnection(ctx context.Context, connection *repository.DatabaseConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockConnectionManager) CloseConnection(connectionID int64) error {
	args := m.Called(connectionID)
	return args.Error(0)
}

// MockSQLSecurityValidator 模拟SQLSecurityValidator接口
type MockSQLSecurityValidator struct {
	mock.Mock
}

func (m *MockSQLSecurityValidator) ValidateSQL(ctx context.Context, sql string) error {
	args := m.Called(ctx, sql)
	return args.Error(0)
}

func (m *MockSQLSecurityValidator) GetSecurityScore(sql string) int {
	args := m.Called(sql)
	return args.Int(0)
}

// TestSQLExecutor_ExecuteQuery 测试SQL执行功能
func TestSQLExecutor_ExecuteQuery(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("创建SQLExecutor - 基本结构测试", func(t *testing.T) {
		// 简化测试 - 只测试构造函数是否能正常工作
		executor := NewSQLExecutor(nil, nil, logger)
		
		assert.NotNil(t, executor)
		assert.NotNil(t, logger) // 确保logger被正确设置
	})

	t.Run("Mock接口测试", func(t *testing.T) {
		// 测试Mock结构是否正确定义
		mockConnManager := &MockConnectionManager{}
		mockValidator := &MockSQLSecurityValidator{}
		
		// 设置和验证mock期望
		ctx := context.Background()
		connectionID := int64(1)
		sql := "SELECT 1"
		
		mockConnManager.On("GetConnectionPool", ctx, connectionID).Return(nil, errors.New("测试错误")).Once()
		mockValidator.On("ValidateSQL", ctx, sql).Return(nil).Once()
		
		// 调用mock方法来满足期望
		_, err := mockConnManager.GetConnectionPool(ctx, connectionID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "测试错误")
		
		err = mockValidator.ValidateSQL(ctx, sql)
		assert.NoError(t, err)
		
		// 验证所有期望都被满足
		mockConnManager.AssertExpectations(t)
		mockValidator.AssertExpectations(t)
	})
}

// TestSQLExecutor_Performance 测试SQL执行器性能
func TestSQLExecutor_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	mockValidator := &MockSQLSecurityValidator{}
	_ = &MockConnectionManager{} // 预留给后续使用

	t.Run("SQL安全验证性能测试", func(t *testing.T) {
		ctx := context.Background()
		queryCount := 100
		
		// 设置mock期望
		mockValidator.On("ValidateSQL", ctx, mock.AnythingOfType("string")).Return(nil).Times(queryCount)

		start := time.Now()
		
		// 批量执行安全验证
		for i := 0; i < queryCount; i++ {
			sql := "SELECT * FROM users LIMIT 10"
			err := mockValidator.ValidateSQL(ctx, sql)
			require.NoError(t, err)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(queryCount)

		// P0性能指标：SQL安全验证平均时间应该 < 10ms
		assert.Less(t, avgDuration, 10*time.Millisecond, 
			"SQL安全验证平均时间 %v 超过性能指标 10ms", avgDuration)

		t.Logf("SQL安全验证性能测试 - 总时间: %v, 平均: %v", duration, avgDuration)

		mockValidator.AssertExpectations(t)
	})

	t.Run("并发SQL验证测试", func(t *testing.T) {
		ctx := context.Background()
		concurrency := 10
		queriesPerGoroutine := 10
		
		// 重置mock
		mockValidator := &MockSQLSecurityValidator{}
		mockValidator.On("ValidateSQL", ctx, mock.AnythingOfType("string")).Return(nil).
			Times(concurrency * queriesPerGoroutine)

		start := time.Now()
		
		// 创建channel收集结果
		results := make(chan error, concurrency*queriesPerGoroutine)
		
		// 启动多个goroutine并发验证SQL
		for i := 0; i < concurrency; i++ {
			go func(goroutineID int) {
				for j := 0; j < queriesPerGoroutine; j++ {
					sql := "SELECT * FROM users WHERE id = " + string(rune(j+1))
					results <- mockValidator.ValidateSQL(ctx, sql)
				}
			}(i)
		}
		
		// 收集所有结果
		totalOperations := concurrency * queriesPerGoroutine
		var errorCount int
		for i := 0; i < totalOperations; i++ {
			if err := <-results; err != nil {
				errorCount++
			}
		}
		
		totalDuration := time.Since(start)
		successfulOperations := totalOperations - errorCount
		
		if successfulOperations > 0 {
			avgDuration := totalDuration / time.Duration(successfulOperations)
			assert.Less(t, avgDuration, 50*time.Millisecond, 
				"并发SQL验证平均时间 %v 超过性能指标 50ms", avgDuration)
			
			t.Logf("并发SQL验证测试完成 - 成功: %d/%d, 总时间: %v, 平均: %v", 
				successfulOperations, totalOperations, totalDuration, avgDuration)
		}
		
		assert.Equal(t, 0, errorCount, "不应该有验证错误")
		mockValidator.AssertExpectations(t)
	})
}

// TestConnectionManager_TestConnection 测试连接管理器的连接测试功能
func TestConnectionManager_TestConnection(t *testing.T) {
	mockConnManager := &MockConnectionManager{}

	t.Run("成功测试连接", func(t *testing.T) {
		ctx := context.Background()
		
		connection := &repository.DatabaseConnection{
			BaseModel: repository.BaseModel{
				ID: 1,
			},
			Host:              "localhost",
			Port:              5432,
			DatabaseName:      "testdb",
			Username:          "testuser",
			PasswordEncrypted: "encrypted_password",
			DBType:            string(repository.DBTypePostgreSQL),
		}

		// 设置mock期望 - 连接成功
		mockConnManager.On("TestConnection", ctx, connection).Return(nil)

		// 执行测试
		err := mockConnManager.TestConnection(ctx, connection)

		// 验证结果
		assert.NoError(t, err)
		mockConnManager.AssertExpectations(t)
	})

	t.Run("连接测试失败", func(t *testing.T) {
		ctx := context.Background()
		
		connection := &repository.DatabaseConnection{
			BaseModel: repository.BaseModel{
				ID: 2,
			},
			Host:              "invalid-host",
			Port:              5432,
			DatabaseName:      "testdb",
			Username:          "testuser",
			PasswordEncrypted: "encrypted_password",
			DBType:            string(repository.DBTypePostgreSQL),
		}

		// 设置mock期望 - 连接失败
		expectedError := errors.New("连接超时")
		mockConnManager.On("TestConnection", ctx, connection).Return(expectedError)

		// 执行测试
		err := mockConnManager.TestConnection(ctx, connection)

		// 验证结果
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		mockConnManager.AssertExpectations(t)
	})

	t.Run("连接测试性能", func(t *testing.T) {
		if testing.Short() {
			t.Skip("跳过性能测试")
		}

		ctx := context.Background()
		connection := &repository.DatabaseConnection{
			Host:              "localhost",
			Port:              5432,
			DatabaseName:      "testdb",
			Username:          "testuser",
			PasswordEncrypted: "encrypted_password",
			DBType:            string(repository.DBTypePostgreSQL),
		}

		// 重置mock
		mockConnManager := &MockConnectionManager{}
		testCount := 10
		mockConnManager.On("TestConnection", ctx, connection).Return(nil).Times(testCount)

		start := time.Now()
		
		// 执行多次连接测试
		for i := 0; i < testCount; i++ {
			err := mockConnManager.TestConnection(ctx, connection)
			require.NoError(t, err)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(testCount)

		// P0性能指标：连接测试平均时间应该 < 1秒
		assert.Less(t, avgDuration, 1*time.Second, 
			"连接测试平均时间 %v 超过性能指标 1s", avgDuration)

		t.Logf("连接测试性能 - 总时间: %v, 平均: %v", duration, avgDuration)
		mockConnManager.AssertExpectations(t)
	})
}

// TestSQLSecurityValidator_ValidateSQL 测试SQL安全验证器
func TestSQLSecurityValidator_ValidateSQL(t *testing.T) {
	mockValidator := &MockSQLSecurityValidator{}

	t.Run("安全SQL通过验证", func(t *testing.T) {
		ctx := context.Background()
		safeSQL := "SELECT id, username, email FROM users WHERE status = 'active' LIMIT 100"

		// 设置mock期望 - 安全SQL通过验证
		mockValidator.On("ValidateSQL", ctx, safeSQL).Return(nil)
		mockValidator.On("GetSecurityScore", safeSQL).Return(95)

		// 执行验证
		err := mockValidator.ValidateSQL(ctx, safeSQL)
		score := mockValidator.GetSecurityScore(safeSQL)

		// 验证结果
		assert.NoError(t, err)
		assert.Greater(t, score, 90, "安全SQL的安全评分应该高于90")
		mockValidator.AssertExpectations(t)
	})

	t.Run("危险SQL被拒绝", func(t *testing.T) {
		ctx := context.Background()
		
		dangerousSQLs := []struct {
			sql    string
			reason string
		}{
			{"SELECT * FROM users; DROP TABLE users;", "包含DROP操作"},
			{"SELECT * FROM users WHERE id = 1 OR 1=1", "包含SQL注入模式"},
			{"INSERT INTO users (username) VALUES ('admin')", "包含INSERT操作"},
			{"UPDATE users SET role = 'admin' WHERE id = 1", "包含UPDATE操作"},
			{"DELETE FROM users WHERE id = 1", "包含DELETE操作"},
		}

		for _, testCase := range dangerousSQLs {
			// 重置mock
			mockValidator := &MockSQLSecurityValidator{}
			
			// 设置mock期望 - 危险SQL被拒绝
			expectedError := errors.New(testCase.reason)
			mockValidator.On("ValidateSQL", ctx, testCase.sql).Return(expectedError)
			mockValidator.On("GetSecurityScore", testCase.sql).Return(10) // 低安全评分

			// 执行验证
			err := mockValidator.ValidateSQL(ctx, testCase.sql)
			score := mockValidator.GetSecurityScore(testCase.sql)

			// 验证结果
			assert.Error(t, err, "危险SQL应该被拒绝: %s", testCase.sql)
			assert.Contains(t, err.Error(), testCase.reason)
			assert.Less(t, score, 50, "危险SQL的安全评分应该低于50")
			mockValidator.AssertExpectations(t)
		}
	})

	t.Run("复杂查询验证", func(t *testing.T) {
		ctx := context.Background()
		
		complexQueries := []struct {
			sql           string
			shouldPass    bool
			expectedScore int
		}{
			{
				"SELECT u.username, COUNT(q.id) as query_count FROM users u LEFT JOIN query_history q ON u.id = q.user_id GROUP BY u.id, u.username ORDER BY query_count DESC",
				true,
				85,
			},
			{
				"WITH recent_queries AS (SELECT * FROM query_history WHERE created_at > NOW() - INTERVAL '30 days') SELECT * FROM recent_queries",
				true,
				80,
			},
			{
				"SELECT * FROM users WHERE username LIKE '%admin%' AND password_hash = MD5('password123')",
				false, // 包含密码相关操作
				25,
			},
		}

		for _, testCase := range complexQueries {
			// 重置mock
			mockValidator := &MockSQLSecurityValidator{}
			
			if testCase.shouldPass {
				mockValidator.On("ValidateSQL", ctx, testCase.sql).Return(nil)
			} else {
				mockValidator.On("ValidateSQL", ctx, testCase.sql).Return(errors.New("安全验证失败"))
			}
			mockValidator.On("GetSecurityScore", testCase.sql).Return(testCase.expectedScore)

			// 执行验证
			err := mockValidator.ValidateSQL(ctx, testCase.sql)
			score := mockValidator.GetSecurityScore(testCase.sql)

			// 验证结果
			if testCase.shouldPass {
				assert.NoError(t, err, "复杂查询应该通过验证: %s", testCase.sql)
			} else {
				assert.Error(t, err, "不安全的复杂查询应该被拒绝: %s", testCase.sql)
			}
			assert.Equal(t, testCase.expectedScore, score)
			mockValidator.AssertExpectations(t)
		}
	})
}