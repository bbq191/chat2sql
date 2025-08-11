package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisConfig Redis配置
type RedisConfig struct {
	Addr             string        `json:"addr" mapstructure:"addr"`
	Password         string        `json:"password" mapstructure:"password"`
	DB               int           `json:"db" mapstructure:"db"`
	MaxRetries       int           `json:"max_retries" mapstructure:"max_retries"`
	MinRetryBackoff  time.Duration `json:"min_retry_backoff" mapstructure:"min_retry_backoff"`
	MaxRetryBackoff  time.Duration `json:"max_retry_backoff" mapstructure:"max_retry_backoff"`
	DialTimeout      time.Duration `json:"dial_timeout" mapstructure:"dial_timeout"`
	ReadTimeout      time.Duration `json:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `json:"write_timeout" mapstructure:"write_timeout"`
	PoolSize         int           `json:"pool_size" mapstructure:"pool_size"`
	MinIdleConns     int           `json:"min_idle_conns" mapstructure:"min_idle_conns"`
	MaxConnAge       time.Duration `json:"max_conn_age" mapstructure:"max_conn_age"`
	PoolTimeout      time.Duration `json:"pool_timeout" mapstructure:"pool_timeout"`
	IdleTimeout      time.Duration `json:"idle_timeout" mapstructure:"idle_timeout"`
	IdleCheckFreq    time.Duration `json:"idle_check_freq" mapstructure:"idle_check_freq"`
	TLSEnabled       bool          `json:"tls_enabled" mapstructure:"tls_enabled"`
	TLSSkipVerify    bool          `json:"tls_skip_verify" mapstructure:"tls_skip_verify"`
	ClusterMode      bool          `json:"cluster_mode" mapstructure:"cluster_mode"`
	ClusterAddrs     []string      `json:"cluster_addrs" mapstructure:"cluster_addrs"`
}

// DefaultRedisConfig 返回默认Redis配置
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:            "localhost:6379",
		Password:        "",
		DB:              0,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        10,
		MinIdleConns:    5,
		MaxConnAge:      0, // 默认不限制
		PoolTimeout:     4 * time.Second,
		IdleTimeout:     5 * time.Minute,
		IdleCheckFreq:   time.Minute,
		TLSEnabled:      false,
		TLSSkipVerify:   false,
		ClusterMode:     false,
		ClusterAddrs:    []string{},
	}
}

// RedisManager Redis客户端管理器
type RedisManager struct {
	client redis.UniversalClient
	config *RedisConfig
	logger *zap.Logger
}

// NewRedisManager 创建新的Redis管理器
func NewRedisManager(config *RedisConfig, logger *zap.Logger) (*RedisManager, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	var client redis.UniversalClient

	// 创建通用客户端选项
	opts := &redis.UniversalOptions{
		Addrs:           []string{config.Addr},
		Password:        config.Password,
		DB:              config.DB,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolSize:        config.PoolSize,
		PoolTimeout:     config.PoolTimeout,
	}

	// 集群模式配置
	if config.ClusterMode && len(config.ClusterAddrs) > 0 {
		opts.Addrs = config.ClusterAddrs
	}

	// TLS配置
	if config.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: config.TLSSkipVerify,
		}
	}

	// 创建客户端
	client = redis.NewUniversalClient(opts)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Redis connected successfully",
		zap.Strings("addrs", opts.Addrs),
		zap.Int("db", config.DB),
		zap.Bool("cluster_mode", config.ClusterMode))

	return &RedisManager{
		client: client,
		config: config,
		logger: logger,
	}, nil
}

// GetClient 获取Redis客户端
func (rm *RedisManager) GetClient() redis.UniversalClient {
	return rm.client
}

// Close 关闭Redis连接
func (rm *RedisManager) Close() error {
	if rm.client != nil {
		return rm.client.Close()
	}
	return nil
}

// HealthCheck 健康检查
func (rm *RedisManager) HealthCheck(ctx context.Context) error {
	return rm.client.Ping(ctx).Err()
}

// GetStats 获取连接池统计信息
func (rm *RedisManager) GetStats() *redis.PoolStats {
	return rm.client.PoolStats()
}

// NewRedisClient 创建Redis客户端的便捷函数
func NewRedisClient(config *RedisConfig) (redis.UniversalClient, error) {
	manager, err := NewRedisManager(config, zap.NewNop())
	if err != nil {
		return nil, err
	}
	return manager.GetClient(), nil
}