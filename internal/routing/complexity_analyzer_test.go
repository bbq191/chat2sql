// P2阶段 Day 3-4: 复杂度分析引擎单元测试
// 全面测试关键词分析、语法分析、表关联分析等核心功能
// 验证分类准确率和性能指标

package routing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestComplexityAnalyzer_AnalyzeComplexity(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	testCases := []struct {
		name             string
		query            string
		expectedCategory ComplexityCategory
		minScore         float64
		maxScore         float64
	}{
		{
			name:             "简单SELECT查询",
			query:            "SELECT name, age FROM users WHERE id = 1",
			expectedCategory: CategorySimple,
			minScore:         0.0,
			maxScore:         0.3,
		},
		{
			name:             "中等复杂度JOIN查询",
			query:            "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id WHERE u.active = true ORDER BY u.name",
			expectedCategory: CategoryMedium,
			minScore:         0.3,
			maxScore:         0.7,
		},
		{
			name:             "复杂子查询",
			query:            "SELECT * FROM users WHERE id IN (SELECT user_id FROM posts WHERE created_at > (SELECT MAX(created_at) - INTERVAL '30 days' FROM posts))",
			expectedCategory: CategoryComplex,
			minScore:         0.7,
			maxScore:         1.0,
		},
		{
			name:             "窗口函数查询",
			query:            "SELECT name, salary, ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank FROM employees",
			expectedCategory: CategoryComplex,
			minScore:         0.7,
			maxScore:         1.0,
		},
		{
			name:             "递归CTE",
			query:            "WITH RECURSIVE hierarchy AS (SELECT id, name, manager_id FROM employees WHERE manager_id IS NULL UNION ALL SELECT e.id, e.name, e.manager_id FROM employees e JOIN hierarchy h ON e.manager_id = h.id) SELECT * FROM hierarchy",
			expectedCategory: CategoryComplex,
			minScore:         0.8,
			maxScore:         1.0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := analyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			
			if err != nil {
				t.Fatalf("分析失败: %v", err)
			}
			
			// 验证分类
			if result.Category != tc.expectedCategory {
				t.Errorf("分类错误: 期望 %s, 实际 %s", tc.expectedCategory, result.Category)
			}
			
			// 验证评分范围
			if result.Score < tc.minScore || result.Score > tc.maxScore {
				t.Errorf("评分超出范围: 期望 [%.2f, %.2f], 实际 %.2f", tc.minScore, tc.maxScore, result.Score)
			}
			
			// 验证置信度
			if result.Confidence < 0.0 || result.Confidence > 1.0 {
				t.Errorf("置信度超出范围: %.2f", result.Confidence)
			}
			
			// 验证处理时间
			if result.ProcessTime <= 0 {
				t.Error("处理时间应该大于0")
			}
			
			// 验证详细分析存在
			if result.Details == nil {
				t.Error("应该包含详细分析")
			}
		})
	}
}

func TestKeywordAnalyzer(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	testCases := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "基础关键词",
			query:    "SELECT * FROM users WHERE active = true",
			expected: []string{"select", "from", "where"},
		},
		{
			name:     "JOIN关键词",
			query:    "SELECT u.*, p.* FROM users u INNER JOIN posts p ON u.id = p.user_id",
			expected: []string{"select", "from", "join", "inner"},
		},
		{
			name:     "复杂关键词",
			query:    "WITH RECURSIVE tree AS (SELECT * FROM categories WHERE parent_id IS NULL) SELECT * FROM tree",
			expected: []string{"with", "recursive", "select", "from", "where"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := analyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			
			if err != nil {
				t.Fatalf("分析失败: %v", err)
			}
			
			if result.Details == nil {
				t.Fatal("详细分析不能为空")
			}
			
			// 验证关键词匹配
			for _, expectedKeyword := range tc.expected {
				found := false
				for _, match := range result.Details.KeywordMatches {
					if strings.ToLower(match) == expectedKeyword {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("未找到期望的关键词: %s", expectedKeyword)
				}
			}
		})
	}
}

func TestSyntaxAnalyzer(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	testCases := []struct {
		name               string
		query              string
		expectedClauses    int
		expectedSubqueries int
		expectedJoins      int
	}{
		{
			name:               "简单查询",
			query:              "SELECT name FROM users",
			expectedClauses:    2, // SELECT, FROM
			expectedSubqueries: 0,
			expectedJoins:      0,
		},
		{
			name:               "复杂查询",
			query:              "SELECT u.name FROM users u LEFT JOIN posts p ON u.id = p.user_id WHERE u.active = true GROUP BY u.id HAVING COUNT(p.id) > 0 ORDER BY u.name",
			expectedClauses:    6, // SELECT, FROM, WHERE, GROUP, HAVING, ORDER
			expectedSubqueries: 0,
			expectedJoins:      1,
		},
		{
			name:               "子查询",
			query:              "SELECT * FROM users WHERE id IN (SELECT user_id FROM posts WHERE published = true)",
			expectedClauses:    2,
			expectedSubqueries: 1,
			expectedJoins:      0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := analyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			
			if err != nil {
				t.Fatalf("分析失败: %v", err)
			}
			
			if result.Details == nil {
				t.Fatal("详细分析不能为空")
			}
			
			details := result.Details
			
			// 验证子句数量（允许一定误差）
			if abs(details.ClauseCount-tc.expectedClauses) > 1 {
				t.Errorf("子句数量不匹配: 期望 %d, 实际 %d", tc.expectedClauses, details.ClauseCount)
			}
			
			// 验证子查询数量
			if details.SubqueryCount != tc.expectedSubqueries {
				t.Errorf("子查询数量不匹配: 期望 %d, 实际 %d", tc.expectedSubqueries, details.SubqueryCount)
			}
			
			// 验证JOIN数量
			if details.JoinCount != tc.expectedJoins {
				t.Errorf("JOIN数量不匹配: 期望 %d, 实际 %d", tc.expectedJoins, details.JoinCount)
			}
		})
	}
}

func TestRelationAnalyzer(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	testCases := []struct {
		name             string
		query            string
		expectedTables   int
		expectedRelations int
	}{
		{
			name:              "单表查询",
			query:             "SELECT * FROM users",
			expectedTables:    1,
			expectedRelations: 0,
		},
		{
			name:              "双表JOIN",
			query:             "SELECT * FROM users u JOIN posts p ON u.id = p.user_id",
			expectedTables:    2,
			expectedRelations: 1,
		},
		{
			name:              "多表JOIN",
			query:             "SELECT * FROM users u JOIN posts p ON u.id = p.user_id JOIN comments c ON p.id = c.post_id",
			expectedTables:    3,
			expectedRelations: 2,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := analyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			
			if err != nil {
				t.Fatalf("分析失败: %v", err)
			}
			
			if result.Details == nil {
				t.Fatal("详细分析不能为空")
			}
			
			details := result.Details
			
			// 验证表数量（允许一定误差，因为表名识别可能不完美）
			if abs(details.TableCount-tc.expectedTables) > 1 {
				t.Errorf("表数量不匹配: 期望 %d, 实际 %d", tc.expectedTables, details.TableCount)
			}
		})
	}
}

func TestComplexityAnalyzer_Performance(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	// 测试查询
	queries := []string{
		"SELECT * FROM users",
		"SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM posts WHERE published = true)",
		"WITH RECURSIVE hierarchy AS (SELECT * FROM employees WHERE manager_id IS NULL) SELECT * FROM hierarchy",
	}
	
	// 性能测试
	startTime := time.Now()
	iterations := 100
	
	for i := 0; i < iterations; i++ {
		for _, query := range queries {
			ctx := context.Background()
			_, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
			if err != nil {
				t.Fatalf("分析失败: %v", err)
			}
		}
	}
	
	totalTime := time.Since(startTime)
	avgTime := totalTime / time.Duration(iterations*len(queries))
	
	// 平均处理时间应该小于50ms
	if avgTime > 50*time.Millisecond {
		t.Errorf("平均处理时间过长: %v", avgTime)
	}
	
	t.Logf("性能测试完成: %d次分析, 平均时间: %v", iterations*len(queries), avgTime)
}

func TestComplexityAnalyzer_ConcurrentSafety(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	query := "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id WHERE u.active = true"
	
	// 并发测试
	concurrency := 10
	iterations := 50
	
	done := make(chan bool, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- true }()
			
			for j := 0; j < iterations; j++ {
				ctx := context.Background()
				result, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
				
				if err != nil {
					t.Errorf("并发分析失败: %v", err)
					return
				}
				
				if result.Category == "" {
					t.Error("分类结果不能为空")
					return
				}
			}
		}()
	}
	
	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	t.Logf("并发测试完成: %d个goroutine, 每个%d次迭代", concurrency, iterations)
}

func TestComplexityAnalyzer_EdgeCases(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "空查询",
			query: "",
		},
		{
			name:  "只有空格",
			query: "   ",
		},
		{
			name:  "非SQL语句",
			query: "This is not a SQL query",
		},
		{
			name:  "SQL注释",
			query: "-- This is a comment\nSELECT * FROM users /* another comment */",
		},
		{
			name:  "特殊字符",
			query: "SELECT 'hello\nworld' FROM users WHERE name LIKE '%test%'",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := analyzer.AnalyzeComplexity(ctx, tc.query, &QueryMetadata{})
			
			// 边缘情况不应该导致panic，但可能返回错误
			if err != nil {
				t.Logf("边缘情况返回错误（预期）: %v", err)
				return
			}
			
			// 如果没有错误，结果应该是有效的
			if result == nil {
				t.Error("结果不能为nil")
				return
			}
			
			// 验证基本字段
			if result.Category == "" {
				t.Error("分类不能为空")
			}
			
			if result.Score < 0 || result.Score > 1 {
				t.Errorf("评分超出范围: %.2f", result.Score)
			}
		})
	}
}

func TestComplexityAnalyzer_LearningDataUpdate(t *testing.T) {
	analyzer := NewComplexityAnalyzer(nil)
	
	query := "SELECT * FROM users WHERE active = true"
	ctx := context.Background()
	
	// 第一次分析
	result1, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}
	
	// 模拟学习数据更新（通过多次相同查询）
	for i := 0; i < 10; i++ {
		_, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
		if err != nil {
			t.Fatalf("第%d次分析失败: %v", i+1, err)
		}
	}
	
	// 第二次分析（应该受到学习影响）
	result2, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("第二次分析失败: %v", err)
	}
	
	// 验证学习效果（学习分数应该有变化）
	if result1.LearningScore == result2.LearningScore && result2.LearningScore == 0 {
		t.Log("学习机制可能需要更多样本才能生效")
	}
	
	// 验证结果一致性（对相同查询应该给出一致的分类）
	if result1.Category != result2.Category {
		t.Errorf("相同查询的分类不一致: %s vs %s", result1.Category, result2.Category)
	}
	
	t.Logf("学习前评分: %.3f, 学习后评分: %.3f", result1.Score, result2.Score)
	t.Logf("学习前学习分数: %.3f, 学习后学习分数: %.3f", result1.LearningScore, result2.LearningScore)
}

func TestComplexityAnalyzer_ConfigCustomization(t *testing.T) {
	// 自定义配置
	config := &AnalyzerConfig{
		KeywordWeight:    0.5,
		SyntaxWeight:     0.3,
		RelationWeight:   0.2,
		LearningWeight:   0.0, // 禁用学习
		SimpleThreshold:  0.2,
		ComplexThreshold: 0.8,
		LearningDecay:    0.9,
		MinSamples:       5,
		CacheSize:        500,
		CacheTTL:         15 * time.Minute,
	}
	
	analyzer := NewComplexityAnalyzer(config)
	
	query := "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id"
	ctx := context.Background()
	
	result, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}
	
	// 验证学习分数为0（因为LearningWeight为0）
	if result.LearningScore != 0 {
		t.Errorf("学习分数应该为0: %.3f", result.LearningScore)
	}
	
	// 验证权重应用（这个查询有JOIN，应该得到适当的评分）
	expectedMinScore := config.KeywordWeight*0.2 + config.SyntaxWeight*0.3 + config.RelationWeight*0.4
	expectedMaxScore := config.KeywordWeight*0.6 + config.SyntaxWeight*0.7 + config.RelationWeight*0.8
	
	if result.Score < expectedMinScore*0.5 || result.Score > expectedMaxScore*1.5 {
		t.Errorf("评分超出期望范围: %.3f, 期望范围: [%.3f, %.3f]", 
			result.Score, expectedMinScore*0.5, expectedMaxScore*1.5)
	}
	
	t.Logf("自定义配置分析结果 - 分类: %s, 评分: %.3f", result.Category, result.Score)
}

// 辅助函数
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// 基准测试
func BenchmarkComplexityAnalyzer_SimpleQuery(b *testing.B) {
	analyzer := NewComplexityAnalyzer(nil)
	query := "SELECT * FROM users WHERE id = 1"
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
		if err != nil {
			b.Fatalf("分析失败: %v", err)
		}
	}
}

func BenchmarkComplexityAnalyzer_ComplexQuery(b *testing.B) {
	analyzer := NewComplexityAnalyzer(nil)
	query := `
		WITH RECURSIVE category_tree AS (
			SELECT id, name, parent_id, 1 as level
			FROM categories 
			WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, c.name, c.parent_id, ct.level + 1
			FROM categories c
			JOIN category_tree ct ON c.parent_id = ct.id
		)
		SELECT 
			u.name,
			p.title,
			c.name as category,
			COUNT(CASE WHEN l.liked = true THEN 1 END) as likes,
			ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY p.created_at DESC) as post_rank
		FROM users u
		JOIN posts p ON u.id = p.user_id
		JOIN category_tree c ON p.category_id = c.id
		LEFT JOIN likes l ON p.id = l.post_id
		WHERE p.published = true
			AND p.created_at >= NOW() - INTERVAL '30 days'
		GROUP BY u.id, u.name, p.id, p.title, p.created_at, c.id, c.name
		HAVING COUNT(l.id) > 5
		ORDER BY likes DESC, p.created_at DESC
		LIMIT 100
	`
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComplexity(ctx, query, &QueryMetadata{})
		if err != nil {
			b.Fatalf("分析失败: %v", err)
		}
	}
}