package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"go.uber.org/zap"
)

// SQLSecurityValidator SQL安全验证器
// 提供全面的SQL语句安全检查，防止SQL注入和危险操作
type SQLSecurityValidator struct {
	logger *zap.Logger
	
	// 配置参数
	allowedStatements []string    // 允许的SQL语句类型
	forbiddenKeywords []string    // 禁止的关键词
	maxStatements     int         // 最大语句数量
	maxQueryLength    int         // 最大查询长度
	
	// 编译后的正则表达式（提高性能）
	sqlInjectionPatterns []*regexp.Regexp
	commentPatterns      []*regexp.Regexp
}

// SQLSecurityConfig SQL安全验证配置
type SQLSecurityConfig struct {
	AllowedStatements []string `json:"allowed_statements"` // 允许的SQL语句类型
	ForbiddenKeywords []string `json:"forbidden_keywords"` // 额外禁止的关键词
	MaxStatements     int      `json:"max_statements"`     // 最大语句数量，默认1
	MaxQueryLength    int      `json:"max_query_length"`   // 最大查询长度，默认10000
	StrictMode        bool     `json:"strict_mode"`        // 严格模式
}

// ValidationResult SQL验证结果
type ValidationResult struct {
	IsValid     bool     `json:"is_valid"`     // 是否通过验证
	QueryType   string   `json:"query_type"`   // 查询类型
	IsReadOnly  bool     `json:"is_read_only"` // 是否只读查询
	Errors      []string `json:"errors"`       // 错误信息
	Warnings    []string `json:"warnings"`     // 警告信息
	TablesUsed  []string `json:"tables_used"`  // 使用的表
	Risk        string   `json:"risk"`         // 风险等级: LOW/MEDIUM/HIGH
}

// SecurityViolation 安全违规错误
type SecurityViolation struct {
	Type    string `json:"type"`    // 违规类型
	Message string `json:"message"` // 错误信息
	Context string `json:"context"` // 上下文信息
}

func (sv SecurityViolation) Error() string {
	return fmt.Sprintf("SQL安全违规 [%s]: %s", sv.Type, sv.Message)
}

// NewSQLSecurityValidator 创建SQL安全验证器
func NewSQLSecurityValidator(logger *zap.Logger) *SQLSecurityValidator {
	// 默认配置
	config := &SQLSecurityConfig{
		AllowedStatements: []string{"SELECT", "WITH", "EXPLAIN", "SHOW"},
		ForbiddenKeywords: []string{},
		MaxStatements:     1,
		MaxQueryLength:    10000,
		StrictMode:        true,
	}
	
	return NewSQLSecurityValidatorWithConfig(config, logger)
}

// NewSQLSecurityValidatorWithConfig 使用自定义配置创建SQL安全验证器
func NewSQLSecurityValidatorWithConfig(config *SQLSecurityConfig, logger *zap.Logger) *SQLSecurityValidator {
	if config == nil {
		return NewSQLSecurityValidator(logger)
	}
	
	// 设置默认值
	if len(config.AllowedStatements) == 0 {
		config.AllowedStatements = []string{"SELECT", "WITH", "EXPLAIN", "SHOW"}
	}
	if config.MaxStatements <= 0 {
		config.MaxStatements = 1
	}
	if config.MaxQueryLength <= 0 {
		config.MaxQueryLength = 10000
	}
	
	validator := &SQLSecurityValidator{
		logger:            logger,
		allowedStatements: config.AllowedStatements,
		forbiddenKeywords: config.ForbiddenKeywords,
		maxStatements:     config.MaxStatements,
		maxQueryLength:    config.MaxQueryLength,
	}
	
	// 初始化SQL注入检测模式
	validator.initializePatterns()
	
	// 添加基础禁止关键词
	validator.addDefaultForbiddenKeywords(config.StrictMode)
	
	return validator
}

// initializePatterns 初始化SQL注入检测模式
func (v *SQLSecurityValidator) initializePatterns() {
	// SQL注入攻击模式
	sqlInjectionPatterns := []string{
		// Union查询注入
		`(?i)\bunion\s+select\b`,
		`(?i)\bunion\s+all\s+select\b`,
		
		// 注释注入
		`--[^\r\n]*`,
		`/\*.*?\*/`,
		`#[^\r\n]*`,
		
		// 字符串逃逸
		`'[^']*'[^']*'`,
		`"[^"]*"[^"]*"`,
		
		// 时间延迟注入
		`(?i)\bwaitfor\s+delay\b`,
		`(?i)\bsleep\s*\(`,
		`(?i)\bbenchmark\s*\(`,
		
		// 布尔盲注
		`(?i)\band\s+1\s*=\s*1\b`,
		`(?i)\bor\s+1\s*=\s*1\b`,
		`(?i)\band\s+1\s*=\s*2\b`,
		
		// 堆叠查询
		`;\s*\w+`,
		
		// 函数调用注入
		`(?i)\bchar\s*\(`,
		`(?i)\bord\s*\(`,
		`(?i)\bhex\s*\(`,
		
		// 系统函数
		`(?i)\bload_file\s*\(`,
		`(?i)\binto\s+outfile\b`,
		`(?i)\binto\s+dumpfile\b`,
	}
	
	// 编译正则表达式
	v.sqlInjectionPatterns = make([]*regexp.Regexp, 0, len(sqlInjectionPatterns))
	for _, pattern := range sqlInjectionPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			v.sqlInjectionPatterns = append(v.sqlInjectionPatterns, re)
		} else {
			v.logger.Warn("无法编译SQL注入检测模式", 
				zap.String("pattern", pattern), 
				zap.Error(err))
		}
	}
	
	// 注释模式
	commentPatterns := []string{
		`--[^\r\n]*`,           // 单行注释 --
		`#[^\r\n]*`,            // MySQL注释 #
		`/\*[\s\S]*?\*/`,       // 多行注释 /* */
	}
	
	v.commentPatterns = make([]*regexp.Regexp, 0, len(commentPatterns))
	for _, pattern := range commentPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			v.commentPatterns = append(v.commentPatterns, re)
		}
	}
}

// addDefaultForbiddenKeywords 添加默认禁止关键词
func (v *SQLSecurityValidator) addDefaultForbiddenKeywords(strictMode bool) {
	// 基础禁止关键词
	basicForbidden := []string{
		"DROP", "DELETE", "INSERT", "UPDATE", "CREATE", "ALTER", 
		"TRUNCATE", "GRANT", "REVOKE", "REPLACE", "MERGE",
		"CALL", "EXEC", "EXECUTE", "DECLARE", "SET",
	}
	
	if strictMode {
		// 严格模式下的额外禁止关键词
		strictForbidden := []string{
			"LOAD", "OUTFILE", "DUMPFILE", "INFILE",
			"HANDLER", "PREPARE", "DEALLOCATE",
			"XA", "LOCK", "UNLOCK", "FLUSH", "RESET",
			"SHUTDOWN", "RESTART", "KILL",
		}
		basicForbidden = append(basicForbidden, strictForbidden...)
	}
	
	// 合并用户定义的禁止关键词
	allForbidden := make(map[string]bool)
	for _, keyword := range basicForbidden {
		allForbidden[strings.ToUpper(keyword)] = true
	}
	for _, keyword := range v.forbiddenKeywords {
		allForbidden[strings.ToUpper(keyword)] = true
	}
	
	// 转换为切片
	v.forbiddenKeywords = make([]string, 0, len(allForbidden))
	for keyword := range allForbidden {
		v.forbiddenKeywords = append(v.forbiddenKeywords, keyword)
	}
}

// ValidateSQL 验证SQL查询的安全性
func (v *SQLSecurityValidator) ValidateSQL(sql string) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Errors:    []string{},
		Warnings:  []string{},
		TablesUsed: []string{},
		Risk:      "LOW",
	}
	
	if sql == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "SQL查询不能为空")
		return result
	}
	
	// 1. 长度检查
	if len(sql) > v.maxQueryLength {
		result.IsValid = false
		result.Errors = append(result.Errors, 
			fmt.Sprintf("SQL查询长度超过限制(%d字符)", v.maxQueryLength))
		result.Risk = "HIGH"
		return result
	}
	
	// 2. 语句数量检查
	if err := v.checkStatementCount(sql); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
		result.Risk = "HIGH"
	}
	
	// 3. 清理和预处理SQL
	cleanedSQL := v.cleanSQL(sql)
	
	// 4. 检测查询类型
	result.QueryType = v.detectQueryType(cleanedSQL)
	result.IsReadOnly = v.isReadOnlyQuery(result.QueryType)
	
	// 5. 检查允许的语句类型
	if !v.isAllowedStatement(result.QueryType) {
		result.IsValid = false
		result.Errors = append(result.Errors, 
			fmt.Sprintf("不支持的SQL语句类型: %s", result.QueryType))
		result.Risk = "HIGH"
	}
	
	// 6. 禁止关键词检查
	if violations := v.checkForbiddenKeywords(cleanedSQL); len(violations) > 0 {
		result.IsValid = false
		for _, violation := range violations {
			result.Errors = append(result.Errors, violation.Error())
		}
		result.Risk = "HIGH"
	}
	
	// 7. SQL注入攻击检查
	if violations := v.checkSQLInjection(sql); len(violations) > 0 {
		result.IsValid = false
		for _, violation := range violations {
			result.Errors = append(result.Errors, violation.Error())
		}
		result.Risk = "HIGH"
	}
	
	// 8. 提取表名
	result.TablesUsed = v.extractTables(cleanedSQL)
	
	// 9. 风险评估
	if result.IsValid && len(result.Warnings) > 0 {
		result.Risk = "MEDIUM"
	}
	
	return result
}

// cleanSQL 清理SQL查询，移除注释和多余空白
func (v *SQLSecurityValidator) cleanSQL(sql string) string {
	// 移除注释
	cleaned := sql
	for _, pattern := range v.commentPatterns {
		cleaned = pattern.ReplaceAllString(cleaned, " ")
	}
	
	// 标准化空白字符
	cleaned = strings.TrimSpace(cleaned)
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	
	return cleaned
}

// checkStatementCount 检查语句数量
func (v *SQLSecurityValidator) checkStatementCount(sql string) error {
	// 简单的分号计数（排除字符串内的分号）
	inString := false
	var stringChar rune
	statementCount := 1
	
	for _, char := range sql {
		if !inString && (char == '\'' || char == '"') {
			inString = true
			stringChar = char
		} else if inString && char == stringChar {
			inString = false
		} else if !inString && char == ';' {
			statementCount++
		}
	}
	
	if statementCount > v.maxStatements {
		return fmt.Errorf("SQL语句数量超过限制，最多允许%d条语句", v.maxStatements)
	}
	
	return nil
}

// detectQueryType 检测查询类型
func (v *SQLSecurityValidator) detectQueryType(sql string) string {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	
	queryTypes := map[string]string{
		"SELECT": "SELECT",
		"WITH":   "WITH",
		"EXPLAIN": "EXPLAIN",
		"DESCRIBE": "DESCRIBE",
		"DESC":   "DESCRIBE", 
		"SHOW":   "SHOW",
		"INSERT": "INSERT",
		"UPDATE": "UPDATE",
		"DELETE": "DELETE",
		"CREATE": "CREATE",
		"DROP":   "DROP",
		"ALTER":  "ALTER",
	}
	
	for prefix, queryType := range queryTypes {
		if strings.HasPrefix(upperSQL, prefix) {
			return queryType
		}
	}
	
	return "UNKNOWN"
}

// isReadOnlyQuery 判断是否为只读查询
func (v *SQLSecurityValidator) isReadOnlyQuery(queryType string) bool {
	readOnlyTypes := []string{"SELECT", "WITH", "EXPLAIN", "DESCRIBE", "SHOW"}
	
	for _, readOnlyType := range readOnlyTypes {
		if queryType == readOnlyType {
			return true
		}
	}
	
	return false
}

// isAllowedStatement 检查是否为允许的语句类型
func (v *SQLSecurityValidator) isAllowedStatement(queryType string) bool {
	for _, allowed := range v.allowedStatements {
		if strings.ToUpper(allowed) == queryType {
			return true
		}
	}
	return false
}

// checkForbiddenKeywords 检查禁止关键词
func (v *SQLSecurityValidator) checkForbiddenKeywords(sql string) []SecurityViolation {
	violations := []SecurityViolation{}
	upperSQL := strings.ToUpper(sql)
	
	for _, keyword := range v.forbiddenKeywords {
		if strings.Contains(upperSQL, keyword) {
			violations = append(violations, SecurityViolation{
				Type:    "FORBIDDEN_KEYWORD",
				Message: fmt.Sprintf("检测到禁止使用的关键词: %s", keyword),
				Context: keyword,
			})
		}
	}
	
	return violations
}

// checkSQLInjection 检查SQL注入攻击
func (v *SQLSecurityValidator) checkSQLInjection(sql string) []SecurityViolation {
	violations := []SecurityViolation{}
	
	// 使用编译后的正则表达式检测
	for _, pattern := range v.sqlInjectionPatterns {
		if matches := pattern.FindAllString(sql, -1); len(matches) > 0 {
			for _, match := range matches {
				violations = append(violations, SecurityViolation{
					Type:    "SQL_INJECTION",
					Message: fmt.Sprintf("检测到疑似SQL注入攻击模式: %s", match),
					Context: match,
				})
			}
		}
	}
	
	// 字符编码检查
	if v.containsSuspiciousEncoding(sql) {
		violations = append(violations, SecurityViolation{
			Type:    "ENCODING_ATTACK",
			Message: "检测到可疑的字符编码，可能为编码绕过攻击",
			Context: "字符编码异常",
		})
	}
	
	return violations
}

// containsSuspiciousEncoding 检查可疑的字符编码
func (v *SQLSecurityValidator) containsSuspiciousEncoding(sql string) bool {
	// 检查是否包含不可见字符或特殊编码
	for _, char := range sql {
		if !unicode.IsPrint(char) && !unicode.IsSpace(char) {
			return true
		}
	}
	
	// 检查十六进制编码
	hexPattern := regexp.MustCompile(`(?i)0x[0-9a-f]+`)
	if hexPattern.MatchString(sql) {
		// 简单检查，实际应用中需要更精确的判断
		return strings.Count(sql, "0x") > 2
	}
	
	return false
}

// extractTables 提取SQL中使用的表名
func (v *SQLSecurityValidator) extractTables(sql string) []string {
	tables := []string{}
	upperSQL := strings.ToUpper(sql)
	
	// 简化的表名提取逻辑
	// 查找FROM和JOIN关键词后的表名
	keywords := []string{"FROM", "JOIN", "UPDATE", "INTO"}
	
	for _, keyword := range keywords {
		if index := strings.Index(upperSQL, keyword); index != -1 {
			afterKeyword := strings.TrimSpace(sql[index+len(keyword):])
			words := strings.Fields(afterKeyword)
			
			if len(words) > 0 {
				tableName := strings.Trim(words[0], " \t\n\r(),;")
				// 移除schema前缀
				if dotIndex := strings.LastIndex(tableName, "."); dotIndex != -1 {
					tableName = tableName[dotIndex+1:]
				}
				
				// 去重添加
				found := false
				for _, existing := range tables {
					if strings.EqualFold(existing, tableName) {
						found = true
						break
					}
				}
				if !found && tableName != "" {
					tables = append(tables, tableName)
				}
			}
		}
	}
	
	return tables
}

// ValidateSQLStrict 严格模式SQL验证
func (v *SQLSecurityValidator) ValidateSQLStrict(sql string) error {
	result := v.ValidateSQL(sql)
	
	if !result.IsValid {
		return errors.New(strings.Join(result.Errors, "; "))
	}
	
	// 严格模式下，警告也被视为错误
	if len(result.Warnings) > 0 {
		return errors.New("严格模式下不允许的操作: " + strings.Join(result.Warnings, "; "))
	}
	
	return nil
}

// GetSecurityReport 获取安全报告
func (v *SQLSecurityValidator) GetSecurityReport(sql string) map[string]interface{} {
	result := v.ValidateSQL(sql)
	
	report := map[string]interface{}{
		"query_length":     len(sql),
		"query_type":       result.QueryType,
		"is_read_only":     result.IsReadOnly,
		"tables_used":      result.TablesUsed,
		"validation_result": result,
		"risk_level":       result.Risk,
		"security_score":   v.calculateSecurityScore(result),
	}
	
	return report
}

// calculateSecurityScore 计算安全评分 (0-100)
func (v *SQLSecurityValidator) calculateSecurityScore(result *ValidationResult) int {
	score := 100
	
	// 错误扣分
	score -= len(result.Errors) * 30
	
	// 警告扣分
	score -= len(result.Warnings) * 10
	
	// 风险等级扣分
	switch result.Risk {
	case "HIGH":
		score -= 50
	case "MEDIUM":
		score -= 20
	}
	
	// 确保分数在合理范围内
	if score < 0 {
		score = 0
	}
	
	return score
}