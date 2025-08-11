package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestDefaultAIConfig(t *testing.T) {
	aiConfig := DefaultAIConfig()
	
	assert.Equal(t, "openai", aiConfig.Primary.Provider)
	assert.Equal(t, "gpt-4o-mini", aiConfig.Primary.ModelName)
	assert.Equal(t, "anthropic", aiConfig.Fallback.Provider)
	assert.Equal(t, "claude-3-haiku-20240307", aiConfig.Fallback.ModelName)
	assert.Equal(t, 10, aiConfig.MaxConcurrency)
	assert.Equal(t, 30*time.Second, aiConfig.Timeout)
	assert.Equal(t, 100.0, aiConfig.Budget.DailyLimit)
	assert.Equal(t, 10.0, aiConfig.Budget.UserLimit)
	assert.Equal(t, 0.8, aiConfig.Budget.AlertThreshold)
}

func TestAIConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *AIConfig
		expectErr bool
	}{
		{
			name:      "valid default config",
			config:    createValidTestAIConfig(),
			expectErr: false,
		},
		{
			name: "invalid temperature",
			config: func() *AIConfig {
				c := createValidTestAIConfig()
				c.Primary.Temperature = 3.0 // 无效值
				return c
			}(),
			expectErr: true,
		},
		{
			name: "invalid max tokens",
			config: func() *AIConfig {
				c := createValidTestAIConfig()
				c.Primary.MaxTokens = -1 // 无效值
				return c
			}(),
			expectErr: true,
		},
		{
			name: "empty provider",
			config: func() *AIConfig {
				c := createValidTestAIConfig()
				c.Primary.Provider = ""
				return c
			}(),
			expectErr: true,
		},
		{
			name: "invalid budget threshold",
			config: func() *AIConfig {
				c := createValidTestAIConfig()
				c.Budget.AlertThreshold = 1.5 // 超过1.0
				return c
			}(),
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadAIConfigFromEnv(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("PRIMARY_MODEL_NAME", "gpt-4")
	os.Setenv("FALLBACK_MODEL_NAME", "claude-3-sonnet")
	os.Setenv("LLM_TIMEOUT", "45s")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("PRIMARY_MODEL_NAME")
		os.Unsetenv("FALLBACK_MODEL_NAME")
		os.Unsetenv("LLM_TIMEOUT")
	}()
	
	aiConfig, err := LoadAIConfigFromEnv()
	require.NoError(t, err)
	
	assert.Equal(t, "test-openai-key", aiConfig.Primary.APIKey)
	assert.Equal(t, "test-anthropic-key", aiConfig.Fallback.APIKey)
	assert.Equal(t, "gpt-4", aiConfig.Primary.ModelName)
	assert.Equal(t, "claude-3-sonnet", aiConfig.Fallback.ModelName)
	assert.Equal(t, 45*time.Second, aiConfig.Timeout)
	assert.Equal(t, 45*time.Second, aiConfig.Primary.Timeout)
	assert.Equal(t, 45*time.Second, aiConfig.Fallback.Timeout)
}

func TestLoadAIConfigFromEnvMissingKeys(t *testing.T) {
	// 清除环境变量
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	
	_, err := LoadAIConfigFromEnv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestLoadAIConfigFromEnvMissingAnthropicKey(t *testing.T) {
	// 设置OpenAI但不设置Anthropic
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("OPENAI_API_KEY")
	
	_, err := LoadAIConfigFromEnv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestGetModelCosts(t *testing.T) {
	costs := GetModelCosts()
	
	// 验证OpenAI模型成本
	assert.Contains(t, costs, "openai")
	assert.Contains(t, costs["openai"], "gpt-4o-mini")
	assert.Equal(t, 0.0015, costs["openai"]["gpt-4o-mini"])
	
	// 验证Anthropic模型成本
	assert.Contains(t, costs, "anthropic")
	assert.Contains(t, costs["anthropic"], "claude-3-haiku-20240307")
	assert.Equal(t, 0.0008, costs["anthropic"]["claude-3-haiku-20240307"])
}

func TestEstimateTokenCost(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		tokens   int
		expected float64
	}{
		{"openai", "gpt-4o-mini", 1000, 0.0015},
		{"anthropic", "claude-3-haiku-20240307", 1000, 0.0008},
		{"unknown", "unknown-model", 1000, 0.002}, // 默认成本
		{"openai", "unknown-model", 1000, 0.002}, // provider存在但model不存在
	}
	
	for _, tt := range tests {
		t.Run(tt.provider+"_"+tt.model, func(t *testing.T) {
			cost := EstimateTokenCost(tt.provider, tt.model, tt.tokens)
			assert.Equal(t, tt.expected, cost)
		})
	}
}

func TestAIConfigLogConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	aiConfig := DefaultAIConfig()
	
	// 应该不会panic
	assert.NotPanics(t, func() {
		aiConfig.LogConfig(logger)
	})
}

func TestModelConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    ModelConfig
		expectErr bool
		errorMsg  string
	}{
		{
			name: "valid config",
			config: ModelConfig{
				Provider:    "openai",
				ModelName:   "gpt-4o-mini",
				APIKey:      "test-key",
				Temperature: 0.7,
				MaxTokens:   2048,
				TopP:        0.9,
				Timeout:     30 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "empty provider",
			config: ModelConfig{
				ModelName:   "gpt-4o-mini",
				APIKey:      "test-key",
				Temperature: 0.7,
				MaxTokens:   2048,
				TopP:        0.9,
				Timeout:     30 * time.Second,
			},
			expectErr: true,
			errorMsg:  "provider cannot be empty",
		},
		{
			name: "temperature too high",
			config: ModelConfig{
				Provider:    "openai",
				ModelName:   "gpt-4o-mini",
				APIKey:      "test-key",
				Temperature: 2.5, // 无效值
				MaxTokens:   2048,
				TopP:        0.9,
				Timeout:     30 * time.Second,
			},
			expectErr: true,
			errorMsg:  "temperature must be between 0 and 2",
		},
		{
			name: "top_p too high",
			config: ModelConfig{
				Provider:    "openai",
				ModelName:   "gpt-4o-mini",
				APIKey:      "test-key",
				Temperature: 0.7,
				MaxTokens:   2048,
				TopP:        1.5, // 无效值
				Timeout:     30 * time.Second,
			},
			expectErr: true,
			errorMsg:  "top_p must be between 0 and 1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// createValidTestAIConfig 创建有效的测试AI配置
func createValidTestAIConfig() *AIConfig {
	return &AIConfig{
		Primary: ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini",
			APIKey:      "test-key",
			Temperature: 0.1,
			MaxTokens:   2048,
			TopP:        0.9,
			Timeout:     30 * time.Second,
		},
		Fallback: ModelConfig{
			Provider:    "anthropic",
			ModelName:   "claude-3-haiku-20240307",
			APIKey:      "test-key",
			Temperature: 0.0,
			MaxTokens:   1024,
			TopP:        0.9,
			Timeout:     30 * time.Second,
		},
		MaxConcurrency: 10,
		Timeout:        30 * time.Second,
		Budget: BudgetConfig{
			DailyLimit:     100.0,
			UserLimit:      10.0,
			AlertThreshold: 0.8,
		},
	}
}