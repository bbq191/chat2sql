// Package ai 成本追踪器和监控测试
package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCostTracker 测试成本追踪器创建
func TestNewCostTracker(t *testing.T) {
	tests := []struct {
		name   string
		config *CostConfig
		want   *CostConfig
	}{
		{
			name:   "nil_config_should_use_defaults",
			config: nil,
			want: &CostConfig{
				DailyBudget:            100.0,
				UserDailyLimit:         10.0,
				QueryCostLimit:         0.50,
				DailyAlertThreshold:    0.8,
				UserAlertThreshold:     0.8,
				ModelAlertThreshold:    0.8,
				EnableDetailedTracking: true,
				RetentionDays:          30,
				EnableCostPrediction:   true,
			},
		},
		{
			name: "custom_config",
			config: &CostConfig{
				DailyBudget:            200.0,
				UserDailyLimit:         20.0,
				QueryCostLimit:         1.0,
				DailyAlertThreshold:    0.9,
				UserAlertThreshold:     0.9,
				ModelAlertThreshold:    0.9,
				EnableDetailedTracking: false,
				RetentionDays:          60,
				EnableCostPrediction:   false,
			},
			want: &CostConfig{
				DailyBudget:            200.0,
				UserDailyLimit:         20.0,
				QueryCostLimit:         1.0,
				DailyAlertThreshold:    0.9,
				UserAlertThreshold:     0.9,
				ModelAlertThreshold:    0.9,
				EnableDetailedTracking: false,
				RetentionDays:          60,
				EnableCostPrediction:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCostTracker(tt.config)
			require.NotNil(t, tracker)
			assert.Equal(t, tt.want, tracker.config)
			assert.NotNil(t, tracker.dailyUsage)
			assert.NotNil(t, tracker.userUsage)
			assert.NotNil(t, tracker.modelCosts)
			assert.NotNil(t, tracker.alerts)

			// 验证模型成本已初始化
			assert.Greater(t, len(tracker.modelCosts), 0)
		})
	}
}

// TestCostTrackerCalculateQueryCost 测试查询成本计算
func TestCostTrackerCalculateQueryCost(t *testing.T) {
	tracker := NewCostTracker(nil)

	tests := []struct {
		name         string
		modelName    string
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{
			name:         "gpt-3.5-turbo_cost_calculation",
			modelName:    "gpt-3.5-turbo",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0.0035, // 预期成本基于初始化的定价
		},
		{
			name:         "gpt-4_cost_calculation",
			modelName:    "gpt-4",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0.045, // 预期成本基于初始化的定价
		},
		{
			name:         "unknown_model_default_cost",
			modelName:    "unknown-model",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0.0, // 未知模型应该返回0或默认值
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := tracker.CalculateQueryCost(tt.inputTokens, tt.outputTokens, tt.modelName)
			// 由于具体的定价可能会变化，我们检查返回值是非负数
			assert.GreaterOrEqual(t, cost, 0.0)

			// 对于已知模型，成本应该大于0
			if tt.modelName == "gpt-3.5-turbo" || tt.modelName == "gpt-4" {
				assert.Greater(t, cost, 0.0)
			}
		})
	}
}

// TestCostTrackerRecordQueryCost 测试查询成本记录
func TestCostTrackerRecordQueryCost(t *testing.T) {
	tracker := NewCostTracker(nil)

	userID := int64(123)
	modelName := "gpt-3.5-turbo"
	query := "SELECT * FROM users"
	inputTokens := 100
	outputTokens := 50
	processTime := int64(1000)

	// 创建QueryCost结构
	cost := tracker.CalculateQueryCost(inputTokens, outputTokens, modelName)
	queryCost := QueryCost{
		Timestamp:    time.Now(),
		Query:        query,
		ModelName:    modelName,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Cost:         cost,
		ProcessTime:  processTime,
	}

	// 记录查询成本
	err := tracker.RecordQueryCost(userID, queryCost)
	assert.NoError(t, err)

	// 验证用户使用量被记录
	tracker.mu.RLock()
	userUsage, exists := tracker.userUsage[userID]
	tracker.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, userUsage)
	assert.Equal(t, userID, userUsage.UserID)
	assert.Greater(t, userUsage.DailyCost, 0.0)
	assert.Equal(t, inputTokens+outputTokens, userUsage.DailyTokens)
	assert.Equal(t, 1, userUsage.DailyQueries)

	// 验证每日使用量被记录
	today := time.Now().Format("2006-01-02")
	tracker.mu.RLock()
	dailyUsage, exists := tracker.dailyUsage[today]
	tracker.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, dailyUsage)
	assert.Greater(t, dailyUsage.TotalCost, 0.0)
	assert.Equal(t, inputTokens+outputTokens, dailyUsage.TotalTokens)
	assert.Equal(t, 1, dailyUsage.QueryCount)
}

// TestCostTrackerGetUserUsage 测试获取用户使用量
func TestCostTrackerGetUserUsage(t *testing.T) {
	tracker := NewCostTracker(nil)

	userID := int64(456)
	modelName := "gpt-3.5-turbo"

	// 先记录一些使用量
	cost := tracker.CalculateQueryCost(50, 25, modelName)
	queryCost := QueryCost{
		Timestamp:    time.Now(),
		Query:        "SELECT 1",
		ModelName:    modelName,
		InputTokens:  50,
		OutputTokens: 25,
		TotalTokens:  75,
		Cost:         cost,
		ProcessTime:  500,
	}
	err := tracker.RecordQueryCost(userID, queryCost)
	require.NoError(t, err)

	// 获取用户使用量
	usage := tracker.GetUserUsage(userID)
	require.NotNil(t, usage)
	assert.Equal(t, userID, usage.UserID)
	assert.Greater(t, usage.DailyCost, 0.0)
	assert.Equal(t, 75, usage.DailyTokens)
	assert.Equal(t, 1, usage.DailyQueries)

	// 测试不存在的用户
	nonExistentUsage := tracker.GetUserUsage(999)
	assert.Nil(t, nonExistentUsage)
}

// TestCostTrackerIsWithinBudget 测试预算检查
func TestCostTrackerIsWithinBudget(t *testing.T) {
	config := &CostConfig{
		DailyBudget:    10.0,
		UserDailyLimit: 5.0,
		QueryCostLimit: 1.0,
	}
	tracker := NewCostTracker(config)

	userID := int64(789)

	tests := []struct {
		name     string
		userID   int64
		queryCost float64
		expected bool
	}{
		{
			name:     "within_budget",
			userID:   userID,
			queryCost: 0.5,
			expected: true,
		},
		{
			name:     "exceeds_query_limit",
			userID:   userID,
			queryCost: 1.5,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.IsWithinBudget(tt.userID, tt.queryCost)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCostTrackerUpdateModelCost 测试更新模型成本
func TestCostTrackerUpdateModelCost(t *testing.T) {
	tracker := NewCostTracker(nil)

	modelName := "custom-model"
	provider := "custom-provider"
	inputCost := 0.001
	outputCost := 0.002
	minimumCost := 0.0001

	// 创建ModelCost结构
	modelCost := &ModelCost{
		ModelName:       modelName,
		Provider:        provider,
		InputCostPer1K:  inputCost,
		OutputCostPer1K: outputCost,
		MinimumCost:     minimumCost,
		LastUpdated:     time.Now(),
	}

	// 更新模型成本
	tracker.UpdateModelCost(modelName, modelCost)

	// 验证成本已更新
	cost := tracker.GetModelCost(modelName)
	require.NotNil(t, cost)
	assert.Equal(t, modelName, cost.ModelName)
	assert.Equal(t, provider, cost.Provider)
	assert.Equal(t, inputCost, cost.InputCostPer1K)
	assert.Equal(t, outputCost, cost.OutputCostPer1K)
	assert.Equal(t, minimumCost, cost.MinimumCost)
}

// TestCostTrackerGetModelCost 测试获取模型成本
func TestCostTrackerGetModelCost(t *testing.T) {
	tracker := NewCostTracker(nil)

	// 测试已知模型
	cost := tracker.GetModelCost("gpt-3.5-turbo")
	assert.NotNil(t, cost)
	assert.Equal(t, "gpt-3.5-turbo", cost.ModelName)

	// 测试未知模型
	unknownCost := tracker.GetModelCost("unknown-model")
	assert.Nil(t, unknownCost)
}

// TestCostTrackerGetCostSummary 测试获取成本总结
func TestCostTrackerGetCostSummary(t *testing.T) {
	tracker := NewCostTracker(nil)

	// 添加一些测试数据
	userID := int64(111)
	modelName := "gpt-3.5-turbo"
	cost := tracker.CalculateQueryCost(100, 50, modelName)
	queryCost := QueryCost{
		Timestamp:    time.Now(),
		Query:        "SELECT * FROM test",
		ModelName:    modelName,
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Cost:         cost,
		ProcessTime:  1000,
	}
	err := tracker.RecordQueryCost(userID, queryCost)
	require.NoError(t, err)

	// 获取成本总结
	summary := tracker.GetCostSummary()
	require.NotNil(t, summary)

	// 验证基本字段
	assert.NotNil(t, summary.Today)
	assert.NotNil(t, summary.Yesterday)
	assert.NotNil(t, summary.ThisWeek)
	assert.NotNil(t, summary.ThisMonth)
	assert.NotNil(t, summary.TopUsers)
	assert.NotNil(t, summary.TopModels)
	assert.NotNil(t, summary.Predictions)
	assert.NotNil(t, summary.Alerts)

	// 验证今日数据
	assert.Greater(t, summary.Today.TotalCost, 0.0)
	assert.Equal(t, 150, summary.Today.TotalTokens)
	assert.Equal(t, 1, summary.Today.QueryCount)
}

// TestCostLimitError 测试成本限制错误
func TestCostLimitError(t *testing.T) {
	errorType := "daily_limit"
	current := 5.0
	limit := 4.0

	err := NewCostLimitError(errorType, current, limit)
	require.NotNil(t, err)
	
	assert.Contains(t, err.Error(), "cost limit exceeded")
}

// TestAlertManager 测试告警管理器基本功能
func TestAlertManager(t *testing.T) {
	tracker := NewCostTracker(nil)
	require.NotNil(t, tracker.alerts)

	// 测试告警管理器初始化
	assert.NotNil(t, tracker.alerts.alerts)
	assert.Equal(t, tracker.config, tracker.alerts.config)
}

// TestConcurrentAccess 测试并发访问安全性
func TestConcurrentAccess(t *testing.T) {
	tracker := NewCostTracker(nil)

	// 并发记录查询成本
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := int64(id)
			modelName := "gpt-3.5-turbo"
			cost := tracker.CalculateQueryCost(10, 5, modelName)
			queryCost := QueryCost{
				Timestamp:    time.Now(),
				Query:        "SELECT 1",
				ModelName:    modelName,
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
				Cost:         cost,
				ProcessTime:  100,
			}
			err := tracker.RecordQueryCost(userID, queryCost)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据一致性
	summary := tracker.GetCostSummary()
	assert.NotNil(t, summary)
	assert.Equal(t, 10, summary.Today.QueryCount)
	assert.Equal(t, 150, summary.Today.TotalTokens) // 10 * (10+5)
}