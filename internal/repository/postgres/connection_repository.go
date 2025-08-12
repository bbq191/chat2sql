package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// PostgreSQLConnectionRepository PostgreSQL数据库连接Repository实现
// 支持多数据库连接管理、连接测试、状态监控等功能
type PostgreSQLConnectionRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgreSQLConnectionRepository 创建PostgreSQL连接Repository
func NewPostgreSQLConnectionRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.ConnectionRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLConnectionRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create 创建数据库连接配置
func (r *PostgreSQLConnectionRepository) Create(ctx context.Context, conn *repository.DatabaseConnection) error {
	const query = `
		INSERT INTO database_connections (user_id, name, host, port, database_name, 
			username, password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.pool.QueryRow(ctx, query,
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
		r.logger.Error("创建数据库连接配置失败",
			zap.Int64("user_id", conn.UserID),
			zap.String("name", conn.Name),
			zap.String("host", conn.Host),
			zap.String("db_type", conn.DBType),
			zap.Error(err),
		)
		
		if isUniqueViolation(err) {
			return fmt.Errorf("连接名称已存在: %w", repository.ErrDuplicateEntry)
		}
		
		return fmt.Errorf("创建数据库连接配置失败: %w", err)
	}
	
	conn.CreateTime = now
	conn.UpdateTime = now
	conn.IsDeleted = false
	
	r.logger.Info("数据库连接配置创建成功",
		zap.Int64("connection_id", conn.ID),
		zap.Int64("user_id", conn.UserID),
		zap.String("name", conn.Name),
		zap.String("db_type", conn.DBType),
	)
	
	return nil
}

// GetByID 根据ID获取数据库连接配置
func (r *PostgreSQLConnectionRepository) GetByID(ctx context.Context, id int64) (*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE id = $1 AND is_deleted = false`

	conn := &repository.DatabaseConnection{}
	
	err := r.pool.QueryRow(ctx, query, id).Scan(
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
			r.logger.Warn("数据库连接配置不存在", zap.Int64("connection_id", id))
			return nil, fmt.Errorf("数据库连接配置不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("获取数据库连接配置失败",
			zap.Int64("connection_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取数据库连接配置失败: %w", err)
	}
	
	return conn, nil
}

// Update 更新数据库连接配置
func (r *PostgreSQLConnectionRepository) Update(ctx context.Context, conn *repository.DatabaseConnection) error {
	const query = `
		UPDATE database_connections 
		SET name = $2, host = $3, port = $4, database_name = $5, 
			username = $6, password_encrypted = $7, db_type = $8, 
			status = $9, last_tested = $10, update_by = $11, update_time = $12
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.pool.Exec(ctx, query,
		conn.ID,
		conn.Name,
		conn.Host,
		conn.Port,
		conn.DatabaseName,
		conn.Username,
		conn.PasswordEncrypted,
		conn.DBType,
		conn.Status,
		conn.LastTested,
		conn.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("更新数据库连接配置失败",
			zap.Int64("connection_id", conn.ID),
			zap.String("name", conn.Name),
			zap.Error(err),
		)
		
		if isUniqueViolation(err) {
			return fmt.Errorf("连接名称已存在: %w", repository.ErrDuplicateEntry)
		}
		
		return fmt.Errorf("更新数据库连接配置失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("数据库连接配置不存在或已删除", zap.Int64("connection_id", conn.ID))
		return fmt.Errorf("数据库连接配置不存在或已删除: %w", repository.ErrNotFound)
	}
	
	conn.UpdateTime = now
	
	r.logger.Info("数据库连接配置更新成功",
		zap.Int64("connection_id", conn.ID),
		zap.String("name", conn.Name),
	)
	
	return nil
}

// Delete 软删除数据库连接配置
func (r *PostgreSQLConnectionRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE database_connections 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("删除数据库连接配置失败",
			zap.Int64("connection_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除数据库连接配置失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("数据库连接配置不存在或已删除", zap.Int64("connection_id", id))
		return fmt.Errorf("数据库连接配置不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("数据库连接配置删除成功", zap.Int64("connection_id", id))
	return nil
}

// ListByUser 根据用户ID获取数据库连接配置列表
func (r *PostgreSQLConnectionRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE user_id = $1 AND is_deleted = false 
		ORDER BY create_time DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("根据用户ID获取连接配置列表失败",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据用户ID获取连接配置列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanConnections(rows)
}

// ListByType 根据数据库类型获取连接配置列表
func (r *PostgreSQLConnectionRepository) ListByType(ctx context.Context, dbType repository.DatabaseType) ([]*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE db_type = $1 AND is_deleted = false 
		ORDER BY create_time DESC`

	rows, err := r.pool.Query(ctx, query, string(dbType))
	if err != nil {
		r.logger.Error("根据数据库类型获取连接配置列表失败",
			zap.String("db_type", string(dbType)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据数据库类型获取连接配置列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanConnections(rows)
}

// ListByStatus 根据连接状态获取连接配置列表
func (r *PostgreSQLConnectionRepository) ListByStatus(ctx context.Context, status repository.ConnectionStatus) ([]*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE status = $1 AND is_deleted = false 
		ORDER BY create_time DESC`

	rows, err := r.pool.Query(ctx, query, string(status))
	if err != nil {
		r.logger.Error("根据连接状态获取连接配置列表失败",
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据连接状态获取连接配置列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanConnections(rows)
}

// GetByUserAndName 根据用户ID和连接名称获取连接配置
func (r *PostgreSQLConnectionRepository) GetByUserAndName(ctx context.Context, userID int64, name string) (*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE user_id = $1 AND name = $2 AND is_deleted = false`

	conn := &repository.DatabaseConnection{}
	
	err := r.pool.QueryRow(ctx, query, userID, name).Scan(
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
			r.logger.Warn("数据库连接配置不存在",
				zap.Int64("user_id", userID),
				zap.String("name", name),
			)
			return nil, fmt.Errorf("数据库连接配置不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("根据用户和名称获取连接配置失败",
			zap.Int64("user_id", userID),
			zap.String("name", name),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据用户和名称获取连接配置失败: %w", err)
	}
	
	return conn, nil
}

// CountByUser 统计用户的连接配置数量
func (r *PostgreSQLConnectionRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM database_connections WHERE user_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		r.logger.Error("统计用户连接配置数量失败",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("统计用户连接配置数量失败: %w", err)
	}
	
	return count, nil
}

// CountByStatus 根据状态统计连接配置数量
func (r *PostgreSQLConnectionRepository) CountByStatus(ctx context.Context, status repository.ConnectionStatus) (int64, error) {
	const query = `SELECT COUNT(*) FROM database_connections WHERE status = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, string(status)).Scan(&count)
	if err != nil {
		r.logger.Error("根据状态统计连接配置数量失败",
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return 0, fmt.Errorf("根据状态统计连接配置数量失败: %w", err)
	}
	
	return count, nil
}

// CountByType 根据数据库类型统计连接配置数量
func (r *PostgreSQLConnectionRepository) CountByType(ctx context.Context, dbType repository.DatabaseType) (int64, error) {
	const query = `SELECT COUNT(*) FROM database_connections WHERE db_type = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, string(dbType)).Scan(&count)
	if err != nil {
		r.logger.Error("根据数据库类型统计连接配置数量失败",
			zap.String("db_type", string(dbType)),
			zap.Error(err),
		)
		return 0, fmt.Errorf("根据数据库类型统计连接配置数量失败: %w", err)
	}
	
	return count, nil
}

// UpdateStatus 更新连接状态
func (r *PostgreSQLConnectionRepository) UpdateStatus(ctx context.Context, connectionID int64, status repository.ConnectionStatus) error {
	const query = `
		UPDATE database_connections 
		SET status = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, connectionID, string(status), now)
	
	if err != nil {
		r.logger.Error("更新连接状态失败",
			zap.Int64("connection_id", connectionID),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("更新连接状态失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("数据库连接配置不存在或已删除", zap.Int64("connection_id", connectionID))
		return fmt.Errorf("数据库连接配置不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("连接状态更新成功",
		zap.Int64("connection_id", connectionID),
		zap.String("status", string(status)),
	)
	
	return nil
}

// UpdateLastTested 更新最后测试时间
func (r *PostgreSQLConnectionRepository) UpdateLastTested(ctx context.Context, connectionID int64, testTime time.Time) error {
	const query = `
		UPDATE database_connections 
		SET last_tested = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, connectionID, testTime.UTC(), now)
	
	if err != nil {
		r.logger.Error("更新最后测试时间失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return fmt.Errorf("更新最后测试时间失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("数据库连接配置不存在或已删除", zap.Int64("connection_id", connectionID))
		return fmt.Errorf("数据库连接配置不存在或已删除: %w", repository.ErrNotFound)
	}
	
	return nil
}

// BatchUpdateStatus 批量更新连接状态
func (r *PostgreSQLConnectionRepository) BatchUpdateStatus(ctx context.Context, connectionIDs []int64, status repository.ConnectionStatus) error {
	if len(connectionIDs) == 0 {
		return nil
	}

	const query = `
		UPDATE database_connections 
		SET status = $1, update_time = $2
		WHERE id = ANY($3) AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, string(status), now, connectionIDs)
	
	if err != nil {
		r.logger.Error("批量更新连接状态失败",
			zap.Int("connection_count", len(connectionIDs)),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("批量更新连接状态失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	r.logger.Info("批量连接状态更新完成",
		zap.Int("request_count", len(connectionIDs)),
		zap.Int64("updated_count", rowsAffected),
		zap.String("status", string(status)),
	)
	
	return nil
}

// ExistsByUserAndName 检查用户的连接名称是否存在
func (r *PostgreSQLConnectionRepository) ExistsByUserAndName(ctx context.Context, userID int64, name string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM database_connections 
			WHERE user_id = $1 AND name = $2 AND is_deleted = false
		)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, userID, name).Scan(&exists)
	if err != nil {
		r.logger.Error("检查连接名称是否存在失败",
			zap.Int64("user_id", userID),
			zap.String("name", name),
			zap.Error(err),
		)
		return false, fmt.Errorf("检查连接名称是否存在失败: %w", err)
	}
	
	return exists, nil
}

// GetActiveConnections 获取所有活跃的连接配置
func (r *PostgreSQLConnectionRepository) GetActiveConnections(ctx context.Context) ([]*repository.DatabaseConnection, error) {
	const query = `
		SELECT id, user_id, name, host, port, database_name, username, 
			password_encrypted, db_type, status, last_tested,
			create_by, create_time, update_by, update_time, is_deleted
		FROM database_connections 
		WHERE status = 'active' AND is_deleted = false 
		ORDER BY last_tested DESC NULLS LAST`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		r.logger.Error("获取活跃连接配置列表失败", zap.Error(err))
		return nil, fmt.Errorf("获取活跃连接配置列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanConnections(rows)
}

// scanConnections 扫描连接配置列表
func (r *PostgreSQLConnectionRepository) scanConnections(rows pgx.Rows) ([]*repository.DatabaseConnection, error) {
	var connections []*repository.DatabaseConnection
	
	for rows.Next() {
		conn := &repository.DatabaseConnection{}
		err := rows.Scan(
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
			r.logger.Error("扫描连接配置数据失败", zap.Error(err))
			return nil, fmt.Errorf("扫描连接配置数据失败: %w", err)
		}
		
		connections = append(connections, conn)
	}
	
	if err := rows.Err(); err != nil {
		r.logger.Error("处理连接配置查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理连接配置查询结果失败: %w", err)
	}
	
	return connections, nil
}