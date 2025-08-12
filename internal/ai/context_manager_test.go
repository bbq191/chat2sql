package ai

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewContextManager(t *testing.T) {
	// 测试默认配置
	cm1 := NewContextManager(nil)
	if cm1 == nil {
		t.Fatal("NewContextManager() 返回了 nil")
	}
	
	if cm1.config == nil {
		t.Fatal("配置未初始化")
	}
	
	if cm1.config.MaxHistorySize != 50 {
		t.Errorf("期望默认历史记录大小为 50，得到 %d", cm1.config.MaxHistorySize)
	}
	
	// 测试自定义配置
	customConfig := &ContextConfig{
		MaxHistorySize:      100,
		CacheTTL:           time.Hour * 2,
		CleanupInterval:    time.Minute * 30,
		MaxConnectionsCache: 200,
		EnablePrewarming:   false,
	}
	
	cm2 := NewContextManager(customConfig)
	if cm2.config.MaxHistorySize != 100 {
		t.Errorf("期望历史记录大小为 100，得到 %d", cm2.config.MaxHistorySize)
	}
	
	// 清理资源
	cm1.Close()
	cm2.Close()
}

func TestContextManager_CacheSchema(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	// 创建测试用的 schema
	schema := &SchemaInfo{
		ConnectionID:  1,
		DatabaseName:  "testdb",
		Tables:        map[string]*Table{
			"users": {
				Name: "users",
				Columns: map[string]*Column{
					"id": {Name: "id", DataType: "INTEGER", IsNullable: false},
					"name": {Name: "name", DataType: "VARCHAR(100)", IsNullable: true},
				},
				PrimaryKeys: []string{"id"},
			},
		},
		LastUpdated:  time.Now(),
	}
	
	// 测试缓存 schema
	err := cm.CacheSchema(1, schema)
	if err != nil {
		t.Errorf("CacheSchema() 出现错误: %v", err)
	}
	
	// 验证 schema 被正确缓存
	cachedSchema, exists := cm.GetSchema(1)
	if !exists {
		t.Error("Schema 未被正确缓存")
	}
	
	if cachedSchema.ConnectionID != schema.ConnectionID {
		t.Errorf("缓存的 ConnectionID 不匹配，期望 %d，得到 %d", 
			schema.ConnectionID, cachedSchema.ConnectionID)
	}
	
	if cachedSchema.DatabaseName != schema.DatabaseName {
		t.Errorf("缓存的 DatabaseName 不匹配，期望 %s，得到 %s", 
			schema.DatabaseName, cachedSchema.DatabaseName)
	}
}

func TestContextManager_GetSchema(t *testing.T) {
	cm := NewContextManager(&ContextConfig{
		CacheTTL: time.Millisecond * 100, // 短 TTL 用于测试过期
	})
	defer cm.Close()
	
	schema := &SchemaInfo{
		ConnectionID: 1,
		DatabaseName: "testdb",
		LastUpdated:  time.Now(),
	}
	
	// 缓存 schema
	err := cm.CacheSchema(1, schema)
	if err != nil {
		t.Fatalf("CacheSchema() 失败: %v", err)
	}
	
	// 测试获取存在的 schema
	cachedSchema, exists := cm.GetSchema(1)
	if !exists {
		t.Error("GetSchema() 应该返回 true for existing schema")
	}
	if cachedSchema == nil {
		t.Error("GetSchema() 应该返回非 nil schema")
	}
	
	// 测试获取不存在的 schema
	_, exists = cm.GetSchema(999)
	if exists {
		t.Error("GetSchema() 应该返回 false for non-existing schema")
	}
	
	// 等待 TTL 过期
	time.Sleep(time.Millisecond * 150)
	
	// 测试过期的 schema
	_, exists = cm.GetSchema(1)
	if exists {
		t.Error("GetSchema() 应该返回 false for expired schema")
	}
}

func TestContextManager_AddQueryHistory(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	userID := int64(123)
	
	// 添加第一条历史记录
	err := cm.AddQueryHistory(userID, "查询用户", "SELECT * FROM users", true)
	if err != nil {
		t.Errorf("AddQueryHistory() 出现错误: %v", err)
	}
	
	// 验证历史记录被添加
	history := cm.GetRecentHistory(userID, 10)
	if len(history) != 1 {
		t.Errorf("期望历史记录数量为 1，得到 %d", len(history))
	}
	
	if history[0].Query != "查询用户" {
		t.Errorf("历史记录查询不匹配，期望 '查询用户'，得到 '%s'", history[0].Query)
	}
	
	if history[0].SQL != "SELECT * FROM users" {
		t.Errorf("历史记录 SQL 不匹配，期望 'SELECT * FROM users'，得到 '%s'", history[0].SQL)
	}
	
	if !history[0].Success {
		t.Error("历史记录成功状态应该为 true")
	}
	
	// 添加更多历史记录
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("查询 %d", i)
		sql := fmt.Sprintf("SELECT %d", i)
		err := cm.AddQueryHistory(userID, query, sql, true)
		if err != nil {
			t.Errorf("AddQueryHistory() 第 %d 次调用出现错误: %v", i, err)
		}
	}
	
	// 验证历史记录数量
	history = cm.GetRecentHistory(userID, 10)
	if len(history) != 6 {
		t.Errorf("期望历史记录数量为 6，得到 %d", len(history))
	}
	
	// 验证历史记录顺序（最新的在前）
	if !strings.Contains(history[0].Query, "查询 4") {
		t.Errorf("最新的历史记录不正确，得到 '%s'", history[0].Query)
	}
}

func TestContextManager_GetRecentHistory(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	userID := int64(456)
	
	// 添加成功和失败的记录
	testData := []struct {
		query   string
		sql     string
		success bool
	}{
		{"查询1", "SELECT 1", true},
		{"查询2", "INVALID SQL", false},
		{"查询3", "SELECT 3", true},
		{"查询4", "DELETE FROM test", false},
		{"查询5", "SELECT 5", true},
	}
	
	for _, data := range testData {
		err := cm.AddQueryHistory(userID, data.query, data.sql, data.success)
		if err != nil {
			t.Errorf("添加历史记录失败: %v", err)
		}
	}
	
	// 获取最近的成功记录
	history := cm.GetRecentHistory(userID, 10)
	
	// 应该只返回成功的记录
	expectedSuccessCount := 3
	if len(history) != expectedSuccessCount {
		t.Errorf("期望成功记录数量为 %d，得到 %d", expectedSuccessCount, len(history))
	}
	
	// 验证都是成功的记录
	for i, record := range history {
		if !record.Success {
			t.Errorf("第 %d 条记录应该是成功的", i)
		}
	}
	
	// 验证按时间倒序排列
	if len(history) >= 2 {
		if history[0].Timestamp.Before(history[1].Timestamp) {
			t.Error("历史记录应该按时间倒序排列")
		}
	}
	
	// 测试限制数量
	limitedHistory := cm.GetRecentHistory(userID, 2)
	if len(limitedHistory) != 2 {
		t.Errorf("期望限制后的记录数量为 2，得到 %d", len(limitedHistory))
	}
}

func TestContextManager_BuildQueryContext(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	connectionID := int64(1)
	userID := int64(789)
	
	// 缓存一些测试数据
	schema := &SchemaInfo{
		ConnectionID: connectionID,
		DatabaseName: "testdb",
		Tables: map[string]*Table{
			"users": {
				Name: "users",
				Columns: map[string]*Column{
					"id":   {Name: "id", DataType: "INTEGER"},
					"name": {Name: "name", DataType: "VARCHAR(100)"},
				},
			},
		},
	}
	cm.CacheSchema(connectionID, schema)
	
	// 添加一些历史记录
	cm.AddQueryHistory(userID, "查询用户", "SELECT * FROM users", true)
	cm.AddQueryHistory(userID, "统计用户", "SELECT COUNT(*) FROM users", true)
	
	// 测试构建查询上下文
	ctx, err := cm.BuildQueryContext(connectionID, userID, "查询所有活跃用户")
	if err != nil {
		t.Errorf("BuildQueryContext() 出现错误: %v", err)
	}
	
	if ctx == nil {
		t.Fatal("BuildQueryContext() 返回了 nil")
	}
	
	// 验证上下文内容
	if ctx.UserQuery != "查询所有活跃用户" {
		t.Errorf("用户查询不匹配，期望 '查询所有活跃用户'，得到 '%s'", ctx.UserQuery)
	}
	
	if !strings.Contains(ctx.DatabaseSchema, "testdb") {
		t.Error("数据库结构应该包含数据库名称")
	}
	
	if len(ctx.TableNames) == 0 {
		t.Error("表名列表应该不为空")
	}
	
	if len(ctx.QueryHistory) == 0 {
		t.Error("查询历史应该不为空")
	}
	
	// 测试危险查询
	_, err = cm.BuildQueryContext(connectionID, userID, "DELETE FROM users")
	if err == nil {
		t.Error("BuildQueryContext() 应该拒绝危险查询")
	}
	
	// 测试空查询
	_, err = cm.BuildQueryContext(connectionID, userID, "")
	if err == nil {
		t.Error("BuildQueryContext() 应该拒绝空查询")
	}
}

func TestContextManager_FormatSchemaForPrompt(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	schema := &SchemaInfo{
		DatabaseName: "testdb",
		Tables: map[string]*Table{
			"users": {
				Name:        "users",
				Comment:     "用户表",
				PrimaryKeys: []string{"id"},
				Columns: map[string]*Column{
					"id": {
						Name:       "id",
						DataType:   "INTEGER",
						IsNullable: false,
						Comment:    "主键ID",
					},
					"name": {
						Name:       "name",
						DataType:   "VARCHAR(100)",
						IsNullable: true,
						Comment:    "用户姓名",
					},
					"email": {
						Name:       "email",
						DataType:   "VARCHAR(255)",
						IsNullable: false,
						Comment:    "邮箱地址",
					},
				},
				ForeignKeys: []ForeignKey{
					{
						ColumnName:       "department_id",
						ReferencedTable:  "departments",
						ReferencedColumn: "id",
					},
				},
			},
		},
	}
	
	result := cm.formatSchemaForPrompt(schema)
	
	// 验证格式化结果包含期望的内容
	expectedContents := []string{
		"数据库: testdb",
		"表: users",
		"用户表",
		"id: INTEGER NOT NULL",
		"name: VARCHAR(100)",
		"email: VARCHAR(255) NOT NULL", 
		"主键: [id]",
		"外键",
		"department_id -> departments.id",
	}
	
	for _, expected := range expectedContents {
		if !strings.Contains(result, expected) {
			t.Errorf("格式化结果应该包含 '%s'，但实际结果为：\n%s", expected, result)
		}
	}
	
	// 测试空 schema
	emptyResult := cm.formatSchemaForPrompt(nil)
	if emptyResult != "" {
		t.Errorf("空 schema 应该返回空字符串，得到 '%s'", emptyResult)
	}
}

func TestContextManager_ExtractTableNames(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	schema := &SchemaInfo{
		Tables: map[string]*Table{
			"users":       {Name: "users"},
			"orders":      {Name: "orders"},
			"products":    {Name: "products"},
			"categories":  {Name: "categories"},
		},
	}
	
	tableNames := cm.extractTableNames(schema)
	
	if len(tableNames) != 4 {
		t.Errorf("期望表名数量为 4，得到 %d", len(tableNames))
	}
	
	expectedTables := map[string]bool{
		"users": false, "orders": false, "products": false, "categories": false,
	}
	
	for _, tableName := range tableNames {
		if _, exists := expectedTables[tableName]; exists {
			expectedTables[tableName] = true
		} else {
			t.Errorf("意外的表名: %s", tableName)
		}
	}
	
	// 验证所有期望的表名都被找到
	for tableName, found := range expectedTables {
		if !found {
			t.Errorf("表名 %s 未被找到", tableName)
		}
	}
	
	// 测试空 schema
	emptyNames := cm.extractTableNames(nil)
	if len(emptyNames) != 0 {
		t.Errorf("空 schema 应该返回空列表，得到 %v", emptyNames)
	}
}

func TestContextManager_GetStats(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	// 初始状态
	stats := cm.GetStats()
	if stats["schema_cache_size"] != 0 {
		t.Errorf("初始 schema cache 大小应该为 0，得到 %d", stats["schema_cache_size"])
	}
	
	// 添加一些数据
	schema := &SchemaInfo{ConnectionID: 1, DatabaseName: "test"}
	cm.CacheSchema(1, schema)
	cm.AddQueryHistory(123, "test query", "SELECT 1", true)
	
	// 检查统计信息
	stats = cm.GetStats()
	if stats["schema_cache_size"] != 1 {
		t.Errorf("schema cache 大小应该为 1，得到 %d", stats["schema_cache_size"])
	}
	
	if stats["history_buffer_size"] != 1 {
		t.Errorf("history buffer 大小应该为 1，得到 %d", stats["history_buffer_size"])
	}
}

func TestContextManager_ConcurrentAccess(t *testing.T) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	const numGoroutines = 10
	const numOperations = 100
	
	var wg sync.WaitGroup
	
	// 并发添加历史记录
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			userID := int64(routineID)
			
			for j := 0; j < numOperations; j++ {
				query := fmt.Sprintf("查询 %d-%d", routineID, j)
				sql := fmt.Sprintf("SELECT %d", j)
				err := cm.AddQueryHistory(userID, query, sql, true)
				if err != nil {
					t.Errorf("并发添加历史记录失败: %v", err)
				}
			}
		}(i)
	}
	
	// 并发缓存 schema
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			connectionID := int64(routineID)
			
			schema := &SchemaInfo{
				ConnectionID: connectionID,
				DatabaseName: fmt.Sprintf("db%d", routineID),
			}
			
			err := cm.CacheSchema(connectionID, schema)
			if err != nil {
				t.Errorf("并发缓存 schema 失败: %v", err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// 验证数据完整性
	stats := cm.GetStats()
	if stats["schema_cache_size"] != numGoroutines {
		t.Errorf("期望 schema cache 大小为 %d，得到 %d", 
			numGoroutines, stats["schema_cache_size"])
	}
	
	if stats["history_buffer_size"] != numGoroutines {
		t.Errorf("期望 history buffer 大小为 %d，得到 %d", 
			numGoroutines, stats["history_buffer_size"])
	}
}

func TestContextManager_HistorySizeLimit(t *testing.T) {
	config := &ContextConfig{
		MaxHistorySize: 5, // 设置较小的历史记录限制
	}
	cm := NewContextManager(config)
	defer cm.Close()
	
	userID := int64(999)
	
	// 添加超过限制的历史记录
	for i := 0; i < 10; i++ {
		query := fmt.Sprintf("查询 %d", i)
		sql := fmt.Sprintf("SELECT %d", i)
		err := cm.AddQueryHistory(userID, query, sql, true)
		if err != nil {
			t.Errorf("添加历史记录失败: %v", err)
		}
	}
	
	// 验证历史记录数量被限制
	history := cm.GetRecentHistory(userID, 20)
	if len(history) != 5 {
		t.Errorf("期望历史记录数量为 5，得到 %d", len(history))
	}
	
	// 验证保留的是最新的记录
	if !strings.Contains(history[0].Query, "查询 9") {
		t.Errorf("应该保留最新的记录，得到 '%s'", history[0].Query)
	}
}

// 基准测试
func BenchmarkContextManager_AddQueryHistory(b *testing.B) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	userID := int64(1)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := fmt.Sprintf("查询 %d", i)
		sql := fmt.Sprintf("SELECT %d", i)
		cm.AddQueryHistory(userID, query, sql, true)
	}
}

func BenchmarkContextManager_GetRecentHistory(b *testing.B) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	userID := int64(1)
	
	// 预填充一些历史记录
	for i := 0; i < 100; i++ {
		query := fmt.Sprintf("查询 %d", i)
		sql := fmt.Sprintf("SELECT %d", i)
		cm.AddQueryHistory(userID, query, sql, true)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.GetRecentHistory(userID, 10)
	}
}

func BenchmarkContextManager_BuildQueryContext(b *testing.B) {
	cm := NewContextManager(nil)
	defer cm.Close()
	
	// 准备测试数据
	connectionID := int64(1)
	userID := int64(1)
	
	schema := &SchemaInfo{
		ConnectionID: connectionID,
		DatabaseName: "testdb",
		Tables: map[string]*Table{
			"users": {
				Name: "users",
				Columns: map[string]*Column{
					"id":   {Name: "id", DataType: "INTEGER"},
					"name": {Name: "name", DataType: "VARCHAR(100)"},
				},
			},
		},
	}
	cm.CacheSchema(connectionID, schema)
	
	cm.AddQueryHistory(userID, "查询用户", "SELECT * FROM users", true)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cm.BuildQueryContext(connectionID, userID, "查询所有用户")
		if err != nil {
			b.Fatalf("BuildQueryContext() 失败: %v", err)
		}
	}
}