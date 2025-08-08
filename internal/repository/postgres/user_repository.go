package postgres

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PostgreSQLUserRepository PostgreSQL用户Repository实现
// 基于pgx/v5实现，支持连接池、事务、批量操作
type PostgreSQLUserRepository struct {
	pool   *pgxpool.Pool // PostgreSQL连接池
	logger *zap.Logger   // 结构化日志器
}

// NewPostgreSQLUserRepository 创建PostgreSQL用户Repository
func NewPostgreSQLUserRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.UserRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLUserRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create 创建新用户
// 自动填充创建时间和更新时间，检查用户名和邮箱唯一性
func (r *PostgreSQLUserRepository) Create(ctx context.Context, user *repository.User) error {
	const query = `
		INSERT INTO users (username, email, password_hash, role, status, 
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.pool.QueryRow(ctx, query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.Status,
		user.CreateBy,    // 可为空，系统创建时为nil
		now,              // create_time
		user.UpdateBy,    // 可为空
		now,              // update_time
		false,            // is_deleted
	).Scan(&user.ID)
	
	if err != nil {
		r.logger.Error("创建用户失败",
			zap.String("username", user.Username),
			zap.String("email", user.Email),
			zap.Error(err),
		)
		
		// 检查是否是唯一性约束违反
		if isUniqueViolation(err) {
			return fmt.Errorf("用户名或邮箱已存在: %w", err)
		}
		
		return fmt.Errorf("创建用户失败: %w", err)
	}
	
	// 更新模型的时间字段
	user.CreateTime = now
	user.UpdateTime = now
	user.IsDeleted = false
	
	r.logger.Info("用户创建成功",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("role", user.Role),
	)
	
	return nil
}

// GetByID 根据ID获取用户
// 只返回未删除的用户
func (r *PostgreSQLUserRepository) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE id = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreateBy,
		&user.CreateTime,
		&user.UpdateBy,
		&user.UpdateTime,
		&user.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			r.logger.Warn("用户不存在", zap.Int64("user_id", id))
			return nil, fmt.Errorf("用户不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("查询用户失败",
			zap.Int64("user_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	
	return user, nil
}

// GetByUsername 根据用户名获取用户
func (r *PostgreSQLUserRepository) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE username = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreateBy,
		&user.CreateTime,
		&user.UpdateBy,
		&user.UpdateTime,
		&user.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			r.logger.Warn("用户不存在", zap.String("username", username))
			return nil, fmt.Errorf("用户不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("根据用户名查询用户失败",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据用户名查询用户失败: %w", err)
	}
	
	return user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *PostgreSQLUserRepository) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE email = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreateBy,
		&user.CreateTime,
		&user.UpdateBy,
		&user.UpdateTime,
		&user.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			r.logger.Warn("用户不存在", zap.String("email", email))
			return nil, fmt.Errorf("用户不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("根据邮箱查询用户失败",
			zap.String("email", email),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据邮箱查询用户失败: %w", err)
	}
	
	return user, nil
}

// Update 更新用户信息
// 自动更新update_time字段
func (r *PostgreSQLUserRepository) Update(ctx context.Context, user *repository.User) error {
	const query = `
		UPDATE users 
		SET username = $2, email = $3, password_hash = $4, role = $5, 
			status = $6, update_by = $7, update_time = $8
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.Status,
		user.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("更新用户失败",
			zap.Int64("user_id", user.ID),
			zap.String("username", user.Username),
			zap.Error(err),
		)
		
		if isUniqueViolation(err) {
			return fmt.Errorf("用户名或邮箱已存在: %w", err)
		}
		
		return fmt.Errorf("更新用户失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("用户不存在或已删除", zap.Int64("user_id", user.ID))
		return fmt.Errorf("用户不存在或已删除: %w", repository.ErrNotFound)
	}
	
	// 更新模型的时间字段
	user.UpdateTime = now
	
	r.logger.Info("用户更新成功",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
	)
	
	return nil
}

// Delete 软删除用户
// 设置is_deleted=true，保留历史数据
func (r *PostgreSQLUserRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE users 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("删除用户失败",
			zap.Int64("user_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除用户失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("用户不存在或已删除", zap.Int64("user_id", id))
		return fmt.Errorf("用户不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("用户删除成功", zap.Int64("user_id", id))
	return nil
}

// List 分页获取用户列表
func (r *PostgreSQLUserRepository) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		r.logger.Error("查询用户列表失败",
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanUsers(rows)
}

// ListByRole 根据角色分页获取用户列表
func (r *PostgreSQLUserRepository) ListByRole(ctx context.Context, role repository.UserRole, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE role = $1 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, string(role), limit, offset)
	if err != nil {
		r.logger.Error("根据角色查询用户列表失败",
			zap.String("role", string(role)),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据角色查询用户列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanUsers(rows)
}

// ListByStatus 根据状态分页获取用户列表
func (r *PostgreSQLUserRepository) ListByStatus(ctx context.Context, status repository.UserStatus, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE status = $1 AND is_deleted = false 
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, string(status), limit, offset)
	if err != nil {
		r.logger.Error("根据状态查询用户列表失败",
			zap.String("status", string(status)),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据状态查询用户列表失败: %w", err)
	}
	defer rows.Close()

	return r.scanUsers(rows)
}

// Count 获取用户总数
func (r *PostgreSQLUserRepository) Count(ctx context.Context) (int64, error) {
	const query = `SELECT COUNT(*) FROM users WHERE is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		r.logger.Error("统计用户数量失败", zap.Error(err))
		return 0, fmt.Errorf("统计用户数量失败: %w", err)
	}
	
	return count, nil
}

// CountByStatus 根据状态统计用户数量
func (r *PostgreSQLUserRepository) CountByStatus(ctx context.Context, status repository.UserStatus) (int64, error) {
	const query = `SELECT COUNT(*) FROM users WHERE status = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, string(status)).Scan(&count)
	if err != nil {
		r.logger.Error("根据状态统计用户数量失败",
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return 0, fmt.Errorf("根据状态统计用户数量失败: %w", err)
	}
	
	return count, nil
}

// ValidateCredentials 验证用户凭据
// 用于登录验证，返回用户信息
func (r *PostgreSQLUserRepository) ValidateCredentials(ctx context.Context, username, passwordHash string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE (username = $1 OR email = $1) AND password_hash = $2 
			AND status = 'active' AND is_deleted = false`

	user := &repository.User{}
	
	err := r.pool.QueryRow(ctx, query, username, passwordHash).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreateBy,
		&user.CreateTime,
		&user.UpdateBy,
		&user.UpdateTime,
		&user.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			r.logger.Warn("用户凭据验证失败", zap.String("username", username))
			return nil, fmt.Errorf("用户名或密码错误: %w", repository.ErrInvalidCredentials)
		}
		
		r.logger.Error("验证用户凭据失败",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, fmt.Errorf("验证用户凭据失败: %w", err)
	}
	
	r.logger.Info("用户凭据验证成功",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
	)
	
	return user, nil
}

// UpdatePassword 更新用户密码
func (r *PostgreSQLUserRepository) UpdatePassword(ctx context.Context, userID int64, newPasswordHash string) error {
	const query = `
		UPDATE users 
		SET password_hash = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, userID, newPasswordHash, now)
	
	if err != nil {
		r.logger.Error("更新用户密码失败",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("更新用户密码失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("用户不存在或已删除", zap.Int64("user_id", userID))
		return fmt.Errorf("用户不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("用户密码更新成功", zap.Int64("user_id", userID))
	return nil
}

// UpdateLastLogin 更新用户最后登录时间
// 实际上更新update_time字段记录最后活跃时间
func (r *PostgreSQLUserRepository) UpdateLastLogin(ctx context.Context, userID int64, loginTime time.Time) error {
	const query = `
		UPDATE users 
		SET update_time = $2
		WHERE id = $1 AND is_deleted = false`

	result, err := r.pool.Exec(ctx, query, userID, loginTime.UTC())
	
	if err != nil {
		r.logger.Error("更新用户最后登录时间失败",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("更新用户最后登录时间失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("用户不存在或已删除", zap.Int64("user_id", userID))
		return fmt.Errorf("用户不存在或已删除: %w", repository.ErrNotFound)
	}
	
	return nil
}

// UpdateStatus 更新用户状态
func (r *PostgreSQLUserRepository) UpdateStatus(ctx context.Context, userID int64, status repository.UserStatus) error {
	const query = `
		UPDATE users 
		SET status = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, userID, string(status), now)
	
	if err != nil {
		r.logger.Error("更新用户状态失败",
			zap.Int64("user_id", userID),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("更新用户状态失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("用户不存在或已删除", zap.Int64("user_id", userID))
		return fmt.Errorf("用户不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Info("用户状态更新成功",
		zap.Int64("user_id", userID),
		zap.String("status", string(status)),
	)
	
	return nil
}

// BatchUpdateStatus 批量更新用户状态
func (r *PostgreSQLUserRepository) BatchUpdateStatus(ctx context.Context, userIDs []int64, status repository.UserStatus) error {
	if len(userIDs) == 0 {
		return nil
	}

	const query = `
		UPDATE users 
		SET status = $1, update_time = $2
		WHERE id = ANY($3) AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, string(status), now, userIDs)
	
	if err != nil {
		r.logger.Error("批量更新用户状态失败",
			zap.Int("user_count", len(userIDs)),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("批量更新用户状态失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	r.logger.Info("批量用户状态更新完成",
		zap.Int("request_count", len(userIDs)),
		zap.Int64("updated_count", rowsAffected),
		zap.String("status", string(status)),
	)
	
	return nil
}

// ExistsByUsername 检查用户名是否存在
func (r *PostgreSQLUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND is_deleted = false)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		r.logger.Error("检查用户名是否存在失败",
			zap.String("username", username),
			zap.Error(err),
		)
		return false, fmt.Errorf("检查用户名是否存在失败: %w", err)
	}
	
	return exists, nil
}

// ExistsByEmail 检查邮箱是否存在
func (r *PostgreSQLUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND is_deleted = false)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		r.logger.Error("检查邮箱是否存在失败",
			zap.String("email", email),
			zap.Error(err),
		)
		return false, fmt.Errorf("检查邮箱是否存在失败: %w", err)
	}
	
	return exists, nil
}

// scanUsers 扫描用户列表
// 通用的行扫描方法，减少代码重复
func (r *PostgreSQLUserRepository) scanUsers(rows pgx.Rows) ([]*repository.User, error) {
	var users []*repository.User
	
	for rows.Next() {
		user := &repository.User{}
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.Status,
			&user.CreateBy,
			&user.CreateTime,
			&user.UpdateBy,
			&user.UpdateTime,
			&user.IsDeleted,
		)
		
		if err != nil {
			r.logger.Error("扫描用户数据失败", zap.Error(err))
			return nil, fmt.Errorf("扫描用户数据失败: %w", err)
		}
		
		users = append(users, user)
	}
	
	if err := rows.Err(); err != nil {
		r.logger.Error("处理用户查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理用户查询结果失败: %w", err)
	}
	
	return users, nil
}

// isUniqueViolation 检查是否是唯一性约束违反错误
func isUniqueViolation(err error) bool {
	// pgx的错误处理，检查PostgreSQL错误代码23505（unique violation）
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23505"
	}
	return false
}