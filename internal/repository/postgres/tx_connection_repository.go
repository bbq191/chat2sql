package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// PostgreSQLTxConnectionRepository PostgreSQL事务版连接Repository实现
// 基于pgx.Tx实现，所有操作在事务上下文中执行
type PostgreSQLTxConnectionRepository struct {
	tx     pgx.Tx       // PostgreSQL事务
	logger *zap.Logger  // 结构化日志器
}

// NewPostgreSQLTxConnectionRepository 创建PostgreSQL事务版连接Repository
func NewPostgreSQLTxConnectionRepository(tx pgx.Tx, logger *zap.Logger) repository.ConnectionRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLTxConnectionRepository{
		tx:     tx,
		logger: logger,
	}
}

// Create 创建数据库连接配置（事务版本）
func (r *PostgreSQLTxConnectionRepository) Create(ctx context.Context, conn *repository.DatabaseConnection) error {
	const query = `
		INSERT INTO database_connections (user_id, name, host, port, database_name, 
			username, password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.tx.QueryRow(ctx, query,
		conn.UserID,
		conn.Name,
		conn.Host,
		conn.Port,
		conn.DatabaseName,
		conn.Username,
		conn.PasswordEncrypted,
		conn.DBType,
		conn.Status,
		conn.LastTested,
		conn.CreateBy,
		now,
		conn.UpdateBy,
		now,
		false,
	).Scan(&conn.ID)
	
	if err != nil {
		r.logger.Error("Failed to create database connection in transaction", 
			zap.Error(err),
			zap.Int64("user_id", conn.UserID))
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	
	r.logger.Info("Database connection created successfully in transaction",
		zap.Int64("connection_id", conn.ID),
		zap.Int64("user_id", conn.UserID))
	
	return nil
}

// GetByID 根据ID获取数据库连接（事务版本）
func (r *PostgreSQLTxConnectionRepository) GetByID(ctx context.Context, id int64) (*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections
		WHERE id = $1 AND is_deleted = false`

	conn := &repository.DatabaseConnection{}
	
	err := r.tx.QueryRow(ctx, query, id).Scan(
		&conn.ID,
		&conn.UserID,
		&conn.Name,
		&conn.Host,
		&conn.Port,
		&conn.DatabaseName,
		&conn.Username,
		&conn.PasswordEncrypted,
		&conn.DBType,
		&conn.Status,
		&conn.LastTested,
		&conn.CreateBy,
		&conn.CreateTime,
		&conn.UpdateBy,
		&conn.UpdateTime,
		&conn.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("database connection not found")
		}
		r.logger.Error("Failed to get database connection by ID in transaction",
			zap.Error(err), zap.Int64("connection_id", id))
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	
	return conn, nil
}

// Update 更新数据库连接配置（事务版本）
func (r *PostgreSQLTxConnectionRepository) Update(ctx context.Context, conn *repository.DatabaseConnection) error {
	const query = `
		UPDATE database_connections 
		SET name = $2, host = $3, port = $4, database_name = $5, 
			username = $6, password_encrypted = $7, db_type = $8, 
			status = $9, update_by = $10, update_time = $11
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query,
		conn.ID,
		conn.Name,
		conn.Host,
		conn.Port,
		conn.DatabaseName,
		conn.Username,
		conn.PasswordEncrypted,
		conn.DBType,
		conn.Status,
		conn.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("Failed to update database connection in transaction",
			zap.Error(err), zap.Int64("connection_id", conn.ID))
		return fmt.Errorf("failed to update database connection: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("database connection not found or already deleted")
	}
	
	r.logger.Info("Database connection updated successfully in transaction",
		zap.Int64("connection_id", conn.ID))
	
	return nil
}

// Delete 软删除数据库连接（事务版本）
func (r *PostgreSQLTxConnectionRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE database_connections 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("Failed to delete database connection in transaction",
			zap.Error(err), zap.Int64("connection_id", id))
		return fmt.Errorf("failed to delete database connection: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("database connection not found or already deleted")
	}
	
	r.logger.Info("Database connection deleted successfully in transaction",
		zap.Int64("connection_id", id))
	
	return nil
}

// ListByUser 获取用户的数据库连接列表（事务版本）
func (r *PostgreSQLTxConnectionRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections
		WHERE user_id = $1 AND is_deleted = false
		ORDER BY create_time DESC`

	rows, err := r.tx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list database connections: %w", err)
	}
	defer rows.Close()

	var connections []*repository.DatabaseConnection
	for rows.Next() {
		conn := &repository.DatabaseConnection{}
		err := rows.Scan(
			&conn.ID, &conn.UserID, &conn.Name, &conn.Host, &conn.Port,
			&conn.DatabaseName, &conn.Username, &conn.PasswordEncrypted,
			&conn.DBType, &conn.Status, &conn.LastTested,
			&conn.CreateBy, &conn.CreateTime, &conn.UpdateBy, &conn.UpdateTime, &conn.IsDeleted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan database connection: %w", err)
		}
		connections = append(connections, conn)
	}

	return connections, nil
}

// CountByUser 统计用户的数据库连接数量（事务版本）
func (r *PostgreSQLTxConnectionRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM database_connections WHERE user_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.tx.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count user connections: %w", err)
	}
	
	return count, nil
}

// UpdateStatus 更新连接状态（事务版本）
func (r *PostgreSQLTxConnectionRepository) UpdateStatus(ctx context.Context, connectionID int64, status repository.ConnectionStatus) error {
	const query = `
		UPDATE database_connections 
		SET status = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, connectionID, string(status), now)
	
	if err != nil {
		return fmt.Errorf("failed to update connection status: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("connection not found or already deleted")
	}
	
	return nil
}

// UpdateLastTested 更新最后测试时间（事务版本）
func (r *PostgreSQLTxConnectionRepository) UpdateLastTested(ctx context.Context, connectionID int64, testTime time.Time) error {
	const query = `
		UPDATE database_connections 
		SET last_tested = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, connectionID, testTime, now)
	
	if err != nil {
		return fmt.Errorf("failed to update last tested time: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("connection not found or already deleted")
	}
	
	return nil
}

// BatchUpdateStatus 批量更新连接状态（事务版本）
func (r *PostgreSQLTxConnectionRepository) BatchUpdateStatus(ctx context.Context, connectionIDs []int64, status repository.ConnectionStatus) error {
	if len(connectionIDs) == 0 {
		return nil
	}

	const query = `
		UPDATE database_connections 
		SET status = $1, update_time = $2
		WHERE id = ANY($3) AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, string(status), now, connectionIDs)
	
	if err != nil {
		return fmt.Errorf("failed to batch update connection status: %w", err)
	}
	
	r.logger.Info("Batch updated connection status in transaction",
		zap.Int("connection_count", len(connectionIDs)),
		zap.String("status", string(status)),
		zap.Int64("rows_affected", result.RowsAffected()))
	
	return nil
}

// ExistsByUserAndName 检查用户是否已有同名连接（事务版本）
func (r *PostgreSQLTxConnectionRepository) ExistsByUserAndName(ctx context.Context, userID int64, name string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM database_connections WHERE user_id = $1 AND name = $2 AND is_deleted = false)`
	
	var exists bool
	err := r.tx.QueryRow(ctx, query, userID, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check connection existence: %w", err)
	}
	
	return exists, nil
}

// 其他接口方法的简化实现，返回未实现错误
func (r *PostgreSQLTxConnectionRepository) ListByType(ctx context.Context, dbType repository.DatabaseType) ([]*repository.DatabaseConnection, error) {
	return nil, fmt.Errorf("ListByType not implemented in transaction version")
}

func (r *PostgreSQLTxConnectionRepository) ListByStatus(ctx context.Context, status repository.ConnectionStatus) ([]*repository.DatabaseConnection, error) {
	return nil, fmt.Errorf("ListByStatus not implemented in transaction version")
}

func (r *PostgreSQLTxConnectionRepository) GetByUserAndName(ctx context.Context, userID int64, name string) (*repository.DatabaseConnection, error) {
	return nil, fmt.Errorf("GetByUserAndName not implemented in transaction version")
}

func (r *PostgreSQLTxConnectionRepository) CountByStatus(ctx context.Context, status repository.ConnectionStatus) (int64, error) {
	return 0, fmt.Errorf("CountByStatus not implemented in transaction version")
}

func (r *PostgreSQLTxConnectionRepository) CountByType(ctx context.Context, dbType repository.DatabaseType) (int64, error) {
	return 0, fmt.Errorf("CountByType not implemented in transaction version")
}

func (r *PostgreSQLTxConnectionRepository) GetActiveConnections(ctx context.Context) ([]*repository.DatabaseConnection, error) {
	return nil, fmt.Errorf("GetActiveConnections not implemented in transaction version")
}