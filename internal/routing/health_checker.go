// 模型健康检查器 - P2阶段智能路由健康监控组件
// 提供实时模型可用性、延迟、限流状态监控
// 支持多种检查策略和智能告警机制

package routing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	// 检查配置
	config *HealthCheckConfig
	
	// 检查历史记录
	history map[string]*HealthHistory
	
	// 并发控制
	mu sync.RWMutex
	
	// 上下文
	ctx context.Context
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	// 检查间隔
	Interval time.Duration `yaml:"interval" json:"interval"`
	
	// 超时时间
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// 重试次数
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	
	// 失败阈值（连续失败多少次认为不健康）
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"`
	
	// 恢复阈值（连续成功多少次认为恢复健康）
	RecoveryThreshold int `yaml:"recovery_threshold" json:"recovery_threshold"`
	
	// 延迟阈值（毫秒）
	LatencyThreshold int64 `yaml:"latency_threshold_ms" json:"latency_threshold_ms"`
	
	// 成功率阈值
	SuccessRateThreshold float64 `yaml:"success_rate_threshold" json:"success_rate_threshold"`
	
	// 测试提示词
	TestPrompt string `yaml:"test_prompt" json:"test_prompt"`
}

// HealthHistory 健康检查历史
type HealthHistory struct {
	ModelName        string                 `json:"model_name"`
	CheckResults     []*HealthCheckResult   `json:"check_results"`
	ConsecutiveFailures int                `json:"consecutive_failures"`
	ConsecutiveSuccesses int               `json:"consecutive_successes"`
	LastHealthyTime  *time.Time            `json:"last_healthy_time,omitempty"`
	LastUnhealthyTime *time.Time           `json:"last_unhealthy_time,omitempty"`
	TotalChecks      int64                 `json:"total_checks"`
	TotalSuccesses   int64                 `json:"total_successes"`
	TotalFailures    int64                 `json:"total_failures"`
	AvgResponseTime  time.Duration         `json:"avg_response_time"`
}

// HealthCheckResult 单次健康检查结果
type HealthCheckResult struct {
	Timestamp    time.Time     `json:"timestamp"`
	Status       ModelStatus   `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(ctx context.Context) *HealthChecker {
	return &HealthChecker{
		config: DefaultHealthCheckConfig(),
		history: make(map[string]*HealthHistory),
		ctx: ctx,
	}
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		Interval:             30 * time.Second,
		Timeout:              15 * time.Second,
		MaxRetries:           3,
		FailureThreshold:     3,  // 连续失败3次认为不健康
		RecoveryThreshold:    2,  // 连续成功2次认为恢复
		LatencyThreshold:     5000, // 5秒延迟阈值
		SuccessRateThreshold: 0.8,  // 80%成功率阈值
		TestPrompt:          "SELECT 1", // 简单的测试查询
	}
}

// CheckModelHealth 检查单个模型的健康状态
func (hc *HealthChecker) CheckModelHealth(ctx context.Context, model *RegisteredModel) *ModelHealth {
	start := time.Now()
	
	// 获取或创建历史记录
	hc.mu.Lock()
	history, exists := hc.history[model.Name]
	if !exists {
		history = &HealthHistory{
			ModelName:    model.Name,
			CheckResults: make([]*HealthCheckResult, 0),
		}
		hc.history[model.Name] = history
	}
	hc.mu.Unlock()
	
	// 执行健康检查
	result := hc.performHealthCheck(ctx, model)
	
	// 更新历史记录
	hc.updateHealthHistory(history, result)
	
	// 计算整体健康状态
	health := hc.calculateOverallHealth(history, result)
	
	// 记录检查耗时
	health.ResponseTime = time.Since(start)
	
	return health
}

// performHealthCheck 执行实际的健康检查
func (hc *HealthChecker) performHealthCheck(ctx context.Context, model *RegisteredModel) *HealthCheckResult {
	checkCtx, cancel := context.WithTimeout(ctx, hc.config.Timeout)
	defer cancel()
	
	start := time.Now()
	result := &HealthCheckResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	// 尝试调用模型API
	err := hc.testModelAPI(checkCtx, model)
	result.ResponseTime = time.Since(start)
	
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Status = hc.determineErrorStatus(err, result.ResponseTime)
		result.Details["error_type"] = hc.categorizeError(err)
	} else {
		result.Success = true
		result.Status = ModelStatusHealthy
		result.Details["test_successful"] = true
	}
	
	// 检查延迟阈值
	if result.ResponseTime.Milliseconds() > hc.config.LatencyThreshold {
		if result.Status == ModelStatusHealthy {
			result.Status = ModelStatusDegraded
		}
		result.Details["latency_exceeded"] = true
	}
	
	return result
}

// testModelAPI 测试模型API调用
func (hc *HealthChecker) testModelAPI(ctx context.Context, model *RegisteredModel) error {
	// 构造测试消息
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant for testing database connections."),
		llms.TextParts(llms.ChatMessageTypeHuman, hc.config.TestPrompt),
	}
	
	// 调用模型API
	_, err := model.Model.GenerateContent(ctx, messages, 
		llms.WithMaxTokens(50), // 限制输出长度以降低成本
		llms.WithTemperature(0.1), // 低温度确保稳定输出
	)
	
	return err
}

// determineErrorStatus 根据错误确定状态
func (hc *HealthChecker) determineErrorStatus(err error, responseTime time.Duration) ModelStatus {
	errorStr := strings.ToLower(err.Error())
	
	// 超时错误
	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline exceeded") {
		return ModelStatusDegraded
	}
	
	// 认证错误
	if strings.Contains(errorStr, "unauthorized") || strings.Contains(errorStr, "invalid api key") {
		return ModelStatusUnhealthy
	}
	
	// 限流错误
	if strings.Contains(errorStr, "rate limit") || strings.Contains(errorStr, "too many requests") {
		return ModelStatusDegraded
	}
	
	// 服务不可用
	if strings.Contains(errorStr, "service unavailable") || strings.Contains(errorStr, "connection refused") {
		return ModelStatusUnhealthy
	}
	
	// 其他错误按延迟判断
	if responseTime.Milliseconds() > hc.config.LatencyThreshold*2 {
		return ModelStatusDegraded
	}
	
	return ModelStatusUnhealthy
}

// categorizeError 错误分类
func (hc *HealthChecker) categorizeError(err error) string {
	errorStr := strings.ToLower(err.Error())
	
	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline") {
		return "timeout"
	}
	if strings.Contains(errorStr, "unauthorized") || strings.Contains(errorStr, "api key") {
		return "authentication"
	}
	if strings.Contains(errorStr, "rate limit") || strings.Contains(errorStr, "too many requests") {
		return "rate_limit"
	}
	if strings.Contains(errorStr, "connection") {
		return "connection"
	}
	if strings.Contains(errorStr, "service unavailable") {
		return "service_unavailable"
	}
	
	return "unknown"
}

// updateHealthHistory 更新健康历史记录
func (hc *HealthChecker) updateHealthHistory(history *HealthHistory, result *HealthCheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	// 添加检查结果
	history.CheckResults = append(history.CheckResults, result)
	history.TotalChecks++
	
	// 保持历史记录长度限制（最近100条）
	if len(history.CheckResults) > 100 {
		history.CheckResults = history.CheckResults[1:]
	}
	
	// 更新统计
	if result.Success {
		history.TotalSuccesses++
		history.ConsecutiveSuccesses++
		history.ConsecutiveFailures = 0
		now := result.Timestamp
		history.LastHealthyTime = &now
	} else {
		history.TotalFailures++
		history.ConsecutiveFailures++
		history.ConsecutiveSuccesses = 0
		now := result.Timestamp
		history.LastUnhealthyTime = &now
	}
	
	// 更新平均响应时间
	hc.updateAvgResponseTime(history, result.ResponseTime)
}

// updateAvgResponseTime 更新平均响应时间
func (hc *HealthChecker) updateAvgResponseTime(history *HealthHistory, responseTime time.Duration) {
	if history.AvgResponseTime == 0 {
		history.AvgResponseTime = responseTime
	} else {
		// 指数移动平均
		alpha := 0.3
		history.AvgResponseTime = time.Duration(
			float64(history.AvgResponseTime)*(1-alpha) + 
			float64(responseTime)*alpha,
		)
	}
}

// calculateOverallHealth 计算整体健康状态
func (hc *HealthChecker) calculateOverallHealth(history *HealthHistory, latest *HealthCheckResult) *ModelHealth {
	health := &ModelHealth{
		Status:        latest.Status,
		LastCheck:     latest.Timestamp,
		ResponseTime:  latest.ResponseTime,
		ErrorCount:    history.TotalFailures,
		TotalRequests: history.TotalChecks,
		LastError:     latest.Error,
	}
	
	// 计算成功率
	if history.TotalChecks > 0 {
		health.SuccessRate = float64(history.TotalSuccesses) / float64(history.TotalChecks)
	}
	
	// 计算可用性（最近10次检查的成功率）
	recentResults := history.CheckResults
	if len(recentResults) > 10 {
		recentResults = recentResults[len(recentResults)-10:]
	}
	
	if len(recentResults) > 0 {
		recentSuccesses := 0
		for _, result := range recentResults {
			if result.Success {
				recentSuccesses++
			}
		}
		health.Availability = float64(recentSuccesses) / float64(len(recentResults))
	}
	
	// 根据连续失败/成功次数调整状态
	if history.ConsecutiveFailures >= hc.config.FailureThreshold {
		health.Status = ModelStatusUnhealthy
	} else if history.ConsecutiveSuccesses >= hc.config.RecoveryThreshold && 
			  health.SuccessRate >= hc.config.SuccessRateThreshold {
		if health.Status != ModelStatusHealthy {
			health.Status = ModelStatusHealthy
		}
	}
	
	// 延迟检查
	if history.AvgResponseTime.Milliseconds() > hc.config.LatencyThreshold {
		if health.Status == ModelStatusHealthy {
			health.Status = ModelStatusDegraded
		}
	}
	
	return health
}

// GetModelHealthHistory 获取模型健康历史
func (hc *HealthChecker) GetModelHealthHistory(modelName string) (*HealthHistory, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	history, exists := hc.history[modelName]
	if !exists {
		return nil, fmt.Errorf("模型 %s 没有健康检查历史", modelName)
	}
	
	// 返回副本避免并发问题
	historyCopy := *history
	historyCopy.CheckResults = make([]*HealthCheckResult, len(history.CheckResults))
	copy(historyCopy.CheckResults, history.CheckResults)
	
	return &historyCopy, nil
}

// GetAllHealthStatus 获取所有模型的健康状态摘要
func (hc *HealthChecker) GetAllHealthStatus() map[string]*ModelHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	result := make(map[string]*ModelHealth)
	
	for modelName, history := range hc.history {
		if len(history.CheckResults) > 0 {
			latest := history.CheckResults[len(history.CheckResults)-1]
			result[modelName] = hc.calculateOverallHealth(history, latest)
		}
	}
	
	return result
}

// GetHealthMetrics 获取健康检查指标
func (hc *HealthChecker) GetHealthMetrics() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	metrics := map[string]interface{}{
		"total_models":    len(hc.history),
		"healthy_count":   0,
		"degraded_count":  0,
		"unhealthy_count": 0,
		"avg_response_time": 0,
		"total_checks":    int64(0),
		"total_failures":  int64(0),
		"overall_success_rate": 0.0,
	}
	
	var totalResponseTime time.Duration
	var totalChecks, totalFailures int64
	
	for _, history := range hc.history {
		if len(history.CheckResults) > 0 {
			latest := history.CheckResults[len(history.CheckResults)-1]
			health := hc.calculateOverallHealth(history, latest)
			
			switch health.Status {
			case ModelStatusHealthy:
				metrics["healthy_count"] = metrics["healthy_count"].(int) + 1
			case ModelStatusDegraded:
				metrics["degraded_count"] = metrics["degraded_count"].(int) + 1
			case ModelStatusUnhealthy:
				metrics["unhealthy_count"] = metrics["unhealthy_count"].(int) + 1
			}
			
			totalResponseTime += history.AvgResponseTime
			totalChecks += history.TotalChecks
			totalFailures += history.TotalFailures
		}
	}
	
	// 计算平均值
	if len(hc.history) > 0 {
		metrics["avg_response_time"] = int64(totalResponseTime) / int64(len(hc.history))
	}
	
	metrics["total_checks"] = totalChecks
	metrics["total_failures"] = totalFailures
	
	if totalChecks > 0 {
		metrics["overall_success_rate"] = float64(totalChecks-totalFailures) / float64(totalChecks)
	}
	
	return metrics
}

// UpdateConfig 更新健康检查配置
func (hc *HealthChecker) UpdateConfig(config *HealthCheckConfig) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.config = config
}

// ClearHistory 清理指定模型的健康历史
func (hc *HealthChecker) ClearHistory(modelName string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	delete(hc.history, modelName)
}

// ClearAllHistory 清理所有健康历史
func (hc *HealthChecker) ClearAllHistory() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.history = make(map[string]*HealthHistory)
}