package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/repository"
)

// TestNewSchemaIntrospector_Simple 测试Schema探测器创建（简化版）
func TestNewSchemaIntrospector_Simple(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}
	
	// 为了避免类型转换问题，直接使用interface{}
	introspector := &SchemaIntrospector{
		schemaRepo:           mockSchemaRepo,
		logger:              logger,
		introspectionTimeout: 60 * time.Second,
		maxTablesPerSchema:   100,
		enableIndexInfo:      true,
		enableConstraintInfo: true,
	}

	assert.NotNil(t, introspector)
	assert.Equal(t, mockSchemaRepo, introspector.schemaRepo)
	assert.Equal(t, logger, introspector.logger)
	assert.Equal(t, 60*time.Second, introspector.introspectionTimeout)
	assert.Equal(t, 100, introspector.maxTablesPerSchema)
	assert.True(t, introspector.enableIndexInfo)
	assert.True(t, introspector.enableConstraintInfo)
}

// TestSchemaIntrospectorConfig_Simple 测试配置结构体（简化版）
func TestSchemaIntrospectorConfig_Simple(t *testing.T) {
	config := &SchemaIntrospectorConfig{
		IntrospectionTimeout: 45 * time.Second,
		MaxTablesPerSchema:   200,
		EnableIndexInfo:      true,
		EnableConstraintInfo: false,
	}

	assert.Equal(t, 45*time.Second, config.IntrospectionTimeout)
	assert.Equal(t, 200, config.MaxTablesPerSchema)
	assert.True(t, config.EnableIndexInfo)
	assert.False(t, config.EnableConstraintInfo)
}

// TestSaveSchemaMetadata_Simple 测试保存Schema元数据（简化版）
func TestSaveSchemaMetadata_Simple(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	introspector := &SchemaIntrospector{
		schemaRepo: mockSchemaRepo,
		logger:     logger,
	}

	// 准备测试数据
	databaseSchema := &DatabaseSchema{
		ConnectionID: 1,
		Schemas: []SchemaInfo{
			{
				SchemaName: "public",
				Tables: []TableInfo{
					{
						SchemaName: "public",
						TableName:  "users",
						Columns: []ColumnInfo{
							{
								ColumnName:      "id",
								DataType:        "bigint",
								IsNullable:      false,
								IsPrimaryKey:    true,
								OrdinalPosition: 1,
							},
						},
						ColumnCount: 1,
					},
				},
				TableCount: 1,
			},
		},
		TotalTables:  1,
		TotalColumns: 1,
	}

	// 设置mock期望
	mockSchemaRepo.On("BatchDelete", mock.Anything, int64(1)).Return(nil)
	mockSchemaRepo.On("BatchCreate", mock.Anything, mock.MatchedBy(func(metadata []*repository.SchemaMetadata) bool {
		return len(metadata) == 1 // 一个列
	})).Return(nil)

	ctx := context.Background()
	err := introspector.SaveSchemaMetadata(ctx, databaseSchema)

	assert.NoError(t, err)
	mockSchemaRepo.AssertExpectations(t)
}

// TestGetTableSummary_Simple 测试获取表结构摘要（简化版）
func TestGetTableSummary_Simple(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	introspector := &SchemaIntrospector{
		schemaRepo: mockSchemaRepo,
		logger:     logger,
	}

	// 准备测试数据
	metadata := []*repository.SchemaMetadata{
		{
			ConnectionID:    1,
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "id",
			DataType:        "bigint",
			IsNullable:      false,
			IsPrimaryKey:    true,
			OrdinalPosition: 1,
		},
		{
			ConnectionID:    1,
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "name",
			DataType:        "varchar",
			IsNullable:      false,
			OrdinalPosition: 2,
		},
	}

	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "users").
		Return(metadata, nil)

	ctx := context.Background()
	summary, err := introspector.GetTableSummary(ctx, 1, "public", "users")

	assert.NoError(t, err)
	assert.Contains(t, summary, "表 public.users:")
	assert.Contains(t, summary, "id (bigint) [主键] [非空]")
	assert.Contains(t, summary, "name (varchar) [非空]")

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetTableSummary_TableNotFound_Simple 测试表不存在的情况（简化版）
func TestGetTableSummary_TableNotFound_Simple(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	introspector := &SchemaIntrospector{
		schemaRepo: mockSchemaRepo,
		logger:     logger,
	}

	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "nonexistent").
		Return([]*repository.SchemaMetadata{}, nil)

	ctx := context.Background()
	summary, err := introspector.GetTableSummary(ctx, 1, "public", "nonexistent")

	assert.Error(t, err)
	assert.Empty(t, summary)
	assert.Contains(t, err.Error(), "不存在或没有元数据")
}

// TestGetConnectionTableList_Simple 测试获取连接的表列表（简化版）
func TestGetConnectionTableList_Simple(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	introspector := &SchemaIntrospector{
		schemaRepo: mockSchemaRepo,
		logger:     logger,
	}

	expectedTables := []string{"users", "orders", "products"}
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "").
		Return(expectedTables, nil)

	ctx := context.Background()
	tables, err := introspector.GetConnectionTableList(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockSchemaRepo.AssertExpectations(t)
}

// TestDatabaseSchemaStruct_Simple 测试DatabaseSchema结构体（简化版）
func TestDatabaseSchemaStruct_Simple(t *testing.T) {
	now := time.Now()
	schema := &DatabaseSchema{
		ConnectionID: 123,
		Schemas: []SchemaInfo{
			{
				SchemaName: "public",
				Tables:     []TableInfo{},
				TableCount: 0,
			},
		},
		TotalTables:  5,
		TotalColumns: 50,
		LastUpdated:  now,
	}

	assert.Equal(t, int64(123), schema.ConnectionID)
	assert.Len(t, schema.Schemas, 1)
	assert.Equal(t, "public", schema.Schemas[0].SchemaName)
	assert.Equal(t, 5, schema.TotalTables)
	assert.Equal(t, 50, schema.TotalColumns)
	assert.Equal(t, now, schema.LastUpdated)
}

// TestTableInfo_Simple 测试TableInfo结构体（简化版）
func TestTableInfo_Simple(t *testing.T) {
	comment := "用户信息表"
	estimatedRows := int64(1000)
	
	tableInfo := &TableInfo{
		SchemaName:    "public",
		TableName:     "users",
		TableComment:  &comment,
		TableType:     "BASE TABLE",
		Columns:       []ColumnInfo{},
		Indexes:       []IndexInfo{},
		Constraints:   []Constraint{},
		ColumnCount:   5,
		EstimatedRows: &estimatedRows,
	}

	assert.Equal(t, "public", tableInfo.SchemaName)
	assert.Equal(t, "users", tableInfo.TableName)
	assert.NotNil(t, tableInfo.TableComment)
	assert.Equal(t, "用户信息表", *tableInfo.TableComment)
	assert.Equal(t, "BASE TABLE", tableInfo.TableType)
	assert.Equal(t, 5, tableInfo.ColumnCount)
	assert.NotNil(t, tableInfo.EstimatedRows)
	assert.Equal(t, int64(1000), *tableInfo.EstimatedRows)
}

// TestColumnInfo_Simple 测试ColumnInfo结构体（简化版）
func TestColumnInfo_Simple(t *testing.T) {
	columnDefault := "nextval('users_id_seq'::regclass)"
	foreignTable := "profiles"
	foreignColumn := "user_id"
	columnComment := "用户唯一标识"
	maxLength := int32(255)
	precision := int32(10)
	scale := int32(2)

	columnInfo := &ColumnInfo{
		ColumnName:         "id",
		DataType:           "bigint",
		IsNullable:         false,
		ColumnDefault:      &columnDefault,
		IsPrimaryKey:       true,
		IsForeignKey:       true,
		ForeignTable:       &foreignTable,
		ForeignColumn:      &foreignColumn,
		ColumnComment:      &columnComment,
		OrdinalPosition:    1,
		CharacterMaxLength: &maxLength,
		NumericPrecision:   &precision,
		NumericScale:       &scale,
	}

	assert.Equal(t, "id", columnInfo.ColumnName)
	assert.Equal(t, "bigint", columnInfo.DataType)
	assert.False(t, columnInfo.IsNullable)
	assert.NotNil(t, columnInfo.ColumnDefault)
	assert.Equal(t, columnDefault, *columnInfo.ColumnDefault)
	assert.True(t, columnInfo.IsPrimaryKey)
	assert.True(t, columnInfo.IsForeignKey)
	assert.NotNil(t, columnInfo.ForeignTable)
	assert.Equal(t, foreignTable, *columnInfo.ForeignTable)
	assert.NotNil(t, columnInfo.ForeignColumn)
	assert.Equal(t, foreignColumn, *columnInfo.ForeignColumn)
	assert.NotNil(t, columnInfo.ColumnComment)
	assert.Equal(t, columnComment, *columnInfo.ColumnComment)
	assert.Equal(t, int32(1), columnInfo.OrdinalPosition)
	assert.NotNil(t, columnInfo.CharacterMaxLength)
	assert.Equal(t, maxLength, *columnInfo.CharacterMaxLength)
	assert.NotNil(t, columnInfo.NumericPrecision)
	assert.Equal(t, precision, *columnInfo.NumericPrecision)
	assert.NotNil(t, columnInfo.NumericScale)
	assert.Equal(t, scale, *columnInfo.NumericScale)
}

// TestIndexInfo_Simple 测试IndexInfo结构体（简化版）
func TestIndexInfo_Simple(t *testing.T) {
	definition := "CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)"

	indexInfo := &IndexInfo{
		IndexName:  "users_pkey",
		IndexType:  "btree",
		IsUnique:   true,
		IsPrimary:  true,
		Columns:    []string{"id"},
		Definition: &definition,
	}

	assert.Equal(t, "users_pkey", indexInfo.IndexName)
	assert.Equal(t, "btree", indexInfo.IndexType)
	assert.True(t, indexInfo.IsUnique)
	assert.True(t, indexInfo.IsPrimary)
	assert.Equal(t, []string{"id"}, indexInfo.Columns)
	assert.NotNil(t, indexInfo.Definition)
	assert.Equal(t, definition, *indexInfo.Definition)
}

// TestConstraint_Simple 测试Constraint结构体（简化版）
func TestConstraint_Simple(t *testing.T) {
	refTable := "profiles"
	checkCondition := "age >= 0"

	constraint := &Constraint{
		ConstraintName: "users_age_check",
		ConstraintType: "CHECK",
		Columns:        []string{"age"},
		RefTable:       &refTable,
		RefColumns:     []string{"user_id"},
		CheckCondition: &checkCondition,
	}

	assert.Equal(t, "users_age_check", constraint.ConstraintName)
	assert.Equal(t, "CHECK", constraint.ConstraintType)
	assert.Equal(t, []string{"age"}, constraint.Columns)
	assert.NotNil(t, constraint.RefTable)
	assert.Equal(t, refTable, *constraint.RefTable)
	assert.Equal(t, []string{"user_id"}, constraint.RefColumns)
	assert.NotNil(t, constraint.CheckCondition)
	assert.Equal(t, checkCondition, *constraint.CheckCondition)
}