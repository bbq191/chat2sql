// 准确率监控系统 - AI查询质量跟踪与优化
// 实现实时准确率监控、反馈收集和模型改进建议

package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// AccuracyMonitor 准确率监控器
type AccuracyMonitor struct {
	feedbackStore   map[string]*QueryFeedback
	metrics         *AccuracyMetrics
	alertManager    *AccuracyAlertManager
	config          *AccuracyConfig
	logger          *zap.Logger
	
	// 数据存储
	dailyStats      map[string]*DailyStats     // date -> stats
	userStats       map[int64]*UserStats       // userID -> stats  
	modelStats      map[string]*ModelStats     // model -> stats
	categoryStats   map[string]*CategoryStats  // category -> stats
	
	// 实时数据
	realtimeMetrics *RealtimeMetrics
	trendAnalyzer   *TrendAnalyzer
	
	mu sync.RWMutex
}

// AccuracyConfig 准确率监控配置
type AccuracyConfig struct {
	MinAccuracyThreshold     float64       `json:"min_accuracy_threshold" yaml:"min_accuracy_threshold"`         // 最低准确率阈值
	DailyAccuracyTarget      float64       `json:"daily_accuracy_target" yaml:"daily_accuracy_target"`           // 日准确率目标
	WeeklyAccuracyTarget     float64       `json:"weekly_accuracy_target" yaml:"weekly_accuracy_target"`         // 周准确率目标
	AlertCooldown           time.Duration `json:"alert_cooldown" yaml:"alert_cooldown"`                         // 告警冷却时间
	DataRetentionDays       int           `json:"data_retention_days" yaml:"data_retention_days"`               // 数据保留天数
	SampleSize              int           `json:"sample_size" yaml:"sample_size"`                               // 统计样本大小
	FeedbackRequiredPercent int           `json:"feedback_required_percent" yaml:"feedback_required_percent"`   // 需要反馈的百分比
	EnableMLAnalysis        bool          `json:"enable_ml_analysis" yaml:"enable_ml_analysis"`                 // 启用ML分析
}

// QueryFeedback 查询反馈信息
type QueryFeedback struct {
	QueryID        string                 `json:"query_id"`
	UserID         int64                  `json:"user_id"`
	UserQuery      string                 `json:"user_query"`
	GeneratedSQL   string                 `json:"generated_sql"`
	ExpectedSQL    string                 `json:"expected_sql,omitempty"`
	IsCorrect      bool                   `json:"is_correct"`
	UserRating     int                    `json:"user_rating"`     // 1-5分评价
	Feedback       string                 `json:"feedback"`        // 用户文本反馈
	Category       QueryCategory          `json:"category"`        // 查询类别
	Difficulty     QueryDifficulty        `json:"difficulty"`      // 查询难度
	ErrorType      string                 `json:"error_type,omitempty"` // 错误类型
	ErrorDetails   string                 `json:"error_details,omitempty"` // 错误详情
	ProcessingTime time.Duration          `json:"processing_time"` // 处理时间
	TokensUsed     int                    `json:"tokens_used"`     // 使用的Token数
	ModelUsed      string                 `json:"model_used"`      // 使用的模型
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type QueryCategory string
const (
	CategoryBasicSelect    QueryCategory = "basic_select"
	CategoryJoinQuery      QueryCategory = "join_query"
	CategoryAggregation    QueryCategory = "aggregation"
	CategorySubquery       QueryCategory = "subquery"
	CategoryTimeAnalysis   QueryCategory = "time_analysis"
	CategoryComplexQuery   QueryCategory = "complex_query"
)

type QueryDifficulty string
const (
	DifficultyEasy    QueryDifficulty = "easy"
	DifficultyMedium  QueryDifficulty = "medium"
	DifficultyHard    QueryDifficulty = "hard"
	DifficultyExpert  QueryDifficulty = "expert"
)

// AccuracyMetrics Prometheus指标
type AccuracyMetrics struct {
	registry           *prometheus.Registry
	overallAccuracy     prometheus.Gauge
	dailyAccuracy       prometheus.Gauge
	userRatings         *prometheus.CounterVec
	errorTypes          *prometheus.CounterVec
	categoryAccuracy    *prometheus.GaugeVec
	modelAccuracy       *prometheus.GaugeVec
	processingTime      *prometheus.HistogramVec
	feedbackCount       prometheus.Counter
	improvementSuggestions prometheus.Counter
}

// DailyStats 日统计数据
type DailyStats struct {
	Date            string            `json:"date"`
	TotalQueries    int               `json:"total_queries"`
	CorrectQueries  int               `json:"correct_queries"`
	AccuracyRate    float64           `json:"accuracy_rate"`
	AvgUserRating   float64           `json:"avg_user_rating"`
	AvgProcessTime  time.Duration     `json:"avg_process_time"`
	ErrorBreakdown  map[string]int    `json:"error_breakdown"`
	CategoryStats   map[string]*CategoryAccuracy `json:"category_stats"`
	TopErrors       []ErrorPattern    `json:"top_errors"`
	ImprovementAreas []string         `json:"improvement_areas"`
}

// UserStats 用户统计数据
type UserStats struct {
	UserID          int64         `json:"user_id"`
	TotalQueries    int           `json:"total_queries"`
	CorrectQueries  int           `json:"correct_queries"`
	AccuracyRate    float64       `json:"accuracy_rate"`
	AvgRating       float64       `json:"avg_rating"`
	FavoriteCategory QueryCategory `json:"favorite_category"`
	LastFeedback    time.Time     `json:"last_feedback"`
	FeedbackCount   int           `json:"feedback_count"`
}

// ModelStats 模型统计数据
type ModelStats struct {
	ModelName       string        `json:"model_name"`
	TotalQueries    int           `json:"total_queries"`
	CorrectQueries  int           `json:"correct_queries"`
	AccuracyRate    float64       `json:"accuracy_rate"`
	AvgProcessTime  time.Duration `json:"avg_process_time"`
	AvgTokensUsed   float64       `json:"avg_tokens_used"`
	CostEfficiency  float64       `json:"cost_efficiency"`  // 准确率/成本比
}

// CategoryStats 类别统计数据
type CategoryStats struct {
	Category       QueryCategory `json:"category"`
	TotalQueries   int           `json:"total_queries"`
	CorrectQueries int           `json:"correct_queries"`
	AccuracyRate   float64       `json:"accuracy_rate"`
	AvgDifficulty  float64       `json:"avg_difficulty"`
	CommonErrors   []string      `json:"common_errors"`
}

// CategoryAccuracy 类别准确率
type CategoryAccuracy struct {
	TotalCount   int     `json:"total_count"`
	CorrectCount int     `json:"correct_count"`
	Accuracy     float64 `json:"accuracy"`
}

// ErrorPattern 错误模式
type ErrorPattern struct {
	Pattern     string `json:"pattern"`
	Count       int    `json:"count"`
	Example     string `json:"example"`
	Suggestion  string `json:"suggestion"`
}

// RealtimeMetrics 实时指标
type RealtimeMetrics struct {
	LastHourAccuracy    float64                    `json:"last_hour_accuracy"`
	Last24HourAccuracy  float64                    `json:"last_24h_accuracy"`
	QueriesPerMinute    float64                    `json:"queries_per_minute"`
	ActiveUsers         int                        `json:"active_users"`
	ErrorRateSpike      bool                       `json:"error_rate_spike"`
	TopFailingPatterns  []ErrorPattern             `json:"top_failing_patterns"`
	ModelPerformance    map[string]float64         `json:"model_performance"`
	CategoryTrends      map[QueryCategory]float64  `json:"category_trends"`
}

// TrendAnalyzer 趋势分析器
type TrendAnalyzer struct {
	hourlyData  []float64         // 24小时内的准确率数据
	dailyData   []float64         // 30天内的准确率数据
	weeklyData  []float64         // 12周内的准确率数据
	trends      map[string][]float64  // 各种趋势数据
	mu          sync.RWMutex
}

// AccuracyAlertManager 准确率告警管理器
type AccuracyAlertManager struct {
	alerts      map[string]*AccuracyAlert
	lastAlert   map[string]time.Time
	webhookURL  string
	slackToken  string
	emailConfig *EmailConfig
	logger      *zap.Logger
	mu          sync.Mutex
}

// AccuracyAlert 准确率告警
type AccuracyAlert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Level       AlertLevel             `json:"level"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Details     map[string]any `json:"details"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	Actions     []RecommendedAction    `json:"actions"`
}

type AlertType string
const (
	AlertTypeLowAccuracy     AlertType = "low_accuracy"
	AlertTypeErrorSpike      AlertType = "error_spike"
	AlertTypeModelDegraded   AlertType = "model_degraded"
	AlertTypeUserComplaint   AlertType = "user_complaint"
	AlertTypeTrendNegative   AlertType = "trend_negative"
)

type AlertLevel string
const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// RecommendedAction 推荐行动
type RecommendedAction struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	AutoFix     bool   `json:"auto_fix"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	FromEmail    string   `json:"from_email"`
	ToEmails     []string `json:"to_emails"`
	EnableTLS    bool     `json:"enable_tls"`
}

// NewAccuracyMonitor 创建准确率监控器
func NewAccuracyMonitor(config *AccuracyConfig, logger *zap.Logger) *AccuracyMonitor {
	if config == nil {
		config = DefaultAccuracyConfig()
	}

	// 创建独立的Prometheus注册表，避免测试时重复注册
	registry := prometheus.NewRegistry()
	
	// 创建Prometheus指标
	metrics := &AccuracyMetrics{
		registry: registry,
		overallAccuracy: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chat2sql_overall_accuracy",
			Help: "整体SQL生成准确率",
		}),
		dailyAccuracy: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chat2sql_daily_accuracy",
			Help: "日SQL生成准确率",
		}),
		userRatings: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat2sql_user_ratings_total",
				Help: "用户评价分布",
			},
			[]string{"rating"},
		),
		errorTypes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat2sql_errors_total",
				Help: "错误类型统计",
			},
			[]string{"error_type", "category"},
		),
		categoryAccuracy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "chat2sql_category_accuracy",
				Help: "各类别查询准确率",
			},
			[]string{"category"},
		),
		modelAccuracy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "chat2sql_model_accuracy",
				Help: "各模型准确率",
			},
			[]string{"model"},
		),
		processingTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat2sql_processing_time_seconds",
				Help:    "查询处理时间分布",
				Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
			[]string{"category", "difficulty"},
		),
		feedbackCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "chat2sql_feedback_total",
			Help: "反馈总数",
		}),
		improvementSuggestions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "chat2sql_improvement_suggestions_total",
			Help: "改进建议总数",
		}),
	}

	// 注册到独立注册表，避免与全局注册表冲突
	registry.MustRegister(
		metrics.overallAccuracy,
		metrics.dailyAccuracy,
		metrics.userRatings,
		metrics.errorTypes,
		metrics.categoryAccuracy,
		metrics.modelAccuracy,
		metrics.processingTime,
		metrics.feedbackCount,
		metrics.improvementSuggestions,
	)

	// 创建告警管理器
	alertManager := &AccuracyAlertManager{
		alerts:    make(map[string]*AccuracyAlert),
		lastAlert: make(map[string]time.Time),
		logger:    logger,
	}

	// 创建趋势分析器
	trendAnalyzer := &TrendAnalyzer{
		hourlyData: make([]float64, 24),
		dailyData:  make([]float64, 30),
		weeklyData: make([]float64, 12),
		trends:     make(map[string][]float64),
	}

	return &AccuracyMonitor{
		feedbackStore:   make(map[string]*QueryFeedback),
		metrics:         metrics,
		alertManager:    alertManager,
		config:          config,
		logger:          logger,
		dailyStats:      make(map[string]*DailyStats),
		userStats:       make(map[int64]*UserStats),
		modelStats:      make(map[string]*ModelStats),
		categoryStats:   make(map[string]*CategoryStats),
		realtimeMetrics: &RealtimeMetrics{},
		trendAnalyzer:   trendAnalyzer,
	}
}

// DefaultAccuracyConfig 默认配置
func DefaultAccuracyConfig() *AccuracyConfig {
	return &AccuracyConfig{
		MinAccuracyThreshold:     0.70,  // 70%
		DailyAccuracyTarget:      0.85,  // 85%
		WeeklyAccuracyTarget:     0.90,  // 90%
		AlertCooldown:           30 * time.Minute,
		DataRetentionDays:       90,
		SampleSize:              1000,
		FeedbackRequiredPercent: 10,  // 10%的查询需要反馈
		EnableMLAnalysis:        true,
	}
}

// RecordFeedback 记录用户反馈
func (am *AccuracyMonitor) RecordFeedback(feedback QueryFeedback) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.logger.Info("记录用户反馈",
		zap.String("query_id", feedback.QueryID),
		zap.Int64("user_id", feedback.UserID),
		zap.Bool("is_correct", feedback.IsCorrect),
		zap.Int("rating", feedback.UserRating),
	)

	// 设置时间戳
	if feedback.Timestamp.IsZero() {
		feedback.Timestamp = time.Now()
	}

	// 自动分类查询
	if feedback.Category == "" {
		feedback.Category = am.categorizeQuery(feedback.UserQuery)
	}

	// 自动评估难度
	if feedback.Difficulty == "" {
		feedback.Difficulty = am.assessDifficulty(feedback.UserQuery, feedback.GeneratedSQL)
	}

	// 存储反馈
	am.feedbackStore[feedback.QueryID] = &feedback

	// 更新各种统计数据
	am.updateDailyStats(feedback)
	am.updateUserStats(feedback)
	am.updateModelStats(feedback)
	am.updateCategoryStats(feedback)
	am.updateRealtimeMetrics(feedback)

	// 更新Prometheus指标
	am.updatePrometheusMetrics(feedback)

	// 检查是否需要告警
	if err := am.checkAlerts(); err != nil {
		am.logger.Warn("检查告警失败", zap.Error(err))
	}

	// 更新趋势分析
	am.trendAnalyzer.AddDataPoint(am.getCurrentAccuracy())

	am.metrics.feedbackCount.Inc()
	
	return nil
}

// categorizeQuery 自动分类查询
func (am *AccuracyMonitor) categorizeQuery(query string) QueryCategory {
	query = strings.ToLower(query)
	
	// 1. 优先检查最复杂的查询类型 - WITH语句和复杂子查询
	if strings.Contains(query, "with ") || strings.Contains(query, "recursive") {
		return CategoryComplexQuery
	}
	
	// 2. 检查子查询 - 在其他检查之前，避免被误分类
	if strings.Contains(query, "exists") || strings.Contains(query, "in (select") || 
		strings.Contains(query, "in(select") || strings.Contains(query, "子查询") {
		return CategorySubquery
	}
	
	// 3. 检查时间分析查询 - 优先于聚合查询检查
	// 同时包含时间字段和聚合函数的查询属于时间分析
	hasTimeField := strings.Contains(query, "date(") || strings.Contains(query, "time") || 
		strings.Contains(query, "年") || strings.Contains(query, "月") || strings.Contains(query, "日") || 
		strings.Contains(query, "趋势") || strings.Contains(query, "created_at") || 
		strings.Contains(query, "updated_at") || strings.Contains(query, "timestamp")
	
	hasTimeCondition := strings.Contains(query, "where") && hasTimeField
	hasGroupByTime := strings.Contains(query, "group by") && hasTimeField
	
	if (hasTimeCondition || hasGroupByTime) && hasTimeField {
		return CategoryTimeAnalysis
	}
	
	// 4. 检查JOIN查询
	if strings.Contains(query, "join") || strings.Contains(query, "联接") || strings.Contains(query, "关联") {
		return CategoryJoinQuery
	}
	
	// 5. 检查聚合查询
	if strings.Contains(query, "count") || strings.Contains(query, "sum") || strings.Contains(query, "avg") || 
		strings.Contains(query, "max") || strings.Contains(query, "min") ||
		strings.Contains(query, "统计") || strings.Contains(query, "总数") || strings.Contains(query, "平均") {
		return CategoryAggregation
	}
	
	// 6. 根据复杂度检查复杂查询（多个复杂关键词组合）
	complexKeywords := []string{"group by", "order by", "having", "union", "case when", "window", "partition"}
	complexCount := 0
	for _, keyword := range complexKeywords {
		if strings.Contains(query, keyword) {
			complexCount++
		}
	}
	if complexCount >= 2 {
		return CategoryComplexQuery
	}
	
	// 7. 默认为基础查询
	return CategoryBasicSelect
}

// assessDifficulty 评估查询难度
func (am *AccuracyMonitor) assessDifficulty(query, sql string) QueryDifficulty {
	query = strings.ToLower(query)
	sql = strings.ToLower(sql)
	
	complexityScore := 0
	
	// 基于查询复杂度的评分（重新调整权重）
	if strings.Contains(sql, "join") {
		complexityScore += 3 // JOIN操作提高权重
		// 多表JOIN额外加分
		joinCount := strings.Count(sql, "join")
		if joinCount > 1 {
			complexityScore += (joinCount - 1) * 2
		}
	}
	
	if strings.Contains(sql, "group by") {
		complexityScore += 2 // GROUP BY提高权重
	}
	
	if strings.Contains(sql, "having") {
		complexityScore += 3 // HAVING子句复杂度较高
	}
	
	if strings.Contains(sql, "union") {
		complexityScore += 4 // UNION操作复杂度高
	}
	
	if strings.Contains(sql, "case when") {
		complexityScore += 3
	}
	
	if strings.Contains(sql, "exists") || strings.Contains(sql, "not exists") {
		complexityScore += 3
	}
	
	// WITH语句（CTE）
	if strings.Contains(sql, "with ") {
		complexityScore += 4
	}
	
	// 窗口函数
	if strings.Contains(sql, "over(") || strings.Contains(sql, "row_number") ||
		strings.Contains(sql, "rank()") || strings.Contains(sql, "partition by") {
		complexityScore += 4
	}
	
	// 子查询层数（提高权重）
	subqueryCount := strings.Count(sql, "select") - 1
	complexityScore += subqueryCount * 3
	
	// IN子查询特别检查
	if strings.Contains(sql, "in (select") || strings.Contains(sql, "in(select") {
		complexityScore += 2
	}
	
	// 聚合函数数量
	aggregateFunctions := []string{"count(", "sum(", "avg(", "max(", "min(", "group_concat("}
	aggregateCount := 0
	for _, fn := range aggregateFunctions {
		aggregateCount += strings.Count(sql, fn)
	}
	if aggregateCount > 2 {
		complexityScore += aggregateCount - 2
	}
	
	// 表数量（多表查询复杂度）
	fromCount := strings.Count(sql, "from")
	if fromCount > 1 {
		complexityScore += (fromCount - 1) * 2
	}
	
	// ORDER BY复杂度
	if strings.Contains(sql, "order by") {
		complexityScore += 1
		// 多字段排序
		orderByIndex := strings.Index(sql, "order by")
		if orderByIndex != -1 {
			orderByClause := sql[orderByIndex:]
			commaCount := strings.Count(orderByClause, ",")
			if commaCount > 0 {
				complexityScore += commaCount
			}
		}
	}
	
	// 根据调整后的评分判断难度
	switch {
	case complexityScore <= 1:
		return DifficultyEasy
	case complexityScore <= 3:
		return DifficultyMedium  
	case complexityScore <= 10: // 调整Hard阈值到10
		return DifficultyHard
	default:
		return DifficultyExpert
	}
}

// updateDailyStats 更新日统计
func (am *AccuracyMonitor) updateDailyStats(feedback QueryFeedback) {
	dateStr := feedback.Timestamp.Format("2006-01-02")
	
	if am.dailyStats[dateStr] == nil {
		am.dailyStats[dateStr] = &DailyStats{
			Date:           dateStr,
			ErrorBreakdown: make(map[string]int),
			CategoryStats:  make(map[string]*CategoryAccuracy),
		}
	}
	
	stats := am.dailyStats[dateStr]
	stats.TotalQueries++
	
	if feedback.IsCorrect {
		stats.CorrectQueries++
	} else if feedback.ErrorType != "" {
		stats.ErrorBreakdown[feedback.ErrorType]++
	}
	
	stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
	
	// 更新类别统计
	categoryStr := string(feedback.Category)
	if stats.CategoryStats[categoryStr] == nil {
		stats.CategoryStats[categoryStr] = &CategoryAccuracy{}
	}
	
	catStats := stats.CategoryStats[categoryStr]
	catStats.TotalCount++
	if feedback.IsCorrect {
		catStats.CorrectCount++
	}
	catStats.Accuracy = float64(catStats.CorrectCount) / float64(catStats.TotalCount)
}

// updateUserStats 更新用户统计
func (am *AccuracyMonitor) updateUserStats(feedback QueryFeedback) {
	if am.userStats[feedback.UserID] == nil {
		am.userStats[feedback.UserID] = &UserStats{
			UserID: feedback.UserID,
		}
	}
	
	stats := am.userStats[feedback.UserID]
	stats.TotalQueries++
	
	if feedback.IsCorrect {
		stats.CorrectQueries++
	}
	
	stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
	
	if feedback.UserRating > 0 {
		// 计算平均评分
		totalRating := stats.AvgRating*float64(stats.FeedbackCount) + float64(feedback.UserRating)
		stats.FeedbackCount++
		stats.AvgRating = totalRating / float64(stats.FeedbackCount)
	}
	
	stats.FavoriteCategory = feedback.Category
	stats.LastFeedback = feedback.Timestamp
}

// updateModelStats 更新模型统计
func (am *AccuracyMonitor) updateModelStats(feedback QueryFeedback) {
	modelName := feedback.ModelUsed
	if modelName == "" {
		modelName = "default"
	}
	
	if am.modelStats[modelName] == nil {
		am.modelStats[modelName] = &ModelStats{
			ModelName: modelName,
		}
	}
	
	stats := am.modelStats[modelName]
	stats.TotalQueries++
	
	if feedback.IsCorrect {
		stats.CorrectQueries++
	}
	
	stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
	
	// 更新平均处理时间
	totalTime := stats.AvgProcessTime*time.Duration(stats.TotalQueries-1) + feedback.ProcessingTime
	stats.AvgProcessTime = totalTime / time.Duration(stats.TotalQueries)
	
	// 更新平均Token使用量
	totalTokens := stats.AvgTokensUsed*float64(stats.TotalQueries-1) + float64(feedback.TokensUsed)
	stats.AvgTokensUsed = totalTokens / float64(stats.TotalQueries)
}

// updateCategoryStats 更新类别统计
func (am *AccuracyMonitor) updateCategoryStats(feedback QueryFeedback) {
	categoryStr := string(feedback.Category)
	
	if am.categoryStats[categoryStr] == nil {
		am.categoryStats[categoryStr] = &CategoryStats{
			Category: feedback.Category,
		}
	}
	
	stats := am.categoryStats[categoryStr]
	stats.TotalQueries++
	
	if feedback.IsCorrect {
		stats.CorrectQueries++
	} else if feedback.ErrorType != "" {
		stats.CommonErrors = append(stats.CommonErrors, feedback.ErrorType)
	}
	
	stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
}

// updateRealtimeMetrics 更新实时指标
func (am *AccuracyMonitor) updateRealtimeMetrics(_ QueryFeedback) {
	now := time.Now()
	
	// 更新最近1小时准确率
	am.realtimeMetrics.LastHourAccuracy = am.getHourlyAccuracy(now)
	
	// 更新最近24小时准确率
	am.realtimeMetrics.Last24HourAccuracy = am.getDailyAccuracy(now)
	
	// 更新QPM（查询每分钟）
	am.realtimeMetrics.QueriesPerMinute = am.getQueriesPerMinute(now)
	
	// 检查错误率突增
	am.checkErrorSpike()
}

// updatePrometheusMetrics 更新Prometheus指标
func (am *AccuracyMonitor) updatePrometheusMetrics(feedback QueryFeedback) {
	// 更新整体准确率
	overallAccuracy := am.getCurrentAccuracy()
	am.metrics.overallAccuracy.Set(overallAccuracy)
	
	// 更新日准确率
	today := time.Now().Format("2006-01-02")
	if dailyStats := am.dailyStats[today]; dailyStats != nil {
		am.metrics.dailyAccuracy.Set(dailyStats.AccuracyRate)
	}
	
	// 用户评价
	if feedback.UserRating > 0 {
		am.metrics.userRatings.WithLabelValues(fmt.Sprintf("%d", feedback.UserRating)).Inc()
	}
	
	// 错误类型
	if !feedback.IsCorrect && feedback.ErrorType != "" {
		am.metrics.errorTypes.WithLabelValues(feedback.ErrorType, string(feedback.Category)).Inc()
	}
	
	// 类别准确率
	categoryStr := string(feedback.Category)
	if catStats := am.categoryStats[categoryStr]; catStats != nil {
		am.metrics.categoryAccuracy.WithLabelValues(categoryStr).Set(catStats.AccuracyRate)
	}
	
	// 模型准确率
	if modelStats := am.modelStats[feedback.ModelUsed]; modelStats != nil {
		am.metrics.modelAccuracy.WithLabelValues(feedback.ModelUsed).Set(modelStats.AccuracyRate)
	}
	
	// 处理时间
	am.metrics.processingTime.WithLabelValues(string(feedback.Category), string(feedback.Difficulty)).
		Observe(feedback.ProcessingTime.Seconds())
}

// getCurrentAccuracy 获取当前整体准确率
func (am *AccuracyMonitor) getCurrentAccuracy() float64 {
	totalQueries := 0
	correctQueries := 0
	
	for _, feedback := range am.feedbackStore {
		totalQueries++
		if feedback.IsCorrect {
			correctQueries++
		}
	}
	
	if totalQueries == 0 {
		return 0.0
	}
	
	return float64(correctQueries) / float64(totalQueries)
}

// getHourlyAccuracy 获取最近一小时准确率
func (am *AccuracyMonitor) getHourlyAccuracy(now time.Time) float64 {
	hourAgo := now.Add(-time.Hour)
	
	totalQueries := 0
	correctQueries := 0
	
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(hourAgo) {
			totalQueries++
			if feedback.IsCorrect {
				correctQueries++
			}
		}
	}
	
	if totalQueries == 0 {
		return 0.0
	}
	
	return float64(correctQueries) / float64(totalQueries)
}

// getDailyAccuracy 获取最近24小时准确率
func (am *AccuracyMonitor) getDailyAccuracy(now time.Time) float64 {
	dayAgo := now.Add(-24 * time.Hour)
	
	totalQueries := 0
	correctQueries := 0
	
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(dayAgo) {
			totalQueries++
			if feedback.IsCorrect {
				correctQueries++
			}
		}
	}
	
	if totalQueries == 0 {
		return 0.0
	}
	
	return float64(correctQueries) / float64(totalQueries)
}

// getQueriesPerMinute 获取每分钟查询数
func (am *AccuracyMonitor) getQueriesPerMinute(now time.Time) float64 {
	minuteAgo := now.Add(-time.Minute)
	
	count := 0
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(minuteAgo) {
			count++
		}
	}
	
	return float64(count)
}

// checkErrorSpike 检查错误率突增
func (am *AccuracyMonitor) checkErrorSpike() {
	currentErrorRate := 1.0 - am.realtimeMetrics.LastHourAccuracy
	historicalAvg := 1.0 - am.realtimeMetrics.Last24HourAccuracy
	
	// 如果当前错误率是历史平均值的2倍以上，认为是突增
	if currentErrorRate > historicalAvg*2.0 && currentErrorRate > 0.1 {
		am.realtimeMetrics.ErrorRateSpike = true
	} else {
		am.realtimeMetrics.ErrorRateSpike = false
	}
}

// checkAlerts 检查告警条件
func (am *AccuracyMonitor) checkAlerts() error {
	now := time.Now()
	
	// 检查准确率过低
	currentAccuracy := am.getCurrentAccuracy()
	if currentAccuracy < am.config.MinAccuracyThreshold {
		if am.shouldSendAlert("low_accuracy", now) {
			alert := &AccuracyAlert{
				ID:        fmt.Sprintf("low_accuracy_%d", now.Unix()),
				Type:      AlertTypeLowAccuracy,
				Level:     AlertLevelCritical,
				Title:     "SQL生成准确率过低",
				Message:   fmt.Sprintf("当前准确率 %.2f%% 低于最低阈值 %.2f%%", currentAccuracy*100, am.config.MinAccuracyThreshold*100),
				Timestamp: now,
				Details: map[string]any{
					"current_accuracy": currentAccuracy,
					"threshold":        am.config.MinAccuracyThreshold,
					"total_queries":    len(am.feedbackStore),
				},
				Actions: []RecommendedAction{
					{
						Action:      "review_prompts",
						Description: "检查和优化提示词模板",
						Priority:    1,
						AutoFix:     false,
					},
					{
						Action:      "retrain_model",
						Description: "考虑重新训练或调整模型参数",
						Priority:    2,
						AutoFix:     false,
					},
				},
			}
			
			if err := am.sendAlert(alert); err != nil {
				return fmt.Errorf("发送低准确率告警失败: %w", err)
			}
		}
	}
	
	// 检查错误率突增
	if am.realtimeMetrics.ErrorRateSpike {
		if am.shouldSendAlert("error_spike", now) {
			alert := &AccuracyAlert{
				ID:        fmt.Sprintf("error_spike_%d", now.Unix()),
				Type:      AlertTypeErrorSpike,
				Level:     AlertLevelWarning,
				Title:     "错误率突然增加",
				Message:   "检测到错误率异常增加，请关注",
				Timestamp: now,
				Details: map[string]any{
					"current_error_rate":    1.0 - am.realtimeMetrics.LastHourAccuracy,
					"historical_error_rate": 1.0 - am.realtimeMetrics.Last24HourAccuracy,
				},
			}
			
			if err := am.sendAlert(alert); err != nil {
				return fmt.Errorf("发送错误率突增告警失败: %w", err)
			}
		}
	}
	
	return nil
}

// shouldSendAlert 判断是否应该发送告警（考虑冷却时间）
func (am *AccuracyMonitor) shouldSendAlert(alertType string, now time.Time) bool {
	am.alertManager.mu.Lock()
	defer am.alertManager.mu.Unlock()
	
	if lastAlert, exists := am.alertManager.lastAlert[alertType]; exists {
		if now.Sub(lastAlert) < am.config.AlertCooldown {
			return false
		}
	}
	
	am.alertManager.lastAlert[alertType] = now
	return true
}

// sendAlert 发送告警
func (am *AccuracyMonitor) sendAlert(alert *AccuracyAlert) error {
	am.alertManager.mu.Lock()
	defer am.alertManager.mu.Unlock()
	
	am.alertManager.alerts[alert.ID] = alert
	
	am.logger.Warn("发送准确率告警",
		zap.String("alert_id", alert.ID),
		zap.String("type", string(alert.Type)),
		zap.String("level", string(alert.Level)),
		zap.String("title", alert.Title),
	)
	
	// 实现实际的告警发送逻辑
	go func() {
		if err := am.sendToWebhook(alert); err != nil {
			am.logger.Error("发送Webhook告警失败", zap.Error(err))
		}
		if err := am.sendToSlack(alert); err != nil {
			am.logger.Error("发送Slack告警失败", zap.Error(err))
		}
		if err := am.sendToEmail(alert); err != nil {
			am.logger.Error("发送邮件告警失败", zap.Error(err))
		}
	}()
	
	return nil
}

// GetAccuracyReport 生成准确率报告
func (am *AccuracyMonitor) GetAccuracyReport(ctx context.Context, days int) (*AccuracyReport, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)
	
	report := &AccuracyReport{
		StartDate:     startDate,
		EndDate:       endDate,
		GeneratedAt:   time.Now(),
		OverallStats:  am.getOverallStats(startDate, endDate),
		DailyStats:    am.getDailyStatsInRange(startDate, endDate),
		CategoryStats: am.getCategoryStatsInRange(startDate, endDate),
		ModelStats:    am.getModelStatsInRange(startDate, endDate),
		UserStats:     am.getTopUserStats(10),
		TopErrors:     am.getTopErrors(startDate, endDate, 10),
		Trends:        am.getTrends(days),
		Recommendations: am.generateRecommendations(),
	}
	
	return report, nil
}

// AccuracyReport 准确率报告
type AccuracyReport struct {
	StartDate       time.Time                        `json:"start_date"`
	EndDate         time.Time                        `json:"end_date"`
	GeneratedAt     time.Time                        `json:"generated_at"`
	OverallStats    *OverallStats                    `json:"overall_stats"`
	DailyStats      []*DailyStats                    `json:"daily_stats"`
	CategoryStats   map[string]*CategoryStats        `json:"category_stats"`
	ModelStats      map[string]*ModelStats           `json:"model_stats"`
	UserStats       []*UserStats                     `json:"user_stats"`
	TopErrors       []ErrorPattern                   `json:"top_errors"`
	Trends          *TrendAnalysis                   `json:"trends"`
	Recommendations []ImprovementRecommendation      `json:"recommendations"`
}

// OverallStats 整体统计
type OverallStats struct {
	TotalQueries    int           `json:"total_queries"`
	CorrectQueries  int           `json:"correct_queries"`
	AccuracyRate    float64       `json:"accuracy_rate"`
	AvgUserRating   float64       `json:"avg_user_rating"`
	AvgProcessTime  time.Duration `json:"avg_process_time"`
	TotalUsers      int           `json:"total_users"`
	FeedbackRate    float64       `json:"feedback_rate"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	AccuracyTrend      TrendDirection `json:"accuracy_trend"`
	UserSatisfaction   TrendDirection `json:"user_satisfaction_trend"`
	ProcessingTime     TrendDirection `json:"processing_time_trend"`
	TrendDescription   string         `json:"trend_description"`
	ForecastAccuracy   float64        `json:"forecast_accuracy"`   // 预测准确率
}

type TrendDirection string
const (
	TrendUpward   TrendDirection = "upward"
	TrendDownward TrendDirection = "downward"
	TrendStable   TrendDirection = "stable"
)

// ImprovementRecommendation 改进建议
type ImprovementRecommendation struct {
	Area        string             `json:"area"`
	Priority    int                `json:"priority"`
	Description string             `json:"description"`
	Actions     []RecommendedAction `json:"actions"`
	Impact      string             `json:"impact"`
	Effort      string             `json:"effort"`
}

// 实现各种统计方法...
func (am *AccuracyMonitor) getOverallStats(startDate, endDate time.Time) *OverallStats {
	totalQueries := 0
	correctQueries := 0
	totalRating := 0.0
	ratingCount := 0
	totalProcessTime := time.Duration(0)
	userSet := make(map[int64]bool)
	feedbackCount := 0

	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startDate) && feedback.Timestamp.Before(endDate) {
			totalQueries++
			if feedback.IsCorrect {
				correctQueries++
			}
			if feedback.UserRating > 0 {
				totalRating += float64(feedback.UserRating)
				ratingCount++
			}
			totalProcessTime += feedback.ProcessingTime
			userSet[feedback.UserID] = true
			if feedback.Feedback != "" {
				feedbackCount++
			}
		}
	}

	var accuracyRate, avgUserRating, feedbackRate float64
	var avgProcessTime time.Duration

	if totalQueries > 0 {
		accuracyRate = float64(correctQueries) / float64(totalQueries)
		avgProcessTime = totalProcessTime / time.Duration(totalQueries)
		feedbackRate = float64(feedbackCount) / float64(totalQueries)
	}
	if ratingCount > 0 {
		avgUserRating = totalRating / float64(ratingCount)
	}

	return &OverallStats{
		TotalQueries:   totalQueries,
		CorrectQueries: correctQueries,
		AccuracyRate:   accuracyRate,
		AvgUserRating:  avgUserRating,
		AvgProcessTime: avgProcessTime,
		TotalUsers:     len(userSet),
		FeedbackRate:   feedbackRate,
	}
}

func (am *AccuracyMonitor) getDailyStatsInRange(startDate, endDate time.Time) []*DailyStats {
	result := make([]*DailyStats, 0)
	
	for dateStr, stats := range am.dailyStats {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if date.After(startDate) && date.Before(endDate) {
			result = append(result, stats)
		}
	}
	
	return result
}

func (am *AccuracyMonitor) getCategoryStatsInRange(startDate, endDate time.Time) map[string]*CategoryStats {
	// 从反馈存储中重新计算类别统计
	categoryData := make(map[string]*CategoryStats)
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startDate) && feedback.Timestamp.Before(endDate) {
			categoryStr := string(feedback.Category)
			if categoryData[categoryStr] == nil {
				categoryData[categoryStr] = &CategoryStats{
					Category: feedback.Category,
				}
			}
			stats := categoryData[categoryStr]
			stats.TotalQueries++
			if feedback.IsCorrect {
				stats.CorrectQueries++
			}
			stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
		}
	}
	
	return categoryData
}

func (am *AccuracyMonitor) getModelStatsInRange(startDate, endDate time.Time) map[string]*ModelStats {
	// 从反馈存储中重新计算模型统计
	modelData := make(map[string]*ModelStats)
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startDate) && feedback.Timestamp.Before(endDate) {
			model := feedback.ModelUsed
			if model == "" {
				model = "default"
			}
			if modelData[model] == nil {
				modelData[model] = &ModelStats{
					ModelName: model,
				}
			}
			stats := modelData[model]
			stats.TotalQueries++
			if feedback.IsCorrect {
				stats.CorrectQueries++
			}
			stats.AccuracyRate = float64(stats.CorrectQueries) / float64(stats.TotalQueries)
			// 更新平均处理时间
			totalTime := stats.AvgProcessTime * time.Duration(stats.TotalQueries-1) + feedback.ProcessingTime
			stats.AvgProcessTime = totalTime / time.Duration(stats.TotalQueries)
			// 更新平均Token使用量
			totalTokens := stats.AvgTokensUsed * float64(stats.TotalQueries-1) + float64(feedback.TokensUsed)
			stats.AvgTokensUsed = totalTokens / float64(stats.TotalQueries)
		}
	}
	
	return modelData
}

func (am *AccuracyMonitor) getTopUserStats(limit int) []*UserStats {
	// 收集所有用户统计数据
	userStatsList := make([]*UserStats, 0, len(am.userStats))
	for _, stats := range am.userStats {
		// 只包含有一定查询量的用户（至少5个查询）
		if stats.TotalQueries >= 5 {
			userStatsList = append(userStatsList, &UserStats{
				UserID:           stats.UserID,
				TotalQueries:     stats.TotalQueries,
				CorrectQueries:   stats.CorrectQueries,
				AccuracyRate:     stats.AccuracyRate,
				AvgRating:        stats.AvgRating,
				FavoriteCategory: stats.FavoriteCategory,
				LastFeedback:     stats.LastFeedback,
				FeedbackCount:    stats.FeedbackCount,
			})
		}
	}
	
	// 按综合分数排序：准确率 * 0.6 + (查询数/最大查询数) * 0.4
	// 同时考虑用户活跃度和准确性
	if len(userStatsList) > 0 {
		// 找出最大查询数用于归一化
		maxQueries := 0
		for _, stats := range userStatsList {
			if stats.TotalQueries > maxQueries {
				maxQueries = stats.TotalQueries
			}
		}
		
		// 使用简单的冒泡排序，根据综合分数排序
		for i := 0; i < len(userStatsList)-1; i++ {
			for j := 0; j < len(userStatsList)-1-i; j++ {
				// 计算用户j的综合分数
				scoreJ := userStatsList[j].AccuracyRate*0.6 + (float64(userStatsList[j].TotalQueries)/float64(maxQueries))*0.4
				// 计算用户j+1的综合分数  
				scoreJ1 := userStatsList[j+1].AccuracyRate*0.6 + (float64(userStatsList[j+1].TotalQueries)/float64(maxQueries))*0.4
				
				if scoreJ < scoreJ1 {
					userStatsList[j], userStatsList[j+1] = userStatsList[j+1], userStatsList[j]
				}
			}
		}
	}
	
	// 限制返回数量
	if limit > 0 && len(userStatsList) > limit {
		return userStatsList[:limit]
	}
	
	return userStatsList
}

func (am *AccuracyMonitor) getTopErrors(startTime time.Time, endTime time.Time, limit int) []ErrorPattern {
	// 统计错误类型和示例
	errorCounts := make(map[string]int)
	errorExamples := make(map[string]string)
	errorQueries := make(map[string]string) // 保存出错的查询示例
	
	// 遍历指定时间范围内的所有反馈
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startTime) && feedback.Timestamp.Before(endTime) && !feedback.IsCorrect {
			errorType := feedback.ErrorType
			if errorType == "" {
				errorType = "未知错误" // 默认错误类型
			}
			
			// 统计错误次数
			errorCounts[errorType]++
			
			// 保存第一个遇到的错误示例和查询
			if _, exists := errorExamples[errorType]; !exists {
				errorExamples[errorType] = feedback.ErrorDetails
				if feedback.ErrorDetails == "" {
					errorExamples[errorType] = "SQL生成失败"
				}
				errorQueries[errorType] = feedback.UserQuery
			}
		}
	}
	
	// 转换为ErrorPattern切片
	errorPatterns := make([]ErrorPattern, 0, len(errorCounts))
	for errorType, count := range errorCounts {
		pattern := ErrorPattern{
			Pattern: errorType,
			Count:   count,
			Example: errorExamples[errorType],
			Suggestion: am.generateErrorSuggestion(errorType, errorQueries[errorType]),
		}
		errorPatterns = append(errorPatterns, pattern)
	}
	
	// 按错误次数排序（冒泡排序，从高到低）
	for i := 0; i < len(errorPatterns)-1; i++ {
		for j := 0; j < len(errorPatterns)-1-i; j++ {
			if errorPatterns[j].Count < errorPatterns[j+1].Count {
				errorPatterns[j], errorPatterns[j+1] = errorPatterns[j+1], errorPatterns[j]
			}
		}
	}
	
	// 限制返回数量
	if limit > 0 && len(errorPatterns) > limit {
		return errorPatterns[:limit]
	}
	
	return errorPatterns
}

// generateErrorSuggestion 根据错误类型生成改进建议
func (am *AccuracyMonitor) generateErrorSuggestion(errorType string, userQuery string) string {
	suggestions := map[string]string{
		"语法错误":     "检查SQL语法，确保关键字拼写正确，括号匹配",
		"表不存在":     "确认表名是否正确，检查数据库schema信息是否最新",
		"字段不存在":   "验证字段名是否正确，检查表结构定义",
		"类型转换错误": "检查数据类型匹配，避免不兼容的类型转换",
		"权限不足":     "确认用户有足够的数据库访问权限",
		"连接超时":     "检查数据库连接配置，优化查询性能",
		"JOIN错误":    "检查表关联条件，确保JOIN字段存在且类型匹配",
		"聚合函数错误": "确认聚合函数使用正确，GROUP BY子句完整",
		"子查询错误":   "检查子查询语法，确保返回正确的数据类型和数量",
		"未知错误":     "建议重新描述查询需求，提供更多上下文信息",
	}
	
	if suggestion, exists := suggestions[errorType]; exists {
		return suggestion
	}
	
	// 针对特定查询内容提供建议
	queryLower := strings.ToLower(userQuery)
	if strings.Contains(queryLower, "join") {
		return "检查表关联逻辑，确保JOIN条件和字段名正确"
	} else if strings.Contains(queryLower, "group") || strings.Contains(queryLower, "聚合") {
		return "检查GROUP BY和聚合函数的使用，确保语法正确"
	} else if strings.Contains(queryLower, "时间") || strings.Contains(queryLower, "日期") {
		return "检查日期时间字段格式和函数使用"
	}
	
	return "建议简化查询条件，分步骤构建复杂查询"
}

func (am *AccuracyMonitor) getTrends(days int) *TrendAnalysis {
	now := time.Now()
	startDate := now.AddDate(0, 0, -days)
	
	// 收集时间序列数据
	dailyAccuracy := make([]float64, 0)
	dailySatisfaction := make([]float64, 0)
	dailyProcessTime := make([]float64, 0)
	
	// 按日期收集数据
	for i := range days {
		date := startDate.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		
		// 从每日统计获取准确率
		if stats, exists := am.dailyStats[dateStr]; exists {
			dailyAccuracy = append(dailyAccuracy, stats.AccuracyRate)
		} else {
			// 如果没有该日数据，从原始反馈中计算
			accuracy := am.calculateDayAccuracy(date)
			dailyAccuracy = append(dailyAccuracy, accuracy)
		}
		
		// 计算当日用户满意度和处理时间
		satisfaction, processTime := am.calculateDayMetrics(date)
		dailySatisfaction = append(dailySatisfaction, satisfaction)
		dailyProcessTime = append(dailyProcessTime, processTime)
	}
	
	// 分析趋势方向
	accuracyTrend := am.analyzeTrend(dailyAccuracy)
	satisfactionTrend := am.analyzeTrend(dailySatisfaction)
	processTimeTrend := am.analyzeTrendReverse(dailyProcessTime) // 处理时间越低越好，所以趋势相反
	
	// 生成趋势描述
	description := am.generateTrendDescription(accuracyTrend, satisfactionTrend, processTimeTrend)
	
	// 简单预测（使用最近3天的平均值）
	forecastAccuracy := am.forecastAccuracy(dailyAccuracy)
	
	return &TrendAnalysis{
		AccuracyTrend:      accuracyTrend,
		UserSatisfaction:   satisfactionTrend,
		ProcessingTime:     processTimeTrend,
		TrendDescription:   description,
		ForecastAccuracy:   forecastAccuracy,
	}
}

// calculateDayAccuracy 计算指定日期的准确率
func (am *AccuracyMonitor) calculateDayAccuracy(date time.Time) float64 {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)
	
	totalQueries := 0
	correctQueries := 0
	
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startOfDay) && feedback.Timestamp.Before(endOfDay) {
			totalQueries++
			if feedback.IsCorrect {
				correctQueries++
			}
		}
	}
	
	if totalQueries == 0 {
		return 0.0
	}
	
	return float64(correctQueries) / float64(totalQueries)
}

// calculateDayMetrics 计算指定日期的用户满意度和处理时间
func (am *AccuracyMonitor) calculateDayMetrics(date time.Time) (satisfaction float64, avgProcessTime float64) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)
	
	totalRating := 0
	ratingCount := 0
	totalProcessTime := time.Duration(0)
	processTimeCount := 0
	
	for _, feedback := range am.feedbackStore {
		if feedback.Timestamp.After(startOfDay) && feedback.Timestamp.Before(endOfDay) {
			if feedback.UserRating > 0 {
				totalRating += feedback.UserRating
				ratingCount++
			}
			
			if feedback.ProcessingTime > 0 {
				totalProcessTime += feedback.ProcessingTime
				processTimeCount++
			}
		}
	}
	
	if ratingCount > 0 {
		satisfaction = float64(totalRating) / float64(ratingCount) / 5.0 // 标准化到0-1
	}
	
	if processTimeCount > 0 {
		avgProcessTime = float64(totalProcessTime.Milliseconds()) / float64(processTimeCount)
	}
	
	return satisfaction, avgProcessTime
}

// analyzeTrend 分析数据趋势方向
func (am *AccuracyMonitor) analyzeTrend(data []float64) TrendDirection {
	if len(data) < 2 {
		return TrendStable
	}
	
	// 使用简单的线性趋势分析
	// 计算最近一半数据与前一半数据的平均值差异
	halfPoint := len(data) / 2
	
	firstHalfSum := 0.0
	firstHalfCount := 0
	for i := range halfPoint {
		if data[i] > 0 { // 只计算有效数据
			firstHalfSum += data[i]
			firstHalfCount++
		}
	}
	
	secondHalfSum := 0.0
	secondHalfCount := 0
	for i := halfPoint; i < len(data); i++ {
		if data[i] > 0 { // 只计算有效数据
			secondHalfSum += data[i]
			secondHalfCount++
		}
	}
	
	if firstHalfCount == 0 || secondHalfCount == 0 {
		return TrendStable
	}
	
	firstHalfAvg := firstHalfSum / float64(firstHalfCount)
	secondHalfAvg := secondHalfSum / float64(secondHalfCount)
	
	diff := secondHalfAvg - firstHalfAvg
	threshold := 0.05 // 5%的变化阈值
	
	if diff > threshold {
		return TrendUpward
	} else if diff < -threshold {
		return TrendDownward
	} else {
		return TrendStable
	}
}

// analyzeTrendReverse 分析反向趋势（用于处理时间，越低越好）
func (am *AccuracyMonitor) analyzeTrendReverse(data []float64) TrendDirection {
	baseTrend := am.analyzeTrend(data)
	switch baseTrend {
	case TrendUpward:
		return TrendDownward
	case TrendDownward:
		return TrendUpward
	default:
		return TrendStable
	}
}

// generateTrendDescription 生成趋势描述
func (am *AccuracyMonitor) generateTrendDescription(accuracy, satisfaction, processTime TrendDirection) string {
	descriptions := []string{}
	
	switch accuracy {
	case TrendUpward:
		descriptions = append(descriptions, "SQL生成准确率呈上升趋势")
	case TrendDownward:
		descriptions = append(descriptions, "SQL生成准确率呈下降趋势，需要关注")
	case TrendStable:
		descriptions = append(descriptions, "SQL生成准确率保持稳定")
	}
	
	switch satisfaction {
	case TrendUpward:
		descriptions = append(descriptions, "用户满意度持续改善")
	case TrendDownward:
		descriptions = append(descriptions, "用户满意度有所下降")
	case TrendStable:
		descriptions = append(descriptions, "用户满意度保持稳定")
	}
	
	switch processTime {
	case TrendUpward:
		descriptions = append(descriptions, "查询处理性能持续优化")
	case TrendDownward:
		descriptions = append(descriptions, "查询处理时间有所增加")
	case TrendStable:
		descriptions = append(descriptions, "查询处理性能保持稳定")
	}
	
	// 综合评价
	if accuracy == TrendUpward && satisfaction == TrendUpward && processTime == TrendUpward {
		descriptions = append(descriptions, "整体表现优秀，各项指标均呈现良好趋势")
	} else if accuracy == TrendDownward || satisfaction == TrendDownward {
		descriptions = append(descriptions, "建议关注服务质量，及时调优")
	}
	
	return strings.Join(descriptions, "；")
}

// forecastAccuracy 简单的准确率预测
func (am *AccuracyMonitor) forecastAccuracy(data []float64) float64 {
	if len(data) == 0 {
		return 0.0
	}
	
	// 使用最近几天的加权平均进行预测
	validData := make([]float64, 0)
	for _, val := range data {
		if val > 0 {
			validData = append(validData, val)
		}
	}
	
	if len(validData) == 0 {
		return 0.0
	}
	
	// 最近的数据权重更高
	totalWeight := 0.0
	weightedSum := 0.0
	
	for i, val := range validData {
		weight := float64(i+1) // 权重递增
		weightedSum += val * weight
		totalWeight += weight
	}
	
	forecast := weightedSum / totalWeight
	
	// 限制预测值在合理范围内
	if forecast > 1.0 {
		forecast = 1.0
	} else if forecast < 0.0 {
		forecast = 0.0
	}
	
	return forecast
}

func (am *AccuracyMonitor) generateRecommendations() []ImprovementRecommendation {
	// 实现改进建议生成
	return []ImprovementRecommendation{}
}

// AddDataPoint 添加趋势数据点
func (ta *TrendAnalyzer) AddDataPoint(accuracy float64) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	
	// 添加到小时数据（滚动窗口，最多保留24个数据点）
	ta.hourlyData = append(ta.hourlyData, accuracy)
	if len(ta.hourlyData) > 24 {
		ta.hourlyData = ta.hourlyData[1:]
	}
	
	// 实现日数据和周数据的更新逻辑
	// 每24个小时数据点计算一个日平均值
	if len(ta.hourlyData) == 24 {
		sum := 0.0
		for _, val := range ta.hourlyData {
			sum += val
		}
		dailyAvg := sum / 24.0
		
		ta.dailyData = append(ta.dailyData, dailyAvg)
		if len(ta.dailyData) > 30 {
			ta.dailyData = ta.dailyData[1:]
		}
	}
	
	// 每7个日数据点计算一个周平均值  
	if len(ta.dailyData) >= 7 && len(ta.dailyData)%7 == 0 {
		sum := 0.0
		for i := len(ta.dailyData) - 7; i < len(ta.dailyData); i++ {
			sum += ta.dailyData[i]
		}
		weeklyAvg := sum / 7.0
		
		ta.weeklyData = append(ta.weeklyData, weeklyAvg)
		if len(ta.weeklyData) > 12 {
			ta.weeklyData = ta.weeklyData[1:]
		}
	}
}

// GetMetrics 获取监控指标
func (am *AccuracyMonitor) GetMetrics() map[string]any {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	// 计算总查询数和正确查询数
	totalQueries := len(am.feedbackStore)
	correctQueries := 0
	totalRating := 0
	totalResponseTime := int64(0)
	
	for _, feedback := range am.feedbackStore {
		if feedback.IsCorrect {
			correctQueries++
		}
		totalRating += feedback.UserRating
		totalResponseTime += feedback.ProcessingTime.Milliseconds()
	}
	
	// 计算当前准确率
	currentAccuracy := 0.0
	if totalQueries > 0 {
		currentAccuracy = float64(correctQueries) / float64(totalQueries)
	}
	
	// 计算错误率
	errorRate := 1.0 - currentAccuracy
	
	// 计算平均置信度（基于用户评分，1-5分映射到0-1）
	avgConfidence := 0.0
	if totalQueries > 0 {
		avgConfidence = float64(totalRating) / float64(totalQueries) / 5.0
	}
	
	// 计算平均响应时间（毫秒）
	avgResponseTime := int64(0)
	if totalQueries > 0 {
		avgResponseTime = totalResponseTime / int64(totalQueries)
	}
	
	return map[string]any{
		// 测试期望的基本指标
		"current_accuracy":      currentAccuracy,
		"total_queries":         totalQueries,
		"correct_queries":       correctQueries,
		"error_rate":           errorRate,
		"avg_confidence":        avgConfidence,
		"avg_response_time":     avgResponseTime,
		
		// 复合指标
		"category_breakdown":    am.getCategoryBreakdown(),
		"daily_accuracy":        am.realtimeMetrics.Last24HourAccuracy,
		"queries_per_minute":    am.realtimeMetrics.QueriesPerMinute,
		
		// 额外指标（兼容性）
		"overall_accuracy":      currentAccuracy,
		"total_feedback_count":  totalQueries,
		"active_alerts":         len(am.alertManager.alerts),
		"model_performance":     am.realtimeMetrics.ModelPerformance,
	}
}

func (am *AccuracyMonitor) getCategoryBreakdown() map[string]float64 {
	breakdown := make(map[string]float64)
	for category, stats := range am.categoryStats {
		breakdown[category] = stats.AccuracyRate
	}
	return breakdown
}

// GetRegistry 获取Prometheus注册表，用于将指标注册到全局注册表
func (am *AccuracyMonitor) GetRegistry() *prometheus.Registry {
	return am.metrics.registry
}

// RegisterToGlobal 将指标注册到全局Prometheus注册表（生产环境使用）
func (am *AccuracyMonitor) RegisterToGlobal() error {
	prometheus.DefaultRegisterer.MustRegister(
		am.metrics.overallAccuracy,
		am.metrics.dailyAccuracy,
		am.metrics.userRatings,
		am.metrics.errorTypes,
		am.metrics.categoryAccuracy,
		am.metrics.modelAccuracy,
		am.metrics.processingTime,
		am.metrics.feedbackCount,
		am.metrics.improvementSuggestions,
	)
	return nil
}

// 告警发送方法实现

// sendToWebhook 发送告警到 Webhook
func (am *AccuracyMonitor) sendToWebhook(alert *AccuracyAlert) error {
	if am.alertManager.webhookURL == "" {
		return nil // 没有配置 webhook，跳过
	}
	
	// 这里实现 HTTP POST 到 webhook URL
	// 简化实现：只记录日志
	am.logger.Info("发送 Webhook 告警",
		zap.String("alert_id", alert.ID),
		zap.String("webhook_url", am.alertManager.webhookURL))
	return nil
}

// sendToSlack 发送告警到 Slack
func (am *AccuracyMonitor) sendToSlack(alert *AccuracyAlert) error {
	if am.alertManager.slackToken == "" {
		return nil // 没有配置 Slack，跳过
	}
	
	// 这里实现 Slack API 调用
	// 简化实现：只记录日志
	am.logger.Info("发送 Slack 告警",
		zap.String("alert_id", alert.ID),
		zap.String("title", alert.Title))
	return nil
}

// sendToEmail 发送告警邮件
func (am *AccuracyMonitor) sendToEmail(alert *AccuracyAlert) error {
	if am.alertManager.emailConfig == nil {
		return nil // 没有配置邮件，跳过
	}
	
	// 这里实现 SMTP 邮件发送
	// 简化实现：只记录日志
	am.logger.Info("发送邮件告警",
		zap.String("alert_id", alert.ID),
		zap.String("title", alert.Title),
		zap.Strings("recipients", am.alertManager.emailConfig.ToEmails))
	return nil
}