package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// SQLSecurityValidatorTestSuite SQL安全验证器测试套件
type SQLSecurityValidatorTestSuite struct {
	suite.Suite
	validator *SQLSecurityValidator
	logger    *zap.Logger
}

// SetupSuite 设置测试套件
func (suite *SQLSecurityValidatorTestSuite) SetupSuite() {
	suite.logger = zap.NewNop()
	suite.validator = NewSQLSecurityValidator(suite.logger)
}

// TestSQLSecurityValidator_ValidSQL 测试安全SQL通过验证
func (suite *SQLSecurityValidatorTestSuite) TestSQLSecurityValidator_ValidSQL() {
	t := suite.T()

	validSQLs := []struct {
		name          string
		sql           string
		expectedScore int // 期望的安全评分
	}{
		{
			"简单查询",
			"SELECT id, name FROM users WHERE status = 'active'",
			95,
		},
		{
			"带LIMIT的查询",
			"SELECT * FROM products WHERE category = 'electronics' LIMIT 100",
			90,
		},
		{
			"JOIN查询",
			"SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			85,
		},
		{
			"子查询",
			"SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE status = 'completed')",
			80,
		},
		{
			"CTE查询",
			"WITH recent_orders AS (SELECT * FROM orders WHERE created_at > '2024-01-01') SELECT * FROM recent_orders",
			85,
		},
		{
			"聚合查询",
			"SELECT COUNT(*), AVG(price) FROM products GROUP BY category ORDER BY COUNT(*) DESC",
			90,
		},
		{
			"EXPLAIN查询",
			"EXPLAIN ANALYZE SELECT * FROM users WHERE email = 'test@example.com'",
			90, // 更新期望评分，因为XA误判问题已修复
		},
		{
			"SHOW查询",
			"SHOW TABLES",
			90,
		},
	}

	for _, testCase := range validSQLs {
		t.Run(testCase.name, func(t *testing.T) {
			start := time.Now()
			
			// 测试SQL验证
			result := suite.validator.ValidateSQL(testCase.sql)
			validateDuration := time.Since(start)
			
			assert.True(t, result.IsValid, "安全SQL应该通过验证: %s", testCase.sql)
			assert.Empty(t, result.Errors, "安全SQL不应该有错误")
			assert.Less(t, validateDuration, 50*time.Millisecond, "SQL验证时间应小于50ms")

			// 验证风险等级
			assert.Contains(t, []string{"LOW", "MEDIUM"}, result.Risk, "安全SQL的风险等级应该是LOW或MEDIUM")

			t.Logf("SQL验证通过 - SQL: %s, 风险: %s, 耗时: %v", 
				testCase.sql, result.Risk, validateDuration)
		})
	}
}

// TestSQLSecurityValidator_DangerousSQL 测试危险SQL被拒绝
func (suite *SQLSecurityValidatorTestSuite) TestSQLSecurityValidator_DangerousSQL() {
	t := suite.T()

	dangerousSQLs := []struct {
		name     string
		sql      string
		reason   string
		maxScore int // 最大允许评分
	}{
		{
			"DROP TABLE",
			"DROP TABLE users",
			"DDL操作",
			30,
		},
		{
			"DELETE ALL",
			"DELETE FROM users",
			"批量删除",
			20,
		},
		{
			"INSERT操作",
			"INSERT INTO users (username) VALUES ('admin')",
			"DML写操作",
			30,
		},
		{
			"UPDATE操作", 
			"UPDATE users SET role = 'admin' WHERE id = 1",
			"DML更新操作",
			30,
		},
		{
			"SQL注入 - Union",
			"SELECT * FROM users WHERE id = 1 UNION SELECT password FROM admin",
			"Union注入",
			15,
		},
		{
			"SQL注入 - OR 1=1",
			"SELECT * FROM users WHERE username = 'admin' OR 1=1",
			"逻辑注入",
			25,
		},
		{
			"SQL注入 - 注释绕过",
			"SELECT * FROM users WHERE id = 1 -- AND password = 'secret'",
			"注释绕过",
			20,
		},
		{
			"堆叠查询",
			"SELECT * FROM users; DROP TABLE logs;",
			"堆叠查询",
			10,
		},
		{
			"时间延迟注入",
			"SELECT * FROM users WHERE id = 1 AND (SELECT COUNT(*) FROM users) > 0 AND pg_sleep(10)",
			"时间注入",
			5,
		},
		{
			"盲注测试",
			"SELECT * FROM users WHERE id = 1 AND SUBSTRING(version(),1,1) = '1'",
			"盲注攻击",
			15,
		},
		{
			"编码绕过",
			"SELECT * FROM users WHERE name = CHAR(65,68,77,73,78)",
			"编码绕过",
			20,
		},
		{
			"十六进制编码",
			"SELECT * FROM users WHERE name = 0x41444D494E",
			"十六进制绕过",
			20,
		},
	}

	for _, testCase := range dangerousSQLs {
		t.Run(testCase.name, func(t *testing.T) {
			start := time.Now()
			
			// 测试SQL验证 - 应该被拒绝
			result := suite.validator.ValidateSQL(testCase.sql)
			validateDuration := time.Since(start)
			
			assert.False(t, result.IsValid, "危险SQL应该被拒绝: %s", testCase.sql)
			assert.NotEmpty(t, result.Errors, "危险SQL应该有错误信息")
			assert.Less(t, validateDuration, 50*time.Millisecond, "SQL验证时间应小于50ms")

			// 验证风险等级应该是HIGH
			assert.Equal(t, "HIGH", result.Risk, "危险SQL的风险等级应该是HIGH")

			t.Logf("危险SQL被成功拒绝 - SQL: %s, 原因: %s, 风险: %s, 耗时: %v", 
				testCase.sql, testCase.reason, result.Risk, validateDuration)
		})
	}
}

// TestSQLSecurityValidator_EdgeCases 测试边界情况
func (suite *SQLSecurityValidatorTestSuite) TestSQLSecurityValidator_EdgeCases() {
	t := suite.T()

	edgeCases := []struct {
		name        string
		sql         string
		expectError bool
		description string
	}{
		{
			"空SQL",
			"",
			true,
			"空字符串应该被拒绝",
		},
		{
			"纯空格",
			"   \n\t  ",
			true,
			"纯空格应该被拒绝",
		},
		{
			"超长SQL",
			"SELECT " + generateLongString(10000) + " FROM users",
			true,
			"超长SQL应该被拒绝",
		},
		{
			"SQL注释",
			"-- This is a comment",
			true,
			"纯注释应该被拒绝",
		},
		{
			"多行SQL",
			`SELECT id, 
                 name,
                 email 
              FROM users 
              WHERE status = 'active'
              ORDER BY name`,
			false,
			"格式良好的多行SQL应该通过",
		},
		{
			"大小写混合",
			"sElEcT Id, NaMe FrOm UsErS wHeRe StAtUs = 'active'",
			false,
			"大小写混合的安全SQL应该通过",
		},
		{
			"带引号的字符串",
			"SELECT * FROM users WHERE name = 'John O''Connor'",
			false,
			"正确转义的字符串应该通过",
		},
	}

	for _, testCase := range edgeCases {
		t.Run(testCase.name, func(t *testing.T) {
			start := time.Now()
			
			result := suite.validator.ValidateSQL(testCase.sql)
			validateDuration := time.Since(start)
			
			if testCase.expectError {
				assert.False(t, result.IsValid, testCase.description)
				assert.NotEmpty(t, result.Errors, "应该有错误信息")
			} else {
				assert.True(t, result.IsValid, testCase.description)
				assert.Empty(t, result.Errors, "不应该有错误信息")
			}
			
			assert.Less(t, validateDuration, 100*time.Millisecond, "验证时间应小于100ms")
			
			// 验证风险等级
			assert.Contains(t, []string{"LOW", "MEDIUM", "HIGH"}, result.Risk, "风险等级应该有效")

			t.Logf("边界情况测试 - SQL: %s, 有效: %v, 风险: %s, 耗时: %v", 
				truncateString(testCase.sql, 50), result.IsValid, result.Risk, validateDuration)
		})
	}
}

// TestSQLSecurityValidator_Performance 测试SQL验证器性能
func (suite *SQLSecurityValidatorTestSuite) TestSQLSecurityValidator_Performance() {
	t := suite.T()
	
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	// 批量SQL验证性能测试
	t.Run("批量SQL验证", func(t *testing.T) {
		sqlQueries := []string{
			"SELECT * FROM users WHERE status = 'active'",
			"SELECT COUNT(*) FROM orders WHERE created_at > '2024-01-01'",
			"SELECT u.name, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id",
			"SELECT * FROM products WHERE category = 'electronics' AND price < 1000",
			"SELECT DISTINCT category FROM products ORDER BY category",
		}

		queryCount := 1000
		start := time.Now()

		for i := 0; i < queryCount; i++ {
			sql := sqlQueries[i%len(sqlQueries)]
			result := suite.validator.ValidateSQL(sql)
			require.True(t, result.IsValid, "安全SQL应该通过验证")
		}

		totalDuration := time.Since(start)
		avgDuration := totalDuration / time.Duration(queryCount)

		// P0性能指标：平均验证时间应小于10ms
		assert.Less(t, avgDuration, 10*time.Millisecond, 
			"平均SQL验证时间 %v 超过性能指标 10ms", avgDuration)

		qps := float64(queryCount) / totalDuration.Seconds()
		assert.Greater(t, qps, 1000.0, "QPS应该大于1000")

		t.Logf("批量SQL验证性能测试 - 查询数: %d, 总时间: %v, 平均: %v, QPS: %.0f", 
			queryCount, totalDuration, avgDuration, qps)
	})

	// 并发SQL验证性能测试
	t.Run("并发SQL验证", func(t *testing.T) {
		concurrency := 10
		queriesPerGoroutine := 100
		sql := "SELECT id, name, email FROM users WHERE status = 'active' LIMIT 100"

		start := time.Now()

		// 创建channel收集结果
		results := make(chan time.Duration, concurrency*queriesPerGoroutine)

		// 启动多个goroutine并发验证SQL
		for i := 0; i < concurrency; i++ {
			go func() {
				for j := 0; j < queriesPerGoroutine; j++ {
					reqStart := time.Now()
					result := suite.validator.ValidateSQL(sql)
					reqDuration := time.Since(reqStart)
					
					if result.IsValid {
						results <- reqDuration
					} else {
						results <- -1 // 标记失败
					}
				}
			}()
		}

		// 收集所有结果
		totalOperations := concurrency * queriesPerGoroutine
		var successfulOperations int
		var totalValidationTime time.Duration

		for i := 0; i < totalOperations; i++ {
			duration := <-results
			if duration > 0 {
				successfulOperations++
				totalValidationTime += duration
			}
		}

		totalTime := time.Since(start)

		if successfulOperations > 0 {
			avgDuration := totalValidationTime / time.Duration(successfulOperations)
			assert.Less(t, avgDuration, 20*time.Millisecond, 
				"并发验证平均时间 %v 超过性能指标 20ms", avgDuration)

			qps := float64(successfulOperations) / totalTime.Seconds()
			assert.Greater(t, qps, 500.0, "并发QPS应该大于500")

			t.Logf("并发SQL验证测试完成 - 成功: %d/%d, 总时间: %v, 平均: %v, QPS: %.0f", 
				successfulOperations, totalOperations, totalTime, avgDuration, qps)
		}

		assert.Greater(t, successfulOperations, totalOperations*9/10, "成功率应该超过90%")
	})
}

// TestSQLSecurityValidator_RiskAssessment 测试风险评估系统
func (suite *SQLSecurityValidatorTestSuite) TestSQLSecurityValidator_RiskAssessment() {
	t := suite.T()

	riskTests := []struct {
		name         string
		sql          string
		expectedRisk string
		shouldPass   bool
		description  string
	}{
		{
			"低风险查询",
			"SELECT id, name FROM users WHERE status = 'active' LIMIT 10",
			"LOW",
			true,
			"简单安全查询应该是低风险",
		},
		{
			"中等风险查询",
			"SELECT u.*, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id HAVING COUNT(o.id) > 5",
			"MEDIUM",
			true,
			"复杂查询可能是中等风险但仍然安全",
		},
		{
			"高风险查询",
			"SELECT * FROM users WHERE id = 1 OR 1=1",
			"HIGH",
			false,
			"SQL注入模式应该是高风险",
		},
		{
			"危险操作",
			"DROP TABLE users",
			"HIGH",
			false,
			"DDL操作应该是高风险",
		},
	}

	for _, testCase := range riskTests {
		t.Run(testCase.name, func(t *testing.T) {
			result := suite.validator.ValidateSQL(testCase.sql)
			
			assert.Equal(t, testCase.expectedRisk, result.Risk,
				"风险等级应该是 %s，实际: %s", testCase.expectedRisk, result.Risk)
			
			assert.Equal(t, testCase.shouldPass, result.IsValid,
				"验证结果应该是 %v，实际: %v", testCase.shouldPass, result.IsValid)

			t.Logf("风险评估测试 - SQL: %s, 风险: %s, 通过: %v", 
				truncateString(testCase.sql, 50), result.Risk, result.IsValid)
		})
	}
}

// 辅助函数：生成长字符串
func generateLongString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = 'A' + byte(i%26)
	}
	return string(b)
}

// 辅助函数：截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestSuite 运行SQL安全验证器测试套件
func TestSQLSecurityValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(SQLSecurityValidatorTestSuite))
}