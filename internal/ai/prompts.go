// Package ai 提供AI相关的核心功能
package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/prompts"
)

// SQLPromptTemplate SQL生成提示词模板
type SQLPromptTemplate struct {
	template     *prompts.PromptTemplate
	templateType string
	description  string
}

// PromptTemplateManager 提示词模板管理器
type PromptTemplateManager struct {
	templates map[string]*SQLPromptTemplate
}

// QueryContext 查询上下文信息
type QueryContext struct {
	UserQuery      string            `json:"user_query"`
	DatabaseSchema string            `json:"database_schema"`
	TableNames     []string          `json:"table_names"`
	QueryHistory   []QueryHistory    `json:"query_history"`
	UserContext    map[string]string `json:"user_context"`
	Timestamp      time.Time         `json:"timestamp"`
}

// QueryHistory 查询历史记录
type QueryHistory struct {
	Query     string    `json:"query"`
	SQL       string    `json:"sql"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

// 基础SQL生成提示词模板
const BaseSQLGenerationPrompt = `你是一个专业的SQL查询生成专家，擅长将自然语言转换为准确的PostgreSQL查询语句。

## 🎯 任务目标
根据用户的自然语言查询需求，生成准确、安全、高效的PostgreSQL 17查询语句。

## 📊 数据库结构信息
{{.DatabaseSchema}}

## 📝 用户查询
{{.UserQuery}}

## 📋 安全规则（必须严格遵守）
1. ✅ **只允许SELECT查询**：禁止任何DELETE、UPDATE、INSERT、DROP、CREATE、ALTER、TRUNCATE操作
2. ✅ **字段名匹配**：所有字段名必须与数据库结构完全匹配，区分大小写
3. ✅ **表名验证**：只能查询已提供的表，不得臆造表名
4. ✅ **SQL注入防护**：避免动态拼接，使用参数化查询思维
5. ✅ **性能考虑**：避免全表扫描，优先使用索引字段

## 🔧 技术规范
- **数据库方言**：PostgreSQL 17语法
- **字符串匹配**：使用ILIKE进行不区分大小写匹配
- **日期处理**：使用PostgreSQL日期函数
- **聚合查询**：正确使用GROUP BY和聚合函数
- **关联查询**：使用适当的JOIN类型

## 📤 输出要求
- **格式**：返回纯净的SQL语句，不包含任何解释文字
- **语法**：符合PostgreSQL 17标准
- **注释**：SQL中可包含必要的行内注释
- **格式化**：保持良好的SQL格式化风格

## 🤔 处理策略
- 如果查询意图不明确，选择最合理的解释
- 如果涉及多表查询，优先使用INNER JOIN
- 如果需要模糊匹配，使用ILIKE操作符
- 如果涉及日期范围，使用BETWEEN或日期函数

生成SQL：`

// 聚合查询专用提示词模板
const AggregationSQLPrompt = `你是SQL聚合查询专家，专门处理统计分析类查询。

## 📊 数据库结构
{{.DatabaseSchema}}

## 📝 用户统计需求
{{.UserQuery}}

## 📈 聚合查询指南
- **COUNT()**: 统计记录数量，使用COUNT(*) 或 COUNT(DISTINCT column)
- **SUM()**: 数值求和，确保字段为数值类型
- **AVG()**: 平均值计算，处理NULL值
- **MAX()/MIN()**: 最大最小值，支持日期和数值
- **GROUP BY**: 分组规则，所有非聚合字段必须在GROUP BY中
- **HAVING**: 聚合结果过滤，区别于WHERE条件

## 🎯 常见统计模式
1. **按时间统计**: DATE_TRUNC('month', created_at) 按月统计
2. **排行榜查询**: ORDER BY count DESC LIMIT 10
3. **占比分析**: 使用子查询计算百分比
4. **多维度分组**: 多字段GROUP BY分析

生成聚合SQL：`

// 关联查询专用提示词模板  
const JoinSQLPrompt = `你是SQL关联查询专家，专门处理多表查询需求。

## 📊 数据库结构与关系
{{.DatabaseSchema}}

## 📝 用户关联查询需求
{{.UserQuery}}

## 🔗 JOIN类型选择指南
- **INNER JOIN**: 只返回两表都有匹配的记录（默认选择）
- **LEFT JOIN**: 返回左表所有记录，右表无匹配时为NULL
- **RIGHT JOIN**: 返回右表所有记录，左表无匹配时为NULL
- **FULL OUTER JOIN**: 返回两表所有记录，无匹配时为NULL

## ⚡ 性能优化建议
1. **JOIN顺序**: 小表在前，大表在后
2. **索引利用**: 优先使用主键和外键关联
3. **条件推入**: WHERE条件尽量推入到JOIN之前
4. **字段选择**: 只SELECT需要的字段，避免SELECT *

## 🎯 关联查询模式
- **一对多查询**: 主表LEFT JOIN明细表
- **多表串联**: A JOIN B JOIN C 的链式关联
- **自关联查询**: 表自己关联自己（如组织架构树）
- **条件关联**: JOIN ON中包含复合条件

生成关联SQL：`

// 时间序列查询专用提示词模板
const TimeSeriesSQLPrompt = `你是时间序列分析SQL专家，专门处理时间相关的查询分析。

## 📊 数据库结构
{{.DatabaseSchema}}

## ⏰ 用户时间查询需求
{{.UserQuery}}

## 📅 时间处理函数指南
- **DATE_TRUNC()**: 时间截断（'year', 'month', 'week', 'day', 'hour'）
- **EXTRACT()**: 提取时间部分（year, month, day, dow, hour）
- **AGE()**: 计算时间间隔
- **NOW()**, **CURRENT_DATE**: 当前时间函数
- **INTERVAL**: 时间间隔计算，如 INTERVAL '7 days'

## 📈 时间分析模式
1. **趋势分析**: 按时间维度分组统计
2. **同比环比**: 使用LAG()窗口函数对比
3. **时间范围过滤**: BETWEEN, >= NOW() - INTERVAL 
4. **工作日/周末**: EXTRACT(dow FROM date) 判断星期
5. **月初月末**: DATE_TRUNC() + INTERVAL组合

## 🎯 常用时间查询模式
- **最近N天**: WHERE created_at >= NOW() - INTERVAL '30 days'
- **按月统计**: GROUP BY DATE_TRUNC('month', created_at)
- **工作时间过滤**: WHERE EXTRACT(dow FROM created_at) BETWEEN 1 AND 5
- **时间段对比**: 使用CASE WHEN或窗口函数

生成时间SQL：`

// NewPromptTemplateManager 创建提示词模板管理器
func NewPromptTemplateManager() *PromptTemplateManager {
	manager := &PromptTemplateManager{
		templates: make(map[string]*SQLPromptTemplate),
	}

	// 注册基础模板
	manager.RegisterTemplate("base", BaseSQLGenerationPrompt, "基础SQL生成模板")
	manager.RegisterTemplate("aggregation", AggregationSQLPrompt, "聚合查询专用模板")
	manager.RegisterTemplate("join", JoinSQLPrompt, "关联查询专用模板") 
	manager.RegisterTemplate("timeseries", TimeSeriesSQLPrompt, "时间序列分析模板")

	return manager
}

// RegisterTemplate 注册提示词模板
func (ptm *PromptTemplateManager) RegisterTemplate(name, templateContent, description string) error {
	template := prompts.NewPromptTemplate(
		templateContent,
		[]string{"DatabaseSchema", "UserQuery"},
	)

	ptm.templates[name] = &SQLPromptTemplate{
		template:     &template,
		templateType: name,
		description:  description,
	}

	return nil
}

// GetTemplate 获取指定类型的模板
func (ptm *PromptTemplateManager) GetTemplate(templateType string) (*SQLPromptTemplate, error) {
	template, exists := ptm.templates[templateType]
	if !exists {
		return nil, fmt.Errorf("模板类型不存在: %s", templateType)
	}
	return template, nil
}

// ListTemplates 列出所有可用模板
func (ptm *PromptTemplateManager) ListTemplates() map[string]string {
	result := make(map[string]string)
	for name, template := range ptm.templates {
		result[name] = template.description
	}
	return result
}

// FormatPrompt 格式化提示词
func (st *SQLPromptTemplate) FormatPrompt(ctx *QueryContext) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("查询上下文不能为空")
	}

	// 构建数据库结构描述
	schemaDescription := st.buildSchemaDescription(ctx.DatabaseSchema, ctx.TableNames)
	
	// 格式化提示词
	prompt, err := st.template.Format(map[string]any{
		"DatabaseSchema": schemaDescription,
		"UserQuery":      ctx.UserQuery,
	})
	if err != nil {
		return "", fmt.Errorf("提示词格式化失败: %w", err)
	}

	return prompt, nil
}

// buildSchemaDescription 构建数据库结构描述
func (st *SQLPromptTemplate) buildSchemaDescription(schema string, tableNames []string) string {
	if schema == "" && len(tableNames) == 0 {
		return "数据库结构信息暂不可用，请根据常见数据库设计模式生成查询。"
	}

	var builder strings.Builder
	builder.WriteString("📋 可用数据表：\n")
	
	if len(tableNames) > 0 {
		builder.WriteString("表名列表：")
		builder.WriteString(strings.Join(tableNames, ", "))
		builder.WriteString("\n\n")
	}

	if schema != "" {
		builder.WriteString("详细表结构：\n")
		builder.WriteString(schema)
		builder.WriteString("\n")
	}

	return builder.String()
}

// PromptWithHistory 包含历史记录的提示词格式化
func (st *SQLPromptTemplate) PromptWithHistory(ctx *QueryContext) (string, error) {
	basePrompt, err := st.FormatPrompt(ctx)
	if err != nil {
		return "", err
	}

	if len(ctx.QueryHistory) == 0 {
		return basePrompt, nil
	}

	var historyBuilder strings.Builder
	historyBuilder.WriteString("\n## 📚 相关查询历史（供参考）\n")
	
	// 只取最近3条成功的查询记录
	successCount := 0
	for i := len(ctx.QueryHistory) - 1; i >= 0 && successCount < 3; i-- {
		history := ctx.QueryHistory[i]
		if history.Success {
			historyBuilder.WriteString(fmt.Sprintf("查询: %s\n", history.Query))
			historyBuilder.WriteString(fmt.Sprintf("SQL: %s\n\n", history.SQL))
			successCount++
		}
	}

	return basePrompt + historyBuilder.String(), nil
}

// ValidatePromptContext 验证提示词上下文
func ValidatePromptContext(ctx *QueryContext) error {
	if ctx == nil {
		return fmt.Errorf("查询上下文不能为空")
	}

	if strings.TrimSpace(ctx.UserQuery) == "" {
		return fmt.Errorf("用户查询不能为空")
	}

	// 安全检查：检测危险关键词
	dangerousKeywords := []string{
		"DELETE", "UPDATE", "INSERT", "DROP", "CREATE", "ALTER", 
		"TRUNCATE", "EXEC", "EXECUTE", "UNION", "--", "/*", "*/",
	}

	upperQuery := strings.ToUpper(ctx.UserQuery)
	for _, keyword := range dangerousKeywords {
		if strings.Contains(upperQuery, keyword) {
			return fmt.Errorf("检测到可能的危险操作关键词: %s", keyword)
		}
	}

	return nil
}