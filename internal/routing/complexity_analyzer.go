// P2阶段 Day 3-4: 查询复杂度分析引擎
// 基于关键词、语法结构、表关联的多维度复杂度评估算法
// 实现简单/中等/复杂查询的自动分类，为智能路由提供决策依据

package routing

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ComplexityAnalyzer 查询复杂度分析引擎
type ComplexityAnalyzer struct {
	// 关键词分析器
	keywordAnalyzer *KeywordAnalyzer
	
	// 语法结构分析器
	syntaxAnalyzer *SyntaxAnalyzer
	
	// 表关联分析器
	relationAnalyzer *RelationAnalyzer
	
	// 历史学习数据
	learningData *LearningData
	
	// 并发控制
	mu sync.RWMutex
	
	// 配置参数
	config *AnalyzerConfig
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	// 权重配置
	KeywordWeight   float64 `yaml:"keyword_weight" json:"keyword_weight"`       // 关键词权重
	SyntaxWeight    float64 `yaml:"syntax_weight" json:"syntax_weight"`         // 语法权重
	RelationWeight  float64 `yaml:"relation_weight" json:"relation_weight"`     // 关联权重
	LearningWeight  float64 `yaml:"learning_weight" json:"learning_weight"`     // 学习权重
	
	// 分类阈值
	SimpleThreshold  float64 `yaml:"simple_threshold" json:"simple_threshold"`   // 简单查询阈值
	ComplexThreshold float64 `yaml:"complex_threshold" json:"complex_threshold"` // 复杂查询阈值
	
	// 学习参数
	LearningDecay   float64 `yaml:"learning_decay" json:"learning_decay"`       // 学习衰减因子
	MinSamples     int     `yaml:"min_samples" json:"min_samples"`             // 最小样本数
	
	// 性能参数
	CacheSize      int           `yaml:"cache_size" json:"cache_size"`           // 缓存大小
	CacheTTL       time.Duration `yaml:"cache_ttl" json:"cache_ttl"`             // 缓存过期时间
}

// ComplexityResult 复杂度分析结果
type ComplexityResult struct {
	// 基本信息
	Query        string              `json:"query"`
	Category     ComplexityCategory  `json:"category"`
	Score        float64             `json:"score"`
	Confidence   float64             `json:"confidence"`
	
	// 详细分析
	KeywordScore  float64            `json:"keyword_score"`
	SyntaxScore   float64            `json:"syntax_score"`
	RelationScore float64            `json:"relation_score"`
	LearningScore float64            `json:"learning_score"`
	
	// 分析详情
	Details      *AnalysisDetails    `json:"details"`
	
	// 时间戳
	AnalyzedAt   time.Time           `json:"analyzed_at"`
	ProcessTime  time.Duration       `json:"process_time"`
}

// AnalysisDetails 分析详情
type AnalysisDetails struct {
	// 关键词分析
	KeywordMatches    []string            `json:"keyword_matches"`
	ComplexKeywords   []string            `json:"complex_keywords"`
	FunctionCount     int                 `json:"function_count"`
	
	// 语法结构分析
	ClauseCount       int                 `json:"clause_count"`
	SubqueryCount     int                 `json:"subquery_count"`
	JoinCount         int                 `json:"join_count"`
	NestedLevel       int                 `json:"nested_level"`
	
	// 表关联分析
	TableCount        int                 `json:"table_count"`
	RelationCount     int                 `json:"relation_count"`
	CrossJoins        int                 `json:"cross_joins"`
	
	// 额外特征
	HasWindowFunction bool                `json:"has_window_function"`
	HasRecursiveCTE   bool                `json:"has_recursive_cte"`
	HasComplexWhere   bool                `json:"has_complex_where"`
	
	// 学习相关
	SimilarQueries    []string            `json:"similar_queries,omitempty"`
	HistoryPattern    string              `json:"history_pattern,omitempty"`
}

// KeywordAnalyzer 关键词分析器
type KeywordAnalyzer struct {
	// 简单查询关键词 (权重: 0.1-0.3)
	SimpleKeywords map[string]float64
	
	// 中等复杂度关键词 (权重: 0.3-0.6)
	MediumKeywords map[string]float64
	
	// 复杂查询关键词 (权重: 0.6-1.0)
	ComplexKeywords map[string]float64
	
	// 函数复杂度映射
	FunctionComplexity map[string]float64
}

// SyntaxAnalyzer 语法结构分析器
type SyntaxAnalyzer struct {
	// 子句识别正则
	clausePatterns map[string]*regexp.Regexp
	
	// 子查询识别正则
	subqueryPattern *regexp.Regexp
	
	// JOIN类型识别
	joinPatterns map[string]*regexp.Regexp
	
	// 嵌套层级计算
	nestingPatterns []*regexp.Regexp
}

// RelationAnalyzer 表关联分析器
type RelationAnalyzer struct {
	// 表名识别正则
	tablePattern *regexp.Regexp
	
	// JOIN关系识别
	relationPatterns map[string]*regexp.Regexp
	
	// 复杂关联模式
	complexPatterns []*regexp.Regexp
}

// LearningData 学习数据存储
type LearningData struct {
	// 历史查询记录
	queryHistory map[string]*QueryRecord
	
	// 分类统计
	categoryStats map[ComplexityCategory]*CategoryStats
	
	// 相似查询索引
	similarityIndex *SimilarityIndex
	
	// 并发控制
	mu sync.RWMutex
}

// QueryRecord 查询记录
type QueryRecord struct {
	Query         string              `json:"query"`
	Category      ComplexityCategory  `json:"category"`
	ActualScore   float64             `json:"actual_score"`
	PredictedScore float64            `json:"predicted_score"`
	Feedback      float64             `json:"feedback"`
	Timestamp     time.Time           `json:"timestamp"`
	UsageCount    int                 `json:"usage_count"`
}

// CategoryStats 分类统计
type CategoryStats struct {
	Count       int64     `json:"count"`
	AvgScore    float64   `json:"avg_score"`
	Accuracy    float64   `json:"accuracy"`
	LastUpdated time.Time `json:"last_updated"`
}

// SimilarityIndex 相似性索引
type SimilarityIndex struct {
	// 基于编辑距离的索引
	editDistanceIndex map[string][]string
	
	// 基于关键词的索引  
	keywordIndex map[string][]string
	
	// 基于结构的索引
	structureIndex map[string][]string
}

// NewComplexityAnalyzer 创建复杂度分析器
func NewComplexityAnalyzer(config *AnalyzerConfig) *ComplexityAnalyzer {
	if config == nil {
		config = getDefaultAnalyzerConfig()
	}
	
	analyzer := &ComplexityAnalyzer{
		keywordAnalyzer:  newKeywordAnalyzer(),
		syntaxAnalyzer:   newSyntaxAnalyzer(),
		relationAnalyzer: newRelationAnalyzer(),
		learningData:     newLearningData(),
		config:           config,
	}
	
	return analyzer
}

// AnalyzeComplexity 分析查询复杂度 - 主要入口函数
func (ca *ComplexityAnalyzer) AnalyzeComplexity(ctx context.Context, query string, metadata *QueryMetadata) (*ComplexityResult, error) {
	start := time.Now()
	
	// 预处理查询
	normalizedQuery := ca.normalizeQuery(query)
	
	// 并行执行各维度分析
	results := make(chan analysisResult, 4)
	errors := make(chan error, 4)
	
	// 启动并行分析
	go ca.analyzeKeywords(normalizedQuery, results, errors)
	go ca.analyzeSyntax(normalizedQuery, results, errors)
	go ca.analyzeRelations(normalizedQuery, metadata, results, errors)
	go ca.applyLearning(normalizedQuery, results, errors)
	
	// 收集结果
	var keywordScore, syntaxScore, relationScore, learningScore float64
	var details *AnalysisDetails
	collectedResults := 0
	
	for collectedResults < 4 {
		select {
		case result := <-results:
			collectedResults++
			switch result.Type {
			case "keyword":
				keywordScore = result.Score
				if details == nil {
					details = &AnalysisDetails{}
				}
				details.KeywordMatches = result.KeywordMatches
				details.ComplexKeywords = result.ComplexKeywords
				details.FunctionCount = result.FunctionCount
			case "syntax":
				syntaxScore = result.Score
				if details == nil {
					details = &AnalysisDetails{}
				}
				details.ClauseCount = result.ClauseCount
				details.SubqueryCount = result.SubqueryCount
				details.JoinCount = result.JoinCount
				details.NestedLevel = result.NestedLevel
				details.HasWindowFunction = result.HasWindowFunction
				details.HasRecursiveCTE = result.HasRecursiveCTE
				details.HasComplexWhere = result.HasComplexWhere
			case "relation":
				relationScore = result.Score
				if details == nil {
					details = &AnalysisDetails{}
				}
				details.TableCount = result.TableCount
				details.RelationCount = result.RelationCount
				details.CrossJoins = result.CrossJoins
			case "learning":
				learningScore = result.Score
				if details == nil {
					details = &AnalysisDetails{}
				}
				details.SimilarQueries = result.SimilarQueries
				details.HistoryPattern = result.HistoryPattern
			}
		case err := <-errors:
			collectedResults++
			// 记录错误但不中断分析
			fmt.Printf("分析子任务错误: %v\n", err)
		case <-ctx.Done():
			return nil, fmt.Errorf("分析超时")
		}
	}
	
	// 计算综合评分
	finalScore := ca.calculateFinalScore(keywordScore, syntaxScore, relationScore, learningScore)
	
	// 确定分类
	category := ca.categorizeQuery(finalScore)
	
	// 计算置信度
	confidence := ca.calculateConfidence(keywordScore, syntaxScore, relationScore, learningScore)
	
	result := &ComplexityResult{
		Query:         query,
		Category:      category,
		Score:         finalScore,
		Confidence:    confidence,
		KeywordScore:  keywordScore,
		SyntaxScore:   syntaxScore,
		RelationScore: relationScore,
		LearningScore: learningScore,
		Details:       details,
		AnalyzedAt:    start,
		ProcessTime:   time.Since(start),
	}
	
	// 更新学习数据
	go ca.updateLearningData(normalizedQuery, result)
	
	return result, nil
}

// analysisResult 分析结果结构
type analysisResult struct {
	Type              string
	Score             float64
	KeywordMatches    []string
	ComplexKeywords   []string
	FunctionCount     int
	ClauseCount       int
	SubqueryCount     int
	JoinCount         int
	NestedLevel       int
	HasWindowFunction bool
	HasRecursiveCTE   bool
	HasComplexWhere   bool
	TableCount        int
	RelationCount     int
	CrossJoins        int
	SimilarQueries    []string
	HistoryPattern    string
}

// QueryMetadata 查询元数据
type QueryMetadata struct {
	DatabaseName string   `json:"database_name"`
	TableNames   []string `json:"table_names"`
	SchemaInfo   string   `json:"schema_info"`
	UserID       int64    `json:"user_id"`
}

// 分析器初始化函数
func newKeywordAnalyzer() *KeywordAnalyzer {
	return &KeywordAnalyzer{
		SimpleKeywords: map[string]float64{
			"select":  0.1,
			"from":    0.1,
			"where":   0.2,
			"order":   0.15,
			"limit":   0.1,
			"count":   0.15,
			"sum":     0.15,
			"avg":     0.15,
			"max":     0.15,
			"min":     0.15,
		},
		MediumKeywords: map[string]float64{
			"join":      0.4,
			"inner":     0.35,
			"left":      0.35,
			"right":     0.35,
			"group":     0.4,
			"having":    0.45,
			"distinct":  0.3,
			"union":     0.5,
			"case":      0.4,
			"when":      0.4,
		},
		ComplexKeywords: map[string]float64{
			"window":    0.8,
			"partition": 0.7,
			"recursive": 0.9,
			"with":      0.6,
			"exists":    0.7,
			"not exists": 0.7,
			"cross":     0.8,
			"pivot":     0.8,
			"unpivot":   0.8,
			"over":      0.8,
		},
		FunctionComplexity: map[string]float64{
			// 基础聚合函数
			"count":      0.3,
			"sum":        0.3,
			"avg":        0.3,
			"min":        0.2,
			"max":        0.2,
			"group_concat": 0.4,
			"string_agg":   0.4,
			// 窗口函数
			"row_number": 0.7,
			"rank":       0.7,
			"dense_rank": 0.7,
			"lag":        0.8,
			"lead":       0.8,
			"ntile":      0.8,
			"cume_dist":  0.9,
			"percent_rank": 0.9,
			// 数学函数
			"abs":        0.1,
			"round":      0.1,
			"ceil":       0.1,
			"floor":      0.1,
			// 日期函数
			"now":        0.2,
			"date":       0.2,
			"extract":    0.3,
			"date_part":  0.3,
		},
	}
}

func newSyntaxAnalyzer() *SyntaxAnalyzer {
	return &SyntaxAnalyzer{
		clausePatterns: map[string]*regexp.Regexp{
			"select":  regexp.MustCompile(`(?i)\bselect\b`),
			"from":    regexp.MustCompile(`(?i)\bfrom\b`),
			"where":   regexp.MustCompile(`(?i)\bwhere\b`),
			"group":   regexp.MustCompile(`(?i)\bgroup\s+by\b`),
			"having":  regexp.MustCompile(`(?i)\bhaving\b`),
			"order":   regexp.MustCompile(`(?i)\border\s+by\b`),
			"limit":   regexp.MustCompile(`(?i)\blimit\b`),
		},
		subqueryPattern: regexp.MustCompile(`\([^)]*\bselect\b[^)]*\)`),
		joinPatterns: map[string]*regexp.Regexp{
			"inner": regexp.MustCompile(`(?i)\binner\s+join\b`),
			"left":  regexp.MustCompile(`(?i)\bleft\s+(outer\s+)?join\b`),
			"right": regexp.MustCompile(`(?i)\bright\s+(outer\s+)?join\b`),
			"full":  regexp.MustCompile(`(?i)\bfull\s+(outer\s+)?join\b`),
			"cross": regexp.MustCompile(`(?i)\bcross\s+join\b`),
		},
		nestingPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\([^)]*\([^)]*\)[^)]*\)`), // 两层嵌套
			regexp.MustCompile(`\([^)]*\([^)]*\([^)]*\)[^)]*\)[^)]*\)`), // 三层嵌套
		},
	}
}

func newRelationAnalyzer() *RelationAnalyzer {
	return &RelationAnalyzer{
		tablePattern: regexp.MustCompile(`(?i)\bfrom\s+(\w+)|join\s+(\w+)`),
		relationPatterns: map[string]*regexp.Regexp{
			"join":   regexp.MustCompile(`(?i)\bjoin\b`),
			"on":     regexp.MustCompile(`(?i)\bon\s+\w+\.\w+\s*=\s*\w+\.\w+`),
			"using":  regexp.MustCompile(`(?i)\busing\s*\(`),
		},
		complexPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\bcross\s+join\b`), // 交叉连接
			regexp.MustCompile(`(?i)\bjoin\s+.*\bjoin\b.*\bjoin\b`), // 多表连接
		},
	}
}

func newLearningData() *LearningData {
	return &LearningData{
		queryHistory:    make(map[string]*QueryRecord),
		categoryStats:   make(map[ComplexityCategory]*CategoryStats),
		similarityIndex: &SimilarityIndex{
			editDistanceIndex: make(map[string][]string),
			keywordIndex:      make(map[string][]string),
			structureIndex:    make(map[string][]string),
		},
	}
}

func getDefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		KeywordWeight:    0.25,
		SyntaxWeight:     0.35,
		RelationWeight:   0.25,
		LearningWeight:   0.15,
		SimpleThreshold:  0.3,
		ComplexThreshold: 0.7,
		LearningDecay:    0.9,
		MinSamples:       10,
		CacheSize:        1000,
		CacheTTL:         30 * time.Minute,
	}
}

// 实现各种分析方法...
func (ca *ComplexityAnalyzer) analyzeKeywords(query string, results chan<- analysisResult, errors chan<- error) {
	// 关键词分析实现
	defer func() {
		if r := recover(); r != nil {
			errors <- fmt.Errorf("关键词分析异常: %v", r)
		}
	}()
	
	var score float64
	var matches, complexKeywords []string
	functionCount := 0
	
	queryLower := strings.ToLower(query)
	
	// 分析简单关键词
	for keyword, weight := range ca.keywordAnalyzer.SimpleKeywords {
		if strings.Contains(queryLower, keyword) {
			score += weight
			matches = append(matches, keyword)
		}
	}
	
	// 分析中等复杂度关键词
	for keyword, weight := range ca.keywordAnalyzer.MediumKeywords {
		if strings.Contains(queryLower, keyword) {
			score += weight
			matches = append(matches, keyword)
		}
	}
	
	// 分析复杂关键词
	for keyword, weight := range ca.keywordAnalyzer.ComplexKeywords {
		if strings.Contains(queryLower, keyword) {
			score += weight
			matches = append(matches, keyword)
			complexKeywords = append(complexKeywords, keyword)
		}
	}
	
	// 分析函数复杂度
	for function, weight := range ca.keywordAnalyzer.FunctionComplexity {
		if strings.Contains(queryLower, function) {
			score += weight
			functionCount++
		}
	}
	
	// 标准化评分
	score = math.Min(score, 1.0)
	
	results <- analysisResult{
		Type:            "keyword",
		Score:           score,
		KeywordMatches:  matches,
		ComplexKeywords: complexKeywords,
		FunctionCount:   functionCount,
	}
}

func (ca *ComplexityAnalyzer) analyzeSyntax(query string, results chan<- analysisResult, errors chan<- error) {
	// 语法结构分析实现
	defer func() {
		if r := recover(); r != nil {
			errors <- fmt.Errorf("语法分析异常: %v", r)
		}
	}()
	
	var score float64
	clauseCount := 0
	joinCount := 0
	nestedLevel := 0
	
	// 统计子句数量
	for _, pattern := range ca.syntaxAnalyzer.clausePatterns {
		if pattern.MatchString(query) {
			clauseCount++
		}
	}
	
	// 统计JOIN数量
	for _, pattern := range ca.syntaxAnalyzer.joinPatterns {
		matches := pattern.FindAllString(query, -1)
		joinCount += len(matches)
	}
	
	// 统计子查询数量
	subqueries := ca.syntaxAnalyzer.subqueryPattern.FindAllString(query, -1)
	subqueryCount := len(subqueries)
	
	// 计算嵌套层级
	for i, pattern := range ca.syntaxAnalyzer.nestingPatterns {
		if pattern.MatchString(query) {
			nestedLevel = i + 2 // 从2层开始
		}
	}
	
	// 检测窗口函数
	hasWindowFunction := strings.Contains(strings.ToLower(query), "over")
	
	// 检测递归CTE
	hasRecursiveCTE := strings.Contains(strings.ToLower(query), "with recursive")
	
	// 检测复杂WHERE条件
	hasComplexWhere := ca.detectComplexWhere(query)
	
	// 计算语法复杂度评分
	score = ca.calculateSyntaxScore(clauseCount, subqueryCount, joinCount, nestedLevel, 
		hasWindowFunction, hasRecursiveCTE, hasComplexWhere)
	
	results <- analysisResult{
		Type:              "syntax",
		Score:             score,
		ClauseCount:       clauseCount,
		SubqueryCount:     subqueryCount,
		JoinCount:         joinCount,
		NestedLevel:       nestedLevel,
		HasWindowFunction: hasWindowFunction,
		HasRecursiveCTE:   hasRecursiveCTE,
		HasComplexWhere:   hasComplexWhere,
	}
}

func (ca *ComplexityAnalyzer) analyzeRelations(query string, metadata *QueryMetadata, results chan<- analysisResult, errors chan<- error) {
	// 表关联分析实现
	defer func() {
		if r := recover(); r != nil {
			errors <- fmt.Errorf("关联分析异常: %v", r)
		}
	}()
	
	var score float64
	tableCount := 0
	relationCount := 0
	crossJoins := 0
	
	// 分析表数量
	tableMatches := ca.relationAnalyzer.tablePattern.FindAllStringSubmatch(query, -1)
	tableSet := make(map[string]bool)
	for _, match := range tableMatches {
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				tableSet[match[i]] = true
			}
		}
	}
	tableCount = len(tableSet)
	
	// 分析关联关系
	for _, pattern := range ca.relationAnalyzer.relationPatterns {
		matches := pattern.FindAllString(query, -1)
		relationCount += len(matches)
	}
	
	// 检测复杂关联模式
	for _, pattern := range ca.relationAnalyzer.complexPatterns {
		if strings.Contains(pattern.String(), "cross") && pattern.MatchString(query) {
			crossJoins++
		}
	}
	
	// 计算关联复杂度评分
	score = ca.calculateRelationScore(tableCount, relationCount, crossJoins)
	
	results <- analysisResult{
		Type:          "relation",
		Score:         score,
		TableCount:    tableCount,
		RelationCount: relationCount,
		CrossJoins:    crossJoins,
	}
}

func (ca *ComplexityAnalyzer) applyLearning(query string, results chan<- analysisResult, errors chan<- error) {
	// 历史学习机制实现
	defer func() {
		if r := recover(); r != nil {
			errors <- fmt.Errorf("学习分析异常: %v", r)
		}
	}()
	
	ca.learningData.mu.RLock()
	defer ca.learningData.mu.RUnlock()
	
	var score float64
	var similarQueries []string
	historyPattern := ""
	
	// 查找相似查询
	normalizedQuery := ca.normalizeQuery(query)
	similarQueries = ca.findSimilarQueries(normalizedQuery, 5)
	
	// 基于历史数据调整评分
	if len(similarQueries) > 0 {
		var totalScore float64
		validSamples := 0
		
		for _, similarQuery := range similarQueries {
			if record, exists := ca.learningData.queryHistory[similarQuery]; exists {
				totalScore += record.ActualScore
				validSamples++
			}
		}
		
		if validSamples >= ca.config.MinSamples {
			score = totalScore / float64(validSamples)
			historyPattern = fmt.Sprintf("基于%d个历史样本", validSamples)
		}
	}
	
	results <- analysisResult{
		Type:           "learning",
		Score:          score,
		SimilarQueries: similarQueries,
		HistoryPattern: historyPattern,
	}
}

// 辅助方法实现
func (ca *ComplexityAnalyzer) normalizeQuery(query string) string {
	// 移除多余空格和注释
	query = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(query), " ")
	query = regexp.MustCompile(`--.*$`).ReplaceAllString(query, "")
	query = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(query, "")
	return strings.ToLower(query)
}

func (ca *ComplexityAnalyzer) calculateFinalScore(keywordScore, syntaxScore, relationScore, learningScore float64) float64 {
	return ca.config.KeywordWeight*keywordScore +
		   ca.config.SyntaxWeight*syntaxScore +
		   ca.config.RelationWeight*relationScore +
		   ca.config.LearningWeight*learningScore
}

func (ca *ComplexityAnalyzer) categorizeQuery(score float64) ComplexityCategory {
	if score <= ca.config.SimpleThreshold {
		return CategorySimple
	} else if score <= ca.config.ComplexThreshold {
		return CategoryMedium
	} else {
		return CategoryComplex
	}
}

func (ca *ComplexityAnalyzer) calculateConfidence(keywordScore, syntaxScore, relationScore, learningScore float64) float64 {
	// 计算各维度评分的一致性，一致性越高置信度越高
	scores := []float64{keywordScore, syntaxScore, relationScore}
	if learningScore > 0 {
		scores = append(scores, learningScore)
	}
	
	// 计算标准差
	mean := 0.0
	for _, score := range scores {
		mean += score
	}
	mean /= float64(len(scores))
	
	variance := 0.0
	for _, score := range scores {
		variance += math.Pow(score-mean, 2)
	}
	variance /= float64(len(scores))
	stddev := math.Sqrt(variance)
	
	// 标准差越小，置信度越高
	confidence := 1.0 - stddev
	return math.Max(0.0, math.Min(1.0, confidence))
}

func (ca *ComplexityAnalyzer) detectComplexWhere(query string) bool {
	complexPatterns := []string{
		`(?i)\bexists\s*\(`,
		`(?i)\bnot\s+exists\s*\(`,
		`(?i)\bin\s*\(.*select`,
		`(?i)\bnot\s+in\s*\(.*select`,
		`(?i)\bcase\s+when`,
		`(?i)\band\s+.*\bor\b`,
		`(?i)\bor\s+.*\band\b`,
	}
	
	for _, pattern := range complexPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			return true
		}
	}
	return false
}

func (ca *ComplexityAnalyzer) calculateSyntaxScore(clauseCount, subqueryCount, joinCount, nestedLevel int, 
	hasWindowFunction, hasRecursiveCTE, hasComplexWhere bool) float64 {
	
	score := 0.0
	
	// 基于子句数量
	score += float64(clauseCount) * 0.05
	
	// 基于子查询
	score += float64(subqueryCount) * 0.2
	
	// 基于JOIN数量
	score += float64(joinCount) * 0.15
	
	// 基于嵌套层级
	score += float64(nestedLevel) * 0.25
	
	// 特殊功能加权
	if hasWindowFunction {
		score += 0.3
	}
	if hasRecursiveCTE {
		score += 0.4
	}
	if hasComplexWhere {
		score += 0.2
	}
	
	return math.Min(score, 1.0)
}

func (ca *ComplexityAnalyzer) calculateRelationScore(tableCount, relationCount, crossJoins int) float64 {
	score := 0.0
	
	// 表数量影响
	score += float64(tableCount) * 0.1
	
	// 关联数量影响
	score += float64(relationCount) * 0.15
	
	// 交叉连接惩罚
	score += float64(crossJoins) * 0.4
	
	// 多表连接复杂度
	if tableCount > 3 {
		score += float64(tableCount-3) * 0.2
	}
	
	return math.Min(score, 1.0)
}

func (ca *ComplexityAnalyzer) findSimilarQueries(query string, limit int) []string {
	// 简单的相似查询查找实现
	// 在生产环境中可以使用更高级的相似度算法
	var similar []string
	for histQuery := range ca.learningData.queryHistory {
		if ca.calculateEditDistance(query, histQuery) < len(query)/3 {
			similar = append(similar, histQuery)
			if len(similar) >= limit {
				break
			}
		}
	}
	return similar
}

func (ca *ComplexityAnalyzer) calculateEditDistance(s1, s2 string) int {
	// 简单的编辑距离实现
	m, n := len(s1), len(s2)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := 1; j <= n; j++ {
		dp[0][j] = j
	}
	
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[m][n]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func (ca *ComplexityAnalyzer) updateLearningData(query string, result *ComplexityResult) {
	ca.learningData.mu.Lock()
	defer ca.learningData.mu.Unlock()
	
	// 更新查询历史
	record := &QueryRecord{
		Query:         query,
		Category:      result.Category,
		PredictedScore: result.Score,
		Timestamp:     time.Now(),
		UsageCount:    1,
	}
	
	if existing, exists := ca.learningData.queryHistory[query]; exists {
		existing.UsageCount++
		existing.Timestamp = time.Now()
	} else {
		ca.learningData.queryHistory[query] = record
	}
	
	// 更新分类统计
	if stats, exists := ca.learningData.categoryStats[result.Category]; exists {
		stats.Count++
		stats.LastUpdated = time.Now()
	} else {
		ca.learningData.categoryStats[result.Category] = &CategoryStats{
			Count:       1,
			LastUpdated: time.Now(),
		}
	}
}

// GetAnalyzerStats 获取分析器统计信息
func (ca *ComplexityAnalyzer) GetAnalyzerStats() map[string]interface{} {
	ca.learningData.mu.RLock()
	defer ca.learningData.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_queries": len(ca.learningData.queryHistory),
		"category_stats": ca.learningData.categoryStats,
		"config": ca.config,
	}
	
	return stats
}