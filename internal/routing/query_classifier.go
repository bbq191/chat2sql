// P2阶段 Day 3-4: 查询分类器实现
// 基于复杂度分析引擎的简单/中等/复杂查询自动分类系统
// 集成机器学习算法，支持在线学习和分类准确率优化

package routing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// QueryClassifier 查询分类器
type QueryClassifier struct {
	// 复杂度分析器
	complexityAnalyzer *ComplexityAnalyzer
	
	// 分类模型
	classificationModel *ClassificationModel
	
	// 特征提取器
	featureExtractor *FeatureExtractor
	
	// 分类缓存
	classificationCache *ClassificationCache
	
	// 分类统计
	classificationStats *ClassificationStats
	
	// 并发控制
	mu sync.RWMutex
	
	// 配置
	config *ClassifierConfig
}

// ClassifierConfig 分类器配置
type ClassifierConfig struct {
	// 分类阈值（可动态调整）
	SimpleThreshold  float64 `yaml:"simple_threshold" json:"simple_threshold"`
	ComplexThreshold float64 `yaml:"complex_threshold" json:"complex_threshold"`
	
	// 机器学习参数
	LearningRate       float64 `yaml:"learning_rate" json:"learning_rate"`
	FeatureWeights     map[string]float64 `yaml:"feature_weights" json:"feature_weights"`
	DecayFactor       float64 `yaml:"decay_factor" json:"decay_factor"`
	
	// 缓存配置
	CacheSize         int           `yaml:"cache_size" json:"cache_size"`
	CacheTTL         time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	
	// 性能配置
	BatchSize        int  `yaml:"batch_size" json:"batch_size"`
	EnableCache      bool `yaml:"enable_cache" json:"enable_cache"`
	EnableLearning   bool `yaml:"enable_learning" json:"enable_learning"`
	
	// 统计配置
	StatsWindowSize  int           `yaml:"stats_window_size" json:"stats_window_size"`
	StatsUpdateInterval time.Duration `yaml:"stats_update_interval" json:"stats_update_interval"`
}

// ClassificationModel 分类模型
type ClassificationModel struct {
	// 特征权重
	featureWeights map[string]float64
	
	// 分类边界
	boundaries []*ClassificationBoundary
	
	// 学习历史
	learningHistory []*LearningRecord
	
	// 模型统计
	modelStats *ModelStats
	
	// 最后更新时间
	lastUpdated time.Time
	
	// 并发控制
	mu sync.RWMutex
}

// ClassificationBoundary 分类边界
type ClassificationBoundary struct {
	Category     ComplexityCategory `json:"category"`
	MinScore     float64           `json:"min_score"`
	MaxScore     float64           `json:"max_score"`
	Confidence   float64           `json:"confidence"`
	SampleCount  int               `json:"sample_count"`
}

// LearningRecord 学习记录
type LearningRecord struct {
	Query          string              `json:"query"`
	Features       *QueryFeatures      `json:"features"`
	PredictedCategory ComplexityCategory `json:"predicted_category"`
	ActualCategory    ComplexityCategory `json:"actual_category"`
	Confidence     float64             `json:"confidence"`
	Timestamp      time.Time           `json:"timestamp"`
	Feedback       float64             `json:"feedback"` // -1 到 1
}

// ModelStats 模型统计
type ModelStats struct {
	TotalPredictions int64             `json:"total_predictions"`
	Accuracy        float64           `json:"accuracy"`
	Precision       map[string]float64 `json:"precision"`
	Recall          map[string]float64 `json:"recall"`
	F1Score         map[string]float64 `json:"f1_score"`
	ConfusionMatrix map[string]map[string]int `json:"confusion_matrix"`
	LastUpdated     time.Time         `json:"last_updated"`
}

// FeatureExtractor 特征提取器
type FeatureExtractor struct {
	// 特征提取函数
	extractors map[string]FeatureExtractorFunc
	
	// 特征标准化参数
	normalizationParams map[string]*NormalizationParams
	
	// 配置
	config *FeatureExtractorConfig
}

// FeatureExtractorFunc 特征提取函数类型
type FeatureExtractorFunc func(query string, complexity *ComplexityResult) float64

// FeatureExtractorConfig 特征提取器配置
type FeatureExtractorConfig struct {
	EnableNormalization bool     `yaml:"enable_normalization" json:"enable_normalization"`
	SelectedFeatures   []string `yaml:"selected_features" json:"selected_features"`
	FeatureThresholds  map[string]float64 `yaml:"feature_thresholds" json:"feature_thresholds"`
}

// NormalizationParams 标准化参数
type NormalizationParams struct {
	Mean   float64 `json:"mean"`
	Std    float64 `json:"std"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// QueryFeatures 查询特征
type QueryFeatures struct {
	// 基础特征
	QueryLength      float64 `json:"query_length"`
	WordCount        float64 `json:"word_count"`
	KeywordDensity   float64 `json:"keyword_density"`
	
	// 结构特征
	ClauseComplexity float64 `json:"clause_complexity"`
	JoinComplexity   float64 `json:"join_complexity"`
	NestingDepth     float64 `json:"nesting_depth"`
	
	// 语义特征
	FunctionComplexity float64 `json:"function_complexity"`
	ConditionComplexity float64 `json:"condition_complexity"`
	
	// 关联特征
	TableComplexity    float64 `json:"table_complexity"`
	RelationComplexity float64 `json:"relation_complexity"`
	
	// 高级特征
	WindowFunctionScore float64 `json:"window_function_score"`
	RecursiveScore     float64 `json:"recursive_score"`
	SubqueryScore      float64 `json:"subquery_score"`
	
	// 组合特征
	OverallComplexity  float64 `json:"overall_complexity"`
	StructuralBalance  float64 `json:"structural_balance"`
	SemanticRichness   float64 `json:"semantic_richness"`
}

// ClassificationCache 分类缓存
type ClassificationCache struct {
	cache map[string]*CacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
	maxSize int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Result    *ClassificationResult `json:"result"`
	CreatedAt time.Time            `json:"created_at"`
	AccessCount int                `json:"access_count"`
	LastAccess  time.Time          `json:"last_access"`
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	Query       string              `json:"query"`
	Category    ComplexityCategory  `json:"category"`
	Confidence  float64             `json:"confidence"`
	Features    *QueryFeatures      `json:"features"`
	Scores      map[string]float64  `json:"scores"`
	ModelScore  float64             `json:"model_score"`
	Reasoning   string              `json:"reasoning"`
	Timestamp   time.Time           `json:"timestamp"`
	ProcessTime time.Duration       `json:"process_time"`
}

// ClassificationStats 分类统计
type ClassificationStats struct {
	// 分类统计
	totalClassifications int64
	categoryDistribution map[ComplexityCategory]int64
	
	// 性能统计
	averageProcessTime time.Duration
	cacheHitRate      float64
	accuracyRate      float64
	
	// 时间窗口统计
	recentClassifications []*ClassificationResult
	windowSize           int
	
	// 并发控制
	mu sync.RWMutex
}

// NewQueryClassifier 创建查询分类器
func NewQueryClassifier(complexityAnalyzer *ComplexityAnalyzer, config *ClassifierConfig) *QueryClassifier {
	if config == nil {
		config = getDefaultClassifierConfig()
	}
	
	classifier := &QueryClassifier{
		complexityAnalyzer:  complexityAnalyzer,
		classificationModel: newClassificationModel(config),
		featureExtractor:    newFeatureExtractor(),
		classificationCache: newClassificationCache(config.CacheSize, config.CacheTTL),
		classificationStats: newClassificationStats(config.StatsWindowSize),
		config:             config,
	}
	
	// 启动统计更新
	go classifier.startStatsUpdater()
	
	return classifier
}

// ClassifyQuery 分类查询 - 主要入口函数
func (qc *QueryClassifier) ClassifyQuery(ctx context.Context, query string, metadata *QueryMetadata) (*ClassificationResult, error) {
	start := time.Now()
	
	// 检查缓存
	if qc.config.EnableCache {
		if cached := qc.classificationCache.Get(query); cached != nil {
			cached.AccessCount++
			cached.LastAccess = time.Now()
			qc.updateStats(cached.Result)
			return cached.Result, nil
		}
	}
	
	// 执行复杂度分析
	complexityResult, err := qc.complexityAnalyzer.AnalyzeComplexity(ctx, query, metadata)
	if err != nil {
		return nil, fmt.Errorf("复杂度分析失败: %w", err)
	}
	
	// 提取特征
	features := qc.featureExtractor.ExtractFeatures(query, complexityResult)
	
	// 执行分类
	category, confidence, modelScore, reasoning := qc.classificationModel.Classify(features)
	
	// 构建结果
	result := &ClassificationResult{
		Query:       query,
		Category:    category,
		Confidence:  confidence,
		Features:    features,
		Scores: map[string]float64{
			"complexity_score": complexityResult.Score,
			"keyword_score":    complexityResult.KeywordScore,
			"syntax_score":     complexityResult.SyntaxScore,
			"relation_score":   complexityResult.RelationScore,
			"learning_score":   complexityResult.LearningScore,
			"model_score":      modelScore,
		},
		ModelScore:  modelScore,
		Reasoning:   reasoning,
		Timestamp:   start,
		ProcessTime: time.Since(start),
	}
	
	// 更新缓存
	if qc.config.EnableCache {
		qc.classificationCache.Put(query, result)
	}
	
	// 更新统计
	qc.updateStats(result)
	
	return result, nil
}

// BatchClassifyQueries 批量分类查询
func (qc *QueryClassifier) BatchClassifyQueries(ctx context.Context, queries []string, metadata *QueryMetadata) ([]*ClassificationResult, error) {
	results := make([]*ClassificationResult, len(queries))
	errors := make([]error, len(queries))
	
	// 并发处理
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, qc.config.BatchSize)
	
	for i, query := range queries {
		wg.Add(1)
		go func(index int, q string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			result, err := qc.ClassifyQuery(ctx, q, metadata)
			results[index] = result
			errors[index] = err
		}(i, query)
	}
	
	wg.Wait()
	
	// 检查错误
	var firstError error
	for _, err := range errors {
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	
	return results, firstError
}

// LearnFromFeedback 从用户反馈中学习
func (qc *QueryClassifier) LearnFromFeedback(query string, predictedCategory, actualCategory ComplexityCategory, feedback float64) error {
	if !qc.config.EnableLearning {
		return nil
	}
	
	// 重新分析查询以获取特征
	ctx := context.Background()
	complexityResult, err := qc.complexityAnalyzer.AnalyzeComplexity(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("重新分析查询失败: %w", err)
	}
	
	features := qc.featureExtractor.ExtractFeatures(query, complexityResult)
	
	// 创建学习记录
	record := &LearningRecord{
		Query:             query,
		Features:          features,
		PredictedCategory: predictedCategory,
		ActualCategory:    actualCategory,
		Confidence:        0, // 之前的置信度可能不可用
		Timestamp:         time.Now(),
		Feedback:          feedback,
	}
	
	// 更新分类模型
	return qc.classificationModel.Learn(record)
}

// UpdateThresholds 动态更新分类阈值
func (qc *QueryClassifier) UpdateThresholds(simpleThreshold, complexThreshold float64) error {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	
	if simpleThreshold >= complexThreshold {
		return fmt.Errorf("简单阈值必须小于复杂阈值")
	}
	
	qc.config.SimpleThreshold = simpleThreshold
	qc.config.ComplexThreshold = complexThreshold
	
	// 更新分类模型边界
	qc.classificationModel.UpdateBoundaries(simpleThreshold, complexThreshold)
	
	// 清理缓存以使用新阈值
	if qc.config.EnableCache {
		qc.classificationCache.Clear()
	}
	
	return nil
}

// GetClassificationStats 获取分类统计信息
func (qc *QueryClassifier) GetClassificationStats() *ClassificationStats {
	qc.classificationStats.mu.RLock()
	defer qc.classificationStats.mu.RUnlock()
	
	// 返回统计信息的副本
	stats := &ClassificationStats{
		totalClassifications: qc.classificationStats.totalClassifications,
		categoryDistribution: make(map[ComplexityCategory]int64),
		averageProcessTime:   qc.classificationStats.averageProcessTime,
		cacheHitRate:        qc.classificationStats.cacheHitRate,
		accuracyRate:        qc.classificationStats.accuracyRate,
		recentClassifications: make([]*ClassificationResult, len(qc.classificationStats.recentClassifications)),
		windowSize:          qc.classificationStats.windowSize,
	}
	
	// 复制分布统计
	for k, v := range qc.classificationStats.categoryDistribution {
		stats.categoryDistribution[k] = v
	}
	
	// 复制最近分类
	copy(stats.recentClassifications, qc.classificationStats.recentClassifications)
	
	return stats
}

// GetModelMetrics 获取模型性能指标
func (qc *QueryClassifier) GetModelMetrics() *ModelStats {
	return qc.classificationModel.GetModelStats()
}

// 实现子组件...

func newClassificationModel(config *ClassifierConfig) *ClassificationModel {
	model := &ClassificationModel{
		featureWeights: make(map[string]float64),
		boundaries: []*ClassificationBoundary{
			{Category: CategorySimple, MinScore: 0.0, MaxScore: config.SimpleThreshold, Confidence: 1.0},
			{Category: CategoryMedium, MinScore: config.SimpleThreshold, MaxScore: config.ComplexThreshold, Confidence: 1.0},
			{Category: CategoryComplex, MinScore: config.ComplexThreshold, MaxScore: 1.0, Confidence: 1.0},
		},
		learningHistory: make([]*LearningRecord, 0),
		modelStats: &ModelStats{
			Precision:       make(map[string]float64),
			Recall:          make(map[string]float64),
			F1Score:         make(map[string]float64),
			ConfusionMatrix: make(map[string]map[string]int),
		},
		lastUpdated: time.Now(),
	}
	
	// 初始化特征权重
	for feature, weight := range config.FeatureWeights {
		model.featureWeights[feature] = weight
	}
	
	return model
}

func (cm *ClassificationModel) Classify(features *QueryFeatures) (ComplexityCategory, float64, float64, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// 计算模型评分
	modelScore := cm.calculateModelScore(features)
	
	// 根据边界确定分类
	category := CategoryMedium // 默认分类
	confidence := 0.5
	reasoning := "基于默认规则"
	
	for _, boundary := range cm.boundaries {
		if modelScore >= boundary.MinScore && modelScore < boundary.MaxScore {
			category = boundary.Category
			confidence = boundary.Confidence
			reasoning = fmt.Sprintf("模型评分%.3f落在%s范围[%.3f, %.3f)", 
				modelScore, category, boundary.MinScore, boundary.MaxScore)
			break
		}
	}
	
	// 调整置信度基于特征一致性
	confidence = cm.adjustConfidenceBasedOnFeatures(features, confidence)
	
	return category, confidence, modelScore, reasoning
}

func (cm *ClassificationModel) calculateModelScore(features *QueryFeatures) float64 {
	score := 0.0
	
	// 加权特征评分
	featureValues := map[string]float64{
		"query_length":       features.QueryLength,
		"word_count":         features.WordCount,
		"keyword_density":    features.KeywordDensity,
		"clause_complexity":  features.ClauseComplexity,
		"join_complexity":    features.JoinComplexity,
		"nesting_depth":      features.NestingDepth,
		"function_complexity": features.FunctionComplexity,
		"condition_complexity": features.ConditionComplexity,
		"table_complexity":    features.TableComplexity,
		"relation_complexity": features.RelationComplexity,
		"window_function_score": features.WindowFunctionScore,
		"recursive_score":     features.RecursiveScore,
		"subquery_score":      features.SubqueryScore,
		"overall_complexity":  features.OverallComplexity,
		"structural_balance":  features.StructuralBalance,
		"semantic_richness":   features.SemanticRichness,
	}
	
	totalWeight := 0.0
	for featureName, featureValue := range featureValues {
		if weight, exists := cm.featureWeights[featureName]; exists {
			score += weight * featureValue
			totalWeight += weight
		}
	}
	
	// 标准化评分
	if totalWeight > 0 {
		score = score / totalWeight
	}
	
	return math.Min(math.Max(score, 0.0), 1.0)
}

func (cm *ClassificationModel) adjustConfidenceBasedOnFeatures(features *QueryFeatures, baseConfidence float64) float64 {
	// 基于特征一致性调整置信度
	consistency := 0.0
	
	// 检查特征间的一致性
	structuralFeatures := []float64{
		features.ClauseComplexity,
		features.JoinComplexity,
		features.NestingDepth,
	}
	
	semanticFeatures := []float64{
		features.FunctionComplexity,
		features.ConditionComplexity,
	}
	
	// 计算结构特征一致性
	structuralConsistency := calculateConsistency(structuralFeatures)
	semanticConsistency := calculateConsistency(semanticFeatures)
	
	consistency = (structuralConsistency + semanticConsistency) / 2.0
	
	// 调整置信度
	adjustedConfidence := baseConfidence * (0.5 + 0.5*consistency)
	
	return math.Min(math.Max(adjustedConfidence, 0.1), 1.0)
}

func calculateConsistency(values []float64) float64 {
	if len(values) < 2 {
		return 1.0
	}
	
	// 计算标准差作为一致性度量
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values))
	
	stddev := math.Sqrt(variance)
	
	// 标准差越小，一致性越高
	return math.Max(0.0, 1.0-stddev)
}

func (cm *ClassificationModel) Learn(record *LearningRecord) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// 添加学习记录
	cm.learningHistory = append(cm.learningHistory, record)
	
	// 限制历史记录数量
	maxHistory := 10000
	if len(cm.learningHistory) > maxHistory {
		cm.learningHistory = cm.learningHistory[len(cm.learningHistory)-maxHistory:]
	}
	
	// 根据反馈调整特征权重
	if record.Feedback != 0 {
		cm.adjustFeatureWeights(record)
	}
	
	// 更新边界（如果有足够的样本）
	if len(cm.learningHistory) > 100 {
		cm.updateBoundariesFromHistory()
	}
	
	// 更新模型统计
	cm.updateModelStats()
	
	cm.lastUpdated = time.Now()
	
	return nil
}

func (cm *ClassificationModel) adjustFeatureWeights(record *LearningRecord) {
	learningRate := 0.01
	
	// 计算当前预测与实际的差异
	expectedScore := cm.categoryToScore(record.ActualCategory)
	actualScore := cm.categoryToScore(record.PredictedCategory)
	
	error := expectedScore - actualScore
	adjustment := learningRate * record.Feedback * error
	
	// 调整与错误分类相关的特征权重
	featureValues := cm.getFeatureValues(record.Features)
	
	for featureName, featureValue := range featureValues {
		if weight, exists := cm.featureWeights[featureName]; exists {
			// 根据特征值和错误调整权重
			cm.featureWeights[featureName] = weight + adjustment*featureValue
			
			// 确保权重在合理范围内
			cm.featureWeights[featureName] = math.Max(-2.0, math.Min(2.0, cm.featureWeights[featureName]))
		}
	}
}

func (cm *ClassificationModel) categoryToScore(category ComplexityCategory) float64 {
	switch category {
	case CategorySimple:
		return 0.2
	case CategoryMedium:
		return 0.5
	case CategoryComplex:
		return 0.8
	default:
		return 0.5
	}
}

func (cm *ClassificationModel) getFeatureValues(features *QueryFeatures) map[string]float64 {
	return map[string]float64{
		"query_length":        features.QueryLength,
		"word_count":          features.WordCount,
		"keyword_density":     features.KeywordDensity,
		"clause_complexity":   features.ClauseComplexity,
		"join_complexity":     features.JoinComplexity,
		"nesting_depth":       features.NestingDepth,
		"function_complexity": features.FunctionComplexity,
		"condition_complexity": features.ConditionComplexity,
		"table_complexity":    features.TableComplexity,
		"relation_complexity": features.RelationComplexity,
		"window_function_score": features.WindowFunctionScore,
		"recursive_score":     features.RecursiveScore,
		"subquery_score":      features.SubqueryScore,
		"overall_complexity":  features.OverallComplexity,
		"structural_balance":  features.StructuralBalance,
		"semantic_richness":   features.SemanticRichness,
	}
}

func (cm *ClassificationModel) UpdateBoundaries(simpleThreshold, complexThreshold float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.boundaries = []*ClassificationBoundary{
		{Category: CategorySimple, MinScore: 0.0, MaxScore: simpleThreshold, Confidence: 1.0},
		{Category: CategoryMedium, MinScore: simpleThreshold, MaxScore: complexThreshold, Confidence: 1.0},
		{Category: CategoryComplex, MinScore: complexThreshold, MaxScore: 1.0, Confidence: 1.0},
	}
}

func (cm *ClassificationModel) updateBoundariesFromHistory() {
	// 基于历史数据调整分类边界（简化实现）
	categories := map[ComplexityCategory][]float64{
		CategorySimple:  make([]float64, 0),
		CategoryMedium:  make([]float64, 0),
		CategoryComplex: make([]float64, 0),
	}
	
	// 收集各类别的模型评分
	for _, record := range cm.learningHistory {
		if record.ActualCategory != "" {
			score := cm.calculateModelScore(record.Features)
			categories[record.ActualCategory] = append(categories[record.ActualCategory], score)
		}
	}
	
	// 计算新的边界（使用分位数）
	for _, boundary := range cm.boundaries {
		if scores, exists := categories[boundary.Category]; exists && len(scores) > 10 {
			sort.Float64s(scores)
			
			// 使用10%和90%分位数作为边界
			boundary.MinScore = scores[len(scores)/10]
			boundary.MaxScore = scores[len(scores)*9/10]
			
			// 更新置信度基于样本数量
			boundary.Confidence = math.Min(1.0, float64(len(scores))/100.0)
			boundary.SampleCount = len(scores)
		}
	}
}

func (cm *ClassificationModel) updateModelStats() {
	// 更新模型统计信息（简化实现）
	cm.modelStats.TotalPredictions = int64(len(cm.learningHistory))
	
	correctPredictions := 0
	for _, record := range cm.learningHistory {
		if record.PredictedCategory == record.ActualCategory {
			correctPredictions++
		}
	}
	
	if len(cm.learningHistory) > 0 {
		cm.modelStats.Accuracy = float64(correctPredictions) / float64(len(cm.learningHistory))
	}
	
	cm.modelStats.LastUpdated = time.Now()
}

func (cm *ClassificationModel) GetModelStats() *ModelStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// 返回统计信息的副本
	stats := &ModelStats{
		TotalPredictions: cm.modelStats.TotalPredictions,
		Accuracy:        cm.modelStats.Accuracy,
		Precision:       make(map[string]float64),
		Recall:          make(map[string]float64),
		F1Score:         make(map[string]float64),
		ConfusionMatrix: make(map[string]map[string]int),
		LastUpdated:     cm.modelStats.LastUpdated,
	}
	
	// 复制映射
	for k, v := range cm.modelStats.Precision {
		stats.Precision[k] = v
	}
	for k, v := range cm.modelStats.Recall {
		stats.Recall[k] = v
	}
	for k, v := range cm.modelStats.F1Score {
		stats.F1Score[k] = v
	}
	for k, v := range cm.modelStats.ConfusionMatrix {
		stats.ConfusionMatrix[k] = make(map[string]int)
		for k2, v2 := range v {
			stats.ConfusionMatrix[k][k2] = v2
		}
	}
	
	return stats
}

func newFeatureExtractor() *FeatureExtractor {
	extractor := &FeatureExtractor{
		extractors: make(map[string]FeatureExtractorFunc),
		normalizationParams: make(map[string]*NormalizationParams),
		config: &FeatureExtractorConfig{
			EnableNormalization: true,
			SelectedFeatures: []string{
				"query_length", "word_count", "keyword_density",
				"clause_complexity", "join_complexity", "nesting_depth",
				"function_complexity", "condition_complexity",
				"table_complexity", "relation_complexity",
				"window_function_score", "recursive_score", "subquery_score",
				"overall_complexity", "structural_balance", "semantic_richness",
			},
			FeatureThresholds: make(map[string]float64),
		},
	}
	
	// 注册特征提取器
	extractor.registerExtractors()
	
	return extractor
}

func (fe *FeatureExtractor) ExtractFeatures(query string, complexity *ComplexityResult) *QueryFeatures {
	features := &QueryFeatures{}
	
	// 提取所有特征
	features.QueryLength = fe.extractors["query_length"](query, complexity)
	features.WordCount = fe.extractors["word_count"](query, complexity)
	features.KeywordDensity = fe.extractors["keyword_density"](query, complexity)
	features.ClauseComplexity = fe.extractors["clause_complexity"](query, complexity)
	features.JoinComplexity = fe.extractors["join_complexity"](query, complexity)
	features.NestingDepth = fe.extractors["nesting_depth"](query, complexity)
	features.FunctionComplexity = fe.extractors["function_complexity"](query, complexity)
	features.ConditionComplexity = fe.extractors["condition_complexity"](query, complexity)
	features.TableComplexity = fe.extractors["table_complexity"](query, complexity)
	features.RelationComplexity = fe.extractors["relation_complexity"](query, complexity)
	features.WindowFunctionScore = fe.extractors["window_function_score"](query, complexity)
	features.RecursiveScore = fe.extractors["recursive_score"](query, complexity)
	features.SubqueryScore = fe.extractors["subquery_score"](query, complexity)
	features.OverallComplexity = fe.extractors["overall_complexity"](query, complexity)
	features.StructuralBalance = fe.extractors["structural_balance"](query, complexity)
	features.SemanticRichness = fe.extractors["semantic_richness"](query, complexity)
	
	// 标准化特征（如果启用）
	if fe.config.EnableNormalization {
		fe.normalizeFeatures(features)
	}
	
	return features
}

func (fe *FeatureExtractor) registerExtractors() {
	fe.extractors["query_length"] = func(query string, complexity *ComplexityResult) float64 {
		return math.Min(float64(len(query))/1000.0, 1.0)
	}
	
	fe.extractors["word_count"] = func(query string, complexity *ComplexityResult) float64 {
		words := len(strings.Fields(query))
		return math.Min(float64(words)/100.0, 1.0)
	}
	
	fe.extractors["keyword_density"] = func(query string, complexity *ComplexityResult) float64 {
		return complexity.KeywordScore
	}
	
	fe.extractors["clause_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.ClauseCount)/10.0, 1.0)
	}
	
	fe.extractors["join_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.JoinCount)/5.0, 1.0)
	}
	
	fe.extractors["nesting_depth"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.NestedLevel)/5.0, 1.0)
	}
	
	fe.extractors["function_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.FunctionCount)/10.0, 1.0)
	}
	
	fe.extractors["condition_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		score := 0.0
		if complexity.Details.HasComplexWhere {
			score += 0.5
		}
		return score
	}
	
	fe.extractors["table_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.TableCount)/10.0, 1.0)
	}
	
	fe.extractors["relation_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.RelationCount)/10.0, 1.0)
	}
	
	fe.extractors["window_function_score"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		if complexity.Details.HasWindowFunction {
			return 1.0
		}
		return 0.0
	}
	
	fe.extractors["recursive_score"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		if complexity.Details.HasRecursiveCTE {
			return 1.0
		}
		return 0.0
	}
	
	fe.extractors["subquery_score"] = func(query string, complexity *ComplexityResult) float64 {
		if complexity.Details == nil {
			return 0.0
		}
		return math.Min(float64(complexity.Details.SubqueryCount)/5.0, 1.0)
	}
	
	fe.extractors["overall_complexity"] = func(query string, complexity *ComplexityResult) float64 {
		return complexity.Score
	}
	
	fe.extractors["structural_balance"] = func(query string, complexity *ComplexityResult) float64 {
		// 计算结构平衡性：各部分复杂度的平衡性
		scores := []float64{
			complexity.KeywordScore,
			complexity.SyntaxScore,
			complexity.RelationScore,
		}
		return calculateConsistency(scores)
	}
	
	fe.extractors["semantic_richness"] = func(query string, complexity *ComplexityResult) float64 {
		// 计算语义丰富性：基于函数、关键词等语义元素
		richness := complexity.KeywordScore
		if complexity.Details != nil {
			richness += float64(complexity.Details.FunctionCount) * 0.1
			richness += float64(len(complexity.Details.ComplexKeywords)) * 0.05
		}
		return math.Min(richness, 1.0)
	}
}

func (fe *FeatureExtractor) normalizeFeatures(features *QueryFeatures) {
	// 简化的特征标准化实现
	// 在生产环境中，应该基于训练数据计算真实的标准化参数
	
	// 对各个特征进行min-max标准化
	features.QueryLength = math.Max(0, math.Min(1, features.QueryLength))
	features.WordCount = math.Max(0, math.Min(1, features.WordCount))
	features.KeywordDensity = math.Max(0, math.Min(1, features.KeywordDensity))
	features.ClauseComplexity = math.Max(0, math.Min(1, features.ClauseComplexity))
	features.JoinComplexity = math.Max(0, math.Min(1, features.JoinComplexity))
	features.NestingDepth = math.Max(0, math.Min(1, features.NestingDepth))
	features.FunctionComplexity = math.Max(0, math.Min(1, features.FunctionComplexity))
	features.ConditionComplexity = math.Max(0, math.Min(1, features.ConditionComplexity))
	features.TableComplexity = math.Max(0, math.Min(1, features.TableComplexity))
	features.RelationComplexity = math.Max(0, math.Min(1, features.RelationComplexity))
	features.WindowFunctionScore = math.Max(0, math.Min(1, features.WindowFunctionScore))
	features.RecursiveScore = math.Max(0, math.Min(1, features.RecursiveScore))
	features.SubqueryScore = math.Max(0, math.Min(1, features.SubqueryScore))
	features.OverallComplexity = math.Max(0, math.Min(1, features.OverallComplexity))
	features.StructuralBalance = math.Max(0, math.Min(1, features.StructuralBalance))
	features.SemanticRichness = math.Max(0, math.Min(1, features.SemanticRichness))
}

func newClassificationCache(maxSize int, ttl time.Duration) *ClassificationCache {
	return &ClassificationCache{
		cache:   make(map[string]*CacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

func (cc *ClassificationCache) Get(query string) *CacheEntry {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	entry, exists := cc.cache[query]
	if !exists {
		return nil
	}
	
	// 检查过期
	if time.Since(entry.CreatedAt) > cc.ttl {
		delete(cc.cache, query)
		return nil
	}
	
	return entry
}

func (cc *ClassificationCache) Put(query string, result *ClassificationResult) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	// 如果缓存已满，清理最少使用的条目
	if len(cc.cache) >= cc.maxSize {
		cc.evictLRU()
	}
	
	cc.cache[query] = &CacheEntry{
		Result:      result,
		CreatedAt:   time.Now(),
		AccessCount: 1,
		LastAccess:  time.Now(),
	}
}

func (cc *ClassificationCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	cc.cache = make(map[string]*CacheEntry)
}

func (cc *ClassificationCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time = time.Now()
	
	for key, entry := range cc.cache {
		if entry.LastAccess.Before(oldestTime) {
			oldestTime = entry.LastAccess
			oldestKey = key
		}
	}
	
	if oldestKey != "" {
		delete(cc.cache, oldestKey)
	}
}

func newClassificationStats(windowSize int) *ClassificationStats {
	return &ClassificationStats{
		categoryDistribution:  make(map[ComplexityCategory]int64),
		recentClassifications: make([]*ClassificationResult, 0, windowSize),
		windowSize:           windowSize,
	}
}

func (qc *QueryClassifier) updateStats(result *ClassificationResult) {
	qc.classificationStats.mu.Lock()
	defer qc.classificationStats.mu.Unlock()
	
	stats := qc.classificationStats
	
	// 更新基本统计
	stats.totalClassifications++
	stats.categoryDistribution[result.Category]++
	
	// 更新时间窗口
	stats.recentClassifications = append(stats.recentClassifications, result)
	if len(stats.recentClassifications) > stats.windowSize {
		stats.recentClassifications = stats.recentClassifications[1:]
	}
	
	// 更新平均处理时间
	if stats.totalClassifications == 1 {
		stats.averageProcessTime = result.ProcessTime
	} else {
		// 指数移动平均
		alpha := 0.1
		stats.averageProcessTime = time.Duration(
			float64(stats.averageProcessTime)*(1-alpha) + 
			float64(result.ProcessTime)*alpha)
	}
}

func (qc *QueryClassifier) startStatsUpdater() {
	// 检查间隔是否有效
	if qc.config.StatsUpdateInterval <= 0 {
		return // 如果间隔无效，直接返回不启动统计更新
	}
	
	ticker := time.NewTicker(qc.config.StatsUpdateInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		qc.updatePeriodicStats()
	}
}

func (qc *QueryClassifier) updatePeriodicStats() {
	// 计算缓存命中率
	if qc.config.EnableCache {
		// 这里需要从缓存中获取统计信息
		// 简化实现，实际应该在缓存中维护命中统计
		qc.classificationStats.mu.Lock()
		qc.classificationStats.cacheHitRate = 0.75 // 示例值
		qc.classificationStats.mu.Unlock()
	}
	
	// 更新准确率（如果有反馈数据）
	modelStats := qc.classificationModel.GetModelStats()
	qc.classificationStats.mu.Lock()
	qc.classificationStats.accuracyRate = modelStats.Accuracy
	qc.classificationStats.mu.Unlock()
}

func getDefaultClassifierConfig() *ClassifierConfig {
	return &ClassifierConfig{
		SimpleThreshold:  0.2,
		ComplexThreshold: 0.34,
		LearningRate:     0.01,
		FeatureWeights: map[string]float64{
			"query_length":        0.01,    // 总和要达到1.0
			"word_count":          0.01,
			"keyword_density":     0.02,
			"clause_complexity":   0.10,    // 子句复杂度权重（GROUP BY, HAVING等）
			"join_complexity":     0.12,    // JOIN复杂度权重  
			"nesting_depth":       0.08,
			"function_complexity": 0.07,    // 函数复杂度权重（COUNT等）
			"condition_complexity": 0.04,   // 条件复杂度权重
			"table_complexity":    0.02,
			"relation_complexity": 0.05,
			"window_function_score": 0.08,  // 窗口函数权重
			"recursive_score":     0.12,    // 递归查询权重
			"subquery_score":      0.10,    // 子查询权重
			"overall_complexity":  0.10,    // 总体复杂度权重
			"structural_balance":  0.04,    // 结构平衡权重
			"semantic_richness":   0.04,    // 语义丰富度权重
		},
		DecayFactor:         0.95,
		CacheSize:           1000,
		CacheTTL:            30 * time.Minute,
		BatchSize:           10,
		EnableCache:         true,
		EnableLearning:      true,
		StatsWindowSize:     1000,
		StatsUpdateInterval: 5 * time.Minute,
	}
}