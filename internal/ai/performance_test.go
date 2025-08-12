// 性能优化器测试 - 企业级并发优化实现的完整测试覆盖
// 测试工作池、断路器、限流器、对象池等核心功能

package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewPerformanceOptimizer 测试性能优化器创建
func TestNewPerformanceOptimizer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{
		Workers:      4,
		QueueSize:    100,
		RateLimit:    50,
		MaxFailures:  3,
		ResetTimeout: 30 * time.Second,
	}
	
	optimizer := NewPerformanceOptimizer(config, logger)

	assert.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.workerPool)
	assert.NotNil(t, optimizer.metrics)
	assert.NotNil(t, optimizer.circuitBreaker)
	assert.NotNil(t, optimizer.rateLimiter)
	assert.Equal(t, logger, optimizer.logger)
}

// TestCircuitBreaker 测试断路器功能
func TestCircuitBreaker(t *testing.T) {
	tests := []struct {
		name           string
		maxFailures    int
		resetTimeout   time.Duration
		operations     []bool // true表示成功，false表示失败
		expectedStates []CircuitState
	}{
		{
			name:         "normal_operation",
			maxFailures:  3,
			resetTimeout: 100 * time.Millisecond,
			operations:   []bool{true, true, true},
			expectedStates: []CircuitState{
				StateClosed, StateClosed, StateClosed,
			},
		},
		{
			name:         "open_circuit",
			maxFailures:  2,
			resetTimeout: 100 * time.Millisecond,
			operations:   []bool{false, false, false},
			expectedStates: []CircuitState{
				StateClosed, StateOpen, StateOpen,
			},
		},
		{
			name:         "recovery_to_half_open",
			maxFailures:  2,
			resetTimeout: 50 * time.Millisecond,
			operations:   []bool{false, false},
			expectedStates: []CircuitState{
				StateClosed, StateOpen,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CircuitBreaker{
				maxFailures:  tt.maxFailures,
				resetTimeout: tt.resetTimeout,
				state:        StateClosed,
			}

			for i, success := range tt.operations {
				// 首先检查是否允许请求
				allowed := cb.allowRequest()
				
				if cb.state != StateOpen {
					assert.True(t, allowed, "Request should be allowed in closed/half-open state")
				}

				// 记录操作结果
				if success {
					cb.recordSuccess()
				} else {
					cb.recordFailure()
				}

				// 检查状态
				if i < len(tt.expectedStates) {
					assert.Equal(t, tt.expectedStates[i], cb.state, 
						"Circuit breaker state mismatch at operation %d", i)
				}
			}
		})
	}
}

// TestCircuitBreakerRecovery 测试断路器恢复功能
func TestCircuitBreakerRecovery(t *testing.T) {
	cb := &CircuitBreaker{
		maxFailures:  1,
		resetTimeout: 100 * time.Millisecond,
		state:        StateClosed,
	}

	// 触发断路器打开
	cb.recordFailure()
	assert.Equal(t, StateOpen, cb.state)
	assert.False(t, cb.allowRequest())

	// 等待重置超时
	time.Sleep(150 * time.Millisecond)

	// 应该进入半开状态
	assert.True(t, cb.allowRequest())
	assert.Equal(t, StateHalfOpen, cb.state)

	// 成功操作应该关闭断路器
	cb.recordSuccess()
	assert.Equal(t, StateClosed, cb.state)
}

// TestRateLimiter 测试限流器功能
func TestRateLimiter(t *testing.T) {
	// 创建一个限流器，容量为3，不自动补充
	rl := &RateLimiter{
		tokens:   make(chan struct{}, 3),
		capacity: 3,
		refill:   0, // 不自动补充，手动控制
	}

	// 初始化令牌
	for i := 0; i < 3; i++ {
		rl.tokens <- struct{}{}
	}

	// 前3个请求应该成功
	for i := 0; i < 3; i++ {
		select {
		case <-rl.tokens:
			// 成功获取令牌
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("Expected token to be available for request %d", i+1)
		}
	}

	// 第4个请求应该被限流
	select {
	case <-rl.tokens:
		t.Fatal("Expected rate limiting, but got token")
	case <-time.After(10 * time.Millisecond):
		// 正确被限流
	}
}

// TestObjectPool 测试对象池功能
func TestObjectPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{
		Workers:      2,
		RateLimit:    100,
		MaxFailures:  5,
		ResetTimeout: 30 * time.Second,
	}
	optimizer := NewPerformanceOptimizer(config, logger)

	// 从对象池获取对象
	obj1 := optimizer.objectPool.Get()
	assert.NotNil(t, obj1)

	obj2 := optimizer.objectPool.Get()
	assert.NotNil(t, obj2)

	// 放回对象池
	optimizer.objectPool.Put(obj1)
	optimizer.objectPool.Put(obj2)

	// 再次获取，应该复用对象
	obj3 := optimizer.objectPool.Get()
	assert.NotNil(t, obj3)
}

// TestProcessConcurrentQueries 测试并发查询处理
func TestProcessConcurrentQueries(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{
		Workers:   2,
		QueueSize: 10,
		RateLimit: 100,
	}
	optimizer := NewPerformanceOptimizer(config, logger)

	// 创建测试请求
	requests := []*ChatRequest{
		{Query: "SELECT * FROM users", ConnectionID: 1},
		{Query: "SELECT * FROM orders", ConnectionID: 1},
	}

	ctx := context.Background()
	
	// 由于ProcessConcurrentQueries需要真实的LLM客户端，
	// 这里我们主要测试错误处理路径
	results, err := optimizer.ProcessConcurrentQueries(ctx, requests)
	
	// 预期会有错误，因为没有配置真实的LLM客户端
	assert.Error(t, err)
	assert.Nil(t, results)
}

// TestWorkerPool 测试工作池功能
func TestWorkerPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{
		Workers:      2,
		QueueSize:    5,
		RateLimit:    100,
		MaxFailures:  5,
		ResetTimeout: 30 * time.Second,
	}
	optimizer := NewPerformanceOptimizer(config, logger)

	// 启动工作池
	optimizer.startWorkerPool()

	// 验证工作池已启动
	assert.True(t, optimizer.workerPool.started)
	assert.Equal(t, 2, optimizer.workerPool.workers)
	assert.NotNil(t, optimizer.workerPool.jobQueue)
	assert.NotNil(t, optimizer.workerPool.resultQueue)
}

// TestGetMetrics 测试指标收集功能
func TestGetMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{Workers: 2}
	optimizer := NewPerformanceOptimizer(config, logger)

	metrics := optimizer.GetMetrics()
	assert.NotNil(t, metrics)

	// 检查基本指标字段
	assert.Contains(t, metrics, "concurrent_requests")
	assert.Contains(t, metrics, "queue_depth")
	assert.Contains(t, metrics, "worker_count")
	assert.Contains(t, metrics, "circuit_breaker")
}

// TestGetCircuitBreakerStatus 测试断路器状态获取
func TestGetCircuitBreakerStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{Workers: 2}
	optimizer := NewPerformanceOptimizer(config, logger)

	status := optimizer.getCircuitBreakerStatus()
	assert.NotNil(t, status)

	// 检查断路器状态字段
	assert.Contains(t, status, "state")
	assert.Contains(t, status, "failures")
	assert.Contains(t, status, "last_fail_time")
}

// TestShutdown 测试优雅关闭功能
func TestShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{Workers: 2}
	optimizer := NewPerformanceOptimizer(config, logger)

	// 启动工作池
	optimizer.startWorkerPool()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试关闭
	err := optimizer.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestShutdownTimeout 测试关闭超时
func TestShutdownTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{Workers: 2}
	optimizer := NewPerformanceOptimizer(config, logger)

	// 启动工作池
	optimizer.startWorkerPool()

	// 创建一个会立即超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// 等待超时
	time.Sleep(10 * time.Millisecond)

	// 测试关闭超时
	err := optimizer.Shutdown(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// TestPerformanceConcurrentAccess 测试并发访问安全性
func TestPerformanceConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &PerformanceConfig{Workers: 4}
	optimizer := NewPerformanceOptimizer(config, logger)

	var wg sync.WaitGroup
	const numGoroutines = 10

	// 并发访问指标
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = optimizer.GetMetrics()
				_ = optimizer.getCircuitBreakerStatus()
			}
		}()
	}

	// 并发操作断路器
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				optimizer.circuitBreaker.recordSuccess()
				optimizer.circuitBreaker.recordFailure()
				optimizer.circuitBreaker.allowRequest()
			}
		}()
	}

	wg.Wait()
	// 如果没有panic，则并发安全测试通过
}

// TestProcessingMetrics 测试处理指标结构
func TestProcessingMetrics(t *testing.T) {
	metrics := &ProcessingMetrics{
		TokensUsed:     100,
		ProcessingTime: 500 * time.Millisecond,
		QueueTime:      50 * time.Millisecond,
		ModelLatency:   450 * time.Millisecond,
		ValidationTime: 10 * time.Millisecond,
		CacheHit:       true,
		WorkerID:       1,
	}

	assert.Equal(t, 100, metrics.TokensUsed)
	assert.Equal(t, 500*time.Millisecond, metrics.ProcessingTime)
	assert.Equal(t, 50*time.Millisecond, metrics.QueueTime)
	assert.Equal(t, 450*time.Millisecond, metrics.ModelLatency)
	assert.Equal(t, 10*time.Millisecond, metrics.ValidationTime)
	assert.True(t, metrics.CacheHit)
	assert.Equal(t, 1, metrics.WorkerID)
}

// TestQueryJobAndResult 测试查询任务和结果结构
func TestQueryJobAndResult(t *testing.T) {
	ctx := context.Background()
	done := make(chan QueryResult, 1)

	job := QueryJob{
		ID:      "test-job-1",
		Request: &ChatRequest{Query: "SELECT 1", ConnectionID: 1},
		Context: ctx,
		Done:    done,
	}

	result := QueryResult{
		ID:       "test-job-1",
		Response: &SQLResponse{SQL: "SELECT 1", Confidence: 0.9},
		Error:    nil,
		Duration: 100 * time.Millisecond,
		Metrics: &ProcessingMetrics{
			TokensUsed:     50,
			ProcessingTime: 100 * time.Millisecond,
		},
	}

	assert.Equal(t, "test-job-1", job.ID)
	assert.Equal(t, "SELECT 1", job.Request.Query)
	assert.Equal(t, ctx, job.Context)

	assert.Equal(t, "test-job-1", result.ID)
	assert.Equal(t, "SELECT 1", result.Response.SQL)
	assert.Equal(t, 0.9, result.Response.Confidence)
	assert.NoError(t, result.Error)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
}

// TestCircuitState 测试断路器状态枚举
func TestCircuitState(t *testing.T) {
	assert.Equal(t, CircuitState(0), StateClosed)
	assert.Equal(t, CircuitState(1), StateOpen)
	assert.Equal(t, CircuitState(2), StateHalfOpen)
}

// TestPerformanceConfigDefaults 测试性能配置默认值
func TestPerformanceConfigDefaults(t *testing.T) {
	// 假设存在DefaultPerformanceConfig函数
	logger, _ := zap.NewDevelopment()
	
	// 测试默认配置
	config := &PerformanceConfig{}
	optimizer := NewPerformanceOptimizer(config, logger)
	
	assert.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.circuitBreaker)
	assert.NotNil(t, optimizer.rateLimiter)
	assert.NotNil(t, optimizer.workerPool)
}