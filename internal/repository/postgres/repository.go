package postgres

import (
	"context"
	"fmt"

	"chat2sql-go/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PostgreSQLRepository PostgreSQL Repository实现
// 聚合所有子Repository，提供统一的数据访问接口和事务管理
type PostgreSQLRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger

	// 子Repository实例
	userRepo         repository.UserRepository
	queryHistoryRepo repository.QueryHistoryRepository
	connectionRepo   repository.ConnectionRepository
	schemaRepo       repository.SchemaRepository
}

// NewPostgreSQLRepository 创建PostgreSQL Repository实例
func NewPostgreSQLRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.Repository {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &PostgreSQLRepository{
		pool:   pool,
		logger: logger,

		// 初始化所有子Repository
		userRepo:         NewPostgreSQLUserRepository(pool, logger),
		queryHistoryRepo: NewPostgreSQLQueryHistoryRepository(pool, logger),
		connectionRepo:   NewPostgreSQLConnectionRepository(pool, logger),
		schemaRepo:       NewPostgreSQLSchemaRepository(pool, logger),
	}
}

// UserRepo 获取用户Repository
func (r *PostgreSQLRepository) UserRepo() repository.UserRepository {
	return r.userRepo
}

// QueryHistoryRepo 获取查询历史Repository
func (r *PostgreSQLRepository) QueryHistoryRepo() repository.QueryHistoryRepository {
	return r.queryHistoryRepo
}

// ConnectionRepo 获取连接Repository
func (r *PostgreSQLRepository) ConnectionRepo() repository.ConnectionRepository {
	return r.connectionRepo
}

// SchemaRepo 获取元数据Repository
func (r *PostgreSQLRepository) SchemaRepo() repository.SchemaRepository {
	return r.schemaRepo
}

// BeginTx 开始事务，返回事务Repository
func (r *PostgreSQLRepository) BeginTx(ctx context.Context) (repository.TxRepository, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("开始事务失败", zap.Error(err))
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}

	r.logger.Debug("事务开始成功")

	return &PostgreSQLTxRepository{
		tx:     tx,
		logger: r.logger,

		// 创建基于事务的子Repository
		userRepo:         NewPostgreSQLTxUserRepository(tx, r.logger),
		queryHistoryRepo: NewPostgreSQLTxQueryHistoryRepository(tx, r.logger),
		connectionRepo:   NewPostgreSQLTxConnectionRepository(tx, r.logger),
		schemaRepo:       NewPostgreSQLTxSchemaRepository(tx, r.logger),
	}, nil
}

// Close 关闭Repository（实际上关闭连接池）
func (r *PostgreSQLRepository) Close() error {
	r.logger.Info("关闭PostgreSQL Repository")
	r.pool.Close()
	return nil
}

// HealthCheck 健康检查
func (r *PostgreSQLRepository) HealthCheck(ctx context.Context) error {
	var result int
	err := r.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		r.logger.Error("repository健康检查失败", zap.Error(err))
		return fmt.Errorf("repository健康检查失败: %w", err)
	}

	if result != 1 {
		err := fmt.Errorf("健康检查返回异常值: %d", result)
		r.logger.Error("Repository健康检查异常", zap.Error(err))
		return err
	}

	r.logger.Debug("Repository健康检查通过")
	return nil
}

// PostgreSQLTxRepository 事务Repository实现
// 在事务上下文中执行所有Repository操作
type PostgreSQLTxRepository struct {
	tx     pgx.Tx
	logger *zap.Logger

	// 基于事务的子Repository实例
	userRepo         repository.UserRepository
	queryHistoryRepo repository.QueryHistoryRepository
	connectionRepo   repository.ConnectionRepository
	schemaRepo       repository.SchemaRepository
}

// UserRepo 获取用户Repository（事务版本）
func (r *PostgreSQLTxRepository) UserRepo() repository.UserRepository {
	return r.userRepo
}

// QueryHistoryRepo 获取查询历史Repository（事务版本）
func (r *PostgreSQLTxRepository) QueryHistoryRepo() repository.QueryHistoryRepository {
	return r.queryHistoryRepo
}

// ConnectionRepo 获取连接Repository（事务版本）
func (r *PostgreSQLTxRepository) ConnectionRepo() repository.ConnectionRepository {
	return r.connectionRepo
}

// SchemaRepo 获取元数据Repository（事务版本）
func (r *PostgreSQLTxRepository) SchemaRepo() repository.SchemaRepository {
	return r.schemaRepo
}

// Commit 提交事务
func (r *PostgreSQLTxRepository) Commit() error {
	err := r.tx.Commit(context.Background())
	if err != nil {
		r.logger.Error("事务提交失败", zap.Error(err))
		return fmt.Errorf("事务提交失败: %w", err)
	}

	r.logger.Debug("事务提交成功")
	return nil
}

// Rollback 回滚事务
func (r *PostgreSQLTxRepository) Rollback() error {
	err := r.tx.Rollback(context.Background())
	if err != nil {
		r.logger.Error("事务回滚失败", zap.Error(err))
		return fmt.Errorf("事务回滚失败: %w", err)
	}

	r.logger.Debug("事务回滚成功")
	return nil
}

// 注意：已实现用户Repository的事务版本，其他Repository待实现
