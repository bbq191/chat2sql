package postgres

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PostgreSQLQueryHistoryRepository PostgreSQL查询历史Repository实现
// 支持全文搜索、统计分析、性能监控等高级功能
type PostgreSQLQueryHistoryRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgreSQLQueryHistoryRepository 创建PostgreSQL查询历史Repository
func NewPostgreSQLQueryHistoryRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.QueryHistoryRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLQueryHistoryRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create 创建查询历史记录
func (r *PostgreSQLQueryHistoryRepository) Create(ctx context.Context, query *repository.QueryHistory) error {
	const sqlQuery = `
		INSERT INTO query_history (user_id, natural_query, generated_sql, 
			execution_time, result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.pool.QueryRow(ctx, sqlQuery,
		query.UserID,
		query.NaturalQuery,
		query.GeneratedSQL,
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
		r.logger.Error("创建查询历史记录失败",
			zap.Int64("user_id", query.UserID),
			zap.String("status", query.Status),
			zap.Error(err),
		)
		return fmt.Errorf("创建查询历史记录失败: %w", err)
	}
	
	query.CreateTime = now
	query.UpdateTime = now
	query.IsDeleted = false
	
	r.logger.Info("查询历史记录创建成功",
		zap.Int64("query_id", query.ID),
		zap.Int64("user_id", query.UserID),
		zap.String("status", query.Status),
	)
	
	return nil
}

// GetByID 根据ID获取查询历史记录
func (r *PostgreSQLQueryHistoryRepository) GetByID(ctx context.Context, id int64) (*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE id = $1 AND is_deleted = false`

	query := &repository.QueryHistory{}
	
	err := r.pool.QueryRow(ctx, sqlQuery, id).Scan(
		&query.ID,
		&query.UserID,
		&query.NaturalQuery,
		&query.GeneratedSQL,
		&query.ExecutionTime,
		&query.ResultRows,
		&query.Status,
		&query.ErrorMessage,
		&query.ConnectionID,
		&query.CreateBy,
		&query.CreateTime,
		&query.UpdateBy,
		&query.UpdateTime,
		&query.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			r.logger.Warn("查询历史记录不存在", zap.Int64("query_id", id))
			return nil, fmt.Errorf("查询历史记录不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("获取查询历史记录失败",
			zap.Int64("query_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取查询历史记录失败: %w", err)
	}
	
	return query, nil
}

// Update 更新查询历史记录
func (r *PostgreSQLQueryHistoryRepository) Update(ctx context.Context, query *repository.QueryHistory) error {
	const sqlQuery = `
		UPDATE query_history 
		SET natural_query = $2, generated_sql = $3, execution_time = $4,
			result_rows = $5, status = $6, error_message = $7,
			connection_id = $8, update_by = $9, update_time = $10
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.pool.Exec(ctx, sqlQuery,
		query.ID,
		query.NaturalQuery,
		query.GeneratedSQL,
		query.ExecutionTime,
		query.ResultRows,
		query.Status,
		query.ErrorMessage,
		query.ConnectionID,
		query.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("更新查询历史记录失败",
			zap.Int64("query_id", query.ID),
			zap.Error(err),
		)
		return fmt.Errorf("更新查询历史记录失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("查询历史记录不存在或已删除", zap.Int64("query_id", query.ID))
		return fmt.Errorf("查询历史记录不存在或已删除: %w", repository.ErrNotFound)
	}
	
	query.UpdateTime = now
	
	r.logger.Info("查询历史记录更新成功",
		zap.Int64("query_id", query.ID),
		zap.String("status", query.Status),
	)
	
	return nil
}

// Delete 软删除查询历史记录
func (r *PostgreSQLQueryHistoryRepository) Delete(ctx context.Context, id int64) error {
	const sqlQuery = `
		UPDATE query_history 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, sqlQuery, id, now)
	
	if err != nil {
		r.logger.Error("删除查询历史记录失败",
			zap.Int64("query_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除查询历史记录失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("查询历史记录不存在或已删除", zap.Int64("query_id", id))
		return fmt.Errorf("查询历史记录不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("查询历史记录删除成功", zap.Int64("query_id", id))
	return nil
}

// ListByUser 根据用户ID分页获取查询历史
func (r *PostgreSQLQueryHistoryRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE user_id = $1 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, sqlQuery, userID, limit, offset)
	if err != nil {
		r.logger.Error("根据用户ID查询历史记录失败",
			zap.Int64("user_id", userID),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据用户ID查询历史记录失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// ListByConnection 根据连接ID分页获取查询历史
func (r *PostgreSQLQueryHistoryRepository) ListByConnection(ctx context.Context, connectionID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE connection_id = $1 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, sqlQuery, connectionID, limit, offset)
	if err != nil {
		r.logger.Error("根据连接ID查询历史记录失败",
			zap.Int64("connection_id", connectionID),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据连接ID查询历史记录失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// ListByStatus 根据状态分页获取查询历史
func (r *PostgreSQLQueryHistoryRepository) ListByStatus(ctx context.Context, status repository.QueryStatus, limit, offset int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE status = $1 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, sqlQuery, string(status), limit, offset)
	if err != nil {
		r.logger.Error("根据状态查询历史记录失败",
			zap.String("status", string(status)),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据状态查询历史记录失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// ListRecent 获取用户最近N小时的查询历史
func (r *PostgreSQLQueryHistoryRepository) ListRecent(ctx context.Context, userID int64, hours int, limit int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE user_id = $1 AND create_time >= $2 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $3`

	cutoffTime := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	
	rows, err := r.pool.Query(ctx, sqlQuery, userID, cutoffTime, limit)
	if err != nil {
		r.logger.Error("获取用户最近查询历史失败",
			zap.Int64("user_id", userID),
			zap.Int("hours", hours),
			zap.Int("limit", limit),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取用户最近查询历史失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// CountByUser 根据用户ID统计查询数量
func (r *PostgreSQLQueryHistoryRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	const sqlQuery = `SELECT COUNT(*) FROM query_history WHERE user_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, sqlQuery, userID).Scan(&count)
	if err != nil {
		r.logger.Error("统计用户查询数量失败",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("统计用户查询数量失败: %w", err)
	}
	
	return count, nil
}

// CountByStatus 根据状态统计查询数量
func (r *PostgreSQLQueryHistoryRepository) CountByStatus(ctx context.Context, status repository.QueryStatus) (int64, error) {
	const sqlQuery = `SELECT COUNT(*) FROM query_history WHERE status = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, sqlQuery, string(status)).Scan(&count)
	if err != nil {
		r.logger.Error("根据状态统计查询数量失败",
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return 0, fmt.Errorf("根据状态统计查询数量失败: %w", err)
	}
	
	return count, nil
}

// GetExecutionStats 获取用户查询执行统计信息
func (r *PostgreSQLQueryHistoryRepository) GetExecutionStats(ctx context.Context, userID int64, days int) (*repository.QueryExecutionStats, error) {
	const sqlQuery = `
		SELECT 
			COUNT(*) as total_queries,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful_queries,
			COUNT(CASE WHEN status != 'success' THEN 1 END) as failed_queries,
			COALESCE(AVG(execution_time), 0) as avg_execution_time,
			COALESCE(SUM(execution_time), 0) as total_execution_time
		FROM query_history 
		WHERE user_id = $1 
			AND create_time >= $2 
			AND is_deleted = false 
			AND execution_time IS NOT NULL`

	cutoffTime := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	
	stats := &repository.QueryExecutionStats{UserID: userID}
	
	err := r.pool.QueryRow(ctx, sqlQuery, userID, cutoffTime).Scan(
		&stats.TotalQueries,
		&stats.SuccessfulQueries,
		&stats.FailedQueries,
		&stats.AverageExecutionTime,
		&stats.TotalExecutionTime,
	)
	
	if err != nil {
		r.logger.Error("获取查询执行统计失败",
			zap.Int64("user_id", userID),
			zap.Int("days", days),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取查询执行统计失败: %w", err)
	}
	
	// 计算成功率
	if stats.TotalQueries > 0 {
		stats.SuccessRate = float64(stats.SuccessfulQueries) / float64(stats.TotalQueries)
	}
	
	return stats, nil
}

// GetPopularQueries 获取热门查询统计
func (r *PostgreSQLQueryHistoryRepository) GetPopularQueries(ctx context.Context, limit int, days int) ([]*repository.PopularQuery, error) {
	const sqlQuery = `
		SELECT 
			natural_query,
			COUNT(*) as query_count,
			AVG(CASE WHEN status = 'success' THEN 1.0 ELSE 0.0 END) as success_rate,
			COALESCE(AVG(execution_time), 0) as avg_exec_time
		FROM query_history 
		WHERE create_time >= $1 
			AND is_deleted = false 
			AND natural_query != ''
		GROUP BY natural_query 
		HAVING COUNT(*) > 1
		ORDER BY query_count DESC, success_rate DESC
		LIMIT $2`

	cutoffTime := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	
	rows, err := r.pool.Query(ctx, sqlQuery, cutoffTime, limit)
	if err != nil {
		r.logger.Error("获取热门查询统计失败",
			zap.Int("limit", limit),
			zap.Int("days", days),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取热门查询统计失败: %w", err)
	}
	defer rows.Close()

	var popularQueries []*repository.PopularQuery
	
	for rows.Next() {
		pq := &repository.PopularQuery{}
		err := rows.Scan(
			&pq.NaturalQuery,
			&pq.QueryCount,
			&pq.SuccessRate,
			&pq.AvgExecTime,
		)
		
		if err != nil {
			r.logger.Error("扫描热门查询数据失败", zap.Error(err))
			return nil, fmt.Errorf("扫描热门查询数据失败: %w", err)
		}
		
		popularQueries = append(popularQueries, pq)
	}
	
	if err := rows.Err(); err != nil {
		r.logger.Error("处理热门查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理热门查询结果失败: %w", err)
	}
	
	return popularQueries, nil
}

// GetSlowQueries 获取慢查询列表
func (r *PostgreSQLQueryHistoryRepository) GetSlowQueries(ctx context.Context, minExecutionTime int32, limit int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE execution_time >= $1 AND is_deleted = false 
		ORDER BY execution_time DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, sqlQuery, minExecutionTime, limit)
	if err != nil {
		r.logger.Error("获取慢查询列表失败",
			zap.Int32("min_execution_time", minExecutionTime),
			zap.Int("limit", limit),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取慢查询列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// SearchByNaturalQuery 根据自然语言查询关键字搜索
func (r *PostgreSQLQueryHistoryRepository) SearchByNaturalQuery(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE user_id = $1 
			AND to_tsvector('simple', natural_query) @@ to_tsquery('simple', $2)
			AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $3 OFFSET $4`

	// 处理搜索关键字，支持模糊搜索
	searchTerm := fmt.Sprintf("%s:*", keyword)
	
	rows, err := r.pool.Query(ctx, sqlQuery, userID, searchTerm, limit, offset)
	if err != nil {
		r.logger.Error("根据自然语言查询搜索失败",
			zap.Int64("user_id", userID),
			zap.String("keyword", keyword),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据自然语言查询搜索失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// SearchBySQL 根据SQL语句关键字搜索
func (r *PostgreSQLQueryHistoryRepository) SearchBySQL(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	const sqlQuery = `
		SELECT id, user_id, natural_query, generated_sql, execution_time,
			result_rows, status, error_message, connection_id,
			create_by, create_time, update_by, update_time, is_deleted
		FROM query_history 
		WHERE user_id = $1 
			AND to_tsvector('simple', generated_sql) @@ to_tsquery('simple', $2)
			AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $3 OFFSET $4`

	// 处理搜索关键字，支持模糊搜索
	searchTerm := fmt.Sprintf("%s:*", keyword)
	
	rows, err := r.pool.Query(ctx, sqlQuery, userID, searchTerm, limit, offset)
	if err != nil {
		r.logger.Error("根据SQL语句搜索失败",
			zap.Int64("user_id", userID),
			zap.String("keyword", keyword),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据SQL语句搜索失败: %w", err)
	}
	defer rows.Close()

	return r.scanQueryHistory(rows)
}

// BatchUpdateStatus 批量更新查询状态
func (r *PostgreSQLQueryHistoryRepository) BatchUpdateStatus(ctx context.Context, queryIDs []int64, status repository.QueryStatus) error {
	if len(queryIDs) == 0 {
		return nil
	}

	const sqlQuery = `
		UPDATE query_history 
		SET status = $1, update_time = $2
		WHERE id = ANY($3) AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, sqlQuery, string(status), now, queryIDs)
	
	if err != nil {
		r.logger.Error("批量更新查询状态失败",
			zap.Int("query_count", len(queryIDs)),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("批量更新查询状态失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	r.logger.Info("批量查询状态更新完成",
		zap.Int("request_count", len(queryIDs)),
		zap.Int64("updated_count", rowsAffected),
		zap.String("status", string(status)),
	)
	
	return nil
}

// CleanupOldQueries 清理旧的查询记录
func (r *PostgreSQLQueryHistoryRepository) CleanupOldQueries(ctx context.Context, beforeDate time.Time) (int64, error) {
	const sqlQuery = `
		UPDATE query_history 
		SET is_deleted = true, update_time = $1
		WHERE create_time < $2 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, sqlQuery, now, beforeDate)
	
	if err != nil {
		r.logger.Error("清理旧查询记录失败",
			zap.Time("before_date", beforeDate),
			zap.Error(err),
		)
		return 0, fmt.Errorf("清理旧查询记录失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	r.logger.Info("旧查询记录清理完成",
		zap.Time("before_date", beforeDate),
		zap.Int64("deleted_count", rowsAffected),
	)
	
	return rowsAffected, nil
}

// scanQueryHistory 扫描查询历史列表
func (r *PostgreSQLQueryHistoryRepository) scanQueryHistory(rows pgx.Rows) ([]*repository.QueryHistory, error) {
	var queries []*repository.QueryHistory
	
	for rows.Next() {
		query := &repository.QueryHistory{}
		err := rows.Scan(
			&query.ID,
			&query.UserID,
			&query.NaturalQuery,
			&query.GeneratedSQL,
			&query.ExecutionTime,
			&query.ResultRows,
			&query.Status,
			&query.ErrorMessage,
			&query.ConnectionID,
			&query.CreateBy,
			&query.CreateTime,
			&query.UpdateBy,
			&query.UpdateTime,
			&query.IsDeleted,
		)
		
		if err != nil {
			r.logger.Error("扫描查询历史数据失败", zap.Error(err))
			return nil, fmt.Errorf("扫描查询历史数据失败: %w", err)
		}
		
		queries = append(queries, query)
	}
	
	if err := rows.Err(); err != nil {
		r.logger.Error("处理查询历史结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理查询历史结果失败: %w", err)
	}
	
	return queries, nil
}