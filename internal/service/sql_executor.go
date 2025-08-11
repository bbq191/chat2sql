package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// SQLExecutor SQL执行器
// 基于pgxpool实现高性能SQL查询执行，支持多数据库连接和超时控制
type SQLExecutor struct {
	// 核心组件
	systemPool        *pgxpool.Pool       // 主连接池（用于系统数据库）
	connectionManager *ConnectionManager  // 连接管理器
	logger            *zap.Logger         // 日志器

	// 配置参数
	queryTimeout time.Duration // 查询超时时间
	maxRows      int32         // 最大返回行数
	maxResultMB  int32         // 最大结果集大小(MB)
}

// SQLExecutorConfig SQL执行器配置
type SQLExecutorConfig struct {
	QueryTimeout time.Duration `json:"query_timeout"`   // 查询超时时间，默认30秒
	MaxRows      int32         `json:"max_rows"`        // 最大返回行数，默认1000行
	MaxResultMB  int32         `json:"max_result_mb"`   // 最大结果集大小，默认10MB
}

// QueryResult SQL查询结果
type QueryResult struct {
	Columns       []string                   `json:"columns"`        // 列名
	Rows          []map[string]any           `json:"rows"`          // 数据行
	RowCount      int32                      `json:"row_count"`     // 行数
	ExecutionTime int32                      `json:"execution_time"` // 执行时间(毫秒)
	QueryType     string                     `json:"query_type"`    // 查询类型
	Status        string                     `json:"status"`        // 执行状态
	Error         string                     `json:"error,omitempty"` // 错误信息
	Warnings      []string                   `json:"warnings,omitempty"` // 警告信息
}

// NewSQLExecutor 创建SQL执行器
func NewSQLExecutor(systemPool *pgxpool.Pool, connectionManager *ConnectionManager, logger *zap.Logger) *SQLExecutor {
	// 默认配置
	config := &SQLExecutorConfig{
		QueryTimeout: 30 * time.Second,
		MaxRows:      1000,
		MaxResultMB:  10,
	}

	return &SQLExecutor{
		systemPool:        systemPool,
		connectionManager: connectionManager,
		logger:            logger,
		queryTimeout:      config.QueryTimeout,
		maxRows:           config.MaxRows,
		maxResultMB:       config.MaxResultMB,
	}
}

// NewSQLExecutorWithConfig 使用自定义配置创建SQL执行器
func NewSQLExecutorWithConfig(systemPool *pgxpool.Pool, connectionManager *ConnectionManager, config *SQLExecutorConfig, logger *zap.Logger) *SQLExecutor {
	if config == nil {
		return NewSQLExecutor(systemPool, connectionManager, logger)
	}

	// 设置默认值
	if config.QueryTimeout <= 0 {
		config.QueryTimeout = 30 * time.Second
	}
	if config.MaxRows <= 0 {
		config.MaxRows = 1000
	}
	if config.MaxResultMB <= 0 {
		config.MaxResultMB = 10
	}

	return &SQLExecutor{
		systemPool:        systemPool,
		connectionManager: connectionManager,
		logger:            logger,
		queryTimeout:      config.QueryTimeout,
		maxRows:           config.MaxRows,
		maxResultMB:       config.MaxResultMB,
	}
}

// ExecuteQuery 执行SQL查询
// 在指定的数据库连接上执行SELECT查询，支持超时控制和结果大小限制
func (e *SQLExecutor) ExecuteQuery(ctx context.Context, sql string, connection *repository.DatabaseConnection) (*QueryResult, error) {
	start := time.Now()

	e.logger.Info("开始执行SQL查询",
		zap.String("sql", sql),
		zap.Int64("connection_id", connection.ID),
		zap.String("database", connection.DatabaseName))

	// 创建查询上下文，设置超时
	queryCtx, cancel := context.WithTimeout(ctx, e.queryTimeout)
	defer cancel()

	// 通过ConnectionManager获取目标数据库连接池
	targetPool, err := e.connectionManager.GetConnectionPool(queryCtx, connection.ID)
	if err != nil {
		return &QueryResult{
			Status:        string(repository.QueryError),
			Error:         fmt.Sprintf("数据库连接失败: %v", err),
			ExecutionTime: int32(time.Since(start).Milliseconds()),
		}, err
	}
	// 注意：不需要关闭连接池，由ConnectionManager管理

	// 执行查询
	result, err := e.executeQueryOnPool(queryCtx, sql, targetPool)
	result.ExecutionTime = int32(time.Since(start).Milliseconds())

	if err != nil {
		e.logger.Error("SQL查询执行失败",
			zap.Error(err),
			zap.String("sql", sql),
			zap.Int64("connection_id", connection.ID),
			zap.Int32("execution_time", result.ExecutionTime))
		return result, err
	}

	e.logger.Info("SQL查询执行成功",
		zap.String("sql", sql),
		zap.Int64("connection_id", connection.ID),
		zap.Int32("row_count", result.RowCount),
		zap.Int32("execution_time", result.ExecutionTime))

	return result, nil
}


// executeQueryOnPool 在指定连接池上执行查询
func (e *SQLExecutor) executeQueryOnPool(ctx context.Context, sql string, pool *pgxpool.Pool) (*QueryResult, error) {
	result := &QueryResult{
		Columns:   []string{},
		Rows:      []map[string]any{},
		QueryType: e.detectQueryType(sql),
		Status:    string(repository.QuerySuccess),
		Warnings:  []string{},
	}

	// 执行查询
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		result.Status = string(repository.QueryError)
		
		// 解析PostgreSQL错误
		if pgErr, ok := err.(*pgconn.PgError); ok {
			result.Error = fmt.Sprintf("数据库错误 [%s]: %s", pgErr.Code, pgErr.Message)
		} else {
			result.Error = fmt.Sprintf("查询执行失败: %v", err)
		}
		
		return result, err
	}
	defer rows.Close()

	// 获取列信息
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, desc := range fieldDescriptions {
		columns[i] = string(desc.Name)
	}
	result.Columns = columns

	// 读取数据行
	var rowCount int32 = 0
	var totalSizeBytes int64 = 0
	maxSizeBytes := int64(e.maxResultMB * 1024 * 1024) // 转换为字节

	for rows.Next() {
		// 检查行数限制
		if rowCount >= e.maxRows {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("查询结果超过最大行数限制(%d行)，已截断显示", e.maxRows))
			break
		}

		// 读取行数据
		values, err := rows.Values()
		if err != nil {
			result.Status = string(repository.QueryError)
			result.Error = fmt.Sprintf("读取查询结果失败: %v", err)
			return result, err
		}

		// 转换为map格式
		rowData := make(map[string]any)
		for i, value := range values {
			// 处理特殊数据类型
			rowData[columns[i]] = e.convertValue(value)
		}

		// 估算行大小
		rowJSON, err := json.Marshal(rowData)
		if err != nil {
			result.Status = string(repository.QueryError)
			result.Error = fmt.Sprintf("JSON序列化失败: %v", err)
			return result, err
		}
		rowSize := int64(len(rowJSON))
		
		// 检查结果集大小限制
		if totalSizeBytes+rowSize > maxSizeBytes {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("查询结果超过最大大小限制(%dMB)，已截断显示", e.maxResultMB))
			break
		}

		result.Rows = append(result.Rows, rowData)
		rowCount++
		totalSizeBytes += rowSize
	}

	result.RowCount = rowCount

	// 检查迭代器错误
	if err := rows.Err(); err != nil {
		result.Status = string(repository.QueryError)
		result.Error = fmt.Sprintf("读取查询结果时发生错误: %v", err)
		return result, err
	}

	return result, nil
}

// convertValue 转换数据库值为JSON友好的格式
func (e *SQLExecutor) convertValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		// 二进制数据转换为base64字符串
		return fmt.Sprintf("base64:%s", base64.StdEncoding.EncodeToString(v))
	case json.Number:
		// JSON数值类型转换
		return v.String()
	default:
		return value
	}
}

// detectQueryType 检测查询类型
func (e *SQLExecutor) detectQueryType(sql string) string {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	
	if strings.HasPrefix(upperSQL, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(upperSQL, "WITH") {
		return "WITH"  // CTE查询
	} else if strings.HasPrefix(upperSQL, "EXPLAIN") {
		return "EXPLAIN"
	} else if strings.HasPrefix(upperSQL, "SHOW") {
		return "SHOW"
	} else {
		return "UNKNOWN"
	}
}

// TestConnection 测试数据库连接
func (e *SQLExecutor) TestConnection(ctx context.Context, connection *repository.DatabaseConnection) error {
	e.logger.Info("测试数据库连接",
		zap.Int64("connection_id", connection.ID),
		zap.String("host", connection.Host),
		zap.String("database", connection.DatabaseName))

	// 直接使用ConnectionManager的TestConnection方法
	err := e.connectionManager.TestConnection(ctx, connection)
	if err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}

	e.logger.Info("数据库连接测试成功",
		zap.Int64("connection_id", connection.ID))

	return nil
}

// GetQueryInfo 获取查询信息（不执行查询）
func (e *SQLExecutor) GetQueryInfo(sql string) *QueryInfo {
	return &QueryInfo{
		QueryType:  e.detectQueryType(sql),
		IsReadOnly: e.isReadOnlyQuery(sql),
		Tables:     e.extractTables(sql),
	}
}

// QueryInfo 查询信息
type QueryInfo struct {
	QueryType  string   `json:"query_type"`  // 查询类型
	IsReadOnly bool     `json:"is_read_only"` // 是否只读查询
	Tables     []string `json:"tables"`      // 涉及的表
}

// isReadOnlyQuery 判断是否为只读查询
func (e *SQLExecutor) isReadOnlyQuery(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	readOnlyPrefixes := []string{"SELECT", "WITH", "EXPLAIN", "SHOW"}
	
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(upperSQL, prefix) {
			return true
		}
	}
	
	return false
}

// extractTables 简单提取SQL中的表名
// 注意: 当前为简化实现，未来可考虑集成专业的SQL解析器提供更准确的表名提取
func (e *SQLExecutor) extractTables(sql string) []string {
	upperSQL := strings.ToUpper(sql)
	tables := []string{}
	
	// 查找FROM关键词
	fromIndex := strings.Index(upperSQL, " FROM ")
	if fromIndex == -1 {
		return tables
	}
	
	// 简化实现：提取FROM后第一个单词作为表名
	sqlAfterFrom := strings.TrimSpace(sql[fromIndex+6:])
	words := strings.Fields(sqlAfterFrom)
	if len(words) > 0 {
		tableName := strings.Trim(words[0], " \t\n\r(),;")
		// 去除schema前缀
		if dotIndex := strings.LastIndex(tableName, "."); dotIndex != -1 {
			tableName = tableName[dotIndex+1:]
		}
		tables = append(tables, tableName)
	}
	
	return tables
}