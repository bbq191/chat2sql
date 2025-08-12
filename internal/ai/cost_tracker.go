// Package ai 成本追踪器和监控
package ai

import (
	"sync"
	"time"
)

// CostTracker 成本追踪器
type CostTracker struct {
	// 使用量数据
	dailyUsage   map[string]*DailyUsage   // date -> usage
	userUsage    map[int64]*UserUsage     // userID -> usage  
	modelCosts   map[string]*ModelCost    // modelName -> cost info
	
	// 配置参数
	config *CostConfig
	
	// 并发安全
	mu sync.RWMutex
	
	// 告警管理
	alerts *AlertManager
}

// CostConfig 成本配置
type CostConfig struct {
	// 预算限制
	DailyBudget   float64 `yaml:"daily_budget"`   // 每日预算限制(USD)
	UserDailyLimit float64 `yaml:"user_daily_limit"` // 每用户每日限制
	QueryCostLimit float64 `yaml:"query_cost_limit"` // 单次查询成本限制
	
	// 告警阈值
	DailyAlertThreshold  float64 `yaml:"daily_alert_threshold"`  // 每日告警阈值(百分比)
	UserAlertThreshold   float64 `yaml:"user_alert_threshold"`   // 用户告警阈值
	ModelAlertThreshold  float64 `yaml:"model_alert_threshold"`  // 模型告警阈值
	
	// 追踪配置
	EnableDetailedTracking bool `yaml:"enable_detailed_tracking"` // 启用详细追踪
	RetentionDays         int  `yaml:"retention_days"`          // 数据保留天数
	
	// 预估配置
	EnableCostPrediction bool `yaml:"enable_cost_prediction"` // 启用成本预测
}

// DailyUsage 每日使用量
type DailyUsage struct {
	Date         string             `json:"date"`
	TotalCost    float64            `json:"total_cost"`
	TotalTokens  int                `json:"total_tokens"`
	QueryCount   int                `json:"query_count"`
	ModelUsage   map[string]*Usage  `json:"model_usage"`
	UserCounts   map[int64]int      `json:"user_counts"`
	LastUpdated  time.Time          `json:"last_updated"`
}

// UserUsage 用户使用量
type UserUsage struct {
	UserID       int64              `json:"user_id"`
	DailyCost    float64            `json:"daily_cost"`
	DailyTokens  int                `json:"daily_tokens"`
	DailyQueries int                `json:"daily_queries"`
	ModelUsage   map[string]*Usage  `json:"model_usage"`
	QueryHistory []QueryCost        `json:"query_history"`
	LastQuery    time.Time          `json:"last_query"`
	LastReset    time.Time          `json:"last_reset"`
}

// Usage 使用统计
type Usage struct {
	QueryCount   int     `json:"query_count"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

// QueryCost 查询成本记录
type QueryCost struct {
	Timestamp    time.Time `json:"timestamp"`
	Query        string    `json:"query"`
	ModelName    string    `json:"model_name"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	Cost         float64   `json:"cost"`
	ProcessTime  int64     `json:"process_time_ms"`
}

// ModelCost 模型成本配置
type ModelCost struct {
	ModelName       string  `json:"model_name"`
	Provider        string  `json:"provider"`
	InputCostPer1K  float64 `json:"input_cost_per_1k"`   // 每1K输入tokens的成本
	OutputCostPer1K float64 `json:"output_cost_per_1k"`  // 每1K输出tokens的成本
	MinimumCost     float64 `json:"minimum_cost"`        // 最小成本
	LastUpdated     time.Time `json:"last_updated"`
}

// CostSummary 成本总结
type CostSummary struct {
	Today        CostPeriod            `json:"today"`
	Yesterday    CostPeriod            `json:"yesterday"`
	ThisWeek     CostPeriod            `json:"this_week"`
	ThisMonth    CostPeriod            `json:"this_month"`
	TopUsers     []UserCostSummary     `json:"top_users"`
	TopModels    []ModelCostSummary    `json:"top_models"`
	Predictions  CostPrediction        `json:"predictions"`
	Alerts       []CostAlert           `json:"alerts"`
	LastUpdated  time.Time             `json:"last_updated"`
}

// CostPeriod 成本周期统计
type CostPeriod struct {
	Period       string  `json:"period"`
	TotalCost    float64 `json:"total_cost"`
	TotalTokens  int     `json:"total_tokens"`
	QueryCount   int     `json:"query_count"`
	UserCount    int     `json:"user_count"`
	AvgCostPerQuery float64 `json:"avg_cost_per_query"`
}

// UserCostSummary 用户成本总结
type UserCostSummary struct {
	UserID       int64   `json:"user_id"`
	TotalCost    float64 `json:"total_cost"`
	QueryCount   int     `json:"query_count"`
	FavoriteModel string `json:"favorite_model"`
}

// ModelCostSummary 模型成本总结
type ModelCostSummary struct {
	ModelName    string  `json:"model_name"`
	TotalCost    float64 `json:"total_cost"`
	QueryCount   int     `json:"query_count"`
	AvgCostPerQuery float64 `json:"avg_cost_per_query"`
}

// CostPrediction 成本预测
type CostPrediction struct {
	DailyTrend      float64 `json:"daily_trend"`       // 每日趋势
	WeeklyForecast  float64 `json:"weekly_forecast"`   // 周预测
	MonthlyForecast float64 `json:"monthly_forecast"`  // 月预测
	BudgetWarning   bool    `json:"budget_warning"`    // 预算警告
}

// CostAlert 成本告警
type CostAlert struct {
	Type        string    `json:"type"`        // 告警类型
	Level       string    `json:"level"`       // 告警级别
	Message     string    `json:"message"`     // 告警消息
	Value       float64   `json:"value"`       // 当前值
	Threshold   float64   `json:"threshold"`   // 阈值
	Timestamp   time.Time `json:"timestamp"`   // 时间戳
	UserID      int64     `json:"user_id,omitempty"` // 用户ID(如果相关)
}

// AlertManager 告警管理器
type AlertManager struct {
	alerts []CostAlert
	config *CostConfig
	mu     sync.RWMutex
}

// NewCostTracker 创建新的成本追踪器
func NewCostTracker(config *CostConfig) *CostTracker {
	if config == nil {
		config = &CostConfig{
			DailyBudget:           100.0,
			UserDailyLimit:       10.0,
			QueryCostLimit:       0.50,
			DailyAlertThreshold:  0.8,
			UserAlertThreshold:   0.8,
			ModelAlertThreshold:  0.8,
			EnableDetailedTracking: true,
			RetentionDays:        30,
			EnableCostPrediction: true,
		}
	}
	
	ct := &CostTracker{
		dailyUsage: make(map[string]*DailyUsage),
		userUsage:  make(map[int64]*UserUsage),
		modelCosts: make(map[string]*ModelCost),
		config:     config,
		alerts:     &AlertManager{
			alerts: make([]CostAlert, 0),
			config: config,
		},
	}
	
	// 初始化模型成本配置
	ct.initializeModelCosts()
	
	return ct
}

// initializeModelCosts 初始化模型成本配置
func (ct *CostTracker) initializeModelCosts() {
	// OpenAI 模型成本 (2024年价格，单位：USD per 1K tokens)
	ct.modelCosts["gpt-4o"] = &ModelCost{
		ModelName:       "gpt-4o",
		Provider:        "openai",
		InputCostPer1K:  0.015,
		OutputCostPer1K: 0.060,
		MinimumCost:     0.001,
		LastUpdated:     time.Now(),
	}
	
	ct.modelCosts["gpt-4o-mini"] = &ModelCost{
		ModelName:       "gpt-4o-mini",
		Provider:        "openai", 
		InputCostPer1K:  0.00015,
		OutputCostPer1K: 0.00060,
		MinimumCost:     0.0001,
		LastUpdated:     time.Now(),
	}
	
	ct.modelCosts["gpt-3.5-turbo"] = &ModelCost{
		ModelName:       "gpt-3.5-turbo",
		Provider:        "openai",
		InputCostPer1K:  0.0015,
		OutputCostPer1K: 0.002,
		MinimumCost:     0.0001,
		LastUpdated:     time.Now(),
	}
	
	// Anthropic Claude 模型成本
	ct.modelCosts["claude-3-opus-20240229"] = &ModelCost{
		ModelName:       "claude-3-opus-20240229",
		Provider:        "anthropic",
		InputCostPer1K:  0.015,
		OutputCostPer1K: 0.075,
		MinimumCost:     0.001,
		LastUpdated:     time.Now(),
	}
	
	ct.modelCosts["claude-3-haiku-20240307"] = &ModelCost{
		ModelName:       "claude-3-haiku-20240307",
		Provider:        "anthropic",
		InputCostPer1K:  0.00025,
		OutputCostPer1K: 0.00125,
		MinimumCost:     0.0001,
		LastUpdated:     time.Now(),
	}
}

// CalculateQueryCost 计算查询成本
func (ct *CostTracker) CalculateQueryCost(inputTokens, outputTokens int, modelName string) float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	modelCost, exists := ct.modelCosts[modelName]
	if !exists {
		// 使用默认成本
		modelCost = &ModelCost{
			InputCostPer1K:  0.002,
			OutputCostPer1K: 0.004,
			MinimumCost:     0.0001,
		}
	}
	
	// 计算成本
	inputCost := float64(inputTokens) * modelCost.InputCostPer1K / 1000.0
	outputCost := float64(outputTokens) * modelCost.OutputCostPer1K / 1000.0
	totalCost := inputCost + outputCost
	
	// 应用最小成本
	if totalCost < modelCost.MinimumCost {
		totalCost = modelCost.MinimumCost
	}
	
	return totalCost
}

// RecordQueryCost 记录查询成本
func (ct *CostTracker) RecordQueryCost(userID int64, cost QueryCost) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	today := time.Now().Format("2006-01-02")
	
	// 更新每日使用量
	ct.updateDailyUsage(today, cost)
	
	// 更新用户使用量
	ct.updateUserUsage(userID, cost)
	
	// 检查预算限制
	if err := ct.checkBudgetLimits(userID, cost.Cost); err != nil {
		// 记录告警但不阻止操作
		ct.addAlert(CostAlert{
			Type:      "budget_exceeded",
			Level:     "warning",
			Message:   err.Error(),
			Value:     cost.Cost,
			Threshold: ct.config.QueryCostLimit,
			Timestamp: time.Now(),
			UserID:    userID,
		})
	}
	
	return nil
}

// updateDailyUsage 更新每日使用量
func (ct *CostTracker) updateDailyUsage(date string, cost QueryCost) {
	usage, exists := ct.dailyUsage[date]
	if !exists {
		usage = &DailyUsage{
			Date:        date,
			ModelUsage:  make(map[string]*Usage),
			UserCounts:  make(map[int64]int),
			LastUpdated: time.Now(),
		}
		ct.dailyUsage[date] = usage
	}
	
	// 更新总计
	usage.TotalCost += cost.Cost
	usage.TotalTokens += cost.TotalTokens
	usage.QueryCount++
	usage.LastUpdated = time.Now()
	
	// 更新模型使用量
	if _, exists := usage.ModelUsage[cost.ModelName]; !exists {
		usage.ModelUsage[cost.ModelName] = &Usage{}
	}
	modelUsage := usage.ModelUsage[cost.ModelName]
	modelUsage.QueryCount++
	modelUsage.InputTokens += cost.InputTokens
	modelUsage.OutputTokens += cost.OutputTokens
	modelUsage.TotalTokens += cost.TotalTokens
	modelUsage.Cost += cost.Cost
}

// updateUserUsage 更新用户使用量
func (ct *CostTracker) updateUserUsage(userID int64, cost QueryCost) {
	// 检查是否需要重置每日计数器
	ct.resetUserDailyUsageIfNeeded(userID)
	
	usage, exists := ct.userUsage[userID]
	if !exists {
		usage = &UserUsage{
			UserID:       userID,
			ModelUsage:   make(map[string]*Usage),
			QueryHistory: make([]QueryCost, 0, 100),
			LastReset:    time.Now(),
		}
		ct.userUsage[userID] = usage
	}
	
	// 更新每日统计
	usage.DailyCost += cost.Cost
	usage.DailyTokens += cost.TotalTokens
	usage.DailyQueries++
	usage.LastQuery = time.Now()
	
	// 更新模型使用量
	if _, exists := usage.ModelUsage[cost.ModelName]; !exists {
		usage.ModelUsage[cost.ModelName] = &Usage{}
	}
	modelUsage := usage.ModelUsage[cost.ModelName]
	modelUsage.QueryCount++
	modelUsage.InputTokens += cost.InputTokens
	modelUsage.OutputTokens += cost.OutputTokens
	modelUsage.TotalTokens += cost.TotalTokens
	modelUsage.Cost += cost.Cost
	
	// 添加到查询历史
	usage.QueryHistory = append(usage.QueryHistory, cost)
	if len(usage.QueryHistory) > 100 {
		usage.QueryHistory = usage.QueryHistory[1:] // 保留最近100条
	}
}

// resetUserDailyUsageIfNeeded 如果需要则重置用户每日使用量
func (ct *CostTracker) resetUserDailyUsageIfNeeded(userID int64) {
	usage, exists := ct.userUsage[userID]
	if !exists {
		return
	}
	
	now := time.Now()
	if now.Format("2006-01-02") != usage.LastReset.Format("2006-01-02") {
		// 新的一天，重置每日计数器
		usage.DailyCost = 0
		usage.DailyTokens = 0
		usage.DailyQueries = 0
		usage.LastReset = now
	}
}

// checkBudgetLimits 检查预算限制
func (ct *CostTracker) checkBudgetLimits(userID int64, queryCost float64) error {
	// 检查单次查询成本限制
	if queryCost > ct.config.QueryCostLimit {
		return NewCostLimitError("single_query", queryCost, ct.config.QueryCostLimit)
	}
	
	// 检查用户每日限制
	if userUsage, exists := ct.userUsage[userID]; exists {
		if userUsage.DailyCost+queryCost > ct.config.UserDailyLimit {
			return NewCostLimitError("user_daily", userUsage.DailyCost+queryCost, ct.config.UserDailyLimit)
		}
	}
	
	// 检查每日总预算
	today := time.Now().Format("2006-01-02")
	if dailyUsage, exists := ct.dailyUsage[today]; exists {
		if dailyUsage.TotalCost+queryCost > ct.config.DailyBudget {
			return NewCostLimitError("daily_budget", dailyUsage.TotalCost+queryCost, ct.config.DailyBudget)
		}
	}
	
	return nil
}

// GetCostSummary 获取成本总结
func (ct *CostTracker) GetCostSummary() *CostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	now := time.Now()
	summary := &CostSummary{
		LastUpdated: now,
	}
	
	// 今日统计
	today := now.Format("2006-01-02")
	if todayUsage, exists := ct.dailyUsage[today]; exists {
		summary.Today = CostPeriod{
			Period:      "today",
			TotalCost:   todayUsage.TotalCost,
			TotalTokens: todayUsage.TotalTokens,
			QueryCount:  todayUsage.QueryCount,
			UserCount:   len(todayUsage.UserCounts),
		}
		if todayUsage.QueryCount > 0 {
			summary.Today.AvgCostPerQuery = todayUsage.TotalCost / float64(todayUsage.QueryCount)
		}
	} else {
		// 如果没有今日数据，初始化空的今日统计
		summary.Today = CostPeriod{Period: "today"}
	}
	
	// 昨日统计
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	if yesterdayUsage, exists := ct.dailyUsage[yesterday]; exists {
		summary.Yesterday = CostPeriod{
			Period:      "yesterday",
			TotalCost:   yesterdayUsage.TotalCost,
			TotalTokens: yesterdayUsage.TotalTokens,
			QueryCount:  yesterdayUsage.QueryCount,
			UserCount:   len(yesterdayUsage.UserCounts),
		}
		if yesterdayUsage.QueryCount > 0 {
			summary.Yesterday.AvgCostPerQuery = yesterdayUsage.TotalCost / float64(yesterdayUsage.QueryCount)
		}
	} else {
		// 如果没有昨日数据，初始化空的昨日统计
		summary.Yesterday = CostPeriod{Period: "yesterday"}
	}
	
	// 本周统计 
	summary.ThisWeek = CostPeriod{Period: "this_week"}
	
	// 本月统计
	summary.ThisMonth = CostPeriod{Period: "this_month"}
	
	// 获取用户排名
	summary.TopUsers = ct.getTopUsers(5)
	if summary.TopUsers == nil {
		summary.TopUsers = []UserCostSummary{}
	}
	
	// 获取模型排名
	summary.TopModels = ct.getTopModels(5)
	if summary.TopModels == nil {
		summary.TopModels = []ModelCostSummary{}
	}
	
	// 成本预测
	if ct.config != nil && ct.config.EnableCostPrediction {
		summary.Predictions = ct.generateCostPrediction()
	} else {
		summary.Predictions = CostPrediction{}
	}
	
	// 获取活跃告警
	summary.Alerts = ct.getActiveAlerts()
	if summary.Alerts == nil {
		summary.Alerts = []CostAlert{}
	}
	
	return summary
}

// getTopUsers 获取用户消费排名
func (ct *CostTracker) getTopUsers(limit int) []UserCostSummary {
	type userCost struct {
		userID int64
		cost   float64
		queries int
		favoriteModel string
	}
	
	var users []userCost
	for userID, usage := range ct.userUsage {
		// 找出最常用的模型
		var favoriteModel string
		var maxQueries int
		for modelName, modelUsage := range usage.ModelUsage {
			if modelUsage.QueryCount > maxQueries {
				maxQueries = modelUsage.QueryCount
				favoriteModel = modelName
			}
		}
		
		users = append(users, userCost{
			userID:        userID,
			cost:          usage.DailyCost,
			queries:       usage.DailyQueries,
			favoriteModel: favoriteModel,
		})
	}
	
	// 按成本排序
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			if users[i].cost < users[j].cost {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
	
	// 转换为结果格式
	var result []UserCostSummary
	for i, user := range users {
		if i >= limit {
			break
		}
		result = append(result, UserCostSummary{
			UserID:        user.userID,
			TotalCost:     user.cost,
			QueryCount:    user.queries,
			FavoriteModel: user.favoriteModel,
		})
	}
	
	return result
}

// getTopModels 获取模型使用排名
func (ct *CostTracker) getTopModels(limit int) []ModelCostSummary {
	modelStats := make(map[string]*ModelCostSummary)
	
	today := time.Now().Format("2006-01-02")
	if todayUsage, exists := ct.dailyUsage[today]; exists {
		for modelName, usage := range todayUsage.ModelUsage {
			modelStats[modelName] = &ModelCostSummary{
				ModelName:   modelName,
				TotalCost:   usage.Cost,
				QueryCount:  usage.QueryCount,
			}
			if usage.QueryCount > 0 {
				modelStats[modelName].AvgCostPerQuery = usage.Cost / float64(usage.QueryCount)
			}
		}
	}
	
	// 转换为切片并排序
	var models []ModelCostSummary
	for _, model := range modelStats {
		models = append(models, *model)
	}
	
	// 按成本排序
	for i := 0; i < len(models)-1; i++ {
		for j := i + 1; j < len(models); j++ {
			if models[i].TotalCost < models[j].TotalCost {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
	
	// 返回前N个
	if len(models) > limit {
		models = models[:limit]
	}
	
	return models
}

// generateCostPrediction 生成成本预测
func (ct *CostTracker) generateCostPrediction() CostPrediction {
	prediction := CostPrediction{}
	
	// 简单的趋势分析
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	
	todayCost := 0.0
	yesterdayCost := 0.0
	
	if todayUsage, exists := ct.dailyUsage[today]; exists {
		todayCost = todayUsage.TotalCost
	}
	
	if yesterdayUsage, exists := ct.dailyUsage[yesterday]; exists {
		yesterdayCost = yesterdayUsage.TotalCost
	}
	
	// 计算趋势
	if yesterdayCost > 0 {
		prediction.DailyTrend = (todayCost - yesterdayCost) / yesterdayCost
	}
	
	// 简单预测
	prediction.WeeklyForecast = todayCost * 7
	prediction.MonthlyForecast = todayCost * 30
	
	// 预算警告
	if todayCost > ct.config.DailyBudget*ct.config.DailyAlertThreshold {
		prediction.BudgetWarning = true
	}
	
	return prediction
}

// getActiveAlerts 获取活跃告警
func (ct *CostTracker) getActiveAlerts() []CostAlert {
	ct.alerts.mu.RLock()
	defer ct.alerts.mu.RUnlock()
	
	// 返回最近24小时的告警
	var activeAlerts []CostAlert
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for _, alert := range ct.alerts.alerts {
		if alert.Timestamp.After(cutoff) {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	
	return activeAlerts
}

// addAlert 添加告警
func (ct *CostTracker) addAlert(alert CostAlert) {
	ct.alerts.mu.Lock()
	defer ct.alerts.mu.Unlock()
	
	ct.alerts.alerts = append(ct.alerts.alerts, alert)
	
	// 保持告警列表大小
	if len(ct.alerts.alerts) > 1000 {
		ct.alerts.alerts = ct.alerts.alerts[100:] // 保留最近900条
	}
}

// GetUserUsage 获取用户使用量
func (ct *CostTracker) GetUserUsage(userID int64) *UserUsage {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	ct.resetUserDailyUsageIfNeeded(userID)
	
	if usage, exists := ct.userUsage[userID]; exists {
		return usage
	}
	
	return nil
}

// IsWithinBudget 检查是否在预算范围内
func (ct *CostTracker) IsWithinBudget(userID int64, estimatedCost float64) bool {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	return ct.checkBudgetLimits(userID, estimatedCost) == nil
}

// UpdateModelCost 更新模型成本配置
func (ct *CostTracker) UpdateModelCost(modelName string, cost *ModelCost) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	cost.LastUpdated = time.Now()
	ct.modelCosts[modelName] = cost
}

// GetModelCost 获取模型成本配置
func (ct *CostTracker) GetModelCost(modelName string) *ModelCost {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	if cost, exists := ct.modelCosts[modelName]; exists {
		return cost
	}
	
	return nil
}

// CostLimitError 成本限制错误
type CostLimitError struct {
	Type      string
	Current   float64
	Limit     float64
}

func (e *CostLimitError) Error() string {
	switch e.Type {
	case "single_query":
		return "单次查询成本超限 (single query cost limit exceeded)"
	case "user_daily":
		return "用户每日成本超限 (user daily cost limit exceeded)"
	case "daily_budget":
		return "每日总预算超限 (daily budget cost limit exceeded)"
	default:
		return "成本超限 (cost limit exceeded)"
	}
}

// NewCostLimitError 创建成本限制错误
func NewCostLimitError(errorType string, current, limit float64) *CostLimitError {
	return &CostLimitError{
		Type:    errorType,
		Current: current,
		Limit:   limit,
	}
}