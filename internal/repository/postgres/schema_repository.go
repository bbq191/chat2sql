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

// PostgreSQLSchemaRepository PostgreSQL元数据Repository实现
// 支持数据库结构探测、元数据缓存、表关系分析等高级功能
type PostgreSQLSchemaRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgreSQLSchemaRepository 创建PostgreSQL元数据Repository
func NewPostgreSQLSchemaRepository(pool *pgxpool.Pool, logger *zap.Logger) repository.SchemaRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &PostgreSQLSchemaRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create 创建元数据记录
func (r *PostgreSQLSchemaRepository) Create(ctx context.Context, schema *repository.SchemaMetadata) error {
	const query = `
		INSERT INTO schema_metadata (connection_id, schema_name, table_name, 
			column_name, data_type, is_nullable, column_default, is_primary_key, 
			is_foreign_key, foreign_table, foreign_column, table_comment, 
			column_comment, ordinal_position, create_by, create_time, 
			update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 
			$15, $16, $17, $18, $19)
		RETURNING id`

	now := time.Now().UTC()
	
	err := r.pool.QueryRow(ctx, query,
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
		r.logger.Error("创建元数据记录失败",
			zap.Int64("connection_id", schema.ConnectionID),
			zap.String("table_name", schema.TableName),
			zap.String("column_name", schema.ColumnName),
			zap.Error(err),
		)
		return fmt.Errorf("创建元数据记录失败: %w", err)
	}
	
	schema.CreateTime = now
	schema.UpdateTime = now
	schema.IsDeleted = false
	
	r.logger.Debug("元数据记录创建成功",
		zap.Int64("schema_id", schema.ID),
		zap.Int64("connection_id", schema.ConnectionID),
		zap.String("table_name", schema.TableName),
		zap.String("column_name", schema.ColumnName),
	)
	
	return nil
}

// GetByID 根据ID获取元数据记录
func (r *PostgreSQLSchemaRepository) GetByID(ctx context.Context, id int64) (*repository.SchemaMetadata, error) {
	const query = `
		SELECT id, connection_id, schema_name, table_name, column_name, data_type,
			is_nullable, column_default, is_primary_key, is_foreign_key, 
			foreign_table, foreign_column, table_comment, column_comment, 
			ordinal_position, create_by, create_time, update_by, update_time, is_deleted
		FROM schema_metadata 
		WHERE id = $1 AND is_deleted = false`

	schema := &repository.SchemaMetadata{}
	
	err := r.pool.QueryRow(ctx, query, id).Scan(
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
			r.logger.Warn("元数据记录不存在", zap.Int64("schema_id", id))
			return nil, fmt.Errorf("元数据记录不存在: %w", repository.ErrNotFound)
		}
		
		r.logger.Error("获取元数据记录失败",
			zap.Int64("schema_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取元数据记录失败: %w", err)
	}
	
	return schema, nil
}

// Update 更新元数据记录
func (r *PostgreSQLSchemaRepository) Update(ctx context.Context, schema *repository.SchemaMetadata) error {
	const query = `
		UPDATE schema_metadata 
		SET schema_name = $2, table_name = $3, column_name = $4, data_type = $5,
			is_nullable = $6, column_default = $7, is_primary_key = $8, 
			is_foreign_key = $9, foreign_table = $10, foreign_column = $11, 
			table_comment = $12, column_comment = $13, ordinal_position = $14,
			update_by = $15, update_time = $16
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	
	result, err := r.pool.Exec(ctx, query,
		schema.ID,
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
		schema.UpdateBy,
		now,
	)
	
	if err != nil {
		r.logger.Error("更新元数据记录失败",
			zap.Int64("schema_id", schema.ID),
			zap.Error(err),
		)
		return fmt.Errorf("更新元数据记录失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("元数据记录不存在或已删除", zap.Int64("schema_id", schema.ID))
		return fmt.Errorf("元数据记录不存在或已删除: %w", repository.ErrNotFound)
	}
	
	schema.UpdateTime = now
	
	r.logger.Debug("元数据记录更新成功",
		zap.Int64("schema_id", schema.ID),
		zap.String("table_name", schema.TableName),
		zap.String("column_name", schema.ColumnName),
	)
	
	return nil
}

// Delete 软删除元数据记录
func (r *PostgreSQLSchemaRepository) Delete(ctx context.Context, id int64) error {
	const query = `
		UPDATE schema_metadata 
		SET is_deleted = true, update_time = $2
		WHERE id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, now)
	
	if err != nil {
		r.logger.Error("删除元数据记录失败",
			zap.Int64("schema_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除元数据记录失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("元数据记录不存在或已删除", zap.Int64("schema_id", id))
		return fmt.Errorf("元数据记录不存在或已删除: %w", repository.ErrNotFound)
	}
	
	r.logger.Debug("元数据记录删除成功", zap.Int64("schema_id", id))
	return nil
}

// ListByConnection 根据连接ID获取所有元数据
func (r *PostgreSQLSchemaRepository) ListByConnection(ctx context.Context, connectionID int64) ([]*repository.SchemaMetadata, error) {
	const query = `
		SELECT id, connection_id, schema_name, table_name, column_name, data_type,
			is_nullable, column_default, is_primary_key, is_foreign_key, 
			foreign_table, foreign_column, table_comment, column_comment, 
			ordinal_position, create_by, create_time, update_by, update_time, is_deleted
		FROM schema_metadata 
		WHERE connection_id = $1 AND is_deleted = false 
		ORDER BY schema_name, table_name, ordinal_position`

	rows, err := r.pool.Query(ctx, query, connectionID)
	if err != nil {
		r.logger.Error("根据连接ID获取元数据失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据连接ID获取元数据失败: %w", err)
	}
	defer rows.Close()

	return r.scanSchemaMetadata(rows)
}

// ListByTable 根据表名获取表结构元数据
func (r *PostgreSQLSchemaRepository) ListByTable(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	const query = `
		SELECT id, connection_id, schema_name, table_name, column_name, data_type,
			is_nullable, column_default, is_primary_key, is_foreign_key, 
			foreign_table, foreign_column, table_comment, column_comment, 
			ordinal_position, create_by, create_time, update_by, update_time, is_deleted
		FROM schema_metadata 
		WHERE connection_id = $1 AND schema_name = $2 AND table_name = $3 
			AND is_deleted = false 
		ORDER BY ordinal_position`

	rows, err := r.pool.Query(ctx, query, connectionID, schemaName, tableName)
	if err != nil {
		r.logger.Error("根据表名获取元数据失败",
			zap.Int64("connection_id", connectionID),
			zap.String("schema_name", schemaName),
			zap.String("table_name", tableName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("根据表名获取元数据失败: %w", err)
	}
	defer rows.Close()

	return r.scanSchemaMetadata(rows)
}

// GetTableStructure 获取表结构（别名，与ListByTable功能相同）
func (r *PostgreSQLSchemaRepository) GetTableStructure(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	return r.ListByTable(ctx, connectionID, schemaName, tableName)
}

// ListTables 获取指定schema下的所有表名
func (r *PostgreSQLSchemaRepository) ListTables(ctx context.Context, connectionID int64, schemaName string) ([]string, error) {
	const query = `
		SELECT DISTINCT table_name 
		FROM schema_metadata 
		WHERE connection_id = $1 AND schema_name = $2 AND is_deleted = false 
		ORDER BY table_name`

	rows, err := r.pool.Query(ctx, query, connectionID, schemaName)
	if err != nil {
		r.logger.Error("获取表名列表失败",
			zap.Int64("connection_id", connectionID),
			zap.String("schema_name", schemaName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取表名列表失败: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			r.logger.Error("扫描表名失败", zap.Error(err))
			return nil, fmt.Errorf("扫描表名失败: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("处理表名查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理表名查询结果失败: %w", err)
	}

	return tables, nil
}

// ListSchemas 获取指定连接下的所有schema名
func (r *PostgreSQLSchemaRepository) ListSchemas(ctx context.Context, connectionID int64) ([]string, error) {
	const query = `
		SELECT DISTINCT schema_name 
		FROM schema_metadata 
		WHERE connection_id = $1 AND is_deleted = false 
		ORDER BY schema_name`

	rows, err := r.pool.Query(ctx, query, connectionID)
	if err != nil {
		r.logger.Error("获取schema名列表失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取schema名列表失败: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			r.logger.Error("扫描schema名失败", zap.Error(err))
			return nil, fmt.Errorf("扫描schema名失败: %w", err)
		}
		schemas = append(schemas, schemaName)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("处理schema查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理schema查询结果失败: %w", err)
	}

	return schemas, nil
}

// BatchCreate 批量创建元数据记录
func (r *PostgreSQLSchemaRepository) BatchCreate(ctx context.Context, schemas []*repository.SchemaMetadata) error {
	if len(schemas) == 0 {
		return nil
	}

	// 使用事务来保证批量操作的原子性
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("开始批量创建事务失败", zap.Error(err))
		return fmt.Errorf("开始批量创建事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO schema_metadata (connection_id, schema_name, table_name, 
			column_name, data_type, is_nullable, column_default, is_primary_key, 
			is_foreign_key, foreign_table, foreign_column, table_comment, 
			column_comment, ordinal_position, create_by, create_time, 
			update_by, update_time, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 
			$15, $16, $17, $18, $19)`

	now := time.Now().UTC()
	
	for _, schema := range schemas {
		_, err := tx.Exec(ctx, query,
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
		)
		
		if err != nil {
			r.logger.Error("批量创建元数据记录失败",
				zap.Int64("connection_id", schema.ConnectionID),
				zap.String("table_name", schema.TableName),
				zap.String("column_name", schema.ColumnName),
				zap.Error(err),
			)
			return fmt.Errorf("批量创建元数据记录失败: %w", err)
		}
		
		schema.CreateTime = now
		schema.UpdateTime = now
		schema.IsDeleted = false
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("提交批量创建事务失败", zap.Error(err))
		return fmt.Errorf("提交批量创建事务失败: %w", err)
	}

	r.logger.Info("批量元数据记录创建成功",
		zap.Int("count", len(schemas)),
		zap.Int64("connection_id", schemas[0].ConnectionID),
	)

	return nil
}

// BatchDelete 删除指定连接的所有元数据
func (r *PostgreSQLSchemaRepository) BatchDelete(ctx context.Context, connectionID int64) error {
	const query = `
		UPDATE schema_metadata 
		SET is_deleted = true, update_time = $2
		WHERE connection_id = $1 AND is_deleted = false`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, connectionID, now)
	
	if err != nil {
		r.logger.Error("批量删除元数据失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return fmt.Errorf("批量删除元数据失败: %w", err)
	}
	
	rowsAffected := result.RowsAffected()
	r.logger.Info("批量元数据删除完成",
		zap.Int64("connection_id", connectionID),
		zap.Int64("deleted_count", rowsAffected),
	)
	
	return nil
}

// RefreshConnectionMetadata 刷新连接的元数据（先删除旧数据，再创建新数据）
func (r *PostgreSQLSchemaRepository) RefreshConnectionMetadata(ctx context.Context, connectionID int64, schemas []*repository.SchemaMetadata) error {
	// 使用事务来保证操作的原子性
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("开始刷新元数据事务失败", zap.Error(err))
		return fmt.Errorf("开始刷新元数据事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	// 删除旧数据
	now := time.Now().UTC()
	deleteQuery := `
		UPDATE schema_metadata 
		SET is_deleted = true, update_time = $2
		WHERE connection_id = $1 AND is_deleted = false`
	
	result, err := tx.Exec(ctx, deleteQuery, connectionID, now)
	if err != nil {
		r.logger.Error("删除旧元数据失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return fmt.Errorf("删除旧元数据失败: %w", err)
	}
	
	deletedCount := result.RowsAffected()

	// 创建新数据
	if len(schemas) > 0 {
		insertQuery := `
			INSERT INTO schema_metadata (connection_id, schema_name, table_name, 
				column_name, data_type, is_nullable, column_default, is_primary_key, 
				is_foreign_key, foreign_table, foreign_column, table_comment, 
				column_comment, ordinal_position, create_by, create_time, 
				update_by, update_time, is_deleted)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 
				$15, $16, $17, $18, $19)`
		
		for _, schema := range schemas {
			_, err := tx.Exec(ctx, insertQuery,
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
			)
			
			if err != nil {
				r.logger.Error("创建新元数据记录失败",
					zap.Int64("connection_id", schema.ConnectionID),
					zap.String("table_name", schema.TableName),
					zap.String("column_name", schema.ColumnName),
					zap.Error(err),
				)
				return fmt.Errorf("创建新元数据记录失败: %w", err)
			}
			
			schema.CreateTime = now
			schema.UpdateTime = now
			schema.IsDeleted = false
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("提交刷新元数据事务失败", zap.Error(err))
		return fmt.Errorf("提交刷新元数据事务失败: %w", err)
	}

	r.logger.Info("元数据刷新完成",
		zap.Int64("connection_id", connectionID),
		zap.Int64("deleted_count", deletedCount),
		zap.Int("created_count", len(schemas)),
	)

	return nil
}

// SearchTables 搜索表名
func (r *PostgreSQLSchemaRepository) SearchTables(ctx context.Context, connectionID int64, keyword string) ([]*repository.TableInfo, error) {
	const query = `
		SELECT schema_name, table_name, 
			COALESCE(table_comment, '') as table_comment, 
			COUNT(*) as column_count
		FROM schema_metadata 
		WHERE connection_id = $1 
			AND (table_name ILIKE $2 OR COALESCE(table_comment, '') ILIKE $2)
			AND is_deleted = false 
		GROUP BY schema_name, table_name, table_comment
		ORDER BY table_name`

	searchPattern := "%" + keyword + "%"
	
	rows, err := r.pool.Query(ctx, query, connectionID, searchPattern)
	if err != nil {
		r.logger.Error("搜索表名失败",
			zap.Int64("connection_id", connectionID),
			zap.String("keyword", keyword),
			zap.Error(err),
		)
		return nil, fmt.Errorf("搜索表名失败: %w", err)
	}
	defer rows.Close()

	var tables []*repository.TableInfo
	for rows.Next() {
		table := &repository.TableInfo{}
		err := rows.Scan(
			&table.SchemaName,
			&table.TableName,
			&table.TableComment,
			&table.ColumnCount,
		)
		
		if err != nil {
			r.logger.Error("扫描表信息失败", zap.Error(err))
			return nil, fmt.Errorf("扫描表信息失败: %w", err)
		}
		
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("处理表搜索结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理表搜索结果失败: %w", err)
	}

	return tables, nil
}

// SearchColumns 搜索列名
func (r *PostgreSQLSchemaRepository) SearchColumns(ctx context.Context, connectionID int64, keyword string) ([]*repository.ColumnInfo, error) {
	const query = `
		SELECT schema_name, table_name, column_name, data_type, is_nullable, 
			is_primary_key, column_comment
		FROM schema_metadata 
		WHERE connection_id = $1 
			AND (column_name ILIKE $2 OR COALESCE(column_comment, '') ILIKE $2)
			AND is_deleted = false 
		ORDER BY table_name, ordinal_position`

	searchPattern := "%" + keyword + "%"
	
	rows, err := r.pool.Query(ctx, query, connectionID, searchPattern)
	if err != nil {
		r.logger.Error("搜索列名失败",
			zap.Int64("connection_id", connectionID),
			zap.String("keyword", keyword),
			zap.Error(err),
		)
		return nil, fmt.Errorf("搜索列名失败: %w", err)
	}
	defer rows.Close()

	var columns []*repository.ColumnInfo
	for rows.Next() {
		column := &repository.ColumnInfo{}
		err := rows.Scan(
			&column.SchemaName,
			&column.TableName,
			&column.ColumnName,
			&column.DataType,
			&column.IsNullable,
			&column.IsPrimaryKey,
			&column.ColumnComment,
		)
		
		if err != nil {
			r.logger.Error("扫描列信息失败", zap.Error(err))
			return nil, fmt.Errorf("扫描列信息失败: %w", err)
		}
		
		columns = append(columns, column)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("处理列搜索结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理列搜索结果失败: %w", err)
	}

	return columns, nil
}

// GetRelatedTables 获取表关系信息
func (r *PostgreSQLSchemaRepository) GetRelatedTables(ctx context.Context, connectionID int64, tableName string) ([]*repository.TableRelation, error) {
	const query = `
		SELECT DISTINCT
			sm1.table_name as from_table,
			sm2.table_name as to_table,
			sm1.column_name as from_column,
			sm1.foreign_column as to_column,
			'foreign_key' as relation_type
		FROM schema_metadata sm1
		JOIN schema_metadata sm2 ON sm1.connection_id = sm2.connection_id 
			AND sm1.foreign_table = sm2.table_name
		WHERE sm1.connection_id = $1 
			AND sm1.is_foreign_key = true
			AND (sm1.table_name = $2 OR sm1.foreign_table = $2)
			AND sm1.is_deleted = false 
			AND sm2.is_deleted = false`

	rows, err := r.pool.Query(ctx, query, connectionID, tableName)
	if err != nil {
		r.logger.Error("获取表关系失败",
			zap.Int64("connection_id", connectionID),
			zap.String("table_name", tableName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取表关系失败: %w", err)
	}
	defer rows.Close()

	var relations []*repository.TableRelation
	for rows.Next() {
		relation := &repository.TableRelation{}
		err := rows.Scan(
			&relation.FromTable,
			&relation.ToTable,
			&relation.FromColumn,
			&relation.ToColumn,
			&relation.RelationType,
		)
		
		if err != nil {
			r.logger.Error("扫描表关系失败", zap.Error(err))
			return nil, fmt.Errorf("扫描表关系失败: %w", err)
		}
		
		relations = append(relations, relation)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("处理表关系查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理表关系查询结果失败: %w", err)
	}

	return relations, nil
}

// CountByConnection 统计连接的元数据记录数量
func (r *PostgreSQLSchemaRepository) CountByConnection(ctx context.Context, connectionID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM schema_metadata WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		r.logger.Error("统计连接元数据数量失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("统计连接元数据数量失败: %w", err)
	}
	
	return count, nil
}

// GetTableCount 获取连接的表数量
func (r *PostgreSQLSchemaRepository) GetTableCount(ctx context.Context, connectionID int64) (int64, error) {
	const query = `
		SELECT COUNT(DISTINCT schema_name || '.' || table_name) 
		FROM schema_metadata 
		WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		r.logger.Error("统计连接表数量失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("统计连接表数量失败: %w", err)
	}
	
	return count, nil
}

// GetColumnCount 获取连接的列数量
func (r *PostgreSQLSchemaRepository) GetColumnCount(ctx context.Context, connectionID int64) (int64, error) {
	const query = `SELECT COUNT(*) FROM schema_metadata WHERE connection_id = $1 AND is_deleted = false`
	
	var count int64
	err := r.pool.QueryRow(ctx, query, connectionID).Scan(&count)
	if err != nil {
		r.logger.Error("统计连接列数量失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err),
		)
		return 0, fmt.Errorf("统计连接列数量失败: %w", err)
	}
	
	return count, nil
}

// scanSchemaMetadata 扫描元数据列表
func (r *PostgreSQLSchemaRepository) scanSchemaMetadata(rows pgx.Rows) ([]*repository.SchemaMetadata, error) {
	var schemas []*repository.SchemaMetadata
	
	for rows.Next() {
		schema := &repository.SchemaMetadata{}
		err := rows.Scan(
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
			r.logger.Error("扫描元数据失败", zap.Error(err))
			return nil, fmt.Errorf("扫描元数据失败: %w", err)
		}
		
		schemas = append(schemas, schema)
	}
	
	if err := rows.Err(); err != nil {
		r.logger.Error("处理元数据查询结果失败", zap.Error(err))
		return nil, fmt.Errorf("处理元数据查询结果失败: %w", err)
	}
	
	return schemas, nil
}