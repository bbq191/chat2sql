// AI服务提供基于LangChainGo的自然语言转SQL能力
package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	
	"chat2sql-go/internal/config"
)

// AIService AI服务基础架构
type AIService struct {
	// LLM客户端
	primaryClient  llms.Model
	fallbackClient llms.Model
	
	// 配置管理
	config *config.AIConfig
	
	// HTTP客户端优化
	httpClient *http.Client
	
	// 监控指标
	metrics *AIMetrics
	
	// 日志记录
	logger *zap.Logger
}

// AIMetrics AI服务监控指标
type AIMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	TokensUsed       *prometheus.CounterVec
	ErrorsTotal      *prometheus.CounterVec
}

// SQLGenerationRequest SQL生成请求
type SQLGenerationRequest struct {
	Query        string `json:"query"`
	ConnectionID int64  `json:"connection_id"`
	UserID       int64  `json:"user_id"`
	Schema       string `json:"schema,omitempty"`
}

// SQLGenerationResponse SQL生成响应
type SQLGenerationResponse struct {
	SQL            string        `json:"sql"`
	Confidence     float64       `json:"confidence"`
	ProcessingTime time.Duration `json:"processing_time"`
	Error          error         `json:"error,omitempty"`
}

// NewAIService 创建新的AI服务实例
func NewAIService(aiConfig *config.AIConfig, logger *zap.Logger) (*AIService, error) {
	// 创建优化的HTTP客户端
	httpClient := createOptimizedHTTPClient()
	
	// 初始化主要模型客户端
	primaryClient, err := createLLMClient(aiConfig.Primary, httpClient)
	if err != nil {
		return nil, fmt.Errorf("创建主要模型客户端失败: %w", err)
	}
	
	// 初始化备用模型客户端
	fallbackClient, err := createLLMClient(aiConfig.Fallback, httpClient)
	if err != nil {
		return nil, fmt.Errorf("创建备用模型客户端失败: %w", err)
	}
	
	// 初始化监控指标
	metrics := createMetrics()
	
	service := &AIService{
		primaryClient:  primaryClient,
		fallbackClient: fallbackClient,
		config:         aiConfig,
		httpClient:     httpClient,
		metrics:        metrics,
		logger:         logger,
	}
	
	logger.Info("AI服务初始化成功",
		zap.String("primary_provider", aiConfig.Primary.Provider),
		zap.String("primary_model", aiConfig.Primary.ModelName),
		zap.String("fallback_provider", aiConfig.Fallback.Provider),
		zap.String("fallback_model", aiConfig.Fallback.ModelName),
	)
	
	return service, nil
}

// createLLMClient 根据配置创建LLM客户端
func createLLMClient(modelConfig config.ModelConfig, httpClient *http.Client) (llms.Model, error) {
	switch modelConfig.Provider {
	case "openai":
		return openai.New(
			openai.WithToken(modelConfig.APIKey),
			openai.WithModel(modelConfig.ModelName),
			openai.WithHTTPClient(httpClient),
		)
	case "anthropic":
		return anthropic.New(
			anthropic.WithToken(modelConfig.APIKey),
			anthropic.WithModel(modelConfig.ModelName),
		)
	default:
		return nil, fmt.Errorf("不支持的模型提供商: %s", modelConfig.Provider)
	}
}

// createOptimizedHTTPClient 创建优化的HTTP客户端
func createOptimizedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			MaxIdleConnsPerHost: 10,               // 每个host最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时
			DisableCompression:  false,            // 启用压缩
			WriteBufferSize:     64 * 1024,        // 写缓冲区
			ReadBufferSize:      64 * 1024,        // 读缓冲区
			ForceAttemptHTTP2:   true,             // 强制HTTP/2
		},
		Timeout: 30 * time.Second,
	}
}

// createMetrics 创建监控指标
func createMetrics() *AIMetrics {
	return &AIMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_requests_total",
				Help: "Total AI service requests",
			},
			[]string{"provider", "model", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ai_request_duration_seconds",
				Help:    "AI request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
			[]string{"provider", "model"},
		),
		TokensUsed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_tokens_used_total",
				Help: "Total tokens used by AI services",
			},
			[]string{"provider", "model", "type"}, // type: input/output
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_errors_total",
				Help: "Total AI service errors",
			},
			[]string{"provider", "model", "error_type"},
		),
	}
}

// GenerateSQL 生成SQL语句
func (ai *AIService) GenerateSQL(ctx context.Context, req *SQLGenerationRequest) (*SQLGenerationResponse, error) {
	start := time.Now()
	
	// 记录请求指标
	defer func() {
		duration := time.Since(start)
		ai.metrics.RequestDuration.WithLabelValues(
			ai.config.Primary.Provider,
			ai.config.Primary.ModelName,
		).Observe(duration.Seconds())
	}()
	
	ai.logger.Info("开始生成SQL",
		zap.String("query", req.Query),
		zap.Int64("connection_id", req.ConnectionID),
		zap.Int64("user_id", req.UserID),
	)
	
	// 构建提示词
	prompt, err := ai.buildPrompt(req)
	if err != nil {
		ai.recordError("prompt_error", err)
		return nil, fmt.Errorf("构建提示词失败: %w", err)
	}
	
	// 调用LLM生成内容，带备用机制
	response, err := ai.callWithFallback(ctx, prompt)
	if err != nil {
		ai.recordError("llm_error", err)
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}
	
	// 解析响应
	sql, confidence := ai.parseResponse(response)
	duration := time.Since(start)
	
	// 记录成功指标
	ai.metrics.RequestsTotal.WithLabelValues(
		ai.config.Primary.Provider,
		ai.config.Primary.ModelName,
		"success",
	).Inc()
	
	// 记录Token使用量 (LangChainGo可能不提供详细的usage信息)
	// TODO: 根据实际LangChainGo API调整Token统计
	
	ai.logger.Info("SQL生成成功",
		zap.String("generated_sql", sql),
		zap.Float64("confidence", confidence),
		zap.Duration("duration", duration),
	)
	
	return &SQLGenerationResponse{
		SQL:            sql,
		Confidence:     confidence,
		ProcessingTime: duration,
	}, nil
}

// callWithFallback 调用LLM，带备用机制
func (ai *AIService) callWithFallback(ctx context.Context, prompt string) (*llms.ContentResponse, error) {
	// 首先尝试主要模型
	response, err := ai.primaryClient.GenerateContent(ctx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, prompt),
		},
		llms.WithTemperature(ai.config.Primary.Temperature),
		llms.WithMaxTokens(ai.config.Primary.MaxTokens),
	)
	
	if err == nil {
		ai.logger.Debug("主要模型调用成功", zap.String("provider", ai.config.Primary.Provider))
		return response, nil
	}
	
	// 记录主要模型失败
	ai.logger.Warn("主要模型调用失败，尝试备用模型",
		zap.Error(err),
		zap.String("primary_provider", ai.config.Primary.Provider),
	)
	
	ai.recordError("primary_failure", err)
	
	// 尝试备用模型
	response, err = ai.fallbackClient.GenerateContent(ctx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, prompt),
		},
		llms.WithTemperature(ai.config.Fallback.Temperature),
		llms.WithMaxTokens(ai.config.Fallback.MaxTokens),
	)
	
	if err != nil {
		ai.recordError("fallback_failure", err)
		return nil, fmt.Errorf("主要和备用模型都失败: %w", err)
	}
	
	ai.logger.Info("备用模型调用成功", zap.String("provider", ai.config.Fallback.Provider))
	return response, nil
}

// buildPrompt 构建SQL生成提示词
func (ai *AIService) buildPrompt(req *SQLGenerationRequest) (string, error) {
	// TODO: 实现更复杂的提示词模板
	prompt := fmt.Sprintf(`你是一个专业的SQL查询生成专家。根据用户的自然语言需求，生成准确的PostgreSQL查询语句。

## 数据库结构信息：
%s

## 用户查询：
%s

## 规则：
1. 只生成SELECT查询，禁止DELETE/UPDATE/INSERT/DROP操作
2. 使用PostgreSQL 17语法
3. 字段名必须与数据库结构完全匹配
4. 返回格式：纯SQL语句，不包含解释文字
5. 如果查询不明确，返回最合理的解释

## 生成SQL：`, req.Schema, req.Query)
	
	return prompt, nil
}

// parseResponse 解析LLM响应
func (ai *AIService) parseResponse(response *llms.ContentResponse) (string, float64) {
	if len(response.Choices) == 0 {
		return "", 0.0
	}
	
	sql := response.Choices[0].Content
	confidence := 0.8 // TODO: 实现置信度算法
	
	return sql, confidence
}

// recordError 记录错误指标
func (ai *AIService) recordError(errorType string, err error) {
	ai.metrics.ErrorsTotal.WithLabelValues(
		ai.config.Primary.Provider,
		ai.config.Primary.ModelName,
		errorType,
	).Inc()
	
	ai.logger.Error("AI服务错误",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}

// Close 关闭AI服务
func (ai *AIService) Close() error {
	ai.logger.Info("AI服务关闭")
	return nil
}