# 💰 AI监控与成本控制

<div align="center">

![Cost Control](https://img.shields.io/badge/Cost_Control-AI_Monitoring-blue.svg)
![Budget](https://img.shields.io/badge/Target_Cost-$0.01_per_query-green.svg)
![Monitoring](https://img.shields.io/badge/Real_time-Monitoring-orange.svg)

**Chat2SQL P1阶段 - AI成本监控与智能预算控制系统**

</div>

## 📋 概述

AI成本控制是Chat2SQL系统的核心竞争力之一。本文档提供全面的AI监控方案、成本控制策略和预算管理系统，确保AI服务在可控成本下提供高质量服务。

## 🎯 成本控制目标

### 核心指标

| 指标类别 | 目标值 | 监控频率 | 告警阈值 |
|---------|-------|---------|---------|
| **每查询成本** | < $0.01 | 实时 | > $0.015 |
| **日预算消耗率** | < 80% | 小时级 | > 85% |
| **Token使用效率** | > 85% | 实时 | < 80% |
| **成本节省率** | > 50% | 日级 | < 45% |

### 成本分层目标

```yaml
成本控制分层:
  用户级别:
    免费用户: "$0.50/day"
    标准用户: "$5.00/day"  
    企业用户: "$50.00/day"
    
  系统级别:
    日预算: "$1000/day"
    月预算: "$25000/month"
    紧急预算: "$200/day"
    
  模型级别:
    GPT-4o-mini: "$0.0015/1K tokens"
    Claude-3-haiku: "$0.00025/1K tokens"
    本地模型: "$0.00001/1K tokens"
```

## 🏗️ 监控架构设计

### 核心监控组件

```go
// internal/monitoring/ai_cost_monitor.go
type AICostMonitor struct {
    // 成本追踪器
    costTracker    *CostTracker
    budgetManager  *BudgetManager
    alertManager   *AlertManager
    
    // 指标收集
    metricsCollector *PrometheusCollector
    realTimeStream   chan *CostEvent
    
    // 存储和缓存
    costStore      *CostStorage
    cache          *RedisCache
    
    // 预测引擎
    costPredictor  *CostPredictor
}

type CostEvent struct {
    UserID       int64     `json:"user_id"`
    QueryID      string    `json:"query_id"`
    ModelName    string    `json:"model_name"`
    Provider     string    `json:"provider"`
    InputTokens  int       `json:"input_tokens"`
    OutputTokens int       `json:"output_tokens"`
    TotalTokens  int       `json:"total_tokens"`
    Cost         float64   `json:"cost_usd"`
    Duration     time.Duration `json:"duration"`
    Timestamp    time.Time `json:"timestamp"`
    Success      bool      `json:"success"`
}
```

### 实时成本追踪

```go
// internal/monitoring/cost_tracker.go
type CostTracker struct {
    pricingTable map[string]*ModelPricing
    userUsage    sync.Map  // userID -> *UserCostData
    dailyUsage   sync.Map  // date -> *DailyCostData
    realTimeWAL  *WriteAheadLog
}

type ModelPricing struct {
    Provider      string  `json:"provider"`
    ModelName     string  `json:"model_name"`
    InputPrice    float64 `json:"input_price_per_1k"`   // 每1K input tokens价格
    OutputPrice   float64 `json:"output_price_per_1k"`  // 每1K output tokens价格
    LastUpdated   time.Time `json:"last_updated"`
}

type UserCostData struct {
    UserID        int64     `json:"user_id"`
    DailyCost     float64   `json:"daily_cost"`
    MonthlyCost   float64   `json:"monthly_cost"`
    QueryCount    int       `json:"query_count"`
    TokensUsed    int       `json:"tokens_used"`
    LastQuery     time.Time `json:"last_query"`
    BudgetLimit   float64   `json:"budget_limit"`
    mu            sync.RWMutex
}

func (ct *CostTracker) RecordCost(event *CostEvent) error {
    // 1. 计算精确成本
    cost := ct.calculateCost(event)
    event.Cost = cost
    
    // 2. 更新用户使用量
    if err := ct.updateUserUsage(event); err != nil {
        return err
    }
    
    // 3. 更新系统统计
    ct.updateSystemUsage(event)
    
    // 4. 实时WAL记录
    if err := ct.realTimeWAL.Write(event); err != nil {
        log.Error("WAL写入失败", zap.Error(err))
    }
    
    // 5. 触发实时监控
    ct.triggerRealTimeMonitoring(event)
    
    return nil
}

func (ct *CostTracker) calculateCost(event *CostEvent) float64 {
    pricing, exists := ct.pricingTable[event.Provider+":"+event.ModelName]
    if !exists {
        log.Warn("模型价格未配置", zap.String("model", event.ModelName))
        return 0
    }
    
    inputCost := float64(event.InputTokens) / 1000.0 * pricing.InputPrice
    outputCost := float64(event.OutputTokens) / 1000.0 * pricing.OutputPrice
    
    return inputCost + outputCost
}
```

## 🛡️ 预算管理系统

### 多层预算控制

```go
// internal/monitoring/budget_manager.go
type BudgetManager struct {
    budgetStore   *BudgetStorage
    enforcer      *BudgetEnforcer
    alertManager  *AlertManager
    cache         *RedisCache
}

type BudgetConfig struct {
    // 用户级预算
    UserBudgets map[int64]*UserBudget `json:"user_budgets"`
    
    // 系统级预算
    SystemBudget *SystemBudget `json:"system_budget"`
    
    // 模型级预算
    ModelBudgets map[string]*ModelBudget `json:"model_budgets"`
    
    // 时间窗口配置
    WindowConfig *TimeWindowConfig `json:"window_config"`
}

type UserBudget struct {
    UserID       int64     `json:"user_id"`
    DailyLimit   float64   `json:"daily_limit"`
    MonthlyLimit float64   `json:"monthly_limit"`
    QueryLimit   int       `json:"query_limit"`
    TokenLimit   int       `json:"token_limit"`
    Priority     int       `json:"priority"`  // 1-10, 越高越优先
    AlertThreshold float64 `json:"alert_threshold"` // 0.8 = 80%时告警
}

type SystemBudget struct {
    DailyLimit    float64 `json:"daily_limit"`
    MonthlyLimit  float64 `json:"monthly_limit"`
    EmergencyLimit float64 `json:"emergency_limit"`
    AutoShutdown  bool    `json:"auto_shutdown"`
}

func (bm *BudgetManager) CheckBudget(
    userID int64, 
    estimatedCost float64) (*BudgetCheckResult, error) {
    
    result := &BudgetCheckResult{
        Allowed:     true,
        Reason:      "",
        Suggestions: make([]string, 0),
    }
    
    // 1. 检查用户预算
    userCheck, err := bm.checkUserBudget(userID, estimatedCost)
    if err != nil {
        return nil, err
    }
    
    if !userCheck.Allowed {
        result.Allowed = false
        result.Reason = userCheck.Reason
        result.Suggestions = userCheck.Suggestions
        return result, nil
    }
    
    // 2. 检查系统预算
    systemCheck, err := bm.checkSystemBudget(estimatedCost)
    if err != nil {
        return nil, err
    }
    
    if !systemCheck.Allowed {
        result.Allowed = false
        result.Reason = systemCheck.Reason
        result.Suggestions = append(result.Suggestions, "系统预算不足，建议稍后重试")
        return result, nil
    }
    
    // 3. 检查模型预算
    // ... 模型级预算检查逻辑
    
    return result, nil
}
```

### 动态预算调整

```go
// internal/monitoring/dynamic_budget.go
type DynamicBudgetAdjuster struct {
    costPredictor  *CostPredictor
    usageAnalyzer  *UsageAnalyzer
    alertManager   *AlertManager
}

func (dba *DynamicBudgetAdjuster) AdjustBudgets() error {
    // 1. 分析历史使用模式
    patterns, err := dba.usageAnalyzer.AnalyzeUsagePatterns()
    if err != nil {
        return err
    }
    
    // 2. 预测未来成本
    prediction, err := dba.costPredictor.PredictDailyCost()
    if err != nil {
        return err
    }
    
    // 3. 基于预测调整预算
    adjustments := dba.calculateBudgetAdjustments(patterns, prediction)
    
    // 4. 应用调整
    for userID, adjustment := range adjustments {
        if err := dba.applyUserBudgetAdjustment(userID, adjustment); err != nil {
            log.Error("预算调整失败", 
                zap.Int64("user_id", userID), 
                zap.Error(err))
        }
    }
    
    return nil
}

type BudgetAdjustment struct {
    NewDailyLimit   float64   `json:"new_daily_limit"`
    NewMonthlyLimit float64   `json:"new_monthly_limit"`
    Reason          string    `json:"reason"`
    EffectiveDate   time.Time `json:"effective_date"`
    AutoGenerated   bool      `json:"auto_generated"`
}
```

## 📊 实时监控仪表板

### Prometheus指标定义

```go
// internal/monitoring/metrics.go
var (
    // 成本相关指标
    aiCostTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_cost_total_usd",
            Help: "Total AI cost in USD",
        },
        []string{"user_id", "provider", "model"},
    )
    
    aiTokensUsed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_tokens_used_total",
            Help: "Total tokens used",
        },
        []string{"user_id", "provider", "model", "type"}, // type: input/output
    )
    
    aiQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ai_query_duration_seconds",
            Help: "AI query duration in seconds",
            Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
        },
        []string{"provider", "model"},
    )
    
    // 预算相关指标
    budgetUtilization = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "budget_utilization_ratio",
            Help: "Budget utilization ratio (0-1)",
        },
        []string{"user_id", "time_window"}, // time_window: daily/monthly
    )
    
    budgetExceeded = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "budget_exceeded_total",
            Help: "Total budget exceeded events",
        },
        []string{"user_id", "budget_type"},
    )
    
    // 效率相关指标
    costPerQuery = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ai_cost_per_query_usd",
            Help: "Cost per query in USD",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        },
        []string{"provider", "model"},
    )
)
```

### Grafana仪表板配置

```yaml
# monitoring/grafana/ai-cost-dashboard.yaml
dashboard:
  title: "AI成本监控仪表板"
  panels:
    - title: "实时成本消耗"
      type: "stat"
      targets:
        - expr: "sum(rate(ai_cost_total_usd[5m])) * 60"
          legendFormat: "每分钟成本"
      fieldConfig:
        unit: "dollars"
        color:
          mode: "thresholds"
          thresholds:
            - color: "green"
              value: 0
            - color: "yellow" 
              value: 0.5
            - color: "red"
              value: 1.0
    
    - title: "用户预算使用率"
      type: "heatmap"
      targets:
        - expr: "budget_utilization_ratio{time_window=\"daily\"}"
          legendFormat: "{{user_id}}"
      
    - title: "每查询成本趋势"
      type: "timeseries"
      targets:
        - expr: "histogram_quantile(0.95, rate(ai_cost_per_query_usd_bucket[5m]))"
          legendFormat: "P95成本/查询"
        - expr: "histogram_quantile(0.50, rate(ai_cost_per_query_usd_bucket[5m]))"
          legendFormat: "P50成本/查询"
    
    - title: "模型成本对比"
      type: "bargauge"
      targets:
        - expr: "sum by (model) (rate(ai_cost_total_usd[1h]))"
          legendFormat: "{{model}}"
```

## 🚨 智能告警系统

### 告警规则配置

```go
// internal/monitoring/alert_rules.go
type AlertRule struct {
    Name        string           `json:"name"`
    Condition   AlertCondition   `json:"condition"`
    Threshold   float64          `json:"threshold"`
    Duration    time.Duration    `json:"duration"`
    Severity    AlertSeverity    `json:"severity"`
    Actions     []AlertAction    `json:"actions"`
    Enabled     bool             `json:"enabled"`
}

type AlertCondition string
const (
    CostExceeded         AlertCondition = "cost_exceeded"
    BudgetUtilizationHigh AlertCondition = "budget_utilization_high"
    UnusualSpending      AlertCondition = "unusual_spending"
    ModelFailureRate     AlertCondition = "model_failure_rate"
    TokenEfficiencyLow   AlertCondition = "token_efficiency_low"
)

var defaultAlertRules = []AlertRule{
    {
        Name:      "用户日预算超限",
        Condition: CostExceeded,
        Threshold: 1.0, // 100%预算
        Duration:  time.Minute,
        Severity:  SeverityCritical,
        Actions:   []AlertAction{ActionBlockUser, ActionNotifyAdmin},
        Enabled:   true,
    },
    {
        Name:      "系统成本异常",
        Condition: UnusualSpending,
        Threshold: 2.0, // 2倍正常消耗
        Duration:  5 * time.Minute,
        Severity:  SeverityHigh,
        Actions:   []AlertAction{ActionThrottleRequests, ActionNotifyOps},
        Enabled:   true,
    },
    {
        Name:      "Token使用效率低",
        Condition: TokenEfficiencyLow,
        Threshold: 0.6, // 60%以下效率
        Duration:  10 * time.Minute,
        Severity:  SeverityMedium,
        Actions:   []AlertAction{ActionOptimizePrompts, ActionNotifyAI},
        Enabled:   true,
    },
}
```

### 告警触发和处理

```go
// internal/monitoring/alert_manager.go
type AlertManager struct {
    rules         []AlertRule
    evaluator     *AlertEvaluator
    actionHandler *AlertActionHandler
    notifier      *AlertNotifier
    
    // 告警状态管理
    activeAlerts  sync.Map // alertID -> *Alert
    alertHistory  *AlertHistory
}

func (am *AlertManager) EvaluateAlerts(metrics *SystemMetrics) {
    for _, rule := range am.rules {
        if !rule.Enabled {
            continue
        }
        
        triggered, value := am.evaluator.Evaluate(rule, metrics)
        if triggered {
            alert := &Alert{
                RuleName:  rule.Name,
                Condition: rule.Condition,
                Value:     value,
                Threshold: rule.Threshold,
                Severity:  rule.Severity,
                Timestamp: time.Now(),
                Status:    AlertStatusFiring,
            }
            
            // 检查是否重复告警
            if existing := am.getActiveAlert(alert.ID()); existing != nil {
                am.updateExistingAlert(existing, alert)
                continue
            }
            
            // 新告警
            am.handleNewAlert(alert, rule.Actions)
        }
    }
}

func (am *AlertManager) handleNewAlert(alert *Alert, actions []AlertAction) {
    // 1. 记录告警
    am.activeAlerts.Store(alert.ID(), alert)
    am.alertHistory.Record(alert)
    
    // 2. 执行告警动作
    for _, action := range actions {
        if err := am.actionHandler.Execute(action, alert); err != nil {
            log.Error("告警动作执行失败", 
                zap.String("action", string(action)),
                zap.Error(err))
        }
    }
    
    // 3. 发送通知
    am.notifier.Send(alert)
}
```

## 💡 成本优化策略

### 智能模型选择

```go
// internal/optimization/model_selector.go
type IntelligentModelSelector struct {
    costCalculator *CostCalculator
    qualityPredictor *QualityPredictor
    loadBalancer   *ModelLoadBalancer
}

func (ims *IntelligentModelSelector) SelectOptimalModel(
    request *QueryRequest,
    userBudget *UserBudget) (*ModelSelection, error) {
    
    // 1. 获取可用模型列表
    availableModels := ims.getAvailableModels()
    
    // 2. 评估每个模型的成本和质量
    evaluations := make([]*ModelEvaluation, 0, len(availableModels))
    
    for _, model := range availableModels {
        evaluation := &ModelEvaluation{
            Model: model,
        }
        
        // 估算成本
        evaluation.EstimatedCost = ims.costCalculator.EstimateCost(request, model)
        
        // 预测质量
        evaluation.PredictedQuality = ims.qualityPredictor.PredictQuality(request, model)
        
        // 计算性价比
        evaluation.ValueScore = evaluation.PredictedQuality / evaluation.EstimatedCost
        
        evaluations = append(evaluations, evaluation)
    }
    
    // 3. 选择最优模型
    optimal := ims.selectBestModel(evaluations, userBudget)
    
    return &ModelSelection{
        SelectedModel:   optimal.Model,
        EstimatedCost:   optimal.EstimatedCost,
        PredictedQuality: optimal.PredictedQuality,
        Reason:          ims.generateSelectionReason(optimal),
    }, nil
}

func (ims *IntelligentModelSelector) selectBestModel(
    evaluations []*ModelEvaluation,
    userBudget *UserBudget) *ModelEvaluation {
    
    // 排序：按性价比排序，成本限制内
    sort.Slice(evaluations, func(i, j int) bool {
        eval1, eval2 := evaluations[i], evaluations[j]
        
        // 超预算的模型排到后面
        if eval1.EstimatedCost > userBudget.DailyLimit && eval2.EstimatedCost <= userBudget.DailyLimit {
            return false
        }
        if eval1.EstimatedCost <= userBudget.DailyLimit && eval2.EstimatedCost > userBudget.DailyLimit {
            return true
        }
        
        // 都在预算内或都超预算，按性价比排序
        return eval1.ValueScore > eval2.ValueScore
    })
    
    return evaluations[0]
}
```

### 缓存优化策略

```go
// internal/optimization/cache_optimizer.go
type CacheOptimizer struct {
    semanticCache *SemanticCache
    costAnalyzer  *CostAnalyzer
    hitRateTracker *HitRateTracker
}

func (co *CacheOptimizer) OptimizeCacheStrategy() (*CacheOptimization, error) {
    // 1. 分析缓存命中率
    hitRates := co.hitRateTracker.GetHitRates()
    
    // 2. 计算缓存节省成本
    savings := co.costAnalyzer.CalculateCacheSavings()
    
    // 3. 识别高价值查询模式
    patterns := co.identifyHighValuePatterns()
    
    // 4. 优化缓存策略
    strategy := &CacheStrategy{
        TTL:              co.calculateOptimalTTL(patterns),
        SimilarityThreshold: co.calculateOptimalSimilarity(hitRates),
        MaxCacheSize:     co.calculateOptimalCacheSize(savings),
        EvictionPolicy:   co.selectOptimalEvictionPolicy(),
    }
    
    return &CacheOptimization{
        Strategy:        strategy,
        ExpectedSavings: savings.ProjectedSavings,
        Implementation:  co.generateImplementationPlan(strategy),
    }, nil
}
```

## 🔍 成本分析和预测

### 成本趋势分析

```go
// internal/analytics/cost_analyzer.go
type CostAnalyzer struct {
    dataStore     *AnalyticsStore
    mlPredictor   *MLCostPredictor
    reportGenerator *ReportGenerator
}

func (ca *CostAnalyzer) GenerateCostAnalysis(
    timeRange TimeRange) (*CostAnalysisReport, error) {
    
    report := &CostAnalysisReport{
        TimeRange: timeRange,
    }
    
    // 1. 基础统计
    report.BasicStats = ca.calculateBasicStats(timeRange)
    
    // 2. 趋势分析
    report.Trends = ca.analyzeTrends(timeRange)
    
    // 3. 用户分析
    report.UserAnalysis = ca.analyzeUserCosts(timeRange)
    
    // 4. 模型效率分析
    report.ModelEfficiency = ca.analyzeModelEfficiency(timeRange)
    
    // 5. 成本预测
    prediction, err := ca.mlPredictor.PredictCosts(timeRange.ExtendDays(30))
    if err != nil {
        log.Error("成本预测失败", zap.Error(err))
    } else {
        report.Prediction = prediction
    }
    
    // 6. 优化建议
    report.Recommendations = ca.generateOptimizationRecommendations(report)
    
    return report, nil
}

type CostAnalysisReport struct {
    TimeRange       TimeRange            `json:"time_range"`
    BasicStats      *BasicCostStats      `json:"basic_stats"`
    Trends          *CostTrends          `json:"trends"`
    UserAnalysis    *UserCostAnalysis    `json:"user_analysis"`
    ModelEfficiency *ModelEfficiencyAnalysis `json:"model_efficiency"`
    Prediction      *CostPrediction      `json:"prediction"`
    Recommendations []OptimizationRecommendation `json:"recommendations"`
}
```

### 机器学习成本预测

```go
// internal/analytics/ml_predictor.go
type MLCostPredictor struct {
    model      *tensorflow.Model
    features   *FeatureExtractor
    scaler     *StandardScaler
    validator  *ModelValidator
}

func (mlp *MLCostPredictor) PredictCosts(
    future TimeRange) (*CostPrediction, error) {
    
    // 1. 提取特征
    features, err := mlp.features.ExtractFeatures(future)
    if err != nil {
        return nil, err
    }
    
    // 2. 特征缩放
    scaledFeatures := mlp.scaler.Transform(features)
    
    // 3. 模型预测
    prediction, err := mlp.model.Predict(scaledFeatures)
    if err != nil {
        return nil, err
    }
    
    // 4. 后处理
    result := &CostPrediction{
        PredictedCost:     prediction.Cost,
        ConfidenceInterval: prediction.ConfidenceInterval,
        Methodology:       "深度学习时间序列预测",
        ModelAccuracy:     mlp.validator.GetCurrentAccuracy(),
        InfluencingFactors: mlp.identifyInfluencingFactors(features),
    }
    
    return result, nil
}

type FeatureExtractor struct {
    historicalWindow time.Duration
    seasonalFactors  []SeasonalFactor
}

func (fe *FeatureExtractor) ExtractFeatures(timeRange TimeRange) (*Features, error) {
    features := &Features{}
    
    // 1. 时间特征
    features.TimeFeatures = fe.extractTimeFeatures(timeRange)
    
    // 2. 历史使用模式
    features.UsagePatterns = fe.extractUsagePatterns(timeRange)
    
    // 3. 季节性因子
    features.SeasonalFactors = fe.extractSeasonalFactors(timeRange)
    
    // 4. 外部因子
    features.ExternalFactors = fe.extractExternalFactors(timeRange)
    
    return features, nil
}
```

## 📋 成本报告系统

### 自动化报告生成

```go
// internal/reporting/cost_reporter.go
type CostReporter struct {
    dataSource    *CostDataSource
    templateEngine *ReportTemplateEngine
    distributor   *ReportDistributor
    scheduler     *ReportScheduler
}

func (cr *CostReporter) GenerateReport(
    reportType ReportType,
    config *ReportConfig) (*CostReport, error) {
    
    switch reportType {
    case ReportTypeDaily:
        return cr.generateDailyReport(config)
    case ReportTypeWeekly:
        return cr.generateWeeklyReport(config)
    case ReportTypeMonthly:
        return cr.generateMonthlyReport(config)
    case ReportTypeCustom:
        return cr.generateCustomReport(config)
    default:
        return nil, fmt.Errorf("不支持的报告类型: %v", reportType)
    }
}

func (cr *CostReporter) generateDailyReport(config *ReportConfig) (*CostReport, error) {
    report := &CostReport{
        Type:      ReportTypeDaily,
        Date:      config.Date,
        Recipient: config.Recipient,
    }
    
    // 1. 收集数据
    data, err := cr.dataSource.GetDailyData(config.Date)
    if err != nil {
        return nil, err
    }
    
    // 2. 生成图表
    charts, err := cr.generateCharts(data)
    if err != nil {
        return nil, err
    }
    
    // 3. 生成摘要
    summary := cr.generateSummary(data)
    
    // 4. 生成建议
    recommendations := cr.generateRecommendations(data)
    
    report.Content = &ReportContent{
        Summary:         summary,
        Charts:          charts,
        DetailedData:    data,
        Recommendations: recommendations,
    }
    
    return report, nil
}
```

### 报告内容模板

```yaml
# config/report_templates/daily_cost_report.yaml
template:
  name: "AI成本日报"
  sections:
    - name: "执行摘要"
      content:
        - "今日总成本: ${{.TotalCost}}"
        - "与昨日对比: {{.DayOverDayChange}}%"
        - "预算使用率: {{.BudgetUtilization}}%"
        - "查询总数: {{.TotalQueries}}"
        
    - name: "成本分解"
      content:
        - "按模型分解:"
          - "GPT-4o-mini: ${{.CostByModel.GPT4oMini}} ({{.PercentByModel.GPT4oMini}}%)"
          - "Claude-3-haiku: ${{.CostByModel.Claude3Haiku}} ({{.PercentByModel.Claude3Haiku}}%)"
        - "按用户分解:"
          - "前5位用户消耗: ${{.TopUsersCost}}"
          
    - name: "性能指标"
      content:
        - "平均每查询成本: ${{.AvgCostPerQuery}}"
        - "Token使用效率: {{.TokenEfficiency}}%"
        - "缓存命中率: {{.CacheHitRate}}%"
        
    - name: "异常和告警"
      content:
        - "预算超限用户: {{.BudgetExceededUsers}}"
        - "异常消耗模式: {{.AnomalousPatterns}}"
        
    - name: "优化建议"
      content:
        - "{{range .Recommendations}}- {{.}}{{end}}"
```

## 🎯 最佳实践

### ✅ 成本控制最佳实践

1. **实时监控**
   - 设置实时成本追踪
   - 配置多层级告警
   - 实施自动限流机制

2. **预算管理**
   - 分层设置预算限制
   - 动态调整预算配置
   - 建立紧急预算机制

3. **成本优化**
   - 智能模型选择策略
   - 积极利用缓存
   - 优化提示词效率

4. **透明度**
   - 定期生成成本报告
   - 提供用户成本透明度
   - 建立成本问责制度

### ⚠️ 常见陷阱

1. **监控盲区**
   - 忽略小额成本累积
   - 缺乏模型级监控
   - 预算设置不合理

2. **优化误区**
   - 过度优化影响质量
   - 忽略用户体验
   - 缓存策略不当

3. **告警疲劳**
   - 告警阈值设置过低
   - 缺乏告警优先级
   - 响应机制不完善

### 🎯 目标达成策略

**短期目标 (1-2周)**:
- 建立基础监控框架
- 实施用户级预算控制
- 配置关键告警规则

**中期目标 (1个月)**:
- 部署智能模型选择
- 优化缓存策略
- 完善报告系统

**长期目标 (3个月)**:
- 实现自适应预算管理
- 部署ML成本预测
- 建立成本效率优化循环

---

<div align="center">

**💰 智能成本控制：监控实时化 + 预算精细化 + 优化自动化**

</div>