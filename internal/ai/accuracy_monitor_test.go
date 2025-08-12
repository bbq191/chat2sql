// 准确率监控系统测试 - 完整测试覆盖AI查询质量跟踪与优化
// 测试监控、反馈收集、趋势分析、告警等核心功能

package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewAccuracyMonitor 测试准确率监控器创建
func TestNewAccuracyMonitor(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	
	monitor := NewAccuracyMonitor(config, logger)
	
	assert.NotNil(t, monitor)
	assert.Equal(t, config, monitor.config)
	assert.Equal(t, logger, monitor.logger)
	assert.NotNil(t, monitor.feedbackStore)
	assert.NotNil(t, monitor.metrics)
	assert.NotNil(t, monitor.alertManager)
	assert.NotNil(t, monitor.dailyStats)
	assert.NotNil(t, monitor.userStats)
	assert.NotNil(t, monitor.modelStats)
	assert.NotNil(t, monitor.categoryStats)
	assert.NotNil(t, monitor.realtimeMetrics)
	assert.NotNil(t, monitor.trendAnalyzer)
}

// TestDefaultAccuracyConfig 测试默认准确率配置
func TestDefaultAccuracyConfig(t *testing.T) {
	config := DefaultAccuracyConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, 0.70, config.MinAccuracyThreshold)
	assert.Equal(t, 0.85, config.DailyAccuracyTarget)
	assert.Equal(t, 0.90, config.WeeklyAccuracyTarget)
	assert.Equal(t, 30*time.Minute, config.AlertCooldown)
	assert.Equal(t, 90, config.DataRetentionDays)
	assert.Equal(t, 1000, config.SampleSize)
	assert.Equal(t, 10, config.FeedbackRequiredPercent)
	assert.True(t, config.EnableMLAnalysis)
}

// TestQueryFeedback 测试查询反馈结构
func TestQueryFeedback(t *testing.T) {
	now := time.Now()
	feedback := QueryFeedback{
		QueryID:        "test-query-1",
		UserID:         123,
		UserQuery:      "SELECT * FROM users",
		GeneratedSQL:   "SELECT id, name, email FROM users",
		IsCorrect:      true,
		UserRating:     4,
		Feedback:       "Good result",
		Category:       CategoryBasicSelect,
		Difficulty:     DifficultyEasy,
		ProcessingTime: 500 * time.Millisecond,
		TokensUsed:     100,
		ModelUsed:      "gpt-4o-mini",
		Timestamp:      now,
	}

	assert.Equal(t, "test-query-1", feedback.QueryID)
	assert.Equal(t, int64(123), feedback.UserID)
	assert.Equal(t, "SELECT * FROM users", feedback.UserQuery)
	assert.Equal(t, "SELECT id, name, email FROM users", feedback.GeneratedSQL)
	assert.True(t, feedback.IsCorrect)
	assert.Equal(t, 4, feedback.UserRating)
	assert.Equal(t, "Good result", feedback.Feedback)
	assert.Equal(t, CategoryBasicSelect, feedback.Category)
	assert.Equal(t, DifficultyEasy, feedback.Difficulty)
	assert.Equal(t, now, feedback.Timestamp)
	assert.Equal(t, 500*time.Millisecond, feedback.ProcessingTime)
	assert.Equal(t, 100, feedback.TokensUsed)
	assert.Equal(t, "gpt-4o-mini", feedback.ModelUsed)
}

// TestQueryCategory 测试查询类别枚举
func TestQueryCategory(t *testing.T) {
	assert.Equal(t, "basic_select", string(CategoryBasicSelect))
	assert.Equal(t, "join_query", string(CategoryJoinQuery))
	assert.Equal(t, "aggregation", string(CategoryAggregation))
	assert.Equal(t, "subquery", string(CategorySubquery))
	assert.Equal(t, "time_analysis", string(CategoryTimeAnalysis))
	assert.Equal(t, "complex_query", string(CategoryComplexQuery))
}

// TestQueryDifficulty 测试查询难度枚举
func TestQueryDifficulty(t *testing.T) {
	assert.Equal(t, "easy", string(DifficultyEasy))
	assert.Equal(t, "medium", string(DifficultyMedium))
	assert.Equal(t, "hard", string(DifficultyHard))
	assert.Equal(t, "expert", string(DifficultyExpert))
}

// TestRecordFeedback 测试记录反馈功能
func TestRecordFeedback(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	feedback := QueryFeedback{
		QueryID:        "test-query-1",
		UserID:         123,
		UserQuery:      "SELECT * FROM users",
		GeneratedSQL:   "SELECT id, name, email FROM users",
		IsCorrect:      true,
		UserRating:     5,
		Category:       CategoryBasicSelect,
		Difficulty:     DifficultyEasy,
		ProcessingTime: 300 * time.Millisecond,
		TokensUsed:     50,
		ModelUsed:      "gpt-4o-mini",
		Timestamp:      time.Now(),
	}

	err := monitor.RecordFeedback(feedback)
	assert.NoError(t, err)

	// 验证反馈是否被存储
	storedFeedback, exists := monitor.feedbackStore[feedback.QueryID]
	assert.True(t, exists)
	assert.Equal(t, feedback.QueryID, storedFeedback.QueryID)
	assert.Equal(t, feedback.IsCorrect, storedFeedback.IsCorrect)
}

// TestCategorizeQuery 测试查询分类功能
func TestCategorizeQuery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	tests := []struct {
		name     string
		query    string
		expected QueryCategory
	}{
		{
			name:     "simple_select",
			query:    "SELECT * FROM users",
			expected: CategoryBasicSelect,
		},
		{
			name:     "aggregation_query",
			query:    "SELECT COUNT(*) FROM orders",
			expected: CategoryAggregation,
		},
		{
			name:     "join_query",
			query:    "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id",
			expected: CategoryJoinQuery,
		},
		{
			name:     "time_analysis",
			query:    "SELECT DATE(created_at), COUNT(*) FROM orders WHERE created_at >= '2024-01-01' GROUP BY DATE(created_at)",
			expected: CategoryTimeAnalysis,
		},
		{
			name:     "subquery",
			query:    "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)",
			expected: CategorySubquery,
		},
		{
			name:     "complex_query",
			query:    "WITH user_stats AS (SELECT user_id, COUNT(*) as order_count FROM orders GROUP BY user_id) SELECT u.name, us.order_count FROM users u JOIN user_stats us ON u.id = us.user_id",
			expected: CategoryComplexQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := monitor.categorizeQuery(tt.query)
			assert.Equal(t, tt.expected, category)
		})
	}
}

// TestAssessDifficulty 测试难度评估功能
func TestAssessDifficulty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	tests := []struct {
		name     string
		query    string
		sql      string
		expected QueryDifficulty
	}{
		{
			name:     "easy_select",
			query:    "获取所有用户",
			sql:      "SELECT * FROM users",
			expected: DifficultyEasy,
		},
		{
			name:     "medium_join",
			query:    "获取用户和订单信息",
			sql:      "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id",
			expected: DifficultyMedium,
		},
		{
			name:     "hard_complex",
			query:    "获取每个用户的订单统计",
			sql:      "SELECT u.name, COUNT(o.id) as order_count, SUM(o.total) as total_spent FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id, u.name HAVING COUNT(o.id) > 5",
			expected: DifficultyHard,
		},
		{
			name:     "expert_subquery",
			query:    "获取高于平均订单金额的订单",
			sql:      "SELECT * FROM orders WHERE total > (SELECT AVG(total) FROM orders) AND user_id IN (SELECT id FROM users WHERE created_at > '2024-01-01')",
			expected: DifficultyExpert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			difficulty := monitor.assessDifficulty(tt.query, tt.sql)
			assert.Equal(t, tt.expected, difficulty)
		})
	}
}

// TestGetCurrentAccuracy 测试获取当前准确率
func TestGetCurrentAccuracy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	// 初始准确率应该为0%（没有反馈时的默认值）
	accuracy := monitor.getCurrentAccuracy()
	assert.Equal(t, 0.0, accuracy)

	// 添加一些反馈
	feedbacks := []QueryFeedback{
		{QueryID: "1", IsCorrect: true, Timestamp: time.Now()},
		{QueryID: "2", IsCorrect: true, Timestamp: time.Now()},
		{QueryID: "3", IsCorrect: false, Timestamp: time.Now()},
		{QueryID: "4", IsCorrect: true, Timestamp: time.Now()},
	}

	for _, feedback := range feedbacks {
		monitor.RecordFeedback(feedback)
	}

	// 更新准确率计算
	accuracy = monitor.getCurrentAccuracy()
	assert.Greater(t, accuracy, 0.0)
	assert.LessOrEqual(t, accuracy, 100.0)
}

// TestAccuracyGetMetrics 测试获取准确率指标
func TestAccuracyGetMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	metrics := monitor.GetMetrics()
	assert.NotNil(t, metrics)

	// 检查基本指标字段
	assert.Contains(t, metrics, "current_accuracy")
	assert.Contains(t, metrics, "total_queries")
	assert.Contains(t, metrics, "correct_queries")
	assert.Contains(t, metrics, "error_rate")
	assert.Contains(t, metrics, "avg_confidence")
	assert.Contains(t, metrics, "avg_response_time")
	assert.Contains(t, metrics, "category_breakdown")
	assert.Contains(t, metrics, "daily_accuracy")
	assert.Contains(t, metrics, "queries_per_minute")
}

// TestTrendAnalyzer 测试趋势分析器
func TestTrendAnalyzer(t *testing.T) {
	analyzer := &TrendAnalyzer{
		hourlyData: make([]float64, 0),
		dailyData:  make([]float64, 0),
		weeklyData: make([]float64, 0),
		trends:     make(map[string][]float64),
	}

	// 添加数据点
	testPoints := []float64{85.0, 87.0, 90.0, 88.0, 92.0}
	for _, point := range testPoints {
		analyzer.AddDataPoint(point)
	}

	// 验证数据被添加到hourlyData
	assert.GreaterOrEqual(t, len(analyzer.hourlyData), 1)
}

// TestAlertTypes 测试告警类型枚举
func TestAlertTypes(t *testing.T) {
	assert.Equal(t, "low_accuracy", string(AlertTypeLowAccuracy))
	assert.Equal(t, "error_spike", string(AlertTypeErrorSpike))
	assert.Equal(t, "model_degraded", string(AlertTypeModelDegraded))
	assert.Equal(t, "user_complaint", string(AlertTypeUserComplaint))
	assert.Equal(t, "trend_negative", string(AlertTypeTrendNegative))
}

// TestAlertLevels 测试告警级别枚举
func TestAlertLevels(t *testing.T) {
	assert.Equal(t, "info", string(AlertLevelInfo))
	assert.Equal(t, "warning", string(AlertLevelWarning))
	assert.Equal(t, "critical", string(AlertLevelCritical))
}

// TestAccuracyReport 测试准确率报告生成
func TestAccuracyReport(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	ctx := context.Background()
	
	// 由于没有真实数据，这个测试主要验证结构和错误处理
	report, err := monitor.GetAccuracyReport(ctx, 7)
	
	// 应该能生成报告，即使没有数据
	assert.NoError(t, err)
	assert.NotNil(t, report, "报告本身不应该为nil")
	
	// 逐个检查字段，提供详细错误信息
	if report != nil {
		assert.NotNil(t, report.OverallStats, "OverallStats不应该为nil")
		assert.NotNil(t, report.DailyStats, "DailyStats不应该为nil")
		assert.NotNil(t, report.CategoryStats, "CategoryStats不应该为nil")
		assert.NotNil(t, report.ModelStats, "ModelStats不应该为nil")
		assert.NotNil(t, report.Trends, "Trends不应该为nil")
		assert.NotNil(t, report.Recommendations, "Recommendations不应该为nil")
	}
}

// TestErrorPatternDetection 测试错误模式检测
func TestErrorPatternDetection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	// 添加一些错误反馈来测试模式检测
	errorFeedbacks := []QueryFeedback{
		{
			QueryID:      "error-1",
			UserQuery:    "SELECT * FROM non_existent_table",
			GeneratedSQL: "SELECT * FROM non_existent_table",
			IsCorrect:    false,
			ErrorType:    "TABLE_NOT_FOUND",
			ErrorDetails: "Table does not exist",
			Timestamp:    time.Now(),
		},
		{
			QueryID:      "error-2",
			UserQuery:    "SELECT invalid_column FROM users",
			GeneratedSQL: "SELECT invalid_column FROM users",
			IsCorrect:    false,
			ErrorType:    "COLUMN_NOT_FOUND",
			ErrorDetails: "Column does not exist",
			Timestamp:    time.Now(),
		},
	}

	for _, feedback := range errorFeedbacks {
		err := monitor.RecordFeedback(feedback)
		assert.NoError(t, err)
	}

	// 验证错误被记录
	assert.Len(t, monitor.feedbackStore, 2)
	for _, feedback := range errorFeedbacks {
		stored, exists := monitor.feedbackStore[feedback.QueryID]
		assert.True(t, exists)
		assert.False(t, stored.IsCorrect)
	}
}

// TestConcurrentFeedbackRecording 测试并发反馈记录
func TestConcurrentFeedbackRecording(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultAccuracyConfig()
	monitor := NewAccuracyMonitor(config, logger)

	const numGoroutines = 10
	const feedbacksPerGoroutine = 10

	// 使用通道等待所有goroutines完成
	done := make(chan bool, numGoroutines)

	// 并发记录反馈
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() { done <- true }()
			
			for j := 0; j < feedbacksPerGoroutine; j++ {
				feedback := QueryFeedback{
					QueryID:      fmt.Sprintf("query-%d-%d", routineID, j),
					UserID:       int64(routineID),
					UserQuery:    "SELECT * FROM users",
					GeneratedSQL: "SELECT id, name FROM users",
					IsCorrect:    j%2 == 0, // 交替true/false
					UserRating:   3 + j%3,
					Timestamp:    time.Now(),
					ModelUsed:    "test-model",
				}
				
				err := monitor.RecordFeedback(feedback)
				assert.NoError(t, err)
			}
		}(i)
	}

	// 等待所有goroutines完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证所有反馈都被记录
	expectedTotal := numGoroutines * feedbacksPerGoroutine
	assert.Equal(t, expectedTotal, len(monitor.feedbackStore))
}

// TestTrendDirection 测试趋势方向枚举
func TestTrendDirection(t *testing.T) {
	assert.Equal(t, "upward", string(TrendUpward))
	assert.Equal(t, "downward", string(TrendDownward))
	assert.Equal(t, "stable", string(TrendStable))
}

// TestAccuracyConfigValidation 测试准确率配置验证
func TestAccuracyConfigValidation(t *testing.T) {
	// 测试边界值配置
	config := &AccuracyConfig{
		MinAccuracyThreshold:     0.5,
		DailyAccuracyTarget:      0.8,
		WeeklyAccuracyTarget:     0.9,
		AlertCooldown:           15 * time.Minute,
		DataRetentionDays:       7,
		SampleSize:              50,
		FeedbackRequiredPercent: 10,
		EnableMLAnalysis:        false,
	}

	logger, _ := zap.NewDevelopment()
	monitor := NewAccuracyMonitor(config, logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, config.MinAccuracyThreshold, monitor.config.MinAccuracyThreshold)
	assert.Equal(t, config.DailyAccuracyTarget, monitor.config.DailyAccuracyTarget)
	assert.Equal(t, config.WeeklyAccuracyTarget, monitor.config.WeeklyAccuracyTarget)
}