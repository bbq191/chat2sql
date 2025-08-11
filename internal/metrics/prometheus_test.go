package metrics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// TestPrometheusMetrics_NewPrometheusMetrics 测试Prometheus指标创建
func TestPrometheusMetrics_NewPrometheusMetrics(t *testing.T) {
	// 创建测试logger
	logger := zaptest.NewLogger(t)
	
	// 创建默认配置
	config := DefaultMetricsConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "chat2sql", config.Namespace)
	assert.Equal(t, "api", config.Subsystem)
	
	// 创建PrometheusMetrics实例
	pm := NewPrometheusMetrics(config, logger)
	assert.NotNil(t, pm)
	assert.NotNil(t, pm.httpRequestsTotal)
	assert.NotNil(t, pm.httpRequestDuration)
	assert.NotNil(t, pm.sqlExecutionsTotal)
	assert.NotNil(t, pm.activeConnections)
}

// TestPrometheusMetrics_HTTPMetricsMiddleware 测试HTTP指标中间件
func TestPrometheusMetrics_HTTPMetricsMiddleware(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	
	// 创建测试路由
	router := gin.New()
	router.Use(pm.HTTPMetricsMiddleware())
	
	// 添加测试端点
	router.GET("/test", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
	})
	
	// 测试GET请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	// 测试POST请求
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/test", bytes.NewBuffer([]byte(`{"data":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	// 验证指标是否被记录
	// 检查HTTP请求总数指标
	metricFamily, err := pm.registry.Gather()
	assert.NoError(t, err)
	assert.True(t, len(metricFamily) > 0)
	
	// 查找我们的指标
	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "chat2sql_api_http_requests_total" {
			found = true
			assert.True(t, len(mf.GetMetric()) >= 2) // 至少有两个请求
			break
		}
	}
	assert.True(t, found, "HTTP requests total metric should be present")
}

// TestPrometheusMetrics_RecordSQLExecution 测试SQL执行指标记录
func TestPrometheusMetrics_RecordSQLExecution(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 记录SQL执行指标
	pm.RecordSQLExecution(123, 456, "success", 250*time.Millisecond)
	pm.RecordSQLExecution(123, 456, "error", 100*time.Millisecond)
	pm.RecordSQLExecution(789, 456, "success", 500*time.Millisecond)
	
	// 验证指标
	metricFamily, err := pm.registry.Gather()
	assert.NoError(t, err)
	
	// 查找SQL执行总数指标
	sqlExecutionsFound := false
	sqlDurationFound := false
	
	for _, mf := range metricFamily {
		switch mf.GetName() {
		case "chat2sql_sql_executions_total":
			sqlExecutionsFound = true
			assert.True(t, len(mf.GetMetric()) >= 2)
		case "chat2sql_sql_execution_duration_seconds":
			sqlDurationFound = true
			assert.True(t, len(mf.GetMetric()) >= 2)
		}
	}
	
	assert.True(t, sqlExecutionsFound, "SQL executions total metric should be present")
	assert.True(t, sqlDurationFound, "SQL execution duration metric should be present")
}

// TestPrometheusMetrics_RecordUserRegistration 测试用户注册指标
func TestPrometheusMetrics_RecordUserRegistration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 记录用户注册指标
	pm.RecordUserRegistration("success")
	pm.RecordUserRegistration("success")
	pm.RecordUserRegistration("error")
	
	// 验证指标
	expected := `
# HELP chat2sql_auth_user_registrations_total Total number of user registrations
# TYPE chat2sql_auth_user_registrations_total counter
chat2sql_auth_user_registrations_total{status="error"} 1
chat2sql_auth_user_registrations_total{status="success"} 2
`
	
	err := testutil.GatherAndCompare(pm.registry, strings.NewReader(expected), 
		"chat2sql_auth_user_registrations_total")
	assert.NoError(t, err)
}

// TestPrometheusMetrics_UpdateDatabaseConnections 测试数据库连接指标更新
func TestPrometheusMetrics_UpdateDatabaseConnections(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 更新数据库连接指标
	pm.UpdateDatabaseConnections(123, "postgresql", "active", 5)
	pm.UpdateDatabaseConnections(456, "mysql", "active", 3)
	pm.UpdateDatabaseConnections(789, "postgresql", "error", 1)
	
	// 验证指标
	expected := `
# HELP chat2sql_database_connections_total Total number of database connections
# TYPE chat2sql_database_connections_total gauge
chat2sql_database_connections_total{db_type="mysql",status="active",user_id="456"} 3
chat2sql_database_connections_total{db_type="postgresql",status="active",user_id="123"} 5
chat2sql_database_connections_total{db_type="postgresql",status="error",user_id="789"} 1
`
	
	err := testutil.GatherAndCompare(pm.registry, strings.NewReader(expected), 
		"chat2sql_database_connections_total")
	assert.NoError(t, err)
}

// TestPrometheusMetrics_UpdateSystemMetrics 测试系统指标更新
func TestPrometheusMetrics_UpdateSystemMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 更新系统指标
	memoryBytes := int64(1024 * 1024 * 100) // 100MB
	goroutines := 50
	pm.UpdateSystemMetrics(memoryBytes, goroutines)
	
	// 验证指标
	expected := `
# HELP chat2sql_system_memory_usage_bytes Current memory usage in bytes
# TYPE chat2sql_system_memory_usage_bytes gauge
chat2sql_system_memory_usage_bytes 1.048576e+08
# HELP chat2sql_system_goroutines_count Number of goroutines
# TYPE chat2sql_system_goroutines_count gauge
chat2sql_system_goroutines_count 50
`
	
	err := testutil.GatherAndCompare(pm.registry, strings.NewReader(expected), 
		"chat2sql_system_memory_usage_bytes", "chat2sql_system_goroutines_count")
	assert.NoError(t, err)
}

// TestPrometheusMetrics_GetMetricsHandler 测试指标处理器
func TestPrometheusMetrics_GetMetricsHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 记录一些指标
	pm.RecordUserRegistration("success")
	pm.UpdateSystemMetrics(1024*1024, 10)
	
	// 创建测试路由
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", pm.GetMetricsHandler())
	
	// 测试指标端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	
	// 验证响应包含我们的指标
	body := w.Body.String()
	assert.Contains(t, body, "chat2sql_auth_user_registrations_total")
	assert.Contains(t, body, "chat2sql_system_memory_usage_bytes")
	assert.Contains(t, body, "chat2sql_system_goroutines_count")
}

// TestPrometheusMetrics_GetCurrentMetrics 测试获取当前指标
func TestPrometheusMetrics_GetCurrentMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	metrics := pm.GetCurrentMetrics()
	assert.NotNil(t, metrics)
	
	// 验证返回的指标包含预期字段
	assert.Contains(t, metrics, "memory_usage_mb")
	assert.Contains(t, metrics, "memory_sys_mb")
	assert.Contains(t, metrics, "goroutines_count")
	assert.Contains(t, metrics, "gc_runs")
	assert.Contains(t, metrics, "active_connections")
	
	// 验证值的合理性
	assert.IsType(t, uint64(0), metrics["memory_usage_mb"])
	assert.IsType(t, uint64(0), metrics["memory_sys_mb"])
	assert.IsType(t, int(0), metrics["goroutines_count"])
	assert.IsType(t, uint32(0), metrics["gc_runs"])
	
	// 验证协程数量是合理的
	goroutineCount := metrics["goroutines_count"].(int)
	assert.True(t, goroutineCount > 0, "Goroutine count should be positive")
	assert.True(t, goroutineCount < 10000, "Goroutine count should be reasonable")
}

// TestCustomMetricsCollector 测试自定义指标收集器
func TestCustomMetricsCollector(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 创建自定义收集器
	collector := NewCustomMetricsCollector(pm)
	assert.NotNil(t, collector)
	
	// 注册收集器
	registry.MustRegister(collector)
	
	// 收集指标
	metricFamily, err := registry.Gather()
	assert.NoError(t, err)
	
	// 验证自定义指标存在
	foundRuntime := false
	foundMemory := false
	
	for _, mf := range metricFamily {
		switch mf.GetName() {
		case "chat2sql_runtime_info":
			foundRuntime = true
			// 验证标签
			metric := mf.GetMetric()[0]
			labels := metric.GetLabel()
			hasVersionLabel := false
			hasGoVersionLabel := false
			
			for _, label := range labels {
				if label.GetName() == "version" {
					hasVersionLabel = true
					assert.Equal(t, "v0.1.0", label.GetValue())
				}
				if label.GetName() == "go_version" {
					hasGoVersionLabel = true
					assert.Equal(t, runtime.Version(), label.GetValue())
				}
			}
			
			assert.True(t, hasVersionLabel, "Runtime info should have version label")
			assert.True(t, hasGoVersionLabel, "Runtime info should have go_version label")
			
		case "chat2sql_go_memory_heap_objects":
			foundMemory = true
		}
	}
	
	assert.True(t, foundRuntime, "Runtime info metric should be present")
	assert.True(t, foundMemory, "Memory heap objects metric should be present")
}

// TestPrometheusMetrics_RecordAPILatency 测试API延迟记录
func TestPrometheusMetrics_RecordAPILatency(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 记录不同端点的延迟
	pm.RecordAPILatency("GET", "/api/v1/users", 100*time.Millisecond)
	pm.RecordAPILatency("POST", "/api/v1/sql/execute", 250*time.Millisecond)
	pm.RecordAPILatency("GET", "/api/v1/users", 150*time.Millisecond)
	
	// 验证指标存在
	metricFamily, err := pm.registry.Gather()
	assert.NoError(t, err)
	
	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "chat2sql_api_http_request_duration_seconds" {
			found = true
			assert.True(t, len(mf.GetMetric()) >= 2, "Should have metrics for different endpoints")
			
			// 验证至少有一个指标的值是合理的
			for _, metric := range mf.GetMetric() {
				histogram := metric.GetHistogram()
				if histogram != nil {
					assert.True(t, histogram.GetSampleCount() > 0, "Should have recorded samples")
					assert.True(t, histogram.GetSampleSum() > 0, "Sample sum should be positive")
				}
			}
			break
		}
	}
	
	assert.True(t, found, "API latency metric should be present")
}

// TestPrometheusMetrics_RecordDatabaseOperation 测试数据库操作记录
func TestPrometheusMetrics_RecordDatabaseOperation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 记录数据库操作
	pm.RecordDatabaseOperation("query", "success", 50*time.Millisecond)
	pm.RecordDatabaseOperation("migration", "success", 500*time.Millisecond)
	pm.RecordDatabaseOperation("query", "error", 10*time.Millisecond)
	
	// 验证指标记录
	metricFamily, err := pm.registry.Gather()
	assert.NoError(t, err)
	
	foundCounter := false
	foundHistogram := false
	
	for _, mf := range metricFamily {
		switch mf.GetName() {
		case "chat2sql_sql_executions_total":
			foundCounter = true
		case "chat2sql_sql_execution_duration_seconds":
			foundHistogram = true
		}
	}
	
	assert.True(t, foundCounter, "Database operation counter should be present")
	assert.True(t, foundHistogram, "Database operation duration should be present")
}

// TestPrometheusMetrics_RecordCacheOperation 测试缓存操作记录
func TestPrometheusMetrics_RecordCacheOperation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 这个方法目前只是记录日志，测试它不会panic
	assert.NotPanics(t, func() {
		pm.RecordCacheOperation("get", "hit")
		pm.RecordCacheOperation("set", "success")
		pm.RecordCacheOperation("get", "miss")
	})
}

// TestCalculateRequestSize 测试请求大小计算
func TestCalculateRequestSize(t *testing.T) {
	// 创建测试请求
	body := `{"test": "data", "number": 123}`
	req, _ := http.NewRequest("POST", "/api/test?param=value", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.ContentLength = int64(len(body))
	
	size := calculateRequestSize(req)
	
	// 验证大小计算包含了请求体、头部和URL
	assert.True(t, size > 0)
	assert.True(t, size >= int64(len(body)), "Size should at least include body length")
	
	// 测试空请求
	emptyReq, _ := http.NewRequest("GET", "/", nil)
	emptySize := calculateRequestSize(emptyReq)
	assert.True(t, emptySize > 0, "Even empty requests should have some size")
}

// TestMetricsConfig_DefaultValues 测试默认指标配置
func TestMetricsConfig_DefaultValues(t *testing.T) {
	config := DefaultMetricsConfig()
	
	assert.Equal(t, "chat2sql", config.Namespace)
	assert.Equal(t, "api", config.Subsystem)
	assert.Equal(t, "chat2sql-api", config.ServiceName)
	assert.Equal(t, "0.1.0", config.ServiceVersion)
}

// TestPrometheusMetrics_ResetMetrics 测试重置指标
func TestPrometheusMetrics_ResetMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	// 这个方法主要用于测试，验证它不会panic
	assert.NotPanics(t, func() {
		pm.ResetMetrics()
	})
}

// BenchmarkPrometheusMetrics_RecordSQLExecution 性能测试：SQL执行指标记录
func BenchmarkPrometheusMetrics_RecordSQLExecution(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordSQLExecution(int64(i%100), int64(i%10), "success", 100*time.Millisecond)
	}
}

// BenchmarkPrometheusMetrics_HTTPMiddleware 性能测试：HTTP中间件
func BenchmarkPrometheusMetrics_HTTPMiddleware(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	config := DefaultMetricsConfig()
	pm := NewPrometheusMetrics(config, logger)
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(pm.HTTPMetricsMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}