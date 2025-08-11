package service

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	
	"chat2sql-go/internal/config"
)

func TestAIService_DefaultConfig(t *testing.T) {
	aiConfig := config.DefaultAIConfig()
	
	assert.Equal(t, "openai", aiConfig.Primary.Provider)
	assert.Equal(t, "gpt-4o-mini", aiConfig.Primary.ModelName)
	assert.Equal(t, "anthropic", aiConfig.Fallback.Provider)
	assert.Equal(t, "claude-3-haiku-20240307", aiConfig.Fallback.ModelName)
	assert.Equal(t, 10, aiConfig.MaxConcurrency)
	assert.Equal(t, 30*time.Second, aiConfig.Timeout)
}

func TestAIService_ConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.AIConfig
		expectErr bool
	}{
		{
			name:      "valid default config",
			config:    createValidTestConfig(),
			expectErr: false,
		},
		{
			name: "invalid temperature",
			config: func() *config.AIConfig {
				c := createValidTestConfig()
				c.Primary.Temperature = 3.0 // 无效值
				return c
			}(),
			expectErr: true,
		},
		{
			name: "invalid max tokens",
			config: func() *config.AIConfig {
				c := createValidTestConfig()
				c.Primary.MaxTokens = -1 // 无效值
				return c
			}(),
			expectErr: true,
		},
		{
			name: "empty provider",
			config: func() *config.AIConfig {
				c := createValidTestConfig()
				c.Primary.Provider = ""
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

func TestAIService_LoadConfigFromEnv(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("PRIMARY_MODEL_NAME", "gpt-4")
	os.Setenv("FALLBACK_MODEL_NAME", "claude-3-sonnet")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("PRIMARY_MODEL_NAME")
		os.Unsetenv("FALLBACK_MODEL_NAME")
	}()
	
	aiConfig, err := config.LoadAIConfigFromEnv()
	require.NoError(t, err)
	
	assert.Equal(t, "test-openai-key", aiConfig.Primary.APIKey)
	assert.Equal(t, "test-anthropic-key", aiConfig.Fallback.APIKey)
	assert.Equal(t, "gpt-4", aiConfig.Primary.ModelName)
	assert.Equal(t, "claude-3-sonnet", aiConfig.Fallback.ModelName)
}

func TestAIService_LoadConfigFromEnvMissingKeys(t *testing.T) {
	// 清除环境变量
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	
	_, err := config.LoadAIConfigFromEnv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestAIService_EstimateTokenCost(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		tokens   int
		expected float64
	}{
		{"openai", "gpt-4o-mini", 1000, 0.0015},
		{"anthropic", "claude-3-haiku-20240307", 1000, 0.0008},
		{"unknown", "unknown-model", 1000, 0.002}, // 默认成本
	}
	
	for _, tt := range tests {
		t.Run(tt.provider+"_"+tt.model, func(t *testing.T) {
			cost := config.EstimateTokenCost(tt.provider, tt.model, tt.tokens)
			assert.Equal(t, tt.expected, cost)
		})
	}
}

// 由于需要真实API密钥，这个测试跳过，除非设置了环境变量
func TestAIService_ServiceCreation(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" || os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping AI service test - API keys not provided")
	}
	
	aiConfig, err := config.LoadAIConfigFromEnv()
	require.NoError(t, err)
	
	logger := zaptest.NewLogger(t)
	
	service, err := NewAIService(aiConfig, logger)
	require.NoError(t, err)
	require.NotNil(t, service)
	
	// 验证服务组件
	assert.NotNil(t, service.primaryClient)
	assert.NotNil(t, service.fallbackClient)
	assert.NotNil(t, service.config)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.metrics)
	assert.NotNil(t, service.logger)
	
	// 清理
	err = service.Close()
	assert.NoError(t, err)
}

func TestAIService_BuildPrompt(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" || os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping prompt test - API keys not provided")
	}
	
	aiConfig, err := config.LoadAIConfigFromEnv()
	require.NoError(t, err)
	
	logger := zaptest.NewLogger(t)
	service, err := NewAIService(aiConfig, logger)
	require.NoError(t, err)
	
	req := &SQLGenerationRequest{
		Query:        "查询所有用户信息",
		ConnectionID: 1,
		UserID:       1,
		Schema:       "CREATE TABLE users (id INT, name VARCHAR(100), email VARCHAR(100));",
	}
	
	prompt, err := service.buildPrompt(req)
	require.NoError(t, err)
	
	assert.Contains(t, prompt, "查询所有用户信息")
	assert.Contains(t, prompt, "CREATE TABLE users")
	assert.Contains(t, prompt, "SELECT")
	assert.Contains(t, prompt, "PostgreSQL")
}

// createValidTestConfig 创建有效的测试配置
func createValidTestConfig() *config.AIConfig {
	return &config.AIConfig{
		Primary: config.ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini",
			APIKey:      "test-key",
			Temperature: 0.1,
			MaxTokens:   2048,
			TopP:        0.9,
			Timeout:     30 * time.Second,
		},
		Fallback: config.ModelConfig{
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
		Budget: config.BudgetConfig{
			DailyLimit:     100.0,
			UserLimit:      10.0,
			AlertThreshold: 0.8,
		},
	}
}