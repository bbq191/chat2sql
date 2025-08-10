package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// PerformanceTestConfig 性能测试配置
type PerformanceTestConfig struct {
	BaseURL        string        `json:"base_url"`
	Concurrency    int           `json:"concurrency"`
	Duration       time.Duration `json:"duration"`
	RequestTimeout time.Duration `json:"request_timeout"`
	RampUpTime     time.Duration `json:"ramp_up_time"`
	TestEndpoints  []TestEndpoint `json:"test_endpoints"`
}

// TestEndpoint 测试端点配置
type TestEndpoint struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Weight int    `json:"weight"` // 请求权重，用于负载分配
}

// PerformanceResult 性能测试结果
type PerformanceResult struct {
	EndpointName     string        `json:"endpoint_name"`
	TotalRequests    int64         `json:"total_requests"`
	SuccessRequests  int64         `json:"success_requests"`
	ErrorRequests    int64         `json:"error_requests"`
	SuccessRate      float64       `json:"success_rate"`
	AverageLatency   time.Duration `json:"average_latency"`
	MinLatency       time.Duration `json:"min_latency"`
	MaxLatency       time.Duration `json:"max_latency"`
	P50Latency       time.Duration `json:"p50_latency"`
	P90Latency       time.Duration `json:"p90_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
	QPS              float64       `json:"qps"`
	ErrorsByStatus   map[int]int64 `json:"errors_by_status"`
	TotalLatency     time.Duration `json:"-"`
	Latencies        []time.Duration `json:"-"`
}

// PerformanceTester 性能测试器
type PerformanceTester struct {
	config     *PerformanceTestConfig
	logger     *zap.Logger
	httpClient *http.Client
	results    map[string]*PerformanceResult
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewPerformanceTester 创建性能测试器
func NewPerformanceTester(config *PerformanceTestConfig, logger *zap.Logger) *PerformanceTester {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &PerformanceTester{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		results: make(map[string]*PerformanceResult),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Run 执行性能测试
func (pt *PerformanceTester) Run() error {
	pt.logger.Info("开始性能测试",
		zap.String("base_url", pt.config.BaseURL),
		zap.Int("concurrency", pt.config.Concurrency),
		zap.Duration("duration", pt.config.Duration),
		zap.Duration("ramp_up", pt.config.RampUpTime))

	// 初始化结果统计
	for _, endpoint := range pt.config.TestEndpoints {
		pt.results[endpoint.Name] = &PerformanceResult{
			EndpointName:   endpoint.Name,
			ErrorsByStatus: make(map[int]int64),
			Latencies:     make([]time.Duration, 0, 10000), // 预分配容量
			MinLatency:    time.Hour,  // 初始化为大值
			MaxLatency:    0,
		}
	}

	// 启动系统监控
	systemStats := pt.startSystemMonitoring()

	// 启动性能测试
	startTime := time.Now()
	var wg sync.WaitGroup

	// 分阶段启动工作器（模拟渐进负载）
	workersPerStep := pt.config.Concurrency / 10
	if workersPerStep == 0 {
		workersPerStep = 1
	}
	stepDuration := pt.config.RampUpTime / 10

	pt.logger.Info("开始渐进式负载测试",
		zap.Int("workers_per_step", workersPerStep),
		zap.Duration("step_duration", stepDuration))

	for step := 0; step < 10; step++ {
		for i := 0; i < workersPerStep; i++ {
			wg.Add(1)
			go pt.worker(&wg)
		}
		
		if step < 9 { // 最后一步不需要等待
			time.Sleep(stepDuration)
		}
	}

	// 等待测试持续时间
	time.Sleep(pt.config.Duration)

	// 停止所有工作器
	pt.cancel()
	wg.Wait()

	totalTime := time.Since(startTime)
	
	// 停止系统监控
	pt.stopSystemMonitoring(systemStats)

	// 计算和输出结果
	pt.calculateResults(totalTime)
	pt.printResults(totalTime)

	return nil
}

// worker 工作器goroutine
func (pt *PerformanceTester) worker(wg *sync.WaitGroup) {
	defer wg.Done()

	// 计算端点权重分配
	totalWeight := 0
	for _, endpoint := range pt.config.TestEndpoints {
		totalWeight += endpoint.Weight
	}

	for {
		select {
		case <-pt.ctx.Done():
			return
		default:
			// 根据权重随机选择端点
			endpoint := pt.selectEndpointByWeight(totalWeight)
			pt.executeRequest(endpoint)
		}
	}
}

// selectEndpointByWeight 根据权重选择端点
func (pt *PerformanceTester) selectEndpointByWeight(totalWeight int) TestEndpoint {
	if len(pt.config.TestEndpoints) == 1 {
		return pt.config.TestEndpoints[0]
	}

	// 简单的权重选择算法
	target := time.Now().Nanosecond() % totalWeight
	currentWeight := 0
	
	for _, endpoint := range pt.config.TestEndpoints {
		currentWeight += endpoint.Weight
		if target < currentWeight {
			return endpoint
		}
	}
	
	return pt.config.TestEndpoints[0]
}

// executeRequest 执行HTTP请求
func (pt *PerformanceTester) executeRequest(endpoint TestEndpoint) {
	startTime := time.Now()
	
	url := pt.config.BaseURL + endpoint.Path
	req, err := http.NewRequestWithContext(pt.ctx, endpoint.Method, url, nil)
	if err != nil {
		pt.recordError(endpoint.Name, 0, time.Since(startTime))
		return
	}

	resp, err := pt.httpClient.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		pt.recordError(endpoint.Name, 0, latency)
		return
	}
	defer resp.Body.Close()

	// 记录结果
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		pt.recordSuccess(endpoint.Name, latency)
	} else {
		pt.recordError(endpoint.Name, resp.StatusCode, latency)
	}
}

// recordSuccess 记录成功请求
func (pt *PerformanceTester) recordSuccess(endpointName string, latency time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	result := pt.results[endpointName]
	atomic.AddInt64(&result.TotalRequests, 1)
	atomic.AddInt64(&result.SuccessRequests, 1)
	
	result.TotalLatency += latency
	result.Latencies = append(result.Latencies, latency)
	
	if latency < result.MinLatency {
		result.MinLatency = latency
	}
	if latency > result.MaxLatency {
		result.MaxLatency = latency
	}
}

// recordError 记录错误请求
func (pt *PerformanceTester) recordError(endpointName string, statusCode int, latency time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	result := pt.results[endpointName]
	atomic.AddInt64(&result.TotalRequests, 1)
	atomic.AddInt64(&result.ErrorRequests, 1)
	
	result.ErrorsByStatus[statusCode]++
	
	result.TotalLatency += latency
	result.Latencies = append(result.Latencies, latency)
}

// calculateResults 计算最终结果
func (pt *PerformanceTester) calculateResults(totalTime time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	for _, result := range pt.results {
		if result.TotalRequests > 0 {
			result.SuccessRate = float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
			result.AverageLatency = result.TotalLatency / time.Duration(result.TotalRequests)
			result.QPS = float64(result.TotalRequests) / totalTime.Seconds()

			// 计算延迟百分位数
			if len(result.Latencies) > 0 {
				pt.calculatePercentiles(result)
			}
		}
	}
}

// calculatePercentiles 计算延迟百分位数
func (pt *PerformanceTester) calculatePercentiles(result *PerformanceResult) {
	latencies := result.Latencies
	if len(latencies) == 0 {
		return
	}

	// 简单排序（生产环境建议使用更高效的算法）
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	n := len(latencies)
	result.P50Latency = latencies[n*50/100]
	result.P90Latency = latencies[n*90/100]
	if n*99/100 < n {
		result.P99Latency = latencies[n*99/100]
	} else {
		result.P99Latency = latencies[n-1]
	}
}

// printResults 输出测试结果
func (pt *PerformanceTester) printResults(totalTime time.Duration) {
	fmt.Printf("\n=== 性能测试结果报告 ===\n")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  目标地址: %s\n", pt.config.BaseURL)
	fmt.Printf("  并发数: %d\n", pt.config.Concurrency)
	fmt.Printf("  持续时间: %v\n", pt.config.Duration)
	fmt.Printf("  实际测试时间: %v\n", totalTime)
	fmt.Printf("\n")

	var totalRequests int64
	var totalSuccess int64
	var totalErrors int64

	for _, result := range pt.results {
		totalRequests += result.TotalRequests
		totalSuccess += result.SuccessRequests
		totalErrors += result.ErrorRequests

		fmt.Printf("端点: %s\n", result.EndpointName)
		fmt.Printf("  总请求数: %d\n", result.TotalRequests)
		fmt.Printf("  成功请求: %d\n", result.SuccessRequests)
		fmt.Printf("  失败请求: %d\n", result.ErrorRequests)
		fmt.Printf("  成功率: %.2f%%\n", result.SuccessRate)
		fmt.Printf("  QPS: %.2f\n", result.QPS)
		fmt.Printf("  平均延迟: %v\n", result.AverageLatency)
		fmt.Printf("  最小延迟: %v\n", result.MinLatency)
		fmt.Printf("  最大延迟: %v\n", result.MaxLatency)
		fmt.Printf("  P50延迟: %v\n", result.P50Latency)
		fmt.Printf("  P90延迟: %v\n", result.P90Latency)
		fmt.Printf("  P99延迟: %v\n", result.P99Latency)

		if len(result.ErrorsByStatus) > 0 {
			fmt.Printf("  错误分布:\n")
			for statusCode, count := range result.ErrorsByStatus {
				fmt.Printf("    HTTP %d: %d\n", statusCode, count)
			}
		}
		fmt.Printf("\n")
	}

	fmt.Printf("=== 总体统计 ===\n")
	fmt.Printf("总请求数: %d\n", totalRequests)
	fmt.Printf("总成功数: %d\n", totalSuccess)
	fmt.Printf("总失败数: %d\n", totalErrors)
	fmt.Printf("总体成功率: %.2f%%\n", float64(totalSuccess)/float64(totalRequests)*100)
	fmt.Printf("总体QPS: %.2f\n", float64(totalRequests)/totalTime.Seconds())
}

// startSystemMonitoring 启动系统监控
func (pt *PerformanceTester) startSystemMonitoring() *SystemMonitoringStats {
	var startStats runtime.MemStats
	runtime.ReadMemStats(&startStats)

	return &SystemMonitoringStats{
		StartTime:    time.Now(),
		StartMemory:  startStats.Alloc,
		StartGoroutines: runtime.NumGoroutine(),
	}
}

// stopSystemMonitoring 停止系统监控并输出统计
func (pt *PerformanceTester) stopSystemMonitoring(stats *SystemMonitoringStats) {
	var endStats runtime.MemStats
	runtime.ReadMemStats(&endStats)

	stats.EndTime = time.Now()
	stats.EndMemory = endStats.Alloc
	stats.EndGoroutines = runtime.NumGoroutine()

	fmt.Printf("=== 系统资源统计 ===\n")
	fmt.Printf("测试持续时间: %v\n", stats.EndTime.Sub(stats.StartTime))
	fmt.Printf("内存使用变化: %d KB -> %d KB (增长: %d KB)\n",
		stats.StartMemory/1024, stats.EndMemory/1024, 
		(int64(stats.EndMemory)-int64(stats.StartMemory))/1024)
	fmt.Printf("Goroutine数量变化: %d -> %d (增长: %d)\n",
		stats.StartGoroutines, stats.EndGoroutines,
		stats.EndGoroutines-stats.StartGoroutines)
	fmt.Printf("GC次数: %d\n", endStats.NumGC)
	fmt.Printf("\n")
}

// SystemMonitoringStats 系统监控统计
type SystemMonitoringStats struct {
	StartTime       time.Time
	EndTime         time.Time
	StartMemory     uint64
	EndMemory       uint64
	StartGoroutines int
	EndGoroutines   int
}

// loadConfig 加载配置文件
func loadConfig(configFile string) (*PerformanceTestConfig, error) {
	if configFile == "" {
		// 返回默认配置
		return &PerformanceTestConfig{
			BaseURL:        "http://localhost:8080",
			Concurrency:    50,
			Duration:       60 * time.Second,
			RequestTimeout: 30 * time.Second,
			RampUpTime:     10 * time.Second,
			TestEndpoints: []TestEndpoint{
				{Name: "health", Method: "GET", Path: "/health", Weight: 10},
				{Name: "metrics", Method: "GET", Path: "/metrics", Weight: 3},
			},
		}, nil
	}

	file, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	var config PerformanceTestConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// main 主函数
func main() {
	// 命令行参数
	var (
		configFile = flag.String("config", "", "配置文件路径 (可选)")
		baseURL    = flag.String("url", "http://localhost:8080", "测试目标URL")
		concurrency = flag.Int("c", 50, "并发数")
		duration   = flag.Duration("d", 60*time.Second, "测试持续时间")
		rampUp     = flag.Duration("ramp", 10*time.Second, "渐进加载时间")
	)
	flag.Parse()

	// 创建日志器
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 加载配置
	config, err := loadConfig(*configFile)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}

	// 命令行参数覆盖配置文件
	if *baseURL != "http://localhost:8080" {
		config.BaseURL = *baseURL
	}
	if *concurrency != 50 {
		config.Concurrency = *concurrency
	}
	if *duration != 60*time.Second {
		config.Duration = *duration
	}
	if *rampUp != 10*time.Second {
		config.RampUpTime = *rampUp
	}

	// 创建性能测试器
	tester := NewPerformanceTester(config, logger)

	// 处理中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("收到中断信号，正在停止测试...")
		tester.cancel()
	}()

	// 执行性能测试
	if err := tester.Run(); err != nil {
		logger.Fatal("性能测试失败", zap.Error(err))
	}

	logger.Info("性能测试完成")
}