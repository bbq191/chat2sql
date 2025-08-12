// Package ai 查询处理器核心组件
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
)

// QueryProcessor AI查询处理器
type QueryProcessor struct {
	// LLM客户端配置
	primaryClient   llms.Model
	fallbackClient  llms.Model
	
	// 核心组件
	contextManager  *ContextManager
	templateManager *PromptTemplateManager
	sqlValidator    *SQLValidator
	intentAnalyzer  *IntentAnalyzer
	costTracker     *CostTracker
	
	// 配置参数
	config *ProcessorConfig
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	// 主要模型配置
	PrimaryModel ModelConfig `yaml:"primary_model"`
	
	// 备用模型配置
	FallbackModel ModelConfig `yaml:"fallback_model"`
	
	// 请求配置
	MaxRetries      int           `yaml:"max_retries"`
	RequestTimeout  time.Duration `yaml:"request_timeout"`
	MaxTokens       int           `yaml:"max_tokens"`
	Temperature     float64       `yaml:"temperature"`
	
	// 成本控制
	EnableCostTracking bool    `yaml:"enable_cost_tracking"`
	MaxCostPerQuery    float64 `yaml:"max_cost_per_query"`
	
	// 安全配置
	EnableSQLValidation bool `yaml:"enable_sql_validation"`
	EnableIntentAnalysis bool `yaml:"enable_intent_analysis"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Provider    string        `yaml:"provider"`     // "openai" 或 "anthropic"
	Model       string        `yaml:"model"`        // 模型名称
	APIKey      string        `yaml:"api_key"`      // API密钥
	Temperature float64       `yaml:"temperature"`  // 温度参数
	MaxTokens   int           `yaml:"max_tokens"`   // 最大令牌数
	TopP        float64       `yaml:"top_p"`        // TopP参数
	Timeout     time.Duration `yaml:"timeout"`      // 请求超时
}

// ChatRequest 聊天请求
type ChatRequest struct {
	// 基础信息
	Query        string `json:"query" binding:"required"`         // 用户查询
	ConnectionID int64  `json:"connection_id" binding:"required"` // 连接ID
	UserID       int64  `json:"user_id" binding:"required"`       // 用户ID
	
	// 可选参数
	QueryType    string            `json:"query_type,omitempty"`    // 查询类型提示
	Context      map[string]string `json:"context,omitempty"`       // 额外上下文
	Options      RequestOptions    `json:"options,omitempty"`       // 请求选项
}

// RequestOptions 请求选项
type RequestOptions struct {
	EnableStreaming   bool    `json:"enable_streaming,omitempty"`   // 启用流式响应
	MaxExecutionTime  int     `json:"max_execution_time,omitempty"` // 最大执行时间（秒）
	Confidence        float64 `json:"confidence,omitempty"`         // 最低置信度阈值
	IncludeExplanation bool   `json:"include_explanation,omitempty"` // 包含解释
}

// SQLResponse SQL响应
type SQLResponse struct {
	// 核心结果
	SQL         string  `json:"sql"`          // 生成的SQL
	Confidence  float64 `json:"confidence"`   // 置信度（0-1）
	QueryType   string  `json:"query_type"`   // 检测到的查询类型
	
	// 执行信息
	ProcessingTime time.Duration `json:"processing_time"` // 处理时间
	TokensUsed     int          `json:"tokens_used"`     // 使用的令牌数
	
	// 可选信息
	Explanation    string            `json:"explanation,omitempty"`    // SQL解释
	Suggestions    []string          `json:"suggestions,omitempty"`    // 优化建议
	Warnings       []string          `json:"warnings,omitempty"`       // 警告信息
	Metadata       map[string]string `json:"metadata,omitempty"`       // 元数据
	
	// 错误处理
	Error string `json:"error,omitempty"` // 错误信息
}

// LLMResponse LLM结构化响应
type LLMResponse struct {
	SQL         string            `json:"sql"`
	Confidence  float64           `json:"confidence"`
	QueryType   string            `json:"query_type"`
	Explanation string            `json:"explanation"`
	TableNames  []string          `json:"table_names"`
	Warnings    []string          `json:"warnings"`
	Metadata    map[string]string `json:"metadata"`
}

// NewQueryProcessor 创建新的查询处理器
func NewQueryProcessor(config *ProcessorConfig, contextManager *ContextManager) (*QueryProcessor, error) {
	if config == nil {
		return nil, fmt.Errorf("配置参数不能为空")
	}
	
	if contextManager == nil {
		return nil, fmt.Errorf("上下文管理器不能为空")
	}
	
	// 初始化主要模型客户端
	primaryClient, err := createLLMClient(config.PrimaryModel)
	if err != nil {
		return nil, fmt.Errorf("创建主要模型客户端失败: %w", err)
	}
	
	// 初始化备用模型客户端
	fallbackClient, err := createLLMClient(config.FallbackModel)
	if err != nil {
		return nil, fmt.Errorf("创建备用模型客户端失败: %w", err)
	}
	
	qp := &QueryProcessor{
		primaryClient:   primaryClient,
		fallbackClient:  fallbackClient,
		contextManager:  contextManager,
		templateManager: NewPromptTemplateManager(),
		config:          config,
	}
	
	// 初始化可选组件
	if config.EnableSQLValidation {
		qp.sqlValidator = NewSQLValidator()
	}
	
	if config.EnableIntentAnalysis {
		qp.intentAnalyzer = NewIntentAnalyzer()
	}
	
	if config.EnableCostTracking {
		qp.costTracker = NewCostTracker(&CostConfig{
			QueryCostLimit: config.MaxCostPerQuery,
		})
	}
	
	return qp, nil
}

// ProcessNaturalLanguageQuery 处理自然语言查询
func (qp *QueryProcessor) ProcessNaturalLanguageQuery(ctx context.Context, req *ChatRequest) (*SQLResponse, error) {
	start := time.Now()
	
	// 0. 检查必要的组件是否初始化
	if qp.primaryClient == nil {
		return nil, fmt.Errorf("LLM客户端未配置")
	}
	if qp.contextManager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}
	if qp.templateManager == nil {
		return nil, fmt.Errorf("提示词模板管理器未初始化")
	}
	
	// 1. 验证请求参数
	if err := qp.validateRequest(req); err != nil {
		return &SQLResponse{Error: fmt.Sprintf("请求参数验证失败: %v", err)}, nil
	}
	
	// 2. 意图分析（可选）
	var queryType string = "base" // 默认类型
	if qp.intentAnalyzer != nil && qp.config.EnableIntentAnalysis {
		intent := qp.intentAnalyzer.AnalyzeIntent(req.Query)
		queryType = qp.mapIntentToTemplateType(intent)
	}
	
	// 3. 构建查询上下文
	queryContext, err := qp.contextManager.BuildQueryContext(req.ConnectionID, req.UserID, req.Query)
	if err != nil {
		return &SQLResponse{Error: fmt.Sprintf("构建查询上下文失败: %v", err)}, nil
	}
	
	// 4. 选择并格式化提示词模板
	template, err := qp.templateManager.GetTemplate(queryType)
	if err != nil {
		// 降级到基础模板
		template, err = qp.templateManager.GetTemplate("base")
		if err != nil {
			return &SQLResponse{Error: fmt.Sprintf("获取提示词模板失败: %v", err)}, nil
		}
	}
	
	// 5. 生成提示词
	prompt, err := template.FormatPrompt(queryContext)
	if err != nil {
		return &SQLResponse{Error: fmt.Sprintf("格式化提示词失败: %v", err)}, nil
	}
	
	// 6. 调用LLM生成SQL（带降级机制）
	llmResponse, tokensUsed, err := qp.generateSQLWithFallback(ctx, prompt)
	if err != nil {
		return &SQLResponse{Error: fmt.Sprintf("LLM调用失败: %v", err)}, nil
	}
	
	// 7. SQL验证（可选）
	if qp.sqlValidator != nil && qp.config.EnableSQLValidation {
		if err := qp.sqlValidator.Validate(llmResponse.SQL); err != nil {
			return &SQLResponse{
				Error: fmt.Sprintf("生成的SQL验证失败: %v", err),
				SQL:   llmResponse.SQL, // 仍返回SQL供调试
			}, nil
		}
	}
	
	// 8. 记录成本信息（可选）
	processingTime := time.Since(start)
	if qp.costTracker != nil && qp.config.EnableCostTracking {
		// 假设输入和输出token各占一半（实际使用时需要从LLM响应获取准确数据）
		inputTokens := tokensUsed / 2
		outputTokens := tokensUsed - inputTokens
		modelName := qp.config.PrimaryModel.Model // 使用配置的模型名称
		
		cost := qp.costTracker.CalculateQueryCost(inputTokens, outputTokens, modelName)
		queryCost := QueryCost{
			Timestamp:    time.Now(),
			Query:        req.Query,
			ModelName:    modelName,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  tokensUsed,
			Cost:         cost,
			ProcessTime:  processingTime.Milliseconds(),
		}
		
		if err := qp.costTracker.RecordQueryCost(req.UserID, queryCost); err != nil {
			// 成本记录失败不应影响主流程，只记录警告
			llmResponse.Warnings = append(llmResponse.Warnings, fmt.Sprintf("成本记录失败: %v", err))
		}
	}
	
	// 9. 添加查询历史记录
	qp.contextManager.AddQueryHistory(req.UserID, req.Query, llmResponse.SQL, true)
	
	// 10. 构建响应
	response := &SQLResponse{
		SQL:            llmResponse.SQL,
		Confidence:     llmResponse.Confidence,
		QueryType:      llmResponse.QueryType,
		ProcessingTime: processingTime,
		TokensUsed:     tokensUsed,
		Explanation:    llmResponse.Explanation,
		Warnings:       llmResponse.Warnings,
		Metadata:       llmResponse.Metadata,
	}
	
	// 添加表名到元数据
	if response.Metadata == nil {
		response.Metadata = make(map[string]string)
	}
	response.Metadata["table_names"] = strings.Join(llmResponse.TableNames, ",")
	response.Metadata["processing_model"] = "primary" // 或者记录实际使用的模型
	
	return response, nil
}

// generateSQLWithFallback 带降级机制的SQL生成
func (qp *QueryProcessor) generateSQLWithFallback(ctx context.Context, prompt string) (*LLMResponse, int, error) {
	// 设置请求超时
	reqCtx, cancel := context.WithTimeout(ctx, qp.config.RequestTimeout)
	defer cancel()
	
	// 首先尝试主要模型
	response, tokensUsed, err := qp.callLLMStructured(reqCtx, qp.primaryClient, prompt, qp.config.PrimaryModel)
	if err == nil {
		return response, tokensUsed, nil
	}
	
	// 检查错误类型，决定是否重试
	if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "exceeded") {
		return nil, 0, fmt.Errorf("主要模型配额已用完: %w", err)
	}
	
	// 如果是速率限制或其他临时错误，尝试备用模型
	if strings.Contains(err.Error(), "rate limit") || isRetryableError(err) {
		response, tokensUsed, fallbackErr := qp.callLLMStructured(reqCtx, qp.fallbackClient, prompt, qp.config.FallbackModel)
		if fallbackErr == nil {
			// 在响应中标记使用了备用模型
			if response.Metadata == nil {
				response.Metadata = make(map[string]string)
			}
			response.Metadata["fallback_used"] = "true"
			response.Metadata["primary_error"] = err.Error()
			return response, tokensUsed, nil
		}
		
		// 两个模型都失败了
		return nil, 0, fmt.Errorf("主要模型和备用模型都失败: primary=%v, fallback=%v", err, fallbackErr)
	}
	
	// 其他类型的错误直接返回
	return nil, 0, err
}

// callLLMStructured 调用LLM并解析结构化响应
func (qp *QueryProcessor) callLLMStructured(ctx context.Context, client llms.Model, prompt string, modelConfig ModelConfig) (*LLMResponse, int, error) {
	// 构建结构化提示词模板
	structuredPrompt := qp.buildStructuredPrompt(prompt)
	
	// 调用LLM
	response, err := client.GenerateContent(ctx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, structuredPrompt),
		},
		llms.WithTemperature(modelConfig.Temperature),
		llms.WithMaxTokens(modelConfig.MaxTokens),
		llms.WithTopP(modelConfig.TopP),
		llms.WithJSONMode(), // 请求JSON格式响应
	)
	
	if err != nil {
		return nil, 0, fmt.Errorf("LLM调用失败: %w", err)
	}
	
	if len(response.Choices) == 0 {
		// 没有Usage字段，返回0
		return nil, 0, fmt.Errorf("LLM返回空响应")
	}
	
	// 解析JSON响应
	var llmResponse LLMResponse
	content := response.Choices[0].Content
	if err := json.Unmarshal([]byte(content), &llmResponse); err != nil {
		// JSON解析失败，尝试从文本中提取SQL
		sql := qp.extractSQLFromText(content)
		if sql == "" {
			// 估算token数量，实际应该从response中获取
			return nil, len(content)/4, fmt.Errorf("无法解析LLM响应: %w", err)
		}
		
		// 构建基础响应
		llmResponse = LLMResponse{
			SQL:        sql,
			Confidence: 0.7, // 默认置信度
			QueryType:  "unknown",
			Warnings:   []string{"JSON解析失败，使用文本提取"},
		}
	}
	
	// 验证响应的完整性
	if llmResponse.SQL == "" {
		// 估算token数量
		return nil, len(content)/4, fmt.Errorf("LLM响应中缺少SQL语句")
	}
	
	// 设置默认值
	if llmResponse.QueryType == "" {
		llmResponse.QueryType = "base"
	}
	
	if llmResponse.Confidence == 0 {
		llmResponse.Confidence = 0.8
	}
	
	// 估算token使用量
	estimatedTokens := len(content)/4
	if estimatedTokens < 10 {
		estimatedTokens = 10 // 最小token数
	}
	
	return &llmResponse, estimatedTokens, nil
}

// buildStructuredPrompt 构建结构化提示词
func (qp *QueryProcessor) buildStructuredPrompt(basePrompt string) string {
	template := prompts.NewPromptTemplate(`
{{.base_prompt}}

请按照以下JSON格式返回结果：
{
  "sql": "生成的SQL查询语句",
  "confidence": 0.95,
  "query_type": "base|aggregation|join|timeseries",
  "explanation": "SQL查询的简要解释",
  "table_names": ["使用的表名列表"],
  "warnings": ["任何警告或注意事项"],
  "metadata": {
    "complexity": "simple|medium|complex",
    "estimated_rows": "预估结果行数"
  }
}

确保返回有效的JSON格式，不要包含其他文本。`,
		[]string{"base_prompt"})
	
	formatted, err := template.Format(map[string]any{
		"base_prompt": basePrompt,
	})
	
	if err != nil {
		// 降级到简单格式
		return fmt.Sprintf("%s\n\n请返回JSON格式的结果，包含sql、confidence等字段。", basePrompt)
	}
	
	return formatted
}

// extractSQLFromText 从文本中提取SQL语句
func (qp *QueryProcessor) extractSQLFromText(text string) string {
	// 检查是否是JSON格式，如果是则跳过处理
	trimmedText := strings.TrimSpace(text)
	if strings.HasPrefix(trimmedText, "{") && strings.HasSuffix(trimmedText, "}") {
		// JSON格式的响应不处理
		return ""
	}
	
	// 简单的SQL提取逻辑
	lines := strings.Split(text, "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		
		// 检查是否是SQL语句
		if strings.HasPrefix(upper, "SELECT") ||
			strings.HasPrefix(upper, "WITH") {
			// 移除可能的引号或代码块标记
			sql := strings.Trim(trimmed, "`\"'")
			if len(sql) > 10 { // 基本长度检查
				return sql
			}
		}
	}
	
	// 如果没找到明显的SQL，返回第一个非空行
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "}") {
			return trimmed
		}
	}
	
	return ""
}

// createLLMClient 创建LLM客户端
func createLLMClient(config ModelConfig) (llms.Model, error) {
	switch config.Provider {
	case "openai":
		return openai.New(
			openai.WithToken(config.APIKey),
			openai.WithModel(config.Model),
		)
	case "anthropic":
		return anthropic.New(
			anthropic.WithToken(config.APIKey),
			anthropic.WithModel(config.Model),
		)
	default:
		return nil, fmt.Errorf("不支持的模型提供商: %s", config.Provider)
	}
}

// 辅助函数

// validateRequest 验证请求参数
func (qp *QueryProcessor) validateRequest(req *ChatRequest) error {
	if req.Query == "" {
		return fmt.Errorf("查询内容不能为空")
	}
	
	if req.ConnectionID == 0 {
		return fmt.Errorf("连接ID不能为0")
	}
	
	if req.UserID == 0 {
		return fmt.Errorf("用户ID不能为0")
	}
	
	if len(req.Query) > 1000 {
		return fmt.Errorf("查询内容过长，最多支持1000字符")
	}
	
	return nil
}

// mapIntentToTemplateType 将意图映射到模板类型
func (qp *QueryProcessor) mapIntentToTemplateType(intent QueryIntent) string {
	switch intent {
	case IntentAggregation:
		return "aggregation"
	case IntentJoinQuery:
		return "join"
	case IntentTimeSeriesAnalysis:
		return "timeseries"
	default:
		return "base"
	}
}

// isRetryableError 判断是否为可重试错误
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"timeout", "connection", "network", "temporary", "unavailable",
	}
	
	for _, keyword := range retryableErrors {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	
	return false
}

// Reset 重置处理器状态，用于对象池重用
func (qp *QueryProcessor) Reset() {
	// 重置处理器的状态，为下次使用做准备
	// 注意：不重置核心组件（contextManager等），只重置临时状态
	
	// 如果有临时缓存或状态，在这里清理
	// 当前实现中，QueryProcessor是无状态的，所以暂时不需要特殊处理
	
	// 可以在这里添加日志或指标记录
	// qp.logger.Debug("QueryProcessor reset for reuse")
}