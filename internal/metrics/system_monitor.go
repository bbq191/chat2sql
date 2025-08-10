package metrics

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// SystemMonitor 系统性能监控器
// 收集CPU、内存、Goroutine等系统级性能指标
type SystemMonitor struct {
	// Prometheus指标
	cpuUsagePercent      prometheus.Gauge
	memoryUsageBytes     prometheus.Gauge
	memoryAllocBytes     prometheus.Gauge
	memoryTotalAllocBytes prometheus.Counter
	memorySysBytes       prometheus.Gauge
	goroutineCount       prometheus.Gauge
	gcPauseSeconds       prometheus.Histogram
	gcRunsTotal          prometheus.Counter
	
	// 内部状态
	isRunning        int32
	stopChan         chan struct{}
	collectInterval  time.Duration
	logger          *zap.Logger
	
	// 缓存的上次GC统计
	lastGCStats runtime.MemStats
}

// SystemMonitorConfig 系统监控配置
type SystemMonitorConfig struct {
	CollectInterval time.Duration // 数据收集间隔
	Namespace       string        // Prometheus指标命名空间
	Enabled         bool          // 是否启用监控
}

// DefaultSystemMonitorConfig 默认系统监控配置
func DefaultSystemMonitorConfig() *SystemMonitorConfig {
	return &SystemMonitorConfig{
		CollectInterval: 15 * time.Second,
		Namespace:       "chat2sql",
		Enabled:         true,
	}
}

// NewSystemMonitor 创建系统监控器
func NewSystemMonitor(config *SystemMonitorConfig, logger *zap.Logger) *SystemMonitor {
	if config == nil {
		config = DefaultSystemMonitorConfig()
	}
	
	monitor := &SystemMonitor{
		collectInterval: config.CollectInterval,
		logger:         logger,
		stopChan:       make(chan struct{}),
	}
	
	if !config.Enabled {
		logger.Info("系统监控器已禁用")
		return monitor
	}
	
	// 初始化Prometheus指标
	monitor.initializeMetrics(config.Namespace)
	
	// 注册指标到Prometheus
	monitor.registerMetrics()
	
	logger.Info("系统监控器初始化完成",
		zap.Duration("collect_interval", config.CollectInterval),
		zap.String("namespace", config.Namespace))
	
	return monitor
}

// initializeMetrics 初始化Prometheus指标
func (sm *SystemMonitor) initializeMetrics(namespace string) {
	// CPU使用率
	sm.cpuUsagePercent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "cpu_usage_percent",
			Help:      "CPU使用率百分比",
		},
	)
	
	// 内存使用指标
	sm.memoryUsageBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "memory_usage_bytes",
			Help:      "当前内存使用量（字节）",
		},
	)
	
	sm.memoryAllocBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system", 
			Name:      "memory_alloc_bytes",
			Help:      "堆内存分配量（字节）",
		},
	)
	
	sm.memoryTotalAllocBytes = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "memory_total_alloc_bytes",
			Help:      "累计分配内存总量（字节）",
		},
	)
	
	sm.memorySysBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "memory_sys_bytes",
			Help:      "从系统获得的内存量（字节）",
		},
	)
	
	// Goroutine数量
	sm.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "goroutines_count",
			Help:      "当前Goroutine数量",
		},
	)
	
	// GC性能指标
	sm.gcPauseSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "gc",
			Name:      "pause_duration_seconds",
			Help:      "GC暂停时间分布（秒）",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
	)
	
	sm.gcRunsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "gc",
			Name:      "runs_total",
			Help:      "GC运行总次数",
		},
	)
}

// registerMetrics 注册指标到Prometheus
func (sm *SystemMonitor) registerMetrics() {
	prometheus.MustRegister(sm.cpuUsagePercent)
	prometheus.MustRegister(sm.memoryUsageBytes)
	prometheus.MustRegister(sm.memoryAllocBytes)
	prometheus.MustRegister(sm.memoryTotalAllocBytes)
	prometheus.MustRegister(sm.memorySysBytes)
	prometheus.MustRegister(sm.goroutineCount)
	prometheus.MustRegister(sm.gcPauseSeconds)
	prometheus.MustRegister(sm.gcRunsTotal)
}

// Start 启动系统监控
func (sm *SystemMonitor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&sm.isRunning, 0, 1) {
		return nil // 已经在运行
	}
	
	sm.logger.Info("启动系统性能监控",
		zap.Duration("interval", sm.collectInterval))
	
	// 启动监控goroutine
	go sm.monitorLoop(ctx)
	
	return nil
}

// Stop 停止系统监控
func (sm *SystemMonitor) Stop() error {
	if !atomic.CompareAndSwapInt32(&sm.isRunning, 1, 0) {
		return nil // 已经停止
	}
	
	sm.logger.Info("停止系统性能监控")
	
	select {
	case sm.stopChan <- struct{}{}:
	default:
	}
	
	return nil
}

// monitorLoop 监控循环
func (sm *SystemMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(sm.collectInterval)
	defer ticker.Stop()
	
	// 立即执行一次收集
	sm.collectMetrics()
	
	for {
		select {
		case <-ctx.Done():
			sm.logger.Info("系统监控收到上下文取消信号")
			return
		case <-sm.stopChan:
			sm.logger.Info("系统监控收到停止信号")
			return
		case <-ticker.C:
			sm.collectMetrics()
		}
	}
}

// collectMetrics 收集系统指标
func (sm *SystemMonitor) collectMetrics() {
	// 收集内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// 更新内存指标
	sm.memoryUsageBytes.Set(float64(memStats.Alloc))
	sm.memoryAllocBytes.Set(float64(memStats.Alloc))
	sm.memorySysBytes.Set(float64(memStats.Sys))
	
	// 累计分配内存（只增不减）
	if memStats.TotalAlloc > sm.lastGCStats.TotalAlloc {
		sm.memoryTotalAllocBytes.Add(float64(memStats.TotalAlloc - sm.lastGCStats.TotalAlloc))
	}
	
	// 更新Goroutine数量
	sm.goroutineCount.Set(float64(runtime.NumGoroutine()))
	
	// 收集GC统计
	sm.collectGCMetrics(&memStats)
	
	// 计算CPU使用率（简化实现）
	sm.collectCPUMetrics()
	
	// 保存当前统计用于下次比较
	sm.lastGCStats = memStats
	
	// 记录监控日志（调试级别）
	sm.logger.Debug("收集系统性能指标",
		zap.Uint64("memory_alloc_mb", memStats.Alloc/1024/1024),
		zap.Uint64("memory_sys_mb", memStats.Sys/1024/1024),
		zap.Int("goroutines", runtime.NumGoroutine()),
		zap.Uint32("gc_runs", memStats.NumGC))
}

// collectGCMetrics 收集GC指标
func (sm *SystemMonitor) collectGCMetrics(memStats *runtime.MemStats) {
	// GC运行次数变化
	if memStats.NumGC > sm.lastGCStats.NumGC {
		gcRuns := memStats.NumGC - sm.lastGCStats.NumGC
		sm.gcRunsTotal.Add(float64(gcRuns))
		
		// 记录最近的GC暂停时间
		if len(memStats.PauseNs) > 0 {
			// 获取最新的GC暂停时间
			recentPauseIndex := (memStats.NumGC + 255) % 256
			pauseNs := memStats.PauseNs[recentPauseIndex]
			if pauseNs > 0 {
				pauseSeconds := float64(pauseNs) / 1e9
				sm.gcPauseSeconds.Observe(pauseSeconds)
			}
		}
	}
}

// collectCPUMetrics 收集CPU指标（简化实现）
func (sm *SystemMonitor) collectCPUMetrics() {
	// 注意：这是一个简化的CPU使用率计算
	// 在生产环境中，建议使用更精确的CPU监控方法
	
	numCPU := float64(runtime.NumCPU())
	numGoroutine := float64(runtime.NumGoroutine())
	
	// 估算CPU使用率（基于Goroutine数量的粗略估算）
	cpuUsageEstimate := (numGoroutine / numCPU) * 10.0
	if cpuUsageEstimate > 100.0 {
		cpuUsageEstimate = 100.0
	}
	
	sm.cpuUsagePercent.Set(cpuUsageEstimate)
}

// GetCurrentStats 获取当前系统统计信息
func (sm *SystemMonitor) GetCurrentStats() *SystemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return &SystemStats{
		MemoryAllocBytes:     memStats.Alloc,
		MemoryTotalAllocBytes: memStats.TotalAlloc,
		MemorySysBytes:       memStats.Sys,
		GoroutinesCount:      runtime.NumGoroutine(),
		CPUCount:             runtime.NumCPU(),
		GCRuns:               memStats.NumGC,
		LastGCTime:           time.Unix(0, int64(memStats.LastGC)),
		CollectedAt:          time.Now(),
	}
}

// SystemStats 系统统计信息
type SystemStats struct {
	MemoryAllocBytes      uint64    `json:"memory_alloc_bytes"`
	MemoryTotalAllocBytes uint64    `json:"memory_total_alloc_bytes"`
	MemorySysBytes        uint64    `json:"memory_sys_bytes"`
	GoroutinesCount       int       `json:"goroutines_count"`
	CPUCount              int       `json:"cpu_count"`
	GCRuns                uint32    `json:"gc_runs"`
	LastGCTime            time.Time `json:"last_gc_time"`
	CollectedAt           time.Time `json:"collected_at"`
}

// IsRunning 检查监控器是否正在运行
func (sm *SystemMonitor) IsRunning() bool {
	return atomic.LoadInt32(&sm.isRunning) == 1
}

// GetMetrics 获取所有监控指标（用于测试和调试）
func (sm *SystemMonitor) GetMetrics() map[string]interface{} {
	stats := sm.GetCurrentStats()
	
	return map[string]interface{}{
		"memory_alloc_mb":    stats.MemoryAllocBytes / 1024 / 1024,
		"memory_sys_mb":      stats.MemorySysBytes / 1024 / 1024,
		"goroutines_count":   stats.GoroutinesCount,
		"cpu_count":          stats.CPUCount,
		"gc_runs":            stats.GCRuns,
		"last_gc_time":       stats.LastGCTime,
		"is_running":         sm.IsRunning(),
		"collect_interval":   sm.collectInterval,
	}
}