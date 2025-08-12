// LLM客户端工厂和路由管理
// 支持OpenAI、Anthropic、Ollama等多种提供商
// 基于LangChainGo的统一接口设计

package ai

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"
)

// LLMClient 统一的LLM客户端接口
type LLMClient struct {
	primary   llms.Model
	fallback  llms.Model
	local     llms.Model
	config    *LLMRouterConfig
	httpClient *http.Client
	logger    *zap.Logger
}

// NewLLMClient 创建新的LLM客户端
func NewLLMClient(config *LLMRouterConfig, logger *zap.Logger) (*LLMClient, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	client := &LLMClient{
		config: config,
		logger: logger,
	}
	
	// 创建优化的HTTP客户端
	httpClient := NewHTTPClient(config.PerformanceConfig)
	client.httpClient = httpClient.Client()
	
	var err error
	
	// 初始化主要模型
	client.primary, err = createLLMProvider(config.PrimaryProvider, config.PrimaryConfig, client.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary LLM provider %s: %w", config.PrimaryProvider, err)
	}
	
	// 初始化备用模型
	client.fallback, err = createLLMProvider(config.FallbackProvider, config.FallbackConfig, client.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback LLM provider %s: %w", config.FallbackProvider, err)
	}
	
	// 初始化本地模型（可选）
	if config.LocalConfig != nil {
		client.local, err = createLLMProvider(config.LocalProvider, config.LocalConfig, client.httpClient)
		if err != nil {
			// 本地模型初始化失败不影响系统启动
			client.logger.Warn("failed to create local LLM provider",
				zap.String("provider", string(config.LocalProvider)),
				zap.Error(err))
		}
	}
	
	return client, nil
}

// createLLMProvider 创建特定提供商的LLM实例
func createLLMProvider(provider LLMProvider, config *LLMConfig, httpClient *http.Client) (llms.Model, error) {
	switch provider {
	case ProviderOpenAI:
		return createOpenAIClient(config, httpClient)
	case ProviderAnthropic:
		return createAnthropicClient(config, httpClient)
	case ProviderOllama:
		return createOllamaClient(config, httpClient)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

// createOpenAIClient 创建OpenAI客户端
func createOpenAIClient(config *LLMConfig, httpClient *http.Client) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithToken(config.APIKey),
		openai.WithModel(config.Model),
		openai.WithHTTPClient(httpClient),
	}
	
	// 可选配置
	if config.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(config.BaseURL))
	}
	
	return openai.New(opts...)
}

// createAnthropicClient 创建Anthropic客户端
func createAnthropicClient(config *LLMConfig, httpClient *http.Client) (llms.Model, error) {
	opts := []anthropic.Option{
		anthropic.WithToken(config.APIKey),
		anthropic.WithModel(config.Model),
		anthropic.WithHTTPClient(httpClient),
	}
	
	return anthropic.New(opts...)
}

// createOllamaClient 创建Ollama客户端
func createOllamaClient(config *LLMConfig, httpClient *http.Client) (llms.Model, error) {
	opts := []ollama.Option{
		ollama.WithModel(config.Model),
		ollama.WithHTTPClient(httpClient),
	}
	
	if config.BaseURL != "" {
		opts = append(opts, ollama.WithServerURL(config.BaseURL))
	}
	
	return ollama.New(opts...)
}


// GetPrimaryLLM 获取主要LLM实例
func (c *LLMClient) GetPrimaryLLM() llms.Model {
	return c.primary
}

// GetFallbackLLM 获取备用LLM实例
func (c *LLMClient) GetFallbackLLM() llms.Model {
	return c.fallback
}

// GetLocalLLM 获取本地LLM实例
func (c *LLMClient) GetLocalLLM() llms.Model {
	return c.local
}

// GetConfig 获取配置信息
func (c *LLMClient) GetConfig() *LLMRouterConfig {
	return c.config
}

// GenerateContent 生成内容的统一入口，支持自动降级
func (c *LLMClient) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// 首先尝试主要模型
	response, err := c.primary.GenerateContent(ctx, messages, options...)
	if err == nil {
		return response, nil
	}
	
	// 如果主要模型失败，尝试备用模型
	c.logger.Warn("Primary LLM failed, trying fallback", zap.Error(err))
	response, fallbackErr := c.fallback.GenerateContent(ctx, messages, options...)
	if fallbackErr == nil {
		return response, nil
	}
	
	// 如果本地模型可用，最后尝试本地模型
	if c.local != nil {
		c.logger.Warn("Fallback LLM also failed, trying local LLM", zap.Error(fallbackErr))
		response, localErr := c.local.GenerateContent(ctx, messages, options...)
		if localErr == nil {
			return response, nil
		}
		c.logger.Error("Local LLM also failed", zap.Error(localErr))
	}
	
	// 所有模型都失败，返回原始错误
	return nil, fmt.Errorf("all LLM providers failed - primary: %v, fallback: %v", err, fallbackErr)
}

// ValidateConfiguration 验证配置是否正确
func (c *LLMClient) ValidateConfiguration(ctx context.Context) error {
	// 测试主要模型
	if err := c.testLLMProvider(ctx, c.primary, string(c.config.PrimaryProvider)); err != nil {
		return fmt.Errorf("primary LLM validation failed: %w", err)
	}
	
	// 测试备用模型
	if err := c.testLLMProvider(ctx, c.fallback, string(c.config.FallbackProvider)); err != nil {
		return fmt.Errorf("fallback LLM validation failed: %w", err)
	}
	
	// 测试本地模型（如果配置了）
	if c.local != nil {
		if err := c.testLLMProvider(ctx, c.local, string(c.config.LocalProvider)); err != nil {
			c.logger.Warn("local LLM validation failed", zap.Error(err))
		}
	}
	
	return nil
}

// testLLMProvider 测试特定LLM提供商
func (c *LLMClient) testLLMProvider(ctx context.Context, provider llms.Model, name string) error {
	testMessages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Hello, this is a test message. Please respond with 'OK'."),
	}
	
	response, err := provider.GenerateContent(ctx, testMessages)
	if err != nil {
		return fmt.Errorf("%s provider test failed: %w", name, err)
	}
	
	if len(response.Choices) == 0 || response.Choices[0].Content == "" {
		return fmt.Errorf("%s provider test failed: empty response", name)
	}
	
	responsePreview := response.Choices[0].Content
	if len(responsePreview) > 50 {
		responsePreview = responsePreview[:50]
	}
	c.logger.Info("LLM provider test passed",
		zap.String("provider", name),
		zap.String("response", responsePreview))
	return nil
}

// Close 关闭客户端并清理资源
func (c *LLMClient) Close() error {
	// 目前LangChainGo的模型实例没有Close方法
	// 未来版本可能会添加，这里预留接口
	return nil
}