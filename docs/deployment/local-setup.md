# 🏠 Chat2SQL 本地部署指南

## 📋 系统要求

### 最低配置
- **操作系统**: Linux/macOS/Windows 10+
- **Go版本**: 1.24+
- **内存**: 4GB RAM
- **CPU**: 2核心
- **磁盘**: 1GB可用空间

### 推荐配置
- **操作系统**: Linux (Ubuntu 20.04+/CentOS 8+)
- **Go版本**: 1.24.5+
- **内存**: 8GB+ RAM
- **CPU**: 4核心+
- **磁盘**: 10GB+ SSD

## 🚀 快速部署

### 1. 克隆项目
```bash
git clone https://github.com/your-org/chat2sql-go.git
cd chat2sql-go
```

### 2. 环境配置
```bash
# 复制环境配置模板
cp .env.example .env

# 编辑配置文件
nano .env
```

#### 必需的环境变量
```bash
# OpenAI配置（主要模型）
OPENAI_API_KEY=sk-your-openai-api-key-here
OPENAI_MODEL=gpt-4o-mini

# Anthropic配置（备用模型）
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key-here
ANTHROPIC_MODEL=claude-3-haiku-20240307

# 本地模型配置（可选）
OLLAMA_SERVER_URL=http://localhost:11434
OLLAMA_MODEL=deepseek-r1:7b
```

### 3. 依赖安装
```bash
# 下载Go依赖
go mod download

# 验证依赖
go mod verify
```

### 4. 编译验证
```bash
# 编译所有模块
go build ./...

# 运行测试
go test ./...
```

### 5. 配置验证
```bash
# 验证LLM配置
go run cmd/llm-test/main.go --config-only

# 输出示例:
# ✅ 配置加载成功
#    - 主要模型: openai (gpt-4o-mini)
#    - 备用模型: anthropic (claude-3-haiku-20240307)
#    - 本地模型: ollama (deepseek-r1:7b)
```

### 6. 启动服务
```bash
# 开发模式启动
go run cmd/server/main.go

# 或编译后启动
go build -o chat2sql cmd/server/main.go
./chat2sql
```

## 🧪 功能测试

### 1. 基础功能验证
```bash
# 健康检查
curl http://localhost:8080/health

# 应返回: {"status": "ok", "timestamp": "..."}
```

### 2. AI集成测试
```bash
# Mock模式测试（无需API密钥）
go run cmd/ai-integration-test/main.go

# 真实API测试（需要API密钥）
go run cmd/ai-integration-test/main.go --api-test
```

### 3. 性能基准测试
```bash
# 轻量级性能测试
go run cmd/performance-test/main.go -c 10 -d 30

# 预期结果:
# 平均QPS: >50
# P95响应时间: <3000ms
```

## 🔧 本地模型配置（可选）

### Ollama 安装
```bash
# Linux/macOS
curl -fsSL https://ollama.ai/install.sh | sh

# 启动服务
ollama serve

# 下载模型
ollama pull deepseek-r1:7b
```

### 验证本地模型
```bash
# 测试Ollama连接
curl http://localhost:11434/api/version

# 测试模型
go run cmd/llm-test/main.go --provider ollama
```

## 📊 监控配置

### Prometheus集成
```bash
# 启用Prometheus指标
export ENABLE_PROMETHEUS_METRICS=true
export PROMETHEUS_PORT=9090

# 访问指标
curl http://localhost:9090/metrics
```

### 日志配置
```bash
# 设置日志级别
export LOG_LEVEL=info

# 日志输出到文件
export LOG_FILE=./logs/chat2sql.log
```

## 🛠️ 开发环境设置

### VS Code 配置
```json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.formatTool": "goimports"
}
```

### 推荐扩展
- Go (Google)
- REST Client
- GitLens

### 开发工具安装
```bash
# 代码格式化
go install golang.org/x/tools/cmd/goimports@latest

# 代码检查
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# API文档生成
go install github.com/swaggo/swag/cmd/swag@latest
```

## 🔍 故障排查

### 常见问题

#### 1. API密钥配置问题
```bash
# 症状: LLM配置加载失败
# 解决: 检查.env文件中的API密钥

# 验证密钥格式
echo $OPENAI_API_KEY | grep -E "^sk-[A-Za-z0-9]{48}$"
```

#### 2. 网络连接问题
```bash
# 测试OpenAI连接
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models

# 测试Anthropic连接  
curl -H "x-api-key: $ANTHROPIC_API_KEY" \
     https://api.anthropic.com/v1/messages
```

#### 3. 内存不足
```bash
# 检查内存使用
free -h

# 调整Go垃圾回收
export GOGC=100
export GOMEMLIMIT=2GiB
```

#### 4. 端口冲突
```bash
# 检查端口占用
lsof -i :8080

# 修改服务端口
export SERVER_PORT=8081
```

### 调试模式
```bash
# 启用详细日志
export LOG_LEVEL=debug

# 启用Go race检测
go run -race cmd/server/main.go

# 启用pprof性能分析
export ENABLE_PPROF=true
# 访问: http://localhost:6060/debug/pprof/
```

## 📈 性能优化

### 系统级优化
```bash
# 增加文件描述符限制
ulimit -n 65536

# 优化网络参数
echo 'net.core.somaxconn = 1024' >> /etc/sysctl.conf
sysctl -p
```

### 应用级优化
```bash
# 调整工作线程数
export AI_WORKERS=8  # 建议CPU核心数*2

# 启用连接池优化
export MAX_IDLE_CONNS=100
export MAX_IDLE_CONNS_PER_HOST=10

# 启用缓存
export ENABLE_AI_CACHE=true
export AI_CACHE_TTL_MINUTES=5
```

## 🔄 更新部署

### 代码更新
```bash
# 拉取最新代码
git pull origin main

# 更新依赖
go mod tidy

# 重新编译
go build -o chat2sql cmd/server/main.go

# 优雅重启服务
kill -USR2 $(pidof chat2sql)
```

### 配置更新
```bash
# 备份当前配置
cp .env .env.backup

# 比较配置差异
diff .env.example .env

# 应用新配置
systemctl reload chat2sql
```

## 📋 部署检查清单

### 部署前检查
- [ ] Go 1.24+ 已安装
- [ ] API密钥已配置
- [ ] 网络连接正常
- [ ] 依赖已下载
- [ ] 编译无错误

### 部署后验证
- [ ] 服务启动成功
- [ ] 健康检查通过
- [ ] API响应正常
- [ ] 日志输出正常
- [ ] 监控指标可见

### 生产就绪检查
- [ ] 性能测试达标
- [ ] 安全配置完成
- [ ] 监控告警配置
- [ ] 备份恢复测试
- [ ] 故障演练完成

## 🆘 技术支持

### 日志收集
```bash
# 收集系统信息
uname -a > system-info.txt
go version >> system-info.txt
cat .env.example >> system-info.txt

# 收集应用日志
tail -n 100 ./logs/chat2sql.log > app-logs.txt

# 收集性能数据
go run cmd/performance-test/main.go --report > perf-report.txt
```

### 联系支持
- **技术文档**: `/docs/`
- **GitHub Issues**: [问题提交](https://github.com/your-org/chat2sql-go/issues)
- **邮件支持**: support@chat2sql.com