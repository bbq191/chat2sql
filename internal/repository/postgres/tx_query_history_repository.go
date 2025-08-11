package postgres

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PostgreSQLTxQueryHistoryRepository PostgreSQL事务版查询历史Repository实现
// 基于pgx.Tx实现，所有操作在事务上下文中执行
type PostgreSQLTxQueryHistoryRepository struct {
	tx     pgx.Tx       // PostgreSQL事务
	logger *zap.Logger  // 结构化日志器
}

// NewPostgreSQLTxQueryHistoryRepository 创建PostgreSQL事务版查询历史Repository
func NewPostgreSQLTxQueryHistoryRepository(tx pgx.Tx, logger *zap.Logger) repository.QueryHistoryRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLTxQueryHistoryRepository{
		tx:     tx,
		logger: logger,
	}
}

// Create 创建查询历史记录（事务版本）
func (r *PostgreSQLTxQueryHistoryRepository) Create(ctx context.Context, query *repository.QueryHistory) error {
	const sqlQuery = `
		INSERT INTO query_history (user_id, natural_query, generated_sql, sql_hash,
			execution_time, result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.tx.QueryRow(ctx, sqlQuery,
		query.UserID,
		query.NaturalQuery,
		query.GeneratedSQL,
		query.SQLHash,
		query.ExecutionTime,
		query.ResultRows,
		query.Status,
		query.ErrorMessage,
		query.ConnectionID,
		query.CreateBy,
		now,
		query.UpdateBy,
		now,
		false,
	).Scan(&query.ID)
	
	if err != nil {
		r.logger.Error("Failed to create query history in transaction", 
			zap.Error(err),
			zap.Int64("user_id", query.UserID))
		return fmt.Errorf("failed to create query history: %w", err)
	}
	
	r.logger.Info("Query history created successfully in transaction",
		zap.Int64("query_id", query.ID),
		zap.Int64("user_id", query.UserID))
	
	return nil
}

// GetByID 根据ID获取查询历史（事务版本）
func (r *PostgreSQLTxQueryHistoryRepository) GetByID(ctx context.Context, id int64) (*repository.QueryHistory, error) {
	const query = `
		SELECT id, user_id, natural_query, generated_sql, sql_hash,
			execution_time, result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history
		WHERE id = $1 AND is_deleted = false`

	query_history := &repository.QueryHistory{}
	
	err := r.tx.QueryRow(ctx, query, id).Scan(
		&query_history.ID,
		&query_history.UserID,
		&query_history.NaturalQuery,
		&query_history.GeneratedSQL,
		&query_history.SQLHash,
		&query_history.ExecutionTime,
		&query_history.ResultRows,
		&query_history.Status,
		&query_history.ErrorMessage,
		&query_history.ConnectionID,
		&query_history.CreateBy,
		&query_history.CreateTime,
		&query_history.UpdateBy,
		&query_history.UpdateTime,
		&query_history.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("query history not found")
		}
		r.logger.Error("Failed to get query history by ID in transaction",
			zap.Error(err), zap.Int64("query_id", id))
		return nil, fmt.Errorf("failed to get query history: %w", err)
	}
	
	return query_history, nil
}

// Update 更新查询历史（事务版本）
func (r *PostgreSQLTxQueryHistoryRepository) Update(ctx context.Context, query *repository.QueryHistory) error {
	const sqlQuery = `
		UPDATE query_history 
		SET execution_time = $2, result_rows = $3, status = $4, 
			error_message = $5, update_by = $6, update_time = $7
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, sqlQuery,
		query.ID,
		query.ExecutionTime,
		query.ResultRows,
		query.Status,
		query.ErrorMessage,
		query.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("Failed to update query history in transaction",
			zap.Error(err), zap.Int64("query_id", query.ID))
		return fmt.Errorf("failed to update query history: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("query history not found or already deleted")
	}
	
	r.logger.Info("Query history updated successfully in transaction",
		zap.Int64("query_id", query.ID))
	
	return nil
}

// Delete 软删除查询历史（事务版本）
func (r *PostgreSQLTxQueryHistoryRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE query_history 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("Failed to delete query history in transaction",
			zap.Error(err), zap.Int64("query_id", id))
		return fmt.Errorf("failed to delete query history: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("query history not found or already deleted")
	}
	
	r.logger.Info("Query history deleted successfully in transaction",
		zap.Int64("query_id", id))
	
	return nil
}

// 以下方法使用简化实现，主要用于满足接口要求

// ListByUser 获取用户查询历史列表（事务版本简化实现）
func (r *PostgreSQLTxQueryHistoryRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	// 简化实现，仅支持基本查询
	const query = `
		SELECT id, user_id, natural_query, generated_sql, sql_hash,
			execution_time, result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history
		WHERE user_id = $1 AND is_deleted = false
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.tx.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list query history: %w", err)
	}
	defer rows.Close()

	var results []*repository.QueryHistory
	for rows.Next() {
		qh := &repository.QueryHistory{}
		err := rows.Scan(
			&qh.ID, &qh.UserID, &qh.NaturalQuery, &qh.GeneratedSQL, &qh.SQLHash,
			&qh.ExecutionTime, &qh.ResultRows, &qh.Status, &qh.ErrorMessage, &qh.ConnectionID,
			&qh.CreateBy, &qh.CreateTime, &qh.UpdateBy, &qh.UpdateTime, &qh.IsDeleted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan query history: %w", err)
		}
		results = append(results, qh)
	}

	return results, nil
}

// CountByUser 统计用户查询数量（事务版本）
func (r *PostgreSQLTxQueryHistoryRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM query_history WHERE user_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.tx.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count user queries: %w", err)
	}
	
	return count, nil
}

// GetExecutionStats 获取执行统计（事务版本简化实现）
func (r *PostgreSQLTxQueryHistoryRepository) GetExecutionStats(ctx context.Context, userID int64, days int) (*repository.QueryExecutionStats, error) {
	const query = `
		SELECT 
			COUNT(*) as total_queries,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful_queries,
			AVG(CASE WHEN execution_time IS NOT NULL THEN execution_time END) as avg_execution_time
		FROM query_history 
		WHERE user_id = $1 AND is_deleted = false 
			AND create_time >= NOW() - INTERVAL '%d days'`

	stats := &repository.QueryExecutionStats{}
	var avgTime *float64
	
	err := r.tx.QueryRow(ctx, fmt.Sprintf(query, days), userID).Scan(
		&stats.TotalQueries,
		&stats.SuccessfulQueries,
		&avgTime,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get execution stats: %w", err)
	}
	
	if avgTime != nil {
		stats.AverageExecutionTime = *avgTime
	}
	
	return stats, nil
}

// 其他接口方法的简化实现，返回未实现错误
func (r *PostgreSQLTxQueryHistoryRepository) ListByConnection(ctx context.Context, connectionID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("ListByConnection not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) ListByStatus(ctx context.Context, status repository.QueryStatus, limit, offset int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("ListByStatus not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) ListRecent(ctx context.Context, userID int64, hours int, limit int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("ListRecent not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) CountByStatus(ctx context.Context, status repository.QueryStatus) (int64, error) {
	return 0, fmt.Errorf("CountByStatus not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) GetPopularQueries(ctx context.Context, limit int, days int) ([]*repository.PopularQuery, error) {
	return nil, fmt.Errorf("GetPopularQueries not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) GetSlowQueries(ctx context.Context, minExecutionTime int32, limit int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("GetSlowQueries not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) SearchByNaturalQuery(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("SearchByNaturalQuery not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) SearchBySQL(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	return nil, fmt.Errorf("SearchBySQL not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) BatchUpdateStatus(ctx context.Context, queryIDs []int64, status repository.QueryStatus) error {
	return fmt.Errorf("BatchUpdateStatus not implemented in transaction version")
}

func (r *PostgreSQLTxQueryHistoryRepository) CleanupOldQueries(ctx context.Context, beforeDate time.Time) (int64, error) {
	return 0, fmt.Errorf("CleanupOldQueries not implemented in transaction version")
}