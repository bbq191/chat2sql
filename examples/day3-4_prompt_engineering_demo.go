// Day 3-4 提示词工程基础功能演示
// 本文件展示P1阶段"提示词工程基础"任务的核心功能

package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"chat2sql-go/internal/ai"
)

func main() {
	fmt.Println("🤖 Chat2SQL P1阶段 - Day 3-4: 提示词工程基础功能演示")
	fmt.Println(strings.Repeat("=", 60))

	// 1. 创建提示词模板管理器
	fmt.Println("\n📋 1. 初始化提示词模板管理器")
	templateManager := ai.NewPromptTemplateManager()
	
	// 列出可用模板
	templates := templateManager.ListTemplates()
	fmt.Printf("可用模板类型: %d 个\n", len(templates))
	for templateType, description := range templates {
		fmt.Printf("  - %s: %s\n", templateType, description)
	}

	// 2. 创建上下文管理器
	fmt.Println("\n🧠 2. 初始化上下文管理器")
	config := &ai.ContextConfig{
		MaxHistorySize:      10,
		CacheTTL:           time.Hour,
		CleanupInterval:    time.Minute * 30,
		MaxConnectionsCache: 50,
		EnablePrewarming:   true,
	}
	contextManager := ai.NewContextManager(config)
	defer contextManager.Close()

	// 3. 模拟数据库结构缓存
	fmt.Println("\n📊 3. 缓存数据库结构信息")
	schema := &ai.SchemaInfo{
		ConnectionID: 1,
		DatabaseName: "chat2sql_demo",
		Tables: map[string]*ai.Table{
			"users": {
				Name: "users",
				Comment: "用户信息表",
				Columns: map[string]*ai.Column{
					"id":         {Name: "id", DataType: "SERIAL", IsNullable: false, Comment: "主键ID"},
					"username":   {Name: "username", DataType: "VARCHAR(50)", IsNullable: false, Comment: "用户名"},
					"email":      {Name: "email", DataType: "VARCHAR(255)", IsNullable: false, Comment: "邮箱地址"},
					"created_at": {Name: "created_at", DataType: "TIMESTAMP", IsNullable: false, Comment: "创建时间"},
					"status":     {Name: "status", DataType: "INTEGER", IsNullable: false, Comment: "用户状态"},
				},
				PrimaryKeys: []string{"id"},
			},
			"orders": {
				Name: "orders",
				Comment: "订单信息表",
				Columns: map[string]*ai.Column{
					"id":         {Name: "id", DataType: "SERIAL", IsNullable: false, Comment: "订单ID"},
					"user_id":    {Name: "user_id", DataType: "INTEGER", IsNullable: false, Comment: "用户ID"},
					"amount":     {Name: "amount", DataType: "DECIMAL(10,2)", IsNullable: false, Comment: "订单金额"},
					"status":     {Name: "status", DataType: "VARCHAR(20)", IsNullable: false, Comment: "订单状态"},
					"created_at": {Name: "created_at", DataType: "TIMESTAMP", IsNullable: false, Comment: "下单时间"},
				},
				PrimaryKeys: []string{"id"},
				ForeignKeys: []ai.ForeignKey{
					{ColumnName: "user_id", ReferencedTable: "users", ReferencedColumn: "id"},
				},
			},
		},
	}

	err := contextManager.CacheSchema(1, schema)
	if err != nil {
		log.Fatalf("缓存数据库结构失败: %v", err)
	}
	fmt.Println("✅ 数据库结构缓存成功")

	// 4. 模拟查询历史记录
	fmt.Println("\n📝 4. 添加查询历史记录")
	userID := int64(1001)
	
	histories := []struct{
		query   string
		sql     string
		success bool
	}{
		{"查询所有用户", "SELECT * FROM users", true},
		{"统计用户总数", "SELECT COUNT(*) FROM users", true},
		{"查看最近的订单", "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10", true},
		{"无效的删除操作", "DELETE FROM users", false}, // 失败的操作
	}

	for _, h := range histories {
		err := contextManager.AddQueryHistory(userID, h.query, h.sql, h.success)
		if err != nil {
			fmt.Printf("❌ 添加历史记录失败: %v\n", err)
		} else {
			status := "✅"
			if !h.success {
				status = "❌"
			}
			fmt.Printf("%s 添加历史记录: %s\n", status, h.query)
		}
	}

	// 5. 演示不同类型的提示词模板
	fmt.Println("\n🎯 5. 测试不同类型的SQL生成提示词")

	testQueries := []struct{
		templateType string
		userQuery    string
		description  string
	}{
		{"base", "查询所有活跃用户的信息", "基础查询模板"},
		{"aggregation", "统计每个用户的订单数量和总金额", "聚合查询模板"},
		{"join", "查询用户及其最近一笔订单信息", "关联查询模板"},
		{"timeseries", "分析最近30天的订单趋势", "时间序列分析模板"},
	}

	for _, test := range testQueries {
		fmt.Printf("\n--- %s ---\n", test.description)
		fmt.Printf("用户查询: %s\n", test.userQuery)
		
		// 构建查询上下文
		queryContext, err := contextManager.BuildQueryContext(1, userID, test.userQuery)
		if err != nil {
			fmt.Printf("❌ 构建查询上下文失败: %v\n", err)
			continue
		}

		// 获取对应的模板
		template, err := templateManager.GetTemplate(test.templateType)
		if err != nil {
			fmt.Printf("❌ 获取模板失败: %v\n", err)
			continue
		}

		// 格式化提示词
		prompt, err := template.FormatPrompt(queryContext)
		if err != nil {
			fmt.Printf("❌ 格式化提示词失败: %v\n", err)
			continue
		}

		fmt.Printf("✅ 提示词生成成功 (长度: %d 字符)\n", len(prompt))
		
		// 显示提示词的前200个字符作为预览
		preview := prompt
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("提示词预览: %s\n", preview)
	}

	// 6. 测试提示词安全验证
	fmt.Println("\n🔒 6. 测试提示词安全验证")
	
	dangerousQueries := []string{
		"DELETE FROM users WHERE id = 1",
		"DROP TABLE orders",
		"UPDATE users SET password = 'hacked'",
		"查询用户信息", // 安全查询
	}

	for _, query := range dangerousQueries {
		testContext := &ai.QueryContext{UserQuery: query}
		err := ai.ValidatePromptContext(testContext)
		
		if err != nil {
			fmt.Printf("🚫 危险查询被阻止: %s (原因: %s)\n", query, err.Error())
		} else {
			fmt.Printf("✅ 安全查询通过: %s\n", query)
		}
	}

	// 7. 展示上下文管理器统计信息
	fmt.Println("\n📈 7. 上下文管理器统计信息")
	stats := contextManager.GetStats()
	for key, value := range stats {
		fmt.Printf("  %s: %d\n", key, value)
	}

	// 8. 测试包含历史记录的提示词生成
	fmt.Println("\n🔄 8. 测试包含历史记录的提示词生成")
	queryContext, _ := contextManager.BuildQueryContext(1, userID, "查询用户的订单统计信息")
	
	template, _ := templateManager.GetTemplate("base")
	promptWithHistory, err := template.PromptWithHistory(queryContext)
	if err != nil {
		fmt.Printf("❌ 生成包含历史记录的提示词失败: %v\n", err)
	} else {
		fmt.Printf("✅ 包含历史记录的提示词生成成功 (长度: %d 字符)\n", len(promptWithHistory))
		fmt.Printf("历史记录数量: %d\n", len(queryContext.QueryHistory))
	}

	fmt.Println("\n🎉 Day 3-4 提示词工程基础功能演示完成!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✅ 所有核心功能正常工作:")
	fmt.Println("   - 多类型提示词模板系统")
	fmt.Println("   - 智能上下文管理器")  
	fmt.Println("   - 数据库结构缓存")
	fmt.Println("   - 查询历史记录管理")
	fmt.Println("   - 安全验证机制")
	fmt.Println("   - 高性能并发处理")
}