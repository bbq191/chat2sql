// Package ai SQL安全验证器和解析器
// 基于sqlmap技术增强的高级安全检测
package ai

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"crypto/md5"
	"encoding/hex"
	"time"
)

// SQLValidator SQL安全验证器
type SQLValidator struct {
	// 危险关键词黑名单
	dangerousKeywords []string
	
	// 允许的操作类型
	allowedOperations []string
	
	// 正则表达式模式
	patterns map[string]*regexp.Regexp
	
	// 配置参数
	config *ValidatorConfig
	
	// SQL-92保留关键词（基于sqlmap词典）
	sqlReservedWords map[string]bool
	
	// 高级注入检测模式
	injectionPatterns []InjectionPattern
	
	// 安全评分缓存
	safetyScoreCache map[string]*CachedScore
}

// ValidatorConfig 验证器配置
type ValidatorConfig struct {
	// 严格模式：是否启用最严格的验证
	StrictMode bool `yaml:"strict_mode"`
	
	// 最大查询长度
	MaxQueryLength int `yaml:"max_query_length"`
	
	// 最大JOIN数量
	MaxJoins int `yaml:"max_joins"`
	
	// 最大子查询深度
	MaxSubqueryDepth int `yaml:"max_subquery_depth"`
	
	// 是否允许函数调用
	AllowFunctions bool `yaml:"allow_functions"`
	
	// 允许的函数白名单
	AllowedFunctions []string `yaml:"allowed_functions"`
	
	// 是否允许通配符 (*)
	AllowWildcard bool `yaml:"allow_wildcard"`
	
	// 最大返回行数限制
	MaxRowsLimit int `yaml:"max_rows_limit"`
	
	// 高级安全检测配置
	EnableAdvancedDetection bool `yaml:"enable_advanced_detection"`
	InjectionSensitivity    int  `yaml:"injection_sensitivity"` // 1-5级敏感度
	EnablePatternCache      bool `yaml:"enable_pattern_cache"`
	CacheTimeout           time.Duration `yaml:"cache_timeout"`
}

// ValidationError 验证错误
type ValidationError struct {
	Type        string `json:"type"`        // 错误类型
	Message     string `json:"message"`     // 错误信息
	Position    int    `json:"position"`    // 错误位置
	Severity    string `json:"severity"`    // 严重程度: critical, warning, info
	Suggestion  string `json:"suggestion"`  // 修复建议
	InjectionType string `json:"injection_type,omitempty"` // 注入类型（如果检测到）
	RiskLevel     int    `json:"risk_level,omitempty"`     // 风险等级 1-10
}

// InjectionPattern 注入攻击模式
type InjectionPattern struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // boolean-blind, time-blind, union, error-based
	Patterns     []string `json:"patterns"`
	Weight       float64  `json:"weight"`
	RiskLevel    int      `json:"risk_level"`
	Description  string   `json:"description"`
}

// CachedScore 缓存的安全评分
type CachedScore struct {
	Score     int       `json:"score"`
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid   bool              `json:"is_valid"`   // 是否有效
	Errors    []ValidationError `json:"errors"`     // 错误列表
	Warnings  []ValidationError `json:"warnings"`   // 警告列表
	SQLInfo   SQLInfo          `json:"sql_info"`   // SQL信息分析
	Score     int              `json:"score"`       // 安全评分 (0-100)
}

// SQLInfo SQL信息结构
type SQLInfo struct {
	QueryType        string   `json:"query_type"`        // 查询类型
	Tables           []string `json:"tables"`            // 涉及的表
	Columns          []string `json:"columns"`           // 涉及的列
	Functions        []string `json:"functions"`         // 使用的函数
	JoinCount        int      `json:"join_count"`        // JOIN数量
	SubqueryDepth    int      `json:"subquery_depth"`    // 子查询深度
	EstimatedComplexity string `json:"estimated_complexity"` // 复杂度估计
	HasLimit         bool     `json:"has_limit"`         // 是否有LIMIT子句
	HasWhere         bool     `json:"has_where"`         // 是否有WHERE子句
}

// NewSQLValidator 创建新的SQL验证器
func NewSQLValidator() *SQLValidator {
	// 默认配置
	config := &ValidatorConfig{
		StrictMode:       true,
		MaxQueryLength:   5000,
		MaxJoins:         5,
		MaxSubqueryDepth: 3,
		AllowFunctions:   true,
		AllowedFunctions: []string{
			"COUNT", "SUM", "AVG", "MAX", "MIN", "DISTINCT",
			"UPPER", "LOWER", "SUBSTRING", "LENGTH", "TRIM",
			"DATE", "YEAR", "MONTH", "DAY", "NOW", "CURRENT_TIMESTAMP",
			"COALESCE", "NULLIF", "CASE", "CAST", "CONVERT",
		},
		AllowWildcard:           true,
		MaxRowsLimit:            10000,
		EnableAdvancedDetection: true,
		InjectionSensitivity:    3, // 中等敏感度
		EnablePatternCache:      true,
		CacheTimeout:           time.Hour,
	}
	
	sv := &SQLValidator{
		config: config,
		dangerousKeywords: []string{
			// DML操作
			"INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE",
			"TRUNCATE", "DROP", "CREATE", "ALTER", "GRANT", "REVOKE",
			
			// 系统函数
			"EXEC", "EXECUTE", "SP_", "XP_", "DBCC",
			
			// 文件操作
			"BULK", "OPENROWSET", "OPENDATASOURCE",
			
			// 危险函数
			"LOAD_FILE", "INTO OUTFILE", "INTO DUMPFILE",
			
			// 权限相关
			"SHOW GRANTS", "SHOW PRIVILEGES",
			
			// 脚本执行
			"SCRIPT", "EVAL", "EXEC(", "SYSTEM",
			
			// 联合查询注入
			"UNION ALL SELECT", "UNION SELECT",
			
			// 注释符号 (可能用于SQL注入)
			"/*", "*/", "--", "#",
		},
		safetyScoreCache: make(map[string]*CachedScore),
		allowedOperations: []string{
			"SELECT", "WITH", "ORDER BY", "GROUP BY", "HAVING",
			"WHERE", "JOIN", "INNER JOIN", "LEFT JOIN", "RIGHT JOIN",
			"FULL JOIN", "CROSS JOIN", "UNION", "LIMIT", "OFFSET",
		},
	}
	
	// 初始化正则表达式模式
	sv.initializePatterns()
	
	// 初始化SQL-92保留关键词
	sv.initializeSQLReservedWords()
	
	// 初始化高级注入检测模式
	sv.initializeInjectionPatterns()
	
	return sv
}

// initializePatterns 初始化正则表达式模式
func (sv *SQLValidator) initializePatterns() {
	sv.patterns = map[string]*regexp.Regexp{
		// 基本SQL结构
		"select_pattern":    regexp.MustCompile(`(?i)^\s*SELECT\b`),
		"from_pattern":      regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)`),
		"where_pattern":     regexp.MustCompile(`(?i)\bWHERE\b`),
		"join_pattern":      regexp.MustCompile(`(?i)\b(?:INNER\s+|LEFT\s+|RIGHT\s+|FULL\s+)?JOIN\b`),
		"limit_pattern":     regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)`),
		"offset_pattern":    regexp.MustCompile(`(?i)\bOFFSET\s+(\d+)`),
		"order_by_pattern":  regexp.MustCompile(`(?i)\bORDER\s+BY\b`),
		"group_by_pattern":  regexp.MustCompile(`(?i)\bGROUP\s+BY\b`),
		"having_pattern":    regexp.MustCompile(`(?i)\bHAVING\b`),
		
		// 函数检测
		"function_pattern":  regexp.MustCompile(`(?i)\b([A-Z_][A-Z0-9_]*)\s*\(`),
		
		// 子查询检测
		"subquery_pattern": regexp.MustCompile(`\([^)]*SELECT[^)]*\)`),
		
		// 表名提取
		"table_extract":    regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*(?:\s+[a-zA-Z_][a-zA-Z0-9_]*)?)`),
		"join_table_extract": regexp.MustCompile(`(?i)\bJOIN\s+([a-zA-Z_][a-zA-Z0-9_]*(?:\s+[a-zA-Z_][a-zA-Z0-9_]*)?)`),
		
		// 危险模式检测
		"union_injection":   regexp.MustCompile(`(?i)UNION\s+(?:ALL\s+)?SELECT`),
		"comment_injection": regexp.MustCompile(`(?:--[^\r\n]*|\/\*[\s\S]*?\*\/|#[^\r\n]*)`),
		"stacked_queries":   regexp.MustCompile(`;\s*(?:SELECT|INSERT|UPDATE|DELETE|DROP|CREATE|ALTER)`),
		
		// 列名提取 (简化版)
		"column_extract":    regexp.MustCompile(`(?i)SELECT\s+([^FROM]+)`),
	}
}

// Validate 验证SQL语句
func (sv *SQLValidator) Validate(sql string) error {
	result := sv.ValidateDetailed(sql)
	
	if !result.IsValid {
		// 返回第一个严重错误
		for _, err := range result.Errors {
			if err.Severity == "critical" {
				return fmt.Errorf("%s: %s", err.Type, err.Message)
			}
		}
		
		// 如果没有严重错误，返回第一个错误
		if len(result.Errors) > 0 {
			err := result.Errors[0]
			return fmt.Errorf("%s: %s", err.Type, err.Message)
		}
	}
	
	return nil
}

// ValidateDetailed 详细验证SQL语句
func (sv *SQLValidator) ValidateDetailed(sql string) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
		Score:    100,
	}
	
	// 基础检查
	if err := sv.basicValidation(sql); err != nil {
		result.Errors = append(result.Errors, *err)
		result.IsValid = false
		result.Score -= 50
	}
	
	// 危险关键词检查
	if errors := sv.checkDangerousKeywords(sql); len(errors) > 0 {
		result.Errors = append(result.Errors, errors...)
		result.IsValid = false
		result.Score -= 30
	}
	
	// 结构验证
	if errors := sv.validateStructure(sql); len(errors) > 0 {
		result.Errors = append(result.Errors, errors...)
		result.IsValid = false
		result.Score -= 20
	}
	
	// 高级注入检测 (基于sqlmap技术增强)
	if sv.config.EnableAdvancedDetection {
		if injections := sv.detectAdvancedInjections(sql); len(injections) > 0 {
			result.Errors = append(result.Errors, injections...)
			result.IsValid = false
			result.Score -= 40
		}
	}
	
	// 智能安全评分算法
	securityScore := sv.calculateSecurityScore(sql)
	result.Score = int(float64(result.Score) * securityScore)
	
	// 复杂度检查
	if warnings := sv.checkComplexity(sql); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
		result.Score -= 10
	}
	
	// 分析SQL信息
	result.SQLInfo = sv.analyzeSQLInfo(sql)
	
	// 性能相关检查
	if warnings := sv.checkPerformance(sql, &result.SQLInfo); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
		result.Score -= 5
	}
	
	// 确保分数不低于0
	if result.Score < 0 {
		result.Score = 0
	}
	
	return result
}

// basicValidation 基础验证
func (sv *SQLValidator) basicValidation(sql string) *ValidationError {
	// 空查询检查
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return &ValidationError{
			Type:     "empty_query",
			Message:  "SQL查询不能为空",
			Severity: "critical",
		}
	}
	
	// 长度检查
	if len(sql) > sv.config.MaxQueryLength {
		return &ValidationError{
			Type:     "query_too_long",
			Message:  fmt.Sprintf("SQL查询过长，最大支持%d字符，当前%d字符", sv.config.MaxQueryLength, len(sql)),
			Severity: "critical",
			Suggestion: "请简化查询或分解为多个查询",
		}
	}
	
	// SELECT检查 (严格模式下只允许SELECT和WITH)
	if sv.config.StrictMode {
		upperTrimmed := strings.ToUpper(trimmed)
		if !strings.HasPrefix(upperTrimmed, "SELECT") && !strings.HasPrefix(upperTrimmed, "WITH") {
			return &ValidationError{
				Type:     "invalid_operation",
				Message:  "严格模式下只允许SELECT和WITH查询",
				Severity: "critical",
				Suggestion: "请使用SELECT或WITH语句进行数据查询",
			}
		}
	}
	
	return nil
}

// checkDangerousKeywords 检查危险关键词
func (sv *SQLValidator) checkDangerousKeywords(sql string) []ValidationError {
	var errors []ValidationError
	upperSQL := strings.ToUpper(sql)
	
	for _, keyword := range sv.dangerousKeywords {
		// 使用单词边界匹配，避免 "created_at" 误匹配 "CREATE"
		pattern := `\b` + regexp.QuoteMeta(keyword) + `\b`
		if matched, _ := regexp.MatchString(pattern, upperSQL); matched {
			pos := strings.Index(upperSQL, keyword)
			errors = append(errors, ValidationError{
				Type:       "dangerous_keyword",
				Message:    fmt.Sprintf("检测到危险关键词: %s", keyword),
				Position:   pos,
				Severity:   "critical",
				Suggestion: "请移除危险操作，只使用SELECT查询",
			})
		}
	}
	
	// 特殊模式检查
	if sv.patterns["union_injection"].MatchString(sql) {
		errors = append(errors, ValidationError{
			Type:     "union_injection",
			Message:  "检测到潜在的UNION注入攻击",
			Severity: "critical",
		})
	}
	
	if sv.patterns["comment_injection"].MatchString(sql) {
		errors = append(errors, ValidationError{
			Type:     "comment_injection",
			Message:  "检测到SQL注释符号，可能存在注入风险",
			Severity: "warning",
		})
	}
	
	if sv.patterns["stacked_queries"].MatchString(sql) {
		errors = append(errors, ValidationError{
			Type:     "stacked_queries",
			Message:  "检测到堆叠查询，可能存在安全风险",
			Severity: "critical",
		})
	}
	
	return errors
}

// validateStructure 验证SQL结构
func (sv *SQLValidator) validateStructure(sql string) []ValidationError {
	var errors []ValidationError
	
	// 检查括号匹配
	if err := sv.checkParenthesesBalance(sql); err != nil {
		errors = append(errors, *err)
	}
	
	// 检查基本SQL语法结构
	if !sv.patterns["from_pattern"].MatchString(sql) {
		errors = append(errors, ValidationError{
			Type:     "missing_from",
			Message:  "SELECT查询缺少FROM子句",
			Severity: "critical",
		})
	}
	
	return errors
}

// checkComplexity 检查查询复杂度
func (sv *SQLValidator) checkComplexity(sql string) []ValidationError {
	var warnings []ValidationError
	
	// 检查JOIN数量
	joinMatches := sv.patterns["join_pattern"].FindAllString(sql, -1)
	if len(joinMatches) > sv.config.MaxJoins {
		warnings = append(warnings, ValidationError{
			Type:     "too_many_joins",
			Message:  fmt.Sprintf("JOIN数量过多: %d, 最大允许: %d", len(joinMatches), sv.config.MaxJoins),
			Severity: "warning",
			Suggestion: "考虑优化查询或分解为多个简单查询",
		})
	}
	
	// 检查子查询深度
	depth := sv.countSubqueryDepth(sql)
	if depth > sv.config.MaxSubqueryDepth {
		warnings = append(warnings, ValidationError{
			Type:     "subquery_too_deep",
			Message:  fmt.Sprintf("子查询层级过深: %d, 最大允许: %d", depth, sv.config.MaxSubqueryDepth),
			Severity: "warning",
			Suggestion: "考虑使用JOIN或临时表简化查询",
		})
	}
	
	return warnings
}

// checkPerformance 检查性能相关问题
func (sv *SQLValidator) checkPerformance(sql string, sqlInfo *SQLInfo) []ValidationError {
	var warnings []ValidationError
	
	// 检查是否缺少LIMIT子句
	if !sqlInfo.HasLimit && !sqlInfo.HasWhere {
		warnings = append(warnings, ValidationError{
			Type:     "missing_limit",
			Message:  "查询缺少LIMIT子句和WHERE条件，可能返回大量数据",
			Severity: "info",
			Suggestion: "添加LIMIT子句限制返回行数",
		})
	}
	
	// 检查LIMIT值
	if matches := sv.patterns["limit_pattern"].FindStringSubmatch(sql); len(matches) > 1 {
		// 这里应该解析limit值，简化处理
		warnings = append(warnings, ValidationError{
			Type:     "large_limit",
			Message:  "LIMIT值较大，注意查询性能",
			Severity: "info",
		})
	}
	
	// 检查SELECT *
	if !sv.config.AllowWildcard && strings.Contains(strings.ToUpper(sql), "SELECT *") {
		warnings = append(warnings, ValidationError{
			Type:     "select_wildcard",
			Message:  "使用SELECT *可能影响性能，建议明确指定列名",
			Severity: "info",
			Suggestion: "替换为具体的列名",
		})
	}
	
	return warnings
}

// analyzeSQLInfo 分析SQL信息
func (sv *SQLValidator) analyzeSQLInfo(sql string) SQLInfo {
	info := SQLInfo{
		QueryType: "SELECT",
		Tables:    []string{},
		Columns:   []string{},
		Functions: []string{},
	}
	
	// 提取表名
	if matches := sv.patterns["table_extract"].FindAllStringSubmatch(sql, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.Fields(match[1])[0] // 取第一个词作为表名
				info.Tables = append(info.Tables, tableName)
			}
		}
	}
	
	// 提取JOIN表
	if matches := sv.patterns["join_table_extract"].FindAllStringSubmatch(sql, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.Fields(match[1])[0]
				info.Tables = append(info.Tables, tableName)
			}
		}
	}
	
	// 提取函数
	if matches := sv.patterns["function_pattern"].FindAllStringSubmatch(sql, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				funcName := strings.ToUpper(match[1])
				info.Functions = append(info.Functions, funcName)
			}
		}
	}
	
	// 统计JOIN数量
	info.JoinCount = len(sv.patterns["join_pattern"].FindAllString(sql, -1))
	
	// 计算子查询深度
	info.SubqueryDepth = sv.countSubqueryDepth(sql)
	
	// 检查各种子句
	info.HasWhere = sv.patterns["where_pattern"].MatchString(sql)
	info.HasLimit = sv.patterns["limit_pattern"].MatchString(sql)
	
	// 估计复杂度
	info.EstimatedComplexity = sv.estimateComplexity(info)
	
	return info
}

// checkParenthesesBalance 检查括号平衡
func (sv *SQLValidator) checkParenthesesBalance(sql string) *ValidationError {
	var balance int
	var position int
	
	for i, char := range sql {
		switch char {
		case '(':
			balance++
		case ')':
			balance--
			if balance < 0 {
				return &ValidationError{
					Type:     "unmatched_parenthesis",
					Message:  "括号不匹配: 多余的右括号",
					Position: i,
					Severity: "critical",
				}
			}
		}
		position = i
	}
	
	if balance > 0 {
		return &ValidationError{
			Type:     "unmatched_parenthesis",
			Message:  "括号不匹配: 缺少右括号",
			Position: position,
			Severity: "critical",
		}
	}
	
	return nil
}

// countSubqueryDepth 计算子查询深度
func (sv *SQLValidator) countSubqueryDepth(sql string) int {
	maxDepth := 0
	currentDepth := 0
	inString := false
	var stringChar rune
	
	for _, char := range sql {
		// 处理字符串字面量
		if char == '\'' || char == '"' {
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				inString = false
			}
			continue
		}
		
		if inString {
			continue
		}
		
		// 检查括号
		if char == '(' {
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		} else if char == ')' {
			currentDepth--
		}
	}
	
	return maxDepth
}

// estimateComplexity 估计查询复杂度
func (sv *SQLValidator) estimateComplexity(info SQLInfo) string {
	score := 0
	
	// 基于各种因素计算复杂度分数
	score += len(info.Tables)         // 表数量
	score += info.JoinCount * 2       // JOIN数量权重更高
	score += info.SubqueryDepth * 3   // 子查询深度权重最高
	score += len(info.Functions)      // 函数数量
	
	if info.HasWhere {
		score += 1
	}
	
	switch {
	case score <= 3:
		return "simple"
	case score <= 8:
		return "medium"
	default:
		return "complex"
	}
}

// GetValidationConfig 获取验证配置
func (sv *SQLValidator) GetValidationConfig() *ValidatorConfig {
	return sv.config
}

// SetValidationConfig 设置验证配置
func (sv *SQLValidator) SetValidationConfig(config *ValidatorConfig) {
	sv.config = config
}

// IsQuerySafe 快速检查查询是否安全
func (sv *SQLValidator) IsQuerySafe(sql string) bool {
	return sv.Validate(sql) == nil
}

// GetSafetyScore 获取查询安全评分
func (sv *SQLValidator) GetSafetyScore(sql string) int {
	result := sv.ValidateDetailed(sql)
	return result.Score
}

// detectAdvancedInjections 高级注入检测 (基于sqlmap技术增强)
func (sv *SQLValidator) detectAdvancedInjections(sql string) []ValidationError {
	var errors []ValidationError
	
	// 计算查询哈希用于缓存
	hash := sv.calculateSQLHash(sql)
	
	// 检查缓存
	if sv.config.EnablePatternCache {
		if cached := sv.safetyScoreCache[hash]; cached != nil {
			if time.Since(cached.Timestamp) < sv.config.CacheTimeout {
				// 从缓存中恢复，这里简化处理
				return errors
			}
		}
	}
	
	// 遍历所有注入模式进行检测
	for _, pattern := range sv.injectionPatterns {
		for _, patternStr := range pattern.Patterns {
			regex, err := regexp.Compile(patternStr)
			if err != nil {
				continue
			}
			
			if regex.MatchString(sql) {
				// 根据敏感度调整风险评级
				adjustedRisk := pattern.RiskLevel
				if sv.config.InjectionSensitivity >= 4 {
					adjustedRisk += 1
				} else if sv.config.InjectionSensitivity <= 2 {
					adjustedRisk -= 1
				}
				
				severity := "warning"
				if adjustedRisk >= 8 {
					severity = "critical"
				} else if adjustedRisk >= 6 {
					severity = "warning"
				} else {
					severity = "info"
				}
				
				errors = append(errors, ValidationError{
					Type:          pattern.Type,
					Message:       fmt.Sprintf("检测到%s攻击模式: %s", pattern.Name, pattern.Description),
					Position:      regex.FindStringIndex(sql)[0],
					Severity:      severity,
					InjectionType: pattern.Type,
					RiskLevel:     adjustedRisk,
					Suggestion:    "请检查查询是否包含恶意代码，建议使用参数化查询",
				})
				
				// 对于高风险模式，停止进一步检测
				if adjustedRisk >= 8 {
					break
				}
			}
		}
	}
	
	// 缓存结果
	if sv.config.EnablePatternCache {
		sv.safetyScoreCache[hash] = &CachedScore{
			Score:     len(errors),
			Timestamp: time.Now(),
			Hash:      hash,
		}
	}
	
	return errors
}

// calculateSecurityScore 智能安全评分算法 (0.0-1.0)
func (sv *SQLValidator) calculateSecurityScore(sql string) float64 {
	score := 1.0
	sqlUpper := strings.ToUpper(sql)
	
	// 1. 基础关键词检查 (权重: 0.3) - 使用单词边界匹配避免误判
	dangerousCount := 0
	for _, keyword := range sv.dangerousKeywords {
		// 使用单词边界匹配，避免 "created_at" 误匹配 "CREATE"
		pattern := `\b` + regexp.QuoteMeta(keyword) + `\b`
		if matched, _ := regexp.MatchString(pattern, sqlUpper); matched {
			dangerousCount++
		}
	}
	keywordPenalty := float64(dangerousCount) * 0.1
	score -= keywordPenalty * 0.3
	
	// 2. 注入模式严重性评估 (权重: 0.4)
	injectionRisk := 0.0
	for _, pattern := range sv.injectionPatterns {
		for _, patternStr := range pattern.Patterns {
			regex, err := regexp.Compile(patternStr)
			if err != nil {
				continue
			}
			if regex.MatchString(sql) {
				injectionRisk += pattern.Weight * (float64(pattern.RiskLevel) / 10.0)
			}
		}
	}
	score -= injectionRisk * 0.4
	
	// 3. 特殊字符和编码检查 (权重: 0.2)
	specialCharPenalty := 0.0
	suspiciousChars := []string{
		"0x", "%", "\\x", "\\u", "&#", "char(", "chr(", "ascii(",
		"/*", "*/", "--", "#", ";", "||", "&&", "xor",
	}
	for _, char := range suspiciousChars {
		if strings.Contains(strings.ToLower(sql), char) {
			specialCharPenalty += 0.05
		}
	}
	score -= specialCharPenalty * 0.2
	
	// 4. 结构异常检查 (权重: 0.1)
	structuralRisk := 0.0
	
	// 检查嵌套层级过深
	nestingLevel := sv.countSubqueryDepth(sql)
	if nestingLevel > 5 {
		structuralRisk += 0.2
	}
	
	// 检查字符串长度异常
	if len(sql) > 2000 {
		structuralRisk += 0.1
	}
	
	// 检查括号平衡异常
	if sv.checkParenthesesBalance(sql) != nil {
		structuralRisk += 0.3
	}
	
	score -= structuralRisk * 0.1
	
	// 确保分数在0.0-1.0范围内
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}
	
	return score
}

// calculateSQLHash 计算SQL语句的MD5哈希
func (sv *SQLValidator) calculateSQLHash(sql string) string {
	hash := md5.Sum([]byte(sql))
	return hex.EncodeToString(hash[:])
}

// IsReservedWord 检查是否为SQL保留关键词
func (sv *SQLValidator) IsReservedWord(word string) bool {
	return sv.sqlReservedWords[strings.ToUpper(word)]
}

// GetInjectionPatterns 获取注入检测模式列表
func (sv *SQLValidator) GetInjectionPatterns() []InjectionPattern {
	return sv.injectionPatterns
}

// ValidateWithSeverity 基于严重性过滤的验证
func (sv *SQLValidator) ValidateWithSeverity(sql string, minSeverity string) *ValidationResult {
	result := sv.ValidateDetailed(sql)
	
	// 过滤错误
	filteredErrors := make([]ValidationError, 0)
	for _, err := range result.Errors {
		if sv.severityLevel(err.Severity) >= sv.severityLevel(minSeverity) {
			filteredErrors = append(filteredErrors, err)
		}
	}
	result.Errors = filteredErrors
	
	// 更新有效性
	result.IsValid = len(result.Errors) == 0
	
	return result
}

// severityLevel 将严重性字符串转换为数值
func (sv *SQLValidator) severityLevel(severity string) int {
	switch strings.ToLower(severity) {
	case "info":
		return 1
	case "warning":
		return 2
	case "critical":
		return 3
	default:
		return 0
	}
}

// SanitizeSQL 清理SQL语句 (移除潜在危险内容)
func (sv *SQLValidator) SanitizeSQL(sql string) string {
	// 移除注释
	sanitized := sv.patterns["comment_injection"].ReplaceAllString(sql, "")
	
	// 标准化空格
	sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, " ")
	
	// 移除前后空格
	sanitized = strings.TrimSpace(sanitized)
	
	return sanitized
}

// initializeSQLReservedWords 初始化SQL-92保留关键词（基于sqlmap词典增强）
func (sv *SQLValidator) initializeSQLReservedWords() {
	sv.sqlReservedWords = map[string]bool{
		// SQL-92标准保留词
		"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "UPDATE": true,
		"DELETE": true, "CREATE": true, "DROP": true, "ALTER": true, "TABLE": true,
		"INDEX": true, "VIEW": true, "DATABASE": true, "SCHEMA": true, "GRANT": true,
		"REVOKE": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
		"UNION": true, "INTERSECT": true, "EXCEPT": true, "JOIN": true, "INNER": true,
		"LEFT": true, "RIGHT": true, "FULL": true, "OUTER": true, "CROSS": true,
		"ON": true, "USING": true, "GROUP": true, "ORDER": true, "BY": true,
		"HAVING": true, "DISTINCT": true, "ALL": true, "ANY": true, "SOME": true,
		"EXISTS": true, "IN": true, "BETWEEN": true, "LIKE": true, "IS": true,
		"NULL": true, "TRUE": true, "FALSE": true, "AND": true, "OR": true,
		"NOT": true, "CASE": true, "WHEN": true, "THEN": true, "ELSE": true,
		"END": true, "IF": true, "WHILE": true, "FOR": true, "DECLARE": true,
		"SET": true, "AS": true, "ASC": true, "DESC": true, "LIMIT": true,
		"OFFSET": true, "FETCH": true, "FIRST": true, "LAST": true, "NEXT": true,
		"PRIOR": true, "ABSOLUTE": true, "RELATIVE": true, "CURRENT": true,
		
		// PostgreSQL特有关键词
		"ILIKE": true, "SIMILAR": true, "REGEXP": true, "RETURNING": true,
		"WITH": true, "RECURSIVE": true, "WINDOW": true, "OVER": true,
		"PARTITION": true, "RANGE": true, "ROWS": true, "UNBOUNDED": true,
		"PRECEDING": true, "FOLLOWING": true, "ROW": true,
		"GROUPS": true, "EXCLUDE": true, "TIES": true, "OTHERS": true,
		
		// 危险函数和存储过程（基于sqlmap增强）
		"EXEC": true, "EXECUTE": true, "SP_": true, "XP_": true, "DBCC": true,
		"BULK": true, "OPENROWSET": true, "OPENDATASOURCE": true,
		"LOAD_FILE": true, "INTO": true, "OUTFILE": true, "DUMPFILE": true,
		"SCRIPT": true, "EVAL": true, "SYSTEM": true, "SHELL": true,
		"MASTER": true, "SYS": true, "SYSDATABASES": true, "SYSTABLES": true,
		"INFORMATION_SCHEMA": true, "PG_": true, "CURRENT_USER": true,
		"SESSION_USER": true, "USER": true, "CURRENT_ROLE": true,
		
		// MySQL特有危险函数
		"LOAD": true, "DATA": true, "LOCAL": true, "INFILE": true,
		"FIELDS": true, "TERMINATED": true, "ENCLOSED": true, "ESCAPED": true,
		"LINES": true, "STARTING": true, "IGNORE": true, "REPLACE": true,
		
		// SQL Server特有危险函数
		"OPENQUERY": true, "OPENXML": true, "CONTAINS": true, "FREETEXT": true,
		"BACKUP": true, "RESTORE": true, "CHECKPOINT": true,
		"RECONFIGURE": true, "SHUTDOWN": true, "WAITFOR": true, "DELAY": true,
	}
}

// initializeInjectionPatterns 初始化高级注入检测模式（基于sqlmap技术增强）
func (sv *SQLValidator) initializeInjectionPatterns() {
	sv.injectionPatterns = []InjectionPattern{
		{
			Name:        "Boolean-based Blind",
			Type:        "boolean-blind",
			Patterns: []string{
				`(?i)AND\s+\d+=\d+`,
				`(?i)OR\s+\d+=\d+`, 
				`(?i)AND\s+\w+\s*=\s*\w+`,
				`(?i)OR\s+\w+\s*=\s*\w+`,
				`(?i)AND\s+ISNULL\s*\(`,
				`(?i)OR\s+ISNULL\s*\(`,
				`(?i)AND\s+EXISTS\s*\(`,
				`(?i)OR\s+EXISTS\s*\(`,
				`(?i)AND\s+\w+\s+IS\s+(NOT\s+)?NULL`,
				`(?i)OR\s+\w+\s+IS\s+(NOT\s+)?NULL`,
			},
			Weight:      0.7,
			RiskLevel:   6,
			Description: "布尔盲注攻击：通过AND/OR条件测试系统响应差异",
		},
		{
			Name:        "UNION Query Injection",
			Type:        "union-based",
			Patterns: []string{
				`(?i)UNION\s+(ALL\s+)?SELECT`,
				`(?i)\'\s*UNION\s+SELECT`,
				`(?i)\"\s*UNION\s+SELECT`,
				`(?i)\)\s*UNION\s+SELECT`,
				`(?i)UNION\s+SELECT\s+NULL`,
				`(?i)UNION\s+SELECT\s+\d+`,
				`(?i)UNION\s+SELECT\s+CHAR\s*\(`,
				`(?i)UNION\s+SELECT\s+CHR\s*\(`,
				`(?i)UNION\s+SELECT\s+CONCAT\s*\(`,
				`(?i)UNION\s+SELECT\s+0x[0-9a-fA-F]+`,
			},
			Weight:      0.9,
			RiskLevel:   8,
			Description: "联合查询注入：通过UNION SELECT获取额外数据",
		},
		{
			Name:        "Error-based Injection",
			Type:        "error-based", 
			Patterns: []string{
				`(?i)EXTRACTVALUE\s*\(`,
				`(?i)UPDATEXML\s*\(`,
				`(?i)EXP\s*\(\s*~\s*\(`,
				`(?i)FLOOR\s*\(\s*RAND\s*\(\s*0\s*\)\s*\*\s*2\s*\)`,
				`(?i)CONVERT\s*\(\s*INT\s*,`,
				`(?i)CAST\s*\(\s*[^)]+\s+AS\s+INT\s*\)`,
				`(?i)XMLTYPE\s*\(`,
				`(?i)UTL_INADDR\.GET_HOST_NAME`,
				`(?i)DBMS_XSLPROCESSOR\.CLOB2FILE`,
				`(?i)JSON_KEYS\s*\(`,
			},
			Weight:      0.8,
			RiskLevel:   7,
			Description: "错误注入：利用数据库错误信息泄露数据",
		},
		{
			Name:        "Time-based Blind",
			Type:        "time-blind",
			Patterns: []string{
				`(?i)SLEEP\s*\(\s*\d+\s*\)`,
				`(?i)WAITFOR\s+DELAY\s+['\"]`,
				`(?i)BENCHMARK\s*\(\s*\d+`,
				`(?i)pg_sleep\s*\(\s*\d+\s*\)`,
				`(?i)dbms_pipe\.receive_message`,
				`(?i)UTL_INADDR\.get_host_name`,
				`(?i)GENERATE_SERIES\s*\(\s*1\s*,\s*\d+\s*\)`,
				`(?i)HEAVY_QUERY\s*\(.*\)`,
				`(?i)IF\s*\([^)]+,\s*SLEEP\s*\(`,
				`(?i)IIF\s*\([^)]+,\s*WAITFOR\s+DELAY`,
			},
			Weight:      0.8,
			RiskLevel:   7,
			Description: "时间盲注：通过延时函数检测注入点",
		},
		{
			Name:        "Stacked Queries",
			Type:        "stacked-queries",
			Patterns: []string{
				`;\s*INSERT\s+INTO`,
				`;\s*UPDATE\s+\w+\s+SET`,
				`;\s*DELETE\s+FROM`,
				`;\s*DROP\s+TABLE`,
				`;\s*CREATE\s+TABLE`,
				`;\s*ALTER\s+TABLE`,
				`;\s*EXEC\s*\(`,
				`;\s*EXECUTE\s+`,
				`;\s*DECLARE\s+@`,
				`;\s*BACKUP\s+DATABASE`,
			},
			Weight:      0.95,
			RiskLevel:   9,
			Description: "堆叠查询：执行多个SQL语句",
		},
		{
			Name:        "Comment Injection",
			Type:        "comment-based",
			Patterns: []string{
				`/\*[^*]*\*/`,
				`--[^\r\n]*`,
				`#[^\r\n]*`,
				`/\*!\d+.*?\*/`,
				`/\*!.*?\*/`,
				`\*\/`,
				`;%00`,
				`%23`,
				`%2D%2D`,
				`\x00`,
			},
			Weight:      0.6,
			RiskLevel:   5,
			Description: "注释注入：利用SQL注释绕过过滤",
		},
		{
			Name:        "Function Injection",
			Type:        "function-based",
			Patterns: []string{
				`(?i)CHAR\s*\(\s*\d+[,\s\d]*\s*\)`,
				`(?i)CHR\s*\(\s*\d+\s*\)`,
				`(?i)ASCII\s*\(\s*`,
				`(?i)ORD\s*\(\s*`,
				`(?i)HEX\s*\(\s*`,
				`(?i)UNHEX\s*\(\s*`,
				`(?i)BIN\s*\(\s*`,
				`(?i)OCT\s*\(\s*`,
				`(?i)LOAD_FILE\s*\(\s*['"]/`,
				`(?i)INTO\s+OUTFILE\s+['"]/`,
			},
			Weight:      0.7,
			RiskLevel:   6,
			Description: "函数注入：利用数据库函数执行攻击",
		},
		{
			Name:        "Blind XPath Injection",
			Type:        "xpath-blind",
			Patterns: []string{
				`(?i)EXTRACTVALUE\s*\(\s*[^,]+,\s*['"]/`,
				`(?i)UPDATEXML\s*\(\s*[^,]+,\s*['"]/`,
				`(?i)XMLTYPE\s*\(\s*['"]/`,
				`extract\([^)]*\)`,
				`xmlquery\([^)]*\)`,
				`xmlexists\([^)]*\)`,
				`xmlcast\([^)]*\)`,
			},
			Weight:      0.8,
			RiskLevel:   7,
			Description: "XPath盲注：利用XML函数进行数据提取",
		},
		{
			Name:        "LDAP Injection",
			Type:        "ldap-injection",
			Patterns: []string{
				`\*\)\(\w+=\*`,
				`\*\)\(\w+=[^)]*\*`,
				`\(\|\(\w+=\*\)\(\w+=\*\)\)`,
				`\(&\(\w+=\*\)\(\w+=\*\)\)`,
				`\(\!\(\w+=\*\)\)`,
				`\*\)\(\w+=\*\)\)\(&\(\w+=\*`,
			},
			Weight:      0.6,
			RiskLevel:   5,
			Description: "LDAP注入：针对LDAP查询的注入攻击",
		},
		{
			Name:        "NoSQL Injection",
			Type:        "nosql-injection",
			Patterns: []string{
				`\{\s*['"]\$where['"]\s*:`,
				`\{\s*['"]\$regex['"]\s*:`,
				`\{\s*['"]\$ne['"]\s*:`,
				`\{\s*['"]\$gt['"]\s*:`,
				`\{\s*['"]\$lt['"]\s*:`,
				`\{\s*['"]\$or['"]\s*:`,
				`\{\s*['"]\$and['"]\s*:`,
				`\{\s*['"]\$not['"]\s*:`,
				`\{\s*['"]\$nin['"]\s*:`,
				`\{\s*['"]\$in['"]\s*:`,
			},
			Weight:      0.7,
			RiskLevel:   6,
			Description: "NoSQL注入：针对MongoDB等NoSQL数据库的注入",
		},
		{
			Name:        "Command Injection",
			Type:        "command-injection",
			Patterns: []string{
				`;\s*cat\s+/`,
				`;\s*ls\s+/`,
				`;\s*dir\s+c:`,
				`;\s*type\s+c:`,
				`;\s*whoami`,
				`;\s*id`,
				`;\s*uname`,
				`;\s*wget\s+http`,
				`;\s*curl\s+http`,
				`;\s*nc\s+-`,
			},
			Weight:      0.9,
			RiskLevel:   9,
			Description: "命令注入：通过SQL执行系统命令",
		},
	}
}

// 辅助函数：检查字符是否为SQL标识符字符
func isValidSQLIdentifierChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}