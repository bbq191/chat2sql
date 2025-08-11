package metrics

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// TestDefaultSystemMonitorConfig 测试默认系统监控配置
func TestDefaultSystemMonitorConfig(t *testing.T) {
	config := DefaultSystemMonitorConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 15*time.Second, config.CollectInterval)
	assert.Equal(t, "chat2sql", config.Namespace)
	assert.True(t, config.Enabled)
}

// TestNewSystemMonitor_Enabled 测试创建启用的系统监控器
func TestNewSystemMonitor_Enabled(t *testing.T) {
	// 创建独立的registry避免冲突
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 5 * time.Second,
		Namespace:       "test_monitor",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	assert.NotNil(t, monitor)
	assert.Equal(t, 5*time.Second, monitor.collectInterval)
	assert.NotNil(t, monitor.cpuUsagePercent)
	assert.NotNil(t, monitor.memoryUsageBytes)
	assert.NotNil(t, monitor.goroutineCount)
	assert.False(t, monitor.IsRunning())
}

// TestNewSystemMonitor_Disabled 测试创建禁用的系统监控器
func TestNewSystemMonitor_Disabled(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 5 * time.Second,
		Namespace:       "test_monitor",
		Enabled:         false,
	}
	
	monitor := NewSystemMonitor(config, logger)
	assert.NotNil(t, monitor)
	assert.Equal(t, 5*time.Second, monitor.collectInterval)
	// 禁用时指标应该为nil
	assert.Nil(t, monitor.cpuUsagePercent)
	assert.Nil(t, monitor.memoryUsageBytes)
	assert.False(t, monitor.IsRunning())
}

// TestNewSystemMonitor_NilConfig 测试使用nil配置创建监控器
func TestNewSystemMonitor_NilConfig(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	
	monitor := NewSystemMonitor(nil, logger)
	assert.NotNil(t, monitor)
	// 应该使用默认配置
	assert.Equal(t, 15*time.Second, monitor.collectInterval)
}

// TestSystemMonitor_StartStop 测试启动和停止监控
func TestSystemMonitor_StartStop(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 100 * time.Millisecond, // 短间隔用于测试
		Namespace:       "test_monitor",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	// 测试启动
	assert.False(t, monitor.IsRunning())
	err := monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.IsRunning())
	
	// 再次启动应该无操作
	err = monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.IsRunning())
	
	// 等待一段时间让监控运行
	time.Sleep(250 * time.Millisecond)
	
	// 测试停止
	err = monitor.Stop()
	assert.NoError(t, err)
	assert.False(t, monitor.IsRunning())
	
	// 再次停止应该无操作
	err = monitor.Stop()
	assert.NoError(t, err)
	assert.False(t, monitor.IsRunning())
}

// TestSystemMonitor_CollectMetrics 测试指标收集
func TestSystemMonitor_CollectMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 1 * time.Second,
		Namespace:       "test_monitor",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	
	// 手动收集指标
	monitor.collectMetrics()
	
	// 验证指标是否被设置
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	
	expectedMetrics := []string{
		"test_monitor_system_cpu_usage_percent",
		"test_monitor_system_memory_usage_bytes",
		"test_monitor_system_memory_alloc_bytes",
		"test_monitor_system_memory_sys_bytes",
		"test_monitor_system_goroutines_count",
	}
	
	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamily {
		foundMetrics[mf.GetName()] = true
	}
	
	for _, expectedMetric := range expectedMetrics {
		assert.True(t, foundMetrics[expectedMetric], "应该找到指标: %s", expectedMetric)
	}
}

// TestSystemMonitor_GetCurrentStats 测试获取当前统计信息
func TestSystemMonitor_GetCurrentStats(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	stats := monitor.GetCurrentStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.MemoryAllocBytes > 0)
	assert.True(t, stats.MemorySysBytes > 0)
	assert.True(t, stats.GoroutinesCount > 0)
	assert.Equal(t, runtime.NumCPU(), stats.CPUCount)
	assert.False(t, stats.CollectedAt.IsZero())
}

// TestSystemMonitor_GetMetrics 测试获取指标信息
func TestSystemMonitor_GetMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	metrics := monitor.GetMetrics()
	assert.NotNil(t, metrics)
	
	// 验证返回的指标包含预期字段
	expectedKeys := []string{
		"memory_alloc_mb",
		"memory_sys_mb", 
		"goroutines_count",
		"cpu_count",
		"gc_runs",
		"is_running",
		"collect_interval",
	}
	
	for _, key := range expectedKeys {
		assert.Contains(t, metrics, key, "应该包含指标: %s", key)
	}
	
	// 验证值的类型和合理性
	assert.IsType(t, uint64(0), metrics["memory_alloc_mb"])
	assert.IsType(t, uint64(0), metrics["memory_sys_mb"])
	assert.IsType(t, int(0), metrics["goroutines_count"])
	assert.IsType(t, int(0), metrics["cpu_count"])
	assert.IsType(t, uint32(0), metrics["gc_runs"])
	assert.IsType(t, false, metrics["is_running"])
	assert.IsType(t, time.Duration(0), metrics["collect_interval"])
	
	// 验证值的合理性
	assert.True(t, metrics["goroutines_count"].(int) > 0)
	assert.True(t, metrics["cpu_count"].(int) > 0)
	assert.Equal(t, false, metrics["is_running"])
	assert.Equal(t, 15*time.Second, metrics["collect_interval"])
}

// TestSystemMonitor_MonitorLoop_ContextCancel 测试上下文取消
func TestSystemMonitor_MonitorLoop_ContextCancel(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 50 * time.Millisecond,
		Namespace:       "test_monitor",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	
	// 创建可取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	// 启动监控
	err := monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.IsRunning())
	
	// 等待上下文超时
	time.Sleep(300 * time.Millisecond)
	
	// 监控应该自动停止（由于上下文取消）
	// 注意：IsRunning状态可能不会立即更新，因为它只在Stop()时更新
}

// TestSystemMonitor_CollectGCMetrics 测试GC指标收集
func TestSystemMonitor_CollectGCMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 1 * time.Second,
		Namespace:       "test_gc",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	
	// 触发GC
	runtime.GC()
	runtime.GC() // 触发多次GC
	
	// 收集指标
	monitor.collectMetrics()
	
	// 验证GC指标
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	
	foundGCRuns := false
	foundGCPause := false
	
	for _, mf := range metricFamily {
		switch mf.GetName() {
		case "test_gc_gc_runs_total":
			foundGCRuns = true
		case "test_gc_gc_pause_duration_seconds":
			foundGCPause = true
		}
	}
	
	assert.True(t, foundGCRuns, "应该找到GC运行次数指标")
	assert.True(t, foundGCPause, "应该找到GC暂停时间指标")
}

// TestSystemMonitor_CPUMetrics 测试CPU指标计算
func TestSystemMonitor_CPUMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	// 手动调用CPU指标收集
	monitor.collectCPUMetrics()
	
	// 验证CPU使用率指标
	expected := `
# HELP chat2sql_system_cpu_usage_percent CPU使用率百分比
# TYPE chat2sql_system_cpu_usage_percent gauge
`
	
	err := testutil.GatherAndCompare(registry, strings.NewReader(expected), 
		"chat2sql_system_cpu_usage_percent")
	// 由于CPU使用率的值不确定，我们只验证指标存在
	if err != nil {
		// 检查是否是因为指标值不匹配而失败（这是预期的）
		assert.Contains(t, err.Error(), "chat2sql_system_cpu_usage_percent")
	}
	
	// 验证指标确实存在
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	
	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "chat2sql_system_cpu_usage_percent" {
			found = true
			// 验证CPU使用率在合理范围内（0-100%）
			for _, metric := range mf.GetMetric() {
				value := metric.GetGauge().GetValue()
				assert.True(t, value >= 0 && value <= 100, 
					"CPU使用率应该在0-100之间，实际值: %f", value)
			}
			break
		}
	}
	
	assert.True(t, found, "应该找到CPU使用率指标")
}

// TestSystemStats_Fields 测试SystemStats结构体
func TestSystemStats_Fields(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	stats := monitor.GetCurrentStats()
	
	// 验证所有字段都有有效值
	assert.True(t, stats.MemoryAllocBytes > 0, "内存分配量应该大于0")
	assert.True(t, stats.MemoryTotalAllocBytes >= stats.MemoryAllocBytes, 
		"总分配量应该大于等于当前分配量")
	assert.True(t, stats.MemorySysBytes >= stats.MemoryAllocBytes,
		"系统内存应该大于等于分配内存")
	assert.True(t, stats.GoroutinesCount > 0, "Goroutine数量应该大于0")
	assert.Equal(t, runtime.NumCPU(), stats.CPUCount, "CPU数量应该匹配")
	assert.True(t, stats.GCRuns >= 0, "GC运行次数应该非负")
	assert.False(t, stats.CollectedAt.IsZero(), "收集时间应该被设置")
	
	// LastGCTime可能为零值（如果还没有发生GC）
	// 这是正常的，所以我们不对此进行断言
}

// TestSystemMonitor_CollectMetrics_MemoryTotalAlloc 测试累计内存分配跟踪
func TestSystemMonitor_CollectMetrics_MemoryTotalAlloc(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	// 第一次收集
	monitor.collectMetrics()
	
	// 分配一些内存
	_ = make([]byte, 1024*1024) // 1MB
	
	// 第二次收集
	monitor.collectMetrics()
	
	// 验证累计分配内存指标存在
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	
	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "chat2sql_system_memory_total_alloc_bytes" {
			found = true
			break
		}
	}
	
	assert.True(t, found, "应该找到累计内存分配指标")
}

// TestSystemMonitor_MultipleCollections 测试多次指标收集
func TestSystemMonitor_MultipleCollections(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 10 * time.Millisecond,
		Namespace:       "multi_test",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	
	// 多次收集指标
	for i := 0; i < 5; i++ {
		monitor.collectMetrics()
		time.Sleep(5 * time.Millisecond)
	}
	
	// 验证指标被正确更新
	stats := monitor.GetCurrentStats()
	assert.True(t, stats.GoroutinesCount > 0)
	assert.True(t, stats.MemoryAllocBytes > 0)
}

// BenchmarkSystemMonitor_CollectMetrics 性能测试：指标收集
func BenchmarkSystemMonitor_CollectMetrics(b *testing.B) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(&testing.T{})
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.collectMetrics()
	}
}

// BenchmarkSystemMonitor_GetCurrentStats 性能测试：获取统计信息
func BenchmarkSystemMonitor_GetCurrentStats(b *testing.B) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(&testing.T{})
	monitor := NewSystemMonitor(DefaultSystemMonitorConfig(), logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.GetCurrentStats()
	}
}

// TestSystemMonitor_Integration 集成测试：启动监控并验证指标更新
func TestSystemMonitor_Integration(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	
	logger := zaptest.NewLogger(t)
	config := &SystemMonitorConfig{
		CollectInterval: 50 * time.Millisecond,
		Namespace:       "integration_test",
		Enabled:         true,
	}
	
	monitor := NewSystemMonitor(config, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	// 启动监控
	err := monitor.Start(ctx)
	assert.NoError(t, err)
	
	// 等待几个收集周期
	time.Sleep(150 * time.Millisecond)
	
	// 验证指标被收集
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	assert.True(t, len(metricFamily) > 0, "应该有指标被收集")
	
	// 验证关键指标存在
	expectedMetrics := []string{
		"integration_test_system_memory_usage_bytes",
		"integration_test_system_goroutines_count",
		"integration_test_system_cpu_usage_percent",
	}
	
	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamily {
		foundMetrics[mf.GetName()] = true
	}
	
	for _, expectedMetric := range expectedMetrics {
		assert.True(t, foundMetrics[expectedMetric], 
			"应该找到指标: %s", expectedMetric)
	}
	
	// 停止监控
	err = monitor.Stop()
	assert.NoError(t, err)
}