package postgres

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PostgreSQLTxUserRepository PostgreSQL事务版用户Repository实现
// 基于pgx.Tx实现，所有操作在事务上下文中执行
type PostgreSQLTxUserRepository struct {
	tx     pgx.Tx       // PostgreSQL事务
	logger *zap.Logger  // 结构化日志器
}

// NewPostgreSQLTxUserRepository 创建PostgreSQL事务版用户Repository
func NewPostgreSQLTxUserRepository(tx pgx.Tx, logger *zap.Logger) repository.UserRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLTxUserRepository{
		tx:     tx,
		logger: logger,
	}
}

// Create 创建新用户（事务版本）
func (r *PostgreSQLTxUserRepository) Create(ctx context.Context, user *repository.User) error {
	const query = `
		INSERT INTO users (username, email, password_hash, role, status, 
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.tx.QueryRow(ctx, query,
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

// GetByID 根据ID获取用户（事务版本）
func (r *PostgreSQLTxUserRepository) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE id = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.tx.QueryRow(ctx, query, id).Scan(
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
			r.logger.Info("用户不存在", zap.Int64("user_id", id))
			return nil, fmt.Errorf("用户不存在: %d", id)
		}
		
		r.logger.Error("获取用户失败",
			zap.Int64("user_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	
	return user, nil
}

// GetByUsername 根据用户名获取用户（事务版本）
func (r *PostgreSQLTxUserRepository) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE username = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.tx.QueryRow(ctx, query, username).Scan(
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
			return nil, fmt.Errorf("用户不存在: %s", username)
		}
		
		r.logger.Error("根据用户名获取用户失败",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据用户名获取用户失败: %w", err)
	}
	
	return user, nil
}

// GetByEmail 根据邮箱获取用户（事务版本）
func (r *PostgreSQLTxUserRepository) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE email = $1 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.tx.QueryRow(ctx, query, email).Scan(
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
			return nil, fmt.Errorf("用户不存在: %s", email)
		}
		
		r.logger.Error("根据邮箱获取用户失败",
			zap.String("email", email),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据邮箱获取用户失败: %w", err)
	}
	
	return user, nil
}

// Update 更新用户（事务版本）
func (r *PostgreSQLTxUserRepository) Update(ctx context.Context, user *repository.User) error {
	const query = `
		UPDATE users 
		SET username = $2, email = $3, password_hash = $4, role = $5, 
			status = $6, update_by = $7, update_time = $8
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query,
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
			zap.Error(err),
		)
		
		if isUniqueViolation(err) {
			return fmt.Errorf("用户名或邮箱已存在: %w", err)
		}
		
		return fmt.Errorf("更新用户失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("用户不存在或已删除: %d", user.ID)
	}
	
	// 更新模型的时间字段
	user.UpdateTime = now
	
	r.logger.Info("用户更新成功",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
	)
	
	return nil
}

// Delete 删除用户（软删除）（事务版本）
func (r *PostgreSQLTxUserRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE users 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query, id, now)
	if err != nil {
		r.logger.Error("删除用户失败",
			zap.Int64("user_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除用户失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("用户不存在: %d", id)
	}
	
	r.logger.Info("用户删除成功", zap.Int64("user_id", id))
	
	return nil
}

// List 获取用户列表（事务版本）
func (r *PostgreSQLTxUserRepository) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE is_deleted = false
		ORDER BY create_time DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.tx.Query(ctx, query, limit, offset)
	if err != nil {
		r.logger.Error("获取用户列表失败", zap.Error(err))
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}
	defer rows.Close()

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

	if rows.Err() != nil {
		return nil, fmt.Errorf("遍历用户数据失败: %w", rows.Err())
	}

	return users, nil
}

// ListByRole 根据角色获取用户列表（事务版本）
func (r *PostgreSQLTxUserRepository) ListByRole(ctx context.Context, role repository.UserRole, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE role = $1 AND is_deleted = false
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.tx.Query(ctx, query, role, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("根据角色获取用户列表失败: %w", err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("扫描用户数据失败: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// ListByStatus 根据状态获取用户列表（事务版本）
func (r *PostgreSQLTxUserRepository) ListByStatus(ctx context.Context, status repository.UserStatus, limit, offset int) ([]*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE status = $1 AND is_deleted = false
		ORDER BY create_time DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.tx.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("根据状态获取用户列表失败: %w", err)
	}
	defer rows.Close()

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
			return nil, fmt.Errorf("扫描用户数据失败: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// Count 统计用户总数（事务版本）
func (r *PostgreSQLTxUserRepository) Count(ctx context.Context) (int64, error) {
	const query = `SELECT COUNT(*) FROM users WHERE is_deleted = false`

	var count int64
	err := r.tx.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计用户总数失败: %w", err)
	}

	return count, nil
}

// CountByStatus 根据状态统计用户数（事务版本）
func (r *PostgreSQLTxUserRepository) CountByStatus(ctx context.Context, status repository.UserStatus) (int64, error) {
	const query = `SELECT COUNT(*) FROM users WHERE status = $1 AND is_deleted = false`

	var count int64
	err := r.tx.QueryRow(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("根据状态统计用户数失败: %w", err)
	}

	return count, nil
}

// ValidateCredentials 验证用户凭证（事务版本）
func (r *PostgreSQLTxUserRepository) ValidateCredentials(ctx context.Context, username, passwordHash string) (*repository.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, status,
			create_by, create_time, update_by, update_time, is_deleted
		FROM users 
		WHERE username = $1 AND password_hash = $2 AND is_deleted = false`

	user := &repository.User{}
	
	err := r.tx.QueryRow(ctx, query, username, passwordHash).Scan(
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
			return nil, fmt.Errorf("用户名或密码错误")
		}
		return nil, fmt.Errorf("验证用户凭证失败: %w", err)
	}
	
	return user, nil
}

// UpdatePassword 更新用户密码（事务版本）
func (r *PostgreSQLTxUserRepository) UpdatePassword(ctx context.Context, userID int64, newPasswordHash string) error {
	const query = `
		UPDATE users 
		SET password_hash = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query, userID, newPasswordHash, now)
	if err != nil {
		return fmt.Errorf("更新用户密码失败: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("用户不存在: %d", userID)
	}
	
	return nil
}

// UpdateLastLogin 更新最后登录时间（事务版本）
func (r *PostgreSQLTxUserRepository) UpdateLastLogin(ctx context.Context, userID int64, loginTime time.Time) error {
	const query = `
		UPDATE users 
		SET last_login_time = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query, userID, loginTime, now)
	if err != nil {
		return fmt.Errorf("更新最后登录时间失败: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("用户不存在: %d", userID)
	}
	
	return nil
}

// UpdateStatus 更新用户状态（事务版本）
func (r *PostgreSQLTxUserRepository) UpdateStatus(ctx context.Context, userID int64, status repository.UserStatus) error {
	const query = `
		UPDATE users 
		SET status = $2, update_time = $3
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query, userID, status, now)
	if err != nil {
		return fmt.Errorf("更新用户状态失败: %w", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("用户不存在: %d", userID)
	}
	
	return nil
}

// BatchUpdateStatus 批量更新用户状态（事务版本）
func (r *PostgreSQLTxUserRepository) BatchUpdateStatus(ctx context.Context, userIDs []int64, status repository.UserStatus) error {
	if len(userIDs) == 0 {
		return nil
	}

	const query = `
		UPDATE users 
		SET status = $1, update_time = $2
		WHERE id = ANY($3) AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query, status, now, userIDs)
	if err != nil {
		return fmt.Errorf("批量更新用户状态失败: %w", err)
	}
	
	r.logger.Info("批量更新用户状态成功",
		zap.Int64("affected_rows", result.RowsAffected()),
		zap.Int("user_count", len(userIDs)),
		zap.String("status", string(status)),
	)
	
	return nil
}

// ExistsByUsername 检查用户名是否存在（事务版本）
func (r *PostgreSQLTxUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND is_deleted = false)`

	var exists bool
	err := r.tx.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("检查用户名存在性失败: %w", err)
	}

	return exists, nil
}

// ExistsByEmail 检查邮箱是否存在（事务版本）
func (r *PostgreSQLTxUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND is_deleted = false)`

	var exists bool
	err := r.tx.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("检查邮箱存在性失败: %w", err)
	}

	return exists, nil
}

