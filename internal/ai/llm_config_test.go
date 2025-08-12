// LLM配置管理测试 - 完整测试覆盖多种LLM提供商的配置加载
// 测试环境变量配置加载、提供商配置、成本控制等核心功能

package ai

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLLMProvider 测试LLM提供商枚举
func TestLLMProvider(t *testing.T) {
	assert.Equal(t, "openai", string(ProviderOpenAI))
	assert.Equal(t, "anthropic", string(ProviderAnthropic))
	assert.Equal(t, "ollama", string(ProviderOllama))
	assert.Equal(t, "googleai", string(ProviderGoogleAI))
	assert.Equal(t, "huggingface", string(ProviderHuggingFace))
}

// TestLLMConfig 测试LLM配置结构
func TestLLMConfig(t *testing.T) {
	config := &LLMConfig{
		Provider:    ProviderOpenAI,
		APIKey:      "test-api-key",
		Model:       "gpt-4o-mini",
		BaseURL:     "https://api.openai.com/v1",
		Temperature: 0.1,
		MaxTokens:   2048,
		TopP:        0.9,
	}

	assert.Equal(t, ProviderOpenAI, config.Provider)
	assert.Equal(t, "test-api-key", config.APIKey)
	assert.Equal(t, "gpt-4o-mini", config.Model)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, 0.1, config.Temperature)
	assert.Equal(t, 2048, config.MaxTokens)
	assert.Equal(t, 0.9, config.TopP)
}

// TestLLMRouterConfig 测试LLM路由配置结构
func TestLLMRouterConfig(t *testing.T) {
	primaryConfig := &LLMConfig{
		Provider: ProviderOpenAI,
		Model:    "gpt-4o-mini",
	}
	
	fallbackConfig := &LLMConfig{
		Provider: ProviderAnthropic,
		Model:    "claude-3-haiku-20240307",
	}

	routerConfig := &LLMRouterConfig{
		PrimaryProvider:  ProviderOpenAI,
		PrimaryConfig:    primaryConfig,
		FallbackProvider: ProviderAnthropic,
		FallbackConfig:   fallbackConfig,
		RequestTimeout:   30 * time.Second,
	}

	assert.Equal(t, ProviderOpenAI, routerConfig.PrimaryProvider)
	assert.Equal(t, primaryConfig, routerConfig.PrimaryConfig)
	assert.Equal(t, ProviderAnthropic, routerConfig.FallbackProvider)
	assert.Equal(t, fallbackConfig, routerConfig.FallbackConfig)
	assert.Equal(t, 30*time.Second, routerConfig.RequestTimeout)
}

// TestLoadProviderConfig_OpenAI 测试OpenAI提供商配置加载
func TestLoadProviderConfig_OpenAI(t *testing.T) {
	// 设置测试环境变量
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	originalModel := os.Getenv("OPENAI_MODEL")
	originalTemp := os.Getenv("OPENAI_TEMPERATURE")
	
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalAPIKey)
		os.Setenv("OPENAI_MODEL", originalModel)
		os.Setenv("OPENAI_TEMPERATURE", originalTemp)
	}()

	os.Setenv("OPENAI_API_KEY", "test-key-123")
	os.Setenv("OPENAI_MODEL", "gpt-4")
	os.Setenv("OPENAI_TEMPERATURE", "0.2")

	config, err := loadProviderConfig(ProviderOpenAI)
	
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, ProviderOpenAI, config.Provider)
	assert.Equal(t, "test-key-123", config.APIKey)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, 0.2, config.Temperature)
	assert.Equal(t, 2048, config.MaxTokens) // 默认值
}

// TestLoadProviderConfig_OpenAI_MissingAPIKey 测试缺失API密钥的错误处理
func TestLoadProviderConfig_OpenAI_MissingAPIKey(t *testing.T) {
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalAPIKey)
	
	os.Unsetenv("OPENAI_API_KEY")

	config, err := loadProviderConfig(ProviderOpenAI)
	
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY environment variable is required")
}

// TestLoadProviderConfig_Anthropic 测试Anthropic提供商配置加载
func TestLoadProviderConfig_Anthropic(t *testing.T) {
	originalAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	originalModel := os.Getenv("ANTHROPIC_MODEL")
	
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", originalAPIKey)
		os.Setenv("ANTHROPIC_MODEL", originalModel)
	}()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("ANTHROPIC_MODEL", "claude-3-opus-20240229")

	config, err := loadProviderConfig(ProviderAnthropic)
	
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, ProviderAnthropic, config.Provider)
	assert.Equal(t, "test-anthropic-key", config.APIKey)
	assert.Equal(t, "claude-3-opus-20240229", config.Model)
	assert.Equal(t, 0.0, config.Temperature) // 默认值
	assert.Equal(t, 1024, config.MaxTokens) // 默认值
}

// TestLoadProviderConfig_Ollama 测试Ollama提供商配置加载
func TestLoadProviderConfig_Ollama(t *testing.T) {
	originalURL := os.Getenv("OLLAMA_SERVER_URL")
	originalModel := os.Getenv("OLLAMA_MODEL")
	
	defer func() {
		os.Setenv("OLLAMA_SERVER_URL", originalURL)
		os.Setenv("OLLAMA_MODEL", originalModel)
	}()

	os.Setenv("OLLAMA_SERVER_URL", "http://localhost:11434")
	os.Setenv("OLLAMA_MODEL", "deepseek-r1:7b")

	config, err := loadProviderConfig(ProviderOllama)
	
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, ProviderOllama, config.Provider)
	assert.Equal(t, "", config.APIKey) // Ollama不需要API密钥
	assert.Equal(t, "deepseek-r1:7b", config.Model)
	assert.Equal(t, "http://localhost:11434", config.BaseURL)
	assert.Equal(t, 0.1, config.Temperature) // 默认值
	assert.Equal(t, 2048, config.MaxTokens) // 默认值
}

// TestLoadProviderConfig_UnsupportedProvider 测试不支持的提供商
func TestLoadProviderConfig_UnsupportedProvider(t *testing.T) {
	config, err := loadProviderConfig(LLMProvider("unsupported"))
	
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "unsupported LLM provider")
}

// TestLoadExtendedCostConfig 测试扩展成本配置加载
func TestLoadExtendedCostConfig(t *testing.T) {
	originalMaxCost := os.Getenv("MAX_COST_PER_QUERY_CENTS")
	originalDailyBudget := os.Getenv("DAILY_BUDGET_PER_USER")
	originalTotalBudget := os.Getenv("TOTAL_DAILY_BUDGET")
	
	defer func() {
		os.Setenv("MAX_COST_PER_QUERY_CENTS", originalMaxCost)
		os.Setenv("DAILY_BUDGET_PER_USER", originalDailyBudget)
		os.Setenv("TOTAL_DAILY_BUDGET", originalTotalBudget)
	}()

	os.Setenv("MAX_COST_PER_QUERY_CENTS", "10")
	os.Setenv("DAILY_BUDGET_PER_USER", "20.0")
	os.Setenv("TOTAL_DAILY_BUDGET", "1000.0")

	config := loadExtendedCostConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, 10, config.MaxCostPerQueryCents)
	assert.Equal(t, 20.0, config.DailyBudgetPerUser)
	assert.Equal(t, 1000.0, config.TotalDailyBudget)
	assert.NotNil(t, config.OpenAIPricing)
	assert.NotNil(t, config.AnthropicPricing)
}

// TestGetDefaultOpenAIPricing 测试默认OpenAI定价获取
func TestGetDefaultOpenAIPricing(t *testing.T) {
	pricing := getDefaultOpenAIPricing()
	
	assert.NotNil(t, pricing)
	assert.Contains(t, pricing, "gpt-4o")
	assert.Contains(t, pricing, "gpt-4o-mini")
	assert.Contains(t, pricing, "gpt-4")
	assert.Contains(t, pricing, "gpt-3.5-turbo")

	// 验证gpt-4o-mini的定价
	gpt4oMini := pricing["gpt-4o-mini"]
	assert.Equal(t, 0.015, gpt4oMini.InputPrice)
	assert.Equal(t, 0.06, gpt4oMini.OutputPrice)

	// 验证gpt-4的定价
	gpt4 := pricing["gpt-4"]
	assert.Equal(t, 3.0, gpt4.InputPrice)
	assert.Equal(t, 6.0, gpt4.OutputPrice)
}

// TestGetDefaultAnthropicPricing 测试默认Anthropic定价获取
func TestGetDefaultAnthropicPricing(t *testing.T) {
	pricing := getDefaultAnthropicPricing()
	
	assert.NotNil(t, pricing)
	assert.Contains(t, pricing, "claude-3-opus-20240229")
	assert.Contains(t, pricing, "claude-3-sonnet-20240229")
	assert.Contains(t, pricing, "claude-3-haiku-20240307")

	// 验证haiku的定价
	haiku := pricing["claude-3-haiku-20240307"]
	assert.Equal(t, 0.025, haiku.InputPrice)
	assert.Equal(t, 0.125, haiku.OutputPrice)

	// 验证opus的定价
	opus := pricing["claude-3-opus-20240229"]
	assert.Equal(t, 1.5, opus.InputPrice)
	assert.Equal(t, 7.5, opus.OutputPrice)
}

// TestGetEnvWithDefault 测试环境变量默认值函数
func TestGetEnvWithDefault(t *testing.T) {
	// 测试环境变量不存在时返回默认值
	result := getEnvWithDefault("NON_EXISTENT_VAR", "default_value")
	assert.Equal(t, "default_value", result)

	// 测试环境变量存在时返回实际值
	os.Setenv("TEST_VAR", "actual_value")
	defer os.Unsetenv("TEST_VAR")
	
	result = getEnvWithDefault("TEST_VAR", "default_value")
	assert.Equal(t, "actual_value", result)
}

// TestGetIntEnvWithDefault 测试整数环境变量默认值函数
func TestGetIntEnvWithDefault(t *testing.T) {
	// 测试环境变量不存在时返回默认值
	result := getIntEnvWithDefault("NON_EXISTENT_INT_VAR", 42)
	assert.Equal(t, 42, result)

	// 测试有效整数环境变量
	os.Setenv("TEST_INT_VAR", "123")
	defer os.Unsetenv("TEST_INT_VAR")
	
	result = getIntEnvWithDefault("TEST_INT_VAR", 42)
	assert.Equal(t, 123, result)

	// 测试无效整数环境变量时返回默认值
	os.Setenv("TEST_INVALID_INT_VAR", "not_a_number")
	defer os.Unsetenv("TEST_INVALID_INT_VAR")
	
	result = getIntEnvWithDefault("TEST_INVALID_INT_VAR", 42)
	assert.Equal(t, 42, result)
}

// TestGetFloatEnvWithDefault 测试浮点数环境变量默认值函数
func TestGetFloatEnvWithDefault(t *testing.T) {
	// 测试环境变量不存在时返回默认值
	result := getFloatEnvWithDefault("NON_EXISTENT_FLOAT_VAR", 3.14)
	assert.Equal(t, 3.14, result)

	// 测试有效浮点数环境变量
	os.Setenv("TEST_FLOAT_VAR", "2.71")
	defer os.Unsetenv("TEST_FLOAT_VAR")
	
	result = getFloatEnvWithDefault("TEST_FLOAT_VAR", 3.14)
	assert.Equal(t, 2.71, result)

	// 测试无效浮点数环境变量时返回默认值
	os.Setenv("TEST_INVALID_FLOAT_VAR", "not_a_float")
	defer os.Unsetenv("TEST_INVALID_FLOAT_VAR")
	
	result = getFloatEnvWithDefault("TEST_INVALID_FLOAT_VAR", 3.14)
	assert.Equal(t, 3.14, result)
}

// TestLoadPerformanceConfigFromEnv 测试从环境变量加载性能配置
func TestLoadPerformanceConfigFromEnv(t *testing.T) {
	// 保存原始环境变量
	originalWorkers := os.Getenv("AI_WORKERS")
	originalQueueSize := os.Getenv("AI_QUEUE_SIZE")
	originalRateLimit := os.Getenv("AI_RATE_LIMIT")
	originalEnableCache := os.Getenv("ENABLE_AI_CACHE")
	
	defer func() {
		os.Setenv("AI_WORKERS", originalWorkers)
		os.Setenv("AI_QUEUE_SIZE", originalQueueSize)
		os.Setenv("AI_RATE_LIMIT", originalRateLimit)
		os.Setenv("ENABLE_AI_CACHE", originalEnableCache)
	}()

	// 设置测试环境变量
	os.Setenv("AI_WORKERS", "8")
	os.Setenv("AI_QUEUE_SIZE", "500")
	os.Setenv("AI_RATE_LIMIT", "200")
	os.Setenv("ENABLE_AI_CACHE", "false")

	config := loadPerformanceConfigFromEnv()
	
	assert.NotNil(t, config)
	assert.Equal(t, 8, config.Workers)
	assert.Equal(t, 500, config.QueueSize)
	assert.Equal(t, 200, config.RateLimit)
	assert.False(t, config.EnableCache)
}

// TestLoadLLMConfig_Success 测试成功加载LLM配置
func TestLoadLLMConfig_Success(t *testing.T) {
	// 保存原始环境变量
	originalVars := map[string]string{
		"PRIMARY_LLM_PROVIDER":   os.Getenv("PRIMARY_LLM_PROVIDER"),
		"FALLBACK_LLM_PROVIDER":  os.Getenv("FALLBACK_LLM_PROVIDER"),
		"LOCAL_LLM_PROVIDER":     os.Getenv("LOCAL_LLM_PROVIDER"),
		"OPENAI_API_KEY":         os.Getenv("OPENAI_API_KEY"),
		"ANTHROPIC_API_KEY":      os.Getenv("ANTHROPIC_API_KEY"),
		"AI_REQUEST_TIMEOUT":     os.Getenv("AI_REQUEST_TIMEOUT"),
	}
	
	defer func() {
		for key, value := range originalVars {
			os.Setenv(key, value)
		}
	}()

	// 设置测试环境变量
	os.Setenv("PRIMARY_LLM_PROVIDER", "openai")
	os.Setenv("FALLBACK_LLM_PROVIDER", "anthropic")
	os.Setenv("LOCAL_LLM_PROVIDER", "ollama")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("AI_REQUEST_TIMEOUT", "45")

	config, err := LoadLLMConfig()
	
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, ProviderOpenAI, config.PrimaryProvider)
	assert.Equal(t, ProviderAnthropic, config.FallbackProvider)
	assert.Equal(t, ProviderOllama, config.LocalProvider)
	assert.Equal(t, 45*time.Second, config.RequestTimeout)
	assert.NotNil(t, config.PrimaryConfig)
	assert.NotNil(t, config.FallbackConfig)
	assert.NotNil(t, config.LocalConfig)
	assert.NotNil(t, config.ExtendedCostConfig)
	assert.NotNil(t, config.PerformanceConfig)
}

// TestLoadLLMConfig_PrimaryProviderFailure 测试主要提供商配置失败
func TestLoadLLMConfig_PrimaryProviderFailure(t *testing.T) {
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	originalProvider := os.Getenv("PRIMARY_LLM_PROVIDER")
	
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalAPIKey)
		os.Setenv("PRIMARY_LLM_PROVIDER", originalProvider)
	}()

	os.Setenv("PRIMARY_LLM_PROVIDER", "openai")
	os.Unsetenv("OPENAI_API_KEY") // 移除必需的API密钥

	config, err := LoadLLMConfig()
	
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "loading primary provider config")
}

// TestExtendedCostConfig 测试扩展成本配置结构
func TestExtendedCostConfig(t *testing.T) {
	config := &ExtendedCostConfig{
		MaxCostPerQueryCents: 5,
		DailyBudgetPerUser:   10.0,
		TotalDailyBudget:     500.0,
		OpenAIPricing:        getDefaultOpenAIPricing(),
		AnthropicPricing:     getDefaultAnthropicPricing(),
	}

	assert.Equal(t, 5, config.MaxCostPerQueryCents)
	assert.Equal(t, 10.0, config.DailyBudgetPerUser)
	assert.Equal(t, 500.0, config.TotalDailyBudget)
	assert.NotNil(t, config.OpenAIPricing)
	assert.NotNil(t, config.AnthropicPricing)
}

// TestOpenAIPricing 测试OpenAI定价结构
func TestOpenAIPricing(t *testing.T) {
	pricing := OpenAIPricing{
		InputPrice:  0.5,
		OutputPrice: 1.5,
	}

	assert.Equal(t, 0.5, pricing.InputPrice)
	assert.Equal(t, 1.5, pricing.OutputPrice)
}

// TestAnthropicPricing 测试Anthropic定价结构
func TestAnthropicPricing(t *testing.T) {
	pricing := AnthropicPricing{
		InputPrice:  0.25,
		OutputPrice: 1.25,
	}

	assert.Equal(t, 0.25, pricing.InputPrice)
	assert.Equal(t, 1.25, pricing.OutputPrice)
}