package database

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Manager PostgreSQL数据库连接管理器
// 基于pgxpool实现高性能连接池管理，支持健康检查和监控
type Manager struct {
	pool   *pgxpool.Pool       // pgx连接池
	config *config.DatabaseConfig // 数据库配置
	logger *zap.Logger        // 结构化日志器
}

// NewManager 创建新的数据库管理器
// 参数:
//   - config: 数据库配置
//   - logger: zap日志器
// 返回:
//   - Manager实例和错误信息
func NewManager(dbConfig *config.DatabaseConfig, logger *zap.Logger) (*Manager, error) {
	if dbConfig == nil {
		return nil, fmt.Errorf("数据库配置不能为空")
	}
	
	if logger == nil {
		logger = zap.NewNop() // 如果没有提供logger，使用空logger
	}

	logger.Info("初始化数据库连接池",
		zap.String("host", dbConfig.Host),
		zap.Int("port", dbConfig.Port),
		zap.String("database", dbConfig.Database),
		zap.Int32("max_conns", dbConfig.MaxConns),
		zap.Int32("min_conns", dbConfig.MinConns),
	)

	// 获取连接池配置
	poolConfig, err := dbConfig.GetPoolConfig()
	if err != nil {
		logger.Error("获取连接池配置失败", zap.Error(err))
		return nil, fmt.Errorf("获取连接池配置失败: %w", err)
	}

	// 创建连接池
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		logger.Error("创建数据库连接池失败", zap.Error(err))
		return nil, fmt.Errorf("创建数据库连接池失败: %w", err)
	}

	manager := &Manager{
		pool:   pool,
		config: dbConfig,
		logger: logger,
	}

	// 执行初始化健康检查
	if err := manager.HealthCheck(context.Background()); err != nil {
		pool.Close()
		logger.Error("数据库健康检查失败", zap.Error(err))
		return nil, fmt.Errorf("数据库健康检查失败: %w", err)
	}

	logger.Info("数据库连接池初始化成功",
		zap.Int32("max_conns", poolConfig.MaxConns),
		zap.Int32("min_conns", poolConfig.MinConns),
	)

	return manager, nil
}

// GetPool 获取数据库连接池
// 返回pgxpool.Pool实例，用于执行数据库操作
func (m *Manager) GetPool() *pgxpool.Pool {
	return m.pool
}

// HealthCheck 执行数据库健康检查
// 验证数据库连接是否正常，并记录连接池状态
func (m *Manager) HealthCheck(ctx context.Context) error {
	if m.pool == nil {
		return fmt.Errorf("数据库连接池未初始化")
	}
	
	// 设置健康检查超时
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 执行简单查询测试连接
	var result int
	err := m.pool.QueryRow(checkCtx, "SELECT 1").Scan(&result)
	if err != nil {
		m.logger.Error("数据库健康检查查询失败", zap.Error(err))
		return fmt.Errorf("数据库健康检查失败: %w", err)
	}

	if result != 1 {
		err := fmt.Errorf("数据库健康检查返回值异常: %d", result)
		m.logger.Error("数据库健康检查失败", zap.Error(err))
		return err
	}

	// 记录连接池统计信息
	stat := m.pool.Stat()
	m.logger.Debug("数据库连接池状态",
		zap.Int32("total_conns", stat.TotalConns()),
		zap.Int32("idle_conns", stat.IdleConns()),
		zap.Int32("acquired_conns", stat.AcquiredConns()),
		zap.Int32("constructing_conns", stat.ConstructingConns()),
		zap.Int64("acquire_count", stat.AcquireCount()),
		zap.Duration("acquire_duration", stat.AcquireDuration()),
	)

	return nil
}

// GetPoolStats 获取连接池统计信息
// 返回连接池的详细状态信息，用于监控和调试
func (m *Manager) GetPoolStats() *PoolStats {
	stat := m.pool.Stat()
	
	return &PoolStats{
		TotalConns:        stat.TotalConns(),
		IdleConns:         stat.IdleConns(),
		AcquiredConns:     stat.AcquiredConns(),
		ConstructingConns: stat.ConstructingConns(),
		AcquireCount:      stat.AcquireCount(),
		AcquireDuration:   stat.AcquireDuration(),
		MaxConns:          m.config.MaxConns,
		MinConns:          m.config.MinConns,
		MaxLifetime:       m.config.MaxConnLifetime,
		MaxIdleTime:       m.config.MaxConnIdleTime,
	}
}

// Close 关闭数据库连接池
// 优雅地关闭所有数据库连接，释放资源
func (m *Manager) Close() {
	if m.pool != nil {
		m.logger.Info("关闭数据库连接池")
		m.pool.Close()
		m.logger.Info("数据库连接池已关闭")
	}
}

// Ping 执行数据库连接测试
// 简化版本的健康检查，仅测试连接是否可用
func (m *Manager) Ping(ctx context.Context) error {
	return m.pool.Ping(ctx)
}

// PoolStats 连接池统计信息
// 包含连接池的运行时状态和配置参数
type PoolStats struct {
	// 运行时状态
	TotalConns        int32         `json:"total_conns"`        // 总连接数
	IdleConns         int32         `json:"idle_conns"`         // 空闲连接数
	AcquiredConns     int32         `json:"acquired_conns"`     // 已获取连接数
	ConstructingConns int32         `json:"constructing_conns"` // 正在创建的连接数
	AcquireCount      int64         `json:"acquire_count"`      // 总获取次数
	AcquireDuration   time.Duration `json:"acquire_duration"`   // 平均获取时间
	
	// 配置参数
	MaxConns    int32         `json:"max_conns"`     // 最大连接数配置
	MinConns    int32         `json:"min_conns"`     // 最小连接数配置
	MaxLifetime time.Duration `json:"max_lifetime"`  // 最大生命周期配置
	MaxIdleTime time.Duration `json:"max_idle_time"` // 最大空闲时间配置
}

// GetUtilization 计算连接池利用率
// 返回连接池使用率百分比 (0.0-1.0)
func (ps *PoolStats) GetUtilization() float64 {
	if ps.MaxConns <= 0 {
		return 0.0
	}
	return float64(ps.AcquiredConns) / float64(ps.MaxConns)
}

// IsHealthy 判断连接池是否健康
// 基于利用率和连接状态判断连接池是否处于健康状态
func (ps *PoolStats) IsHealthy() bool {
	utilization := ps.GetUtilization()
	
	// 利用率超过90%认为不健康
	if utilization > 0.9 {
		return false
	}
	
	// 没有空闲连接且总连接数达到最大值认为不健康
	if ps.IdleConns == 0 && ps.TotalConns >= ps.MaxConns {
		return false
	}
	
	return true
}

// String 返回连接池统计信息的字符串表示
func (ps *PoolStats) String() string {
	return fmt.Sprintf(
		"Pool Stats - Total: %d, Idle: %d, Acquired: %d, Utilization: %.1f%%",
		ps.TotalConns, ps.IdleConns, ps.AcquiredConns, ps.GetUtilization()*100,
	)
}