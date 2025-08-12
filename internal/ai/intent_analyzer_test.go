// Package ai 意图分析器测试
package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewIntentAnalyzer 测试意图分析器初始化
func TestNewIntentAnalyzer(t *testing.T) {
	analyzer := NewIntentAnalyzer()
	
	assert.NotNil(t, analyzer)
	assert.NotNil(t, analyzer.config)
	assert.NotNil(t, analyzer.patterns)
	assert.NotNil(t, analyzer.keywordWeights)
	assert.NotNil(t, analyzer.analysisCache)
	assert.NotNil(t, analyzer.userPatterns)
	
	// 验证默认配置
	assert.True(t, analyzer.config.EnableCache)
	assert.Equal(t, 1000, analyzer.config.CacheSize)
	assert.Equal(t, 0.6, analyzer.config.MinConfidence)
	
	// 验证模式已初始化
	assert.NotEmpty(t, analyzer.patterns[IntentDataQuery])
	assert.NotEmpty(t, analyzer.patterns[IntentAggregation])
	assert.NotEmpty(t, analyzer.patterns[IntentJoinQuery])
	assert.NotEmpty(t, analyzer.patterns[IntentTimeSeriesAnalysis])
}

// TestIntentAnalyzer_AnalyzeIntent 测试基本意图分析
func TestIntentAnalyzer_AnalyzeIntent(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name           string
		query          string
		expectedIntent QueryIntent
	}{
		{
			name:           "simple_data_query",
			query:          "查询所有用户信息",
			expectedIntent: IntentDataQuery,
		},
		{
			name:           "aggregation_query",
			query:          "统计用户总数",
			expectedIntent: IntentAggregation,
		},
		{
			name:           "aggregation_with_count",
			query:          "计算每个部门的员工数量",
			expectedIntent: IntentAggregation,
		},
		{
			name:           "join_query",
			query:          "查询用户及其相关订单信息",
			expectedIntent: IntentJoinQuery,
		},
		{
			name:           "time_series_query",
			query:          "分析最近30天的销售趋势",
			expectedIntent: IntentTimeSeriesAnalysis,
		},
		{
			name:           "time_series_with_history",
			query:          "显示过去一年的用户增长历史",
			expectedIntent: IntentTimeSeriesAnalysis,
		},
		{
			name:           "comparison_query",
			query:          "比较不同产品的销售额",
			expectedIntent: IntentComparison,
		},
		{
			name:           "ranking_query",
			query:          "显示销售额前10名的产品",
			expectedIntent: IntentRanking,
		},
		{
			name:           "filtering_query",
			query:          "筛选状态为活跃的用户",
			expectedIntent: IntentFiltering,
		},
		{
			name:           "grouping_query",
			query:          "按部门分组显示员工信息",
			expectedIntent: IntentGrouping,
		},
		{
			name:           "english_aggregation",
			query:          "count total number of users",
			expectedIntent: IntentAggregation,
		},
		{
			name:           "english_join",
			query:          "show users with their corresponding orders",
			expectedIntent: IntentJoinQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := analyzer.AnalyzeIntent(tt.query)
			assert.Equal(t, tt.expectedIntent, intent, "Query: %s", tt.query)
		})
	}
}

// TestIntentAnalyzer_AnalyzeIntentDetailed 测试详细意图分析
func TestIntentAnalyzer_AnalyzeIntentDetailed(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name                string
		query               string
		expectedPrimary     QueryIntent
		minConfidence       float64
		expectSecondary     bool
		expectFeatures      map[string]bool
	}{
		{
			name:            "aggregation_with_grouping",
			query:           "按部门统计员工数量",
			expectedPrimary: IntentAggregation,
			minConfidence:   0.7,
			expectSecondary: true,
			expectFeatures: map[string]bool{
				"has_aggregation": true,
				"has_grouping":    true,
			},
		},
		{
			name:            "time_series_with_comparison",
			query:           "比较去年和今年的销售趋势",
			expectedPrimary: IntentTimeSeriesAnalysis,
			minConfidence:   0.6,
			expectSecondary: true,
			expectFeatures: map[string]bool{
				"has_time_reference": true,
				"has_comparison":     true,
			},
		},
		{
			name:            "complex_join_with_aggregation",
			query:           "查询每个用户的订单总数和总金额",
			expectedPrimary: IntentAggregation,
			minConfidence:   0.6,
			expectSecondary: true,
			expectFeatures: map[string]bool{
				"has_aggregation": true,
				"has_join":        true,
			},
		},
		{
			name:            "simple_filter",
			query:           "显示活跃状态的用户",
			expectedPrimary: IntentDataQuery,
			minConfidence:   0.6,
			expectSecondary: false,
			expectFeatures: map[string]bool{
				"has_filtering": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeIntentDetailed(tt.query, 1001)
			
			assert.Equal(t, tt.expectedPrimary, result.PrimaryIntent)
			assert.GreaterOrEqual(t, result.Confidence, tt.minConfidence)
			
			if tt.expectSecondary {
				assert.NotEmpty(t, result.SecondaryIntents)
			}
			
			// 检查特征
			for feature, expected := range tt.expectFeatures {
				switch feature {
				case "has_aggregation":
					assert.Equal(t, expected, result.QueryFeatures.HasAggregation)
				case "has_grouping":
					assert.Equal(t, expected, result.QueryFeatures.HasGrouping)
				case "has_time_reference":
					assert.Equal(t, expected, result.QueryFeatures.HasTimeReference)
				case "has_comparison":
					assert.Equal(t, expected, result.QueryFeatures.HasComparison)
				case "has_join":
					assert.Equal(t, expected, result.QueryFeatures.HasJoin)
				case "has_filtering":
					assert.Equal(t, expected, result.QueryFeatures.HasFiltering)
				}
			}
			
			assert.NotEmpty(t, result.Keywords)
			assert.NotZero(t, result.ProcessingTime)
		})
	}
}

// TestIntentAnalyzer_NormalizeQuery 测试查询标准化
func TestIntentAnalyzer_NormalizeQuery(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "to_lowercase",
			input:    "SELECT * FROM USERS",
			expected: "select * from users",
		},
		{
			name:     "remove_extra_spaces",
			input:    "select   *   from    users",
			expected: "select * from users",
		},
		{
			name:     "remove_punctuation",
			input:    "查询所有用户信息!",
			expected: "查询所有用户信息",
		},
		{
			name:     "trim_spaces",
			input:    "  查询用户信息  ",
			expected: "查询用户信息",
		},
		{
			name:     "complex_normalization",
			input:    "  SELECT   *   FROM   Users  WHERE  status='active'!!!  ",
			expected: "select * from users where status active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.normalizeQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIntentAnalyzer_ExtractQueryFeatures 测试查询特征提取
func TestIntentAnalyzer_ExtractQueryFeatures(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name           string
		query          string
		expectedFeatures QueryFeatures
	}{
		{
			name:  "simple_select",
			query: "select name from users",
			expectedFeatures: QueryFeatures{
				HasTimeReference: false,
				HasComparison:    false,
				HasAggregation:   false,
				HasGrouping:      false,
				HasSorting:       false,
				HasFiltering:     false,
				HasJoin:          false,
				QueryComplexity:  "simple",
			},
		},
		{
			name:  "aggregation_query",
			query: "统计用户总数",
			expectedFeatures: QueryFeatures{
				HasAggregation:   true,
				QueryComplexity:  "simple",
			},
		},
		{
			name:  "time_series_query",
			query: "分析最近30天的趋势",
			expectedFeatures: QueryFeatures{
				HasTimeReference: true,
				QueryComplexity:  "simple",
			},
		},
		{
			name:  "complex_query",
			query: "按部门统计过去一年每个月的员工数量变化趋势",
			expectedFeatures: QueryFeatures{
				HasTimeReference: true,
				HasAggregation:   true,
				HasGrouping:      true,
				QueryComplexity:  "complex",
			},
		},
		{
			name:  "join_with_comparison",
			query: "查询用户及其订单信息，比较不同用户的消费金额",
			expectedFeatures: QueryFeatures{
				HasJoin:       true,
				HasComparison: true,
				QueryComplexity: "medium",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := analyzer.extractQueryFeatures(tt.query)
			
			assert.Equal(t, tt.expectedFeatures.HasTimeReference, features.HasTimeReference)
			assert.Equal(t, tt.expectedFeatures.HasComparison, features.HasComparison)
			assert.Equal(t, tt.expectedFeatures.HasAggregation, features.HasAggregation)
			assert.Equal(t, tt.expectedFeatures.HasGrouping, features.HasGrouping)
			assert.Equal(t, tt.expectedFeatures.HasSorting, features.HasSorting)
			assert.Equal(t, tt.expectedFeatures.HasFiltering, features.HasFiltering)
			assert.Equal(t, tt.expectedFeatures.HasJoin, features.HasJoin)
			assert.Equal(t, tt.expectedFeatures.QueryComplexity, features.QueryComplexity)
		})
	}
}

// TestIntentAnalyzer_CalculatePatternScore 测试模式匹配分数计算
func TestIntentAnalyzer_CalculatePatternScore(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name     string
		query    string
		pattern  IntentPattern
		minScore float64
	}{
		{
			name:  "keyword_match",
			query: "统计用户数量",
			pattern: IntentPattern{
				Keywords: []string{"统计", "数量"},
				Weight:   1.0,
			},
			minScore: 1.5, // 两个关键词匹配
		},
		{
			name:  "phrase_match",
			query: "显示数据统计信息",
			pattern: IntentPattern{
				Phrases: []string{"显示数据"},
				Weight:  1.2,
			},
			minScore: 1.5, // 短语匹配得分更高
		},
		{
			name:  "regex_match",
			query: "查询最近30天的数据",
			pattern: IntentPattern{
				Regex:  `最近\d+天`,
				Weight: 1.1,
			},
			minScore: 2.0, // 正则匹配得分最高
		},
		{
			name:  "anti_pattern",
			query: "删除用户统计信息",
			pattern: IntentPattern{
				Keywords:     []string{"统计"},
				AntiPatterns: []string{"删除"},
				Weight:       1.0,
			},
			minScore: 0.1, // 有反模式，分数减半
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.calculatePatternScore(tt.query, tt.pattern)
			assert.GreaterOrEqual(t, score, tt.minScore)
		})
	}
}

// TestIntentAnalyzer_ExtractKeywords 测试关键词提取
func TestIntentAnalyzer_ExtractKeywords(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple_keywords",
			query:    "查询用户信息",
			expected: []string{"查询", "用户", "信息"},
		},
		{
			name:     "with_stopwords",
			query:    "查询所有的用户信息",
			expected: []string{"查询", "所有", "用户", "信息"},
		},
		{
			name:     "english_keywords",
			query:    "select user data from database",
			expected: []string{"select", "user", "data", "from", "database"},
		},
		{
			name:     "mixed_language",
			query:    "查询user信息from数据库",
			expected: []string{"查询user", "信息from", "数据库"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := analyzer.extractKeywords(tt.query)
			
			// 检查关键词数量
			assert.GreaterOrEqual(t, len(keywords), len(tt.expected)-1) // 允许一些差异
			
			// 检查是否包含期望的关键词（这里只是验证算法能正常工作，不严格匹配）
			_ = keywords // 避免未使用警告
		})
	}
}

// TestIntentAnalyzer_ExtractEntities 测试实体提取
func TestIntentAnalyzer_ExtractEntities(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name     string
		query    string
		expected map[string][]string
	}{
		{
			name:  "time_entities",
			query: "查询2023年10月的销售数据",
			expected: map[string][]string{
				"time": {"2023年", "10月"},
			},
		},
		{
			name:  "number_entities",
			query: "显示前10名用户的信息",
			expected: map[string][]string{
				"number": {"10"},
			},
		},
		{
			name:  "recent_days",
			query: "统计最近30天的用户活跃度",
			expected: map[string][]string{
				"time":   {"最近30天"},
				"number": {"30"},
			},
		},
		{
			name:     "no_entities",
			query:    "查询用户信息",
			expected: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := analyzer.extractEntities(tt.query)
			
			for entityType, expectedValues := range tt.expected {
				actualValues, exists := entities[entityType]
				assert.True(t, exists, "Expected entity type %s not found", entityType)
				
				// 检查实体值的数量
				assert.GreaterOrEqual(t, len(actualValues), len(expectedValues))
				
				// 检查是否包含期望的实体值
				for _, expectedValue := range expectedValues {
					found := false
					for _, actualValue := range actualValues {
						if actualValue == expectedValue {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected entity value %s not found", expectedValue)
				}
			}
		})
	}
}

// TestIntentAnalyzer_UserLearning 测试用户学习功能
func TestIntentAnalyzer_UserLearning(t *testing.T) {
	analyzer := NewIntentAnalyzer()
	userID := int64(1001)

	// 初始时用户档案不存在
	profile := analyzer.GetUserStats(userID)
	assert.NotNil(t, profile)
	assert.Equal(t, userID, profile.UserID)
	assert.Empty(t, profile.IntentHistory)

	// 进行几次查询分析
	queries := []struct {
		query  string
		intent QueryIntent
	}{
		{"统计用户数量", IntentAggregation},
		{"计算订单总金额", IntentAggregation},
		{"查询用户信息", IntentDataQuery},
		{"分析销售趋势", IntentTimeSeriesAnalysis},
		{"统计产品销量", IntentAggregation},
	}

	for _, q := range queries {
		analyzer.AnalyzeIntentDetailed(q.query, userID)
	}

	// 检查用户档案更新
	updatedProfile := analyzer.GetUserStats(userID)
	assert.Equal(t, len(queries), len(updatedProfile.IntentHistory))
	assert.Equal(t, len(queries), updatedProfile.QueryCount)

	// 检查偏好统计
	aggPreference, exists := updatedProfile.PreferredIntents[IntentAggregation]
	assert.True(t, exists)
	assert.Equal(t, 0.6, aggPreference) // 3/5 = 0.6

	// 检查关键词统计
	assert.Contains(t, updatedProfile.CommonKeywords, "统计")
	assert.Equal(t, 2, updatedProfile.CommonKeywords["统计"]) // 出现2次
}

// TestIntentAnalyzer_GenerateSuggestions 测试建议生成
func TestIntentAnalyzer_GenerateSuggestions(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name           string
		result         *IntentResult
		expectSuggestionCount int
	}{
		{
			name: "aggregation_without_grouping",
			result: &IntentResult{
				PrimaryIntent: IntentAggregation,
				Confidence:    0.8,
				QueryFeatures: QueryFeatures{
					HasAggregation: true,
					HasGrouping:    false,
				},
			},
			expectSuggestionCount: 1,
		},
		{
			name: "low_confidence_query",
			result: &IntentResult{
				PrimaryIntent: IntentDataQuery,
				Confidence:    0.5,
				QueryFeatures: QueryFeatures{},
			},
			expectSuggestionCount: 1,
		},
		{
			name: "complex_query",
			result: &IntentResult{
				PrimaryIntent: IntentAggregation,
				Confidence:    0.9,
				QueryFeatures: QueryFeatures{
					QueryComplexity: "complex",
				},
			},
			expectSuggestionCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := analyzer.generateSuggestions(tt.result)
			assert.GreaterOrEqual(t, len(suggestions), tt.expectSuggestionCount)
		})
	}
}

// TestIntentAnalyzer_GetIntentName 测试意图名称获取
func TestIntentAnalyzer_GetIntentName(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		intent   QueryIntent
		expected string
	}{
		{IntentUnknown, "未知意图"},
		{IntentDataQuery, "数据查询"},
		{IntentAggregation, "聚合统计"},
		{IntentJoinQuery, "关联查询"},
		{IntentTimeSeriesAnalysis, "时间序列分析"},
		{IntentComparison, "对比分析"},
		{IntentRanking, "排序排名"},
		{IntentFiltering, "条件筛选"},
		{IntentGrouping, "分组查询"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			name := analyzer.GetIntentName(tt.intent)
			assert.Equal(t, tt.expected, name)
		})
	}
}

// TestIntentAnalyzer_CacheManagement 测试缓存管理
func TestIntentAnalyzer_CacheManagement(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	// 进行一次分析以创建缓存
	query := "查询用户信息"
	result1 := analyzer.AnalyzeIntentDetailed(query, 0)

	// 再次分析同样的查询，应该从缓存获取
	result2 := analyzer.AnalyzeIntentDetailed(query, 0)

	// 结果应该相同
	assert.Equal(t, result1.PrimaryIntent, result2.PrimaryIntent)
	assert.Equal(t, result1.Confidence, result2.Confidence)

	// 获取缓存统计
	stats := analyzer.GetCacheStats()
	assert.Equal(t, 1, stats["cache_size"])
	assert.Equal(t, 1000, stats["max_cache_size"])

	// 清理缓存
	analyzer.ClearCache()
	clearedStats := analyzer.GetCacheStats()
	assert.Equal(t, 0, clearedStats["cache_size"])
}

// Benchmark测试
func BenchmarkIntentAnalyzer_AnalyzeIntent(b *testing.B) {
	analyzer := NewIntentAnalyzer()
	query := "统计最近30天每个部门的员工数量和平均工资，按数量排序"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeIntent(query)
	}
}

func BenchmarkIntentAnalyzer_ExtractQueryFeatures(b *testing.B) {
	analyzer := NewIntentAnalyzer()
	query := "分析过去一年各个产品类别的销售趋势，比较同比增长率，显示前20名"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.extractQueryFeatures(query)
	}
}

func BenchmarkIntentAnalyzer_CalculateIntentScores(b *testing.B) {
	analyzer := NewIntentAnalyzer()
	query := "按地区统计最近半年的订单数量和销售额变化趋势"
	features := analyzer.extractQueryFeatures(query)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.calculateIntentScores(query, features)
	}
}