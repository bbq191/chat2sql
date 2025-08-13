// 模型注册中心 - P2阶段智能路由核心组件
// 支持OpenAI、Claude、DeepSeek、Ollama等多种模型的统一注册和管理
// 基于LangChainGo v0.1.15实现，提供模型发现、注册、健康检查等功能

package routing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/llms/ollama"
)

// ModelRegistry 模型注册中心 - 管理所有可用的AI模型
type ModelRegistry struct {
	// 注册的模型实例
	models map[string]*RegisteredModel
	
	// 模型配置
	configs map[string]*ModelConfig
	
	// 健康检查器
	healthChecker *HealthChecker
	
	// 配置管理器
	configManager *ConfigManager
	
	// 并发控制
	mu sync.RWMutex
	
	// 生命周期控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RegisteredModel 已注册的模型信息
type RegisteredModel struct {
	Name         string                 `json:"name"`
	Provider     string                 `json:"provider"`
	Category     ComplexityCategory     `json:"category"`
	Model        llms.Model            `json:"-"` // LangChainGo模型实例
	Config       *ModelConfig          `json:"config"`
	Health       *ModelHealth          `json:"health"`
	CreatedAt    time.Time             `json:"created_at"`
	LastUsed     time.Time             `json:"last_used"`
	Status       ModelStatus           `json:"status"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Name            string        `yaml:"name" json:"name"`
	Provider        string        `yaml:"provider" json:"provider"`
	Category        string        `yaml:"category" json:"category"`
	Endpoint        string        `yaml:"endpoint" json:"endpoint"`
	APIKey          string        `yaml:"api_key" json:"api_key"`
	
	// 性能参数
	CostPer1K       float64       `yaml:"cost_per_1k" json:"cost_per_1k"`
	MaxTokens       int           `yaml:"max_tokens" json:"max_tokens"`
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`
	QPS             int           `yaml:"qps" json:"qps"`
	
	// 质量参数
	Accuracy        float64       `yaml:"accuracy" json:"accuracy"`
	AvgLatency      time.Duration `yaml:"avg_latency" json:"avg_latency"`
	Reliability     float64       `yaml:"reliability" json:"reliability"`
	
	// 状态管理
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	Priority        int           `yaml:"priority" json:"priority"`
	
	// 高级配置
	Temperature     float64       `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	TopP           float64       `yaml:"top_p,omitempty" json:"top_p,omitempty"`
	MaxRetries     int           `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
}

// ModelHealth 模型健康状态
type ModelHealth struct {
	Status          ModelStatus   `json:"status"`
	LastCheck       time.Time     `json:"last_check"`
	ResponseTime    time.Duration `json:"response_time"`
	SuccessRate     float64       `json:"success_rate"`
	ErrorCount      int64         `json:"error_count"`
	TotalRequests   int64         `json:"total_requests"`
	LastError       string        `json:"last_error,omitempty"`
	Availability    float64       `json:"availability"`
}

// ModelStatus 模型状态
type ModelStatus string

const (
	ModelStatusHealthy   ModelStatus = "healthy"
	ModelStatusUnhealthy ModelStatus = "unhealthy"
	ModelStatusDegraded  ModelStatus = "degraded"
	ModelStatusUnknown   ModelStatus = "unknown"
)

// ComplexityCategory 复杂度类别
type ComplexityCategory string

const (
	CategorySimple  ComplexityCategory = "simple"
	CategoryMedium  ComplexityCategory = "medium"
	CategoryComplex ComplexityCategory = "complex"
)

// NewModelRegistry 创建模型注册中心
func NewModelRegistry(ctx context.Context) *ModelRegistry {
	registryCtx, cancel := context.WithCancel(ctx)
	
	registry := &ModelRegistry{
		models:        make(map[string]*RegisteredModel),
		configs:       make(map[string]*ModelConfig),
		healthChecker: NewHealthChecker(registryCtx),
		configManager: NewConfigManager(),
		ctx:           registryCtx,
		cancel:        cancel,
	}
	
	// 启动后台健康检查
	registry.wg.Add(1)
	go registry.startHealthCheck()
	
	return registry
}

// RegisterModel 注册模型
func (mr *ModelRegistry) RegisterModel(config *ModelConfig) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	// 验证配置
	if err := mr.validateModelConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}
	
	// 创建LangChainGo模型实例
	model, err := mr.createModelInstance(config)
	if err != nil {
		return fmt.Errorf("创建模型实例失败: %w", err)
	}
	
	// 创建注册模型
	registered := &RegisteredModel{
		Name:      config.Name,
		Provider:  config.Provider,
		Category:  ComplexityCategory(config.Category),
		Model:     model,
		Config:    config,
		Health:    &ModelHealth{
			Status:        ModelStatusUnknown,
			LastCheck:     time.Now(),
			Availability:  0.0,
		},
		CreatedAt: time.Now(),
		Status:    ModelStatusUnknown,
		Metadata:  make(map[string]interface{}),
	}
	
	// 存储注册信息
	mr.models[config.Name] = registered
	mr.configs[config.Name] = config
	
	// 立即进行健康检查
	go mr.performInitialHealthCheck(registered)
	
	return nil
}

// createModelInstance 创建LangChainGo模型实例
func (mr *ModelRegistry) createModelInstance(config *ModelConfig) (llms.Model, error) {
	switch config.Provider {
	case "openai":
		return mr.createOpenAIModel(config)
	case "anthropic":
		return mr.createAnthropicModel(config)
	case "deepseek":
		return mr.createDeepSeekModel(config)
	case "ollama":
		return mr.createOllamaModel(config)
	default:
		return nil, fmt.Errorf("不支持的提供商: %s", config.Provider)
	}
}

// createOpenAIModel 创建OpenAI模型实例
func (mr *ModelRegistry) createOpenAIModel(config *ModelConfig) (llms.Model, error) {
	options := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Name),
	}
	
	if config.Endpoint != "" {
		options = append(options, openai.WithBaseURL(config.Endpoint))
	}
	
	// 注意：LangChainGo v0.1.13可能不支持直接在New时设置Temperature等参数
	// 这些参数通常在GenerateContent时通过CallOption设置
	
	return openai.New(options...)
}

// createAnthropicModel 创建Anthropic模型实例
func (mr *ModelRegistry) createAnthropicModel(config *ModelConfig) (llms.Model, error) {
	options := []anthropic.Option{
		anthropic.WithToken(config.APIKey),
		anthropic.WithModel(config.Name),
	}
	
	// 注意：LangChainGo v0.1.13中Temperature等参数通常在GenerateContent时设置
	
	return anthropic.New(options...)
}

// createDeepSeekModel 创建DeepSeek模型实例（使用OpenAI兼容接口）
func (mr *ModelRegistry) createDeepSeekModel(config *ModelConfig) (llms.Model, error) {
	options := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Name),
		openai.WithBaseURL(config.Endpoint), // DeepSeek API endpoint
	}
	
	// 注意：LangChainGo v0.1.13中Temperature等参数通常在GenerateContent时设置
	
	return openai.New(options...)
}

// createOllamaModel 创建Ollama模型实例
func (mr *ModelRegistry) createOllamaModel(config *ModelConfig) (llms.Model, error) {
	options := []ollama.Option{
		ollama.WithModel(config.Name),
	}
	
	if config.Endpoint != "" {
		options = append(options, ollama.WithServerURL(config.Endpoint))
	}
	
	return ollama.New(options...)
}

// validateModelConfig 验证模型配置
func (mr *ModelRegistry) validateModelConfig(config *ModelConfig) error {
	if config.Name == "" {
		return fmt.Errorf("模型名称不能为空")
	}
	
	if config.Provider == "" {
		return fmt.Errorf("模型提供商不能为空")
	}
	
	if config.Category == "" {
		return fmt.Errorf("模型类别不能为空")
	}
	
	// 验证类别有效性
	switch ComplexityCategory(config.Category) {
	case CategorySimple, CategoryMedium, CategoryComplex:
		// 有效类别
	default:
		return fmt.Errorf("无效的模型类别: %s", config.Category)
	}
	
	// 验证API密钥（本地模型除外）
	if config.Provider != "ollama" && config.APIKey == "" {
		return fmt.Errorf("API密钥不能为空")
	}
	
	return nil
}

// GetModel 获取已注册的模型
func (mr *ModelRegistry) GetModel(name string) (*RegisteredModel, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	model, exists := mr.models[name]
	if !exists {
		return nil, fmt.Errorf("模型 %s 未注册", name)
	}
	
	return model, nil
}

// GetModelsByCategory 按类别获取模型
func (mr *ModelRegistry) GetModelsByCategory(category ComplexityCategory) []*RegisteredModel {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	var models []*RegisteredModel
	for _, model := range mr.models {
		if model.Category == category && model.Config.Enabled {
			models = append(models, model)
		}
	}
	
	return models
}

// GetHealthyModels 获取健康的模型
func (mr *ModelRegistry) GetHealthyModels() []*RegisteredModel {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	var models []*RegisteredModel
	for _, model := range mr.models {
		if model.Health.Status == ModelStatusHealthy && model.Config.Enabled {
			models = append(models, model)
		}
	}
	
	return models
}

// GetAllModels 获取所有注册的模型
func (mr *ModelRegistry) GetAllModels() []*RegisteredModel {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	models := make([]*RegisteredModel, 0, len(mr.models))
	for _, model := range mr.models {
		models = append(models, model)
	}
	
	return models
}

// UnregisterModel 注销模型
func (mr *ModelRegistry) UnregisterModel(name string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	model, exists := mr.models[name]
	if !exists {
		return fmt.Errorf("模型 %s 未注册", name)
	}
	
	// 清理资源
	delete(mr.models, name)
	delete(mr.configs, name)
	
	// 记录注销
	model.Status = ModelStatusUnknown
	
	return nil
}

// UpdateModelConfig 更新模型配置
func (mr *ModelRegistry) UpdateModelConfig(name string, newConfig *ModelConfig) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	// 验证模型是否存在
	model, exists := mr.models[name]
	if !exists {
		return fmt.Errorf("模型 %s 未注册", name)
	}
	
	// 验证新配置
	if err := mr.validateModelConfig(newConfig); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}
	
	// 如果关键参数发生变化，需要重新创建模型实例
	if mr.needRecreateModel(model.Config, newConfig) {
		newModel, err := mr.createModelInstance(newConfig)
		if err != nil {
			return fmt.Errorf("重新创建模型实例失败: %w", err)
		}
		model.Model = newModel
	}
	
	// 更新配置
	model.Config = newConfig
	model.Category = ComplexityCategory(newConfig.Category) // 更新类别
	mr.configs[name] = newConfig
	
	// 触发健康检查
	go mr.performInitialHealthCheck(model)
	
	return nil
}

// needRecreateModel 判断是否需要重新创建模型实例
func (mr *ModelRegistry) needRecreateModel(oldConfig, newConfig *ModelConfig) bool {
	// 关键参数变化需要重新创建
	return oldConfig.Provider != newConfig.Provider ||
		   oldConfig.APIKey != newConfig.APIKey ||
		   oldConfig.Endpoint != newConfig.Endpoint ||
		   oldConfig.Name != newConfig.Name
}

// performInitialHealthCheck 执行初始健康检查
func (mr *ModelRegistry) performInitialHealthCheck(model *RegisteredModel) {
	ctx, cancel := context.WithTimeout(mr.ctx, 30*time.Second)
	defer cancel()
	
	result := mr.healthChecker.CheckModelHealth(ctx, model)
	
	mr.mu.Lock()
	model.Health = result
	if result.Status == ModelStatusHealthy {
		model.Status = ModelStatusHealthy
	} else {
		model.Status = ModelStatusUnhealthy
	}
	mr.mu.Unlock()
}

// startHealthCheck 启动后台健康检查
func (mr *ModelRegistry) startHealthCheck() {
	defer mr.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mr.performHealthChecks()
		case <-mr.ctx.Done():
			return
		}
	}
}

// performHealthChecks 执行所有模型的健康检查
func (mr *ModelRegistry) performHealthChecks() {
	mr.mu.RLock()
	models := make([]*RegisteredModel, 0, len(mr.models))
	for _, model := range mr.models {
		if model.Config.Enabled {
			models = append(models, model)
		}
	}
	mr.mu.RUnlock()
	
	// 并发检查所有模型
	var wg sync.WaitGroup
	for _, model := range models {
		wg.Add(1)
		go func(m *RegisteredModel) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(mr.ctx, 15*time.Second)
			defer cancel()
			
			result := mr.healthChecker.CheckModelHealth(ctx, m)
			
			mr.mu.Lock()
			m.Health = result
			m.Status = result.Status
			mr.mu.Unlock()
		}(model)
	}
	
	wg.Wait()
}

// GetRegistryStats 获取注册中心统计信息
func (mr *ModelRegistry) GetRegistryStats() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_models":   len(mr.models),
		"healthy_models": 0,
		"degraded_models": 0,
		"unhealthy_models": 0,
		"models_by_provider": make(map[string]int),
		"models_by_category": make(map[string]int),
	}
	
	providerStats := stats["models_by_provider"].(map[string]int)
	categoryStats := stats["models_by_category"].(map[string]int)
	
	for _, model := range mr.models {
		// 健康状态统计
		switch model.Health.Status {
		case ModelStatusHealthy:
			stats["healthy_models"] = stats["healthy_models"].(int) + 1
		case ModelStatusDegraded:
			stats["degraded_models"] = stats["degraded_models"].(int) + 1
		case ModelStatusUnhealthy:
			stats["unhealthy_models"] = stats["unhealthy_models"].(int) + 1
		}
		
		// 提供商统计
		providerStats[model.Provider]++
		
		// 类别统计
		categoryStats[string(model.Category)]++
	}
	
	return stats
}

// Close 优雅关闭注册中心
func (mr *ModelRegistry) Close() error {
	mr.cancel()
	mr.wg.Wait()
	
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	// 清理所有模型
	mr.models = nil
	mr.configs = nil
	
	return nil
}