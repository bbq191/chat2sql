package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/repository"
)

// TestSchemaIntrospector_ErrorHandling 测试Schema探测器的错误处理
func TestSchemaIntrospector_ErrorHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("SaveSchemaMetadata_BatchDeleteError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		databaseSchema := &DatabaseSchema{
			ConnectionID: 1,
			Schemas:      []SchemaInfo{},
		}

		// 模拟BatchDelete失败
		mockSchemaRepo.On("BatchDelete", mock.Anything, int64(1)).
			Return(errors.New("database connection failed"))

		ctx := context.Background()
		err := introspector.SaveSchemaMetadata(ctx, databaseSchema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "清除旧元数据失败")
		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("SaveSchemaMetadata_BatchCreateError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

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
								{ColumnName: "id", DataType: "bigint"},
							},
						},
					},
				},
			},
		}

		// BatchDelete成功，但BatchCreate失败
		mockSchemaRepo.On("BatchDelete", mock.Anything, int64(1)).Return(nil)
		mockSchemaRepo.On("BatchCreate", mock.Anything, mock.AnythingOfType("[]*repository.SchemaMetadata")).
			Return(errors.New("database write failed"))

		ctx := context.Background()
		err := introspector.SaveSchemaMetadata(ctx, databaseSchema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "保存Schema元数据失败")
		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("GetTableSummary_RepositoryError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		// 模拟repository错误
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "users").
			Return(nil, errors.New("database query failed"))

		ctx := context.Background()
		summary, err := introspector.GetTableSummary(ctx, 1, "public", "users")

		assert.Error(t, err)
		assert.Empty(t, summary)
		assert.Contains(t, err.Error(), "获取表结构失败")
		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("GetConnectionTableList_RepositoryError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		// 模拟repository错误
		mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "").
			Return(nil, errors.New("connection lost"))

		ctx := context.Background()
		tables, err := introspector.GetConnectionTableList(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, tables)
		assert.Contains(t, err.Error(), "获取表列表失败")
		mockSchemaRepo.AssertExpectations(t)
	})
}

// TestMetadataCache_ErrorHandling 测试元数据缓存的错误处理
func TestMetadataCache_ErrorHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("GetConnectionSchemas_RepositoryError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟ListSchemas失败
		mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).
			Return(nil, errors.New("database connection timeout"))

		ctx := context.Background()
		schemas, err := cache.GetConnectionSchemas(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, schemas)
		assert.Contains(t, err.Error(), "获取Schema列表失败")
		
		// 验证错误统计被正确记录
		stats := cache.GetCacheStatistics()
		assert.Equal(t, int64(1), stats.SchemaMisses)
		assert.Equal(t, int64(1), stats.TotalQueries)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("GetConnectionSchemas_PartialTableListError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// ListSchemas成功，但部分ListTables失败
		mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).
			Return([]string{"public", "app"}, nil)
		mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "public").
			Return([]string{"users"}, nil)
		mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "app").
			Return(nil, errors.New("permission denied"))

		ctx := context.Background()
		schemas, err := cache.GetConnectionSchemas(ctx, 1)

		// 应该成功，但只包含成功的schema
		assert.NoError(t, err)
		assert.Equal(t, []string{"public", "app"}, schemas)

		// 验证缓存中只包含成功获取的表
		cached, ok := cache.connectionSchemas.Load(int64(1))
		assert.True(t, ok)
		cachedSchema := cached.(*CachedConnectionSchema)
		assert.Contains(t, cachedSchema.Tables, "public")
		assert.Len(t, cachedSchema.Tables["public"], 1)
		// app schema的表应该为空或不存在，因为获取失败了

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("GetTableSummary_RepositoryError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟GetTableStructure失败
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "users").
			Return(nil, errors.New("table not accessible"))

		ctx := context.Background()
		summary, columns, err := cache.GetTableSummary(ctx, 1, "public", "users")

		assert.Error(t, err)
		assert.Empty(t, summary)
		assert.Nil(t, columns)
		assert.Contains(t, err.Error(), "获取表结构失败")

		// 验证统计信息
		stats := cache.GetCacheStatistics()
		assert.Equal(t, int64(1), stats.TableMisses)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("GetRelatedTables_Error", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := &MetadataCache{
			schemaRepo:        mockSchemaRepo,
			logger:            logger,
			enablePrefetch:    true,
			prefetchThreshold: 1,
			stats:             &CacheStatistics{},
		}

		// 模拟GetRelatedTables失败
		mockSchemaRepo.On("GetRelatedTables", mock.Anything, int64(1), "users").
			Return(nil, errors.New("foreign key query failed"))

		ctx := context.Background()
		// 调用prefetchRelatedTables方法（这通常在GetTableSummary内部调用）
		cache.prefetchRelatedTables(ctx, 1, "public", "users")

		// 验证预取操作统计被更新
		stats := cache.GetCacheStatistics()
		assert.Equal(t, int64(1), stats.PrefetchOperations)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("Start_AlreadyRunning", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 第一次启动应该成功
		err := cache.Start()
		assert.NoError(t, err)
		assert.True(t, cache.isRunning)

		// 第二次启动应该返回错误
		err = cache.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "已在运行")

		// 清理
		cache.Stop()
	})

	t.Run("PrewarmCache_ListSchemasError", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟第一个ListSchemas调用失败（在GetConnectionSchemas中）
		mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).
			Return(nil, errors.New("database unavailable")).Once()

		ctx := context.Background()
		err := cache.PrewarmCache(ctx, 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "预热Schema缓存失败")

		mockSchemaRepo.AssertExpectations(t)
	})
}

// TestServiceErrors_DatabaseScenarios 测试数据库相关的错误场景
func TestServiceErrors_DatabaseScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("ConnectionTimeout", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		// 模拟连接超时
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "users").
			Return(nil, context.DeadlineExceeded)

		time.Sleep(2 * time.Millisecond) // 确保context超时

		summary, err := introspector.GetTableSummary(timeoutCtx, 1, "public", "users")

		assert.Error(t, err)
		assert.Empty(t, summary)
		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("NoRowsFound", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		// 模拟没有找到行（pgx.ErrNoRows类似的情况）
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "nonexistent").
			Return(nil, sql.ErrNoRows)

		ctx := context.Background()
		summary, err := introspector.GetTableSummary(ctx, 1, "public", "nonexistent")

		assert.Error(t, err)
		assert.Empty(t, summary)
		assert.Contains(t, err.Error(), "获取表结构失败")
		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("DatabaseConnectionLost", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟数据库连接丢失
		connLostErr := errors.New("connection lost: server closed the connection unexpectedly")
		mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).
			Return(nil, connLostErr)

		ctx := context.Background()
		schemas, err := cache.GetConnectionSchemas(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, schemas)
		assert.Contains(t, err.Error(), "connection lost")

		mockSchemaRepo.AssertExpectations(t)
	})
}

// TestConcurrentErrorScenarios 测试并发错误场景
func TestConcurrentErrorScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("ConcurrentCacheAccess_WithErrors", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟并发访问时的错误（一些成功，一些失败）
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "table1").
			Return([]*repository.SchemaMetadata{
				{ConnectionID: 1, SchemaName: "public", TableName: "table1", ColumnName: "id", DataType: "bigint"},
			}, nil)
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "table2").
			Return(nil, errors.New("table locked"))

		ctx := context.Background()
		
		// 并发访问不同的表
		results := make(chan error, 2)
		
		go func() {
			_, _, err := cache.GetTableSummary(ctx, 1, "public", "table1")
			results <- err
		}()
		
		go func() {
			_, _, err := cache.GetTableSummary(ctx, 1, "public", "table2")
			results <- err
		}()

		// 收集结果
		var successCount, errorCount int
		for i := 0; i < 2; i++ {
			err := <-results
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		assert.Equal(t, 1, successCount)
		assert.Equal(t, 1, errorCount)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("CacheInvalidation_DuringOperation", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 预设一些缓存数据
		cache.connectionSchemas.Store(int64(1), &CachedConnectionSchema{
			ConnectionID: 1,
			Schemas:      []string{"public"},
			LastUpdated:  time.Now(),
			TTL:          30 * time.Minute,
		})

		// 在操作进行中使缓存失效
		go func() {
			time.Sleep(10 * time.Millisecond)
			cache.InvalidateConnection(1)
		}()

		// 同时进行缓存访问
		ctx := context.Background()
		schemas, err := cache.GetConnectionSchemas(ctx, 1)

		// 即使缓存被清除，操作也应该能处理（可能会触发重新加载）
		if err != nil {
			// 如果失败，应该是因为没有设置fallback mock
			assert.Contains(t, err.Error(), "ListSchemas")
		} else {
			// 如果成功，说明获取到了缓存数据
			assert.NotNil(t, schemas)
		}
	})
}

// TestConfigurationErrors 测试配置错误场景
func TestConfigurationErrors(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("SchemaIntrospector_InvalidConfig", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}

		// 测试零值配置的默认值设置
		config := &SchemaIntrospectorConfig{
			IntrospectionTimeout: 0,
			MaxTablesPerSchema:   0,
		}

		introspector := NewSchemaIntrospectorWithConfig(nil, mockSchemaRepo, config, logger)

		// 验证默认值被正确设置
		assert.Equal(t, 60*time.Second, introspector.introspectionTimeout)
		assert.Equal(t, 100, introspector.maxTablesPerSchema)
	})

	t.Run("MetadataCache_InvalidConfig", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}

		// 测试无效配置的默认值设置
		config := &MetadataCacheConfig{
			DefaultTTL:      -1 * time.Minute, // 无效值
			MaxCacheSize:    -1,               // 无效值
			CleanupInterval: 0,                // 无效值
		}

		cache := NewMetadataCacheWithConfig(mockSchemaRepo, config, logger)

		// 验证默认值被正确设置
		assert.Equal(t, 30*time.Minute, cache.defaultTTL)
		assert.Equal(t, 10000, cache.maxCacheSize)
		assert.NotNil(t, cache.cleanupTicker)
	})
}

// TestResourceExhaustionScenarios 测试资源耗尽场景
func TestResourceExhaustionScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("MemoryExhaustion_LargeMetadata", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟非常大的元数据集
		largeMetadata := make([]*repository.SchemaMetadata, 10000)
		for i := 0; i < 10000; i++ {
			largeMetadata[i] = &repository.SchemaMetadata{
				ConnectionID:    1,
				SchemaName:      "public",
				TableName:       "large_table",
				ColumnName:      "column_" + string(rune(i)),
				DataType:        "varchar",
				OrdinalPosition: int32(i + 1),
			}
		}

		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "large_table").
			Return(largeMetadata, nil)

		ctx := context.Background()
		summary, columns, err := cache.GetTableSummary(ctx, 1, "public", "large_table")

		// 应该能够处理大型数据集，但可能会消耗大量内存
		assert.NoError(t, err)
		assert.NotEmpty(t, summary)
		assert.Len(t, columns, 10000)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("TooManyConnections", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 模拟数据库连接池耗尽
		mockSchemaRepo.On("ListSchemas", mock.Anything, mock.AnythingOfType("int64")).
			Return(nil, errors.New("too many connections"))

		ctx := context.Background()
		
		// 尝试多个并发连接
		results := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func(connID int64) {
				_, err := cache.GetConnectionSchemas(ctx, connID)
				results <- err
			}(int64(i + 1))
		}

		// 所有连接都应该失败
		errorCount := 0
		for i := 0; i < 10; i++ {
			err := <-results
			if err != nil {
				errorCount++
				assert.Contains(t, err.Error(), "too many connections")
			}
		}

		assert.Equal(t, 10, errorCount)
		mockSchemaRepo.AssertExpectations(t)
	})
}

// TestEdgeCases 测试边缘情况
func TestEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("EmptyTableName", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		introspector := &SchemaIntrospector{
			schemaRepo: mockSchemaRepo,
			logger:     logger,
		}

		// 空表名应该被正确处理
		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "").
			Return([]*repository.SchemaMetadata{}, nil)

		ctx := context.Background()
		summary, err := introspector.GetTableSummary(ctx, 1, "public", "")

		assert.Error(t, err)
		assert.Empty(t, summary)
		assert.Contains(t, err.Error(), "不存在或没有元数据")

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("SpecialCharactersInNames", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 包含特殊字符的表名
		specialTableName := "table-with.special@chars"
		metadata := []*repository.SchemaMetadata{
			{
				ConnectionID:    1,
				SchemaName:      "public",
				TableName:       specialTableName,
				ColumnName:      "id",
				DataType:        "bigint",
				OrdinalPosition: 1,
			},
		}

		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", specialTableName).
			Return(metadata, nil)

		ctx := context.Background()
		summary, columns, err := cache.GetTableSummary(ctx, 1, "public", specialTableName)

		assert.NoError(t, err)
		assert.Contains(t, summary, specialTableName)
		assert.Len(t, columns, 1)

		mockSchemaRepo.AssertExpectations(t)
	})

	t.Run("NilPointerInMetadata", func(t *testing.T) {
		mockSchemaRepo := &MockSchemaRepository{}
		cache := NewMetadataCache(mockSchemaRepo, logger)

		// 包含nil指针的元数据（测试空指针安全性）
		metadata := []*repository.SchemaMetadata{
			nil, // nil元数据项
			{
				ConnectionID:    1,
				SchemaName:      "public",
				TableName:       "users",
				ColumnName:      "id",
				DataType:        "bigint",
				OrdinalPosition: 1,
			},
		}

		mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "users").
			Return(metadata, nil)
		mockSchemaRepo.On("GetRelatedTables", mock.Anything, int64(1), "users").
			Return([]*repository.TableRelation{}, nil)

		ctx := context.Background()
		
		// buildTableSummary应该能够安全处理nil项
		assert.NotPanics(t, func() {
			_, _, _ = cache.GetTableSummary(ctx, 1, "public", "users")
		})

		mockSchemaRepo.AssertExpectations(t)
	})
}