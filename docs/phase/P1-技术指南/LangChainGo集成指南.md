# 🔗 LangChainGo集成技术指南

<div align="center">

![LangChainGo](https://img.shields.io/badge/LangChainGo-v0.1.15-blue.svg)
![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)
![AI](https://img.shields.io/badge/AI-Production-green.svg)

**Chat2SQL P1阶段 - LangChainGo框架集成实战指南**

</div>

## 📋 概述

本文档专门针对P1阶段LangChainGo框架的集成实现，提供详细的技术指导、最佳实践和常见问题解决方案。

## 🏗️ 架构设计

### 🔧 核心组件架构

```go
// P1 LangChainGo集成架构
type LangChainGoService struct {
    // 核心LLM客户端
    primaryClient   llms.Model
    fallbackClient  llms.Model
    
    // 配置管理
    config         *LLMConfig
    
    // 连接池优化
    httpClient     *http.Client
    
    // 监控和指标
    metrics        *Metrics
    costTracker    *CostTracker
}
```

## 📦 依赖配置

### go.mod配置

```go
module chat2sql-go

go 1.23

require (
    // LangChainGo核心库
    github.com/tmc/langchaingo v0.1.15
    
    // LLM提供商
    github.com/tmc/langchaingo/llms/openai v0.1.15
    github.com/tmc/langchaingo/llms/anthropic v0.1.15
    
    // 专用组件
    github.com/tmc/langchaingo/prompts v0.1.15
    github.com/tmc/langchaingo/chains v0.1.15
    github.com/tmc/langchaingo/memory v0.1.15
    github.com/tmc/langchaingo/embeddings v0.1.15
    
    // 基础依赖
    github.com/gin-gonic/gin v1.10.0
    github.com/jackc/pgx/v5 v5.6.0
)
```

### 环境变量配置

```bash
# LangChainGo环境配置
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# 模型配置
export PRIMARY_MODEL_PROVIDER="openai"
export PRIMARY_MODEL_NAME="gpt-4o-mini"
export FALLBACK_MODEL_PROVIDER="anthropic"
export FALLBACK_MODEL_NAME="claude-3-haiku-20240307"

# 性能配置
export LLM_TIMEOUT="30s"
export LLM_MAX_RETRIES="3"
export LLM_MAX_TOKENS="2048"
```

## 🔧 初始化配置

### 客户端初始化

```go
// internal/ai/langchain_service.go
package ai

import (
    "context"
    "fmt"
    "time"
    
    "github.com/tmc/langchaingo/llms"
    "github.com/tmc/langchaingo/llms/openai"
    "github.com/tmc/langchaingo/llms/anthropic"
)

type LangChainGoService struct {
    primaryClient  llms.Model
    fallbackClient llms.Model
    config         *Config
    metrics        *Metrics
}

func NewLangChainGoService(config *Config) (*LangChainGoService, error) {
    // 初始化主要模型客户端
    primaryClient, err := createClient(config.Primary)
    if err != nil {
        return nil, fmt.Errorf("创建主要模型客户端失败: %w", err)
    }
    
    // 初始化备用模型客户端
    fallbackClient, err := createClient(config.Fallback)
    if err != nil {
        return nil, fmt.Errorf("创建备用模型客户端失败: %w", err)
    }
    
    return &LangChainGoService{
        primaryClient:  primaryClient,
        fallbackClient: fallbackClient,
        config:         config,
        metrics:        NewMetrics(),
    }, nil
}

func createClient(modelConfig ModelConfig) (llms.Model, error) {
    switch modelConfig.Provider {
    case "openai":
        return openai.New(
            openai.WithToken(modelConfig.APIKey),
            openai.WithModel(modelConfig.ModelName),
            openai.WithHTTPClient(&http.Client{
                Timeout: modelConfig.Timeout,
            }),
        )
    case "anthropic":
        return anthropic.New(
            anthropic.WithToken(modelConfig.APIKey),
            anthropic.WithModel(modelConfig.ModelName),
        )
    default:
        return nil, fmt.Errorf("不支持的模型提供商: %s", modelConfig.Provider)
    }
}
```

### 配置结构体

```go
// internal/ai/config.go
type Config struct {
    Primary  ModelConfig `yaml:"primary"`
    Fallback ModelConfig `yaml:"fallback"`
    
    // 性能配置
    MaxConcurrency int           `yaml:"max_concurrency"`
    Timeout        time.Duration `yaml:"timeout"`
    
    // 成本控制
    Budget BudgetConfig `yaml:"budget"`
}

type ModelConfig struct {
    Provider    string        `yaml:"provider"`
    ModelName   string        `yaml:"model_name"`
    APIKey      string        `yaml:"api_key"`
    Temperature float64       `yaml:"temperature"`
    MaxTokens   int           `yaml:"max_tokens"`
    TopP        float64       `yaml:"top_p"`
    Timeout     time.Duration `yaml:"timeout"`
}

type BudgetConfig struct {
    DailyLimit   float64 `yaml:"daily_limit"`   // 每日预算上限（美元）
    UserLimit    float64 `yaml:"user_limit"`    // 每用户限制
    AlertThreshold float64 `yaml:"alert_threshold"` // 告警阈值
}
```

## 🚀 核心功能实现

### SQL生成服务

```go
// internal/ai/sql_generator.go
func (lc *LangChainGoService) GenerateSQL(
    ctx context.Context, 
    req *SQLGenerationRequest) (*SQLGenerationResponse, error) {
    
    start := time.Now()
    
    // 1. 构建提示词
    prompt, err := lc.buildPrompt(req)
    if err != nil {
        return nil, err
    }
    
    // 2. 调用LangChainGo生成内容
    response, err := lc.callWithFallback(ctx, prompt)
    if err != nil {
        return nil, err
    }
    
    // 3. 解析响应
    sql, confidence := lc.parseResponse(response)
    
    // 4. 记录指标
    duration := time.Since(start)
    lc.metrics.RecordGeneration(duration, response.Usage.TotalTokens)
    
    return &SQLGenerationResponse{
        SQL:        sql,
        Confidence: confidence,
        TokensUsed: response.Usage.TotalTokens,
        Duration:   duration,
    }, nil
}

func (lc *LangChainGoService) callWithFallback(
    ctx context.Context, 
    prompt string) (*llms.ContentResponse, error) {
    
    // 首先尝试主要模型
    response, err := lc.primaryClient.GenerateContent(ctx, 
        []llms.MessageContent{
            llms.TextParts(llms.ChatMessageTypeHuman, prompt),
        },
        llms.WithTemperature(lc.config.Primary.Temperature),
        llms.WithMaxTokens(lc.config.Primary.MaxTokens),
    )
    
    if err == nil {
        lc.metrics.RecordSuccess("primary")
        return response, nil
    }
    
    // 主要模型失败，尝试备用模型
    lc.metrics.RecordFailure("primary", err)
    
    response, err = lc.fallbackClient.GenerateContent(ctx,
        []llms.MessageContent{
            llms.TextParts(llms.ChatMessageTypeHuman, prompt),
        },
        llms.WithTemperature(lc.config.Fallback.Temperature),
        llms.WithMaxTokens(lc.config.Fallback.MaxTokens),
    )
    
    if err != nil {
        lc.metrics.RecordFailure("fallback", err)
        return nil, fmt.Errorf("主要和备用模型都失败: %w", err)
    }
    
    lc.metrics.RecordSuccess("fallback")
    return response, nil
}
```

### 并发处理优化

```go
// internal/ai/concurrent_processor.go
type ConcurrentProcessor struct {
    langChainService *LangChainGoService
    workerPool       *WorkerPool
    objectPool       sync.Pool
}

func (cp *ConcurrentProcessor) ProcessBatch(
    ctx context.Context, 
    requests []*SQLGenerationRequest) ([]*SQLGenerationResponse, error) {
    
    var wg sync.WaitGroup
    results := make([]*SQLGenerationResponse, len(requests))
    semaphore := make(chan struct{}, cp.workerPool.MaxWorkers)
    
    for i, req := range requests {
        wg.Add(1)
        go func(index int, request *SQLGenerationRequest) {
            defer wg.Done()
            
            // 并发控制
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            // 对象重用
            processor := cp.objectPool.Get().(*RequestProcessor)
            defer func() {
                processor.Reset()
                cp.objectPool.Put(processor)
            }()
            
            // 处理请求
            response, err := cp.langChainService.GenerateSQL(ctx, request)
            if err != nil {
                results[index] = &SQLGenerationResponse{Error: err}
                return
            }
            
            results[index] = response
        }(i, req)
    }
    
    wg.Wait()
    return results, nil
}
```

## 📊 性能优化

### HTTP客户端优化

```go
// internal/ai/http_optimization.go
func CreateOptimizedHTTPClient() *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            MaxIdleConns:        100,              // 最大空闲连接数
            MaxIdleConnsPerHost: 10,               // 每个host最大空闲连接数
            IdleConnTimeout:     90 * time.Second, // 空闲连接超时
            DisableCompression:  false,            // 启用压缩
            WriteBufferSize:     64 * 1024,        // 写缓冲区
            ReadBufferSize:      64 * 1024,        // 读缓冲区
            ForceAttemptHTTP2:   true,             // 强制HTTP/2
        },
        Timeout: 30 * time.Second,
    }
}
```

### 对象池优化

```go
// internal/ai/object_pool.go
var (
    requestPool = sync.Pool{
        New: func() interface{} {
            return &SQLGenerationRequest{}
        },
    }
    
    responsePool = sync.Pool{
        New: func() interface{} {
            return &SQLGenerationResponse{}
        },
    }
)

func GetRequest() *SQLGenerationRequest {
    return requestPool.Get().(*SQLGenerationRequest)
}

func PutRequest(req *SQLGenerationRequest) {
    req.Reset()
    requestPool.Put(req)
}
```

## 🔍 监控和指标

### Prometheus指标

```go
// internal/ai/metrics.go
var (
    llmRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "langchaingo_requests_total",
            Help: "Total LangChainGo requests",
        },
        []string{"provider", "model", "status"},
    )
    
    llmRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "langchaingo_request_duration_seconds",
            Help: "LangChainGo request duration",
            Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
        },
        []string{"provider", "model"},
    )
    
    llmTokensUsed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "langchaingo_tokens_used_total",
            Help: "Total tokens used",
        },
        []string{"provider", "model", "type"}, // type: input/output
    )
)
```

## 🐛 错误处理

### 重试机制

```go
// internal/ai/retry.go
type RetryConfig struct {
    MaxRetries int
    BaseDelay  time.Duration
    MaxDelay   time.Duration
    Multiplier float64
}

func (lc *LangChainGoService) GenerateWithRetry(
    ctx context.Context, 
    req *SQLGenerationRequest) (*SQLGenerationResponse, error) {
    
    var lastErr error
    
    for attempt := 0; attempt <= lc.config.Retry.MaxRetries; attempt++ {
        response, err := lc.GenerateSQL(ctx, req)
        if err == nil {
            return response, nil
        }
        
        lastErr = err
        
        // 检查是否应该重试
        if !shouldRetry(err) {
            break
        }
        
        // 计算延迟时间
        delay := calculateDelay(attempt, lc.config.Retry)
        
        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    return nil, fmt.Errorf("重试%d次后仍失败: %w", 
        lc.config.Retry.MaxRetries, lastErr)
}

func shouldRetry(err error) bool {
    // 检查错误类型，决定是否重试
    if strings.Contains(err.Error(), "rate limit") {
        return true
    }
    if strings.Contains(err.Error(), "timeout") {
        return true
    }
    if strings.Contains(err.Error(), "connection refused") {
        return true
    }
    return false
}
```

## 🧪 测试策略

### 单元测试

```go
// internal/ai/langchain_service_test.go
func TestLangChainGoService_GenerateSQL(t *testing.T) {
    mockClient := &MockLLMClient{}
    service := &LangChainGoService{
        primaryClient: mockClient,
        config:       defaultTestConfig(),
        metrics:      NewTestMetrics(),
    }
    
    testCases := []struct {
        name          string
        request       *SQLGenerationRequest
        mockResponse  *llms.ContentResponse
        mockError     error
        expectedSQL   string
        expectedError bool
    }{
        {
            name: "成功生成SQL",
            request: &SQLGenerationRequest{
                Query:  "查询所有用户",
                Schema: testSchema,
            },
            mockResponse: &llms.ContentResponse{
                Choices: []llms.ContentChoice{
                    {Content: "SELECT * FROM users"},
                },
                Usage: llms.Usage{TotalTokens: 50},
            },
            expectedSQL: "SELECT * FROM users",
        },
        {
            name: "API错误",
            request: &SQLGenerationRequest{
                Query: "查询用户",
            },
            mockError:     errors.New("API rate limit"),
            expectedError: true,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            mockClient.SetResponse(tc.mockResponse, tc.mockError)
            
            response, err := service.GenerateSQL(context.Background(), tc.request)
            
            if tc.expectedError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tc.expectedSQL, response.SQL)
            }
        })
    }
}
```

## ⚠️ 常见问题

### Q1: LangChainGo客户端初始化失败

**问题**: `failed to create OpenAI client: invalid API key`

**解决方案**:
```bash
# 检查环境变量
echo $OPENAI_API_KEY

# 确保API密钥格式正确
export OPENAI_API_KEY="sk-proj-..."  # 新格式
```

### Q2: 请求超时

**问题**: `context deadline exceeded`

**解决方案**:
```go
// 增加超时时间
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// 或调整客户端配置
client := openai.New(
    openai.WithHTTPClient(&http.Client{
        Timeout: 60 * time.Second,
    }),
)
```

### Q3: Token使用量过高

**问题**: 成本控制失效

**解决方案**:
```go
// 添加Token预检查
if estimatedTokens > maxTokensPerRequest {
    return nil, errors.New("请求Token数量超限")
}

// 使用更便宜的模型
config.Primary.ModelName = "gpt-4o-mini"  // 而不是 "gpt-4"
```

### Q4: 并发请求限制

**问题**: `rate limit exceeded`

**解决方案**:
```go
// 添加速率限制器
rateLimiter := rate.NewLimiter(rate.Limit(10), 1) // 每秒10请求

// 在请求前检查
if err := rateLimiter.Wait(ctx); err != nil {
    return nil, err
}
```

## 📚 最佳实践

### 1. 配置管理

```go
// 使用配置文件而不是硬编码
config, err := LoadConfig("configs/langchaingo.yaml")
if err != nil {
    log.Fatal("配置加载失败", err)
}
```

### 2. 错误处理

```go
// 总是处理LangChainGo错误
response, err := client.GenerateContent(ctx, messages)
if err != nil {
    // 记录详细错误信息
    log.Error("LangChainGo调用失败", 
        zap.String("model", modelName),
        zap.Error(err))
    return nil, err
}
```

### 3. 资源清理

```go
// 确保适当清理资源
defer func() {
    if closer, ok := client.(io.Closer); ok {
        closer.Close()
    }
}()
```

### 4. 监控集成

```go
// 每个请求都要记录指标
defer func(start time.Time) {
    duration := time.Since(start)
    metrics.RecordRequestDuration(duration)
    metrics.RecordTokenUsage(response.Usage.TotalTokens)
}(time.Now())
```

---

<div align="center">

**🔗 LangChainGo集成成功的关键：合理配置 + 错误处理 + 性能监控**

</div>