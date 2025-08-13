// P2阶段 Day 3-4: 查询分类器单元测试
// 测试机器学习分类器、特征提取、缓存机制等功能
// 验证分类准确率和学习效果

package routing

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestQueryClassifier_ClassifyQuery(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	testCases := []struct {
		name             string
		query            string
		expectedCategory ComplexityCategory
		minConfidence    float64
	}{
		{
			name:             "简单查询分类",
			query:            "SELECT id, name FROM users",
			expectedCategory: CategorySimple,
			minConfidence:    0.5,
		},
		{
			name:             "中等复杂度查询分类",
			query:            "SELECT u.name, COUNT(p.id) FROM users u LEFT JOIN posts p ON u.id = p.user_id GROUP BY u.id",
			expectedCategory: CategoryMedium,
			minConfidence:    0.6,
		},
		{
			name:             "复杂查询分类",
			query:            "WITH RECURSIVE tree AS (SELECT id, parent_id FROM categories WHERE parent_id IS NULL UNION ALL SELECT c.id, c.parent_id FROM categories c JOIN tree t ON c.parent_id = t.id) SELECT * FROM tree",
			expectedCategory: CategoryComplex,
			minConfidence:    0.7,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := classifier.ClassifyQuery(ctx, tc.query, &QueryMetadata{})
			
			if err != nil {
				t.Fatalf("分类失败: %v", err)
			}
			
			// 验证分类结果
			if result.Category != tc.expectedCategory {
				t.Errorf("分类错误: 期望 %s, 实际 %s", tc.expectedCategory, result.Category)
			}
			
			// 验证置信度
			if result.Confidence < tc.minConfidence {
				t.Errorf("置信度过低: 期望 >= %.2f, 实际 %.2f", tc.minConfidence, result.Confidence)
			}
			
			// 验证特征存在
			if result.Features == nil {
				t.Error("特征不能为空")
			}
			
			// 验证评分存在
			if len(result.Scores) == 0 {
				t.Error("评分不能为空")
			}
			
			// 验证处理时间
			if result.ProcessTime <= 0 {
				t.Error("处理时间应该大于0")
			}
		})
	}
}

func TestQueryClassifier_BatchClassifyQueries(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	queries := []string{
		"SELECT * FROM users",
		"SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)",
	}
	
	ctx := context.Background()
	results, err := classifier.BatchClassifyQueries(ctx, queries, &QueryMetadata{})
	
	if err != nil {
		t.Fatalf("批量分类失败: %v", err)
	}
	
	// 验证结果数量
	if len(results) != len(queries) {
		t.Errorf("结果数量不匹配: 期望 %d, 实际 %d", len(queries), len(results))
	}
	
	// 验证每个结果
	for i, result := range results {
		if result == nil {
			t.Errorf("第%d个结果不能为空", i)
			continue
		}
		
		if result.Query != queries[i] {
			t.Errorf("第%d个结果查询不匹配: 期望 %s, 实际 %s", i, queries[i], result.Query)
		}
		
		if result.Category == "" {
			t.Errorf("第%d个结果分类不能为空", i)
		}
	}
}

func TestQueryClassifier_LearnFromFeedback(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	config := &ClassifierConfig{
		EnableLearning: true,
		LearningRate:   0.1,
	}
	classifier := NewQueryClassifier(complexityAnalyzer, config)
	
	query := "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id"
	
	// 初始分类
	ctx := context.Background()
	result1, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("初始分类失败: %v", err)
	}
	
	// 提供反馈学习
	err = classifier.LearnFromFeedback(query, result1.Category, CategoryComplex, 1.0)
	if err != nil {
		t.Fatalf("反馈学习失败: %v", err)
	}
	
	// 再次分类，检查学习效果
	result2, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("学习后分类失败: %v", err)
	}
	
	// 验证学习效果（可能需要多次反馈才能明显看到效果）
	t.Logf("学习前: 类别=%s, 置信度=%.3f", result1.Category, result1.Confidence)
	t.Logf("学习后: 类别=%s, 置信度=%.3f", result2.Category, result2.Confidence)
	
	// 至少验证没有出现错误
	if result2.Category == "" {
		t.Error("学习后分类不能为空")
	}
}

func TestQueryClassifier_UpdateThresholds(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	// 更新阈值
	err := classifier.UpdateThresholds(0.2, 0.8)
	if err != nil {
		t.Fatalf("更新阈值失败: %v", err)
	}
	
	// 测试无效阈值
	err = classifier.UpdateThresholds(0.8, 0.2) // 简单阈值大于复杂阈值
	if err == nil {
		t.Error("应该拒绝无效阈值")
	}
	
	// 测试查询分类是否受到新阈值影响
	ctx := context.Background()
	query := "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id"
	result, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
	
	if err != nil {
		t.Fatalf("使用新阈值分类失败: %v", err)
	}
	
	// 验证分类仍然有效
	if result.Category == "" {
		t.Error("分类不能为空")
	}
	
	t.Logf("新阈值下的分类: %s, 评分: %.3f", result.Category, result.ModelScore)
}

func TestQueryClassifier_CachingMechanism(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	config := &ClassifierConfig{
		EnableCache: true,
		CacheSize:   100,
		CacheTTL:    5 * time.Minute,
	}
	classifier := NewQueryClassifier(complexityAnalyzer, config)
	
	query := "SELECT * FROM users WHERE active = true"
	ctx := context.Background()
	
	// 第一次分类（应该计算）
	start1 := time.Now()
	result1, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
	time1 := time.Since(start1)
	
	if err != nil {
		t.Fatalf("第一次分类失败: %v", err)
	}
	
	// 第二次分类（应该从缓存获取）
	start2 := time.Now()
	result2, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
	time2 := time.Since(start2)
	
	if err != nil {
		t.Fatalf("第二次分类失败: %v", err)
	}
	
	// 验证结果一致
	if result1.Category != result2.Category {
		t.Errorf("缓存结果不一致: %s vs %s", result1.Category, result2.Category)
	}
	
	// 验证缓存效果（第二次应该更快）
	if time2 >= time1 {
		t.Logf("缓存可能未生效，时间：第一次=%v, 第二次=%v", time1, time2)
	}
	
	t.Logf("缓存测试 - 第一次: %v, 第二次: %v, 加速比: %.2fx", time1, time2, float64(time1)/float64(time2))
}

func TestQueryClassifier_GetStats(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	// 执行几次分类以生成统计数据
	queries := []string{
		"SELECT * FROM users",
		"SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)",
	}
	
	ctx := context.Background()
	for _, query := range queries {
		_, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
		if err != nil {
			t.Fatalf("分类失败: %v", err)
		}
	}
	
	// 获取分类统计
	stats := classifier.GetClassificationStats()
	if stats == nil {
		t.Fatal("统计不能为空")
	}
	
	// 验证统计数据
	if stats.totalClassifications == 0 {
		t.Error("总分类数应该大于0")
	}
	
	if len(stats.categoryDistribution) == 0 {
		t.Error("分类分布不能为空")
	}
	
	// 获取模型指标
	metrics := classifier.GetModelMetrics()
	if metrics == nil {
		t.Fatal("模型指标不能为空")
	}
	
	t.Logf("分类统计: 总数=%d, 分布=%+v", stats.totalClassifications, stats.categoryDistribution)
	t.Logf("模型指标: 预测数=%d, 准确率=%.3f", metrics.TotalPredictions, metrics.Accuracy)
}

func TestFeatureExtractor_ExtractFeatures(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	query := "SELECT u.name, COUNT(p.id) as post_count FROM users u LEFT JOIN posts p ON u.id = p.user_id WHERE u.active = true GROUP BY u.id, u.name HAVING COUNT(p.id) > 5 ORDER BY post_count DESC"
	
	ctx := context.Background()
	complexityResult, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("复杂度分析失败: %v", err)
	}
	
	extractor := newFeatureExtractor()
	features := extractor.ExtractFeatures(query, complexityResult)
	
	// 验证基础特征
	if features.QueryLength <= 0 {
		t.Error("查询长度应该大于0")
	}
	
	if features.WordCount <= 0 {
		t.Error("词数应该大于0")
	}
	
	if features.KeywordDensity < 0 || features.KeywordDensity > 1 {
		t.Errorf("关键词密度应该在[0,1]范围内: %.3f", features.KeywordDensity)
	}
	
	// 验证结构特征
	if features.ClauseComplexity < 0 || features.ClauseComplexity > 1 {
		t.Errorf("子句复杂度应该在[0,1]范围内: %.3f", features.ClauseComplexity)
	}
	
	if features.JoinComplexity < 0 {
		t.Error("JOIN复杂度不能为负")
	}
	
	// 验证组合特征
	if features.OverallComplexity < 0 || features.OverallComplexity > 1 {
		t.Errorf("总体复杂度应该在[0,1]范围内: %.3f", features.OverallComplexity)
	}
	
	t.Logf("特征提取结果:")
	t.Logf("  查询长度: %.3f", features.QueryLength)
	t.Logf("  词数: %.3f", features.WordCount)
	t.Logf("  关键词密度: %.3f", features.KeywordDensity)
	t.Logf("  子句复杂度: %.3f", features.ClauseComplexity)
	t.Logf("  JOIN复杂度: %.3f", features.JoinComplexity)
	t.Logf("  总体复杂度: %.3f", features.OverallComplexity)
}

func TestClassificationModel_Classify(t *testing.T) {
	config := getDefaultClassifierConfig()
	model := newClassificationModel(config)
	
	// 测试特征
	features := &QueryFeatures{
		QueryLength:        0.5,
		WordCount:          0.4,
		KeywordDensity:     0.3,
		ClauseComplexity:   0.6,
		JoinComplexity:     0.7,
		NestingDepth:       0.2,
		FunctionComplexity: 0.1,
		ConditionComplexity: 0.4,
		TableComplexity:    0.3,
		RelationComplexity: 0.5,
		WindowFunctionScore: 0.0,
		RecursiveScore:     0.0,
		SubqueryScore:      0.2,
		OverallComplexity:  0.5,
		StructuralBalance:  0.6,
		SemanticRichness:   0.4,
	}
	
	category, confidence, modelScore, reasoning := model.Classify(features)
	
	// 验证结果
	if category == "" {
		t.Error("分类不能为空")
	}
	
	if confidence < 0 || confidence > 1 {
		t.Errorf("置信度应该在[0,1]范围内: %.3f", confidence)
	}
	
	if modelScore < 0 || modelScore > 1 {
		t.Errorf("模型评分应该在[0,1]范围内: %.3f", modelScore)
	}
	
	if reasoning == "" {
		t.Error("推理不能为空")
	}
	
	t.Logf("分类结果: 类别=%s, 置信度=%.3f, 评分=%.3f", category, confidence, modelScore)
	t.Logf("推理: %s", reasoning)
}

func TestQueryClassifier_Performance(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	queries := []string{
		"SELECT * FROM users",
		"SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)",
		"WITH tree AS (SELECT * FROM categories) SELECT * FROM tree",
	}
	
	ctx := context.Background()
	iterations := 50
	
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		for _, query := range queries {
			_, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
			if err != nil {
				t.Fatalf("分类失败: %v", err)
			}
		}
	}
	
	totalTime := time.Since(start)
	avgTime := totalTime / time.Duration(iterations*len(queries))
	
	// 平均分类时间应该在合理范围内
	maxAvgTime := 100 * time.Millisecond
	if avgTime > maxAvgTime {
		t.Errorf("平均分类时间过长: %v (最大允许: %v)", avgTime, maxAvgTime)
	}
	
	t.Logf("性能测试完成: %d次分类, 总时间: %v, 平均时间: %v", 
		iterations*len(queries), totalTime, avgTime)
}

func TestQueryClassifier_ConcurrentSafety(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	query := "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id"
	
	concurrency := 10
	iterations := 20
	
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency*iterations)
	
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			for j := 0; j < iterations; j++ {
				ctx := context.Background()
				result, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
				
				if err != nil {
					errors <- err
					return
				}
				
				if result.Category == "" {
					errors <- fmt.Errorf("goroutine %d: 分类结果为空", id)
					return
				}
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	// 检查错误
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	
	t.Logf("并发测试完成: %d个goroutine, 每个%d次迭代", concurrency, iterations)
}

func TestQueryClassifier_ConfigValidation(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	
	// 测试有效配置
	validConfig := &ClassifierConfig{
		SimpleThreshold:     0.3,
		ComplexThreshold:    0.7,
		LearningRate:        0.01,
		FeatureWeights:      map[string]float64{"query_length": 0.1},
		EnableCache:         true,
		EnableLearning:      true,
		CacheSize:          1000,
		CacheTTL:           30 * time.Minute,
		BatchSize:          10,
		StatsWindowSize:    1000,
		StatsUpdateInterval: 5 * time.Minute,
	}
	
	classifier := NewQueryClassifier(complexityAnalyzer, validConfig)
	if classifier == nil {
		t.Error("有效配置应该创建成功")
	}
	
	// 测试nil配置（应该使用默认配置）
	classifier2 := NewQueryClassifier(complexityAnalyzer, nil)
	if classifier2 == nil {
		t.Error("nil配置应该使用默认配置创建成功")
	}
	
	t.Log("配置验证测试通过")
}

// 基准测试
func BenchmarkQueryClassifier_ClassifyQuery(b *testing.B) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	query := "SELECT u.name, COUNT(p.id) FROM users u LEFT JOIN posts p ON u.id = p.user_id GROUP BY u.id"
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
		if err != nil {
			b.Fatalf("分类失败: %v", err)
		}
	}
}

func BenchmarkQueryClassifier_FeatureExtraction(b *testing.B) {
	analyzer := NewComplexityAnalyzer(nil)
	extractor := newFeatureExtractor()
	
	query := "SELECT u.name, COUNT(p.id) FROM users u LEFT JOIN posts p ON u.id = p.user_id GROUP BY u.id"
	
	ctx := context.Background()
	complexityResult, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
	if err != nil {
		b.Fatalf("复杂度分析失败: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractor.ExtractFeatures(query, complexityResult)
	}
}

func BenchmarkQueryClassifier_ModelClassify(b *testing.B) {
	config := getDefaultClassifierConfig()
	model := newClassificationModel(config)
	
	features := &QueryFeatures{
		QueryLength:         0.5,
		WordCount:          0.4,
		KeywordDensity:     0.3,
		ClauseComplexity:   0.6,
		JoinComplexity:     0.7,
		NestingDepth:       0.2,
		FunctionComplexity: 0.1,
		ConditionComplexity: 0.4,
		TableComplexity:    0.3,
		RelationComplexity: 0.5,
		WindowFunctionScore: 0.0,
		RecursiveScore:     0.0,
		SubqueryScore:      0.2,
		OverallComplexity:  0.5,
		StructuralBalance:  0.6,
		SemanticRichness:   0.4,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = model.Classify(features)
	}
}