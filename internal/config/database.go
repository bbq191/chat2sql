package config

import (
	"fmt"
	"time"
	
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseConfig PostgreSQL数据库连接配置
// 支持环境变量配置，适用于容器化部署
type DatabaseConfig struct {
	// 数据库连接基础配置
	Host     string `env:"DB_HOST" envDefault:"localhost" json:"host"`         // 数据库主机地址
	Port     int    `env:"DB_PORT" envDefault:"5432" json:"port"`              // 数据库端口
	User     string `env:"DB_USER" envDefault:"postgres" json:"user"`          // 数据库用户名
	Password string `env:"DB_PASSWORD" json:"-"`                               // 数据库密码（不输出到JSON）
	Database string `env:"DB_NAME" envDefault:"chat2sql" json:"database"`      // 数据库名称
	
	// SSL连接配置
	SSLMode          string `env:"DB_SSL_MODE" envDefault:"prefer" json:"ssl_mode"`           // SSL模式：disable, require, verify-ca, verify-full
	SSLCert          string `env:"DB_SSL_CERT" json:"ssl_cert,omitempty"`                     // SSL证书文件路径
	SSLKey           string `env:"DB_SSL_KEY" json:"ssl_key,omitempty"`                       // SSL私钥文件路径
	SSLRootCert      string `env:"DB_SSL_ROOT_CERT" json:"ssl_root_cert,omitempty"`           // SSL根证书路径
	
	// 连接池配置 - 基于pgxpool最佳实践
	MaxConns        int32         `env:"DB_MAX_CONNS" envDefault:"100" json:"max_conns"`               // 最大连接数（生产环境建议100-200）
	MinConns        int32         `env:"DB_MIN_CONNS" envDefault:"10" json:"min_conns"`                // 最小连接数（保持热连接）
	MaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h" json:"max_conn_lifetime"` // 连接最大生命周期
	MaxConnIdleTime time.Duration `env:"DB_MAX_CONN_IDLE" envDefault:"30m" json:"max_conn_idle_time"`   // 连接最大空闲时间
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"5m" json:"health_check_period"` // 健康检查周期
	
	// 查询超时配置
	ConnectTimeout  time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"30s" json:"connect_timeout"`   // 连接超时
	QueryTimeout    time.Duration `env:"DB_QUERY_TIMEOUT" envDefault:"30s" json:"query_timeout"`       // 查询超时
	PreparedStatementCacheSize int32 `env:"DB_PREPARED_STATEMENT_CACHE_SIZE" envDefault:"100" json:"prepared_statement_cache_size"` // 预处理语句缓存大小
	
	// 监控与日志配置
	LogLevel         string `env:"DB_LOG_LEVEL" envDefault:"warn" json:"log_level"`               // 日志级别：trace, debug, info, warn, error, none
	LogSlowQueries   bool   `env:"DB_LOG_SLOW_QUERIES" envDefault:"true" json:"log_slow_queries"` // 是否记录慢查询
	SlowQueryThreshold time.Duration `env:"DB_SLOW_QUERY_THRESHOLD" envDefault:"1s" json:"slow_query_threshold"` // 慢查询阈值
	
	// 应用级配置
	ApplicationName string `env:"DB_APPLICATION_NAME" envDefault:"chat2sql" json:"application_name"` // 应用名称（用于数据库监控）
	SearchPath      string `env:"DB_SEARCH_PATH" envDefault:"public" json:"search_path"`             // 默认模式搜索路径
}

// GetConnectionString 构建PostgreSQL连接字符串
// 基于pgx连接字符串格式，支持所有配置参数
func (c *DatabaseConfig) GetConnectionString() string {
	// 构建基础连接字符串
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s application_name=%s search_path=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode, c.ApplicationName, c.SearchPath,
	)
	
	// 添加SSL证书配置（如果提供）
	if c.SSLCert != "" {
		connStr += fmt.Sprintf(" sslcert=%s", c.SSLCert)
	}
	if c.SSLKey != "" {
		connStr += fmt.Sprintf(" sslkey=%s", c.SSLKey)
	}
	if c.SSLRootCert != "" {
		connStr += fmt.Sprintf(" sslrootcert=%s", c.SSLRootCert)
	}
	
	// 添加超时配置
	connStr += fmt.Sprintf(" connect_timeout=%d", int(c.ConnectTimeout.Seconds()))
	
	return connStr
}

// Validate 验证数据库配置的有效性
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("数据库主机地址不能为空")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("数据库端口必须在1-65535范围内")
	}
	if c.User == "" {
		return fmt.Errorf("数据库用户名不能为空")
	}
	if c.Database == "" {
		return fmt.Errorf("数据库名称不能为空")
	}
	if c.MaxConns <= 0 {
		return fmt.Errorf("最大连接数必须大于0")
	}
	if c.MinConns < 0 {
		return fmt.Errorf("最小连接数不能小于0")
	}
	if c.MinConns > c.MaxConns {
		return fmt.Errorf("最小连接数不能大于最大连接数")
	}
	
	// 验证SSL模式
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	valid := false
	for _, mode := range validSSLModes {
		if c.SSLMode == mode {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("无效的SSL模式: %s", c.SSLMode)
	}
	
	return nil
}

// GetPoolConfig 获取pgxpool连接池配置
func (c *DatabaseConfig) GetPoolConfig() (*pgxpool.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("数据库配置验证失败: %w", err)
	}
	
	// 解析连接字符串
	config, err := pgxpool.ParseConfig(c.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("解析数据库连接字符串失败: %w", err)
	}
	
	// 配置连接池参数
	config.MaxConns = c.MaxConns
	config.MinConns = c.MinConns
	config.MaxConnLifetime = c.MaxConnLifetime
	config.MaxConnIdleTime = c.MaxConnIdleTime
	config.HealthCheckPeriod = c.HealthCheckPeriod
	
	// 配置查询日志（暂时简化实现）
	// TODO: 在后续版本中集成zap日志系统
	
	return config, nil
}

// DefaultDatabaseConfig 返回默认的数据库配置
// 适用于开发环境的默认设置
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "chat2sql",
		SSLMode:  "prefer",
		
		MaxConns:        100,
		MinConns:        10,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		HealthCheckPeriod: 5 * time.Minute,
		
		ConnectTimeout: 30 * time.Second,
		QueryTimeout:   30 * time.Second,
		PreparedStatementCacheSize: 100,
		
		LogLevel:           "warn",
		LogSlowQueries:     true,
		SlowQueryThreshold: time.Second,
		
		ApplicationName: "chat2sql",
		SearchPath:      "public",
	}
}

// ProductionDatabaseConfig 返回生产环境优化的数据库配置
func ProductionDatabaseConfig() *DatabaseConfig {
	config := DefaultDatabaseConfig()
	
	// 生产环境优化设置
	config.MaxConns = 200                    // 更高的并发连接数
	config.MinConns = 20                     // 更多的热连接
	config.MaxConnLifetime = 2 * time.Hour   // 更长的连接生命周期
	config.HealthCheckPeriod = 2 * time.Minute // 更频繁的健康检查
	config.PreparedStatementCacheSize = 200   // 更大的缓存
	config.LogLevel = "error"                // 只记录错误日志
	config.SlowQueryThreshold = 500 * time.Millisecond // 更严格的慢查询阈值
	
	return config
}