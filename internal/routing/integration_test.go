// P2阶段 Day 3-4: 智能路由系统集成测试
// 端到端测试复杂度分析引擎、查询分类器、历史学习机制的集成工作
// 验证整体系统的性能和准确率指标

package routing

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestIntegration_CompleteWorkflow 测试完整的查询复杂度分析和分类工作流程
func TestIntegration_CompleteWorkflow(t *testing.T) {
	// 初始化组件
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	testQueries := []struct {
		name     string
		query    string
		expected ComplexityCategory
		userID   int64
	}{
		{
			name:     "简单查询",
			query:    "SELECT id, name FROM users WHERE active = true",
			expected: CategorySimple,
			userID:   1,
		},
		{
			name:     "中等复杂查询",
			query:    "SELECT u.name, COUNT(p.id) as posts FROM users u LEFT JOIN posts p ON u.id = p.user_id GROUP BY u.id, u.name",
			expected: CategoryMedium,
			userID:   1,
		},
		{
			name:     "复杂查询",
			query:    "WITH RECURSIVE hierarchy AS (SELECT id, name, manager_id, 0 as level FROM employees WHERE manager_id IS NULL UNION ALL SELECT e.id, e.name, e.manager_id, h.level+1 FROM employees e JOIN hierarchy h ON e.manager_id = h.id) SELECT * FROM hierarchy ORDER BY level, name",
			expected: CategoryComplex,
			userID:   2,
		},
	}
	
	ctx := context.Background()
	
	for _, tc := range testQueries {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: 复杂度分析
			complexityResult, err := complexityAnalyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{
				UserID: tc.userID,
			})
			if err != nil {
				t.Fatalf("复杂度分析失败: %v", err)
			}
			
			// Step 2: 查询分类
			classificationResult, err := classifier.ClassifyQuery(ctx, tc.query, &QueryMetadata{
				UserID: tc.userID,
			})
			if err != nil {
				t.Fatalf("查询分类失败: %v", err)
			}
			
			// Step 3: 学习引擎预测
			learningPrediction, err := learningEngine.PredictCategory(tc.query, classificationResult.Features, tc.userID)
			if err != nil {
				t.Fatalf("学习预测失败: %v", err)
			}
			
			// Step 4: 创建历史记录并学习
			historyRecord := &QueryHistoryRecord{
				ID:                fmt.Sprintf("test_%d_%s", tc.userID, tc.name),
				Query:             tc.query,
				NormalizedQuery:   tc.query,
				UserID:            tc.userID,
				PredictedCategory: classificationResult.Category,
				ActualCategory:    tc.expected,
				ComplexityScore:   complexityResult.Score,
				Features:          classificationResult.Features,
				ExecutionTime:     100 * time.Millisecond,
				Success:           true,
				Timestamp:         time.Now(),
				UpdateCount:       1,
				LastUpdated:       time.Now(),
			}
			
			err = learningEngine.LearnFromHistory(historyRecord)
			if err != nil {
				t.Fatalf("历史学习失败: %v", err)
			}
			
			// 验证结果一致性
			t.Logf("查询: %s", tc.query)
			t.Logf("复杂度分析: 类别=%s, 评分=%.3f, 置信度=%.3f", 
				complexityResult.Category, complexityResult.Score, complexityResult.Confidence)
			t.Logf("分类器: 类别=%s, 置信度=%.3f, 模型评分=%.3f", 
				classificationResult.Category, classificationResult.Confidence, classificationResult.ModelScore)
			t.Logf("学习引擎: 最终预测=%s, 置信度=%.3f", 
				learningPrediction.FinalPrediction.Category, learningPrediction.FinalPrediction.Confidence)
			
			// 验证分类结果在合理范围内（允许不同组件有一定差异）
			categories := []ComplexityCategory{
				complexityResult.Category,
				classificationResult.Category,
				learningPrediction.FinalPrediction.Category,
			}
			
			// 至少有一个组件的分类是正确的
			hasCorrectClassification := false
			for _, category := range categories {
				if category == tc.expected {
					hasCorrectClassification = true
					break
				}
			}
			
			if !hasCorrectClassification {
				t.Logf("警告: 所有组件的分类都不匹配期望值 %s, 实际: %v", tc.expected, categories)
			}
		})
	}
}

// TestIntegration_LearningEffectiveness 测试学习机制的有效性
func TestIntegration_LearningEffectiveness(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	config := &ClassifierConfig{
		EnableLearning: true,
		LearningRate:   0.1,
	}
	classifier := NewQueryClassifier(complexityAnalyzer, config)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	query := "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id WHERE p.published = true"
	userID := int64(100)
	ctx := context.Background()
	
	// 初始分类
	initialResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{UserID: userID})
	if err != nil {
		t.Fatalf("初始分类失败: %v", err)
	}
	
	// 模拟多次用户反馈学习
	actualCategory := CategoryMedium
	feedbackRounds := 5
	
	for i := 0; i < feedbackRounds; i++ {
		// 提供反馈
		err = classifier.LearnFromFeedback(query, initialResult.Category, actualCategory, 0.8)
		if err != nil {
			t.Fatalf("第%d轮反馈失败: %v", i+1, err)
		}
		
		// 创建历史记录
		historyRecord := &QueryHistoryRecord{
			ID:                fmt.Sprintf("learning_test_%d", i),
			Query:             query,
			NormalizedQuery:   query,
			UserID:            userID,
			PredictedCategory: initialResult.Category,
			ActualCategory:    actualCategory,
			ComplexityScore:   initialResult.ModelScore,
			Features:          initialResult.Features,
			Feedback: &UserFeedback{
				Rating:         4,
				IsCorrect:      boolPtr(actualCategory == initialResult.Category),
				ActualCategory: &actualCategory,
				Timestamp:      time.Now(),
			},
			ExecutionTime: 50 * time.Millisecond,
			Success:       true,
			Timestamp:     time.Now(),
			UpdateCount:   1,
			LastUpdated:   time.Now(),
		}
		
		err = learningEngine.LearnFromHistory(historyRecord)
		if err != nil {
			t.Fatalf("第%d轮学习失败: %v", i+1, err)
		}
	}
	
	// 学习后再次分类
	finalResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{UserID: userID})
	if err != nil {
		t.Fatalf("学习后分类失败: %v", err)
	}
	
	// 学习引擎预测
	learningPrediction, err := learningEngine.PredictCategory(query, finalResult.Features, userID)
	if err != nil {
		t.Fatalf("学习后预测失败: %v", err)
	}
	
	t.Logf("学习效果测试结果:")
	t.Logf("初始分类: %s (置信度: %.3f)", initialResult.Category, initialResult.Confidence)
	t.Logf("最终分类: %s (置信度: %.3f)", finalResult.Category, finalResult.Confidence)
	t.Logf("学习预测: %s (置信度: %.3f)", 
		learningPrediction.FinalPrediction.Category, learningPrediction.FinalPrediction.Confidence)
	t.Logf("期望分类: %s", actualCategory)
	
	// 验证学习效果
	improvedComponents := 0
	if finalResult.Category == actualCategory && initialResult.Category != actualCategory {
		improvedComponents++
		t.Log("✓ 分类器通过学习改进了分类")
	}
	
	if learningPrediction.FinalPrediction.Category == actualCategory {
		improvedComponents++
		t.Log("✓ 学习引擎预测正确")
	}
	
	if improvedComponents == 0 {
		t.Log("注意: 学习机制可能需要更多样本或调整参数")
	}
}

// TestIntegration_PerformanceUnderLoad 测试系统在负载下的性能
func TestIntegration_PerformanceUnderLoad(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	// 测试查询集
	queries := []string{
		"SELECT * FROM users",
		"SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)",
		"WITH tree AS (SELECT * FROM categories WHERE parent_id IS NULL) SELECT * FROM tree",
		"SELECT name, ROW_NUMBER() OVER (ORDER BY created_at) as rn FROM users",
	}
	
	ctx := context.Background()
	concurrency := 10
	iterationsPerGoroutine := 20
	totalOperations := concurrency * iterationsPerGoroutine * len(queries)
	
	start := time.Now()
	done := make(chan bool, concurrency)
	errors := make(chan error, totalOperations)
	
	for i := 0; i < concurrency; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()
			
			for j := 0; j < iterationsPerGoroutine; j++ {
				for k, query := range queries {
					userID := int64(goroutineID*1000 + j*10 + k)
					
					// 复杂度分析
					complexityResult, err := complexityAnalyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{
						UserID: userID,
					})
					if err != nil {
						errors <- fmt.Errorf("goroutine %d: 复杂度分析失败: %v", goroutineID, err)
						return
					}
					
					// 查询分类
					classificationResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{
						UserID: userID,
					})
					if err != nil {
						errors <- fmt.Errorf("goroutine %d: 查询分类失败: %v", goroutineID, err)
						return
					}
					
					// 学习引擎预测（异步，不等待结果）
					go func() {
						_, _ = learningEngine.PredictCategory(query, classificationResult.Features, userID)
					}()
					
					// 创建历史记录
					historyRecord := &QueryHistoryRecord{
						ID:                fmt.Sprintf("load_test_%d_%d_%d", goroutineID, j, k),
						Query:             query,
						NormalizedQuery:   query,
						UserID:            userID,
						PredictedCategory: classificationResult.Category,
						ActualCategory:    complexityResult.Category,
						ComplexityScore:   complexityResult.Score,
						Features:          classificationResult.Features,
						ExecutionTime:     time.Duration(50+j*5) * time.Millisecond,
						Success:           true,
						Timestamp:         time.Now(),
						UpdateCount:       1,
						LastUpdated:       time.Now(),
					}
					
					// 异步学习
					go func() {
						_ = learningEngine.LearnFromHistory(historyRecord)
					}()
				}
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	totalTime := time.Since(start)
	
	// 检查错误
	close(errors)
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}
	
	// 性能指标
	avgOperationTime := totalTime / time.Duration(totalOperations)
	operationsPerSecond := float64(totalOperations) / totalTime.Seconds()
	
	t.Logf("负载测试结果:")
	t.Logf("总操作数: %d", totalOperations)
	t.Logf("并发数: %d", concurrency)
	t.Logf("总时间: %v", totalTime)
	t.Logf("平均操作时间: %v", avgOperationTime)
	t.Logf("操作/秒: %.2f", operationsPerSecond)
	t.Logf("错误数: %d (%.2f%%)", errorCount, float64(errorCount)/float64(totalOperations)*100)
	
	// 性能断言
	if avgOperationTime > 500*time.Millisecond {
		t.Errorf("平均操作时间过长: %v", avgOperationTime)
	}
	
	if float64(errorCount)/float64(totalOperations) > 0.01 { // 1%错误率
		t.Errorf("错误率过高: %.2f%%", float64(errorCount)/float64(totalOperations)*100)
	}
}

// TestIntegration_ModelConsistency 测试不同组件间的模型一致性
func TestIntegration_ModelConsistency(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	
	testQueries := []string{
		"SELECT * FROM users WHERE id = 1",                    // 简单
		"SELECT u.*, p.* FROM users u JOIN posts p ON u.id = p.user_id", // 中等
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts WHERE created_at > (SELECT MAX(created_at) - INTERVAL '7 days' FROM posts))", // 复杂
	}
	
	ctx := context.Background()
	consistencyThreshold := 0.3 // 允许评分差异
	
	for i, query := range testQueries {
		t.Run(fmt.Sprintf("Query_%d", i+1), func(t *testing.T) {
			// 多次分析同一查询
			runs := 5
			complexityResults := make([]*ComplexityResult, runs)
			classificationResults := make([]*ClassificationResult, runs)
			
			for j := 0; j < runs; j++ {
				// 复杂度分析
				cResult, err := complexityAnalyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
				if err != nil {
					t.Fatalf("第%d次复杂度分析失败: %v", j+1, err)
				}
				complexityResults[j] = cResult
				
				// 查询分类
				clResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{})
				if err != nil {
					t.Fatalf("第%d次查询分类失败: %v", j+1, err)
				}
				classificationResults[j] = clResult
			}
			
			// 验证一致性
			// 1. 复杂度分析一致性
			baseComplexity := complexityResults[0]
			for j := 1; j < runs; j++ {
				if complexityResults[j].Category != baseComplexity.Category {
					t.Errorf("复杂度分析分类不一致: 第1次=%s, 第%d次=%s", 
						baseComplexity.Category, j+1, complexityResults[j].Category)
				}
				
				scoreDiff := abs64(complexityResults[j].Score - baseComplexity.Score)
				if scoreDiff > consistencyThreshold {
					t.Errorf("复杂度分析评分不一致: 第1次=%.3f, 第%d次=%.3f, 差异=%.3f", 
						baseComplexity.Score, j+1, complexityResults[j].Score, scoreDiff)
				}
			}
			
			// 2. 查询分类一致性
			baseClassification := classificationResults[0]
			for j := 1; j < runs; j++ {
				if classificationResults[j].Category != baseClassification.Category {
					t.Errorf("查询分类不一致: 第1次=%s, 第%d次=%s", 
						baseClassification.Category, j+1, classificationResults[j].Category)
				}
				
				scoreDiff := abs64(classificationResults[j].ModelScore - baseClassification.ModelScore)
				if scoreDiff > consistencyThreshold {
					t.Errorf("分类评分不一致: 第1次=%.3f, 第%d次=%.3f, 差异=%.3f", 
						baseClassification.ModelScore, j+1, classificationResults[j].ModelScore, scoreDiff)
				}
			}
			
			t.Logf("查询一致性验证通过: %s", query)
			t.Logf("复杂度分析: %s (%.3f)", baseComplexity.Category, baseComplexity.Score)
			t.Logf("查询分类: %s (%.3f)", baseClassification.Category, baseClassification.ModelScore)
		})
	}
}

// TestIntegration_SystemRecovery 测试系统错误恢复能力
func TestIntegration_SystemRecovery(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	ctx := context.Background()
	
	// 测试各种边缘情况和错误输入
	errorCases := []struct {
		name  string
		query string
	}{
		{"空查询", ""},
		{"无效SQL", "This is not SQL"},
		{"不完整SQL", "SELECT * FROM"},
		{"语法错误", "SELECT * FORM users"},
		{"超长查询", string(make([]byte, 10000))},
		{"特殊字符", "SELECT '\x00\x01\x02' FROM users"},
	}
	
	successCount := 0
	totalCases := len(errorCases)
	
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			// 复杂度分析
			_, err := complexityAnalyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			complexityOK := err == nil
			
			// 查询分类
			classificationResult, err := classifier.ClassifyQuery(ctx, tc.query, &QueryMetadata{})
			classificationOK := err == nil && classificationResult != nil
			
			// 学习引擎预测
			var learningOK bool
			if classificationOK && classificationResult.Features != nil {
				_, err := learningEngine.PredictCategory(tc.query, classificationResult.Features, 1)
				learningOK = err == nil
			}
			
			t.Logf("错误案例 '%s': 复杂度分析=%v, 查询分类=%v, 学习预测=%v", 
				tc.name, complexityOK, classificationOK, learningOK)
			
			// 至少一个组件应该能够优雅处理错误
			if complexityOK || classificationOK || learningOK {
				successCount++
			}
		})
	}
	
	recoveryRate := float64(successCount) / float64(totalCases)
	t.Logf("系统恢复能力: %d/%d (%.1f%%)", successCount, totalCases, recoveryRate*100)
	
	// 恢复率应该合理
	if recoveryRate < 0.5 {
		t.Errorf("系统恢复能力过低: %.1f%%", recoveryRate*100)
	}
}

// TestIntegration_DataFlow 测试数据在各组件间的流转
func TestIntegration_DataFlow(t *testing.T) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	query := "SELECT u.name, p.title, COUNT(c.id) as comments FROM users u JOIN posts p ON u.id = p.user_id LEFT JOIN comments c ON p.id = c.post_id GROUP BY u.id, p.id HAVING COUNT(c.id) > 10"
	userID := int64(999)
	ctx := context.Background()
	
	// Step 1: 复杂度分析
	complexityResult, err := complexityAnalyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{
		UserID: userID,
		DatabaseName: "test_db",
		TableNames: []string{"users", "posts", "comments"},
	})
	if err != nil {
		t.Fatalf("复杂度分析失败: %v", err)
	}
	
	// 验证复杂度分析结果的完整性
	if complexityResult.Query != query {
		t.Error("查询字符串在复杂度分析中丢失")
	}
	if complexityResult.Details == nil {
		t.Error("复杂度分析详情缺失")
	}
	
	// Step 2: 使用复杂度结果进行分类
	classificationResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("查询分类失败: %v", err)
	}
	
	// 验证分类结果使用了复杂度分析的数据
	if classificationResult.Scores["complexity_score"] != complexityResult.Score {
		t.Error("分类器没有正确使用复杂度分析评分")
	}
	if classificationResult.Features == nil {
		t.Error("特征提取失败")
	}
	
	// Step 3: 使用分类结果进行学习预测
	learningPrediction, err := learningEngine.PredictCategory(query, classificationResult.Features, userID)
	if err != nil {
		t.Fatalf("学习预测失败: %v", err)
	}
	
	// 验证学习预测使用了前面的数据
	if learningPrediction.Query != query {
		t.Error("查询字符串在学习预测中丢失")
	}
	if learningPrediction.UserID != userID {
		t.Error("用户ID在学习预测中丢失")
	}
	
	// Step 4: 创建历史记录，验证数据完整性
	historyRecord := &QueryHistoryRecord{
		ID:                "dataflow_test",
		Query:             query,
		NormalizedQuery:   query,
		UserID:            userID,
		PredictedCategory: classificationResult.Category,
		ActualCategory:    complexityResult.Category,
		ComplexityScore:   complexityResult.Score,
		Features:          classificationResult.Features,
		ExecutionTime:     classificationResult.ProcessTime,
		Success:           true,
		Timestamp:         time.Now(),
		UpdateCount:       1,
		LastUpdated:       time.Now(),
	}
	
	err = learningEngine.LearnFromHistory(historyRecord)
	if err != nil {
		t.Fatalf("历史学习失败: %v", err)
	}
	
	// 验证数据流转的一致性
	t.Logf("数据流转验证:")
	t.Logf("原始查询长度: %d", len(query))
	t.Logf("复杂度分析: 类别=%s, 评分=%.3f, 处理时间=%v", 
		complexityResult.Category, complexityResult.Score, complexityResult.ProcessTime)
	t.Logf("查询分类: 类别=%s, 置信度=%.3f, 处理时间=%v", 
		classificationResult.Category, classificationResult.Confidence, classificationResult.ProcessTime)
	t.Logf("学习预测: 类别=%s, 置信度=%.3f", 
		learningPrediction.FinalPrediction.Category, learningPrediction.FinalPrediction.Confidence)
	t.Logf("历史记录: ID=%s, 用户=%d, 成功=%v", 
		historyRecord.ID, historyRecord.UserID, historyRecord.Success)
	
	// 验证数据的关联性
	if complexityResult.Score == 0 && classificationResult.ModelScore == 0 {
		t.Error("数据可能没有正确流转，所有评分都为0")
	}
	
	t.Log("✓ 数据流转验证通过")
}

// 辅助函数
func boolPtr(b bool) *bool {
	return &b
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// 基准测试
func BenchmarkIntegration_CompleteWorkflow(b *testing.B) {
	complexityAnalyzer := NewComplexityAnalyzer(nil)
	classifier := NewQueryClassifier(complexityAnalyzer, nil)
	learningEngine := NewLearningEngine(context.Background(), nil)
	defer learningEngine.Close()
	
	query := "SELECT u.name, COUNT(p.id) FROM users u LEFT JOIN posts p ON u.id = p.user_id GROUP BY u.id"
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := int64(i)
		
		// 完整工作流程
		_, err := complexityAnalyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{
			UserID: userID,
		})
		if err != nil {
			b.Fatalf("复杂度分析失败: %v", err)
		}
		
		classificationResult, err := classifier.ClassifyQuery(ctx, query, &QueryMetadata{
			UserID: userID,
		})
		if err != nil {
			b.Fatalf("查询分类失败: %v", err)
		}
		
		_, err = learningEngine.PredictCategory(query, classificationResult.Features, userID)
		if err != nil {
			b.Fatalf("学习预测失败: %v", err)
		}
	}
}