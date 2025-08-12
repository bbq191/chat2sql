// AI模块配置 - 增加性能优化配置
// 基于企业级并发处理需求设计

package ai

import (
	"net/http"
	"time"
)

// PerformanceConfig 性能优化配置
type PerformanceConfig struct {
	// 工作池配置
	Workers   int `json:"workers" yaml:"workers" mapstructure:"workers"`         // 工作线程数，0表示使用CPU核心数*2
	QueueSize int `json:"queue_size" yaml:"queue_size" mapstructure:"queue_size"` // 任务队列大小

	// 熔断器配置
	MaxFailures  int           `json:"max_failures" yaml:"max_failures" mapstructure:"max_failures"`    // 最大失败次数
	ResetTimeout time.Duration `json:"reset_timeout" yaml:"reset_timeout" mapstructure:"reset_timeout"` // 熔断器重置超时

	// 限流配置
	RateLimit int `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"` // 每秒最大请求数

	// HTTP连接池配置
	MaxIdleConns        int           `json:"max_idle_conns" yaml:"max_idle_conns" mapstructure:"max_idle_conns"`                         // 最大空闲连接数
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host" mapstructure:"max_idle_conns_per_host"` // 每个主机最大空闲连接数
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout" mapstructure:"idle_conn_timeout"`               // 空闲连接超时

	// 缓存配置
	EnableCache   bool          `json:"enable_cache" yaml:"enable_cache" mapstructure:"enable_cache"`       // 启用响应缓存
	CacheTTL      time.Duration `json:"cache_ttl" yaml:"cache_ttl" mapstructure:"cache_ttl"`               // 缓存生存时间
	CacheSize     int           `json:"cache_size" yaml:"cache_size" mapstructure:"cache_size"`             // 缓存大小（条目数）
	
	// 预热配置
	EnablePrewarming bool     `json:"enable_prewarming" yaml:"enable_prewarming" mapstructure:"enable_prewarming"` // 启用预热
	PrewarmPatterns  []string `json:"prewarm_patterns" yaml:"prewarm_patterns" mapstructure:"prewarm_patterns"`   // 预热查询模式
}

// DefaultPerformanceConfig 默认性能配置
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		Workers:   0, // 自动检测CPU核心数
		QueueSize: 1000,

		MaxFailures:  5,
		ResetTimeout: 30 * time.Second,

		RateLimit: 100, // 每秒100个请求

		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,

		EnableCache: true,
		CacheTTL:    5 * time.Minute,
		CacheSize:   10000,

		EnablePrewarming: false,
		PrewarmPatterns:  []string{},
	}
}

// HTTPClient 自定义HTTP客户端，优化连接池
type HTTPClient struct {
	client *http.Client
	config *PerformanceConfig
}

// NewHTTPClient 创建优化的HTTP客户端
func NewHTTPClient(config *PerformanceConfig) *HTTPClient {
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		// 启用HTTP/2
		ForceAttemptHTTP2: true,
		// 优化拨号器
		DisableKeepAlives: false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 总超时时间
	}

	return &HTTPClient{
		client: client,
		config: config,
	}
}

// 注意：不提供 Get/Post 等具体的 HTTP 方法
// 这个包装器专注于提供优化的连接池配置
// 具体的 HTTP 请求应该通过 Client() 方法获取底层客户端来执行

// Client 获取底层HTTP客户端
func (hc *HTTPClient) Client() *http.Client {
	return hc.client
}