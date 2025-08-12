// Package ai 查询处理器单元测试
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLLMClient 模拟LLM客户端
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	args := m.Called(ctx, messages, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llms.ContentResponse), args.Error(1)
}

// 实现llms.Model接口的其他方法
func (m *MockLLMClient) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	args := m.Called(ctx, prompt, options)
	return args.String(0), args.Error(1)
}

// TestNewQueryProcessor 测试查询处理器初始化
func TestNewQueryProcessor(t *testing.T) {
	tests := []struct {
		name           string
		config         *ProcessorConfig
		contextManager *ContextManager
		wantError      bool
	}{
		{
			name: "valid_config",
			config: &ProcessorConfig{
				PrimaryModel: ModelConfig{
					Provider: "openai",
					Model:    "gpt-4o-mini",
					APIKey:   "test-key",
				},
				FallbackModel: ModelConfig{
					Provider: "openai", 
					Model:    "gpt-3.5-turbo",
					APIKey:   "test-key",
				},
				EnableSQLValidation:  true,
				EnableIntentAnalysis: true,
				EnableCostTracking:   true,
				MaxRetries:           3,
				RequestTimeout:       30 * time.Second,
			},
			contextManager: NewContextManager(nil),
			wantError:      false, // 配置有效，应该能够创建
		},
		{
			name: "unsupported_provider",
			config: &ProcessorConfig{
				PrimaryModel: ModelConfig{
					Provider: "unsupported",
					Model:    "test-model",
					APIKey:   "test-key",
				},
				FallbackModel: ModelConfig{
					Provider: "openai", 
					Model:    "gpt-3.5-turbo",
					APIKey:   "test-key",
				},
			},
			contextManager: NewContextManager(nil),
			wantError:      true, // 不支持的提供商应该出错
		},
		{
			name:           "nil_config",
			config:         nil,
			contextManager: NewContextManager(nil),
			wantError:      true,
		},
		{
			name: "nil_context_manager",
			config: &ProcessorConfig{
				PrimaryModel: ModelConfig{
					Provider: "openai",
					Model:    "gpt-4o-mini",
					APIKey:   "test-key",
				},
			},
			contextManager: nil,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 由于实际创建LLM客户端需要API密钥，这里我们只测试参数验证
			_, err := NewQueryProcessor(tt.config, tt.contextManager)
			
			if tt.wantError {
				assert.Error(t, err)
				// 对于预期错误的情况，可以检查错误信息
				if err != nil {
					// 检查是否是配置相关错误或LLM客户端创建错误
					assert.True(t, 
						strings.Contains(err.Error(), "创建") || 
						strings.Contains(err.Error(), "配置") ||
						strings.Contains(err.Error(), "不能为空"),
						"Expected configuration or LLM client creation error, got: %v", err,
					)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestQueryProcessor_ValidateRequest 测试请求验证
func TestQueryProcessor_ValidateRequest(t *testing.T) {
	qp := &QueryProcessor{}

	tests := []struct {
		name      string
		request   *ChatRequest
		wantError bool
	}{
		{
			name: "valid_request",
			request: &ChatRequest{
				Query:        "查询用户信息",
				ConnectionID: 1,
				UserID:       1001,
			},
			wantError: false,
		},
		{
			name: "empty_query",
			request: &ChatRequest{
				Query:        "",
				ConnectionID: 1,
				UserID:       1001,
			},
			wantError: true,
		},
		{
			name: "zero_connection_id",
			request: &ChatRequest{
				Query:        "查询用户信息",
				ConnectionID: 0,
				UserID:       1001,
			},
			wantError: true,
		},
		{
			name: "zero_user_id",
			request: &ChatRequest{
				Query:        "查询用户信息",
				ConnectionID: 1,
				UserID:       0,
			},
			wantError: true,
		},
		{
			name: "query_too_long",
			request: &ChatRequest{
				Query:        strings.Repeat("a", 1001),
				ConnectionID: 1,
				UserID:       1001,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := qp.validateRequest(tt.request)
			
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestQueryProcessor_MapIntentToTemplateType 测试意图映射
func TestQueryProcessor_MapIntentToTemplateType(t *testing.T) {
	qp := &QueryProcessor{}

	tests := []struct {
		name         string
		intent       QueryIntent
		expectedType string
	}{
		{
			name:         "aggregation_intent",
			intent:       IntentAggregation,
			expectedType: "aggregation",
		},
		{
			name:         "join_intent",
			intent:       IntentJoinQuery,
			expectedType: "join",
		},
		{
			name:         "timeseries_intent",
			intent:       IntentTimeSeriesAnalysis,
			expectedType: "timeseries",
		},
		{
			name:         "unknown_intent",
			intent:       IntentUnknown,
			expectedType: "base",
		},
		{
			name:         "data_query_intent",
			intent:       IntentDataQuery,
			expectedType: "base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateType := qp.mapIntentToTemplateType(tt.intent)
			assert.Equal(t, tt.expectedType, templateType)
		})
	}
}

// TestQueryProcessor_ExtractSQLFromText 测试SQL提取
func TestQueryProcessor_ExtractSQLFromText(t *testing.T) {
	qp := &QueryProcessor{}

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name: "simple_select",
			text: "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name: "select_with_quotes",
			text: "```sql\nSELECT * FROM users\n```",
			expected: "SELECT * FROM users",
		},
		{
			name: "json_response",
			text: `{
				"sql": "SELECT id, name FROM users WHERE status = 'active'",
				"confidence": 0.95
			}`,
			expected: "",
		},
		{
			name: "multiline_select",
			text: `SELECT u.id, u.name, p.title
FROM users u
JOIN posts p ON u.id = p.user_id
WHERE u.status = 'active'`,
			expected: "SELECT u.id, u.name, p.title",
		},
		{
			name: "with_cte",
			text: "WITH active_users AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM active_users",
			expected: "WITH active_users AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM active_users",
		},
		{
			name: "empty_text",
			text: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qp.extractSQLFromText(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestQueryProcessor_BuildStructuredPrompt 测试结构化提示词构建
func TestQueryProcessor_BuildStructuredPrompt(t *testing.T) {
	qp := &QueryProcessor{}

	basePrompt := "根据用户查询生成SQL语句"
	structuredPrompt := qp.buildStructuredPrompt(basePrompt)

	// 验证包含基础提示词
	assert.Contains(t, structuredPrompt, basePrompt)
	
	// 验证包含JSON格式要求
	assert.Contains(t, structuredPrompt, "JSON格式")
	assert.Contains(t, structuredPrompt, "sql")
	assert.Contains(t, structuredPrompt, "confidence")
	assert.Contains(t, structuredPrompt, "query_type")
}

// TestQueryProcessor_IsRetryableError 测试可重试错误判断
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expected  bool
	}{
		{
			name:     "nil_error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout_error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "connection_error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "network_error",
			err:      errors.New("network unreachable"),
			expected: true,
		},
		{
			name:     "temporary_error",
			err:      errors.New("temporary failure"),
			expected: true,
		},
		{
			name:     "unavailable_error",
			err:      errors.New("service unavailable"),
			expected: true,
		},
		{
			name:     "other_error",
			err:      errors.New("invalid request"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestQueryProcessor_CallLLMStructured 测试结构化LLM调用
func TestQueryProcessor_CallLLMStructured(t *testing.T) {
	qp := &QueryProcessor{}

	tests := []struct {
		name           string
		mockResponse   *llms.ContentResponse
		mockError      error
		expectedSQL    string
		expectedError  bool
	}{
		{
			name: "valid_json_response",
			mockResponse: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						Content: `{
							"sql": "SELECT * FROM users",
							"confidence": 0.95,
							"query_type": "base",
							"explanation": "简单查询所有用户",
							"table_names": ["users"],
							"warnings": [],
							"metadata": {"complexity": "simple"}
						}`,
					},
				},
			},
			mockError:   nil,
			expectedSQL: "SELECT * FROM users",
			expectedError: false,
		},
		{
			name: "invalid_json_with_sql",
			mockResponse: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						Content: `这是一个查询语句：
						SELECT id, name FROM users WHERE status = 'active'
						这个查询会返回所有活跃用户`,
					},
				},
			},
			mockError:   nil,
			expectedSQL: "SELECT id, name FROM users WHERE status = 'active'",
			expectedError: false,
		},
		{
			name: "empty_response",
			mockResponse: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{},
			},
			mockError:     nil,
			expectedError: true,
		},
		{
			name:          "llm_error",
			mockResponse:  nil,
			mockError:     errors.New("LLM调用失败"),
			expectedError: true,
		},
		{
			name: "empty_sql_in_json",
			mockResponse: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						Content: `{
							"sql": "",
							"confidence": 0.1,
							"query_type": "unknown"
						}`,
					},
				},
			},
			mockError:     nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟客户端
			mockClient := new(MockLLMClient)
			mockClient.On("GenerateContent", 
				mock.Anything, 
				mock.Anything, 
				mock.Anything).Return(tt.mockResponse, tt.mockError)

			// 调用方法
			result, tokens, err := qp.callLLMStructured(
				context.Background(),
				mockClient,
				"test prompt",
				ModelConfig{Temperature: 0.1},
			)

			// 验证结果
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSQL, result.SQL)
				assert.NotZero(t, tokens)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestQueryProcessor_Integration 集成测试
func TestQueryProcessor_Integration(t *testing.T) {
	// 创建上下文管理器
	contextManager := NewContextManager(&ContextConfig{
		MaxHistorySize: 10,
		CacheTTL:       time.Hour,
		CleanupInterval: 0, // 禁用后台清理
	})

	// 添加测试数据库结构
	testSchema := &SchemaInfo{
		ConnectionID: 1,
		DatabaseName: "test_db",
		Tables: map[string]*Table{
			"users": {
				Name: "users",
				Columns: map[string]*Column{
					"id":   {Name: "id", DataType: "INTEGER"},
					"name": {Name: "name", DataType: "VARCHAR(100)"},
				},
			},
		},
	}
	contextManager.CacheSchema(1, testSchema)

	// 创建查询处理器配置
	config := &ProcessorConfig{
		PrimaryModel: ModelConfig{
			Provider:    "mock",
			Model:      "test-model",
			Temperature: 0.1,
		},
		EnableSQLValidation:  false, // 禁用验证避免复杂初始化
		EnableIntentAnalysis: false, // 禁用意图分析
		EnableCostTracking:   false, // 禁用成本追踪
		RequestTimeout:       30 * time.Second,
	}

	// 由于无法创建真实的查询处理器，我们测试各个组件的集成
	_ = config // 避免未使用警告
	
	t.Run("context_building", func(t *testing.T) {
		context, err := contextManager.BuildQueryContext(1, 1001, "查询用户信息")
		assert.NoError(t, err)
		assert.NotNil(t, context)
		assert.Equal(t, "查询用户信息", context.UserQuery)
		assert.Contains(t, context.DatabaseSchema, "users")
	})

	t.Run("template_selection", func(t *testing.T) {
		templateManager := NewPromptTemplateManager()
		
		template, err := templateManager.GetTemplate("base")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		
		// 测试模板格式化
		context := &QueryContext{
			UserQuery:      "查询用户信息",
			DatabaseSchema: "test schema",
		}
		
		prompt, err := template.FormatPrompt(context)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "查询用户信息")
	})

	// 清理资源
	contextManager.Close()
}

// Benchmark测试
func BenchmarkQueryProcessor_ExtractSQL(b *testing.B) {
	qp := &QueryProcessor{}
	text := `{
		"sql": "SELECT u.id, u.name, COUNT(o.id) as order_count FROM users u LEFT JOIN orders o ON u.id = o.user_id WHERE u.status = 'active' GROUP BY u.id, u.name ORDER BY order_count DESC LIMIT 10",
		"confidence": 0.95,
		"query_type": "aggregation"
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qp.extractSQLFromText(text)
	}
}

func BenchmarkQueryProcessor_ValidateRequest(b *testing.B) {
	qp := &QueryProcessor{}
	request := &ChatRequest{
		Query:        "查询用户订单统计信息",
		ConnectionID: 1,
		UserID:       1001,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qp.validateRequest(request)
	}
}

// 测试辅助函数
func createMockLLMResponse(sql string, confidence float64) *llms.ContentResponse {
	responseData := map[string]interface{}{
		"sql":        sql,
		"confidence": confidence,
		"query_type": "base",
		"explanation": "测试查询",
		"table_names": []string{"users"},
		"warnings":   []string{},
		"metadata":   map[string]string{"complexity": "simple"},
	}

	jsonData, _ := json.Marshal(responseData)

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: string(jsonData)},
		},
	}
}