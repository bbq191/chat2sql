# 🚀 Chat2SQL API 文档

## 📋 概述

Chat2SQL AI服务提供自然语言转SQL的RESTful API接口，支持多模型路由、智能降级、成本控制等企业级功能。

## 🎯 快速开始

### 环境准备

1. **设置环境变量**
```bash
# 复制环境配置
cp .env.example .env

# 编辑配置文件，填入真实API密钥
nano .env
```

2. **启动服务**
```bash
# 开发模式
go run cmd/server/main.go

# 或编译后运行
go build -o chat2sql cmd/server/main.go
./chat2sql
```

3. **验证部署**
```bash
# 健康检查
curl http://localhost:8080/health

# 配置验证
go run cmd/llm-test/main.go --config-only
```

## 🔧 核心API端点

### 1. 自然语言转SQL

**POST** `/api/v1/ai/chat2sql`

将自然语言查询转换为SQL语句。

#### 请求示例
```bash
curl -X POST http://localhost:8080/api/v1/ai/chat2sql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "query": "查询所有用户信息",
    "connection_id": 1,
    "user_id": 123
  }'
```

#### 请求参数
```json
{
  "query": "自然语言查询内容",
  "connection_id": 1,
  "user_id": 123,
  "options": {
    "include_explanation": true,
    "max_results": 100,
    "timeout": 30
  }
}
```

#### 响应示例
```json
{
  "success": true,
  "data": {
    "sql": "SELECT * FROM users",
    "confidence": 0.95,
    "explanation": "查询users表中的所有记录",
    "processing_time_ms": 1250,
    "tokens_used": 156,
    "model_used": "gpt-4o-mini",
    "cost_cents": 0.5
  },
  "timestamp": "2025-08-12T10:30:00Z",
  "request_id": "req_abc123"
}
```

### 2. 流式SQL生成

**POST** `/api/v1/ai/chat2sql/stream`

支持WebSocket或SSE的流式SQL生成。

#### WebSocket连接
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ai/chat2sql/stream');

ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Stream data:', data);
};

// 发送查询
ws.send(JSON.stringify({
  query: "统计用户总数",
  connection_id: 1,
  user_id: 123
}));
```

#### 流式响应格式
```json
{"type": "start", "timestamp": "2025-08-12T10:30:00Z"}
{"type": "progress", "progress": 0.3, "message": "分析查询意图..."}
{"type": "chunk", "content": "SELECT COUNT(*) "}
{"type": "chunk", "content": "FROM users"}
{"type": "result", "sql": "SELECT COUNT(*) FROM users", "confidence": 0.92}
{"type": "complete", "processing_time_ms": 2100}
```

### 3. 查询反馈

**POST** `/api/v1/ai/feedback`

提交SQL生成结果的反馈，用于模型优化。

#### 请求示例
```bash
curl -X POST http://localhost:8080/api/v1/ai/feedback \
  -H "Content-Type: application/json" \
  -d '{
    "query_id": "req_abc123",
    "user_query": "查询所有用户",
    "generated_sql": "SELECT * FROM users", 
    "is_correct": true,
    "user_rating": 5,
    "feedback": "生成的SQL完全正确"
  }'
```

### 4. 成本监控

**GET** `/api/v1/ai/usage`

查询AI使用量和成本统计。

#### 响应示例
```json
{
  "user_id": 123,
  "daily_usage": {
    "date": "2025-08-12",
    "queries_count": 45,
    "tokens_used": 12560,
    "cost_usd": 2.35,
    "remaining_budget": 7.65
  },
  "model_usage": {
    "gpt-4o-mini": {"queries": 30, "cost": 1.20},
    "claude-3-haiku": {"queries": 15, "cost": 1.15}
  }
}
```

## 🛡️ 认证与安全

### JWT认证
所有API请求需要包含有效的JWT token：

```bash
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 请求限流
- **速率限制**: 100 请求/分钟/用户
- **并发限制**: 10 个并发请求/用户
- **成本限制**: $10 日预算/用户

### SQL安全
- ✅ 只允许SELECT查询
- ❌ 禁止DELETE/UPDATE/INSERT/DROP操作
- 🔍 125+安全检查规则
- 🛡️ 自动SQL注入防护

## 📊 错误处理

### 错误响应格式
```json
{
  "success": false,
  "error": {
    "code": "INVALID_QUERY",
    "message": "查询包含禁止的操作",
    "details": {
      "forbidden_operation": "DELETE",
      "suggestion": "请使用SELECT查询"
    }
  },
  "request_id": "req_abc123",
  "timestamp": "2025-08-12T10:30:00Z"
}
```

### 常见错误码
| 错误码 | 描述 | 解决方案 |
|--------|------|----------|
| `INVALID_QUERY` | 查询包含非法操作 | 使用SELECT查询 |
| `BUDGET_EXCEEDED` | 超出预算限制 | 等待预算重置或联系管理员 |
| `RATE_LIMITED` | 请求过于频繁 | 减少请求频率 |
| `LLM_UNAVAILABLE` | AI模型不可用 | 稍后重试或使用备用模型 |
| `INVALID_TOKEN` | 认证token无效 | 刷新token或重新登录 |

## 🔧 模型配置

### 支持的模型提供商
- **OpenAI**: gpt-4o, gpt-4o-mini, gpt-3.5-turbo
- **Anthropic**: claude-3-opus, claude-3-sonnet, claude-3-haiku  
- **本地模型**: Ollama支持的模型

### 智能路由策略
```mermaid
graph TD
    A[用户查询] --> B{查询复杂度}
    B -->|简单| C[GPT-4o-mini]
    B -->|中等| D[Claude-3-haiku]
    B -->|复杂| E[GPT-4o]
    C --> F{成功?}
    D --> F
    E --> F
    F -->|失败| G[降级到备用模型]
    F -->|成功| H[返回结果]
```

## 🧪 测试工具

### 1. 配置验证
```bash
# 检查配置
go run cmd/llm-test/main.go --config-only

# 测试特定提供商
go run cmd/llm-test/main.go --provider openai
```

### 2. 集成测试
```bash
# 查看测试用例
go run cmd/ai-integration-test/main.go --dry-run

# 运行Mock测试
go run cmd/ai-integration-test/main.go

# 测试真实API
go run cmd/ai-integration-test/main.go --api-test
```

### 3. 性能测试
```bash
# 轻量级测试
go run cmd/performance-test/main.go -c 10 -d 30

# 压力测试
go run cmd/performance-test/main.go -c 50 -d 300 --report
```

## 📈 监控指标

### Prometheus指标
服务暴露以下Prometheus指标 (`:9090/metrics`):

- `chat2sql_requests_total`: 总请求数
- `chat2sql_request_duration_seconds`: 请求耗时
- `chat2sql_sql_generation_accuracy`: SQL生成准确率
- `chat2sql_token_usage_total`: Token使用量
- `chat2sql_cost_total`: 总成本
- `chat2sql_model_availability`: 模型可用性

### 健康检查
- **HTTP**: `GET /health`
- **AI模块**: `GET /health/ai`
- **数据库**: `GET /health/db`

## 🎯 性能基准

| 指标 | P1阶段目标 | 生产级目标 |
|------|------------|------------|
| SQL生成准确率 | >70% | >95% |
| API响应时间P95 | <3s | <500ms |
| 系统可用性 | >99% | >99.9% |
| 并发处理能力 | >50 QPS | >500 QPS |
| Token消耗效率 | <$0.01/查询 | <$0.005/查询 |

## 🔄 版本更新

### 当前版本: P1.3 (2025-08-12)
- ✅ LangChainGo v0.1.13集成
- ✅ 多模型智能路由
- ✅ 实时成本监控
- ✅ 企业级性能优化
- ✅ 125+安全检查规则

### 路线图: P2阶段
- 🚧 向量缓存优化
- 🚧 多租户架构
- 🚧 高级语义理解
- 🚧 自动模型选择

## 📞 技术支持

- **文档**: `/docs/`
- **示例**: `/examples/`
- **GitHub**: [chat2sql-go](https://github.com/your-org/chat2sql-go)
- **API状态**: [status.chat2sql.com](https://status.chat2sql.com)