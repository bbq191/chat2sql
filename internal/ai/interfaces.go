// Package ai 提供AI相关的核心接口定义
// 本文件定义了AI模块的所有公共接口，确保模块间的解耦和可测试性
package ai

import (
	"context"
	"time"
)

// =========================================================================
// 核心AI服务接口
// =========================================================================

// AIServiceInterface 定义AI服务的核心能力
// 这是AI模块对外提供服务的主要接口，封装了自然语言转SQL的完整功能链
type AIServiceInterface interface {
	// GenerateSQL 根据自然语言生成SQL查询
	// 这是AI服务的核心功能，支持多模型路由、成本优化和准确率监控
	// 
	// 参数:
	//   ctx: 请求上下文，支持超时控制和取消操作
	//   req: 包含用户查询、数据库元信息和配置选项的请求对象
	//
	// 返回:
	//   *SQLGenerationResponse: 包含生成的SQL、置信度、性能指标等信息
	//   error: 生成过程中的错误，包括但不限于：
	//     - 输入验证错误
	//     - LLM调用失败  
	//     - SQL验证错误
	//     - 超时错误
	GenerateSQL(ctx context.Context, req *SQLGenerationRequest) (*SQLGenerationResponse, error)
	
	// Close 优雅关闭AI服务并释放所有资源
	// 包括：关闭LLM连接、停止后台任务、清理缓存等
	Close() error
	
	// GetServiceInfo 获取服务信息和健康状态
	// 用于服务发现、健康检查和监控
	GetServiceInfo() *ServiceInfo
	
	// UpdateConfig 动态更新服务配置
	// 支持热更新，无需重启服务即可调整配置参数
	UpdateConfig(config *AIModuleConfig) error
}

// =========================================================================
// LLM客户端管理接口 
// =========================================================================

// LLMClientInterface 定义LLM客户端的标准接口
// 支持多种LLM提供商的统一抽象，包括OpenAI、Anthropic、Ollama等
type LLMClientInterface interface {
	// GenerateContent 生成内容的统一入口
	// 支持自动降级：主模型 -> 备用模型 -> 本地模型
	//
	// 参数:
	//   ctx: 上下文，包含超时和取消机制
	//   messages: 符合LangChain标准的消息列表
	//   options: 可选的调用参数（温度、最大token等）
	//
	// 返回:
	//   response: LLM生成的结果
	//   error: 调用错误，当所有模型都失败时返回
	GenerateContent(ctx context.Context, messages []any, options ...any) (any, error)
	
	// ValidateConfiguration 验证配置有效性
	// 通过发送测试请求验证所有配置的LLM提供商是否可用
	ValidateConfiguration(ctx context.Context) error
	
	// GetActiveProvider 获取当前活跃的提供商信息
	GetActiveProvider() (provider string, model string)
	
	// GetPerformanceMetrics 获取性能指标
	// 包括：成功率、平均响应时间、token使用量等
	GetPerformanceMetrics() *LLMPerformanceMetrics
}

// =========================================================================
// 查询处理器接口
// =========================================================================

// QueryProcessorInterface 定义查询处理的核心接口
// 负责自然语言到SQL的完整转换流程
type QueryProcessorInterface interface {
	// ProcessNaturalLanguageQuery 处理自然语言查询
	// 这是查询处理的主要入口，包含完整的处理管道：
	// 1. 意图分析 2. 上下文构建 3. 提示词生成 4. LLM调用 5. SQL验证
	//
	// 参数:
	//   ctx: 请求上下文
	//   req: 查询请求，包含用户输入、连接信息、选项配置
	//
	// 返回:
	//   *SQLResponse: 结构化的SQL响应，包含生成的SQL、置信度、元数据
	//   error: 处理过程中的错误
	ProcessNaturalLanguageQuery(ctx context.Context, req *ChatRequest) (*SQLResponse, error)
	
	// Reset 重置处理器状态
	// 用于对象池场景，清理临时状态以便重用
	Reset()
	
	// GetSupportedQueryTypes 获取支持的查询类型
	// 返回处理器能够处理的查询类型列表
	GetSupportedQueryTypes() []string
}

// =========================================================================
// 准确率监控接口
// =========================================================================

// AccuracyMonitorInterface 定义准确率监控的接口
// 提供SQL生成质量的实时监控和分析能力
type AccuracyMonitorInterface interface {
	// RecordFeedback 记录用户反馈
	// 用于收集SQL生成结果的准确性反馈，是准确率计算的数据来源
	//
	// 参数:
	//   feedback: 用户反馈信息，包含查询ID、正确性评价、用户评分等
	//
	// 返回:
	//   error: 记录失败的错误信息
	RecordFeedback(feedback QueryFeedback) error
	
	// GetCurrentAccuracy 获取当前准确率
	// 返回基于最近反馈计算的实时准确率
	GetCurrentAccuracy() float64
	
	// GetMetrics 获取详细监控指标
	// 返回包含准确率、查询统计、性能数据等的完整指标集
	GetMetrics() map[string]any
	
	// GenerateAccuracyReport 生成准确率分析报告
	// 参数:
	//   period: 统计周期（日、周、月）
	//   startTime, endTime: 时间范围
	//
	// 返回:
	//   *AccuracyReport: 详细的准确率分析报告
	//   error: 报告生成错误
	GenerateAccuracyReport(period string, startTime, endTime time.Time) (*AccuracyReport, error)
}

// =========================================================================
// 意图分析器接口
// =========================================================================

// IntentAnalyzerInterface 定义查询意图分析的接口
// 用于理解用户查询的意图，选择合适的处理策略
type IntentAnalyzerInterface interface {
	// AnalyzeIntent 分析查询意图
	// 基于自然语言处理技术识别用户查询的类型和意图
	//
	// 参数:
	//   query: 用户的自然语言查询
	//
	// 返回:
	//   QueryIntent: 识别出的查询意图枚举值
	AnalyzeIntent(query string) QueryIntent
	
	// AnalyzeIntentDetailed 详细意图分析
	// 提供更详细的意图分析结果，包含置信度、候选意图等
	//
	// 参数:
	//   query: 用户查询
	//   userID: 用户ID，用于个性化分析
	//
	// 返回:
	//   *IntentResult: 详细的意图分析结果
	AnalyzeIntentDetailed(query string, userID int64) *IntentResult
	
	// GetIntentName 获取意图的可读名称
	GetIntentName(intent QueryIntent) string
	
	// GetUserStats 获取用户的意图偏好统计
	GetUserStats(userID int64) *UserIntentProfile
}

// =========================================================================
// 性能优化器接口
// =========================================================================

// PerformanceOptimizerInterface 定义性能优化的接口
// 提供并发控制、熔断、限流、缓存等性能优化功能
type PerformanceOptimizerInterface interface {
	// ProcessRequest 处理请求（带性能优化）
	// 在请求处理过程中应用性能优化策略
	//
	// 参数:
	//   ctx: 请求上下文
	//   request: 通用请求接口
	//   processor: 实际的处理函数
	//
	// 返回:
	//   any: 处理结果
	//   error: 处理错误
	ProcessRequest(ctx context.Context, request any, processor func(context.Context, any) (any, error)) (any, error)
	
	// GetMetrics 获取性能指标
	// 返回工作池、熔断器、限流器等组件的性能数据
	GetMetrics() map[string]any
	
	// Shutdown 优雅关闭优化器
	// 等待正在处理的请求完成，然后关闭所有组件
	Shutdown(timeout time.Duration) error
}

// =========================================================================
// 数据类型定义
// =========================================================================

// ServiceInfo 服务信息结构
type ServiceInfo struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Status          string            `json:"status"` // healthy, unhealthy, degraded
	StartTime       time.Time         `json:"start_time"`
	Uptime          time.Duration     `json:"uptime"`
	ActiveProviders []string          `json:"active_providers"`
	Capabilities    []string          `json:"capabilities"`
	Metrics         map[string]any    `json:"metrics"`
}

// LLMPerformanceMetrics LLM性能指标
type LLMPerformanceMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	SuccessRate        float64       `json:"success_rate"`
	AverageLatency     time.Duration `json:"average_latency"`
	TokensUsed         int64         `json:"tokens_used"`
	CostIncurred       float64       `json:"cost_incurred"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// SQLGenerationRequest SQL生成请求
type SQLGenerationRequest struct {
	Query           string            `json:"query" binding:"required"`
	ConnectionID    int64             `json:"connection_id" binding:"required"`
	UserID          int64             `json:"user_id" binding:"required"`
	DatabaseSchema  string            `json:"database_schema,omitempty"`
	TableNames      []string          `json:"table_names,omitempty"`
	QueryContext    map[string]string `json:"query_context,omitempty"`
	Options         RequestOptions    `json:"options,omitempty"`
}

// SQLGenerationResponse SQL生成响应
type SQLGenerationResponse struct {
	SQL             string            `json:"sql"`
	Confidence      float64           `json:"confidence"`
	QueryType       string            `json:"query_type"`
	ProcessingTime  time.Duration     `json:"processing_time"`
	TokensUsed      int              `json:"tokens_used"`
	Cost            float64           `json:"cost,omitempty"`
	Explanation     string            `json:"explanation,omitempty"`
	Suggestions     []string          `json:"suggestions,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Error           string            `json:"error,omitempty"`
}