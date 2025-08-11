package postgres

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PostgreSQLTxSchemaRepository PostgreSQL事务版元数据Repository实现
// 基于pgx.Tx实现，所有操作在事务上下文中执行
type PostgreSQLTxSchemaRepository struct {
	tx     pgx.Tx       // PostgreSQL事务
	logger *zap.Logger  // 结构化日志器
}

// NewPostgreSQLTxSchemaRepository 创建PostgreSQL事务版元数据Repository
func NewPostgreSQLTxSchemaRepository(tx pgx.Tx, logger *zap.Logger) repository.SchemaRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLTxSchemaRepository{
		tx:     tx,
		logger: logger,
	}
}

// Create 创建元数据记录（事务版本）
func (r *PostgreSQLTxSchemaRepository) Create(ctx context.Context, schema *repository.SchemaMetadata) error {
	const query = `
		INSERT INTO schema_metadata (connection_id, schema_name, table_name, column_name,
			data_type, is_nullable, column_default, is_primary_key, is_foreign_key,
			foreign_table, foreign_column, table_comment, column_comment, ordinal_position,
			create_by, create_time, update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.tx.QueryRow(ctx, query,
		schema.ConnectionID,
		schema.SchemaName,
		schema.TableName,
		schema.ColumnName,
		schema.DataType,
		schema.IsNullable,
		schema.ColumnDefault,
		schema.IsPrimaryKey,
		schema.IsForeignKey,
		schema.ForeignTable,
		schema.ForeignColumn,
		schema.TableComment,
		schema.ColumnComment,
		schema.OrdinalPosition,
		schema.CreateBy,
		now,
		schema.UpdateBy,
		now,
		false,
	).Scan(&schema.ID)
	
	if err != nil {
		r.logger.Error("Failed to create schema metadata in transaction", 
			zap.Error(err),
			zap.Int64("connection_id", schema.ConnectionID))
		return fmt.Errorf("failed to create schema metadata: %w", err)
	}
	
	r.logger.Info("Schema metadata created successfully in transaction",
		zap.Int64("schema_id", schema.ID),
		zap.Int64("connection_id", schema.ConnectionID))
	
	return nil
}

// GetByID 根据ID获取元数据（事务版本）
func (r *PostgreSQLTxSchemaRepository) GetByID(ctx context.Context, id int64) (*repository.SchemaMetadata, error) {
	const query = `
		SELECT id, connection_id, schema_name, table_name, column_name,
			data_type, is_nullable, column_default, is_primary_key, is_foreign_key,
			foreign_table, foreign_column, table_comment, column_comment, ordinal_position,
			create_by, create_time, update_by, update_time, is_deleted
		FROM schema_metadata
		WHERE id = $1 AND is_deleted = false`

	schema := &repository.SchemaMetadata{}
	
	err := r.tx.QueryRow(ctx, query, id).Scan(
		&schema.ID,
		&schema.ConnectionID,
		&schema.SchemaName,
		&schema.TableName,
		&schema.ColumnName,
		&schema.DataType,
		&schema.IsNullable,
		&schema.ColumnDefault,
		&schema.IsPrimaryKey,
		&schema.IsForeignKey,
		&schema.ForeignTable,
		&schema.ForeignColumn,
		&schema.TableComment,
		&schema.ColumnComment,
		&schema.OrdinalPosition,
		&schema.CreateBy,
		&schema.CreateTime,
		&schema.UpdateBy,
		&schema.UpdateTime,
		&schema.IsDeleted,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("schema metadata not found")
		}
		r.logger.Error("Failed to get schema metadata by ID in transaction",
			zap.Error(err), zap.Int64("schema_id", id))
		return nil, fmt.Errorf("failed to get schema metadata: %w", err)
	}
	
	return schema, nil
}

// Update 更新元数据（事务版本）
func (r *PostgreSQLTxSchemaRepository) Update(ctx context.Context, schema *repository.SchemaMetadata) error {
	const query = `
		UPDATE schema_metadata 
		SET data_type = $2, is_nullable = $3, column_default = $4,
			is_primary_key = $5, is_foreign_key = $6, foreign_table = $7,
			foreign_column = $8, table_comment = $9, column_comment = $10,
			update_by = $11, update_time = $12
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	
	result, err := r.tx.Exec(ctx, query,
		schema.ID,
		schema.DataType,
		schema.IsNullable,
		schema.ColumnDefault,
		schema.IsPrimaryKey,
		schema.IsForeignKey,
		schema.ForeignTable,
		schema.ForeignColumn,
		schema.TableComment,
		schema.ColumnComment,
		schema.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("Failed to update schema metadata in transaction",
			zap.Error(err), zap.Int64("schema_id", schema.ID))
		return fmt.Errorf("failed to update schema metadata: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("schema metadata not found or already deleted")
	}
	
	r.logger.Info("Schema metadata updated successfully in transaction",
		zap.Int64("schema_id", schema.ID))
	
	return nil
}

// Delete 软删除元数据（事务版本）
func (r *PostgreSQLTxSchemaRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE schema_metadata 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("Failed to delete schema metadata in transaction",
			zap.Error(err), zap.Int64("schema_id", id))
		return fmt.Errorf("failed to delete schema metadata: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("schema metadata not found or already deleted")
	}
	
	r.logger.Info("Schema metadata deleted successfully in transaction",
		zap.Int64("schema_id", id))
	
	return nil
}

// ListByConnection 获取连接的元数据列表（事务版本）
func (r *PostgreSQLTxSchemaRepository) ListByConnection(ctx context.Context, connectionID int64) ([]*repository.SchemaMetadata, error) {
	const query = `
		SELECT id, connection_id, schema_name, table_name, column_name,
			data_type, is_nullable, column_default, is_primary_key, is_foreign_key,
			foreign_table, foreign_column, table_comment, column_comment, ordinal_position,
			create_by, create_time, update_by, update_time, is_deleted
		FROM schema_metadata
		WHERE connection_id = $1 AND is_deleted = false
		ORDER BY schema_name, table_name, ordinal_position`

	rows, err := r.tx.Query(ctx, query, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema metadata: %w", err)
	}
	defer rows.Close()

	var schemas []*repository.SchemaMetadata
	for rows.Next() {
		schema := &repository.SchemaMetadata{}
		err := rows.Scan(
			&schema.ID, &schema.ConnectionID, &schema.SchemaName, &schema.TableName, &schema.ColumnName,
			&schema.DataType, &schema.IsNullable, &schema.ColumnDefault, &schema.IsPrimaryKey, &schema.IsForeignKey,
			&schema.ForeignTable, &schema.ForeignColumn, &schema.TableComment, &schema.ColumnComment, &schema.OrdinalPosition,
			&schema.CreateBy, &schema.CreateTime, &schema.UpdateBy, &schema.UpdateTime, &schema.IsDeleted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schema metadata: %w", err)
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// BatchCreate 批量创建元数据（事务版本）
func (r *PostgreSQLTxSchemaRepository) BatchCreate(ctx context.Context, schemas []*repository.SchemaMetadata) error {
	if len(schemas) == 0 {
		return nil
	}

	// 为简化实现，逐个创建
	for _, schema := range schemas {
		if err := r.Create(ctx, schema); err != nil {
			return fmt.Errorf("failed to batch create schema metadata: %w", err)
		}
	}

	r.logger.Info("Batch created schema metadata in transaction",
		zap.Int("schema_count", len(schemas)))

	return nil
}

// BatchDelete 批量删除连接的所有元数据（事务版本）
func (r *PostgreSQLTxSchemaRepository) BatchDelete(ctx context.Context, connectionID int64) error {
	const query = `
		UPDATE schema_metadata 
		SET is_deleted = true, update_time = $2
		WHERE connection_id = $1 AND is_deleted = false`
	
	now := time.Now().UTC()
	result, err := r.tx.Exec(ctx, query, connectionID, now)
	
	if err != nil {
		return fmt.Errorf("failed to batch delete schema metadata: %w", err)
	}
	
	r.logger.Info("Batch deleted schema metadata in transaction",
		zap.Int64("connection_id", connectionID),
		zap.Int64("rows_affected", result.RowsAffected()))
	
	return nil
}

// CountByConnection 统计连接的元数据数量（事务版本）
func (r *PostgreSQLTxSchemaRepository) CountByConnection(ctx context.Context, connectionID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM schema_metadata WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.tx.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count schema metadata: %w", err)
	}
	
	return count, nil
}

// GetTableCount 获取连接中的表数量（事务版本）
func (r *PostgreSQLTxSchemaRepository) GetTableCount(ctx context.Context, connectionID int64) (int64, error) {
	const query = `
		SELECT COUNT(DISTINCT CONCAT(schema_name, '.', table_name))
		FROM schema_metadata 
		WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.tx.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tables: %w", err)
	}
	
	return count, nil
}

// GetColumnCount 获取连接中的列数量（事务版本）
func (r *PostgreSQLTxSchemaRepository) GetColumnCount(ctx context.Context, connectionID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM schema_metadata WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.tx.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count columns: %w", err)
	}
	
	return count, nil
}

// 其他接口方法的简化实现，返回未实现错误
func (r *PostgreSQLTxSchemaRepository) ListByTable(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	return nil, fmt.Errorf("ListByTable not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) GetTableStructure(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	return nil, fmt.Errorf("GetTableStructure not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) ListTables(ctx context.Context, connectionID int64, schemaName string) ([]string, error) {
	return nil, fmt.Errorf("ListTables not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) ListSchemas(ctx context.Context, connectionID int64) ([]string, error) {
	return nil, fmt.Errorf("ListSchemas not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) RefreshConnectionMetadata(ctx context.Context, connectionID int64, schemas []*repository.SchemaMetadata) error {
	return fmt.Errorf("RefreshConnectionMetadata not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) SearchTables(ctx context.Context, connectionID int64, keyword string) ([]*repository.TableInfo, error) {
	return nil, fmt.Errorf("SearchTables not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) SearchColumns(ctx context.Context, connectionID int64, keyword string) ([]*repository.ColumnInfo, error) {
	return nil, fmt.Errorf("SearchColumns not implemented in transaction version")
}

func (r *PostgreSQLTxSchemaRepository) GetRelatedTables(ctx context.Context, connectionID int64, tableName string) ([]*repository.TableRelation, error) {
	return nil, fmt.Errorf("GetRelatedTables not implemented in transaction version")
}