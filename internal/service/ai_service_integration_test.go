// AI服务集成测试 - 基于LangChainGo最佳实践
package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/config"
)

// 基于Context7研究的LangChainGo测试最佳实践
// 使用httprr模式进行HTTP录制和回放（模拟实现）

// TestAIService_IntegrationSuite 集成测试套件
func TestAIService_IntegrationSuite(t *testing.T) {
	// 跳过如果没有API密钥且没有录制数据
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	t.Run("SQL生成基础功能", TestAIService_GenerateSQL_Basic)
	t.Run("SQL生成聚合查询", TestAIService_GenerateSQL_Aggregation)
	t.Run("模型降级机制", TestAIService_GenerateSQL_Fallback)
	t.Run("并发处理能力", TestAIService_GenerateSQL_Concurrent)
	t.Run("错误处理测试", TestAIService_GenerateSQL_ErrorHandling)
	t.Run("监控指标验证", TestAIService_Metrics_Validation)
}

// TestAIService_GenerateSQL_Basic 基础SQL生成测试
func TestAIService_GenerateSQL_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	t.Parallel()

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	// 创建测试AI服务
	aiService := createTestAIService(t)
	defer aiService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name         string
		request      *SQLGenerationRequest
		expectSQL    string
		expectError  bool
		minConfidence float64
	}{
		{
			name: "简单查询",
			request: &SQLGenerationRequest{
				Query:        "查询所有用户信息",
				ConnectionID: 1,
				UserID:       1,
				Schema:       "CREATE TABLE users (id INTEGER, name VARCHAR(100), email VARCHAR(200));",
			},
			expectSQL:    "SELECT",
			minConfidence: 0.7,
		},
		{
			name: "条件查询",
			request: &SQLGenerationRequest{
				Query:        "查找年龄大于25岁的用户",
				ConnectionID: 1,
				UserID:       1,
				Schema:       "CREATE TABLE users (id INTEGER, name VARCHAR(100), age INTEGER);",
			},
			expectSQL:    "WHERE",
			minConfidence: 0.6,
		},
		{
			name: "排序查询",
			request: &SQLGenerationRequest{
				Query:        "按创建时间排序显示所有订单",
				ConnectionID: 1,
				UserID:       1,
				Schema:       "CREATE TABLE orders (id INTEGER, user_id INTEGER, created_at TIMESTAMP);",
			},
			expectSQL:    "ORDER BY",
			minConfidence: 0.6,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := aiService.GenerateSQL(ctx, tc.request)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, response)
			
			// 验证SQL内容
			assert.Contains(t, response.SQL, tc.expectSQL, "生成的SQL应包含预期内容")
			assert.NotEmpty(t, response.SQL, "SQL不应为空")
			
			// 验证置信度
			assert.GreaterOrEqual(t, response.Confidence, tc.minConfidence, "置信度应达到最小要求")
			assert.LessOrEqual(t, response.Confidence, 1.0, "置信度不应超过1.0")
			
			// 验证处理时间
			assert.Positive(t, response.ProcessingTime, "处理时间应为正数")
			assert.Less(t, response.ProcessingTime, 30*time.Second, "处理时间应在合理范围内")
			
			t.Logf("生成SQL: %s", response.SQL)
			t.Logf("置信度: %.2f", response.Confidence)
			t.Logf("处理时间: %v", response.ProcessingTime)
		})
	}
}

// TestAIService_GenerateSQL_Aggregation 聚合查询测试
func TestAIService_GenerateSQL_Aggregation(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	t.Parallel()

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	aiService := createTestAIService(t)
	defer aiService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name      string
		query     string
		expectSQL []string // 期望包含的SQL关键词
	}{
		{
			name:      "统计查询",
			query:     "统计用户总数",
			expectSQL: []string{"COUNT", "users"},
		},
		{
			name:      "平均值查询",
			query:     "计算订单平均金额",
			expectSQL: []string{"AVG", "amount"},
		},
		{
			name:      "分组统计",
			query:     "按部门统计员工数量",
			expectSQL: []string{"GROUP BY", "COUNT", "department"},
		},
		{
			name:      "最大值查询",
			query:     "查找最高销售额",
			expectSQL: []string{"MAX", "sales"},
		},
	}

	schema := `CREATE TABLE users (id INTEGER, name VARCHAR(100), department VARCHAR(50));
               CREATE TABLE orders (id INTEGER, user_id INTEGER, amount DECIMAL(10,2));
               CREATE TABLE sales (id INTEGER, amount DECIMAL(10,2), date DATE);`

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := &SQLGenerationRequest{
				Query:        tc.query,
				ConnectionID: 1,
				UserID:       1,
				Schema:       schema,
			}

			response, err := aiService.GenerateSQL(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, response)

			// 验证SQL包含期望的关键词
			for _, keyword := range tc.expectSQL {
				assert.Contains(t, response.SQL, keyword,
					"SQL应包含关键词: %s", keyword)
			}

			t.Logf("查询: %s", tc.query)
			t.Logf("生成SQL: %s", response.SQL)
		})
	}
}

// TestAIService_GenerateSQL_Fallback 模型降级机制测试
func TestAIService_GenerateSQL_Fallback(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	// 不使用t.Parallel()，因为我们要模拟主模型失败

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	logger := zaptest.NewLogger(t)

	// 创建带有无效主模型的配置
	aiConfig := &config.AIConfig{
		Primary: config.ModelConfig{
			Provider:    "openai",
			ModelName:   "invalid-model",
			APIKey:      "invalid-key",
			Temperature: 0.1,
			MaxTokens:   2048,
		},
		Fallback: config.ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini",
			APIKey:      getTestAPIKey("OPENAI_API_KEY"),
			Temperature: 0.0,
			MaxTokens:   1024,
		},
	}

	// skipIfNoCredentialsOrRecording 已经检查了环境变量

	aiService, err := NewAIService(aiConfig, logger)
	require.NoError(t, err)
	defer aiService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	request := &SQLGenerationRequest{
		Query:        "查询所有用户",
		ConnectionID: 1,
		UserID:       1,
		Schema:       "CREATE TABLE users (id INTEGER, name VARCHAR(100));",
	}

	// 这应该失败在主模型，但成功在备用模型
	response, err := aiService.GenerateSQL(ctx, request)
	require.NoError(t, err, "备用模型应该成功")
	require.NotNil(t, response)
	assert.NotEmpty(t, response.SQL)

	t.Logf("备用模型成功生成SQL: %s", response.SQL)
}

// TestAIService_GenerateSQL_Concurrent 并发处理测试
func TestAIService_GenerateSQL_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	// 不使用t.Parallel()，避免与其他并发测试冲突

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	aiService := createTestAIService(t)
	defer aiService.Close()

	const concurrency = 5
	const requests = 10

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 创建请求通道
	requestChan := make(chan *SQLGenerationRequest, requests)
	responseChan := make(chan *SQLGenerationResponse, requests)
	errorChan := make(chan error, requests)

	// 填充请求
	for i := 0; i < requests; i++ {
		requestChan <- &SQLGenerationRequest{
			Query:        "查询用户信息",
			ConnectionID: int64(i + 1),
			UserID:       int64(i + 1),
			Schema:       "CREATE TABLE users (id INTEGER, name VARCHAR(100));",
		}
	}
	close(requestChan)

	// 启动worker goroutines
	for i := 0; i < concurrency; i++ {
		go func() {
			for req := range requestChan {
				response, err := aiService.GenerateSQL(ctx, req)
				if err != nil {
					errorChan <- err
				} else {
					responseChan <- response
				}
			}
		}()
	}

	// 收集结果
	var responses []*SQLGenerationResponse
	var errors []error

	for i := 0; i < requests; i++ {
		select {
		case response := <-responseChan:
			responses = append(responses, response)
		case err := <-errorChan:
			errors = append(errors, err)
		case <-ctx.Done():
			t.Fatal("测试超时")
		}
	}

	// 验证结果
	t.Logf("成功响应: %d, 错误: %d", len(responses), len(errors))
	assert.Greater(t, len(responses), requests/2, "至少一半的请求应该成功")

	for _, response := range responses {
		assert.NotEmpty(t, response.SQL, "生成的SQL不应为空")
		assert.Positive(t, response.Confidence, "置信度应为正数")
	}
}

// TestAIService_GenerateSQL_ErrorHandling 错误处理测试
func TestAIService_GenerateSQL_ErrorHandling(t *testing.T) {
	t.Parallel()

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	logger := zaptest.NewLogger(t)

	testCases := []struct {
		name        string
		config      *config.AIConfig
		request     *SQLGenerationRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "无效API密钥",
			config: &config.AIConfig{
				Primary: config.ModelConfig{
					Provider:    "openai",
					ModelName:   "gpt-4o-mini",
					APIKey:      "invalid-key",
					Temperature: 0.1,
					MaxTokens:   1024,
				},
				Fallback: config.ModelConfig{
					Provider:    "openai",
					ModelName:   "gpt-4o-mini",
					APIKey:      "invalid-key-fallback",
					Temperature: 0.1,
					MaxTokens:   1024,
				},
			},
			request: &SQLGenerationRequest{
				Query:        "测试查询",
				ConnectionID: 1,
				UserID:       1,
				Schema:       "CREATE TABLE test (id INTEGER);",
			},
			expectError: true,
			errorMsg:    "LLM调用失败",
		},
		{
			name: "空查询请求",
			config: createTestConfig(t),
			request: &SQLGenerationRequest{
				Query:        "",
				ConnectionID: 1,
				UserID:       1,
				Schema:       "CREATE TABLE test (id INTEGER);",
			},
			expectError: false, // 应该由prompt处理空查询
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aiService, err := NewAIService(tc.config, logger)
			require.NoError(t, err)
			defer aiService.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			response, err := aiService.GenerateSQL(ctx, tc.request)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

// TestAIService_Metrics_Validation 监控指标验证测试
func TestAIService_Metrics_Validation(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	t.Parallel()

	// 检查环境变量
	skipIfNoCredentialsOrRecording(t, "OPENAI_API_KEY", "ANTHROPIC_API_KEY")

	aiService := createTestAIService(t)
	defer aiService.Close()

	// 验证指标已初始化
	require.NotNil(t, aiService.metrics)
	require.NotNil(t, aiService.metrics.RequestsTotal)
	require.NotNil(t, aiService.metrics.RequestDuration)
	require.NotNil(t, aiService.metrics.TokensUsed)
	require.NotNil(t, aiService.metrics.ErrorsTotal)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &SQLGenerationRequest{
		Query:        "测试指标查询",
		ConnectionID: 1,
		UserID:       1,
		Schema:       "CREATE TABLE test (id INTEGER);",
	}

	// 执行请求以生成指标数据
	_, err := aiService.GenerateSQL(ctx, request)
	require.NoError(t, err)

	// 注意：在实际环境中，这里我们会检查Prometheus指标
	// 由于这是集成测试，我们主要验证指标对象存在且方法可调用
	t.Log("监控指标验证完成")
}

// 辅助函数：创建测试AI服务
func createTestAIService(t *testing.T) *AIService {
	t.Helper()

	config := createTestConfig(t)
	logger := zaptest.NewLogger(t)

	aiService, err := NewAIService(config, logger)
	require.NoError(t, err)

	return aiService
}

// 辅助函数：创建测试配置
func createTestConfig(t *testing.T) *config.AIConfig {
	t.Helper()

	apiKey := getTestAPIKey("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("需要OPENAI_API_KEY环境变量进行集成测试")
	}

	return &config.AIConfig{
		Primary: config.ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini", // 使用性价比高的模型
			APIKey:      apiKey,
			Temperature: 0.1,
			MaxTokens:   2048,
		},
		Fallback: config.ModelConfig{
			Provider:    "openai",
			ModelName:   "gpt-4o-mini",
			APIKey:      apiKey,
			Temperature: 0.0,
			MaxTokens:   1024,
		},
	}
}

// 辅助函数：获取测试API密钥
func getTestAPIKey(envVar string) string {
	return os.Getenv(envVar)
}

// 辅助函数：跳过测试如果没有凭证或录制
// 模拟httprr的SkipIfNoCredentialsOrRecording功能
func skipIfNoCredentialsOrRecording(t *testing.T, envVars ...string) {
	t.Helper()

	hasCredentials := false
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			hasCredentials = true
			break
		}
	}

	if !hasCredentials {
		// 在真实环境中，这里会检查是否存在httprr录制文件
		// testdata/TestName.httprr*
		t.Skipf("跳过测试：需要环境变量 %v 或录制文件", envVars)
	}
}

// MockLLMForUnitTests Mock LLM实现用于单元测试
type MockLLMForUnitTests struct {
	responses     []string
	currentIndex  int
	shouldError   bool
	errorResponse error
}

func (m *MockLLMForUnitTests) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if m.shouldError {
		return nil, m.errorResponse
	}

	if m.currentIndex >= len(m.responses) {
		return nil, fmt.Errorf("no more responses available")
	}

	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: m.responses[m.currentIndex]},
		},
	}
	m.currentIndex++
	return response, nil
}

// TestAIService_WithMockLLM 使用Mock LLM的单元测试示例
func TestAIService_WithMockLLM(t *testing.T) {
	t.Parallel()

	// 这个测试展示了如何在单元测试中使用Mock LLM
	// 在实际实现中，需要修改AIService以支持依赖注入
	
	mockResponses := []string{
		"SELECT * FROM users;",
		"SELECT COUNT(*) FROM orders;",
		"SELECT name FROM products WHERE price > 100;",
	}

	mockLLM := &MockLLMForUnitTests{
		responses:    mockResponses,
		shouldError:  false,
	}

	// 注意：这需要AIService支持依赖注入才能工作
	// 当前的AIService直接创建LLM客户端
	_ = mockLLM // 避免未使用变量警告

	t.Log("Mock LLM单元测试模式演示完成")
}