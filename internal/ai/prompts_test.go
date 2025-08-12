package ai

import (
	"strings"
	"testing"
	"time"
)

func TestNewPromptTemplateManager(t *testing.T) {
	manager := NewPromptTemplateManager()
	
	if manager == nil {
		t.Fatal("NewPromptTemplateManager() 返回了 nil")
	}
	
	if manager.templates == nil {
		t.Fatal("templates map 未初始化")
	}
	
	// 验证默认模板是否注册
	expectedTemplates := []string{"base", "aggregation", "join", "timeseries"}
	for _, templateType := range expectedTemplates {
		if _, exists := manager.templates[templateType]; !exists {
			t.Errorf("默认模板 %s 未注册", templateType)
		}
	}
}

func TestPromptTemplateManager_RegisterTemplate(t *testing.T) {
	manager := NewPromptTemplateManager()
	
	testCases := []struct {
		name        string
		templateType string
		content     string
		description string
		expectError bool
	}{
		{
			name:        "注册自定义模板",
			templateType: "custom",
			content:     "这是一个测试模板：{{.UserQuery}}",
			description: "测试用模板",
			expectError: false,
		},
		{
			name:        "覆盖现有模板",
			templateType: "base",
			content:     "新的基础模板：{{.UserQuery}}",
			description: "新的基础模板",
			expectError: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.RegisterTemplate(tc.templateType, tc.content, tc.description)
			
			if tc.expectError && err == nil {
				t.Errorf("期望出现错误但没有")
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("不期望出现错误但出现了: %v", err)
			}
			
			// 验证模板是否正确注册
			if !tc.expectError {
				template, exists := manager.templates[tc.templateType]
				if !exists {
					t.Errorf("模板 %s 未正确注册", tc.templateType)
				}
				
				if template.description != tc.description {
					t.Errorf("模板描述不匹配，期望 %s，得到 %s", tc.description, template.description)
				}
			}
		})
	}
}

func TestPromptTemplateManager_GetTemplate(t *testing.T) {
	manager := NewPromptTemplateManager()
	
	testCases := []struct {
		name         string
		templateType string
		expectError  bool
	}{
		{
			name:         "获取存在的模板",
			templateType: "base",
			expectError:  false,
		},
		{
			name:         "获取不存在的模板",
			templateType: "nonexistent",
			expectError:  true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			template, err := manager.GetTemplate(tc.templateType)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("期望出现错误但没有")
				}
				if template != nil {
					t.Errorf("期望返回 nil 模板但返回了 %v", template)
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误但出现了: %v", err)
				}
				if template == nil {
					t.Errorf("期望返回模板但返回了 nil")
				}
			}
		})
	}
}

func TestPromptTemplateManager_ListTemplates(t *testing.T) {
	manager := NewPromptTemplateManager()
	
	templates := manager.ListTemplates()
	
	if len(templates) == 0 {
		t.Fatal("ListTemplates() 返回空列表")
	}
	
	expectedTypes := []string{"base", "aggregation", "join", "timeseries"}
	for _, expectedType := range expectedTypes {
		if _, exists := templates[expectedType]; !exists {
			t.Errorf("ListTemplates() 结果中缺少模板类型 %s", expectedType)
		}
	}
}

func TestSQLPromptTemplate_FormatPrompt(t *testing.T) {
	manager := NewPromptTemplateManager()
	template, err := manager.GetTemplate("base")
	if err != nil {
		t.Fatalf("获取基础模板失败: %v", err)
	}
	
	testCases := []struct {
		name        string
		context     *QueryContext
		expectError bool
		checkContains []string
	}{
		{
			name:        "空上下文",
			context:     nil,
			expectError: true,
		},
		{
			name: "正常格式化",
			context: &QueryContext{
				UserQuery:      "查询所有用户信息",
				DatabaseSchema: "users 表包含 id, name, email 字段",
				TableNames:     []string{"users"},
				QueryHistory:   []QueryHistory{},
				Timestamp:      time.Now(),
			},
			expectError: false,
			checkContains: []string{
				"查询所有用户信息",
				"users 表包含 id, name, email 字段",
			},
		},
		{
			name: "包含历史记录",
			context: &QueryContext{
				UserQuery:      "统计用户数量",
				DatabaseSchema: "users 表",
				QueryHistory: []QueryHistory{
					{
						Query:     "查询用户列表",
						SQL:       "SELECT * FROM users",
						Success:   true,
						Timestamp: time.Now().Add(-time.Hour),
					},
				},
				Timestamp: time.Now(),
			},
			expectError: false,
			checkContains: []string{
				"统计用户数量",
				"users 表",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := template.FormatPrompt(tc.context)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("期望出现错误但没有")
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误但出现了: %v", err)
				}
				
				// 检查结果是否包含期望的内容
				for _, expected := range tc.checkContains {
					if !strings.Contains(result, expected) {
						t.Errorf("格式化结果不包含期望内容 %s", expected)
					}
				}
			}
		})
	}
}

func TestSQLPromptTemplate_PromptWithHistory(t *testing.T) {
	manager := NewPromptTemplateManager()
	template, err := manager.GetTemplate("base")
	if err != nil {
		t.Fatalf("获取基础模板失败: %v", err)
	}
	
	// 无历史记录的情况
	ctx := &QueryContext{
		UserQuery:      "查询用户信息",
		DatabaseSchema: "users 表",
		QueryHistory:   []QueryHistory{},
		Timestamp:      time.Now(),
	}
	
	result, err := template.PromptWithHistory(ctx)
	if err != nil {
		t.Errorf("PromptWithHistory() 出现错误: %v", err)
	}
	
	if !strings.Contains(result, "查询用户信息") {
		t.Errorf("结果不包含用户查询内容")
	}
	
	// 包含历史记录的情况
	ctx.QueryHistory = []QueryHistory{
		{
			Query:     "获取所有部门信息",
			SQL:       "SELECT * FROM departments",
			Success:   true,
			Timestamp: time.Now().Add(-time.Hour),
		},
		{
			Query:     "统计员工数量",
			SQL:       "SELECT COUNT(*) FROM employees",
			Success:   true,
			Timestamp: time.Now().Add(-time.Minute * 30),
		},
		{
			Query:     "查询失败的记录",
			SQL:       "INVALID SQL",
			Success:   false,
			Timestamp: time.Now().Add(-time.Minute * 15),
		},
	}
	
	resultWithHistory, err := template.PromptWithHistory(ctx)
	if err != nil {
		t.Errorf("PromptWithHistory() with history 出现错误: %v", err)
	}
	
	// 验证只包含成功的历史记录
	if !strings.Contains(resultWithHistory, "SELECT * FROM departments") {
		t.Errorf("结果不包含成功的历史查询")
	}
	
	if !strings.Contains(resultWithHistory, "SELECT COUNT(*) FROM employees") {
		t.Errorf("结果不包含成功的历史查询")
	}
	
	if strings.Contains(resultWithHistory, "INVALID SQL") {
		t.Errorf("结果不应包含失败的历史查询")
	}
}

func TestValidatePromptContext(t *testing.T) {
	testCases := []struct {
		name        string
		context     *QueryContext
		expectError bool
		errorMsg    string
	}{
		{
			name:        "空上下文",
			context:     nil,
			expectError: true,
			errorMsg:    "查询上下文不能为空",
		},
		{
			name: "空查询",
			context: &QueryContext{
				UserQuery: "",
			},
			expectError: true,
			errorMsg:    "用户查询不能为空",
		},
		{
			name: "只有空格的查询",
			context: &QueryContext{
				UserQuery: "   ",
			},
			expectError: true,
			errorMsg:    "用户查询不能为空",
		},
		{
			name: "包含危险关键词",
			context: &QueryContext{
				UserQuery: "DELETE FROM users",
			},
			expectError: true,
			errorMsg:    "检测到可能的危险操作关键词",
		},
		{
			name: "正常查询",
			context: &QueryContext{
				UserQuery: "查询所有用户信息",
			},
			expectError: false,
		},
		{
			name: "包含SELECT的正常查询",
			context: &QueryContext{
				UserQuery: "我想要查询用户数据",
			},
			expectError: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePromptContext(tc.context)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("期望出现错误但没有")
				} else if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("错误信息不匹配，期望包含 %s，得到 %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误但出现了: %v", err)
				}
			}
		})
	}
}

func TestBuildSchemaDescription(t *testing.T) {
	manager := NewPromptTemplateManager()
	template, err := manager.GetTemplate("base")
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}
	
	testCases := []struct {
		name       string
		schema     string
		tableNames []string
		expected   []string
	}{
		{
			name:       "空结构信息",
			schema:     "",
			tableNames: []string{},
			expected:   []string{"数据库结构信息暂不可用"},
		},
		{
			name:       "只有表名",
			schema:     "",
			tableNames: []string{"users", "orders"},
			expected:   []string{"可用数据表", "users, orders"},
		},
		{
			name:       "完整结构信息",
			schema:     "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))",
			tableNames: []string{"users"},
			expected:   []string{"可用数据表", "详细表结构", "CREATE TABLE"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := template.buildSchemaDescription(tc.schema, tc.tableNames)
			
			for _, expected := range tc.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("结果不包含期望内容 %s，实际结果: %s", expected, result)
				}
			}
		})
	}
}

// 基准测试
func BenchmarkFormatPrompt(b *testing.B) {
	manager := NewPromptTemplateManager()
	template, err := manager.GetTemplate("base")
	if err != nil {
		b.Fatalf("获取模板失败: %v", err)
	}
	
	ctx := &QueryContext{
		UserQuery:      "查询所有用户信息",
		DatabaseSchema: "users 表包含 id, name, email 字段",
		TableNames:     []string{"users", "orders", "products"},
		QueryHistory: []QueryHistory{
			{Query: "test query", SQL: "SELECT 1", Success: true, Timestamp: time.Now()},
		},
		Timestamp: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := template.FormatPrompt(ctx)
		if err != nil {
			b.Fatalf("FormatPrompt() 失败: %v", err)
		}
	}
}

func BenchmarkValidatePromptContext(b *testing.B) {
	ctx := &QueryContext{
		UserQuery: "查询用户信息并统计数量",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePromptContext(ctx)
	}
}