// P2阶段 Day 3-4: 历史学习机制实现
// 基于查询历史数据的在线学习引擎，持续优化分类准确率
// 集成多种学习算法：协同过滤、模式识别、反馈学习

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

// LearningEngine 历史学习引擎
type LearningEngine struct {
	// 查询历史存储
	historyStore *QueryHistoryStore
	
	// 模式识别器
	patternRecognizer *PatternRecognizer
	
	// 反馈学习器
	feedbackLearner *FeedbackLearner
	
	// 相似性匹配器
	similarityMatcher *SimilarityMatcher
	
	// 学习统计
	learningStats *LearningStats
	
	// 并发控制
	mu sync.RWMutex
	
	// 配置
	config *LearningConfig
	
	// 生命周期控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LearningConfig 学习引擎配置
type LearningConfig struct {
	// 历史数据配置
	MaxHistorySize     int           `yaml:"max_history_size" json:"max_history_size"`
	HistoryRetention   time.Duration `yaml:"history_retention" json:"history_retention"`
	
	// 学习参数
	LearningRate       float64 `yaml:"learning_rate" json:"learning_rate"`
	DecayRate          float64 `yaml:"decay_rate" json:"decay_rate"`
	MinSampleSize      int     `yaml:"min_sample_size" json:"min_sample_size"`
	
	// 相似性参数
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"`
	MaxSimilarQueries   int     `yaml:"max_similar_queries" json:"max_similar_queries"`
	
	// 模式识别参数
	PatternUpdateInterval time.Duration `yaml:"pattern_update_interval" json:"pattern_update_interval"`
	MinPatternSupport     float64       `yaml:"min_pattern_support" json:"min_pattern_support"`
	MaxPatternLength      int           `yaml:"max_pattern_length" json:"max_pattern_length"`
	
	// 反馈学习参数
	FeedbackWeight      float64 `yaml:"feedback_weight" json:"feedback_weight"`
	NegativeFeedbackPenalty float64 `yaml:"negative_feedback_penalty" json:"negative_feedback_penalty"`
	
	// 性能参数
	BatchUpdateSize     int           `yaml:"batch_update_size" json:"batch_update_size"`
	UpdateInterval      time.Duration `yaml:"update_interval" json:"update_interval"`
	EnableAsyncLearning bool          `yaml:"enable_async_learning" json:"enable_async_learning"`
}

// QueryHistoryStore 查询历史存储
type QueryHistoryStore struct {
	// 查询历史记录
	history []*QueryHistoryRecord
	
	// 查询索引（快速查找）
	queryIndex map[string]*QueryHistoryRecord
	
	// 用户查询历史
	userHistory map[int64][]*QueryHistoryRecord
	
	// 时间索引
	timeIndex *TimeIndex
	
	// 并发控制
	mu sync.RWMutex
	
	// 配置
	maxSize   int
	retention time.Duration
}

// QueryHistoryRecord 查询历史记录
type QueryHistoryRecord struct {
	ID               string              `json:"id"`
	Query            string              `json:"query"`
	NormalizedQuery  string              `json:"normalized_query"`
	UserID           int64               `json:"user_id"`
	PredictedCategory ComplexityCategory `json:"predicted_category"`
	ActualCategory   ComplexityCategory  `json:"actual_category"`
	ComplexityScore  float64             `json:"complexity_score"`
	Features         *QueryFeatures      `json:"features"`
	Feedback         *UserFeedback       `json:"feedback,omitempty"`
	ExecutionTime    time.Duration       `json:"execution_time"`
	Success          bool                `json:"success"`
	ErrorMessage     string              `json:"error_message,omitempty"`
	Timestamp        time.Time           `json:"timestamp"`
	UpdateCount      int                 `json:"update_count"`
	LastUpdated      time.Time           `json:"last_updated"`
}

// UserFeedback 用户反馈
type UserFeedback struct {
	Rating       int       `json:"rating"`        // 1-5评分
	IsCorrect    *bool     `json:"is_correct"`    // 分类是否正确
	ActualCategory *ComplexityCategory `json:"actual_category,omitempty"` // 用户认为的正确分类
	Comments     string    `json:"comments,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// TimeIndex 时间索引
type TimeIndex struct {
	// 按小时的查询索引
	hourlyIndex map[string][]*QueryHistoryRecord
	
	// 按天的查询索引
	dailyIndex map[string][]*QueryHistoryRecord
	
	// 最后更新时间
	lastUpdate time.Time
	
	// 并发控制
	mu sync.RWMutex
}

// PatternRecognizer 模式识别器
type PatternRecognizer struct {
	// 发现的查询模式
	patterns []*QueryPattern
	
	// 模式索引
	patternIndex map[string]*QueryPattern
	
	// 模式统计
	patternStats map[string]*PatternStats
	
	// 最后更新时间
	lastUpdate time.Time
	
	// 并发控制
	mu sync.RWMutex
}

// QueryPattern 查询模式
type QueryPattern struct {
	ID          string              `json:"id"`
	Pattern     string              `json:"pattern"`
	Category    ComplexityCategory  `json:"category"`
	Support     float64             `json:"support"`     // 支持度
	Confidence  float64             `json:"confidence"`  // 置信度
	Frequency   int                 `json:"frequency"`   // 出现频次
	Examples    []string            `json:"examples"`    // 示例查询
	Features    *PatternFeatures    `json:"features"`    // 模式特征
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// PatternFeatures 模式特征
type PatternFeatures struct {
	Keywords       []string  `json:"keywords"`
	Structure      string    `json:"structure"`
	Complexity     float64   `json:"complexity"`
	AvgExecutionTime time.Duration `json:"avg_execution_time"`
}

// PatternStats 模式统计
type PatternStats struct {
	MatchCount    int64     `json:"match_count"`
	SuccessRate   float64   `json:"success_rate"`
	AvgAccuracy   float64   `json:"avg_accuracy"`
	LastMatch     time.Time `json:"last_match"`
}

// FeedbackLearner 反馈学习器
type FeedbackLearner struct {
	// 反馈历史
	feedbackHistory []*FeedbackRecord
	
	// 学习权重
	categoryWeights map[ComplexityCategory]float64
	
	// 特征权重调整
	featureAdjustments map[string]float64
	
	// 学习统计
	learningMetrics *LearningMetrics
	
	// 并发控制
	mu sync.RWMutex
}

// FeedbackRecord 反馈记录
type FeedbackRecord struct {
	QueryID       string              `json:"query_id"`
	Query         string              `json:"query"`
	UserID        int64               `json:"user_id"`
	Predicted     ComplexityCategory  `json:"predicted"`
	Actual        ComplexityCategory  `json:"actual"`
	Feedback      *UserFeedback       `json:"feedback"`
	Adjustment    float64             `json:"adjustment"` // 学习调整量
	Timestamp     time.Time           `json:"timestamp"`
}

// LearningMetrics 学习指标
type LearningMetrics struct {
	TotalFeedback     int64             `json:"total_feedback"`
	PositiveFeedback  int64             `json:"positive_feedback"`
	NegativeFeedback  int64             `json:"negative_feedback"`
	AccuracyImprovement float64         `json:"accuracy_improvement"`
	CategoryAccuracy  map[string]float64 `json:"category_accuracy"`
	LastUpdate        time.Time         `json:"last_update"`
}

// SimilarityMatcher 相似性匹配器
type SimilarityMatcher struct {
	// 查询向量化器
	vectorizer *QueryVectorizer
	
	// 相似性索引
	similarityIndex *SimilarityIndex
	
	// 匹配缓存
	matchCache map[string][]*SimilarityMatch
	
	// 配置
	threshold float64
	maxMatches int
	
	// 并发控制
	mu sync.RWMutex
}

// QueryVectorizer 查询向量化器
type QueryVectorizer struct {
	// TF-IDF向量化器
	tfidfVectorizer *TFIDFVectorizer
	
	// 特征向量化器
	featureVectorizer *FeatureVectorizer
	
	// 词汇表
	vocabulary map[string]int
	
	// IDF值
	idfValues map[string]float64
}

// SimilarityMatch 相似性匹配结果
type SimilarityMatch struct {
	QueryID    string  `json:"query_id"`
	Query      string  `json:"query"`
	Similarity float64 `json:"similarity"`
	Category   ComplexityCategory `json:"category"`
	Confidence float64 `json:"confidence"`
}

// TFIDFVectorizer TF-IDF向量化器
type TFIDFVectorizer struct {
	vocabulary map[string]int
	idf        map[string]float64
	documents  []string
}

// FeatureVectorizer 特征向量化器
type FeatureVectorizer struct {
	featureNames []string
	normalizers  map[string]*Normalizer
}

// Normalizer 标准化器
type Normalizer struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
	Std  float64 `json:"std"`
}

// LearningStats 学习统计
type LearningStats struct {
	// 总体统计
	TotalQueries      int64   `json:"total_queries"`
	TotalFeedback     int64   `json:"total_feedback"`
	AccuracyRate      float64 `json:"accuracy_rate"`
	
	// 学习效果
	LearningProgress  float64 `json:"learning_progress"`
	ModelImprovement  float64 `json:"model_improvement"`
	
	// 模式识别统计
	DiscoveredPatterns int     `json:"discovered_patterns"`
	ActivePatterns     int     `json:"active_patterns"`
	
	// 相似性匹配统计
	SimilarityMatches  int64   `json:"similarity_matches"`
	MatchAccuracy      float64 `json:"match_accuracy"`
	
	// 时间统计
	LastLearningUpdate time.Time `json:"last_learning_update"`
	
	// 并发控制
	mu sync.RWMutex
}

// NewLearningEngine 创建学习引擎
func NewLearningEngine(ctx context.Context, config *LearningConfig) *LearningEngine {
	if config == nil {
		config = getDefaultLearningConfig()
	}
	
	engineCtx, cancel := context.WithCancel(ctx)
	
	engine := &LearningEngine{
		historyStore:      newQueryHistoryStore(config.MaxHistorySize, config.HistoryRetention),
		patternRecognizer: newPatternRecognizer(),
		feedbackLearner:   newFeedbackLearner(),
		similarityMatcher: newSimilarityMatcher(config.SimilarityThreshold, config.MaxSimilarQueries),
		learningStats:     newLearningStats(),
		config:            config,
		ctx:               engineCtx,
		cancel:            cancel,
	}
	
	// 启动后台学习任务
	if config.EnableAsyncLearning {
		engine.startAsyncLearning()
	}
	
	return engine
}

// LearnFromHistory 从历史数据学习
func (le *LearningEngine) LearnFromHistory(record *QueryHistoryRecord) error {
	// 存储历史记录
	if err := le.historyStore.AddRecord(record); err != nil {
		return fmt.Errorf("存储历史记录失败: %w", err)
	}
	
	// 异步学习（如果启用）
	if le.config.EnableAsyncLearning {
		go func() {
			le.processRecordAsync(record)
		}()
		return nil
	}
	
	// 同步学习
	return le.processRecordSync(record)
}

// processRecordSync 同步处理记录
func (le *LearningEngine) processRecordSync(record *QueryHistoryRecord) error {
	// 更新相似性匹配器
	if err := le.similarityMatcher.UpdateIndex(record); err != nil {
		return fmt.Errorf("更新相似性索引失败: %w", err)
	}
	
	// 更新模式识别器
	if err := le.patternRecognizer.AnalyzeRecord(record); err != nil {
		return fmt.Errorf("模式分析失败: %w", err)
	}
	
	// 处理反馈学习
	if record.Feedback != nil {
		if err := le.feedbackLearner.ProcessFeedback(record); err != nil {
			return fmt.Errorf("处理反馈失败: %w", err)
		}
	}
	
	// 更新统计
	le.updateLearningStats(record)
	
	return nil
}

// processRecordAsync 异步处理记录
func (le *LearningEngine) processRecordAsync(record *QueryHistoryRecord) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("异步学习过程异常: %v\n", r)
		}
	}()
	
	if err := le.processRecordSync(record); err != nil {
		fmt.Printf("异步学习失败: %v\n", err)
	}
}

// PredictCategory 基于历史学习预测分类
func (le *LearningEngine) PredictCategory(query string, features *QueryFeatures, userID int64) (*LearningPrediction, error) {
	le.mu.RLock()
	defer le.mu.RUnlock()
	
	prediction := &LearningPrediction{
		Query:     query,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	
	// 1. 相似性匹配预测
	similarMatches, err := le.similarityMatcher.FindSimilarQueries(query, features)
	if err == nil && len(similarMatches) > 0 {
		prediction.SimilarityPrediction = le.aggregateSimilarityPredictions(similarMatches)
	}
	
	// 2. 模式匹配预测
	patternMatch, err := le.patternRecognizer.MatchPattern(query, features)
	if err == nil && patternMatch != nil {
		prediction.PatternPrediction = &PatternPrediction{
			Pattern:    patternMatch.Pattern,
			Category:   patternMatch.Category,
			Confidence: patternMatch.Confidence,
		}
	}
	
	// 3. 反馈学习预测
	feedbackPrediction := le.feedbackLearner.PredictWithFeedback(query, features, userID)
	if feedbackPrediction != nil {
		prediction.FeedbackPrediction = feedbackPrediction
	}
	
	// 4. 综合预测
	prediction.FinalPrediction = le.combinePredictions(prediction)
	
	return prediction, nil
}

// LearningPrediction 学习预测结果
type LearningPrediction struct {
	Query               string              `json:"query"`
	UserID              int64               `json:"user_id"`
	SimilarityPrediction *SimilarityPrediction `json:"similarity_prediction,omitempty"`
	PatternPrediction    *PatternPrediction    `json:"pattern_prediction,omitempty"`
	FeedbackPrediction   *FeedbackPrediction   `json:"feedback_prediction,omitempty"`
	FinalPrediction      *FinalPrediction      `json:"final_prediction"`
	Timestamp            time.Time           `json:"timestamp"`
}

// SimilarityPrediction 相似性预测
type SimilarityPrediction struct {
	Category        ComplexityCategory  `json:"category"`
	Confidence      float64             `json:"confidence"`
	SimilarMatches  []*SimilarityMatch  `json:"similar_matches"`
	AvgSimilarity   float64             `json:"avg_similarity"`
}

// PatternPrediction 模式预测
type PatternPrediction struct {
	Pattern     string              `json:"pattern"`
	Category    ComplexityCategory  `json:"category"`
	Confidence  float64             `json:"confidence"`
	Support     float64             `json:"support"`
}

// FeedbackPrediction 反馈预测
type FeedbackPrediction struct {
	Category       ComplexityCategory `json:"category"`
	Confidence     float64            `json:"confidence"`
	UserBias       float64            `json:"user_bias"`       // 用户偏好
	HistoryWeight  float64            `json:"history_weight"`  // 历史权重
}

// FinalPrediction 最终预测
type FinalPrediction struct {
	Category    ComplexityCategory `json:"category"`
	Confidence  float64            `json:"confidence"`
	Method      string             `json:"method"`      // 主要预测方法
	Weights     map[string]float64 `json:"weights"`     // 各方法权重
	Reasoning   string             `json:"reasoning"`   // 预测推理
}

// 实现各组件的核心方法...

func newQueryHistoryStore(maxSize int, retention time.Duration) *QueryHistoryStore {
	return &QueryHistoryStore{
		history:     make([]*QueryHistoryRecord, 0, maxSize),
		queryIndex:  make(map[string]*QueryHistoryRecord),
		userHistory: make(map[int64][]*QueryHistoryRecord),
		timeIndex:   newTimeIndex(),
		maxSize:     maxSize,
		retention:   retention,
	}
}

func (qhs *QueryHistoryStore) AddRecord(record *QueryHistoryRecord) error {
	qhs.mu.Lock()
	defer qhs.mu.Unlock()
	
	// 检查是否需要清理过期记录
	qhs.cleanupExpiredRecords()
	
	// 检查容量限制
	if len(qhs.history) >= qhs.maxSize {
		qhs.evictOldestRecord()
	}
	
	// 添加新记录
	qhs.history = append(qhs.history, record)
	qhs.queryIndex[record.ID] = record
	
	// 更新用户历史
	userRecords := qhs.userHistory[record.UserID]
	userRecords = append(userRecords, record)
	
	// 限制用户历史数量
	maxUserHistory := 1000
	if len(userRecords) > maxUserHistory {
		userRecords = userRecords[len(userRecords)-maxUserHistory:]
	}
	qhs.userHistory[record.UserID] = userRecords
	
	// 更新时间索引
	qhs.timeIndex.AddRecord(record)
	
	return nil
}

func (qhs *QueryHistoryStore) GetRecord(id string) *QueryHistoryRecord {
	qhs.mu.RLock()
	defer qhs.mu.RUnlock()
	
	return qhs.queryIndex[id]
}

func (qhs *QueryHistoryStore) GetUserHistory(userID int64, limit int) []*QueryHistoryRecord {
	qhs.mu.RLock()
	defer qhs.mu.RUnlock()
	
	userRecords := qhs.userHistory[userID]
	if len(userRecords) == 0 {
		return nil
	}
	
	if limit > 0 && len(userRecords) > limit {
		return userRecords[len(userRecords)-limit:]
	}
	
	result := make([]*QueryHistoryRecord, len(userRecords))
	copy(result, userRecords)
	return result
}

func (qhs *QueryHistoryStore) cleanupExpiredRecords() {
	now := time.Now()
	validRecords := make([]*QueryHistoryRecord, 0, len(qhs.history))
	
	for _, record := range qhs.history {
		if now.Sub(record.Timestamp) <= qhs.retention {
			validRecords = append(validRecords, record)
		} else {
			// 从索引中删除
			delete(qhs.queryIndex, record.ID)
		}
	}
	
	qhs.history = validRecords
	
	// 清理用户历史中的过期记录
	for userID, userRecords := range qhs.userHistory {
		validUserRecords := make([]*QueryHistoryRecord, 0, len(userRecords))
		for _, record := range userRecords {
			if now.Sub(record.Timestamp) <= qhs.retention {
				validUserRecords = append(validUserRecords, record)
			}
		}
		
		if len(validUserRecords) == 0 {
			delete(qhs.userHistory, userID)
		} else {
			qhs.userHistory[userID] = validUserRecords
		}
	}
}

func (qhs *QueryHistoryStore) evictOldestRecord() {
	if len(qhs.history) == 0 {
		return
	}
	
	// 移除最旧的记录
	oldest := qhs.history[0]
	qhs.history = qhs.history[1:]
	delete(qhs.queryIndex, oldest.ID)
	
	// 从用户历史中删除
	userRecords := qhs.userHistory[oldest.UserID]
	for i, record := range userRecords {
		if record.ID == oldest.ID {
			qhs.userHistory[oldest.UserID] = append(userRecords[:i], userRecords[i+1:]...)
			break
		}
	}
}

func newTimeIndex() *TimeIndex {
	return &TimeIndex{
		hourlyIndex: make(map[string][]*QueryHistoryRecord),
		dailyIndex:  make(map[string][]*QueryHistoryRecord),
	}
}

func (ti *TimeIndex) AddRecord(record *QueryHistoryRecord) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	
	// 按小时索引
	hourKey := record.Timestamp.Format("2006-01-02-15")
	ti.hourlyIndex[hourKey] = append(ti.hourlyIndex[hourKey], record)
	
	// 按天索引
	dayKey := record.Timestamp.Format("2006-01-02")
	ti.dailyIndex[dayKey] = append(ti.dailyIndex[dayKey], record)
	
	ti.lastUpdate = time.Now()
}

func newPatternRecognizer() *PatternRecognizer {
	return &PatternRecognizer{
		patterns:     make([]*QueryPattern, 0),
		patternIndex: make(map[string]*QueryPattern),
		patternStats: make(map[string]*PatternStats),
	}
}

func (pr *PatternRecognizer) AnalyzeRecord(record *QueryHistoryRecord) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	// 从查询中提取模式
	patterns := pr.extractPatterns(record.NormalizedQuery)
	
	for _, pattern := range patterns {
		if existing, exists := pr.patternIndex[pattern]; exists {
			// 更新现有模式
			existing.Frequency++
			existing.UpdatedAt = time.Now()
			
			// 添加示例（如果不存在）
			if len(existing.Examples) < 10 && !contains(existing.Examples, record.Query) {
				existing.Examples = append(existing.Examples, record.Query)
			}
			
			// 更新置信度
			pr.updatePatternConfidence(existing, record)
		} else {
			// 创建新模式
			newPattern := &QueryPattern{
				ID:        generatePatternID(),
				Pattern:   pattern,
				Category:  record.ActualCategory,
				Frequency: 1,
				Examples:  []string{record.Query},
				Features:  pr.extractPatternFeatures(pattern, record),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			
			pr.patterns = append(pr.patterns, newPattern)
			pr.patternIndex[pattern] = newPattern
		}
		
		// 更新模式统计
		pr.updatePatternStats(pattern, record)
	}
	
	pr.lastUpdate = time.Now()
	return nil
}

func (pr *PatternRecognizer) MatchPattern(query string, features *QueryFeatures) (*QueryPattern, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	normalizedQuery := normalizeQueryForPattern(query)
	
	// 寻找匹配的模式
	var bestMatch *QueryPattern
	var bestScore float64
	
	for _, pattern := range pr.patterns {
		score := pr.calculatePatternMatchScore(normalizedQuery, pattern)
		if score > bestScore {
			bestScore = score
			bestMatch = pattern
		}
	}
	
	// 检查是否满足最小置信度
	if bestMatch != nil && bestScore > 0.7 {
		return bestMatch, nil
	}
	
	return nil, nil
}

func (pr *PatternRecognizer) extractPatterns(query string) []string {
	// 简化的模式提取实现
	var patterns []string
	
	// 提取关键词模式
	keywords := extractKeywords(query)
	if len(keywords) > 0 {
		pattern := "KEYWORDS:" + strings.Join(keywords, ",")
		patterns = append(patterns, pattern)
	}
	
	// 提取结构模式
	structure := extractQueryStructure(query)
	if structure != "" {
		patterns = append(patterns, "STRUCTURE:"+structure)
	}
	
	// 提取JOIN模式
	joinPattern := extractJoinPattern(query)
	if joinPattern != "" {
		patterns = append(patterns, "JOIN:"+joinPattern)
	}
	
	return patterns
}

func extractKeywords(query string) []string {
	keywords := []string{
		"select", "from", "where", "join", "group", "order", "having",
		"union", "with", "case", "when", "exists", "not exists",
	}
	
	var found []string
	queryLower := strings.ToLower(query)
	
	for _, keyword := range keywords {
		if strings.Contains(queryLower, keyword) {
			found = append(found, keyword)
		}
	}
	
	return found
}

func extractQueryStructure(query string) string {
	// 简化的结构提取
	queryLower := strings.ToLower(query)
	
	structure := ""
	if strings.Contains(queryLower, "select") {
		structure += "SELECT"
	}
	if strings.Contains(queryLower, "join") {
		structure += "+JOIN"
	}
	if strings.Contains(queryLower, "group by") {
		structure += "+GROUP"
	}
	if strings.Contains(queryLower, "order by") {
		structure += "+ORDER"
	}
	if strings.Contains(queryLower, "having") {
		structure += "+HAVING"
	}
	
	return structure
}

func extractJoinPattern(query string) string {
	queryLower := strings.ToLower(query)
	
	joinTypes := []string{"inner join", "left join", "right join", "full join", "cross join"}
	var pattern string
	
	for _, joinType := range joinTypes {
		count := strings.Count(queryLower, joinType)
		if count > 0 {
			if pattern != "" {
				pattern += "+"
			}
			pattern += fmt.Sprintf("%s:%d", strings.ToUpper(strings.Replace(joinType, " join", "", 1)), count)
		}
	}
	
	return pattern
}

func normalizeQueryForPattern(query string) string {
	// 规范化查询用于模式匹配
	normalized := strings.ToLower(strings.TrimSpace(query))
	
	// 移除多余空格
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	// 移除字符串字面量
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "'STRING'")
	normalized = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(normalized, "\"STRING\"")
	
	// 移除数字字面量
	normalized = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(normalized, "NUMBER")
	
	return normalized
}

func (pr *PatternRecognizer) calculatePatternMatchScore(query string, pattern *QueryPattern) float64 {
	// 简化的模式匹配评分
	score := 0.0
	
	// 检查模式字符串匹配
	if strings.Contains(query, pattern.Pattern) {
		score += 0.5
	}
	
	// 检查结构相似性
	if pattern.Features != nil {
		if len(pattern.Features.Keywords) > 0 {
			keywordMatches := 0
			for _, keyword := range pattern.Features.Keywords {
				if strings.Contains(query, keyword) {
					keywordMatches++
				}
			}
			score += float64(keywordMatches) / float64(len(pattern.Features.Keywords)) * 0.3
		}
	}
	
	// 基于频率和置信度调整
	score += pattern.Confidence * 0.2
	
	return math.Min(score, 1.0)
}

func (pr *PatternRecognizer) updatePatternConfidence(pattern *QueryPattern, record *QueryHistoryRecord) {
	// 简化的置信度更新
	if record.ActualCategory == pattern.Category {
		// 分类正确，提升置信度
		pattern.Confidence = math.Min(pattern.Confidence+0.05, 1.0)
	} else {
		// 分类错误，降低置信度
		pattern.Confidence = math.Max(pattern.Confidence-0.1, 0.0)
	}
}

func (pr *PatternRecognizer) extractPatternFeatures(pattern string, record *QueryHistoryRecord) *PatternFeatures {
	return &PatternFeatures{
		Keywords:    extractKeywords(record.Query),
		Structure:   extractQueryStructure(record.Query),
		Complexity:  record.ComplexityScore,
		AvgExecutionTime: record.ExecutionTime,
	}
}

func (pr *PatternRecognizer) updatePatternStats(pattern string, record *QueryHistoryRecord) {
	stats, exists := pr.patternStats[pattern]
	if !exists {
		stats = &PatternStats{
			MatchCount:  0,
			SuccessRate: 0.0,
			AvgAccuracy: 0.0,
		}
		pr.patternStats[pattern] = stats
	}
	
	stats.MatchCount++
	stats.LastMatch = record.Timestamp
	
	// 更新成功率和准确率（简化计算）
	if record.Success {
		stats.SuccessRate = (stats.SuccessRate*float64(stats.MatchCount-1) + 1.0) / float64(stats.MatchCount)
	} else {
		stats.SuccessRate = (stats.SuccessRate*float64(stats.MatchCount-1) + 0.0) / float64(stats.MatchCount)
	}
}

func newFeedbackLearner() *FeedbackLearner {
	return &FeedbackLearner{
		feedbackHistory:    make([]*FeedbackRecord, 0),
		categoryWeights:    make(map[ComplexityCategory]float64),
		featureAdjustments: make(map[string]float64),
		learningMetrics:    &LearningMetrics{
			CategoryAccuracy: make(map[string]float64),
		},
	}
}

func (fl *FeedbackLearner) ProcessFeedback(record *QueryHistoryRecord) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	
	if record.Feedback == nil {
		return nil
	}
	
	// 创建反馈记录
	feedbackRecord := &FeedbackRecord{
		QueryID:   record.ID,
		Query:     record.Query,
		UserID:    record.UserID,
		Predicted: record.PredictedCategory,
		Actual:    record.ActualCategory,
		Feedback:  record.Feedback,
		Timestamp: time.Now(),
	}
	
	// 计算调整量
	adjustment := fl.calculateFeedbackAdjustment(feedbackRecord)
	feedbackRecord.Adjustment = adjustment
	
	// 存储反馈记录
	fl.feedbackHistory = append(fl.feedbackHistory, feedbackRecord)
	
	// 限制历史记录数量
	maxHistory := 10000
	if len(fl.feedbackHistory) > maxHistory {
		fl.feedbackHistory = fl.feedbackHistory[len(fl.feedbackHistory)-maxHistory:]
	}
	
	// 更新类别权重
	fl.updateCategoryWeights(feedbackRecord)
	
	// 更新特征权重
	fl.updateFeatureWeights(feedbackRecord, record.Features)
	
	// 更新学习指标
	fl.updateLearningMetrics(feedbackRecord)
	
	return nil
}

func (fl *FeedbackLearner) PredictWithFeedback(query string, features *QueryFeatures, userID int64) *FeedbackPrediction {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	// 基于反馈历史预测
	userBias := fl.calculateUserBias(userID)
	historyWeight := fl.calculateHistoryWeight(query, features)
	
	// 计算预测分类
	categoryScores := make(map[ComplexityCategory]float64)
	
	for category, weight := range fl.categoryWeights {
		score := weight * historyWeight * (1.0 + userBias)
		categoryScores[category] = score
	}
	
	// 选择最高分的类别
	var bestCategory ComplexityCategory
	var bestScore float64
	
	for category, score := range categoryScores {
		if score > bestScore {
			bestScore = score
			bestCategory = category
		}
	}
	
	confidence := math.Min(bestScore, 1.0)
	
	return &FeedbackPrediction{
		Category:      bestCategory,
		Confidence:    confidence,
		UserBias:      userBias,
		HistoryWeight: historyWeight,
	}
}

func (fl *FeedbackLearner) calculateFeedbackAdjustment(record *FeedbackRecord) float64 {
	// 基于反馈计算调整量
	adjustment := 0.0
	
	if record.Feedback.IsCorrect != nil {
		if *record.Feedback.IsCorrect {
			adjustment = 0.1 // 正反馈
		} else {
			adjustment = -0.2 // 负反馈
		}
	}
	
	// 基于评分调整
	if record.Feedback.Rating > 0 {
		ratingAdjustment := (float64(record.Feedback.Rating) - 3.0) / 10.0 // -0.2 到 0.2
		adjustment += ratingAdjustment
	}
	
	return adjustment
}

func (fl *FeedbackLearner) updateCategoryWeights(record *FeedbackRecord) {
	// 更新类别权重
	if record.Feedback.IsCorrect != nil {
		if *record.Feedback.IsCorrect {
			// 正确预测，增强该类别权重
			current := fl.categoryWeights[record.Predicted]
			fl.categoryWeights[record.Predicted] = current + 0.01
		} else {
			// 错误预测，降低该类别权重，增强正确类别权重
			current := fl.categoryWeights[record.Predicted]
			fl.categoryWeights[record.Predicted] = math.Max(current-0.02, 0.0)
			
			if record.Feedback.ActualCategory != nil {
				correct := fl.categoryWeights[*record.Feedback.ActualCategory]
				fl.categoryWeights[*record.Feedback.ActualCategory] = correct + 0.01
			}
		}
	}
}

func (fl *FeedbackLearner) updateFeatureWeights(record *FeedbackRecord, features *QueryFeatures) {
	if record.Feedback.IsCorrect == nil || features == nil {
		return
	}
	
	adjustment := record.Adjustment * 0.1 // 缩放调整量
	
	// 更新各个特征权重
	featureValues := map[string]float64{
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
	}
	
	for featureName, featureValue := range featureValues {
		// 基于特征值和反馈调整权重
		currentAdjustment := fl.featureAdjustments[featureName]
		fl.featureAdjustments[featureName] = currentAdjustment + adjustment*featureValue
		
		// 限制调整范围
		fl.featureAdjustments[featureName] = math.Max(-0.5, math.Min(0.5, fl.featureAdjustments[featureName]))
	}
}

func (fl *FeedbackLearner) calculateUserBias(userID int64) float64 {
	// 计算用户偏好
	userFeedbacks := 0
	positiveCount := 0
	
	for _, record := range fl.feedbackHistory {
		if record.UserID == userID {
			userFeedbacks++
			if record.Feedback.IsCorrect != nil && *record.Feedback.IsCorrect {
				positiveCount++
			}
		}
	}
	
	if userFeedbacks == 0 {
		return 0.0
	}
	
	// 返回偏好值 (-1 到 1)
	return (float64(positiveCount)/float64(userFeedbacks) - 0.5) * 2.0
}

func (fl *FeedbackLearner) calculateHistoryWeight(query string, features *QueryFeatures) float64 {
	// 基于历史相似查询计算权重
	weight := 0.5 // 基础权重
	
	similarCount := 0
	for _, record := range fl.feedbackHistory {
		if fl.calculateQuerySimilarity(query, record.Query) > 0.7 {
			similarCount++
		}
	}
	
	// 相似查询越多，权重越高
	weight += float64(similarCount) * 0.1
	
	return math.Min(weight, 1.0)
}

func (fl *FeedbackLearner) calculateQuerySimilarity(query1, query2 string) float64 {
	// 简化的查询相似度计算
	words1 := strings.Fields(strings.ToLower(query1))
	words2 := strings.Fields(strings.ToLower(query2))
	
	// 计算Jaccard相似度
	set1 := make(map[string]bool)
	for _, word := range words1 {
		set1[word] = true
	}
	
	set2 := make(map[string]bool)
	for _, word := range words2 {
		set2[word] = true
	}
	
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}
	
	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}
	
	return float64(intersection) / float64(union)
}

func (fl *FeedbackLearner) updateLearningMetrics(record *FeedbackRecord) {
	metrics := fl.learningMetrics
	
	metrics.TotalFeedback++
	
	if record.Feedback.IsCorrect != nil {
		if *record.Feedback.IsCorrect {
			metrics.PositiveFeedback++
		} else {
			metrics.NegativeFeedback++
		}
	}
	
	// 更新类别准确率
	categoryKey := string(record.Predicted)
	if record.Feedback.IsCorrect != nil && *record.Feedback.IsCorrect {
		current := metrics.CategoryAccuracy[categoryKey]
		metrics.CategoryAccuracy[categoryKey] = current + 0.01
	}
	
	metrics.LastUpdate = time.Now()
}

func newSimilarityMatcher(threshold float64, maxMatches int) *SimilarityMatcher {
	return &SimilarityMatcher{
		vectorizer:      newQueryVectorizer(),
		similarityIndex: &SimilarityIndex{}, // 使用之前定义的SimilarityIndex
		matchCache:     make(map[string][]*SimilarityMatch),
		threshold:      threshold,
		maxMatches:     maxMatches,
	}
}

func (sm *SimilarityMatcher) UpdateIndex(record *QueryHistoryRecord) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 向量化查询
	vector, err := sm.vectorizer.Vectorize(record.NormalizedQuery, record.Features)
	if err != nil {
		return fmt.Errorf("查询向量化失败: %w", err)
	}
	
	// 更新相似性索引
	return sm.similarityIndex.Add(record.ID, record.NormalizedQuery, vector, record.ActualCategory)
}

func (sm *SimilarityMatcher) FindSimilarQueries(query string, features *QueryFeatures) ([]*SimilarityMatch, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// 检查缓存
	if cached, exists := sm.matchCache[query]; exists {
		return cached, nil
	}
	
	// 向量化目标查询
	vector, err := sm.vectorizer.Vectorize(query, features)
	if err != nil {
		return nil, fmt.Errorf("查询向量化失败: %w", err)
	}
	
	// 在索引中搜索相似查询
	matches, err := sm.similarityIndex.Search(vector, sm.threshold, sm.maxMatches)
	if err != nil {
		return nil, fmt.Errorf("相似性搜索失败: %w", err)
	}
	
	// 缓存结果
	sm.matchCache[query] = matches
	
	// 限制缓存大小
	maxCacheSize := 1000
	if len(sm.matchCache) > maxCacheSize {
		// 简单的缓存清理策略：删除一半
		count := 0
		for key := range sm.matchCache {
			delete(sm.matchCache, key)
			count++
			if count >= maxCacheSize/2 {
				break
			}
		}
	}
	
	return matches, nil
}

func newQueryVectorizer() *QueryVectorizer {
	return &QueryVectorizer{
		tfidfVectorizer:   newTFIDFVectorizer(),
		featureVectorizer: newFeatureVectorizer(),
		vocabulary:        make(map[string]int),
		idfValues:         make(map[string]float64),
	}
}

func (qv *QueryVectorizer) Vectorize(query string, features *QueryFeatures) ([]float64, error) {
	// 文本向量化
	textVector, err := qv.tfidfVectorizer.Transform(query)
	if err != nil {
		return nil, err
	}
	
	// 特征向量化
	featureVector := qv.featureVectorizer.Transform(features)
	
	// 组合向量
	vector := append(textVector, featureVector...)
	
	return vector, nil
}

func newTFIDFVectorizer() *TFIDFVectorizer {
	return &TFIDFVectorizer{
		vocabulary: make(map[string]int),
		idf:        make(map[string]float64),
		documents:  make([]string, 0),
	}
}

func (tv *TFIDFVectorizer) Transform(query string) ([]float64, error) {
	// 简化的TF-IDF向量化
	words := strings.Fields(strings.ToLower(query))
	
	// 计算词频
	tf := make(map[string]float64)
	for _, word := range words {
		tf[word]++
	}
	
	// 归一化TF
	maxTf := 0.0
	for _, count := range tf {
		if count > maxTf {
			maxTf = count
		}
	}
	
	for word := range tf {
		tf[word] = tf[word] / maxTf
	}
	
	// 生成TF-IDF向量（简化实现）
	vector := make([]float64, 100) // 固定维度
	for i, word := range words {
		if i >= len(vector) {
			break
		}
		
		tfValue := tf[word]
		idfValue := tv.idf[word]
		if idfValue == 0 {
			idfValue = 1.0 // 默认IDF
		}
		
		vector[i] = tfValue * idfValue
	}
	
	return vector, nil
}

func newFeatureVectorizer() *FeatureVectorizer {
	return &FeatureVectorizer{
		featureNames: []string{
			"query_length", "word_count", "keyword_density",
			"clause_complexity", "join_complexity", "nesting_depth",
			"function_complexity", "condition_complexity",
			"table_complexity", "relation_complexity",
			"window_function_score", "recursive_score", "subquery_score",
			"overall_complexity", "structural_balance", "semantic_richness",
		},
		normalizers: make(map[string]*Normalizer),
	}
}

func (fv *FeatureVectorizer) Transform(features *QueryFeatures) []float64 {
	if features == nil {
		return make([]float64, len(fv.featureNames))
	}
	
	featureValues := map[string]float64{
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
	
	vector := make([]float64, len(fv.featureNames))
	for i, name := range fv.featureNames {
		value := featureValues[name]
		
		// 应用标准化（如果存在）
		if normalizer, exists := fv.normalizers[name]; exists {
			value = (value - normalizer.Mean) / normalizer.Std
		}
		
		vector[i] = value
	}
	
	return vector
}

// SimilarityIndex 相似性索引实现
func (si *SimilarityIndex) Add(id, query string, vector []float64, category ComplexityCategory) error {
	// 简化实现：存储到编辑距离索引
	if si.editDistanceIndex == nil {
		si.editDistanceIndex = make(map[string][]string)
	}
	
	// 按类别组织
	categoryKey := string(category)
	si.editDistanceIndex[categoryKey] = append(si.editDistanceIndex[categoryKey], query)
	
	return nil
}

func (si *SimilarityIndex) Search(vector []float64, threshold float64, maxMatches int) ([]*SimilarityMatch, error) {
	// 简化的搜索实现
	var matches []*SimilarityMatch
	
	// 这里应该实现真正的向量相似度搜索
	// 为了简化，返回空结果
	
	return matches, nil
}

func newLearningStats() *LearningStats {
	return &LearningStats{
		LastLearningUpdate: time.Now(),
	}
}

func (le *LearningEngine) updateLearningStats(record *QueryHistoryRecord) {
	le.learningStats.mu.Lock()
	defer le.learningStats.mu.Unlock()
	
	le.learningStats.TotalQueries++
	
	if record.Feedback != nil {
		le.learningStats.TotalFeedback++
	}
	
	le.learningStats.LastLearningUpdate = time.Now()
}

func (le *LearningEngine) aggregateSimilarityPredictions(matches []*SimilarityMatch) *SimilarityPrediction {
	if len(matches) == 0 {
		return nil
	}
	
	// 统计各类别的匹配数和平均相似度
	categoryStats := make(map[ComplexityCategory][]float64)
	totalSimilarity := 0.0
	
	for _, match := range matches {
		categoryStats[match.Category] = append(categoryStats[match.Category], match.Similarity)
		totalSimilarity += match.Similarity
	}
	
	// 选择最多匹配的类别
	var bestCategory ComplexityCategory
	var bestCount int
	var bestAvgSimilarity float64
	
	for category, similarities := range categoryStats {
		if len(similarities) > bestCount {
			bestCount = len(similarities)
			bestCategory = category
			
			sum := 0.0
			for _, sim := range similarities {
				sum += sim
			}
			bestAvgSimilarity = sum / float64(len(similarities))
		}
	}
	
	// 计算置信度
	confidence := bestAvgSimilarity * (float64(bestCount) / float64(len(matches)))
	
	return &SimilarityPrediction{
		Category:       bestCategory,
		Confidence:     confidence,
		SimilarMatches: matches,
		AvgSimilarity:  totalSimilarity / float64(len(matches)),
	}
}

func (le *LearningEngine) combinePredictions(prediction *LearningPrediction) *FinalPrediction {
	weights := map[string]float64{
		"similarity": 0.4,
		"pattern":    0.3,
		"feedback":   0.3,
	}
	
	categoryScores := map[ComplexityCategory]float64{
		CategorySimple:  0.0,
		CategoryMedium:  0.0,
		CategoryComplex: 0.0,
	}
	
	totalWeight := 0.0
	reasoning := "基于: "
	
	// 相似性预测
	if prediction.SimilarityPrediction != nil {
		weight := weights["similarity"] * prediction.SimilarityPrediction.Confidence
		categoryScores[prediction.SimilarityPrediction.Category] += weight
		totalWeight += weight
		reasoning += fmt.Sprintf("相似性匹配(%.2f) ", prediction.SimilarityPrediction.Confidence)
	}
	
	// 模式预测
	if prediction.PatternPrediction != nil {
		weight := weights["pattern"] * prediction.PatternPrediction.Confidence
		categoryScores[prediction.PatternPrediction.Category] += weight
		totalWeight += weight
		reasoning += fmt.Sprintf("模式匹配(%.2f) ", prediction.PatternPrediction.Confidence)
	}
	
	// 反馈预测
	if prediction.FeedbackPrediction != nil {
		weight := weights["feedback"] * prediction.FeedbackPrediction.Confidence
		categoryScores[prediction.FeedbackPrediction.Category] += weight
		totalWeight += weight
		reasoning += fmt.Sprintf("反馈学习(%.2f)", prediction.FeedbackPrediction.Confidence)
	}
	
	// 选择最高分类别
	var bestCategory ComplexityCategory
	var bestScore float64
	
	for category, score := range categoryScores {
		if score > bestScore {
			bestScore = score
			bestCategory = category
		}
	}
	
	confidence := 0.5 // 默认置信度
	if totalWeight > 0 {
		confidence = bestScore / totalWeight
	}
	
	return &FinalPrediction{
		Category:   bestCategory,
		Confidence: confidence,
		Method:     "combined",
		Weights:    weights,
		Reasoning:  reasoning,
	}
}

func (le *LearningEngine) startAsyncLearning() {
	le.wg.Add(1)
	go func() {
		defer le.wg.Done()
		
		ticker := time.NewTicker(le.config.UpdateInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				le.performPeriodicLearning()
			case <-le.ctx.Done():
				return
			}
		}
	}()
}

func (le *LearningEngine) performPeriodicLearning() {
	// 执行定期学习任务
	
	// 1. 更新模式识别
	if time.Since(le.patternRecognizer.lastUpdate) > le.config.PatternUpdateInterval {
		le.updatePatterns()
	}
	
	// 2. 清理过期数据
	le.cleanupExpiredData()
	
	// 3. 更新学习统计
	le.updatePeriodicStats()
}

func (le *LearningEngine) updatePatterns() {
	// 重新计算模式支持度和置信度
	le.patternRecognizer.mu.Lock()
	defer le.patternRecognizer.mu.Unlock()
	
	totalQueries := le.learningStats.TotalQueries
	if totalQueries == 0 {
		return
	}
	
	for _, pattern := range le.patternRecognizer.patterns {
		// 计算支持度
		pattern.Support = float64(pattern.Frequency) / float64(totalQueries)
		
		// 移除低支持度模式
		if pattern.Support < le.config.MinPatternSupport {
			delete(le.patternRecognizer.patternIndex, pattern.Pattern)
		}
	}
	
	// 过滤低支持度模式
	validPatterns := make([]*QueryPattern, 0)
	for _, pattern := range le.patternRecognizer.patterns {
		if pattern.Support >= le.config.MinPatternSupport {
			validPatterns = append(validPatterns, pattern)
		}
	}
	
	le.patternRecognizer.patterns = validPatterns
	le.patternRecognizer.lastUpdate = time.Now()
}

func (le *LearningEngine) cleanupExpiredData() {
	// 清理过期的历史记录和缓存
	le.historyStore.cleanupExpiredRecords()
	
	// 清理相似性匹配缓存
	le.similarityMatcher.mu.Lock()
	if len(le.similarityMatcher.matchCache) > 500 {
		// 清理一半缓存
		count := 0
		for key := range le.similarityMatcher.matchCache {
			delete(le.similarityMatcher.matchCache, key)
			count++
			if count >= len(le.similarityMatcher.matchCache)/2 {
				break
			}
		}
	}
	le.similarityMatcher.mu.Unlock()
}

func (le *LearningEngine) updatePeriodicStats() {
	le.learningStats.mu.Lock()
	defer le.learningStats.mu.Unlock()
	
	// 更新发现的模式数
	le.learningStats.DiscoveredPatterns = len(le.patternRecognizer.patterns)
	
	// 计算活跃模式数
	activePatterns := 0
	for _, pattern := range le.patternRecognizer.patterns {
		if time.Since(pattern.UpdatedAt) < 24*time.Hour {
			activePatterns++
		}
	}
	le.learningStats.ActivePatterns = activePatterns
	
	// 更新相似性匹配统计
	totalMatches := int64(0)
	for _, matches := range le.similarityMatcher.matchCache {
		totalMatches += int64(len(matches))
	}
	le.learningStats.SimilarityMatches = totalMatches
}

// GetLearningStats 获取学习统计信息
func (le *LearningEngine) GetLearningStats() *LearningStats {
	le.learningStats.mu.RLock()
	defer le.learningStats.mu.RUnlock()
	
	// 返回统计信息的副本
	stats := &LearningStats{
		TotalQueries:       le.learningStats.TotalQueries,
		TotalFeedback:      le.learningStats.TotalFeedback,
		AccuracyRate:       le.learningStats.AccuracyRate,
		LearningProgress:   le.learningStats.LearningProgress,
		ModelImprovement:   le.learningStats.ModelImprovement,
		DiscoveredPatterns: le.learningStats.DiscoveredPatterns,
		ActivePatterns:     le.learningStats.ActivePatterns,
		SimilarityMatches:  le.learningStats.SimilarityMatches,
		MatchAccuracy:      le.learningStats.MatchAccuracy,
		LastLearningUpdate: le.learningStats.LastLearningUpdate,
	}
	
	return stats
}

// Close 关闭学习引擎
func (le *LearningEngine) Close() error {
	le.cancel()
	le.wg.Wait()
	return nil
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func generatePatternID() string {
	return fmt.Sprintf("pattern_%d", time.Now().UnixNano())
}

func getDefaultLearningConfig() *LearningConfig {
	return &LearningConfig{
		MaxHistorySize:          100000,
		HistoryRetention:        30 * 24 * time.Hour, // 30天
		LearningRate:           0.01,
		DecayRate:              0.95,
		MinSampleSize:          10,
		SimilarityThreshold:    0.7,
		MaxSimilarQueries:      10,
		PatternUpdateInterval:  1 * time.Hour,
		MinPatternSupport:      0.01, // 1%支持度
		MaxPatternLength:       100,
		FeedbackWeight:         1.0,
		NegativeFeedbackPenalty: 2.0,
		BatchUpdateSize:        100,
		UpdateInterval:         5 * time.Minute,
		EnableAsyncLearning:    true,
	}
}