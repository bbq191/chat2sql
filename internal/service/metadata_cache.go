package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// MetadataCache 元数据智能缓存
// 提供多级缓存机制，提升Schema查询性能，支持自动过期和智能预加载
type MetadataCache struct {
	// 核心组件
	schemaRepo repository.SchemaRepository // Schema Repository
	logger     *zap.Logger                 // 日志器

	// 缓存存储
	connectionSchemas sync.Map // key: connectionID, value: *CachedConnectionSchema
	tableSummaries    sync.Map // key: "connectionID:schema:table", value: *CachedTableSummary
	queryPatterns     sync.Map // key: "connectionID:pattern", value: *CachedQueryPattern

	// 配置参数
	defaultTTL        time.Duration // 默认缓存TTL
	maxCacheSize      int           // 最大缓存项数
	enablePrefetch    bool          // 是否启用预取
	prefetchThreshold int           // 预取阈值（访问次数）

	// 统计信息
	stats *CacheStatistics

	// 生命周期管理
	isRunning     bool          // 是否运行中
	stopCh        chan struct{} // 停止通道
	cleanupTicker *time.Ticker  // 清理定时器
}

// CachedConnectionSchema 缓存的连接Schema
type CachedConnectionSchema struct {
	ConnectionID int64               `json:"connection_id"`
	Schemas      []string            `json:"schemas"`
	Tables       map[string][]string `json:"tables"` // key: schemaName, value: tableNames
	LastUpdated  time.Time           `json:"last_updated"`
	AccessCount  int64               `json:"access_count"`
	TTL          time.Duration       `json:"-"`
	mutex        sync.RWMutex        `json:"-"`
}

// CachedTableSummary 缓存的表摘要
type CachedTableSummary struct {
	ConnectionID int64          `json:"connection_id"`
	SchemaName   string         `json:"schema_name"`
	TableName    string         `json:"table_name"`
	Summary      string         `json:"summary"`
	Columns      []CachedColumn `json:"columns"`
	LastUpdated  time.Time      `json:"last_updated"`
	AccessCount  int64          `json:"access_count"`
	TTL          time.Duration  `json:"-"`
	mutex        sync.RWMutex   `json:"-"`
}

// CachedColumn 缓存的列信息
type CachedColumn struct {
	ColumnName    string  `json:"column_name"`
	DataType      string  `json:"data_type"`
	IsNullable    bool    `json:"is_nullable"`
	IsPrimaryKey  bool    `json:"is_primary_key"`
	IsForeignKey  bool    `json:"is_foreign_key"`
	ForeignTable  *string `json:"foreign_table"`
	ColumnComment *string `json:"column_comment"`
}

// CachedQueryPattern 缓存的查询模式
type CachedQueryPattern struct {
	ConnectionID    int64         `json:"connection_id"`
	Pattern         string        `json:"pattern"`
	RelevantTables  []string      `json:"relevant_tables"`
	SuggestedSchema string        `json:"suggested_schema"`
	LastUpdated     time.Time     `json:"last_updated"`
	AccessCount     int64         `json:"access_count"`
	TTL             time.Duration `json:"-"`
	mutex           sync.RWMutex  `json:"-"`
}

// CacheStatistics 缓存统计信息
type CacheStatistics struct {
	mutex              sync.RWMutex
	SchemaHits         int64 `json:"schema_hits"`
	SchemaMisses       int64 `json:"schema_misses"`
	TableHits          int64 `json:"table_hits"`
	TableMisses        int64 `json:"table_misses"`
	PatternHits        int64 `json:"pattern_hits"`
	PatternMisses      int64 `json:"pattern_misses"`
	TotalQueries       int64 `json:"total_queries"`
	CacheEvictions     int64 `json:"cache_evictions"`
	PrefetchOperations int64 `json:"prefetch_operations"`
}

// MetadataCacheConfig 元数据缓存配置
type MetadataCacheConfig struct {
	DefaultTTL        time.Duration `json:"default_ttl"`        // 默认缓存TTL，默认30分钟
	MaxCacheSize      int           `json:"max_cache_size"`     // 最大缓存项数，默认10000
	EnablePrefetch    bool          `json:"enable_prefetch"`    // 是否启用预取，默认true
	PrefetchThreshold int           `json:"prefetch_threshold"` // 预取阈值，默认5次访问
	CleanupInterval   time.Duration `json:"cleanup_interval"`   // 清理间隔，默认5分钟
}

// NewMetadataCache 创建元数据缓存
func NewMetadataCache(schemaRepo repository.SchemaRepository, logger *zap.Logger) *MetadataCache {
	config := &MetadataCacheConfig{
		DefaultTTL:        30 * time.Minute,
		MaxCacheSize:      10000,
		EnablePrefetch:    true,
		PrefetchThreshold: 5,
		CleanupInterval:   5 * time.Minute,
	}

	return NewMetadataCacheWithConfig(schemaRepo, config, logger)
}

// NewMetadataCacheWithConfig 使用自定义配置创建元数据缓存
func NewMetadataCacheWithConfig(
	schemaRepo repository.SchemaRepository,
	config *MetadataCacheConfig,
	logger *zap.Logger,
) *MetadataCache {
	if config == nil {
		return NewMetadataCache(schemaRepo, logger)
	}

	// 设置默认值
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 30 * time.Minute
	}
	if config.MaxCacheSize <= 0 {
		config.MaxCacheSize = 10000
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	return &MetadataCache{
		schemaRepo:        schemaRepo,
		logger:            logger,
		defaultTTL:        config.DefaultTTL,
		maxCacheSize:      config.MaxCacheSize,
		enablePrefetch:    config.EnablePrefetch,
		prefetchThreshold: config.PrefetchThreshold,
		stats:             &CacheStatistics{},
		stopCh:            make(chan struct{}),
		cleanupTicker:     time.NewTicker(config.CleanupInterval),
	}
}

// Start 启动缓存服务
func (mc *MetadataCache) Start() error {
	if mc.isRunning {
		return fmt.Errorf("元数据缓存已在运行")
	}

	mc.isRunning = true

	// 启动清理例程
	go mc.cleanupRoutine()

	mc.logger.Info("元数据缓存服务已启动",
		zap.Duration("default_ttl", mc.defaultTTL),
		zap.Int("max_cache_size", mc.maxCacheSize),
		zap.Bool("enable_prefetch", mc.enablePrefetch))

	return nil
}

// Stop 停止缓存服务
func (mc *MetadataCache) Stop() error {
	if !mc.isRunning {
		return nil
	}

	mc.isRunning = false
	close(mc.stopCh)
	mc.cleanupTicker.Stop()

	// 清除所有缓存
	mc.connectionSchemas = sync.Map{}
	mc.tableSummaries = sync.Map{}
	mc.queryPatterns = sync.Map{}

	mc.logger.Info("元数据缓存服务已停止")
	return nil
}

// GetConnectionSchemas 获取连接的Schema列表（带缓存）
func (mc *MetadataCache) GetConnectionSchemas(ctx context.Context, connectionID int64) ([]string, error) {
	mc.stats.mutex.Lock()
	mc.stats.TotalQueries++
	mc.stats.mutex.Unlock()

	// 尝试从缓存获取
	if cached, ok := mc.connectionSchemas.Load(connectionID); ok {
		cachedSchema := cached.(*CachedConnectionSchema)

		cachedSchema.mutex.RLock()
		isExpired := time.Since(cachedSchema.LastUpdated) > cachedSchema.TTL
		cachedSchema.mutex.RUnlock()

		if !isExpired {
			cachedSchema.mutex.Lock()
			cachedSchema.AccessCount++
			cachedSchema.mutex.Unlock()

			mc.stats.mutex.Lock()
			mc.stats.SchemaHits++
			mc.stats.mutex.Unlock()

			mc.logger.Debug("命中Schema缓存",
				zap.Int64("connection_id", connectionID))

			return cachedSchema.Schemas, nil
		}
	}

	// 缓存未命中，从数据库获取
	mc.stats.mutex.Lock()
	mc.stats.SchemaMisses++
	mc.stats.mutex.Unlock()

	schemas, err := mc.schemaRepo.ListSchemas(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("获取Schema列表失败: %w", err)
	}

	// 获取每个schema的表列表
	tablesBySchema := make(map[string][]string)
	for _, schema := range schemas {
		tables, err := mc.schemaRepo.ListTables(ctx, connectionID, schema)
		if err != nil {
			mc.logger.Warn("获取表列表失败",
				zap.String("schema", schema),
				zap.Error(err))
			continue
		}
		tablesBySchema[schema] = tables
	}

	// 更新缓存
	cachedSchema := &CachedConnectionSchema{
		ConnectionID: connectionID,
		Schemas:      schemas,
		Tables:       tablesBySchema,
		LastUpdated:  time.Now(),
		AccessCount:  1,
		TTL:          mc.defaultTTL,
	}

	mc.connectionSchemas.Store(connectionID, cachedSchema)

	mc.logger.Debug("Schema缓存已更新",
		zap.Int64("connection_id", connectionID),
		zap.Int("schema_count", len(schemas)))

	return schemas, nil
}

// GetTableSummary 获取表摘要（带缓存）
func (mc *MetadataCache) GetTableSummary(ctx context.Context, connectionID int64, schemaName, tableName string) (string, []CachedColumn, error) {
	mc.stats.mutex.Lock()
	mc.stats.TotalQueries++
	mc.stats.mutex.Unlock()

	cacheKey := fmt.Sprintf("%d:%s:%s", connectionID, schemaName, tableName)

	// 尝试从缓存获取
	if cached, ok := mc.tableSummaries.Load(cacheKey); ok {
		cachedTable := cached.(*CachedTableSummary)

		cachedTable.mutex.RLock()
		isExpired := time.Since(cachedTable.LastUpdated) > cachedTable.TTL
		cachedTable.mutex.RUnlock()

		if !isExpired {
			cachedTable.mutex.Lock()
			cachedTable.AccessCount++
			cachedTable.mutex.Unlock()

			mc.stats.mutex.Lock()
			mc.stats.TableHits++
			mc.stats.mutex.Unlock()

			// 检查是否需要预取相关表
			if mc.enablePrefetch && cachedTable.AccessCount >= int64(mc.prefetchThreshold) {
				go mc.prefetchRelatedTables(ctx, connectionID, schemaName, tableName)
			}

			mc.logger.Debug("命中表摘要缓存",
				zap.String("cache_key", cacheKey))

			return cachedTable.Summary, cachedTable.Columns, nil
		}
	}

	// 缓存未命中，从数据库获取
	mc.stats.mutex.Lock()
	mc.stats.TableMisses++
	mc.stats.mutex.Unlock()

	// 获取表结构
	schemaMetadataList, err := mc.schemaRepo.GetTableStructure(ctx, connectionID, schemaName, tableName)
	if err != nil {
		return "", nil, fmt.Errorf("获取表结构失败: %w", err)
	}

	if len(schemaMetadataList) == 0 {
		return "", nil, fmt.Errorf("表 %s.%s 不存在", schemaName, tableName)
	}

	// 构建表摘要
	summary := mc.buildTableSummary(schemaMetadataList)

	// 转换为缓存格式
	var cachedColumns []CachedColumn
	for _, metadata := range schemaMetadataList {
		column := CachedColumn{
			ColumnName:    metadata.ColumnName,
			DataType:      metadata.DataType,
			IsNullable:    metadata.IsNullable,
			IsPrimaryKey:  metadata.IsPrimaryKey,
			IsForeignKey:  metadata.IsForeignKey,
			ForeignTable:  metadata.ForeignTable,
			ColumnComment: metadata.ColumnComment,
		}
		cachedColumns = append(cachedColumns, column)
	}

	// 更新缓存
	cachedTable := &CachedTableSummary{
		ConnectionID: connectionID,
		SchemaName:   schemaName,
		TableName:    tableName,
		Summary:      summary,
		Columns:      cachedColumns,
		LastUpdated:  time.Now(),
		AccessCount:  1,
		TTL:          mc.defaultTTL,
	}

	mc.tableSummaries.Store(cacheKey, cachedTable)

	mc.logger.Debug("表摘要缓存已更新",
		zap.String("cache_key", cacheKey))

	return summary, cachedColumns, nil
}

// buildTableSummary 构建表摘要
func (mc *MetadataCache) buildTableSummary(schemaMetadataList []*repository.SchemaMetadata) string {
	if len(schemaMetadataList) == 0 {
		return ""
	}

	first := schemaMetadataList[0]
	if first == nil {
		return ""
	}
	summary := fmt.Sprintf("表 %s.%s", first.SchemaName, first.TableName)

	if first.TableComment != nil {
		summary += fmt.Sprintf(" (%s)", *first.TableComment)
	}

	summary += ":\n"

	for _, metadata := range schemaMetadataList {
		if metadata == nil {
			continue
		}
		summary += fmt.Sprintf("  - %s (%s)", metadata.ColumnName, metadata.DataType)

		if metadata.IsPrimaryKey {
			summary += " [PK]"
		}
		if metadata.IsForeignKey && metadata.ForeignTable != nil {
			summary += fmt.Sprintf(" [FK->%s]", *metadata.ForeignTable)
		}
		if !metadata.IsNullable {
			summary += " [NOT NULL]"
		}
		if metadata.ColumnComment != nil {
			summary += fmt.Sprintf(" // %s", *metadata.ColumnComment)
		}

		summary += "\n"
	}

	return summary
}

// prefetchRelatedTables 预取相关表
func (mc *MetadataCache) prefetchRelatedTables(ctx context.Context, connectionID int64, schemaName, tableName string) {
	mc.stats.mutex.Lock()
	mc.stats.PrefetchOperations++
	mc.stats.mutex.Unlock()

	mc.logger.Debug("开始预取相关表",
		zap.Int64("connection_id", connectionID),
		zap.String("schema", schemaName),
		zap.String("table", tableName))

	// 查找外键相关的表
	relatedTables, err := mc.schemaRepo.GetRelatedTables(ctx, connectionID, tableName)
	if err != nil {
		mc.logger.Warn("获取相关表列表失败",
			zap.Error(err))
		return
	}

	// 异步预取相关表的摘要
	for _, relatedTable := range relatedTables {
		go func(relTable *repository.TableRelation) {
			_, _, err := mc.GetTableSummary(ctx, connectionID, schemaName, relTable.ToTable)
			if err != nil {
				mc.logger.Debug("预取表摘要失败",
					zap.String("table", relTable.ToTable),
					zap.Error(err))
			}
		}(relatedTable)
	}
}

// GetQueryPattern 获取查询模式（带缓存）
func (mc *MetadataCache) GetQueryPattern(ctx context.Context, connectionID int64, pattern string) (*CachedQueryPattern, error) {
	cacheKey := fmt.Sprintf("%d:%s", connectionID, pattern)

	// 尝试从缓存获取
	if cached, ok := mc.queryPatterns.Load(cacheKey); ok {
		cachedPattern := cached.(*CachedQueryPattern)

		cachedPattern.mutex.RLock()
		isExpired := time.Since(cachedPattern.LastUpdated) > cachedPattern.TTL
		cachedPattern.mutex.RUnlock()

		if !isExpired {
			cachedPattern.mutex.Lock()
			cachedPattern.AccessCount++
			cachedPattern.mutex.Unlock()

			mc.stats.mutex.Lock()
			mc.stats.PatternHits++
			mc.stats.mutex.Unlock()

			return cachedPattern, nil
		}
	}

	// 缓存未命中，生成新的查询模式
	mc.stats.mutex.Lock()
	mc.stats.PatternMisses++
	mc.stats.mutex.Unlock()

	// 简化的模式匹配逻辑
	relevantTables := mc.findRelevantTables(ctx, connectionID, pattern)
	suggestedSchema := mc.suggestSchema(ctx, connectionID, relevantTables)

	cachedPattern := &CachedQueryPattern{
		ConnectionID:    connectionID,
		Pattern:         pattern,
		RelevantTables:  relevantTables,
		SuggestedSchema: suggestedSchema,
		LastUpdated:     time.Now(),
		AccessCount:     1,
		TTL:             mc.defaultTTL,
	}

	mc.queryPatterns.Store(cacheKey, cachedPattern)

	return cachedPattern, nil
}

// findRelevantTables 查找相关表
func (mc *MetadataCache) findRelevantTables(_ context.Context, connectionID int64, pattern string) []string {
	// 简化实现：通过关键词匹配查找相关表
	var relevantTables []string

	// 从缓存的Schema信息中查找
	if cached, ok := mc.connectionSchemas.Load(connectionID); ok {
		cachedSchema := cached.(*CachedConnectionSchema)

		cachedSchema.mutex.RLock()
		defer cachedSchema.mutex.RUnlock()

		lowerPattern := strings.ToLower(pattern)

		for _, tables := range cachedSchema.Tables {
			for _, table := range tables {
				lowerTable := strings.ToLower(table)
				// 检查模式词汇是否与表名相关
				for _, word := range strings.Fields(lowerPattern) {
					if strings.Contains(lowerTable, word) || strings.Contains(word, lowerTable) {
						relevantTables = append(relevantTables, table)
						break
					}
				}
			}
		}
	}

	return relevantTables
}

// suggestSchema 建议Schema
func (mc *MetadataCache) suggestSchema(_ context.Context, _ int64, relevantTables []string) string {
	if len(relevantTables) == 0 {
		return "public" // 默认返回public schema
	}

	// 简化实现：返回第一个相关表所在的schema
	return "public"
}

// InvalidateConnection 使连接的所有缓存失效
func (mc *MetadataCache) InvalidateConnection(connectionID int64) {
	// 清除连接Schema缓存
	mc.connectionSchemas.Delete(connectionID)

	// 清除表摘要缓存
	mc.tableSummaries.Range(func(key, value any) bool {
		keyStr := key.(string)
		if strings.HasPrefix(keyStr, fmt.Sprintf("%d:", connectionID)) {
			mc.tableSummaries.Delete(key)
		}
		return true
	})

	// 清除查询模式缓存
	mc.queryPatterns.Range(func(key, value any) bool {
		keyStr := key.(string)
		if strings.HasPrefix(keyStr, fmt.Sprintf("%d:", connectionID)) {
			mc.queryPatterns.Delete(key)
		}
		return true
	})

	mc.logger.Info("连接缓存已清除",
		zap.Int64("connection_id", connectionID))
}

// GetCacheStatistics 获取缓存统计信息
func (mc *MetadataCache) GetCacheStatistics() *CacheStatistics {
	mc.stats.mutex.RLock()
	defer mc.stats.mutex.RUnlock()

	// 返回统计信息的副本
	return &CacheStatistics{
		SchemaHits:         mc.stats.SchemaHits,
		SchemaMisses:       mc.stats.SchemaMisses,
		TableHits:          mc.stats.TableHits,
		TableMisses:        mc.stats.TableMisses,
		PatternHits:        mc.stats.PatternHits,
		PatternMisses:      mc.stats.PatternMisses,
		TotalQueries:       mc.stats.TotalQueries,
		CacheEvictions:     mc.stats.CacheEvictions,
		PrefetchOperations: mc.stats.PrefetchOperations,
	}
}

// cleanupRoutine 清理过期缓存例程
func (mc *MetadataCache) cleanupRoutine() {
	for {
		select {
		case <-mc.stopCh:
			return
		case <-mc.cleanupTicker.C:
			mc.performCleanup()
		}
	}
}

// performCleanup 执行缓存清理
func (mc *MetadataCache) performCleanup() {
	now := time.Now()
	var evictions int64

	// 清理过期的连接Schema缓存
	mc.connectionSchemas.Range(func(key, value any) bool {
		cached := value.(*CachedConnectionSchema)
		cached.mutex.RLock()
		isExpired := now.Sub(cached.LastUpdated) > cached.TTL
		cached.mutex.RUnlock()

		if isExpired {
			mc.connectionSchemas.Delete(key)
			evictions++
		}
		return true
	})

	// 清理过期的表摘要缓存
	mc.tableSummaries.Range(func(key, value any) bool {
		cached := value.(*CachedTableSummary)
		cached.mutex.RLock()
		isExpired := now.Sub(cached.LastUpdated) > cached.TTL
		cached.mutex.RUnlock()

		if isExpired {
			mc.tableSummaries.Delete(key)
			evictions++
		}
		return true
	})

	// 清理过期的查询模式缓存
	mc.queryPatterns.Range(func(key, value any) bool {
		cached := value.(*CachedQueryPattern)
		cached.mutex.RLock()
		isExpired := now.Sub(cached.LastUpdated) > cached.TTL
		cached.mutex.RUnlock()

		if isExpired {
			mc.queryPatterns.Delete(key)
			evictions++
		}
		return true
	})

	if evictions > 0 {
		mc.stats.mutex.Lock()
		mc.stats.CacheEvictions += evictions
		mc.stats.mutex.Unlock()

		mc.logger.Debug("缓存清理完成",
			zap.Int64("evictions", evictions))
	}
}

// GetCacheSize 获取当前缓存大小
func (mc *MetadataCache) GetCacheSize() map[string]int {
	var schemaCount, tableCount, patternCount int

	mc.connectionSchemas.Range(func(key, value any) bool {
		schemaCount++
		return true
	})

	mc.tableSummaries.Range(func(key, value any) bool {
		tableCount++
		return true
	})

	mc.queryPatterns.Range(func(key, value any) bool {
		patternCount++
		return true
	})

	return map[string]int{
		"schemas":  schemaCount,
		"tables":   tableCount,
		"patterns": patternCount,
		"total":    schemaCount + tableCount + patternCount,
	}
}

// PrewarmCache 预热缓存
func (mc *MetadataCache) PrewarmCache(ctx context.Context, connectionID int64) error {
	mc.logger.Info("开始预热缓存",
		zap.Int64("connection_id", connectionID))

	// 预热Schema列表
	_, err := mc.GetConnectionSchemas(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("预热Schema缓存失败: %w", err)
	}

	// 预热常用表的摘要
	schemas, err := mc.schemaRepo.ListSchemas(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("获取Schema列表失败: %w", err)
	}

	for _, schema := range schemas {
		tables, err := mc.schemaRepo.ListTables(ctx, connectionID, schema)
		if err != nil {
			continue
		}

		// 只预热前几个表，避免过度预热
		maxPrewarmTables := 5
		if len(tables) > maxPrewarmTables {
			tables = tables[:maxPrewarmTables]
		}

		for _, table := range tables {
			go func(s, t string) {
				_, _, err := mc.GetTableSummary(ctx, connectionID, s, t)
				if err != nil {
					mc.logger.Debug("预热表摘要失败",
						zap.String("schema", s),
						zap.String("table", t),
						zap.Error(err))
				}
			}(schema, table)
		}
	}

	mc.logger.Info("缓存预热完成",
		zap.Int64("connection_id", connectionID))

	return nil
}
