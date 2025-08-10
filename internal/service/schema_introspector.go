package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// SchemaIntrospector 数据库Schema探测器
// 自动探测目标数据库的表结构、字段类型、索引、约束等元数据信息
type SchemaIntrospector struct {
	// 核心组件
	connectionManager *ConnectionManager               // 连接管理器
	schemaRepo        repository.SchemaRepository     // Schema Repository
	logger            *zap.Logger                     // 日志器
	
	// 配置参数
	introspectionTimeout time.Duration // 探测超时时间
	maxTablesPerSchema   int           // 每个schema最大表数量
	enableIndexInfo      bool          // 是否启用索引信息收集
	enableConstraintInfo bool          // 是否启用约束信息收集
}

// SchemaIntrospectorConfig Schema探测器配置
type SchemaIntrospectorConfig struct {
	IntrospectionTimeout time.Duration `json:"introspection_timeout"` // 探测超时时间，默认60秒
	MaxTablesPerSchema   int           `json:"max_tables_per_schema"`  // 每个schema最大表数量，默认100
	EnableIndexInfo      bool          `json:"enable_index_info"`      // 是否启用索引信息收集，默认true
	EnableConstraintInfo bool          `json:"enable_constraint_info"` // 是否启用约束信息收集，默认true
}

// DatabaseSchema 完整的数据库Schema信息
type DatabaseSchema struct {
	ConnectionID int64                   `json:"connection_id"` // 数据库连接ID
	Schemas      []SchemaInfo            `json:"schemas"`       // Schema列表
	TotalTables  int                     `json:"total_tables"`  // 总表数
	TotalColumns int                     `json:"total_columns"` // 总列数
	LastUpdated  time.Time              `json:"last_updated"`  // 最后更新时间
}

// SchemaInfo Schema信息
type SchemaInfo struct {
	SchemaName string      `json:"schema_name"` // Schema名称
	Tables     []TableInfo `json:"tables"`      // 表列表
	TableCount int         `json:"table_count"` // 表数量
}

// TableInfo 表信息
type TableInfo struct {
	SchemaName   string       `json:"schema_name"`   // Schema名称
	TableName    string       `json:"table_name"`    // 表名
	TableComment *string      `json:"table_comment"` // 表注释
	TableType    string       `json:"table_type"`    // 表类型：BASE TABLE, VIEW等
	Columns      []ColumnInfo `json:"columns"`       // 列信息
	Indexes      []IndexInfo  `json:"indexes"`       // 索引信息
	Constraints  []Constraint `json:"constraints"`   // 约束信息
	ColumnCount  int          `json:"column_count"`  // 列数量
	EstimatedRows *int64       `json:"estimated_rows"` // 估计行数
}

// ColumnInfo 列信息
type ColumnInfo struct {
	ColumnName       string  `json:"column_name"`       // 列名
	DataType         string  `json:"data_type"`         // 数据类型
	IsNullable       bool    `json:"is_nullable"`       // 是否可为空
	ColumnDefault    *string `json:"column_default"`    // 默认值
	IsPrimaryKey     bool    `json:"is_primary_key"`    // 是否主键
	IsForeignKey     bool    `json:"is_foreign_key"`    // 是否外键
	ForeignTable     *string `json:"foreign_table"`     // 外键引用表
	ForeignColumn    *string `json:"foreign_column"`    // 外键引用列
	ColumnComment    *string `json:"column_comment"`    // 列注释
	OrdinalPosition  int32   `json:"ordinal_position"`  // 列位置
	CharacterMaxLength *int32 `json:"character_max_length"` // 字符最大长度
	NumericPrecision *int32  `json:"numeric_precision"`    // 数值精度
	NumericScale     *int32  `json:"numeric_scale"`        // 数值标度
}

// IndexInfo 索引信息
type IndexInfo struct {
	IndexName   string   `json:"index_name"`   // 索引名
	IndexType   string   `json:"index_type"`   // 索引类型：btree, hash等
	IsUnique    bool     `json:"is_unique"`    // 是否唯一索引
	IsPrimary   bool     `json:"is_primary"`   // 是否主键索引
	Columns     []string `json:"columns"`      // 索引包含的列
	Definition  *string  `json:"definition"`   // 索引定义SQL
}

// Constraint 约束信息
type Constraint struct {
	ConstraintName string   `json:"constraint_name"` // 约束名
	ConstraintType string   `json:"constraint_type"` // 约束类型：PRIMARY KEY, FOREIGN KEY, CHECK等
	Columns        []string `json:"columns"`         // 涉及的列
	RefTable       *string  `json:"ref_table"`       // 引用的表（外键）
	RefColumns     []string `json:"ref_columns"`     // 引用的列（外键）
	CheckCondition *string  `json:"check_condition"` // 检查条件（CHECK约束）
}

// NewSchemaIntrospector 创建Schema探测器
func NewSchemaIntrospector(
	connectionManager *ConnectionManager,
	schemaRepo repository.SchemaRepository,
	logger *zap.Logger,
) *SchemaIntrospector {
	config := &SchemaIntrospectorConfig{
		IntrospectionTimeout: 60 * time.Second,
		MaxTablesPerSchema:   100,
		EnableIndexInfo:      true,
		EnableConstraintInfo: true,
	}
	
	return NewSchemaIntrospectorWithConfig(connectionManager, schemaRepo, config, logger)
}

// NewSchemaIntrospectorWithConfig 使用自定义配置创建Schema探测器
func NewSchemaIntrospectorWithConfig(
	connectionManager *ConnectionManager,
	schemaRepo repository.SchemaRepository,
	config *SchemaIntrospectorConfig,
	logger *zap.Logger,
) *SchemaIntrospector {
	if config == nil {
		return NewSchemaIntrospector(connectionManager, schemaRepo, logger)
	}
	
	// 设置默认值
	if config.IntrospectionTimeout <= 0 {
		config.IntrospectionTimeout = 60 * time.Second
	}
	if config.MaxTablesPerSchema <= 0 {
		config.MaxTablesPerSchema = 100
	}
	
	return &SchemaIntrospector{
		connectionManager:    connectionManager,
		schemaRepo:          schemaRepo,
		logger:              logger,
		introspectionTimeout: config.IntrospectionTimeout,
		maxTablesPerSchema:   config.MaxTablesPerSchema,
		enableIndexInfo:      config.EnableIndexInfo,
		enableConstraintInfo: config.EnableConstraintInfo,
	}
}

// IntrospectDatabase 完整探测数据库Schema
func (si *SchemaIntrospector) IntrospectDatabase(ctx context.Context, connectionID int64) (*DatabaseSchema, error) {
	start := time.Now()
	
	si.logger.Info("开始探测数据库Schema",
		zap.Int64("connection_id", connectionID))
	
	// 创建探测上下文，设置超时
	introspectCtx, cancel := context.WithTimeout(ctx, si.introspectionTimeout)
	defer cancel()
	
	// 获取数据库连接
	pool, err := si.connectionManager.GetConnectionPool(introspectCtx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}
	
	// 获取所有schema
	schemas, err := si.getSchemas(introspectCtx, pool)
	if err != nil {
		return nil, fmt.Errorf("获取schema列表失败: %w", err)
	}
	
	// 探测每个schema的表结构
	var schemaInfos []SchemaInfo
	var totalTables, totalColumns int
	
	for _, schemaName := range schemas {
		schemaInfo, err := si.introspectSchema(introspectCtx, pool, schemaName)
		if err != nil {
			si.logger.Warn("探测schema失败",
				zap.String("schema", schemaName),
				zap.Error(err))
			continue
		}
		
		schemaInfos = append(schemaInfos, *schemaInfo)
		totalTables += schemaInfo.TableCount
		
		for _, table := range schemaInfo.Tables {
			totalColumns += table.ColumnCount
		}
	}
	
	databaseSchema := &DatabaseSchema{
		ConnectionID: connectionID,
		Schemas:      schemaInfos,
		TotalTables:  totalTables,
		TotalColumns: totalColumns,
		LastUpdated:  time.Now(),
	}
	
	si.logger.Info("数据库Schema探测完成",
		zap.Int64("connection_id", connectionID),
		zap.Int("total_schemas", len(schemaInfos)),
		zap.Int("total_tables", totalTables),
		zap.Int("total_columns", totalColumns),
		zap.Duration("duration", time.Since(start)))
	
	return databaseSchema, nil
}

// getSchemas 获取数据库中的所有schema
func (si *SchemaIntrospector) getSchemas(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY schema_name
	`
	
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询schema列表失败: %w", err)
	}
	defer rows.Close()
	
	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("扫描schema名称失败: %w", err)
		}
		schemas = append(schemas, schemaName)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取schema列表时发生错误: %w", err)
	}
	
	return schemas, nil
}

// introspectSchema 探测指定schema的表结构
func (si *SchemaIntrospector) introspectSchema(ctx context.Context, pool *pgxpool.Pool, schemaName string) (*SchemaInfo, error) {
	// 获取schema中的表列表
	tables, err := si.getTables(ctx, pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("获取表列表失败: %w", err)
	}
	
	// 限制表数量
	if len(tables) > si.maxTablesPerSchema {
		si.logger.Warn("Schema中表数量超过限制，将截断",
			zap.String("schema", schemaName),
			zap.Int("total_tables", len(tables)),
			zap.Int("max_tables", si.maxTablesPerSchema))
		tables = tables[:si.maxTablesPerSchema]
	}
	
	var tableInfos []TableInfo
	for _, tableName := range tables {
		tableInfo, err := si.introspectTable(ctx, pool, schemaName, tableName)
		if err != nil {
			si.logger.Warn("探测表结构失败",
				zap.String("schema", schemaName),
				zap.String("table", tableName),
				zap.Error(err))
			continue
		}
		
		tableInfos = append(tableInfos, *tableInfo)
	}
	
	return &SchemaInfo{
		SchemaName: schemaName,
		Tables:     tableInfos,
		TableCount: len(tableInfos),
	}, nil
}

// getTables 获取指定schema中的表列表
func (si *SchemaIntrospector) getTables(ctx context.Context, pool *pgxpool.Pool, schemaName string) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`
	
	rows, err := pool.Query(ctx, query, schemaName)
	if err != nil {
		return nil, fmt.Errorf("查询表列表失败: %w", err)
	}
	defer rows.Close()
	
	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("扫描表名失败: %w", err)
		}
		tables = append(tables, tableName)
	}
	
	return tables, rows.Err()
}

// introspectTable 探测指定表的详细结构
func (si *SchemaIntrospector) introspectTable(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) (*TableInfo, error) {
	// 获取表基本信息
	tableInfo, err := si.getTableInfo(ctx, pool, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("获取表基本信息失败: %w", err)
	}
	
	// 获取列信息
	columns, err := si.getColumns(ctx, pool, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("获取列信息失败: %w", err)
	}
	tableInfo.Columns = columns
	tableInfo.ColumnCount = len(columns)
	
	// 获取索引信息（可选）
	if si.enableIndexInfo {
		indexes, err := si.getIndexes(ctx, pool, schemaName, tableName)
		if err != nil {
			si.logger.Warn("获取索引信息失败",
				zap.String("schema", schemaName),
				zap.String("table", tableName),
				zap.Error(err))
		} else {
			tableInfo.Indexes = indexes
		}
	}
	
	// 获取约束信息（可选）
	if si.enableConstraintInfo {
		constraints, err := si.getConstraints(ctx, pool, schemaName, tableName)
		if err != nil {
			si.logger.Warn("获取约束信息失败",
				zap.String("schema", schemaName),
				zap.String("table", tableName),
				zap.Error(err))
		} else {
			tableInfo.Constraints = constraints
		}
	}
	
	// 获取表行数估计
	if rowCount, err := si.getEstimatedRowCount(ctx, pool, schemaName, tableName); err == nil {
		tableInfo.EstimatedRows = &rowCount
	}
	
	return tableInfo, nil
}

// getTableInfo 获取表基本信息
func (si *SchemaIntrospector) getTableInfo(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) (*TableInfo, error) {
	query := `
		SELECT 
			table_schema,
			table_name,
			table_type,
			COALESCE(obj_description(
				(table_schema || '.' || table_name)::regclass::oid, 'pg_class'
			), '') as table_comment
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_name = $2
	`
	
	var tableInfo TableInfo
	var tableComment string
	
	err := pool.QueryRow(ctx, query, schemaName, tableName).Scan(
		&tableInfo.SchemaName,
		&tableInfo.TableName,
		&tableInfo.TableType,
		&tableComment,
	)
	
	if err != nil {
		return nil, fmt.Errorf("查询表信息失败: %w", err)
	}
	
	if tableComment != "" {
		tableInfo.TableComment = &tableComment
	}
	
	return &tableInfo, nil
}

// getColumns 获取表的列信息
func (si *SchemaIntrospector) getColumns(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END as is_nullable,
			c.column_default,
			c.ordinal_position,
			c.character_maximum_length,
			c.numeric_precision,
			c.numeric_scale,
			COALESCE(col_description(
				(c.table_schema || '.' || c.table_name)::regclass::oid,
				c.ordinal_position
			), '') as column_comment,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key,
			CASE WHEN fk.column_name IS NOT NULL THEN true ELSE false END as is_foreign_key,
			fk.foreign_table_name,
			fk.foreign_column_name
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
				AND tc.table_schema = ku.table_schema
			WHERE tc.constraint_type = 'PRIMARY KEY'
				AND tc.table_schema = $1
				AND tc.table_name = $2
		) pk ON c.column_name = pk.column_name
		LEFT JOIN (
			SELECT 
				ku.column_name,
				ccu.table_name as foreign_table_name,
				ccu.column_name as foreign_column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
				AND tc.table_schema = ku.table_schema
			JOIN information_schema.constraint_column_usage ccu
				ON tc.constraint_name = ccu.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY'
				AND tc.table_schema = $1
				AND tc.table_name = $2
		) fk ON c.column_name = fk.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`
	
	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询列信息失败: %w", err)
	}
	defer rows.Close()
	
	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var columnComment string
		var foreignTable, foreignColumn *string
		
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&col.IsNullable,
			&col.ColumnDefault,
			&col.OrdinalPosition,
			&col.CharacterMaxLength,
			&col.NumericPrecision,
			&col.NumericScale,
			&columnComment,
			&col.IsPrimaryKey,
			&col.IsForeignKey,
			&foreignTable,
			&foreignColumn,
		)
		
		if err != nil {
			return nil, fmt.Errorf("扫描列信息失败: %w", err)
		}
		
		if columnComment != "" {
			col.ColumnComment = &columnComment
		}
		
		if foreignTable != nil {
			col.ForeignTable = foreignTable
		}
		if foreignColumn != nil {
			col.ForeignColumn = foreignColumn
		}
		
		columns = append(columns, col)
	}
	
	return columns, rows.Err()
}

// getIndexes 获取表的索引信息
func (si *SchemaIntrospector) getIndexes(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]IndexInfo, error) {
	query := `
		SELECT 
			i.indexname as index_name,
			CASE 
				WHEN i.indisprimary THEN 'PRIMARY'
				WHEN i.indisunique THEN 'UNIQUE'
				ELSE 'INDEX'
			END as index_type,
			i.indisunique as is_unique,
			i.indisprimary as is_primary,
			string_agg(a.attname, ', ' ORDER BY k.ordinality) as columns,
			pg_get_indexdef(i.indexrelid) as definition
		FROM pg_indexes idx
		JOIN pg_class c ON c.relname = idx.tablename
		JOIN pg_index i ON i.indexrelid = (idx.schemaname || '.' || idx.indexname)::regclass::oid
		JOIN unnest(i.indkey) WITH ORDINALITY k(attnum, ordinality) ON true
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
		WHERE idx.schemaname = $1 AND idx.tablename = $2
		GROUP BY i.indexrelid, idx.indexname, i.indisunique, i.indisprimary
		ORDER BY idx.indexname
	`
	
	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		// 索引信息获取失败不是致命错误，返回空列表
		si.logger.Warn("查询索引信息失败，跳过索引信息收集",
			zap.Error(err))
		return []IndexInfo{}, nil
	}
	defer rows.Close()
	
	var indexes []IndexInfo
	for rows.Next() {
		var idx IndexInfo
		var columnsStr, definition string
		
		err := rows.Scan(
			&idx.IndexName,
			&idx.IndexType,
			&idx.IsUnique,
			&idx.IsPrimary,
			&columnsStr,
			&definition,
		)
		
		if err != nil {
			si.logger.Warn("扫描索引信息失败",
				zap.Error(err))
			continue
		}
		
		idx.Columns = strings.Split(columnsStr, ", ")
		idx.Definition = &definition
		
		indexes = append(indexes, idx)
	}
	
	return indexes, nil
}

// getConstraints 获取表的约束信息
func (si *SchemaIntrospector) getConstraints(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]Constraint, error) {
	query := `
		SELECT 
			tc.constraint_name,
			tc.constraint_type,
			string_agg(ku.column_name, ', ' ORDER BY ku.ordinal_position) as columns,
			ccu.table_name as ref_table,
			string_agg(ccu.column_name, ', ' ORDER BY ku.ordinal_position) as ref_columns,
			cc.check_clause
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage ku
			ON tc.constraint_name = ku.constraint_name
			AND tc.table_schema = ku.table_schema
		LEFT JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
		LEFT JOIN information_schema.check_constraints cc
			ON tc.constraint_name = cc.constraint_name
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		GROUP BY tc.constraint_name, tc.constraint_type, ccu.table_name, cc.check_clause
		ORDER BY tc.constraint_name
	`
	
	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		// 约束信息获取失败不是致命错误，返回空列表
		si.logger.Warn("查询约束信息失败，跳过约束信息收集",
			zap.Error(err))
		return []Constraint{}, nil
	}
	defer rows.Close()
	
	var constraints []Constraint
	for rows.Next() {
		var constraint Constraint
		var columnsStr string
		var refTable, refColumnsStr, checkClause *string
		
		err := rows.Scan(
			&constraint.ConstraintName,
			&constraint.ConstraintType,
			&columnsStr,
			&refTable,
			&refColumnsStr,
			&checkClause,
		)
		
		if err != nil {
			si.logger.Warn("扫描约束信息失败",
				zap.Error(err))
			continue
		}
		
		constraint.Columns = strings.Split(columnsStr, ", ")
		
		if refTable != nil {
			constraint.RefTable = refTable
		}
		if refColumnsStr != nil {
			constraint.RefColumns = strings.Split(*refColumnsStr, ", ")
		}
		if checkClause != nil {
			constraint.CheckCondition = checkClause
		}
		
		constraints = append(constraints, constraint)
	}
	
	return constraints, nil
}

// getEstimatedRowCount 获取表的估计行数
func (si *SchemaIntrospector) getEstimatedRowCount(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) (int64, error) {
	query := `
		SELECT reltuples::bigint as estimated_rows
		FROM pg_class
		WHERE oid = ($1 || '.' || $2)::regclass::oid
	`
	
	var rowCount int64
	err := pool.QueryRow(ctx, query, schemaName, tableName).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("查询表行数估计失败: %w", err)
	}
	
	return rowCount, nil
}

// SaveSchemaMetadata 保存Schema元数据到数据库
func (si *SchemaIntrospector) SaveSchemaMetadata(ctx context.Context, databaseSchema *DatabaseSchema) error {
	// 先清除旧的元数据
	if err := si.schemaRepo.BatchDelete(ctx, databaseSchema.ConnectionID); err != nil {
		return fmt.Errorf("清除旧元数据失败: %w", err)
	}
	
	// 转换为SchemaMetadata格式并批量保存
	var metadataList []*repository.SchemaMetadata
	
	for _, schema := range databaseSchema.Schemas {
		for _, table := range schema.Tables {
			for _, column := range table.Columns {
				metadata := &repository.SchemaMetadata{
					ConnectionID:    databaseSchema.ConnectionID,
					SchemaName:      schema.SchemaName,
					TableName:       table.TableName,
					ColumnName:      column.ColumnName,
					DataType:        column.DataType,
					IsNullable:      column.IsNullable,
					ColumnDefault:   column.ColumnDefault,
					IsPrimaryKey:    column.IsPrimaryKey,
					IsForeignKey:    column.IsForeignKey,
					ForeignTable:    column.ForeignTable,
					ForeignColumn:   column.ForeignColumn,
					TableComment:    table.TableComment,
					ColumnComment:   column.ColumnComment,
					OrdinalPosition: column.OrdinalPosition,
				}
				
				metadataList = append(metadataList, metadata)
			}
		}
	}
	
	if len(metadataList) > 0 {
		if err := si.schemaRepo.BatchCreate(ctx, metadataList); err != nil {
			return fmt.Errorf("保存Schema元数据失败: %w", err)
		}
	}
	
	si.logger.Info("Schema元数据保存成功",
		zap.Int64("connection_id", databaseSchema.ConnectionID),
		zap.Int("metadata_count", len(metadataList)))
	
	return nil
}

// RefreshConnectionMetadata 刷新指定连接的元数据
func (si *SchemaIntrospector) RefreshConnectionMetadata(ctx context.Context, connectionID int64) error {
	// 探测数据库Schema
	databaseSchema, err := si.IntrospectDatabase(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("探测数据库Schema失败: %w", err)
	}
	
	// 保存到数据库
	if err := si.SaveSchemaMetadata(ctx, databaseSchema); err != nil {
		return fmt.Errorf("保存Schema元数据失败: %w", err)
	}
	
	return nil
}

// GetTableSummary 获取表结构摘要（用于AI模型）
func (si *SchemaIntrospector) GetTableSummary(ctx context.Context, connectionID int64, schemaName, tableName string) (string, error) {
	// 从缓存的元数据中获取表结构
	columns, err := si.schemaRepo.GetTableStructure(ctx, connectionID, schemaName, tableName)
	if err != nil {
		return "", fmt.Errorf("获取表结构失败: %w", err)
	}
	
	if len(columns) == 0 {
		return "", fmt.Errorf("表 %s.%s 不存在或没有元数据", schemaName, tableName)
	}
	
	// 构建表结构摘要
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("表 %s.%s:\n", schemaName, tableName))
	
	// 添加表注释
	if columns[0].TableComment != nil {
		summary.WriteString(fmt.Sprintf("  说明: %s\n", *columns[0].TableComment))
	}
	
	summary.WriteString("  列信息:\n")
	
	// 按位置排序列
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].OrdinalPosition < columns[j].OrdinalPosition
	})
	
	for _, column := range columns {
		summary.WriteString(fmt.Sprintf("    %s (%s)", column.ColumnName, column.DataType))
		
		if column.IsPrimaryKey {
			summary.WriteString(" [主键]")
		}
		if column.IsForeignKey && column.ForeignTable != nil {
			summary.WriteString(fmt.Sprintf(" [外键->%s]", *column.ForeignTable))
		}
		if !column.IsNullable {
			summary.WriteString(" [非空]")
		}
		if column.ColumnComment != nil {
			summary.WriteString(fmt.Sprintf(" // %s", *column.ColumnComment))
		}
		
		summary.WriteString("\n")
	}
	
	return summary.String(), nil
}

// GetConnectionTableList 获取连接的所有表列表
func (si *SchemaIntrospector) GetConnectionTableList(ctx context.Context, connectionID int64) ([]string, error) {
	tables, err := si.schemaRepo.ListTables(ctx, connectionID, "")
	if err != nil {
		return nil, fmt.Errorf("获取表列表失败: %w", err)
	}
	
	return tables, nil
}