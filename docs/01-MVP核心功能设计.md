# 阶段1：MVP核心功能详细设计

## 📋 阶段概述

### 目标与价值
**核心目标**：在3周内交付一个可用的Chat2SQL最小可用产品，验证核心价值假设

**关键价值主张**：
- 用户能够用中文自然语言查询业务数据
- AI自动生成准确的SQL语句
- 查询结果以直观的图表形式展示
- 完整的对话历史记录和管理

### 成功指标
- **功能完整性**：支持基础SELECT查询、聚合分析、简单JOIN
- **准确率**：SQL生成准确率 > 85%
- **性能**：端到端响应时间 < 10秒
- **用户体验**：直观的聊天界面，清晰的结果展示

## 🛠️ 技术选型详细分析

### 后端技术栈：Go + Gin

#### 选择理由深度分析

**Go语言优势**：
```go
// 1. 静态类型安全 - 编译期发现错误
type ChatRequest struct {
    Message   string `json:"message" validate:"required,min=1,max=1000"`
    SessionID string `json:"session_id" validate:"required,uuid"`
}

// 2. 并发性能优异 - 协程处理多用户请求  
func (s *ChatService) HandleConcurrentRequests(ctx context.Context) {
    for i := 0; i < 1000; i++ {
        go s.processChatRequest(ctx, request) // 轻量级协程
    }
}

// 3. 内存管理高效 - 无GC停顿问题
```

**Gin框架优势**：
```go
// 1. 中间件生态丰富
r := gin.Default()
r.Use(gin.Logger())                    // 日志中间件
r.Use(gin.Recovery())                  // 崩溃恢复
r.Use(cors.New(cors.DefaultConfig()))  // 跨域处理
r.Use(RateLimitMiddleware())           // 限流中间件

// 2. 路由设计优雅
api := r.Group("/api/v1")
{
    api.POST("/chat", chatHandler)
    api.GET("/history/:sessionId", historyHandler)
    api.DELETE("/session/:sessionId", clearSessionHandler)
}

// 3. JSON处理高效
c.JSON(http.StatusOK, gin.H{
    "data": result,
    "success": true,
    "timestamp": time.Now(),
})
```

#### 替代方案分析
| 方案 | 优势 | 劣势 | 适用场景 |
|------|------|------|----------|
| **Node.js + Express** | 生态最丰富，JavaScript统一栈 | 单线程，CPU密集任务性能差 | 小型项目，前端团队主导 |
| **Python + FastAPI** | AI生态最好，开发速度快 | GIL限制并发，部署复杂 | AI原型验证，数据科学团队 |
| **Java + Spring Boot** | 企业级稳定，工具链完善 | 启动慢，内存占用大 | 大型企业级项目 |
| **Go + Gin** ⭐ | 高并发，静态编译，部署简单 | 生态相对较小，学习成本 | 中小型高性能项目 |

### 前端技术栈：Svelte + DaisyUI + TypeScript

#### Svelte选择理由
```svelte
<!-- 1. 编译时优化 - 运行时性能最佳 -->
<script>
  let count = 0;
  // 编译后直接是DOM操作，无虚拟DOM开销
</script>

<!-- 2. 语法简洁直观 -->
<button on:click={() => count += 1}>
  点击次数：{count}
</button>

<!-- 3. 响应式数据绑定 -->
<input bind:value={userInput} placeholder="输入您的问题" />
{#if userInput.length > 0}
  <p>您输入了：{userInput}</p>
{/if}

<!-- 4. 内置状态管理 -->
<script>
  import { writable } from 'svelte/store';
  export const chatHistory = writable([]);
</script>
```

#### DaisyUI组件库优势
```html
<!-- 1. 开箱即用的美观组件 -->
<div class="chat chat-start">
  <div class="chat-bubble">用户消息</div>
</div>
<div class="chat chat-end">
  <div class="chat-bubble chat-bubble-primary">AI回复</div>
</div>

<!-- 2. 响应式设计内置 -->
<div class="stats stats-vertical lg:stats-horizontal shadow">
  <div class="stat">
    <div class="stat-title">查询次数</div>
    <div class="stat-value">1,200</div>
  </div>
</div>

<!-- 3. 主题切换支持 -->
<html data-theme="corporate"> <!-- 或 dark、light等 -->
```

### 数据库设计：SQLite → PostgreSQL渐进式升级

#### MVP阶段：SQLite
```sql
-- 优势：零配置，文件数据库，快速启动
-- 劣势：并发性能限制，功能相对简单

-- 创建表结构
CREATE TABLE chat_sessions (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL DEFAULT 'anonymous',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chat_sessions_user_id ON chat_sessions(user_id);
CREATE INDEX idx_chat_sessions_created_at ON chat_sessions(created_at);
```

#### 升级路径：PostgreSQL
```sql
-- 平滑迁移策略：相同表结构，数据导出导入
-- 增强功能：JSON字段支持，全文搜索，复杂查询

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id TEXT NOT NULL DEFAULT 'anonymous',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chat_sessions_metadata_gin ON chat_sessions USING GIN (metadata);
```

## 📊 数据库详细设计

### 核心表结构

```sql
-- 1. 聊天会话表
CREATE TABLE chat_sessions (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL DEFAULT 'anonymous',
    title TEXT DEFAULT '新对话',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_deleted BOOLEAN DEFAULT FALSE
);

-- 2. 聊天消息表
CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    sql_query TEXT,                    -- AI生成的SQL（仅assistant消息）
    query_result JSON,                 -- 查询结果（仅assistant消息）
    chart_config JSON,                 -- 图表配置（仅assistant消息）
    token_count INTEGER DEFAULT 0,     -- Token消耗统计
    latency_ms INTEGER DEFAULT 0,      -- 响应延迟
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
);

-- 3. 查询缓存表
CREATE TABLE query_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    query_hash TEXT UNIQUE NOT NULL,   -- 查询内容的Hash
    sql_query TEXT NOT NULL,
    result_data JSON NOT NULL,
    chart_config JSON,
    hit_count INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

-- 4. 系统配置表
CREATE TABLE system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 5. 性能统计表
CREATE TABLE usage_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    total_queries INTEGER DEFAULT 0,
    successful_queries INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    avg_latency_ms REAL DEFAULT 0,
    cache_hit_rate REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX idx_chat_messages_session_id ON chat_messages(session_id);
CREATE INDEX idx_chat_messages_created_at ON chat_messages(created_at);
CREATE INDEX idx_query_cache_hash ON query_cache(query_hash);
CREATE INDEX idx_query_cache_expires ON query_cache(expires_at);
CREATE INDEX idx_usage_stats_date ON usage_stats(date);
```

### 数据迁移策略
```go
// internal/database/migration.go
package database

type Migration struct {
    Version     int
    Description string
    UpSQL       string
    DownSQL     string
}

var migrations = []Migration{
    {
        Version:     1,
        Description: "Create initial tables",
        UpSQL: `
            CREATE TABLE chat_sessions (...);
            CREATE TABLE chat_messages (...);
        `,
        DownSQL: `
            DROP TABLE chat_messages;
            DROP TABLE chat_sessions;  
        `,
    },
}

func RunMigrations(db *sql.DB) error {
    // 执行数据库迁移逻辑
}
```

## 🔧 详细技术实现

### 项目结构设计

```
chat2sql-go/
├── cmd/
│   └── server/
│       └── main.go                 # 应用入口
├── internal/                       # 内部包，不对外暴露
│   ├── config/
│   │   ├── config.go              # 配置管理
│   │   └── env.go                 # 环境变量处理
│   ├── handler/                   # HTTP处理器
│   │   ├── chat.go                # 聊天相关接口
│   │   ├── session.go             # 会话管理接口
│   │   └── health.go              # 健康检查
│   ├── service/                   # 业务服务层
│   │   ├── chat_service.go        # 聊天服务核心逻辑
│   │   ├── llm_service.go         # LLM调用服务
│   │   ├── sql_service.go         # SQL执行服务
│   │   └── chart_service.go       # 图表生成服务
│   ├── repository/                # 数据访问层
│   │   ├── session_repo.go        # 会话数据访问
│   │   ├── message_repo.go        # 消息数据访问
│   │   └── cache_repo.go          # 缓存数据访问
│   ├── model/                     # 数据模型
│   │   ├── chat.go                # 聊天相关模型
│   │   ├── database.go            # 数据库连接模型
│   │   └── response.go            # API响应模型
│   ├── middleware/                # 中间件
│   │   ├── cors.go                # 跨域处理
│   │   ├── logger.go              # 请求日志
│   │   └── recovery.go            # 错误恢复
│   ├── llm/                       # LLM抽象层（关键设计）
│   │   ├── interface.go           # LLM接口定义
│   │   ├── ollama.go              # Ollama实现
│   │   └── mock.go                # 测试Mock实现
│   └── database/
│       ├── connection.go          # 数据库连接
│       ├── migration.go           # 数据库迁移
│       └── query.go               # 查询构建器
├── pkg/                           # 公共包，可对外暴露
│   ├── logger/                    # 日志工具
│   ├── validator/                 # 数据验证
│   └── utils/                     # 通用工具
├── web/                           # 前端代码
│   ├── src/
│   │   ├── routes/                # 路由页面
│   │   ├── lib/
│   │   │   ├── components/        # 组件库
│   │   │   ├── stores/            # 状态管理
│   │   │   └── utils/             # 工具函数
│   │   └── app.html               # 应用模板
│   ├── static/                    # 静态资源
│   ├── package.json
│   ├── svelte.config.js
│   └── tailwind.config.js
├── deployments/                   # 部署配置
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── docker-compose.yml
│   └── k8s/                       # Kubernetes配置
├── docs/                          # 文档
├── scripts/                       # 脚本工具
│   ├── build.sh                  # 构建脚本
│   └── migrate.sh                # 数据库迁移脚本
├── tests/                         # 测试文件
│   ├── integration/              # 集成测试
│   └── fixtures/                 # 测试数据
├── go.mod
├── go.sum
└── README.md
```

### 核心组件实现

#### 1. LLM抽象层设计（关键设计）

```go
// internal/llm/interface.go
package llm

import (
    "context"
    "time"
)

// LLMProvider 统一的LLM接口，支持后续扩展多种模型
type LLMProvider interface {
    // 生成SQL查询
    GenerateSQL(ctx context.Context, req *SQLGenerateRequest) (*SQLGenerateResponse, error)
    
    // 流式生成（为后续扩展预留）
    GenerateStream(ctx context.Context, req *SQLGenerateRequest) (<-chan *StreamChunk, error)
    
    // 健康检查
    Health(ctx context.Context) error
    
    // 获取模型信息
    GetModelInfo() ModelInfo
}

// 请求结构
type SQLGenerateRequest struct {
    UserQuestion    string            `json:"user_question"`
    DatabaseSchema  *DatabaseSchema   `json:"database_schema,omitempty"`
    ChatHistory     []ChatMessage     `json:"chat_history,omitempty"`
    MaxTokens       int               `json:"max_tokens,omitempty"`
    Temperature     float32           `json:"temperature,omitempty"`
}

// 响应结构
type SQLGenerateResponse struct {
    SQL             string            `json:"sql"`
    Confidence      float32           `json:"confidence"`      // 置信度
    Explanation     string            `json:"explanation"`     // SQL解释
    TokensUsed      int               `json:"tokens_used"`
    Latency         time.Duration     `json:"latency"`
    ModelUsed       string            `json:"model_used"`
}

// 流式响应块
type StreamChunk struct {
    Content         string            `json:"content"`
    Done            bool              `json:"done"`
    TokenCount      int               `json:"token_count"`
}

// 数据库结构信息
type DatabaseSchema struct {
    Tables          []TableInfo       `json:"tables"`
    Relationships   []Relationship    `json:"relationships"`
}

type TableInfo struct {
    Name            string            `json:"name"`
    Comment         string            `json:"comment"`
    Columns         []ColumnInfo      `json:"columns"`
}

type ColumnInfo struct {
    Name            string            `json:"name"`
    Type            string            `json:"type"`
    Comment         string            `json:"comment"`
    IsPrimaryKey    bool              `json:"is_primary_key"`
    IsForeignKey    bool              `json:"is_foreign_key"`
    IsNullable      bool              `json:"is_nullable"`
}
```

#### 2. Ollama Provider实现

```go
// internal/llm/ollama.go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type OllamaProvider struct {
    baseURL     string
    model       string
    client      *http.Client
    timeout     time.Duration
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
    return &OllamaProvider{
        baseURL: baseURL,
        model:   model,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        timeout: 30 * time.Second,
    }
}

func (o *OllamaProvider) GenerateSQL(ctx context.Context, req *SQLGenerateRequest) (*SQLGenerateResponse, error) {
    startTime := time.Now()
    
    // 构建提示词
    prompt := o.buildPrompt(req)
    
    // 调用Ollama API
    ollamaReq := map[string]interface{}{
        "model":  o.model,
        "prompt": prompt,
        "stream": false,
        "options": map[string]interface{}{
            "temperature": req.Temperature,
            "num_predict": req.MaxTokens,
        },
    }
    
    reqBody, err := json.Marshal(ollamaReq)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewBuffer(reqBody))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := o.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    defer resp.Body.Close()
    
    var ollamaResp struct {
        Response     string `json:"response"`
        TotalTokens  int    `json:"total_tokens"`
        Done         bool   `json:"done"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }
    
    // 解析SQL和解释
    sql, explanation := o.parseResponse(ollamaResp.Response)
    
    return &SQLGenerateResponse{
        SQL:         sql,
        Confidence:  0.8, // 暂时固定值，后续可基于模型返回调整
        Explanation: explanation,
        TokensUsed:  ollamaResp.TotalTokens,
        Latency:     time.Since(startTime),
        ModelUsed:   o.model,
    }, nil
}

func (o *OllamaProvider) buildPrompt(req *SQLGenerateRequest) string {
    prompt := fmt.Sprintf(`你是一个专业的SQL查询助手。请根据用户的自然语言问题生成准确的SQL查询语句。

用户问题: %s

数据库结构信息:
`, req.UserQuestion)
    
    // 添加数据库结构信息
    if req.DatabaseSchema != nil {
        for _, table := range req.DatabaseSchema.Tables {
            prompt += fmt.Sprintf("\n表名: %s", table.Name)
            if table.Comment != "" {
                prompt += fmt.Sprintf(" (%s)", table.Comment)
            }
            prompt += "\n字段:"
            
            for _, col := range table.Columns {
                prompt += fmt.Sprintf("\n  - %s %s", col.Name, col.Type)
                if col.Comment != "" {
                    prompt += fmt.Sprintf(" // %s", col.Comment)
                }
            }
            prompt += "\n"
        }
    }
    
    prompt += `
请按照以下格式返回:

SQL:
[生成的SQL语句]

解释:
[对SQL语句的简单解释]

注意事项:
1. 只生成SELECT语句，不要生成DELETE、UPDATE、DROP等危险操作
2. 确保SQL语法正确
3. 使用适当的WHERE条件和聚合函数
4. 如果需要分组，记得使用GROUP BY
5. 返回结果要包含有意义的列名
`
    
    return prompt
}

func (o *OllamaProvider) parseResponse(response string) (sql, explanation string) {
    // 简单的响应解析逻辑，后续可以优化
    lines := strings.Split(response, "\n")
    
    var sqlLines, explainLines []string
    currentSection := ""
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "SQL:") {
            currentSection = "sql"
            continue
        }
        if strings.HasPrefix(line, "解释:") {
            currentSection = "explain"
            continue
        }
        
        switch currentSection {
        case "sql":
            if line != "" {
                sqlLines = append(sqlLines, line)
            }
        case "explain":
            if line != "" {
                explainLines = append(explainLines, line)
            }
        }
    }
    
    sql = strings.Join(sqlLines, "\n")
    explanation = strings.Join(explainLines, "\n")
    
    return sql, explanation
}

func (o *OllamaProvider) Health(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
    if err != nil {
        return err
    }
    
    resp, err := o.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

func (o *OllamaProvider) GetModelInfo() ModelInfo {
    return ModelInfo{
        Name:     o.model,
        Provider: "Ollama",
        Type:     "Local",
    }
}
```

#### 3. 聊天服务核心逻辑

```go
// internal/service/chat_service.go  
package service

import (
    "context"
    "fmt"
    "time"
    
    "chat2sql/internal/llm"
    "chat2sql/internal/model"
    "chat2sql/internal/repository"
)

type ChatService struct {
    llmProvider    llm.LLMProvider
    messageRepo    repository.MessageRepository
    sessionRepo    repository.SessionRepository
    cacheRepo      repository.CacheRepository
    sqlService     *SQLService
    chartService   *ChartService
}

func NewChatService(
    llmProvider llm.LLMProvider,
    messageRepo repository.MessageRepository,
    sessionRepo repository.SessionRepository,
    cacheRepo repository.CacheRepository,
    sqlService *SQLService,
    chartService *ChartService,
) *ChatService {
    return &ChatService{
        llmProvider:  llmProvider,
        messageRepo:  messageRepo,
        sessionRepo:  sessionRepo,
        cacheRepo:    cacheRepo,
        sqlService:   sqlService,
        chartService: chartService,
    }
}

// ProcessChat 处理用户聊天请求的核心逻辑
func (cs *ChatService) ProcessChat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
    // 1. 验证会话存在性
    session, err := cs.sessionRepo.GetByID(ctx, req.SessionID)
    if err != nil {
        return nil, fmt.Errorf("get session: %w", err)
    }
    
    // 2. 保存用户消息
    userMessage := &model.ChatMessage{
        SessionID: req.SessionID,
        Role:      "user", 
        Content:   req.Message,
        CreatedAt: time.Now(),
    }
    
    if err := cs.messageRepo.Create(ctx, userMessage); err != nil {
        return nil, fmt.Errorf("save user message: %w", err)
    }
    
    // 3. 检查缓存
    cacheKey := cs.generateCacheKey(req.Message)
    if cached, err := cs.cacheRepo.Get(ctx, cacheKey); err == nil && cached != nil {
        // 返回缓存结果
        return cs.buildResponseFromCache(cached), nil
    }
    
    // 4. 获取聊天历史（用于上下文）
    history, err := cs.messageRepo.GetBySessionID(ctx, req.SessionID, 10) // 最近10条
    if err != nil {
        return nil, fmt.Errorf("get chat history: %w", err)
    }
    
    // 5. 获取数据库结构信息
    dbSchema, err := cs.sqlService.GetDatabaseSchema(ctx)
    if err != nil {
        return nil, fmt.Errorf("get database schema: %w", err)
    }
    
    // 6. 调用LLM生成SQL
    llmReq := &llm.SQLGenerateRequest{
        UserQuestion:   req.Message,
        DatabaseSchema: dbSchema,
        ChatHistory:    cs.convertToLLMHistory(history),
        MaxTokens:      1000,
        Temperature:    0.1, // 较低的温度确保结果稳定
    }
    
    llmResp, err := cs.llmProvider.GenerateSQL(ctx, llmReq)
    if err != nil {
        return nil, fmt.Errorf("generate SQL: %w", err)
    }
    
    // 7. 验证和执行SQL
    queryResult, err := cs.sqlService.ExecuteQuery(ctx, llmResp.SQL)
    if err != nil {
        return nil, fmt.Errorf("execute SQL: %w", err)
    }
    
    // 8. 生成图表配置
    chartConfig, err := cs.chartService.GenerateChartConfig(ctx, queryResult)
    if err != nil {
        // 图表生成失败不影响主流程
        chartConfig = nil
    }
    
    // 9. 保存AI回复消息
    aiMessage := &model.ChatMessage{
        SessionID:    req.SessionID,
        Role:         "assistant",
        Content:      cs.formatAIResponse(llmResp.Explanation, queryResult),
        SQLQuery:     llmResp.SQL,
        QueryResult:  queryResult,
        ChartConfig:  chartConfig,
        TokenCount:   llmResp.TokensUsed,
        LatencyMS:    int(llmResp.Latency.Milliseconds()),
        CreatedAt:    time.Now(),
    }
    
    if err := cs.messageRepo.Create(ctx, aiMessage); err != nil {
        return nil, fmt.Errorf("save AI message: %w", err)
    }
    
    // 10. 缓存结果（24小时有效期）
    cacheData := &model.CacheData{
        SQL:         llmResp.SQL,
        Result:      queryResult,
        ChartConfig: chartConfig,
        ExpiresAt:   time.Now().Add(24 * time.Hour),
    }
    cs.cacheRepo.Set(ctx, cacheKey, cacheData)
    
    // 11. 构建响应
    response := &model.ChatResponse{
        MessageID:   aiMessage.ID,
        Content:     aiMessage.Content,
        SQL:         llmResp.SQL,
        Data:        queryResult.Data,
        ChartConfig: chartConfig,
        TokensUsed:  llmResp.TokensUsed,
        Latency:     llmResp.Latency,
        Cached:      false,
    }
    
    return response, nil
}

// 其他辅助方法...
func (cs *ChatService) generateCacheKey(message string) string {
    // 使用消息内容的哈希作为缓存键
    h := sha256.Sum256([]byte(message))
    return fmt.Sprintf("chat_cache_%x", h)
}

func (cs *ChatService) formatAIResponse(explanation string, result *model.QueryResult) string {
    return fmt.Sprintf("%s\n\n查询结果：共找到 %d 条记录", explanation, len(result.Data))
}
```

#### 4. HTTP处理器实现

```go
// internal/handler/chat.go
package handler

import (
    "net/http"
    "chat2sql/internal/service"
    "chat2sql/internal/model"
    "github.com/gin-gonic/gin"
)

type ChatHandler struct {
    chatService *service.ChatService
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
    return &ChatHandler{
        chatService: chatService,
    }
}

// PostChat 处理聊天请求
func (h *ChatHandler) PostChat(c *gin.Context) {
    var req model.ChatRequest
    
    // 1. 绑定和验证请求参数
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, model.ErrorResponse{
            Error:   "invalid_request",
            Message: "请求参数错误: " + err.Error(),
        })
        return
    }
    
    // 2. 参数验证
    if req.Message == "" {
        c.JSON(http.StatusBadRequest, model.ErrorResponse{
            Error:   "empty_message", 
            Message: "消息内容不能为空",
        })
        return
    }
    
    if req.SessionID == "" {
        c.JSON(http.StatusBadRequest, model.ErrorResponse{
            Error:   "empty_session_id",
            Message: "会话ID不能为空",
        })
        return
    }
    
    // 3. 调用服务处理
    response, err := h.chatService.ProcessChat(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, model.ErrorResponse{
            Error:   "process_error",
            Message: "处理请求失败: " + err.Error(),
        })
        return
    }
    
    // 4. 返回成功响应
    c.JSON(http.StatusOK, model.SuccessResponse{
        Success: true,
        Data:    response,
    })
}

// GetChatHistory 获取聊天历史
func (h *ChatHandler) GetChatHistory(c *gin.Context) {
    sessionID := c.Param("sessionId")
    if sessionID == "" {
        c.JSON(http.StatusBadRequest, model.ErrorResponse{
            Error:   "empty_session_id",
            Message: "会话ID不能为空",
        })
        return
    }
    
    history, err := h.chatService.GetChatHistory(c.Request.Context(), sessionID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, model.ErrorResponse{
            Error:   "get_history_error", 
            Message: "获取聊天历史失败: " + err.Error(),
        })
        return
    }
    
    c.JSON(http.StatusOK, model.SuccessResponse{
        Success: true,
        Data:    history,
    })
}

// CreateSession 创建新会话
func (h *ChatHandler) CreateSession(c *gin.Context) {
    var req model.CreateSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        req.Title = "新对话" // 使用默认标题
    }
    
    session, err := h.chatService.CreateSession(c.Request.Context(), req.Title)
    if err != nil {
        c.JSON(http.StatusInternalServerError, model.ErrorResponse{
            Error:   "create_session_error",
            Message: "创建会话失败: " + err.Error(),
        })
        return
    }
    
    c.JSON(http.StatusOK, model.SuccessResponse{
        Success: true,
        Data:    session,
    })
}

// DeleteSession 删除会话
func (h *ChatHandler) DeleteSession(c *gin.Context) {
    sessionID := c.Param("sessionId")
    if sessionID == "" {
        c.JSON(http.StatusBadRequest, model.ErrorResponse{
            Error:   "empty_session_id",
            Message: "会话ID不能为空", 
        })
        return
    }
    
    err := h.chatService.DeleteSession(c.Request.Context(), sessionID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, model.ErrorResponse{
            Error:   "delete_session_error",
            Message: "删除会话失败: " + err.Error(),
        })
        return
    }
    
    c.JSON(http.StatusOK, model.SuccessResponse{
        Success: true,
        Message: "会话删除成功",
    })
}
```

### 前端详细实现

#### 1. 主要组件结构

```svelte
<!-- web/src/routes/+page.svelte - 主页面 -->
<script lang="ts">
  import { onMount } from 'svelte';
  import ChatInterface from '$lib/components/ChatInterface.svelte';
  import Sidebar from '$lib/components/Sidebar.svelte';
  import SqlDisplay from '$lib/components/SqlDisplay.svelte';
  import ChartDisplay from '$lib/components/ChartDisplay.svelte';
  import { chatStore } from '$lib/stores/chatStore';

  let currentSessionId = '';

  onMount(() => {
    // 初始化或恢复会话
    initializeSession();
  });

  async function initializeSession() {
    try {
      const response = await fetch('/api/v1/session', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '新对话' })
      });
      
      const result = await response.json();
      if (result.success) {
        currentSessionId = result.data.id;
        chatStore.setSessionId(currentSessionId);
      }
    } catch (error) {
      console.error('创建会话失败:', error);
    }
  }
</script>

<div class="flex h-screen bg-base-100">
  <!-- 侧边栏 -->
  <div class="w-80 border-r border-base-300">
    <Sidebar bind:currentSessionId />
  </div>
  
  <!-- 主要内容区域 -->
  <div class="flex-1 flex flex-col">
    <!-- 聊天界面 -->
    <div class="flex-1 flex">
      <div class="flex-1 p-4">
        <ChatInterface sessionId={currentSessionId} />
      </div>
      
      <!-- 右侧结果展示 -->
      <div class="w-1/2 p-4 border-l border-base-300">
        <div class="space-y-4">
          <SqlDisplay />
          <ChartDisplay />
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  /* 自定义样式 */
  :global(body) {
    font-family: 'Inter', 'Segoe UI', 'Roboto', sans-serif;
  }
</style>
```

#### 2. 聊天界面组件

```svelte
<!-- web/src/lib/components/ChatInterface.svelte -->
<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { chatStore, type ChatMessage } from '$lib/stores/chatStore';
  import LoadingSpinner from './LoadingSpinner.svelte';
  import MessageBubble from './MessageBubble.svelte';

  export let sessionId: string;

  let messageInput = '';
  let isLoading = false;
  let chatContainer: HTMLElement;
  let messages: ChatMessage[] = [];

  // 订阅聊天记录变化
  $: messages = $chatStore.messages;

  onMount(() => {
    loadChatHistory();
  });

  afterUpdate(() => {
    // 自动滚动到底部
    if (chatContainer) {
      chatContainer.scrollTop = chatContainer.scrollHeight;
    }
  });

  async function loadChatHistory() {
    if (!sessionId) return;
    
    try {
      const response = await fetch(`/api/v1/chat/history/${sessionId}`);
      const result = await response.json();
      
      if (result.success) {
        chatStore.setMessages(result.data);
      }
    } catch (error) {
      console.error('加载聊天历史失败:', error);
    }
  }

  async function sendMessage() {
    if (!messageInput.trim() || isLoading) return;

    const userMessage = messageInput.trim();
    messageInput = '';
    isLoading = true;

    try {
      // 立即显示用户消息
      chatStore.addMessage({
        role: 'user',
        content: userMessage,
        timestamp: new Date()
      });

      // 发送请求到后端
      const response = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: userMessage,
          session_id: sessionId
        })
      });

      const result = await response.json();

      if (result.success) {
        // 添加AI回复消息
        chatStore.addMessage({
          role: 'assistant',
          content: result.data.content,
          sql: result.data.sql,
          data: result.data.data,
          chartConfig: result.data.chart_config,
          timestamp: new Date()
        });
      } else {
        throw new Error(result.message);
      }
    } catch (error) {
      console.error('发送消息失败:', error);
      
      // 显示错误消息
      chatStore.addMessage({
        role: 'assistant',
        content: `抱歉，处理您的请求时出现了错误：${error.message}`,
        timestamp: new Date(),
        isError: true
      });
    } finally {
      isLoading = false;
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      sendMessage();
    }
  }
</script>

<div class="flex flex-col h-full">
  <!-- 聊天标题 -->
  <div class="bg-base-200 p-4 border-b border-base-300">
    <h1 class="text-xl font-semibold">Chat2SQL 智能查询</h1>
    <p class="text-sm text-base-content/70">使用自然语言查询您的业务数据</p>
  </div>

  <!-- 消息列表 -->
  <div 
    bind:this={chatContainer}
    class="flex-1 overflow-y-auto p-4 space-y-4"
  >
    {#if messages.length === 0}
      <!-- 欢迎界面 -->
      <div class="text-center py-12">
        <div class="text-4xl mb-4">👋</div>
        <h2 class="text-xl font-semibold mb-2">欢迎使用 Chat2SQL</h2>
        <p class="text-base-content/70 mb-6">
          请输入您想要查询的问题，我会帮您生成相应的SQL语句并展示结果
        </p>
        
        <!-- 示例问题 -->
        <div class="grid gap-2 max-w-md mx-auto">
          <button 
            class="btn btn-outline btn-sm"
            on:click={() => messageInput = '显示2025年6月各部门工资最高的人'}
          >
            显示2025年6月各部门工资最高的人
          </button>
          <button 
            class="btn btn-outline btn-sm"
            on:click={() => messageInput = '统计最近30天的销售额趋势'}
          >
            统计最近30天的销售额趋势
          </button>
          <button 
            class="btn btn-outline btn-sm"
            on:click={() => messageInput = '查看订单状态分布情况'}
          >
            查看订单状态分布情况
          </button>
        </div>
      </div>
    {/if}

    {#each messages as message (message.id)}
      <MessageBubble {message} />
    {/each}

    {#if isLoading}
      <div class="chat chat-start">
        <div class="chat-bubble">
          <LoadingSpinner />
          <span class="ml-2">AI正在思考中...</span>
        </div>
      </div>
    {/if}
  </div>

  <!-- 输入区域 -->
  <div class="bg-base-200 p-4 border-t border-base-300">
    <div class="flex gap-2">
      <textarea
        bind:value={messageInput}
        on:keydown={handleKeyDown}
        placeholder="输入您的问题，例如：显示2025年6月各部门工资最高的人"
        class="textarea textarea-bordered flex-1 resize-none"
        rows="1"
        disabled={isLoading}
      ></textarea>
      
      <button
        on:click={sendMessage}
        disabled={isLoading || !messageInput.trim()}
        class="btn btn-primary min-w-[80px]"
      >
        {#if isLoading}
          <LoadingSpinner size="sm" />
        {:else}
          发送
        {/if}
      </button>
    </div>
    
    <!-- 提示信息 -->
    <div class="text-xs text-base-content/50 mt-2">
      按 Enter 发送，Shift + Enter 换行
    </div>
  </div>
</div>
```

#### 3. 消息气泡组件

```svelte
<!-- web/src/lib/components/MessageBubble.svelte -->
<script lang="ts">
  import { chatStore, type ChatMessage } from '$lib/stores/chatStore';
  import CodeBlock from './CodeBlock.svelte';
  import DataTable from './DataTable.svelte';
  
  export let message: ChatMessage;

  function formatTimestamp(date: Date): string {
    return date.toLocaleTimeString('zh-CN', { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    // 显示复制成功提示
  }
</script>

<div class="chat {message.role === 'user' ? 'chat-end' : 'chat-start'}">
  <!-- 头像 -->
  <div class="chat-image avatar">
    <div class="w-8 rounded-full">
      {#if message.role === 'user'}
        <div class="bg-primary text-primary-content w-8 h-8 rounded-full flex items-center justify-center">
          👤
        </div>
      {:else}
        <div class="bg-secondary text-secondary-content w-8 h-8 rounded-full flex items-center justify-center">
          🤖
        </div>
      {/if}
    </div>
  </div>

  <!-- 消息内容 -->
  <div class="chat-bubble {message.role === 'user' ? 'chat-bubble-primary' : ''} {message.isError ? 'chat-bubble-error' : ''}">
    <!-- 文本内容 -->
    <div class="whitespace-pre-wrap">{message.content}</div>
    
    <!-- SQL代码块 -->
    {#if message.sql}
      <div class="mt-3">
        <div class="text-xs text-base-content/70 mb-1">生成的SQL语句：</div>
        <CodeBlock 
          code={message.sql} 
          language="sql" 
          on:copy={() => copyToClipboard(message.sql)}
        />
      </div>
    {/if}
    
    <!-- 数据表格 -->
    {#if message.data && message.data.length > 0}
      <div class="mt-3">
        <div class="text-xs text-base-content/70 mb-1">查询结果：</div>
        <DataTable data={message.data} />
      </div>
    {/if}
  </div>

  <!-- 时间戳 -->
  <div class="chat-footer opacity-50">
    <time class="text-xs">{formatTimestamp(message.timestamp)}</time>
    {#if message.tokensUsed}
      <span class="text-xs ml-2">Tokens: {message.tokensUsed}</span>
    {/if}
  </div>
</div>
```

#### 4. 状态管理

```typescript
// web/src/lib/stores/chatStore.ts
import { writable } from 'svelte/store';

export interface ChatMessage {
  id?: string;
  role: 'user' | 'assistant';
  content: string;
  sql?: string;
  data?: any[];
  chartConfig?: any;
  tokensUsed?: number;
  timestamp: Date;
  isError?: boolean;
}

export interface ChatState {
  sessionId: string;
  messages: ChatMessage[];
  currentQuery: {
    sql: string;
    data: any[];
    chartConfig: any;
  } | null;
}

function createChatStore() {
  const { subscribe, set, update } = writable<ChatState>({
    sessionId: '',
    messages: [],
    currentQuery: null
  });

  return {
    subscribe,
    
    // 设置会话ID
    setSessionId: (sessionId: string) => update(state => ({
      ...state,
      sessionId
    })),
    
    // 设置消息列表
    setMessages: (messages: ChatMessage[]) => update(state => ({
      ...state,
      messages
    })),
    
    // 添加消息
    addMessage: (message: ChatMessage) => update(state => ({
      ...state,
      messages: [...state.messages, { 
        ...message, 
        id: crypto.randomUUID() 
      }],
      // 如果是AI消息且包含查询结果，更新当前查询
      currentQuery: message.role === 'assistant' && message.data 
        ? {
            sql: message.sql || '',
            data: message.data,
            chartConfig: message.chartConfig
          }
        : state.currentQuery
    })),
    
    // 清除消息
    clearMessages: () => update(state => ({
      ...state,
      messages: [],
      currentQuery: null
    })),
    
    // 更新当前查询
    setCurrentQuery: (query: ChatState['currentQuery']) => update(state => ({
      ...state,
      currentQuery: query
    }))
  };
}

export const chatStore = createChatStore();
```

## 🧪 测试策略详细设计

### 1. 单元测试

```go
// internal/service/chat_service_test.go
package service

import (
    "context"
    "testing"
    "time"
    
    "chat2sql/internal/llm"
    "chat2sql/internal/model"
    "chat2sql/internal/repository/mocks"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestChatService_ProcessChat(t *testing.T) {
    // 创建Mock依赖
    mockLLM := &llm.MockProvider{}
    mockMessageRepo := &mocks.MessageRepository{}
    mockSessionRepo := &mocks.SessionRepository{}
    mockCacheRepo := &mocks.CacheRepository{}
    mockSQLService := &MockSQLService{}
    mockChartService := &MockChartService{}
    
    // 创建服务实例
    chatService := NewChatService(
        mockLLM,
        mockMessageRepo,
        mockSessionRepo,
        mockCacheRepo,
        mockSQLService,
        mockChartService,
    )
    
    t.Run("成功处理聊天请求", func(t *testing.T) {
        // 准备测试数据
        req := &model.ChatRequest{
            Message:   "查询所有用户",
            SessionID: "test-session-id",
        }
        
        // 设置Mock预期
        mockSessionRepo.On("GetByID", mock.Anything, "test-session-id").
            Return(&model.ChatSession{ID: "test-session-id"}, nil)
        
        mockMessageRepo.On("Create", mock.Anything, mock.AnythingOfType("*model.ChatMessage")).
            Return(nil)
        
        mockMessageRepo.On("GetBySessionID", mock.Anything, "test-session-id", 10).
            Return([]*model.ChatMessage{}, nil)
        
        mockCacheRepo.On("Get", mock.Anything, mock.AnythingOfType("string")).
            Return(nil, errors.New("not found"))
        
        mockSQLService.On("GetDatabaseSchema", mock.Anything).
            Return(&llm.DatabaseSchema{Tables: []llm.TableInfo{}}, nil)
        
        mockLLM.On("GenerateSQL", mock.Anything, mock.AnythingOfType("*llm.SQLGenerateRequest")).
            Return(&llm.SQLGenerateResponse{
                SQL:         "SELECT * FROM users",
                Explanation: "查询所有用户信息",
                TokensUsed:  50,
                Latency:     time.Millisecond * 500,
            }, nil)
        
        mockSQLService.On("ExecuteQuery", mock.Anything, "SELECT * FROM users").
            Return(&model.QueryResult{
                Data: []map[string]interface{}{
                    {"id": 1, "name": "张三"},
                    {"id": 2, "name": "李四"},
                },
            }, nil)
        
        mockChartService.On("GenerateChartConfig", mock.Anything, mock.AnythingOfType("*model.QueryResult")).
            Return(map[string]interface{}{"type": "table"}, nil)
        
        mockCacheRepo.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*model.CacheData")).
            Return(nil)
        
        // 执行测试
        response, err := chatService.ProcessChat(context.Background(), req)
        
        // 验证结果
        assert.NoError(t, err)
        assert.NotNil(t, response)
        assert.Equal(t, "SELECT * FROM users", response.SQL)
        assert.Equal(t, 2, len(response.Data))
        assert.Equal(t, 50, response.TokensUsed)
        assert.False(t, response.Cached)
        
        // 验证Mock调用
        mockLLM.AssertExpectations(t)
        mockMessageRepo.AssertExpectations(t)
        mockSessionRepo.AssertExpectations(t)
    })
    
    t.Run("处理缓存命中情况", func(t *testing.T) {
        // 测试缓存逻辑
        req := &model.ChatRequest{
            Message:   "查询所有用户",
            SessionID: "test-session-id",
        }
        
        cachedData := &model.CacheData{
            SQL: "SELECT * FROM users",
            Result: &model.QueryResult{
                Data: []map[string]interface{}{
                    {"id": 1, "name": "张三"},
                },
            },
            ChartConfig: map[string]interface{}{"type": "table"},
        }
        
        mockSessionRepo.On("GetByID", mock.Anything, "test-session-id").
            Return(&model.ChatSession{ID: "test-session-id"}, nil)
        
        mockMessageRepo.On("Create", mock.Anything, mock.AnythingOfType("*model.ChatMessage")).
            Return(nil)
        
        mockCacheRepo.On("Get", mock.Anything, mock.AnythingOfType("string")).
            Return(cachedData, nil)
        
        response, err := chatService.ProcessChat(context.Background(), req)
        
        assert.NoError(t, err)
        assert.True(t, response.Cached)
        assert.Equal(t, "SELECT * FROM users", response.SQL)
    })
}
```

### 2. 集成测试

```go
// tests/integration/chat_api_test.go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    
    "chat2sql/internal/handler"
    "chat2sql/internal/model"
    
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestChatAPI_Integration(t *testing.T) {
    // 设置测试环境
    gin.SetMode(gin.TestMode)
    
    // 创建测试路由
    router := setupTestRouter()
    
    t.Run("完整聊天流程测试", func(t *testing.T) {
        // 1. 创建会话
        createSessionReq := map[string]string{"title": "测试会话"}
        reqBody, _ := json.Marshal(createSessionReq)
        
        w := httptest.NewRecorder()
        req, _ := http.NewRequest("POST", "/api/v1/session", bytes.NewBuffer(reqBody))
        req.Header.Set("Content-Type", "application/json")
        router.ServeHTTP(w, req)
        
        assert.Equal(t, http.StatusOK, w.Code)
        
        var sessionResp model.SuccessResponse
        err := json.Unmarshal(w.Body.Bytes(), &sessionResp)
        assert.NoError(t, err)
        assert.True(t, sessionResp.Success)
        
        sessionData := sessionResp.Data.(map[string]interface{})
        sessionID := sessionData["id"].(string)
        
        // 2. 发送聊天消息
        chatReq := model.ChatRequest{
            Message:   "查询所有用户信息",
            SessionID: sessionID,
        }
        reqBody, _ = json.Marshal(chatReq)
        
        w = httptest.NewRecorder()
        req, _ = http.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(reqBody))
        req.Header.Set("Content-Type", "application/json")
        router.ServeHTTP(w, req)
        
        assert.Equal(t, http.StatusOK, w.Code)
        
        var chatResp model.SuccessResponse
        err = json.Unmarshal(w.Body.Bytes(), &chatResp)
        assert.NoError(t, err)
        assert.True(t, chatResp.Success)
        
        chatData := chatResp.Data.(map[string]interface{})
        assert.NotEmpty(t, chatData["sql"])
        assert.NotEmpty(t, chatData["content"])
        
        // 3. 获取聊天历史
        w = httptest.NewRecorder()
        req, _ = http.NewRequest("GET", "/api/v1/chat/history/"+sessionID, nil)
        router.ServeHTTP(w, req)
        
        assert.Equal(t, http.StatusOK, w.Code)
        
        var historyResp model.SuccessResponse
        err = json.Unmarshal(w.Body.Bytes(), &historyResp)
        assert.NoError(t, err)
        assert.True(t, historyResp.Success)
        
        historyData := historyResp.Data.([]interface{})
        assert.GreaterOrEqual(t, len(historyData), 2) // 用户消息 + AI回复
    })
}

func setupTestRouter() *gin.Engine {
    // 设置测试数据库和依赖
    // 创建测试路由
    // 返回配置好的路由
    return router
}
```

### 3. E2E测试（Playwright）

```typescript
// tests/e2e/chat.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Chat2SQL E2E测试', () => {
  test.beforeEach(async ({ page }) => {
    // 访问应用首页
    await page.goto('/');
  });

  test('完整对话流程测试', async ({ page }) => {
    // 1. 验证页面加载
    await expect(page.locator('h1')).toContainText('Chat2SQL 智能查询');
    
    // 2. 点击示例问题
    await page.click('text=显示2025年6月各部门工资最高的人');
    
    // 3. 验证输入框已填充
    const messageInput = page.locator('textarea');
    await expect(messageInput).toHaveValue('显示2025年6月各部门工资最高的人');
    
    // 4. 发送消息
    await page.click('button:has-text("发送")');
    
    // 5. 验证加载状态
    await expect(page.locator('text=AI正在思考中...')).toBeVisible();
    
    // 6. 等待响应并验证结果
    await expect(page.locator('.chat-bubble').last()).toContainText('查询结果', { timeout: 10000 });
    
    // 7. 验证SQL代码块显示
    await expect(page.locator('code')).toBeVisible();
    
    // 8. 验证数据表格显示  
    await expect(page.locator('table')).toBeVisible();
    
    // 9. 验证图表显示
    await expect(page.locator('canvas, svg')).toBeVisible();
    
    // 10. 测试复制SQL功能
    await page.click('button:has-text("复制")');
    // 验证复制成功提示
  });

  test('会话管理测试', async ({ page }) => {
    // 1. 创建新会话
    await page.click('button:has-text("新建对话")');
    
    // 2. 验证会话列表更新
    await expect(page.locator('.session-item')).toHaveCount.greaterThanOrEqual(1);
    
    // 3. 发送消息
    await page.fill('textarea', '测试消息');
    await page.click('button:has-text("发送")');
    
    // 4. 等待响应
    await expect(page.locator('.chat-bubble').last()).toContainText('查询结果', { timeout: 10000 });
    
    // 5. 切换到另一个会话
    if (await page.locator('.session-item').count() > 1) {
      await page.click('.session-item:first-child');
      // 验证聊天记录已切换
    }
    
    // 6. 删除会话
    await page.hover('.session-item:first-child');
    await page.click('.delete-session-btn');
    await page.click('button:has-text("确认")');
    
    // 验证会话已删除
  });

  test('错误处理测试', async ({ page }) => {
    // 1. 测试空消息发送
    await page.click('button:has-text("发送")');
    // 验证按钮仍然禁用或显示错误提示
    
    // 2. 测试网络错误情况
    await page.route('/api/v1/chat', route => route.abort());
    
    await page.fill('textarea', '测试网络错误');
    await page.click('button:has-text("发送")');
    
    // 验证错误消息显示
    await expect(page.locator('.chat-bubble-error')).toBeVisible();
    await expect(page.locator('text=处理您的请求时出现了错误')).toBeVisible();
  });

  test('响应式设计测试', async ({ page }) => {
    // 1. 测试桌面端布局
    await page.setViewportSize({ width: 1920, height: 1080 });
    await expect(page.locator('.sidebar')).toBeVisible();
    await expect(page.locator('.chart-panel')).toBeVisible();
    
    // 2. 测试平板端布局
    await page.setViewportSize({ width: 768, height: 1024 });
    // 验证布局自适应
    
    // 3. 测试手机端布局
    await page.setViewportSize({ width: 375, height: 667 });
    // 验证移动端布局
    await expect(page.locator('.sidebar')).toBeHidden();
  });
});
```

## 🚀 部署方案详细设计

### 1. 开发环境部署

```yaml
# docker-compose.dev.yml
version: '3.8'

services:
  # 应用服务
  app:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
    environment:
      - ENV=development
      - DB_TYPE=sqlite
      - DB_DSN=./data/chat2sql_dev.db
      - OLLAMA_BASE_URL=http://ollama:11434
      - LOG_LEVEL=debug
    volumes:
      - .:/app
      - ./data:/app/data
    depends_on:
      - ollama
      - redis
    restart: unless-stopped

  # Ollama AI服务
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ./ollama-data:/root/.ollama
    environment:
      - OLLAMA_HOST=0.0.0.0
    restart: unless-stopped
    
  # 初始化Ollama模型
  ollama-init:
    image: ollama/ollama:latest
    depends_on:
      - ollama
    volumes:
      - ./scripts:/scripts
    command: /scripts/init-ollama.sh
    restart: "no"

  # Redis缓存
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    restart: unless-stopped

  # 数据库管理工具
  adminer:
    image: adminer:latest
    ports:
      - "8081:8080"
    environment:
      - ADMINER_DEFAULT_SERVER=app
    restart: unless-stopped

  # 前端开发服务器
  web-dev:
    build:
      context: ./web
      dockerfile: Dockerfile.dev  
    ports:
      - "5173:5173"
    volumes:
      - ./web:/app
      - /app/node_modules
    environment:
      - VITE_API_BASE_URL=http://localhost:8080/api/v1
    restart: unless-stopped

volumes:
  redis-data:
  postgres-data:

networks:
  default:
    name: chat2sql-dev
```

```dockerfile
# Dockerfile.dev
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o chat2sql ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/

COPY --from=builder /app/chat2sql .
COPY --from=builder /app/web/dist ./web/dist

CMD ["./chat2sql"]
```

```bash
#!/bin/bash
# scripts/init-ollama.sh
echo "等待Ollama服务启动..."
sleep 10

echo "拉取DeepSeek R1模型..."
ollama pull deepseek-r1:7b

echo "拉取备用模型..."
ollama pull llama3.2:3b

echo "模型初始化完成"
```

### 2. 生产环境部署

```yaml
# docker-compose.prod.yml  
version: '3.8'

services:
  # 负载均衡器
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf
      - ./nginx/ssl:/etc/nginx/ssl
      - ./web/dist:/usr/share/nginx/html
    depends_on:
      - app-1
      - app-2
    restart: unless-stopped

  # 应用实例1
  app-1:
    image: chat2sql:latest
    environment:
      - ENV=production
      - DB_TYPE=postgres
      - DB_DSN=postgres://user:pass@postgres:5432/chat2sql?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - OLLAMA_BASE_URL=http://ollama:11434
      - LOG_LEVEL=info
    depends_on:
      - postgres
      - redis
      - ollama
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "0.5"

  # 应用实例2
  app-2:
    image: chat2sql:latest
    environment:
      - ENV=production
      - DB_TYPE=postgres
      - DB_DSN=postgres://user:pass@postgres:5432/chat2sql?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - OLLAMA_BASE_URL=http://ollama:11434
      - LOG_LEVEL=info
    depends_on:
      - postgres
      - redis
      - ollama
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "0.5"

  # PostgreSQL数据库
  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=chat2sql
      - POSTGRES_USER=chat2sql_user
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init-db.sql
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: "1.0"

  # Redis集群
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis-data:/data
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "0.25"

  # Ollama集群（如果需要高可用）
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-data:/root/.ollama
    environment:
      - OLLAMA_HOST=0.0.0.0
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: "2.0"

  # 监控服务
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    restart: unless-stopped

  # 日志收集
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    volumes:
      - ./monitoring/loki-config.yaml:/etc/loki/local-config.yaml
      - loki-data:/loki
    restart: unless-stopped

  # 可视化面板
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
    volumes:
      - grafana-data:/var/lib/grafana
      - ./monitoring/grafana:/etc/grafana/provisioning
    depends_on:
      - prometheus
      - loki
    restart: unless-stopped

volumes:
  postgres-data:
  redis-data:
  ollama-data:
  prometheus-data:
  loki-data:
  grafana-data:

networks:
  default:
    name: chat2sql-prod
```

### 3. Kubernetes部署

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: chat2sql

---
# k8s/configmap.yaml  
apiVersion: v1
kind: ConfigMap
metadata:
  name: chat2sql-config
  namespace: chat2sql
data:
  DB_TYPE: "postgres"
  LOG_LEVEL: "info"
  REDIS_URL: "redis://redis-service:6379"
  OLLAMA_BASE_URL: "http://ollama-service:11434"

---
# k8s/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: chat2sql-secret
  namespace: chat2sql
type: Opaque
stringData:
  DB_DSN: "postgres://user:password@postgres-service:5432/chat2sql?sslmode=disable"
  REDIS_PASSWORD: "your-redis-password"
  JWT_SECRET: "your-jwt-secret"

---
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chat2sql-backend
  namespace: chat2sql
spec:
  replicas: 3
  selector:
    matchLabels:
      app: chat2sql-backend
  template:
    metadata:
      labels:
        app: chat2sql-backend
    spec:
      containers:
      - name: backend
        image: chat2sql:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: chat2sql-config
        - secretRef:
            name: chat2sql-secret
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"  
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5

---
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: chat2sql-backend-service
  namespace: chat2sql
spec:
  selector:
    app: chat2sql-backend
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP

---
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: chat2sql-ingress
  namespace: chat2sql
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - chat2sql.yourdomain.com
    secretName: chat2sql-tls
  rules:
  - host: chat2sql.yourdomain.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: chat2sql-backend-service
            port:
              number: 80
      - path: /
        pathType: Prefix
        backend:
          service:
            name: chat2sql-frontend-service
            port:
              number: 80

---
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: chat2sql-backend-hpa
  namespace: chat2sql
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: chat2sql-backend
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 📈 性能优化策略

### 1. 后端性能优化

```go
// internal/middleware/cache.go - 智能缓存中间件
package middleware

import (
    "crypto/sha256"
    "fmt"
    "time"
    
    "github.com/gin-gonic/gin"
)

func IntelligentCache() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 只对特定端点启用缓存
        if c.Request.Method != "POST" || c.Request.URL.Path != "/api/v1/chat" {
            c.Next()
            return
        }
        
        // 读取请求体生成缓存键
        body, err := io.ReadAll(c.Request.Body)
        if err != nil {
            c.Next()
            return
        }
        
        // 恢复请求体供后续使用
        c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
        
        // 生成缓存键
        hash := sha256.Sum256(body)
        cacheKey := fmt.Sprintf("chat_response_%x", hash)
        
        // 尝试从缓存获取
        if cached, exists := cache.Get(cacheKey); exists {
            c.JSON(200, cached)
            c.Header("X-Cache", "HIT")
            c.Abort()
            return
        }
        
        // 继续处理请求
        c.Next()
        
        // 如果响应成功，缓存结果
        if c.Writer.Status() == 200 {
            response := c.GetString("response")
            cache.Set(cacheKey, response, 24*time.Hour)
        }
    }
}

// internal/service/performance.go - 性能优化服务
package service

type PerformanceOptimizer struct {
    queryCache     map[string]*CachedQuery
    connectionPool *sql.DB
    rateLimiter    *rate.Limiter
}

type CachedQuery struct {
    Result    interface{}
    ExpiresAt time.Time
    HitCount  int64
}

func (po *PerformanceOptimizer) OptimizeQuery(ctx context.Context, sql string) (interface{}, error) {
    // 1. 查询缓存检查
    if cached, ok := po.queryCache[sql]; ok && time.Now().Before(cached.ExpiresAt) {
        atomic.AddInt64(&cached.HitCount, 1)
        return cached.Result, nil
    }
    
    // 2. 限流检查
    if !po.rateLimiter.Allow() {
        return nil, errors.New("rate limit exceeded")
    }
    
    // 3. 查询优化
    optimizedSQL := po.optimizeSQL(sql)
    
    // 4. 执行查询
    result, err := po.executeOptimizedQuery(ctx, optimizedSQL)
    if err != nil {
        return nil, err
    }
    
    // 5. 缓存结果
    po.queryCache[sql] = &CachedQuery{
        Result:    result,
        ExpiresAt: time.Now().Add(time.Hour),
        HitCount:  1,
    }
    
    return result, nil
}

func (po *PerformanceOptimizer) optimizeSQL(sql string) string {
    // SQL优化逻辑
    // 1. 添加LIMIT子句（如果没有）
    // 2. 优化JOIN顺序
    // 3. 添加必要的索引提示
    
    if !strings.Contains(strings.ToUpper(sql), "LIMIT") {
        sql += " LIMIT 1000" // 默认限制返回1000条记录
    }
    
    return sql
}
```

### 2. 前端性能优化

```typescript
// web/src/lib/utils/performance.ts
export class PerformanceOptimizer {
  private queryCache = new Map<string, CachedResponse>();
  private debounceTimers = new Map<string, NodeJS.Timeout>();
  
  // 防抖处理用户输入
  debounce<T extends (...args: any[]) => void>(func: T, delay: number, key: string): T {
    return ((...args: any[]) => {
      const existingTimer = this.debounceTimers.get(key);
      if (existingTimer) {
        clearTimeout(existingTimer);
      }
      
      const timer = setTimeout(() => {
        func.apply(this, args);
        this.debounceTimers.delete(key);
      }, delay);
      
      this.debounceTimers.set(key, timer);
    }) as T;
  }
  
  // 智能缓存查询结果
  async cachedFetch(url: string, options?: RequestInit): Promise<Response> {
    const cacheKey = this.generateCacheKey(url, options);
    const cached = this.queryCache.get(cacheKey);
    
    // 检查缓存是否有效
    if (cached && Date.now() < cached.expiresAt) {
      return new Response(JSON.stringify(cached.data), {
        status: 200,
        headers: { 'Content-Type': 'application/json', 'X-Cache': 'HIT' }
      });
    }
    
    // 发起请求
    const response = await fetch(url, options);
    
    // 缓存成功响应
    if (response.ok) {
      const data = await response.clone().json();
      this.queryCache.set(cacheKey, {
        data,
        expiresAt: Date.now() + 24 * 60 * 60 * 1000 // 24小时
      });
    }
    
    return response;
  }
  
  // 虚拟滚动优化大量消息
  createVirtualScroller(container: HTMLElement, items: any[], itemHeight: number) {
    const visibleStart = Math.floor(container.scrollTop / itemHeight);
    const visibleEnd = Math.min(
      visibleStart + Math.ceil(container.clientHeight / itemHeight) + 1,
      items.length
    );
    
    return {
      visibleItems: items.slice(visibleStart, visibleEnd),
      offsetY: visibleStart * itemHeight,
      totalHeight: items.length * itemHeight
    };
  }
  
  private generateCacheKey(url: string, options?: RequestInit): string {
    const key = url + (options?.body ? JSON.stringify(options.body) : '');
    return btoa(key).replace(/[/+=]/g, '');
  }
}
```

## ⚠️ 风险评估与应对

### 1. 技术风险

| 风险项 | 概率 | 影响 | 应对策略 |
|--------|------|------|----------|
| **LLM生成SQL准确率低** | 中 | 高 | 1. 多轮提示词优化<br>2. SQL语法验证<br>3. 人工审核机制<br>4. 回滚机制 |
| **Ollama服务不稳定** | 中 | 中 | 1. 服务健康检查<br>2. 自动重启机制<br>3. 多实例部署<br>4. 备用API接口 |
| **数据库查询性能问题** | 低 | 中 | 1. 查询超时控制<br>2. 结果集大小限制<br>3. 索引优化<br>4. 只读副本 |
| **前端兼容性问题** | 低 | 低 | 1. 浏览器兼容性测试<br>2. Polyfill支持<br>3. 渐进式增强 |

### 2. 业务风险

| 风险项 | 概率 | 影响 | 应对策略 |
|--------|------|------|----------|
| **数据安全泄露** | 低 | 高 | 1. 数据脱敏处理<br>2. 权限控制<br>3. 审计日志<br>4. 数据加密 |
| **SQL注入攻击** | 中 | 高 | 1. 参数化查询<br>2. SQL语法检查<br>3. 白名单机制<br>4. WAF防护 |
| **成本超支** | 中 | 中 | 1. 使用配额控制<br>2. 成本监控告警<br>3. 缓存策略<br>4. 模型选择优化 |
| **用户体验差** | 中 | 中 | 1. 响应时间优化<br>2. 错误处理优化<br>3. 用户反馈收集<br>4. A/B测试 |

### 3. 项目风险

| 风险项 | 概率 | 影响 | 应对策略 |
|--------|------|------|----------|
| **开发进度延期** | 中 | 中 | 1. 敏捷开发方法<br>2. 里程碑管控<br>3. 风险预警机制<br>4. 资源弹性调配 |
| **团队技能不足** | 低 | 中 | 1. 技术培训计划<br>2. 专家顾问支持<br>3. 知识文档完善<br>4. 代码审查制度 |
| **需求变更频繁** | 高 | 中 | 1. 需求变更控制流程<br>2. 原型验证<br>3. 用户参与设计<br>4. 迭代式开发 |

### 4. 应急响应预案

```go
// internal/emergency/response.go
package emergency

import (
    "context"
    "log"
    "time"
)

type EmergencyResponseSystem struct {
    alerts     chan Alert
    handlers   map[AlertType]AlertHandler
    fallbacks  map[string]FallbackStrategy
}

type Alert struct {
    Type      AlertType
    Level     AlertLevel
    Message   string
    Timestamp time.Time
    Context   map[string]interface{}
}

type AlertType string
const (
    AlertTypeLLMFailure     AlertType = "llm_failure"
    AlertTypeDBFailure      AlertType = "db_failure"
    AlertTypeHighLatency    AlertType = "high_latency"
    AlertTypeHighErrorRate  AlertType = "high_error_rate"
)

func (ers *EmergencyResponseSystem) HandleAlert(alert Alert) {
    switch alert.Type {
    case AlertTypeLLMFailure:
        // 切换到备用LLM服务
        ers.switchToFallbackLLM()
        
    case AlertTypeDBFailure:
        // 切换到只读模式
        ers.enableReadOnlyMode()
        
    case AlertTypeHighLatency:
        // 启用降级服务
        ers.enableGracefulDegradation()
        
    case AlertTypeHighErrorRate:
        // 暂停新请求处理
        ers.pauseNewRequests()
    }
}

func (ers *EmergencyResponseSystem) switchToFallbackLLM() {
    // 实现LLM切换逻辑
    log.Println("切换到备用LLM服务")
}

func (ers *EmergencyResponseSystem) enableReadOnlyMode() {
    // 实现只读模式
    log.Println("启用只读模式")
}
```

## 📋 交付清单

### 1. 代码交付物
- [ ] 完整的Go后端代码
- [ ] 完整的Svelte前端代码
- [ ] 数据库迁移脚本
- [ ] Docker配置文件
- [ ] 单元测试和集成测试
- [ ] API文档（OpenAPI）

### 2. 部署交付物
- [ ] 开发环境部署指南
- [ ] 生产环境部署指南
- [ ] Kubernetes部署配置
- [ ] 监控和日志配置
- [ ] 备份和恢复方案

### 3. 文档交付物
- [ ] 技术架构文档
- [ ] 用户操作手册
- [ ] 运维手册
- [ ] 故障排除指南
- [ ] 性能优化指南

### 4. 验收标准
- [ ] 功能完整性验收（90%+用例通过）
- [ ] 性能指标验收（响应时间<10s）
- [ ] 安全检查验收（安全扫描通过）
- [ ] 兼容性验收（主流浏览器兼容）
- [ ] 压力测试验收（100并发用户）

---

**下一步行动**：开始MVP核心功能开发，预计3周内完成基础版本交付。

*本文档由技术架构团队维护，版本：v1.0，最后更新：2025-01-08*