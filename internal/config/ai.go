package config

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
)

// AIConfig AI服务配置
type AIConfig struct {
	Primary  ModelConfig   `yaml:"primary"`
	Fallback ModelConfig   `yaml:"fallback"`
	
	// 性能配置
	MaxConcurrency int           `yaml:"max_concurrency"`
	Timeout        time.Duration `yaml:"timeout"`
	
	// 成本控制
	Budget BudgetConfig `yaml:"budget"`
}

// ModelConfig 单个模型配置
type ModelConfig struct {
	Provider    string        `yaml:"provider"`
	ModelName   string        `yaml:"model_name"`
	APIKey      string        `yaml:"api_key"`
	Temperature float64       `yaml:"temperature"`
	MaxTokens   int           `yaml:"max_tokens"`
	TopP        float64       `yaml:"top_p"`
	Timeout     time.Duration `yaml:"timeout"`
}

// BudgetConfig 预算配置
type BudgetConfig struct {
	DailyLimit     float64 `yaml:"daily_limit"`     // 每日预算上限（美元）
	UserLimit      float64 `yaml:"user_limit"`      // 每用户限制
	AlertThreshold float64 `yaml:"alert_threshold"` // 告警阈值
}

// DefaultAIConfig 创建默认AI配置
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		Primary: ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini",
			Temperature: 0.1,
			MaxTokens:   2048,
			TopP:        0.9,
			Timeout:     30 * time.Second,
		},
		Fallback: ModelConfig{
			Provider:    "anthropic",
			ModelName:   "claude-3-haiku-20240307",
			Temperature: 0.0,
			MaxTokens:   1024,
			TopP:        0.9,
			Timeout:     30 * time.Second,
		},
		MaxConcurrency: 10,
		Timeout:        30 * time.Second,
		Budget: BudgetConfig{
			DailyLimit:     100.0, // $100 per day
			UserLimit:      10.0,  // $10 per user per day
			AlertThreshold: 0.8,   // 80% of limit
		},
	}
}

// LoadAIConfigFromEnv 从环境变量加载AI配置
func LoadAIConfigFromEnv() (*AIConfig, error) {
	config := DefaultAIConfig()
	
	// 加载API密钥
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		config.Primary.APIKey = openaiKey
	} else {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}
	
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		config.Fallback.APIKey = anthropicKey
	} else {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}
	
	// 可选的模型配置覆盖
	if primaryModel := os.Getenv("PRIMARY_MODEL_NAME"); primaryModel != "" {
		config.Primary.ModelName = primaryModel
	}
	
	if fallbackModel := os.Getenv("FALLBACK_MODEL_NAME"); fallbackModel != "" {
		config.Fallback.ModelName = fallbackModel
	}
	
	// 性能配置
	if llmTimeout := os.Getenv("LLM_TIMEOUT"); llmTimeout != "" {
		if duration, err := time.ParseDuration(llmTimeout); err == nil {
			config.Timeout = duration
			config.Primary.Timeout = duration
			config.Fallback.Timeout = duration
		}
	}
	
	return config, nil
}

// Validate 验证AI配置的有效性
func (c *AIConfig) Validate() error {
	// 验证主要模型配置
	if err := c.Primary.validate(); err != nil {
		return fmt.Errorf("primary model config invalid: %w", err)
	}
	
	// 验证备用模型配置
	if err := c.Fallback.validate(); err != nil {
		return fmt.Errorf("fallback model config invalid: %w", err)
	}
	
	// 验证性能配置
	if c.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be positive, got: %d", c.MaxConcurrency)
	}
	
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", c.Timeout)
	}
	
	// 验证预算配置
	if c.Budget.DailyLimit <= 0 {
		return fmt.Errorf("daily_limit must be positive, got: %.2f", c.Budget.DailyLimit)
	}
	
	if c.Budget.UserLimit <= 0 {
		return fmt.Errorf("user_limit must be positive, got: %.2f", c.Budget.UserLimit)
	}
	
	if c.Budget.AlertThreshold <= 0 || c.Budget.AlertThreshold > 1 {
		return fmt.Errorf("alert_threshold must be between 0 and 1, got: %.2f", c.Budget.AlertThreshold)
	}
	
	return nil
}

// validate 验证单个模型配置
func (mc *ModelConfig) validate() error {
	if mc.Provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	
	if mc.ModelName == "" {
		return fmt.Errorf("model_name cannot be empty")
	}
	
	if mc.APIKey == "" {
		return fmt.Errorf("api_key cannot be empty")
	}
	
	if mc.Temperature < 0 || mc.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got: %.2f", mc.Temperature)
	}
	
	if mc.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive, got: %d", mc.MaxTokens)
	}
	
	if mc.TopP <= 0 || mc.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1, got: %.2f", mc.TopP)
	}
	
	if mc.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", mc.Timeout)
	}
	
	return nil
}

// LogConfig 记录AI配置信息（不包含敏感信息）
func (c *AIConfig) LogConfig(logger *zap.Logger) {
	logger.Info("AI服务配置",
		zap.String("primary_provider", c.Primary.Provider),
		zap.String("primary_model", c.Primary.ModelName),
		zap.Float64("primary_temperature", c.Primary.Temperature),
		zap.Int("primary_max_tokens", c.Primary.MaxTokens),
		zap.String("fallback_provider", c.Fallback.Provider),
		zap.String("fallback_model", c.Fallback.ModelName),
		zap.Float64("fallback_temperature", c.Fallback.Temperature),
		zap.Int("fallback_max_tokens", c.Fallback.MaxTokens),
		zap.Int("max_concurrency", c.MaxConcurrency),
		zap.Duration("timeout", c.Timeout),
		zap.Float64("daily_budget_limit", c.Budget.DailyLimit),
		zap.Float64("user_budget_limit", c.Budget.UserLimit),
		zap.Float64("alert_threshold", c.Budget.AlertThreshold),
	)
}

// GetModelCosts 获取模型成本信息（美元/1K tokens）
func GetModelCosts() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"openai": {
			"gpt-4o":        0.015, // input + output average
			"gpt-4o-mini":   0.0015,
			"gpt-4-turbo":   0.020,
			"gpt-3.5-turbo": 0.0020,
		},
		"anthropic": {
			"claude-3-opus-20240229":   0.045,
			"claude-3-sonnet-20240229": 0.012,
			"claude-3-haiku-20240307":  0.0008,
		},
	}
}

// EstimateTokenCost 估算Token成本
func EstimateTokenCost(provider, model string, tokens int) float64 {
	costs := GetModelCosts()
	
	if providerCosts, exists := costs[provider]; exists {
		if modelCost, exists := providerCosts[model]; exists {
			return float64(tokens) * modelCost / 1000.0
		}
	}
	
	// 默认成本估算
	return float64(tokens) * 0.002 / 1000.0
}