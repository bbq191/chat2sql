// LLM客户端工厂和路由管理测试
// 测试OpenAI、Anthropic、Ollama等多种提供商的集成

package ai

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewLLMClient 测试LLM客户端创建
func TestNewLLMClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *LLMRouterConfig
		wantErr bool
	}{
		{
			name: "valid_ollama_config",
			config: &LLMRouterConfig{
				PrimaryProvider:  ProviderOllama,
				FallbackProvider: ProviderOllama,
				LocalProvider:    ProviderOllama,
				PrimaryConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				FallbackConfig: &LLMConfig{
					Model:   "deepseek-r1:7b", 
					BaseURL: "http://localhost:11434",
				},
				LocalConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				PerformanceConfig: DefaultPerformanceConfig(),
			},
			wantErr: false,
		},
		{
			name: "minimal_config",
			config: &LLMRouterConfig{
				PrimaryProvider:  ProviderOllama,
				FallbackProvider: ProviderOllama,
				PrimaryConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				FallbackConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				PerformanceConfig: DefaultPerformanceConfig(),
			},
			wantErr: false,
		},
		{
			name: "invalid_primary_provider",
			config: &LLMRouterConfig{
				PrimaryProvider:  "invalid",
				FallbackProvider: ProviderOllama,
				PrimaryConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				FallbackConfig: &LLMConfig{
					Model:   "deepseek-r1:7b",
					BaseURL: "http://localhost:11434",
				},
				PerformanceConfig: DefaultPerformanceConfig(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop() // 使用空日志记录器进行测试
		client, err := NewLLMClient(tt.config, logger)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, client)
				assert.NotNil(t, client.GetPrimaryLLM())
				assert.NotNil(t, client.GetFallbackLLM())
				assert.Equal(t, tt.config, client.GetConfig())
				
				// 如果配置了本地模型，验证它是否被创建
				if tt.config.LocalConfig != nil {
					assert.NotNil(t, client.GetLocalLLM())
				}
			}
		})
	}
}

// TestCreateLLMProvider 测试LLM提供商创建函数
func TestCreateLLMProvider(t *testing.T) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	
	tests := []struct {
		name     string
		provider LLMProvider
		config   *LLMConfig
		wantErr  bool
	}{
		{
			name:     "ollama_provider",
			provider: ProviderOllama,
			config: &LLMConfig{
				Model:   "deepseek-r1:7b",
				BaseURL: "http://localhost:11434",
			},
			wantErr: false,
		},
		{
			name:     "openai_provider",
			provider: ProviderOpenAI,
			config: &LLMConfig{
				APIKey: "test-key",
				Model:  "gpt-3.5-turbo",
			},
			wantErr: false,
		},
		{
			name:     "anthropic_provider",
			provider: ProviderAnthropic,
			config: &LLMConfig{
				APIKey: "test-key",
				Model:  "claude-3-haiku-20240307",
			},
			wantErr: false,
		},
		{
			name:     "unsupported_provider",
			provider: "unsupported",
			config:   &LLMConfig{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := createLLMProvider(tt.provider, tt.config, httpClient)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

// TestCreateOpenAIClient 测试OpenAI客户端创建
func TestCreateOpenAIClient(t *testing.T) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	
	tests := []struct {
		name   string
		config *LLMConfig
	}{
		{
			name: "basic_config",
			config: &LLMConfig{
				APIKey: "test-api-key",
				Model:  "gpt-3.5-turbo",
			},
		},
		{
			name: "config_with_base_url",
			config: &LLMConfig{
				APIKey:  "test-api-key",
				Model:   "gpt-3.5-turbo",
				BaseURL: "https://api.custom.com/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := createOpenAIClient(tt.config, httpClient)
			assert.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestCreateAnthropicClient 测试Anthropic客户端创建
func TestCreateAnthropicClient(t *testing.T) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	
	config := &LLMConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-haiku-20240307",
	}

	client, err := createAnthropicClient(config, httpClient)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// TestCreateOllamaClient 测试Ollama客户端创建
func TestCreateOllamaClient(t *testing.T) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	
	tests := []struct {
		name   string
		config *LLMConfig
	}{
		{
			name: "basic_config",
			config: &LLMConfig{
				Model: "deepseek-r1:7b",
			},
		},
		{
			name: "config_with_base_url",
			config: &LLMConfig{
				Model:   "deepseek-r1:7b",
				BaseURL: "http://localhost:11434",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := createOllamaClient(tt.config, httpClient)
			assert.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestLLMClientGetters 测试LLM客户端的获取方法
func TestLLMClientGetters(t *testing.T) {
	config := &LLMRouterConfig{
		PrimaryProvider:  ProviderOllama,
		FallbackProvider: ProviderOllama,
		LocalProvider:    ProviderOllama,
		PrimaryConfig: &LLMConfig{
			Model:   "deepseek-r1:7b",
			BaseURL: "http://localhost:11434",
		},
		FallbackConfig: &LLMConfig{
			Model:   "deepseek-r1:7b",
			BaseURL: "http://localhost:11434",
		},
		LocalConfig: &LLMConfig{
			Model:   "deepseek-r1:7b",
			BaseURL: "http://localhost:11434",
		},
		PerformanceConfig: DefaultPerformanceConfig(),
	}

	logger := zap.NewNop() // 使用空日志记录器进行测试
	client, err := NewLLMClient(config, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// 测试获取方法
	assert.NotNil(t, client.GetPrimaryLLM())
	assert.NotNil(t, client.GetFallbackLLM())
	assert.NotNil(t, client.GetLocalLLM())
	assert.Equal(t, config, client.GetConfig())
}

// TestLLMClientClose 测试客户端关闭
func TestLLMClientClose(t *testing.T) {
	config := &LLMRouterConfig{
		PrimaryProvider:  ProviderOllama,
		FallbackProvider: ProviderOllama,
		PrimaryConfig: &LLMConfig{
			Model:   "deepseek-r1:7b",
			BaseURL: "http://localhost:11434",
		},
		FallbackConfig: &LLMConfig{
			Model:   "deepseek-r1:7b",
			BaseURL: "http://localhost:11434",
		},
		PerformanceConfig: DefaultPerformanceConfig(),
	}

	logger := zap.NewNop() // 使用空日志记录器进行测试
	client, err := NewLLMClient(config, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// 测试关闭方法
	err = client.Close()
	assert.NoError(t, err)
}

// TestGenerateContentErrorHandling 测试内容生成的错误处理逻辑
func TestGenerateContentErrorHandling(t *testing.T) {
	config := &LLMRouterConfig{
		PrimaryProvider:  ProviderOllama,
		FallbackProvider: ProviderOllama,
		PrimaryConfig: &LLMConfig{
			Model:   "non-existent-model",
			BaseURL: "http://localhost:11434",
		},
		FallbackConfig: &LLMConfig{
			Model:   "another-non-existent-model",
			BaseURL: "http://localhost:11434",
		},
		PerformanceConfig: DefaultPerformanceConfig(),
	}

	logger := zap.NewNop() // 使用空日志记录器进行测试
	client, err := NewLLMClient(config, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Hello, test message"),
	}

	// 由于模型不存在，这应该返回错误
	response, err := client.GenerateContent(ctx, messages)
	assert.Error(t, err)
	assert.Nil(t, response)
}

// TestValidateConfigurationErrorHandling 测试配置验证的错误处理
func TestValidateConfigurationErrorHandling(t *testing.T) {
	config := &LLMRouterConfig{
		PrimaryProvider:  ProviderOllama,
		FallbackProvider: ProviderOllama,
		PrimaryConfig: &LLMConfig{
			Model:   "non-existent-model",
			BaseURL: "http://localhost:11434",
		},
		FallbackConfig: &LLMConfig{
			Model:   "another-non-existent-model", 
			BaseURL: "http://localhost:11434",
		},
		PerformanceConfig: DefaultPerformanceConfig(),
	}

	logger := zap.NewNop() // 使用空日志记录器进行测试
	client, err := NewLLMClient(config, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	ctx := context.Background()
	
	// 由于模型不存在，验证应该失败
	err = client.ValidateConfiguration(ctx)
	assert.Error(t, err)
}