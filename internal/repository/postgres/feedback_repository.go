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

// PostgreSQLFeedbackRepository PostgreSQL反馈Repository实现
type PostgreSQLFeedbackRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgreSQLFeedbackRepository 创建PostgreSQL反馈Repository实例
func NewPostgreSQLFeedbackRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.FeedbackRepository {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &PostgreSQLFeedbackRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create 创建反馈记录
func (r *PostgreSQLFeedbackRepository) Create(ctx context.Context, feedback *repository.Feedback) error {
	query := `
		INSERT INTO feedbacks (
			query_id, user_id, user_query, generated_sql, expected_sql,
			is_correct, user_rating, feedback_text, category, difficulty,
			error_type, error_details, processing_time, tokens_used, model_used,
			connection_id, create_by, create_time, update_by, update_time, is_deleted
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		) RETURNING id`

	now := time.Now()
	err := r.pool.QueryRow(ctx, query,
		feedback.QueryID, feedback.UserID, feedback.UserQuery, feedback.GeneratedSQL, feedback.ExpectedSQL,
		feedback.IsCorrect, feedback.UserRating, feedback.FeedbackText, feedback.Category, feedback.Difficulty,
		feedback.ErrorType, feedback.ErrorDetails, feedback.ProcessingTime, feedback.TokensUsed, feedback.ModelUsed,
		feedback.ConnectionID, feedback.UserID, now, feedback.UserID, now, false,
	).Scan(&feedback.ID)

	if err != nil {
		r.logger.Error("创建反馈记录失败", 
			zap.String("query_id", feedback.QueryID), 
			zap.Error(err))
		return fmt.Errorf("创建反馈记录失败: %w", err)
	}

	feedback.CreateTime = now
	feedback.UpdateTime = now

	r.logger.Debug("反馈记录创建成功", 
		zap.Int64("feedback_id", feedback.ID),
		zap.String("query_id", feedback.QueryID))

	return nil
}

// GetByID 根据ID获取反馈记录
func (r *PostgreSQLFeedbackRepository) GetByID(ctx context.Context, id int64) (*repository.Feedback, error) {
	query := `
		SELECT id, query_id, user_id, user_query, generated_sql, expected_sql,
			   is_correct, user_rating, feedback_text, category, difficulty,
			   error_type, error_details, processing_time, tokens_used, model_used,
			   connection_id, create_by, create_time, update_by, update_time, is_deleted
		FROM feedbacks WHERE id = $1 AND is_deleted = false`

	feedback := &repository.Feedback{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&feedback.ID, &feedback.QueryID, &feedback.UserID, &feedback.UserQuery, &feedback.GeneratedSQL, &feedback.ExpectedSQL,
		&feedback.IsCorrect, &feedback.UserRating, &feedback.FeedbackText, &feedback.Category, &feedback.Difficulty,
		&feedback.ErrorType, &feedback.ErrorDetails, &feedback.ProcessingTime, &feedback.TokensUsed, &feedback.ModelUsed,
		&feedback.ConnectionID, &feedback.CreateBy, &feedback.CreateTime, &feedback.UpdateBy, &feedback.UpdateTime, &feedback.IsDeleted,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("反馈记录不存在: ID=%d", id)
		}
		r.logger.Error("获取反馈记录失败", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("获取反馈记录失败: %w", err)
	}

	return feedback, nil
}

// GetByQueryID 根据查询ID获取反馈记录
func (r *PostgreSQLFeedbackRepository) GetByQueryID(ctx context.Context, queryID string) (*repository.Feedback, error) {
	query := `
		SELECT id, query_id, user_id, user_query, generated_sql, expected_sql,
			   is_correct, user_rating, feedback_text, category, difficulty,
			   error_type, error_details, processing_time, tokens_used, model_used,
			   connection_id, create_by, create_time, update_by, update_time, is_deleted
		FROM feedbacks WHERE query_id = $1 AND is_deleted = false
		ORDER BY create_time DESC LIMIT 1`

	feedback := &repository.Feedback{}
	err := r.pool.QueryRow(ctx, query, queryID).Scan(
		&feedback.ID, &feedback.QueryID, &feedback.UserID, &feedback.UserQuery, &feedback.GeneratedSQL, &feedback.ExpectedSQL,
		&feedback.IsCorrect, &feedback.UserRating, &feedback.FeedbackText, &feedback.Category, &feedback.Difficulty,
		&feedback.ErrorType, &feedback.ErrorDetails, &feedback.ProcessingTime, &feedback.TokensUsed, &feedback.ModelUsed,
		&feedback.ConnectionID, &feedback.CreateBy, &feedback.CreateTime, &feedback.UpdateBy, &feedback.UpdateTime, &feedback.IsDeleted,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("反馈记录不存在: query_id=%s", queryID)
		}
		r.logger.Error("获取反馈记录失败", zap.String("query_id", queryID), zap.Error(err))
		return nil, fmt.Errorf("获取反馈记录失败: %w", err)
	}

	return feedback, nil
}

// Update 更新反馈记录
func (r *PostgreSQLFeedbackRepository) Update(ctx context.Context, feedback *repository.Feedback) error {
	query := `
		UPDATE feedbacks SET
			user_query = $2, generated_sql = $3, expected_sql = $4,
			is_correct = $5, user_rating = $6, feedback_text = $7, category = $8, difficulty = $9,
			error_type = $10, error_details = $11, processing_time = $12, tokens_used = $13, model_used = $14,
			connection_id = $15, update_by = $16, update_time = $17
		WHERE id = $1 AND is_deleted = false`

	now := time.Now()
	result, err := r.pool.Exec(ctx, query,
		feedback.ID, feedback.UserQuery, feedback.GeneratedSQL, feedback.ExpectedSQL,
		feedback.IsCorrect, feedback.UserRating, feedback.FeedbackText, feedback.Category, feedback.Difficulty,
		feedback.ErrorType, feedback.ErrorDetails, feedback.ProcessingTime, feedback.TokensUsed, feedback.ModelUsed,
		feedback.ConnectionID, feedback.UserID, now,
	)

	if err != nil {
		r.logger.Error("更新反馈记录失败", zap.Int64("id", feedback.ID), zap.Error(err))
		return fmt.Errorf("更新反馈记录失败: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("反馈记录不存在或已删除: ID=%d", feedback.ID)
	}

	feedback.UpdateTime = now
	r.logger.Debug("反馈记录更新成功", zap.Int64("id", feedback.ID))

	return nil
}

// Delete 软删除反馈记录
func (r *PostgreSQLFeedbackRepository) Delete(ctx context.Context, id int64) error {
	query := `UPDATE feedbacks SET is_deleted = true, update_time = $2 WHERE id = $1 AND is_deleted = false`

	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		r.logger.Error("删除反馈记录失败", zap.Int64("id", id), zap.Error(err))
		return fmt.Errorf("删除反馈记录失败: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("反馈记录不存在: ID=%d", id)
	}

	r.logger.Debug("反馈记录删除成功", zap.Int64("id", id))
	return nil
}

// ListByUser 根据用户ID获取反馈记录列表
func (r *PostgreSQLFeedbackRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*repository.Feedback, error) {
	query := `
		SELECT id, query_id, user_id, user_query, generated_sql, expected_sql,
			   is_correct, user_rating, feedback_text, category, difficulty,
			   error_type, error_details, processing_time, tokens_used, model_used,
			   connection_id, create_by, create_time, update_by, update_time, is_deleted
		FROM feedbacks 
		WHERE user_id = $1 AND is_deleted = false 
		ORDER BY create_time DESC 
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		r.logger.Error("获取用户反馈列表失败", zap.Int64("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("获取用户反馈列表失败: %w", err)
	}
	defer rows.Close()

	var feedbacks []*repository.Feedback
	for rows.Next() {
		feedback := &repository.Feedback{}
		err := rows.Scan(
			&feedback.ID, &feedback.QueryID, &feedback.UserID, &feedback.UserQuery, &feedback.GeneratedSQL, &feedback.ExpectedSQL,
			&feedback.IsCorrect, &feedback.UserRating, &feedback.FeedbackText, &feedback.Category, &feedback.Difficulty,
			&feedback.ErrorType, &feedback.ErrorDetails, &feedback.ProcessingTime, &feedback.TokensUsed, &feedback.ModelUsed,
			&feedback.ConnectionID, &feedback.CreateBy, &feedback.CreateTime, &feedback.UpdateBy, &feedback.UpdateTime, &feedback.IsDeleted,
		)
		if err != nil {
			r.logger.Error("扫描反馈记录失败", zap.Error(err))
			return nil, fmt.Errorf("扫描反馈记录失败: %w", err)
		}
		feedbacks = append(feedbacks, feedback)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("遍历反馈记录失败", zap.Error(err))
		return nil, fmt.Errorf("遍历反馈记录失败: %w", err)
	}

	return feedbacks, nil
}

// 为了简化实现，其他方法暂时返回空实现
// 实际项目中需要逐一完整实现所有接口方法

func (r *PostgreSQLFeedbackRepository) ListByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) ListByCorrectness(ctx context.Context, isCorrect bool, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) ListByRating(ctx context.Context, minRating int, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) ListByCategory(ctx context.Context, category string, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) ListByModel(ctx context.Context, model string, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	return 0, nil
}

func (r *PostgreSQLFeedbackRepository) CountByCorrectness(ctx context.Context, isCorrect bool) (int64, error) {
	return 0, nil
}

func (r *PostgreSQLFeedbackRepository) CountByTimeRange(ctx context.Context, startTime, endTime time.Time) (int64, error) {
	return 0, nil
}

func (r *PostgreSQLFeedbackRepository) GetAccuracyStats(ctx context.Context, startTime, endTime time.Time) (*repository.AccuracyStats, error) {
	return &repository.AccuracyStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) GetRatingStats(ctx context.Context, startTime, endTime time.Time) (*repository.RatingStats, error) {
	return &repository.RatingStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) GetCategoryStats(ctx context.Context, startTime, endTime time.Time) ([]*repository.CategoryFeedbackStats, error) {
	return []*repository.CategoryFeedbackStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) GetModelStats(ctx context.Context, startTime, endTime time.Time) ([]*repository.ModelFeedbackStats, error) {
	return []*repository.ModelFeedbackStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) GetUserStats(ctx context.Context, limit int) ([]*repository.UserFeedbackStats, error) {
	return []*repository.UserFeedbackStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) GetErrorStats(ctx context.Context, startTime, endTime time.Time, limit int) ([]*repository.ErrorStats, error) {
	return []*repository.ErrorStats{}, nil
}

func (r *PostgreSQLFeedbackRepository) SearchByQuery(ctx context.Context, keyword string, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) SearchByFeedback(ctx context.Context, keyword string, limit, offset int) ([]*repository.Feedback, error) {
	return []*repository.Feedback{}, nil
}

func (r *PostgreSQLFeedbackRepository) BatchCreate(ctx context.Context, feedbacks []*repository.Feedback) error {
	return nil
}

func (r *PostgreSQLFeedbackRepository) BatchUpdateProcessed(ctx context.Context, feedbackIDs []int64) error {
	return nil
}

func (r *PostgreSQLFeedbackRepository) CleanupOldFeedbacks(ctx context.Context, beforeDate time.Time) (int64, error) {
	return 0, nil
}