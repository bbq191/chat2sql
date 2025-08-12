// Package ai 上下文管理器实现
package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ContextManager 上下文管理器
type ContextManager struct {
	// 数据库结构缓存
	schemaCache map[int64]*SchemaInfo
	
	// 查询历史缓存
	historyBuffer map[int64]*UserQueryHistory
	
	// 用户上下文缓存  
	userContextCache map[int64]*UserContext
	
	// 配置参数
	config *ContextConfig
	
	// 读写锁保护并发访问
	mu sync.RWMutex
	
	// 清理任务取消函数
	cleanupCancel context.CancelFunc
}

// ContextConfig 上下文管理器配置
type ContextConfig struct {
	// 最大历史记录数量
	MaxHistorySize int `yaml:"max_history_size"`
	
	// 缓存过期时间
	CacheTTL time.Duration `yaml:"cache_ttl"`
	
	// 清理任务间隔
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	
	// 最大连接数缓存
	MaxConnectionsCache int `yaml:"max_connections_cache"`
	
	// 启用智能预热
	EnablePrewarming bool `yaml:"enable_prewarming"`
}

// SchemaInfo 数据库结构信息
type SchemaInfo struct {
	ConnectionID  int64               `json:"connection_id"`
	DatabaseName  string              `json:"database_name"`
	Tables        map[string]*Table   `json:"tables"`
	Relationships []Relationship      `json:"relationships"`
	Indexes       []IndexInfo         `json:"indexes"`
	LastUpdated   time.Time           `json:"last_updated"`
	Version       string              `json:"version"`
}

// Table 表结构信息
type Table struct {
	Name        string              `json:"name"`
	Columns     map[string]*Column  `json:"columns"`
	PrimaryKeys []string            `json:"primary_keys"`
	ForeignKeys []ForeignKey        `json:"foreign_keys"`
	Indexes     []string            `json:"indexes"`
	Comment     string              `json:"comment"`
}

// Column 列信息
type Column struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value"`
	Comment      string `json:"comment"`
	MaxLength    int    `json:"max_length,omitempty"`
	Precision    int    `json:"precision,omitempty"`
	Scale        int    `json:"scale,omitempty"`
}

// ForeignKey 外键信息
type ForeignKey struct {
	ColumnName       string `json:"column_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

// Relationship 表关系信息
type Relationship struct {
	FromTable  string `json:"from_table"`
	ToTable    string `json:"to_table"`
	Type       string `json:"type"` // one-to-one, one-to-many, many-to-many
	JoinColumn string `json:"join_column"`
}

// IndexInfo 索引信息
type IndexInfo struct {
	Name      string   `json:"name"`
	TableName string   `json:"table_name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique"`
	Type      string   `json:"type"`
}

// UserQueryHistory 用户查询历史
type UserQueryHistory struct {
	UserID       int64           `json:"user_id"`
	Queries      []QueryHistory  `json:"queries"`
	LastAccess   time.Time       `json:"last_access"`
	TotalQueries int             `json:"total_queries"`
}

// UserContext 用户上下文信息
type UserContext struct {
	UserID          int64             `json:"user_id"`
	PreferredLang   string            `json:"preferred_lang"`
	QueryPatterns   []string          `json:"query_patterns"`
	FrequentTables  []string          `json:"frequent_tables"`
	CustomSettings  map[string]string `json:"custom_settings"`
	LastActive      time.Time         `json:"last_active"`
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(config *ContextConfig) *ContextManager {
	if config == nil {
		config = &ContextConfig{
			MaxHistorySize:      50,
			CacheTTL:           time.Hour * 24,
			CleanupInterval:    time.Hour,
			MaxConnectionsCache: 100,
			EnablePrewarming:   true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	cm := &ContextManager{
		schemaCache:      make(map[int64]*SchemaInfo),
		historyBuffer:    make(map[int64]*UserQueryHistory),
		userContextCache: make(map[int64]*UserContext),
		config:           config,
		cleanupCancel:    cancel,
	}

	// 启动后台清理任务（只在间隔大于0时启动）
	if config.CleanupInterval > 0 {
		go cm.backgroundCleanup(ctx)
	}

	return cm
}

// CacheSchema 缓存数据库结构信息
func (cm *ContextManager) CacheSchema(connectionID int64, schema *SchemaInfo) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查缓存大小限制
	if len(cm.schemaCache) >= cm.config.MaxConnectionsCache {
		// 删除最旧的缓存项
		var oldestID int64
		var oldestTime time.Time
		
		for id, info := range cm.schemaCache {
			if oldestTime.IsZero() || info.LastUpdated.Before(oldestTime) {
				oldestID = id
				oldestTime = info.LastUpdated
			}
		}
		
		delete(cm.schemaCache, oldestID)
	}

	schema.LastUpdated = time.Now()
	cm.schemaCache[connectionID] = schema

	return nil
}

// GetSchema 获取数据库结构信息
func (cm *ContextManager) GetSchema(connectionID int64) (*SchemaInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	schema, exists := cm.schemaCache[connectionID]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(schema.LastUpdated) > cm.config.CacheTTL {
		return nil, false
	}

	return schema, true
}

// BuildQueryContext 构建查询上下文
func (cm *ContextManager) BuildQueryContext(connectionID, userID int64, userQuery string) (*QueryContext, error) {
	if userQuery == "" {
		return nil, fmt.Errorf("用户查询不能为空")
	}

	// 验证上下文安全性
	if err := ValidatePromptContext(&QueryContext{UserQuery: userQuery}); err != nil {
		return nil, fmt.Errorf("查询上下文验证失败: %w", err)
	}

	// 获取数据库结构
	schema, _ := cm.GetSchema(connectionID)
	var schemaStr string
	var tableNames []string
	
	if schema != nil {
		schemaStr = cm.formatSchemaForPrompt(schema)
		tableNames = cm.extractTableNames(schema)
	}

	// 获取查询历史
	history := cm.GetRecentHistory(userID, 5)

	// 获取用户上下文
	userContext := cm.getUserContext(userID)
	
	ctx := &QueryContext{
		UserQuery:      userQuery,
		DatabaseSchema: schemaStr,
		TableNames:     tableNames,
		QueryHistory:   history,
		UserContext:    cm.buildUserContextMap(userContext),
		Timestamp:      time.Now(),
	}

	return ctx, nil
}

// AddQueryHistory 添加查询历史记录
func (cm *ContextManager) AddQueryHistory(userID int64, query, sql string, success bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 获取或创建用户历史记录
	userHistory, exists := cm.historyBuffer[userID]
	if !exists {
		userHistory = &UserQueryHistory{
			UserID:       userID,
			Queries:      make([]QueryHistory, 0, cm.config.MaxHistorySize),
			LastAccess:   time.Now(),
			TotalQueries: 0,
		}
		cm.historyBuffer[userID] = userHistory
	}

	// 添加新记录
	newRecord := QueryHistory{
		Query:     query,
		SQL:       sql,
		Success:   success,
		Timestamp: time.Now(),
	}

	userHistory.Queries = append(userHistory.Queries, newRecord)
	userHistory.TotalQueries++
	userHistory.LastAccess = time.Now()

	// 保持历史记录数量限制
	if len(userHistory.Queries) > cm.config.MaxHistorySize {
		// 删除最旧的记录，保留最新的记录
		copy(userHistory.Queries, userHistory.Queries[1:])
		userHistory.Queries = userHistory.Queries[:cm.config.MaxHistorySize]
	}

	// 更新用户上下文（在持有锁的情况下）
	if success {
		cm.updateUserContextLocked(userID, query, sql)
	}

	return nil
}

// GetRecentHistory 获取最近的查询历史
func (cm *ContextManager) GetRecentHistory(userID int64, limit int) []QueryHistory {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	userHistory, exists := cm.historyBuffer[userID]
	if !exists {
		return []QueryHistory{}
	}

	// 更新访问时间
	userHistory.LastAccess = time.Now()

	// 返回最近的成功记录
	var recentHistory []QueryHistory
	count := 0
	
	for i := len(userHistory.Queries) - 1; i >= 0 && count < limit; i-- {
		if userHistory.Queries[i].Success {
			recentHistory = append(recentHistory, userHistory.Queries[i])
			count++
		}
	}

	return recentHistory
}

// formatSchemaForPrompt 将结构信息格式化为提示词格式
func (cm *ContextManager) formatSchemaForPrompt(schema *SchemaInfo) string {
	if schema == nil {
		return ""
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("数据库: %s\n", schema.DatabaseName))
	result.WriteString("表结构信息:\n\n")

	for tableName, table := range schema.Tables {
		result.WriteString(fmt.Sprintf("📋 表: %s", tableName))
		if table.Comment != "" {
			result.WriteString(fmt.Sprintf(" (%s)", table.Comment))
		}
		result.WriteString("\n")

		// 输出列信息
		for colName, col := range table.Columns {
			result.WriteString(fmt.Sprintf("  - %s: %s", colName, col.DataType))
			if !col.IsNullable {
				result.WriteString(" NOT NULL")
			}
			if col.Comment != "" {
				result.WriteString(fmt.Sprintf(" // %s", col.Comment))
			}
			result.WriteString("\n")
		}

		// 输出主键信息
		if len(table.PrimaryKeys) > 0 {
			result.WriteString(fmt.Sprintf("  🔑 主键: %v\n", table.PrimaryKeys))
		}

		// 输出外键信息
		if len(table.ForeignKeys) > 0 {
			result.WriteString("  🔗 外键:\n")
			for _, fk := range table.ForeignKeys {
				result.WriteString(fmt.Sprintf("    %s -> %s.%s\n", 
					fk.ColumnName, fk.ReferencedTable, fk.ReferencedColumn))
			}
		}

		result.WriteString("\n")
	}

	return result.String()
}

// extractTableNames 提取表名列表
func (cm *ContextManager) extractTableNames(schema *SchemaInfo) []string {
	if schema == nil {
		return []string{}
	}

	tableNames := make([]string, 0, len(schema.Tables))
	for tableName := range schema.Tables {
		tableNames = append(tableNames, tableName)
	}

	return tableNames
}

// getUserContext 获取用户上下文
func (cm *ContextManager) getUserContext(userID int64) *UserContext {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ctx, exists := cm.userContextCache[userID]
	if !exists {
		// 创建默认用户上下文
		ctx = &UserContext{
			UserID:         userID,
			PreferredLang:  "zh-CN",
			QueryPatterns:  []string{},
			FrequentTables: []string{},
			CustomSettings: make(map[string]string),
			LastActive:     time.Now(),
		}
		cm.userContextCache[userID] = ctx
	}

	ctx.LastActive = time.Now()
	return ctx
}

// buildUserContextMap 构建用户上下文映射
func (cm *ContextManager) buildUserContextMap(ctx *UserContext) map[string]string {
	if ctx == nil {
		return make(map[string]string)
	}

	result := make(map[string]string)
	result["preferred_lang"] = ctx.PreferredLang
	result["frequent_tables"] = fmt.Sprintf("%v", ctx.FrequentTables)
	
	// 合并自定义设置
	for k, v := range ctx.CustomSettings {
		result[k] = v
	}

	return result
}

// updateUserContext 更新用户上下文
func (cm *ContextManager) updateUserContext(userID int64, query, sql string, success bool) {
	userCtx := cm.getUserContext(userID)
	
	if success {
		// 分析查询模式
		cm.analyzeQueryPattern(userCtx, query, sql)
	}
}

// updateUserContextLocked 在持有锁的情况下更新用户上下文
func (cm *ContextManager) updateUserContextLocked(userID int64, query, sql string) {
	// 获取或创建用户上下文（在已持有锁的情况下）
	ctx, exists := cm.userContextCache[userID]
	if !exists {
		ctx = &UserContext{
			UserID:         userID,
			PreferredLang:  "zh-CN",
			QueryPatterns:  []string{},
			FrequentTables: []string{},
			CustomSettings: make(map[string]string),
			LastActive:     time.Now(),
		}
		cm.userContextCache[userID] = ctx
	}

	ctx.LastActive = time.Now()
	
	// 分析查询模式
	cm.analyzeQueryPattern(ctx, query, sql)
}

// analyzeQueryPattern 分析查询模式 - 更智能的模式识别实现
func (cm *ContextManager) analyzeQueryPattern(ctx *UserContext, query, sql string) {
	if query == "" || sql == "" {
		return
	}
	
	// 1. 提取用户常用的表名
	tableNames := cm.extractTableNamesFromSQL(sql)
	for _, tableName := range tableNames {
		if !cm.containsString(ctx.FrequentTables, tableName) {
			ctx.FrequentTables = append(ctx.FrequentTables, tableName)
			// 限制常用表数量
			if len(ctx.FrequentTables) > 10 {
				ctx.FrequentTables = ctx.FrequentTables[1:]
			}
		}
	}
	
	// 2. 分析查询模式类型
	queryPattern := cm.identifyQueryPattern(query, sql)
	if queryPattern != "" && !cm.containsString(ctx.QueryPatterns, queryPattern) {
		ctx.QueryPatterns = append(ctx.QueryPatterns, queryPattern)
		// 限制模式数量
		if len(ctx.QueryPatterns) > 5 {
			ctx.QueryPatterns = ctx.QueryPatterns[1:]
		}
	}
	
	// 3. 更新自定义设置
	cm.updateCustomSettings(ctx, query, sql)
}

// extractTableNamesFromSQL 从SQL中提取表名
func (cm *ContextManager) extractTableNamesFromSQL(sql string) []string {
	var tables []string
	sql = strings.ToLower(sql)
	
	// 简单的表名提取逻辑
	words := strings.Fields(sql)
	for i, word := range words {
		if (word == "from" || word == "join" || word == "update" || word == "into") && i+1 < len(words) {
			tableName := strings.Trim(words[i+1], "(),;")
			if tableName != "" && !strings.Contains(tableName, "(") {
				tables = append(tables, tableName)
			}
		}
	}
	
	return tables
}

// identifyQueryPattern 识别查询模式
func (cm *ContextManager) identifyQueryPattern(query, sql string) string {
	query = strings.ToLower(query)
	sql = strings.ToLower(sql)
	
	// 基于关键词识别查询模式
	if strings.Contains(query, "统计") || strings.Contains(query, "总数") || 
	   strings.Contains(sql, "count") || strings.Contains(sql, "sum") {
		return "aggregation_queries"
	}
	
	if strings.Contains(query, "时间") || strings.Contains(query, "日期") || 
	   strings.Contains(query, "趋势") || strings.Contains(sql, "date") {
		return "time_analysis"
	}
	
	if strings.Contains(sql, "join") || strings.Contains(query, "关联") {
		return "join_queries"
	}
	
	if strings.Contains(sql, "order by") || strings.Contains(query, "排序") || 
	   strings.Contains(query, "最高") || strings.Contains(query, "最低") {
		return "ranking_queries"
	}
	
	if strings.Contains(sql, "group by") || strings.Contains(query, "分组") {
		return "grouping_queries"
	}
	
	return "basic_select"
}

// updateCustomSettings 更新用户自定义设置
func (cm *ContextManager) updateCustomSettings(ctx *UserContext, query, sql string) {
	// 分析查询偏好
	if strings.Contains(query, "limit") || strings.Contains(query, "前") {
		ctx.CustomSettings["prefers_limit"] = "true"
	}
	
	if strings.Contains(sql, "order by") {
		ctx.CustomSettings["uses_ordering"] = "true"
	}
	
	// 分析语言偏好
	if containsChinese(query) {
		ctx.CustomSettings["preferred_lang"] = "zh-CN"
	} else {
		ctx.CustomSettings["preferred_lang"] = "en-US"
	}
}

// containsString 检查字符串是否在切片中
func (cm *ContextManager) containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// containsChinese 检查字符串是否包含中文字符
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// backgroundCleanup 后台清理任务
func (cm *ContextManager) backgroundCleanup(ctx context.Context) {
	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cm.performCleanup()
		}
	}
}

// performCleanup 执行清理任务
func (cm *ContextManager) performCleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	expiredTime := now.Add(-cm.config.CacheTTL)

	// 清理过期的schema缓存
	for id, schema := range cm.schemaCache {
		if schema.LastUpdated.Before(expiredTime) {
			delete(cm.schemaCache, id)
		}
	}

	// 清理过期的用户历史
	for id, history := range cm.historyBuffer {
		if history.LastAccess.Before(expiredTime) {
			delete(cm.historyBuffer, id)
		}
	}

	// 清理过期的用户上下文
	for id, userCtx := range cm.userContextCache {
		if userCtx.LastActive.Before(expiredTime) {
			delete(cm.userContextCache, id)
		}
	}
}

// Close 关闭上下文管理器
func (cm *ContextManager) Close() error {
	if cm.cleanupCancel != nil {
		cm.cleanupCancel()
	}
	return nil
}

// GetStats 获取缓存统计信息
func (cm *ContextManager) GetStats() map[string]int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]int{
		"schema_cache_size":   len(cm.schemaCache),
		"history_buffer_size": len(cm.historyBuffer),
		"user_context_size":   len(cm.userContextCache),
	}
}