package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/repository"
)

// MockSchemaRepository 模拟Schema Repository - 与schema_introspector_test.go中的定义保持一致
type MockSchemaRepository struct {
	mock.Mock
}

func (m *MockSchemaRepository) Create(ctx context.Context, metadata *repository.SchemaMetadata) error {
	args := m.Called(ctx, metadata)
	return args.Error(0)
}

func (m *MockSchemaRepository) BatchCreate(ctx context.Context, metadata []*repository.SchemaMetadata) error {
	args := m.Called(ctx, metadata)
	return args.Error(0)
}

func (m *MockSchemaRepository) GetByConnection(ctx context.Context, connectionID int64, schemaName, tableName string) (*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID, schemaName, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) ListSchemas(ctx context.Context, connectionID int64) ([]string, error) {
	args := m.Called(ctx, connectionID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]string), args.Error(1)
}

func (m *MockSchemaRepository) ListTables(ctx context.Context, connectionID int64, schemaName string) ([]string, error) {
	args := m.Called(ctx, connectionID, schemaName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]string), args.Error(1)
}

func (m *MockSchemaRepository) GetTableStructure(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID, schemaName, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) GetRelatedTables(ctx context.Context, connectionID int64, tableName string) ([]*repository.TableRelation, error) {
	args := m.Called(ctx, connectionID, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.TableRelation), args.Error(1)
}

func (m *MockSchemaRepository) BatchDelete(ctx context.Context, connectionID int64) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockSchemaRepository) Update(ctx context.Context, metadata *repository.SchemaMetadata) error {
	args := m.Called(ctx, metadata)
	return args.Error(0)
}

func (m *MockSchemaRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSchemaRepository) CountByConnection(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSchemaRepository) GetByID(ctx context.Context, id int64) (*repository.SchemaMetadata, error) {
	args := m.Called(ctx, id)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) ListByConnection(ctx context.Context, connectionID int64) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) ListByTable(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID, schemaName, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) RefreshConnectionMetadata(ctx context.Context, connectionID int64, schemas []*repository.SchemaMetadata) error {
	args := m.Called(ctx, connectionID, schemas)
	return args.Error(0)
}

func (m *MockSchemaRepository) SearchTables(ctx context.Context, connectionID int64, keyword string) ([]*repository.TableInfo, error) {
	args := m.Called(ctx, connectionID, keyword)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.TableInfo), args.Error(1)
}

func (m *MockSchemaRepository) SearchColumns(ctx context.Context, connectionID int64, keyword string) ([]*repository.ColumnInfo, error) {
	args := m.Called(ctx, connectionID, keyword)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.ColumnInfo), args.Error(1)
}

func (m *MockSchemaRepository) GetTableCount(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSchemaRepository) GetColumnCount(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

// TestNewMetadataCache 测试元数据缓存创建
func TestNewMetadataCache(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	assert.NotNil(t, cache)
	assert.Equal(t, mockSchemaRepo, cache.schemaRepo)
	assert.Equal(t, logger, cache.logger)
	assert.Equal(t, 30*time.Minute, cache.defaultTTL)
	assert.Equal(t, 10000, cache.maxCacheSize)
	assert.True(t, cache.enablePrefetch)
	assert.Equal(t, 5, cache.prefetchThreshold)
	assert.NotNil(t, cache.stats)
	assert.NotNil(t, cache.stopCh)
	assert.NotNil(t, cache.cleanupTicker)
}

// TestNewMetadataCacheWithConfig 测试使用自定义配置创建缓存
func TestNewMetadataCacheWithConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	config := &MetadataCacheConfig{
		DefaultTTL:        15 * time.Minute,
		MaxCacheSize:      5000,
		EnablePrefetch:    false,
		PrefetchThreshold: 10,
		CleanupInterval:   2 * time.Minute,
	}

	cache := NewMetadataCacheWithConfig(mockSchemaRepo, config, logger)

	assert.NotNil(t, cache)
	assert.Equal(t, 15*time.Minute, cache.defaultTTL)
	assert.Equal(t, 5000, cache.maxCacheSize)
	assert.False(t, cache.enablePrefetch)
	assert.Equal(t, 10, cache.prefetchThreshold)
}

// TestNewMetadataCacheWithConfig_NilConfig 测试使用nil配置
func TestNewMetadataCacheWithConfig_NilConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCacheWithConfig(mockSchemaRepo, nil, logger)

	assert.NotNil(t, cache)
	assert.Equal(t, 30*time.Minute, cache.defaultTTL)
	assert.Equal(t, 10000, cache.maxCacheSize)
}

// TestNewMetadataCacheWithConfig_DefaultValues 测试默认值设置
func TestNewMetadataCacheWithConfig_DefaultValues(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	config := &MetadataCacheConfig{
		DefaultTTL:        0, // 无效值，应该使用默认值
		MaxCacheSize:      -1, // 无效值，应该使用默认值
		CleanupInterval:   0, // 无效值，应该使用默认值
	}

	cache := NewMetadataCacheWithConfig(mockSchemaRepo, config, logger)

	assert.Equal(t, 30*time.Minute, cache.defaultTTL)
	assert.Equal(t, 10000, cache.maxCacheSize)
	// 注意：cleanupTicker在这里无法直接测试间隔，但不应该为nil
	assert.NotNil(t, cache.cleanupTicker)
}

// TestMetadataCache_StartStop 测试启动和停止缓存服务
func TestMetadataCache_StartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	config := &MetadataCacheConfig{
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 100 * time.Millisecond, // 短间隔用于测试
	}

	cache := NewMetadataCacheWithConfig(mockSchemaRepo, config, logger)

	// 测试启动
	assert.False(t, cache.isRunning)
	err := cache.Start()
	assert.NoError(t, err)
	assert.True(t, cache.isRunning)

	// 重复启动应该返回错误
	err = cache.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已在运行")

	// 等待一段时间让清理例程运行
	time.Sleep(150 * time.Millisecond)

	// 测试停止
	err = cache.Stop()
	assert.NoError(t, err)
	assert.False(t, cache.isRunning)

	// 重复停止应该无操作
	err = cache.Stop()
	assert.NoError(t, err)
}

// TestGetConnectionSchemas_CacheHit 测试Schema缓存命中
func TestGetConnectionSchemas_CacheHit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 预设缓存数据
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: 1,
		Schemas:      []string{"public", "app_schema"},
		Tables:       map[string][]string{"public": {"users", "orders"}},
		LastUpdated:  time.Now(),
		AccessCount:  0,
		TTL:          30 * time.Minute,
	}
	cache.connectionSchemas.Store(int64(1), cachedSchema)

	ctx := context.Background()
	schemas, err := cache.GetConnectionSchemas(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, []string{"public", "app_schema"}, schemas)
	assert.Equal(t, int64(1), cachedSchema.AccessCount)

	// 验证统计信息
	stats := cache.GetCacheStatistics()
	assert.Equal(t, int64(1), stats.SchemaHits)
	assert.Equal(t, int64(0), stats.SchemaMisses)
	assert.Equal(t, int64(1), stats.TotalQueries)
}

// TestGetConnectionSchemas_CacheMiss 测试Schema缓存未命中
func TestGetConnectionSchemas_CacheMiss(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 设置mock期望
	expectedSchemas := []string{"public", "test_schema"}
	mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).Return(expectedSchemas, nil)
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "public").Return([]string{"users"}, nil)
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "test_schema").Return([]string{"tests"}, nil)

	ctx := context.Background()
	schemas, err := cache.GetConnectionSchemas(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expectedSchemas, schemas)

	// 验证缓存已更新
	cached, ok := cache.connectionSchemas.Load(int64(1))
	assert.True(t, ok)
	cachedSchema := cached.(*CachedConnectionSchema)
	assert.Equal(t, expectedSchemas, cachedSchema.Schemas)
	assert.Equal(t, int64(1), cachedSchema.AccessCount)

	// 验证统计信息
	stats := cache.GetCacheStatistics()
	assert.Equal(t, int64(0), stats.SchemaHits)
	assert.Equal(t, int64(1), stats.SchemaMisses)
	assert.Equal(t, int64(1), stats.TotalQueries)

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetConnectionSchemas_CacheExpired 测试Schema缓存过期
func TestGetConnectionSchemas_CacheExpired(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 设置过期的缓存数据
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: 1,
		Schemas:      []string{"old_schema"},
		Tables:       map[string][]string{},
		LastUpdated:  time.Now().Add(-2 * time.Hour), // 2小时前，已过期
		AccessCount:  0,
		TTL:          30 * time.Minute,
	}
	cache.connectionSchemas.Store(int64(1), cachedSchema)

	// 设置mock期望（缓存过期，需要重新获取）
	expectedSchemas := []string{"public", "new_schema"}
	mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).Return(expectedSchemas, nil)
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "public").Return([]string{"users"}, nil)
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "new_schema").Return([]string{"data"}, nil)

	ctx := context.Background()
	schemas, err := cache.GetConnectionSchemas(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expectedSchemas, schemas)

	// 验证统计信息（应该记录为miss）
	stats := cache.GetCacheStatistics()
	assert.Equal(t, int64(1), stats.SchemaMisses)

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetTableSummary_CacheHit 测试表摘要缓存命中
func TestGetTableSummary_CacheHit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 预设缓存数据
	cachedTable := &CachedTableSummary{
		ConnectionID: 1,
		SchemaName:   "public",
		TableName:    "users",
		Summary:      "表 public.users:\n  - id (bigint) [PK]\n  - name (varchar)\n",
		Columns: []CachedColumn{
			{ColumnName: "id", DataType: "bigint", IsPrimaryKey: true},
			{ColumnName: "name", DataType: "varchar"},
		},
		LastUpdated: time.Now(),
		AccessCount: 0,
		TTL:         30 * time.Minute,
	}
	cache.tableSummaries.Store("1:public:users", cachedTable)

	ctx := context.Background()
	summary, columns, err := cache.GetTableSummary(ctx, 1, "public", "users")

	assert.NoError(t, err)
	assert.Contains(t, summary, "表 public.users:")
	assert.Len(t, columns, 2)
	assert.Equal(t, "id", columns[0].ColumnName)
	assert.True(t, columns[0].IsPrimaryKey)
	assert.Equal(t, int64(1), cachedTable.AccessCount)

	// 验证统计信息
	stats := cache.GetCacheStatistics()
	assert.Equal(t, int64(1), stats.TableHits)
	assert.Equal(t, int64(0), stats.TableMisses)
}

// TestGetTableSummary_CacheMiss 测试表摘要缓存未命中
func TestGetTableSummary_CacheMiss(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 准备测试数据
	metadata := []*repository.SchemaMetadata{
		{
			ConnectionID:    1,
			SchemaName:      "public",
			TableName:       "products",
			ColumnName:      "id",
			DataType:        "serial",
			IsNullable:      false,
			IsPrimaryKey:    true,
			OrdinalPosition: 1,
		},
		{
			ConnectionID:    1,
			SchemaName:      "public",
			TableName:       "products",
			ColumnName:      "name",
			DataType:        "varchar",
			IsNullable:      false,
			OrdinalPosition: 2,
		},
	}

	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "products").
		Return(metadata, nil)

	ctx := context.Background()
	summary, columns, err := cache.GetTableSummary(ctx, 1, "public", "products")

	assert.NoError(t, err)
	assert.Contains(t, summary, "表 public.products")
	assert.Len(t, columns, 2)
	assert.Equal(t, "id", columns[0].ColumnName)
	assert.True(t, columns[0].IsPrimaryKey)
	assert.Equal(t, "name", columns[1].ColumnName)
	assert.False(t, columns[1].IsPrimaryKey)

	// 验证缓存已更新
	cached, ok := cache.tableSummaries.Load("1:public:products")
	assert.True(t, ok)
	cachedTable := cached.(*CachedTableSummary)
	assert.Equal(t, int64(1), cachedTable.AccessCount)

	// 验证统计信息
	stats := cache.GetCacheStatistics()
	assert.Equal(t, int64(0), stats.TableHits)
	assert.Equal(t, int64(1), stats.TableMisses)

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetTableSummary_TableNotExists 测试表不存在
func TestGetTableSummary_TableNotExists(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "nonexistent").
		Return([]*repository.SchemaMetadata{}, nil)

	ctx := context.Background()
	summary, columns, err := cache.GetTableSummary(ctx, 1, "public", "nonexistent")

	assert.Error(t, err)
	assert.Empty(t, summary)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "不存在")

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetTableSummary_Prefetch 测试预取功能
func TestGetTableSummary_Prefetch(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	config := &MetadataCacheConfig{
		EnablePrefetch:    true,
		PrefetchThreshold: 2, // 降低阈值便于测试
	}
	cache := NewMetadataCacheWithConfig(mockSchemaRepo, config, logger)

	// 预设缓存数据，访问次数达到预取阈值
	cachedTable := &CachedTableSummary{
		ConnectionID: 1,
		SchemaName:   "public",
		TableName:    "users",
		Summary:      "test summary",
		Columns:      []CachedColumn{},
		LastUpdated:  time.Now(),
		AccessCount:  1, // 下次访问会达到阈值2
		TTL:          30 * time.Minute,
	}
	cache.tableSummaries.Store("1:public:users", cachedTable)

	// 设置预取相关的mock
	relatedTables := []*repository.TableRelation{
		{ToTable: "orders"},
	}
	mockSchemaRepo.On("GetRelatedTables", mock.Anything, int64(1), "users").
		Return(relatedTables, nil)
	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "orders").
		Return([]*repository.SchemaMetadata{
			{
				ConnectionID: 1,
				SchemaName:   "public", 
				TableName:    "orders",
				ColumnName:   "id",
				DataType:     "bigint",
			},
		}, nil)

	ctx := context.Background()
	_, _, err := cache.GetTableSummary(ctx, 1, "public", "users")

	assert.NoError(t, err)

	// 等待异步预取完成
	time.Sleep(100 * time.Millisecond)

	// 验证预取统计
	stats := cache.GetCacheStatistics()
	assert.True(t, stats.PrefetchOperations > 0)

	mockSchemaRepo.AssertExpectations(t)
}

// TestGetQueryPattern 测试查询模式缓存
func TestGetQueryPattern(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 预设连接Schema缓存以支持模式匹配
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: 1,
		Schemas:      []string{"public"},
		Tables:       map[string][]string{"public": {"users", "orders", "products"}},
		LastUpdated:  time.Now(),
		TTL:          30 * time.Minute,
	}
	cache.connectionSchemas.Store(int64(1), cachedSchema)

	ctx := context.Background()
	pattern, err := cache.GetQueryPattern(ctx, 1, "select users")

	assert.NoError(t, err)
	assert.NotNil(t, pattern)
	assert.Equal(t, int64(1), pattern.ConnectionID)
	assert.Equal(t, "select users", pattern.Pattern)
	assert.Contains(t, pattern.RelevantTables, "users")
	assert.Equal(t, "public", pattern.SuggestedSchema)

	// 验证缓存已更新
	cached, ok := cache.queryPatterns.Load("1:select users")
	assert.True(t, ok)
	cachedPattern := cached.(*CachedQueryPattern)
	assert.Equal(t, int64(1), cachedPattern.AccessCount)
}

// TestInvalidateConnection 测试使连接缓存失效
func TestInvalidateConnection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 添加测试数据到各种缓存
	cache.connectionSchemas.Store(int64(1), &CachedConnectionSchema{})
	cache.tableSummaries.Store("1:public:users", &CachedTableSummary{})
	cache.tableSummaries.Store("1:public:orders", &CachedTableSummary{})
	cache.tableSummaries.Store("2:public:products", &CachedTableSummary{}) // 不同连接
	cache.queryPatterns.Store("1:select", &CachedQueryPattern{})
	cache.queryPatterns.Store("2:select", &CachedQueryPattern{}) // 不同连接

	// 使连接1的缓存失效
	cache.InvalidateConnection(1)

	// 验证连接1的缓存已被清除
	_, ok := cache.connectionSchemas.Load(int64(1))
	assert.False(t, ok)

	_, ok = cache.tableSummaries.Load("1:public:users")
	assert.False(t, ok)
	_, ok = cache.tableSummaries.Load("1:public:orders")
	assert.False(t, ok)

	_, ok = cache.queryPatterns.Load("1:select")
	assert.False(t, ok)

	// 验证其他连接的缓存未受影响
	_, ok = cache.tableSummaries.Load("2:public:products")
	assert.True(t, ok)
	_, ok = cache.queryPatterns.Load("2:select")
	assert.True(t, ok)
}

// TestGetCacheSize 测试获取缓存大小
func TestGetCacheSize(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 添加测试数据
	cache.connectionSchemas.Store(int64(1), &CachedConnectionSchema{})
	cache.connectionSchemas.Store(int64(2), &CachedConnectionSchema{})
	cache.tableSummaries.Store("1:public:users", &CachedTableSummary{})
	cache.tableSummaries.Store("1:public:orders", &CachedTableSummary{})
	cache.tableSummaries.Store("2:public:products", &CachedTableSummary{})
	cache.queryPatterns.Store("1:pattern1", &CachedQueryPattern{})

	size := cache.GetCacheSize()

	assert.Equal(t, 2, size["schemas"])
	assert.Equal(t, 3, size["tables"])
	assert.Equal(t, 1, size["patterns"])
	assert.Equal(t, 6, size["total"])
}

// TestPerformCleanup 测试缓存清理
func TestPerformCleanup(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 添加过期的缓存数据
	expiredTime := time.Now().Add(-2 * time.Hour)
	validTime := time.Now()

	expiredSchema := &CachedConnectionSchema{
		LastUpdated: expiredTime,
		TTL:         30 * time.Minute,
	}
	validSchema := &CachedConnectionSchema{
		LastUpdated: validTime,
		TTL:         30 * time.Minute,
	}

	cache.connectionSchemas.Store(int64(1), expiredSchema)
	cache.connectionSchemas.Store(int64(2), validSchema)

	expiredTable := &CachedTableSummary{
		LastUpdated: expiredTime,
		TTL:         30 * time.Minute,
	}
	validTable := &CachedTableSummary{
		LastUpdated: validTime,
		TTL:         30 * time.Minute,
	}

	cache.tableSummaries.Store("1:public:users", expiredTable)
	cache.tableSummaries.Store("2:public:orders", validTable)

	// 执行清理
	cache.performCleanup()

	// 验证过期数据被清除
	_, ok := cache.connectionSchemas.Load(int64(1))
	assert.False(t, ok)
	_, ok = cache.tableSummaries.Load("1:public:users")
	assert.False(t, ok)

	// 验证有效数据保留
	_, ok = cache.connectionSchemas.Load(int64(2))
	assert.True(t, ok)
	_, ok = cache.tableSummaries.Load("2:public:orders")
	assert.True(t, ok)

	// 验证清理统计
	stats := cache.GetCacheStatistics()
	assert.True(t, stats.CacheEvictions >= 2)
}

// TestPrewarmCache 测试缓存预热
func TestPrewarmCache(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 设置mock期望
	schemas := []string{"public", "app"}
	mockSchemaRepo.On("ListSchemas", mock.Anything, int64(1)).Return(schemas, nil).Twice()
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "public").
		Return([]string{"users", "orders", "products", "categories", "reviews", "extras"}, nil)
	mockSchemaRepo.On("ListTables", mock.Anything, int64(1), "app").
		Return([]string{"logs"}, nil)

	// 预热只处理前5个表
	metadata := []*repository.SchemaMetadata{
		{ConnectionID: 1, SchemaName: "public", TableName: "users", ColumnName: "id", DataType: "bigint"},
	}
	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", mock.AnythingOfType("string")).
		Return(metadata, nil).Times(5) // 5次表调用
	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "app", "logs").
		Return(metadata, nil)

	ctx := context.Background()
	err := cache.PrewarmCache(ctx, 1)

	assert.NoError(t, err)

	// 等待异步预热完成
	time.Sleep(100 * time.Millisecond)

	// 验证Schema缓存已预热
	_, ok := cache.connectionSchemas.Load(int64(1))
	assert.True(t, ok)

	mockSchemaRepo.AssertExpectations(t)
}

// TestBuildTableSummary 测试构建表摘要
func TestBuildTableSummary(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 测试空metadata
	summary := cache.buildTableSummary([]*repository.SchemaMetadata{})
	assert.Empty(t, summary)

	// 测试有数据的metadata
	tableComment := "用户信息表"
	columnComment := "用户唯一标识"
	foreignTable := "profiles"

	metadata := []*repository.SchemaMetadata{
		{
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "id",
			DataType:        "bigint",
			IsNullable:      false,
			IsPrimaryKey:    true,
			TableComment:    &tableComment,
			OrdinalPosition: 1,
		},
		{
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "profile_id",
			DataType:        "bigint",
			IsNullable:      true,
			IsForeignKey:    true,
			ForeignTable:    &foreignTable,
			ColumnComment:   &columnComment,
			OrdinalPosition: 2,
		},
	}

	summary = cache.buildTableSummary(metadata)

	assert.Contains(t, summary, "表 public.users (用户信息表):")
	assert.Contains(t, summary, "id (bigint) [PK] [NOT NULL]")
	assert.Contains(t, summary, "profile_id (bigint) [FK->profiles] // 用户唯一标识")
}

// TestFindRelevantTables 测试查找相关表
func TestFindRelevantTables(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 预设连接Schema缓存
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: 1,
		Schemas:      []string{"public"},
		Tables: map[string][]string{
			"public": {"users", "user_profiles", "orders", "products", "categories"},
		},
		LastUpdated: time.Now(),
		TTL:         30 * time.Minute,
	}
	cache.connectionSchemas.Store(int64(1), cachedSchema)

	// 测试查找用户相关的表
	relevantTables := cache.findRelevantTables(context.Background(), 1, "user information")

	assert.Contains(t, relevantTables, "users")
	assert.Contains(t, relevantTables, "user_profiles")
	assert.NotContains(t, relevantTables, "orders")
	assert.NotContains(t, relevantTables, "products")

	// 测试查找order相关的表
	relevantTables = cache.findRelevantTables(context.Background(), 1, "order details")
	assert.Contains(t, relevantTables, "orders")
	assert.NotContains(t, relevantTables, "users")
}

// TestSuggestSchema 测试Schema建议
func TestSuggestSchema(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 测试无相关表的情况
	schema := cache.suggestSchema(context.Background(), 1, []string{})
	assert.Equal(t, "public", schema)

	// 测试有相关表的情况
	schema = cache.suggestSchema(context.Background(), 1, []string{"users", "orders"})
	assert.Equal(t, "public", schema) // 简化实现总是返回public
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 设置mock期望以支持并发访问
	metadata := []*repository.SchemaMetadata{
		{
			ConnectionID: 1,
			SchemaName:   "public",
			TableName:    "test_table",
			ColumnName:   "id",
			DataType:     "bigint",
		},
	}
	mockSchemaRepo.On("GetTableStructure", mock.Anything, int64(1), "public", "test_table").
		Return(metadata, nil)
	
	// 添加GetRelatedTables的mock期望，用于预取相关表
	mockSchemaRepo.On("GetRelatedTables", mock.Anything, int64(1), "test_table").
		Return([]*repository.TableRelation{}, nil)

	var wg sync.WaitGroup
	errorsChan := make(chan error, 10)

	// 并发访问缓存
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_, _, err := cache.GetTableSummary(ctx, 1, "public", "test_table")
			if err != nil {
				errorsChan <- err
			}
		}()
	}

	wg.Wait()
	close(errorsChan)

	// 验证无错误
	for err := range errorsChan {
		assert.NoError(t, err)
	}

	// 验证缓存中只有一个条目
	cached, ok := cache.tableSummaries.Load("1:public:test_table")
	assert.True(t, ok)
	cachedTable := cached.(*CachedTableSummary)
	assert.True(t, cachedTable.AccessCount >= 1) // 至少被访问过一次

	mockSchemaRepo.AssertExpectations(t)
}

// BenchmarkGetTableSummary 性能测试：获取表摘要
func BenchmarkGetTableSummary(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 预设缓存数据
	cachedTable := &CachedTableSummary{
		ConnectionID: 1,
		SchemaName:   "public",
		TableName:    "users",
		Summary:      "test summary",
		Columns:      []CachedColumn{},
		LastUpdated:  time.Now(),
		AccessCount:  0,
		TTL:          30 * time.Minute,
	}
	cache.tableSummaries.Store("1:public:users", cachedTable)

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = cache.GetTableSummary(ctx, 1, "public", "users")
	}
}

// BenchmarkBuildTableSummary_MetadataCache 性能测试：构建表摘要（缓存版本）
func BenchmarkBuildTableSummary_MetadataCache(b *testing.B) {
	logger := zaptest.NewLogger(&testing.T{})
	mockSchemaRepo := &MockSchemaRepository{}

	cache := NewMetadataCache(mockSchemaRepo, logger)

	// 准备测试数据
	metadata := make([]*repository.SchemaMetadata, 50)
	for i := 0; i < 50; i++ {
		metadata[i] = &repository.SchemaMetadata{
			SchemaName:      "public",
			TableName:       "test_table",
			ColumnName:      "column_" + string(rune('a'+i%26)),
			DataType:        "varchar",
			IsNullable:      i%2 == 0,
			IsPrimaryKey:    i == 0,
			OrdinalPosition: int32(i + 1),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.buildTableSummary(metadata)
	}
}

// TestCacheStatisticsStruct 测试缓存统计结构体
func TestCacheStatisticsStruct(t *testing.T) {
	stats := &CacheStatistics{
		SchemaHits:         10,
		SchemaMisses:       5,
		TableHits:          20,
		TableMisses:        8,
		PatternHits:        3,
		PatternMisses:      2,
		TotalQueries:       50,
		CacheEvictions:     1,
		PrefetchOperations: 4,
	}

	assert.Equal(t, int64(10), stats.SchemaHits)
	assert.Equal(t, int64(5), stats.SchemaMisses)
	assert.Equal(t, int64(20), stats.TableHits)
	assert.Equal(t, int64(8), stats.TableMisses)
	assert.Equal(t, int64(3), stats.PatternHits)
	assert.Equal(t, int64(2), stats.PatternMisses)
	assert.Equal(t, int64(50), stats.TotalQueries)
	assert.Equal(t, int64(1), stats.CacheEvictions)
	assert.Equal(t, int64(4), stats.PrefetchOperations)
}

// TestCachedStructures 测试缓存结构体
func TestCachedStructures(t *testing.T) {
	// 测试CachedConnectionSchema
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: 123,
		Schemas:      []string{"public", "app"},
		Tables:       map[string][]string{"public": {"users"}},
		LastUpdated:  time.Now(),
		AccessCount:  5,
		TTL:          30 * time.Minute,
	}

	assert.Equal(t, int64(123), cachedSchema.ConnectionID)
	assert.Len(t, cachedSchema.Schemas, 2)
	assert.Equal(t, int64(5), cachedSchema.AccessCount)

	// 测试CachedTableSummary
	cachedTable := &CachedTableSummary{
		ConnectionID: 456,
		SchemaName:   "public",
		TableName:    "users",
		Summary:      "test summary",
		Columns:      []CachedColumn{{ColumnName: "id"}},
		LastUpdated:  time.Now(),
		AccessCount:  3,
		TTL:          15 * time.Minute,
	}

	assert.Equal(t, int64(456), cachedTable.ConnectionID)
	assert.Equal(t, "public", cachedTable.SchemaName)
	assert.Equal(t, "users", cachedTable.TableName)
	assert.Len(t, cachedTable.Columns, 1)
	assert.Equal(t, int64(3), cachedTable.AccessCount)

	// 测试CachedQueryPattern
	cachedPattern := &CachedQueryPattern{
		ConnectionID:    789,
		Pattern:         "select * from users",
		RelevantTables:  []string{"users"},
		SuggestedSchema: "public",
		LastUpdated:     time.Now(),
		AccessCount:     2,
		TTL:             10 * time.Minute,
	}

	assert.Equal(t, int64(789), cachedPattern.ConnectionID)
	assert.Equal(t, "select * from users", cachedPattern.Pattern)
	assert.Len(t, cachedPattern.RelevantTables, 1)
	assert.Equal(t, "public", cachedPattern.SuggestedSchema)
	assert.Equal(t, int64(2), cachedPattern.AccessCount)
}