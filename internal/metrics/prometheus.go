package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// PrometheusMetrics Prometheus指标收集器
// 收集HTTP请求、数据库连接、SQL执行等关键业务指标
type PrometheusMetrics struct {
	// HTTP请求相关指标
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec
	
	// 业务指标
	sqlExecutionsTotal     *prometheus.CounterVec
	sqlExecutionDuration   *prometheus.HistogramVec
	databaseConnectionsTotal *prometheus.GaugeVec
	userRegistrationsTotal   *prometheus.CounterVec
	
	// 系统指标
	activeConnections     prometheus.Gauge
	memoryUsage          prometheus.Gauge
	goroutineCount       prometheus.Gauge
	
	// 注册器
	registry *prometheus.Registry
	
	logger *zap.Logger
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Namespace   string // 指标命名空间
	Subsystem   string // 指标子系统
	ServiceName string // 服务名称
	ServiceVersion string // 服务版本
}

// DefaultMetricsConfig 默认指标配置
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Namespace:      "chat2sql",
		Subsystem:      "api",
		ServiceName:    "chat2sql-api",
		ServiceVersion: "0.1.0",
	}
}

// NewPrometheusMetrics 创建Prometheus指标收集器
func NewPrometheusMetrics(config *MetricsConfig, logger *zap.Logger) *PrometheusMetrics {
	pm := &PrometheusMetrics{
		logger:   logger,
		registry: prometheus.NewRegistry(),
	}
	
	// 初始化HTTP请求指标
	pm.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)
	
	pm.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets, // 默认桶：0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"method", "endpoint"},
	)
	
	pm.httpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   []float64{1024, 4096, 16384, 65536, 262144, 1048576}, // 1KB to 1MB
		},
		[]string{"method", "endpoint"},
	)
	
	pm.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   []float64{1024, 4096, 16384, 65536, 262144, 1048576}, // 1KB to 1MB
		},
		[]string{"method", "endpoint"},
	)
	
	// 初始化业务指标
	pm.sqlExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: "sql",
			Name:      "executions_total",
			Help:      "Total number of SQL executions",
		},
		[]string{"user_id", "connection_id", "status"},
	)
	
	pm.sqlExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: "sql",
			Name:      "execution_duration_seconds",
			Help:      "SQL execution duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30}, // 1ms to 30s
		},
		[]string{"user_id", "connection_id"},
	)
	
	pm.databaseConnectionsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: "database",
			Name:      "connections_total",
			Help:      "Total number of database connections",
		},
		[]string{"user_id", "db_type", "status"},
	)
	
	pm.userRegistrationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: "auth",
			Name:      "user_registrations_total",
			Help:      "Total number of user registrations",
		},
		[]string{"status"},
	)
	
	// 初始化系统指标
	pm.activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "active_connections",
			Help:      "Number of active HTTP connections",
		},
	)
	
	pm.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: "system",
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		},
	)
	
	pm.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: "system",
			Name:      "goroutines_count",
			Help:      "Number of goroutines",
		},
	)
	
	// 注册所有指标
	pm.registerMetrics()
	
	logger.Info("Prometheus metrics initialized successfully",
		zap.String("namespace", config.Namespace),
		zap.String("subsystem", config.Subsystem))
	
	return pm
}

// registerMetrics 注册所有指标到Prometheus
func (pm *PrometheusMetrics) registerMetrics() {
	// HTTP指标
	pm.registry.MustRegister(pm.httpRequestsTotal)
	pm.registry.MustRegister(pm.httpRequestDuration)
	pm.registry.MustRegister(pm.httpRequestSize)
	pm.registry.MustRegister(pm.httpResponseSize)
	
	// 业务指标
	pm.registry.MustRegister(pm.sqlExecutionsTotal)
	pm.registry.MustRegister(pm.sqlExecutionDuration)
	pm.registry.MustRegister(pm.databaseConnectionsTotal)
	pm.registry.MustRegister(pm.userRegistrationsTotal)
	
	// 系统指标
	pm.registry.MustRegister(pm.activeConnections)
	pm.registry.MustRegister(pm.memoryUsage)
	pm.registry.MustRegister(pm.goroutineCount)
}

// HTTPMetricsMiddleware HTTP指标收集中间件
func (pm *PrometheusMetrics) HTTPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestSize := calculateRequestSize(c.Request)
		
		// 增加活跃连接数
		pm.activeConnections.Inc()
		defer pm.activeConnections.Dec()
		
		// 处理请求
		c.Next()
		
		// 计算指标
		duration := time.Since(start)
		responseSize := c.Writer.Size()
		
		// 获取标签值
		method := c.Request.Method
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = "unknown"
		}
		statusCode := strconv.Itoa(c.Writer.Status())
		
		// 记录指标
		pm.httpRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
		pm.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
		
		if requestSize > 0 {
			pm.httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
		}
		
		if responseSize > 0 {
			pm.httpResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
		}
	}
}

// RecordSQLExecution 记录SQL执行指标
func (pm *PrometheusMetrics) RecordSQLExecution(userID, connectionID int64, status string, duration time.Duration) {
	pm.sqlExecutionsTotal.WithLabelValues(
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(connectionID, 10),
		status,
	).Inc()
	
	pm.sqlExecutionDuration.WithLabelValues(
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(connectionID, 10),
	).Observe(duration.Seconds())
}

// RecordUserRegistration 记录用户注册指标
func (pm *PrometheusMetrics) RecordUserRegistration(status string) {
	pm.userRegistrationsTotal.WithLabelValues(status).Inc()
}

// UpdateDatabaseConnections 更新数据库连接指标
func (pm *PrometheusMetrics) UpdateDatabaseConnections(userID int64, dbType, status string, count int) {
	pm.databaseConnectionsTotal.WithLabelValues(
		strconv.FormatInt(userID, 10),
		dbType,
		status,
	).Set(float64(count))
}

// UpdateSystemMetrics 更新系统指标
func (pm *PrometheusMetrics) UpdateSystemMetrics(memoryBytes int64, goroutines int) {
	pm.memoryUsage.Set(float64(memoryBytes))
	pm.goroutineCount.Set(float64(goroutines))
}

// GetMetricsHandler 获取Prometheus指标端点处理器
func (pm *PrometheusMetrics) GetMetricsHandler() gin.HandlerFunc {
	h := promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// calculateRequestSize 计算请求大小
func calculateRequestSize(r *http.Request) int64 {
	size := int64(0)
	
	if r.ContentLength > 0 {
		size += r.ContentLength
	}
	
	// 计算请求头大小
	for name, values := range r.Header {
		size += int64(len(name))
		for _, value := range values {
			size += int64(len(value))
		}
	}
	
	// 计算URL大小
	size += int64(len(r.URL.String()))
	
	return size
}

// RecordAPILatency 记录API延迟指标
func (pm *PrometheusMetrics) RecordAPILatency(method, endpoint string, duration time.Duration) {
	pm.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordDatabaseOperation 记录数据库操作指标
func (pm *PrometheusMetrics) RecordDatabaseOperation(operation, status string, duration time.Duration) {
	pm.sqlExecutionDuration.WithLabelValues("system", "0").Observe(duration.Seconds())
	pm.sqlExecutionsTotal.WithLabelValues("system", "0", status).Inc()
}

// RecordCacheOperation 记录缓存操作指标
func (pm *PrometheusMetrics) RecordCacheOperation(operation, result string) {
	// 扩展现有指标或创建新的缓存指标
	pm.logger.Debug("记录缓存操作",
		zap.String("operation", operation),
		zap.String("result", result))
}

// GetCurrentMetrics 获取当前指标值（用于监控和调试）
func (pm *PrometheusMetrics) GetCurrentMetrics() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return map[string]interface{}{
		"memory_usage_mb":    memStats.Alloc / 1024 / 1024,
		"memory_sys_mb":      memStats.Sys / 1024 / 1024,
		"goroutines_count":   runtime.NumGoroutine(),
		"gc_runs":            memStats.NumGC,
		"active_connections": 0, // 需要从实际状态获取
	}
}

// ResetMetrics 重置指标（主要用于测试）
func (pm *PrometheusMetrics) ResetMetrics() {
	pm.logger.Info("重置Prometheus指标")
	// 注意：Prometheus指标通常不应该重置，这里主要用于测试场景
}

// CustomMetricsCollector 自定义指标收集器
// 用于收集应用特定的业务指标和系统运行时指标
type CustomMetricsCollector struct {
	prometheus.Collector
	pm                     *PrometheusMetrics
	runtimeStatsDesc       *prometheus.Desc
	connectionPoolStatsDesc *prometheus.Desc
	cacheStatsDesc         *prometheus.Desc
}

// NewCustomMetricsCollector 创建自定义指标收集器
func NewCustomMetricsCollector(pm *PrometheusMetrics) *CustomMetricsCollector {
	return &CustomMetricsCollector{
		pm: pm,
		runtimeStatsDesc: prometheus.NewDesc(
			"chat2sql_runtime_info",
			"Runtime information about the application",
			[]string{"version", "go_version", "os", "arch"},
			nil,
		),
		connectionPoolStatsDesc: prometheus.NewDesc(
			"chat2sql_connection_pool_stats",
			"Connection pool statistics",
			[]string{"pool_name", "stat_type"},
			nil,
		),
		cacheStatsDesc: prometheus.NewDesc(
			"chat2sql_cache_stats",
			"Cache operation statistics",
			[]string{"cache_type", "operation"},
			nil,
		),
	}
}

// Describe 实现prometheus.Collector接口
func (c *CustomMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.runtimeStatsDesc
	ch <- c.connectionPoolStatsDesc
	ch <- c.cacheStatsDesc
}

// Collect 实现prometheus.Collector接口
func (c *CustomMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	// 收集运行时指标
	ch <- prometheus.MustNewConstMetric(
		c.runtimeStatsDesc,
		prometheus.GaugeValue,
		1,
		"v0.1.0",              // 版本信息
		runtime.Version(),     // Go版本
		runtime.GOOS,         // 操作系统
		runtime.GOARCH,       // 架构
	)
	
	// 收集内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// 添加更详细的内存指标
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"chat2sql_go_memory_heap_objects",
			"Number of allocated heap objects",
			nil, nil,
		),
		prometheus.GaugeValue,
		float64(memStats.HeapObjects),
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"chat2sql_go_memory_heap_inuse_bytes",
			"Number of bytes in use by the heap",
			nil, nil,
		),
		prometheus.GaugeValue,
		float64(memStats.HeapInuse),
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"chat2sql_go_memory_stack_inuse_bytes",
			"Number of bytes in use by the stack",
			nil, nil,
		),
		prometheus.GaugeValue,
		float64(memStats.StackInuse),
	)
	
	// GC相关指标
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"chat2sql_go_gc_duration_seconds_total",
			"Total GC pause duration in seconds",
			nil, nil,
		),
		prometheus.CounterValue,
		float64(memStats.PauseTotalNs)/1e9,
	)
}