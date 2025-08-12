package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// TestPerformanceTestConfig_DefaultValues 测试默认性能测试配置
func TestPerformanceTestConfig_DefaultValues(t *testing.T) {
	config, err := loadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	
	assert.Equal(t, "http://localhost:8080", config.BaseURL)
	assert.Equal(t, 50, config.Concurrency)
	assert.Equal(t, 60*time.Second, config.Duration)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 10*time.Second, config.RampUpTime)
	assert.Len(t, config.TestEndpoints, 2)
	
	// 验证默认端点
	healthEndpoint := config.TestEndpoints[0]
	assert.Equal(t, "health", healthEndpoint.Name)
	assert.Equal(t, "GET", healthEndpoint.Method)
	assert.Equal(t, "/health", healthEndpoint.Path)
	assert.Equal(t, 10, healthEndpoint.Weight)
}

// TestLoadConfig_FromFile 测试从文件加载配置
func TestLoadConfig_FromFile(t *testing.T) {
	// 创建临时配置文件
	configData := PerformanceTestConfig{
		BaseURL:        "http://test.example.com:9090",
		Concurrency:    100,
		Duration:       120 * time.Second,
		RequestTimeout: 60 * time.Second,
		RampUpTime:     20 * time.Second,
		TestEndpoints: []TestEndpoint{
			{Name: "api_test", Method: "POST", Path: "/api/test", Weight: 5},
		},
	}
	
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "perf_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	// 写入配置
	encoder := json.NewEncoder(tmpFile)
	err = encoder.Encode(configData)
	assert.NoError(t, err)
	tmpFile.Close()
	
	// 测试加载
	loadedConfig, err := loadConfig(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, configData.BaseURL, loadedConfig.BaseURL)
	assert.Equal(t, configData.Concurrency, loadedConfig.Concurrency)
	assert.Equal(t, configData.Duration, loadedConfig.Duration)
	assert.Len(t, loadedConfig.TestEndpoints, 1)
	assert.Equal(t, "api_test", loadedConfig.TestEndpoints[0].Name)
}

// TestLoadConfig_FileNotFound 测试配置文件不存在的情况
func TestLoadConfig_FileNotFound(t *testing.T) {
	config, err := loadConfig("nonexistent_file.json")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "打开配置文件失败")
}

// TestLoadConfig_InvalidJSON 测试无效JSON配置文件
func TestLoadConfig_InvalidJSON(t *testing.T) {
	// 创建包含无效JSON的临时文件
	tmpFile, err := os.CreateTemp("", "invalid_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	_, err = tmpFile.WriteString(`{"invalid": json}`) // 无效的JSON
	assert.NoError(t, err)
	tmpFile.Close()
	
	config, err := loadConfig(tmpFile.Name())
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "解析配置文件失败")
}

// TestNewPerformanceTester 测试性能测试器创建
func TestNewPerformanceTester(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		BaseURL:        "http://localhost:8080",
		Concurrency:    10,
		Duration:       5 * time.Second,
		RequestTimeout: 10 * time.Second,
		RampUpTime:     2 * time.Second,
		TestEndpoints: []TestEndpoint{
			{Name: "test", Method: "GET", Path: "/test", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	assert.NotNil(t, tester)
	assert.Equal(t, config, tester.config)
	assert.NotNil(t, tester.httpClient)
	assert.NotNil(t, tester.results)
	assert.NotNil(t, tester.ctx)
	assert.Equal(t, config.RequestTimeout, tester.httpClient.Timeout)
}

// TestPerformanceResult_InitialValues 测试性能结果初始值
func TestPerformanceResult_InitialValues(t *testing.T) {
	result := &PerformanceResult{
		EndpointName:   "test_endpoint",
		ErrorsByStatus: make(map[int]int64),
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour,
		MaxLatency:    0,
	}
	
	assert.Equal(t, "test_endpoint", result.EndpointName)
	assert.Equal(t, int64(0), result.TotalRequests)
	assert.Equal(t, int64(0), result.SuccessRequests)
	assert.Equal(t, float64(0), result.SuccessRate)
	assert.Equal(t, time.Hour, result.MinLatency)
	assert.Equal(t, time.Duration(0), result.MaxLatency)
	assert.Empty(t, result.ErrorsByStatus)
	assert.Empty(t, result.Latencies)
}

// TestSelectEndpointByWeight 测试权重选择算法
func TestSelectEndpointByWeight(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "endpoint1", Method: "GET", Path: "/path1", Weight: 10},
			{Name: "endpoint2", Method: "GET", Path: "/path2", Weight: 20},
			{Name: "endpoint3", Method: "GET", Path: "/path3", Weight: 30},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	totalWeight := 60 // 10 + 20 + 30
	
	// 测试多次选择，验证算法正确性
	selections := make(map[string]int)
	for i := 0; i < 1000; i++ {
		endpoint := tester.selectEndpointByWeight(totalWeight)
		selections[endpoint.Name]++
	}
	
	// 验证所有端点都被选择了
	assert.True(t, selections["endpoint1"] > 0, "endpoint1 should be selected")
	assert.True(t, selections["endpoint2"] > 0, "endpoint2 should be selected")
	assert.True(t, selections["endpoint3"] > 0, "endpoint3 should be selected")
	
	// 权重更高的端点应该被选择更多次（但由于随机性，我们不做严格验证）
}

// TestSelectEndpointByWeight_SingleEndpoint 测试单一端点选择
func TestSelectEndpointByWeight_SingleEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "only_endpoint", Method: "GET", Path: "/path", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	endpoint := tester.selectEndpointByWeight(1)
	
	assert.Equal(t, "only_endpoint", endpoint.Name)
}

// TestRecordSuccess 测试成功记录
func TestRecordSuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "test", Method: "GET", Path: "/test", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test"] = &PerformanceResult{
		EndpointName:   "test",
		ErrorsByStatus: make(map[int]int64),
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour,
		MaxLatency:    0,
	}
	
	latency := 100 * time.Millisecond
	tester.recordSuccess("test", latency)
	
	result := tester.results["test"]
	assert.Equal(t, int64(1), result.TotalRequests)
	assert.Equal(t, int64(1), result.SuccessRequests)
	assert.Equal(t, latency, result.TotalLatency)
	assert.Equal(t, latency, result.MinLatency)
	assert.Equal(t, latency, result.MaxLatency)
	assert.Len(t, result.Latencies, 1)
	assert.Equal(t, latency, result.Latencies[0])
}

// TestRecordError 测试错误记录
func TestRecordError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "test", Method: "GET", Path: "/test", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test"] = &PerformanceResult{
		EndpointName:   "test",
		ErrorsByStatus: make(map[int]int64),
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour,
		MaxLatency:    0,
	}
	
	latency := 200 * time.Millisecond
	statusCode := 500
	tester.recordError("test", statusCode, latency)
	
	result := tester.results["test"]
	assert.Equal(t, int64(1), result.TotalRequests)
	assert.Equal(t, int64(0), result.SuccessRequests)
	assert.Equal(t, int64(1), result.ErrorRequests)
	assert.Equal(t, latency, result.TotalLatency)
	assert.Equal(t, int64(1), result.ErrorsByStatus[statusCode])
	assert.Len(t, result.Latencies, 1)
}

// TestCalculateResults 测试结果计算
func TestCalculateResults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test"] = &PerformanceResult{
		EndpointName:   "test",
		TotalRequests:  10,
		SuccessRequests: 8,
		ErrorRequests:  2,
		TotalLatency:   500 * time.Millisecond,
		Latencies: []time.Duration{
			10 * time.Millisecond,
			20 * time.Millisecond,
			30 * time.Millisecond,
			40 * time.Millisecond,
			50 * time.Millisecond,
		},
	}
	
	totalTime := 5 * time.Second
	tester.calculateResults(totalTime)
	
	result := tester.results["test"]
	assert.Equal(t, 80.0, result.SuccessRate)
	assert.Equal(t, 50*time.Millisecond, result.AverageLatency)
	assert.Equal(t, 2.0, result.QPS) // 10 requests in 5 seconds
	
	// 验证百分位数计算
	assert.Equal(t, 30*time.Millisecond, result.P50Latency)
	assert.Equal(t, 50*time.Millisecond, result.P90Latency)
	assert.Equal(t, 50*time.Millisecond, result.P99Latency)
}

// TestCalculatePercentiles 测试百分位数计算
func TestCalculatePercentiles(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{}
	tester := NewPerformanceTester(config, logger)
	
	result := &PerformanceResult{
		Latencies: []time.Duration{
			100 * time.Millisecond,
			50 * time.Millisecond,
			200 * time.Millisecond,
			75 * time.Millisecond,
			150 * time.Millisecond,
			25 * time.Millisecond,
			300 * time.Millisecond,
			125 * time.Millisecond,
			175 * time.Millisecond,
			250 * time.Millisecond,
		},
	}
	
	tester.calculatePercentiles(result)
	
	// 排序后: [25, 50, 75, 100, 125, 150, 175, 200, 250, 300]
	// P50 (50%): 150ms (index 5)
	// P90 (90%): 300ms (index 9)
	// P99 (99%): 300ms (index 9, because 99% of 10 is 9.9, rounded down to 9)
	assert.Equal(t, 150*time.Millisecond, result.P50Latency)
	assert.Equal(t, 300*time.Millisecond, result.P90Latency)
	assert.Equal(t, 300*time.Millisecond, result.P99Latency)
}

// TestCalculatePercentiles_EmptyLatencies 测试空延迟列表的百分位数计算
func TestCalculatePercentiles_EmptyLatencies(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{}
	tester := NewPerformanceTester(config, logger)
	
	result := &PerformanceResult{
		Latencies: []time.Duration{},
	}
	
	// 不应该panic
	assert.NotPanics(t, func() {
		tester.calculatePercentiles(result)
	})
	
	// 百分位数应该保持零值
	assert.Equal(t, time.Duration(0), result.P50Latency)
	assert.Equal(t, time.Duration(0), result.P90Latency)
	assert.Equal(t, time.Duration(0), result.P99Latency)
}

// TestExecuteRequest_Success 测试HTTP请求执行（成功）
func TestExecuteRequest_Success(t *testing.T) {
	// 创建测试HTTP服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()
	
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		BaseURL:        server.URL,
		RequestTimeout: 5 * time.Second,
		TestEndpoints: []TestEndpoint{
			{Name: "test", Method: "GET", Path: "/test", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test"] = &PerformanceResult{
		EndpointName:   "test",
		ErrorsByStatus: make(map[int]int64),
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour,
		MaxLatency:    0,
	}
	
	endpoint := TestEndpoint{Name: "test", Method: "GET", Path: "/test", Weight: 1}
	tester.executeRequest(endpoint)
	
	result := tester.results["test"]
	assert.Equal(t, int64(1), result.TotalRequests)
	assert.Equal(t, int64(1), result.SuccessRequests)
	assert.Equal(t, int64(0), result.ErrorRequests)
	assert.True(t, result.TotalLatency > 0)
}

// TestExecuteRequest_Error 测试HTTP请求执行（错误）
func TestExecuteRequest_Error(t *testing.T) {
	// 创建返回错误的测试HTTP服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()
	
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		BaseURL:        server.URL,
		RequestTimeout: 5 * time.Second,
		TestEndpoints: []TestEndpoint{
			{Name: "test", Method: "GET", Path: "/test", Weight: 1},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test"] = &PerformanceResult{
		EndpointName:   "test",
		ErrorsByStatus: make(map[int]int64),
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour,
		MaxLatency:    0,
	}
	
	endpoint := TestEndpoint{Name: "test", Method: "GET", Path: "/test", Weight: 1}
	tester.executeRequest(endpoint)
	
	result := tester.results["test"]
	assert.Equal(t, int64(1), result.TotalRequests)
	assert.Equal(t, int64(0), result.SuccessRequests)
	assert.Equal(t, int64(1), result.ErrorRequests)
	assert.Equal(t, int64(1), result.ErrorsByStatus[500])
}

// TestSystemMonitoringStats 测试系统监控统计
func TestSystemMonitoringStats(t *testing.T) {
	stats := &SystemMonitoringStats{
		StartTime:       time.Now(),
		StartMemory:     1000,
		StartGoroutines: 10,
	}
	
	// 模拟一段时间后
	time.Sleep(10 * time.Millisecond)
	stats.EndTime = time.Now()
	stats.EndMemory = 1500
	stats.EndGoroutines = 15
	
	// 验证统计信息
	assert.True(t, stats.EndTime.After(stats.StartTime))
	assert.Equal(t, uint64(1000), stats.StartMemory)
	assert.Equal(t, uint64(1500), stats.EndMemory)
	assert.Equal(t, 10, stats.StartGoroutines)
	assert.Equal(t, 15, stats.EndGoroutines)
}

// TestPerformanceTester_InitializeResults 测试结果初始化
func TestPerformanceTester_InitializeResults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "endpoint1", Method: "GET", Path: "/path1", Weight: 1},
			{Name: "endpoint2", Method: "POST", Path: "/path2", Weight: 2},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	
	// 模拟Run方法中的结果初始化逻辑
	for _, endpoint := range tester.config.TestEndpoints {
		tester.results[endpoint.Name] = &PerformanceResult{
			EndpointName:   endpoint.Name,
			ErrorsByStatus: make(map[int]int64),
			Latencies:     make([]time.Duration, 0, 10000),
			MinLatency:    time.Hour,
			MaxLatency:    0,
		}
	}
	
	assert.Len(t, tester.results, 2)
	assert.Contains(t, tester.results, "endpoint1")
	assert.Contains(t, tester.results, "endpoint2")
	
	for _, result := range tester.results {
		assert.NotNil(t, result.ErrorsByStatus)
		assert.NotNil(t, result.Latencies)
		assert.Equal(t, time.Hour, result.MinLatency)
		assert.Equal(t, time.Duration(0), result.MaxLatency)
	}
}

// BenchmarkSelectEndpointByWeight 性能测试：端点权重选择
func BenchmarkSelectEndpointByWeight(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	config := &PerformanceTestConfig{
		TestEndpoints: []TestEndpoint{
			{Name: "endpoint1", Method: "GET", Path: "/path1", Weight: 10},
			{Name: "endpoint2", Method: "GET", Path: "/path2", Weight: 20},
			{Name: "endpoint3", Method: "GET", Path: "/path3", Weight: 30},
			{Name: "endpoint4", Method: "GET", Path: "/path4", Weight: 40},
		},
	}
	
	tester := NewPerformanceTester(config, logger)
	totalWeight := 100
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tester.selectEndpointByWeight(totalWeight)
	}
}

// BenchmarkCalculatePercentiles 性能测试：百分位数计算
func BenchmarkCalculatePercentiles(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	config := &PerformanceTestConfig{}
	tester := NewPerformanceTester(config, logger)
	
	// 创建测试数据
	latencies := make([]time.Duration, 1000)
	for i := 0; i < 1000; i++ {
		latencies[i] = time.Duration(i) * time.Millisecond
	}
	
	result := &PerformanceResult{
		Latencies: latencies,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 重新设置latencies，因为calculatePercentiles会修改它
		result.Latencies = make([]time.Duration, len(latencies))
		copy(result.Latencies, latencies)
		tester.calculatePercentiles(result)
	}
}

// TestPerformanceTester_HTTPClientConfig 测试HTTP客户端配置
func TestPerformanceTester_HTTPClientConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		RequestTimeout: 15 * time.Second,
	}
	
	tester := NewPerformanceTester(config, logger)
	
	assert.Equal(t, 15*time.Second, tester.httpClient.Timeout)
	
	transport := tester.httpClient.Transport.(*http.Transport)
	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 100, transport.MaxConnsPerHost)
	assert.Equal(t, 100, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 30*time.Second, transport.IdleConnTimeout)
}

// TestPerformanceTester_ContextCancel 测试上下文取消
func TestPerformanceTester_ContextCancel(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{}
	
	tester := NewPerformanceTester(config, logger)
	
	// 验证初始状态
	select {
	case <-tester.ctx.Done():
		t.Fatal("Context should not be cancelled initially")
	default:
		// 预期行为
	}
	
	// 取消上下文
	tester.cancel()
	
	// 验证上下文被取消
	select {
	case <-tester.ctx.Done():
		// 预期行为
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be cancelled")
	}
}

// TestPrintResults 测试结果打印（主要验证不会panic）
func TestPrintResults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{
		BaseURL:     "http://test.example.com",
		Concurrency: 10,
		Duration:    5 * time.Second,
	}
	
	tester := NewPerformanceTester(config, logger)
	tester.results["test1"] = &PerformanceResult{
		EndpointName:    "test1",
		TotalRequests:   100,
		SuccessRequests: 95,
		ErrorRequests:   5,
		SuccessRate:     95.0,
		QPS:             20.0,
		AverageLatency:  50 * time.Millisecond,
		MinLatency:      10 * time.Millisecond,
		MaxLatency:      200 * time.Millisecond,
		P50Latency:      45 * time.Millisecond,
		P90Latency:      100 * time.Millisecond,
		P99Latency:      180 * time.Millisecond,
		ErrorsByStatus:  map[int]int64{500: 3, 404: 2},
	}
	
	// 验证不会panic
	assert.NotPanics(t, func() {
		tester.printResults(5 * time.Second)
	})
}

// TestStartStopSystemMonitoring 测试系统监控启动停止
func TestStartStopSystemMonitoring(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &PerformanceTestConfig{}
	tester := NewPerformanceTester(config, logger)
	
	// 启动监控
	stats := tester.startSystemMonitoring()
	assert.NotNil(t, stats)
	assert.False(t, stats.StartTime.IsZero())
	assert.True(t, stats.StartGoroutines > 0)
	
	// 等待一小段时间
	time.Sleep(10 * time.Millisecond)
	
	// 停止监控（验证不会panic）
	assert.NotPanics(t, func() {
		tester.stopSystemMonitoring(stats)
	})
	
	// 验证结束时间被设置
	assert.False(t, stats.EndTime.IsZero())
	assert.True(t, stats.EndTime.After(stats.StartTime))
}