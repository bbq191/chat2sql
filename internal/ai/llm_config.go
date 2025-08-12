// LLM提供商配置管理
// 支持OpenAI, Anthropic, Ollama等多种提供商
// 基于环境变量的动态配置加载

package ai

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LLMProvider 定义LLM提供商类型
type LLMProvider string

const (
	ProviderOpenAI     LLMProvider = "openai"
	ProviderAnthropic  LLMProvider = "anthropic"  
	ProviderOllama     LLMProvider = "ollama"
	ProviderGoogleAI   LLMProvider = "googleai"
	ProviderHuggingFace LLMProvider = "huggingface"
)

// LLMConfig 单个LLM提供商配置
type LLMConfig struct {
	Provider    LLMProvider `json:"provider"`
	APIKey      string      `json:"api_key,omitempty"`
	Model       string      `json:"model"`
	BaseURL     string      `json:"base_url,omitempty"`
	Temperature float64     `json:"temperature"`
	MaxTokens   int         `json:"max_tokens"`
	TopP        float64     `json:"top_p,omitempty"`
}

// LLMRouterConfig LLM路由配置
type LLMRouterConfig struct {
	// 主要模型配置
	PrimaryProvider  LLMProvider `json:"primary_provider"`
	PrimaryConfig    *LLMConfig  `json:"primary_config"`
	
	// 备用模型配置
	FallbackProvider LLMProvider `json:"fallback_provider"`
	FallbackConfig   *LLMConfig  `json:"fallback_config"`
	
	// 本地模型配置（敏感数据查询）
	LocalProvider    LLMProvider `json:"local_provider"`
	LocalConfig      *LLMConfig  `json:"local_config"`
	
	// 成本控制
	ExtendedCostConfig *ExtendedCostConfig `json:"extended_cost_config"`
	
	// 性能配置
	PerformanceConfig *PerformanceConfig `json:"performance_config"`
	
	// 超时配置
	RequestTimeout   time.Duration `json:"request_timeout"`
}

// ExtendedCostConfig 扩展成本控制配置
type ExtendedCostConfig struct {
	MaxCostPerQueryCents int     `json:"max_cost_per_query_cents"` // 单次查询最大成本（美分）
	DailyBudgetPerUser   float64 `json:"daily_budget_per_user"`    // 用户日预算（美元）
	TotalDailyBudget     float64 `json:"total_daily_budget"`       // 系统总日预算（美元）
	
	// 价格表（每1K tokens的价格，美分）
	OpenAIPricing map[string]OpenAIPricing `json:"openai_pricing"`
	AnthropicPricing map[string]AnthropicPricing `json:"anthropic_pricing"`
}

// OpenAI定价结构
type OpenAIPricing struct {
	InputPrice  float64 `json:"input_price"`  // 输入token价格（美分/1K tokens）
	OutputPrice float64 `json:"output_price"` // 输出token价格（美分/1K tokens）
}

// Anthropic定价结构
type AnthropicPricing struct {
	InputPrice  float64 `json:"input_price"`  // 输入token价格（美分/1K tokens）
	OutputPrice float64 `json:"output_price"` // 输出token价格（美分/1K tokens）
}

// LoadLLMConfig 从环境变量加载LLM配置
func LoadLLMConfig() (*LLMRouterConfig, error) {
	config := &LLMRouterConfig{
		RequestTimeout: 30 * time.Second,
	}
	
	// 加载主要模型配置
	primaryProvider := LLMProvider(getEnvWithDefault("PRIMARY_LLM_PROVIDER", "openai"))
	config.PrimaryProvider = primaryProvider
	
	var err error
	config.PrimaryConfig, err = loadProviderConfig(primaryProvider)
	if err != nil {
		return nil, fmt.Errorf("loading primary provider config: %w", err)
	}
	
	// 加载备用模型配置
	fallbackProvider := LLMProvider(getEnvWithDefault("FALLBACK_LLM_PROVIDER", "anthropic"))
	config.FallbackProvider = fallbackProvider
	config.FallbackConfig, err = loadProviderConfig(fallbackProvider)
	if err != nil {
		return nil, fmt.Errorf("loading fallback provider config: %w", err)
	}
	
	// 加载本地模型配置
	localProvider := LLMProvider(getEnvWithDefault("LOCAL_LLM_PROVIDER", "ollama"))
	config.LocalProvider = localProvider
	config.LocalConfig, err = loadProviderConfig(localProvider)
	if err != nil {
		// 本地模型配置失败不影响系统启动，只记录警告
		// 这里只是设置为 nil，实际的日志记录在使用该配置的地方进行
		config.LocalConfig = nil
	}
	
	// 加载成本控制配置
	config.ExtendedCostConfig = loadExtendedCostConfig()
	
	// 加载性能配置
	config.PerformanceConfig = loadPerformanceConfigFromEnv()
	
	// 加载超时配置
	if timeoutStr := os.Getenv("AI_REQUEST_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.RequestTimeout = time.Duration(timeout) * time.Second
		}
	}
	
	return config, nil
}

// loadProviderConfig 加载特定提供商的配置
func loadProviderConfig(provider LLMProvider) (*LLMConfig, error) {
	config := &LLMConfig{Provider: provider}
	
	switch provider {
	case ProviderOpenAI:
		config.APIKey = os.Getenv("OPENAI_API_KEY")
		config.Model = getEnvWithDefault("OPENAI_MODEL", "gpt-4o-mini")
		config.BaseURL = os.Getenv("OPENAI_BASE_URL") // 可选，支持Azure OpenAI
		config.Temperature = getFloatEnvWithDefault("OPENAI_TEMPERATURE", 0.1)
		config.MaxTokens = getIntEnvWithDefault("OPENAI_MAX_TOKENS", 2048)
		config.TopP = getFloatEnvWithDefault("OPENAI_TOP_P", 0.9)
		
		if config.APIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
		}
		
	case ProviderAnthropic:
		config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		config.Model = getEnvWithDefault("ANTHROPIC_MODEL", "claude-3-haiku-20240307")
		config.Temperature = getFloatEnvWithDefault("ANTHROPIC_TEMPERATURE", 0.0)
		config.MaxTokens = getIntEnvWithDefault("ANTHROPIC_MAX_TOKENS", 1024)
		
		if config.APIKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
		}
		
	case ProviderOllama:
		config.BaseURL = getEnvWithDefault("OLLAMA_SERVER_URL", "http://localhost:11434")
		config.Model = getEnvWithDefault("OLLAMA_MODEL", "llama3.1:8b")
		config.Temperature = getFloatEnvWithDefault("OLLAMA_TEMPERATURE", 0.1)
		config.MaxTokens = getIntEnvWithDefault("OLLAMA_MAX_TOKENS", 2048)
		// Ollama不需要API密钥
		
		
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
	
	return config, nil
}

// loadExtendedCostConfig 加载扩展成本控制配置
func loadExtendedCostConfig() *ExtendedCostConfig {
	return &ExtendedCostConfig{
		MaxCostPerQueryCents: getIntEnvWithDefault("MAX_COST_PER_QUERY_CENTS", 5),
		DailyBudgetPerUser:   getFloatEnvWithDefault("DAILY_BUDGET_PER_USER", 10.0),
		TotalDailyBudget:     getFloatEnvWithDefault("TOTAL_DAILY_BUDGET", 500.0),
		OpenAIPricing:        getDefaultOpenAIPricing(),
		AnthropicPricing:     getDefaultAnthropicPricing(),
	}
}

// loadPerformanceConfigFromEnv 从环境变量加载性能配置
func loadPerformanceConfigFromEnv() *PerformanceConfig {
	config := DefaultPerformanceConfig()
	
	if workers := os.Getenv("AI_WORKERS"); workers != "" {
		if w, err := strconv.Atoi(workers); err == nil {
			config.Workers = w
		}
	}
	
	if queueSize := os.Getenv("AI_QUEUE_SIZE"); queueSize != "" {
		if qs, err := strconv.Atoi(queueSize); err == nil {
			config.QueueSize = qs
		}
	}
	
	if rateLimit := os.Getenv("AI_RATE_LIMIT"); rateLimit != "" {
		if rl, err := strconv.Atoi(rateLimit); err == nil {
			config.RateLimit = rl
		}
	}
	
	// 缓存配置
	if enableCache := os.Getenv("ENABLE_AI_CACHE"); enableCache != "" {
		config.EnableCache = strings.ToLower(enableCache) == "true"
	}
	
	if cacheTTL := os.Getenv("AI_CACHE_TTL_MINUTES"); cacheTTL != "" {
		if ttl, err := strconv.Atoi(cacheTTL); err == nil {
			config.CacheTTL = time.Duration(ttl) * time.Minute
		}
	}
	
	if cacheSize := os.Getenv("AI_CACHE_SIZE"); cacheSize != "" {
		if cs, err := strconv.Atoi(cacheSize); err == nil {
			config.CacheSize = cs
		}
	}
	
	return config
}

// getDefaultOpenAIPricing 获取默认OpenAI定价（2024年价格）
func getDefaultOpenAIPricing() map[string]OpenAIPricing {
	return map[string]OpenAIPricing{
		"gpt-4o": {
			InputPrice:  0.25,  // $0.0025/1K input tokens
			OutputPrice: 1.0,   // $0.01/1K output tokens
		},
		"gpt-4o-mini": {
			InputPrice:  0.015, // $0.00015/1K input tokens
			OutputPrice: 0.06,  // $0.0006/1K output tokens
		},
		"gpt-4": {
			InputPrice:  3.0,   // $0.03/1K input tokens
			OutputPrice: 6.0,   // $0.06/1K output tokens
		},
		"gpt-3.5-turbo": {
			InputPrice:  0.05,  // $0.0005/1K input tokens
			OutputPrice: 0.15,  // $0.0015/1K output tokens
		},
	}
}

// getDefaultAnthropicPricing 获取默认Anthropic定价（2024年价格）
func getDefaultAnthropicPricing() map[string]AnthropicPricing {
	return map[string]AnthropicPricing{
		"claude-3-opus-20240229": {
			InputPrice:  1.5,   // $0.015/1K input tokens
			OutputPrice: 7.5,   // $0.075/1K output tokens
		},
		"claude-3-sonnet-20240229": {
			InputPrice:  0.3,   // $0.003/1K input tokens
			OutputPrice: 1.5,   // $0.015/1K output tokens
		},
		"claude-3-haiku-20240307": {
			InputPrice:  0.025, // $0.00025/1K input tokens
			OutputPrice: 0.125, // $0.00125/1K output tokens
		},
	}
}

// 辅助函数：获取环境变量，如果不存在则返回默认值
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// 辅助函数：获取整数类型环境变量
func getIntEnvWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// 辅助函数：获取浮点数类型环境变量
func getFloatEnvWithDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}