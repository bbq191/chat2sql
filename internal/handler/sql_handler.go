package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"chat2sql-go/internal/middleware"
	"chat2sql-go/internal/repository"
	"chat2sql-go/internal/service"
)

// SQLExecutorInterface SQL执行器接口 - 直接使用Service层的QueryResult
type SQLExecutorInterface interface {
	ExecuteQuery(ctx context.Context, sql string, connection *repository.DatabaseConnection) (*service.QueryResult, error)
}


// SQLHandler SQL查询处理器
// 处理SQL执行、查询历史、语法验证等操作
type SQLHandler struct {
	queryRepo     repository.QueryHistoryRepository
	connectionRepo repository.ConnectionRepository
	sqlExecutor   SQLExecutorInterface  // SQL执行器
	logger        *zap.Logger
}

// NewSQLHandler 创建SQL处理器实例
func NewSQLHandler(
	queryRepo repository.QueryHistoryRepository,
	connectionRepo repository.ConnectionRepository,
	sqlExecutor SQLExecutorInterface,
	logger *zap.Logger,
) *SQLHandler {
	return &SQLHandler{
		queryRepo:      queryRepo,
		connectionRepo: connectionRepo,
		sqlExecutor:    sqlExecutor,
		logger:         logger,
	}
}

// ExecuteSQLRequest SQL执行请求结构
type ExecuteSQLRequest struct {
	SQL          string `json:"sql" binding:"required" example:"SELECT * FROM users LIMIT 10"`
	NaturalQuery string `json:"natural_query,omitempty" example:"获取前10个用户"`
	ConnectionID int64  `json:"connection_id" binding:"required" example:"1"`
}

// ValidateSQLRequest SQL验证请求结构
type ValidateSQLRequest struct {
	SQL string `json:"sql" binding:"required" example:"SELECT * FROM users"`
}

// QueryHistoryParams 查询历史参数
type QueryHistoryParams struct {
	Limit        int    `form:"limit,default=20" binding:"min=1,max=100" example:"20"`
	Offset       int    `form:"offset,default=0" binding:"min=0" example:"0"`
	Status       string `form:"status" binding:"omitempty,oneof=pending success error timeout" example:"success"`
	ConnectionID int64  `form:"connection_id" binding:"omitempty,min=1" example:"1"`
	Keyword      string `form:"keyword" binding:"omitempty,max=200" example:"用户查询"`
}

// SQLExecutionResult SQL执行结果
type SQLExecutionResult struct {
	QueryID       int64                    `json:"query_id" example:"123"`
	ExecutionTime int32                    `json:"execution_time" example:"150"`
	RowCount      int32                    `json:"row_count" example:"10"`
	Status        string                   `json:"status" example:"success"`
	Data          []map[string]any `json:"data,omitempty"`
	Error         string                   `json:"error,omitempty"`
}

// QueryHistoryResponse 查询历史响应
type QueryHistoryResponse struct {
	Queries    []*QueryHistoryItem `json:"queries"`
	Total      int64               `json:"total" example:"156"`
	Page       int                 `json:"page" example:"1"`
	Limit      int                 `json:"limit" example:"20"`
	HasMore    bool                `json:"has_more" example:"true"`
}

// QueryHistoryItem 查询历史项
type QueryHistoryItem struct {
	ID            int64     `json:"id" example:"123"`
	NaturalQuery  string    `json:"natural_query" example:"获取所有活跃用户"`
	GeneratedSQL  string    `json:"generated_sql" example:"SELECT * FROM users WHERE status = 'active'"`
	ExecutionTime *int32    `json:"execution_time" example:"150"`
	ResultRows    *int32    `json:"result_rows" example:"25"`
	Status        string    `json:"status" example:"success"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
	ConnectionID  *int64    `json:"connection_id" example:"1"`
	CreateTime    time.Time `json:"create_time" example:"2024-01-08T12:00:00Z"`
}

// SQLValidationResult SQL验证结果
type SQLValidationResult struct {
	IsValid      bool     `json:"is_valid" example:"true"`
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	QueryType    string   `json:"query_type" example:"SELECT"`
	TablesUsed   []string `json:"tables_used" example:"[\"users\", \"orders\"]"`
	IsReadOnly   bool     `json:"is_read_only" example:"true"`
}

// ExecuteSQL 执行SQL查询
// @Summary 执行SQL查询
// @Description 在指定数据库连接上执行SQL查询语句
// @Tags SQL查询
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ExecuteSQLRequest true "SQL执行请求"
// @Success 200 {object} SQLExecutionResult "执行成功"
// @Failure 400 {object} ErrorResponse "请求参数错误或SQL语法错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 403 {object} ErrorResponse "SQL操作被禁止"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/sql/execute [post]
func (h *SQLHandler) ExecuteSQL(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var req ExecuteSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// SQL安全验证
	if err := h.validateSQLSecurity(req.SQL); err != nil {
		h.logger.Warn("SQL security validation failed",
			zap.Error(err),
			zap.String("sql", req.SQL),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusForbidden, ErrorResponse{
			Code:    "SQL_FORBIDDEN",
			Message: err.Error(),
		})
		return
	}
	
	// 验证数据库连接权限
	connection, err := h.connectionRepo.GetByID(c.Request.Context(), req.ConnectionID)
	if err != nil || connection.UserID != userID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Code:    "CONNECTION_FORBIDDEN",
			Message: "无权访问该数据库连接",
		})
		return
	}
	
	// 创建查询历史记录
	queryHistory := &repository.QueryHistory{
		UserID:       userID,
		NaturalQuery: req.NaturalQuery,
		GeneratedSQL: req.SQL,
		Status:       string(repository.QueryPending),
		ConnectionID: &req.ConnectionID,
	}
	
	if err := h.queryRepo.Create(c.Request.Context(), queryHistory); err != nil {
		h.logger.Error("Failed to create query history",
			zap.Error(err),
			zap.Int64("user_id", userID))
	}
	
	// 执行SQL查询
	result := h.executeSQL(c.Request.Context(), req.SQL, connection)
	
	// 更新查询历史状态
	queryHistory.Status = result.Status
	queryHistory.ExecutionTime = &result.ExecutionTime
	queryHistory.ResultRows = &result.RowCount
	if result.Error != "" {
		queryHistory.ErrorMessage = &result.Error
	}
	
	if err := h.queryRepo.Update(c.Request.Context(), queryHistory); err != nil {
		h.logger.Warn("Failed to update query history",
			zap.Error(err),
			zap.Int64("query_id", queryHistory.ID))
	}
	
	h.logger.Info("SQL executed",
		zap.Int64("user_id", userID),
		zap.Int64("query_id", queryHistory.ID),
		zap.String("status", result.Status),
		zap.Int32("execution_time", result.ExecutionTime))
	
	result.QueryID = queryHistory.ID
	c.JSON(http.StatusOK, result)
}

// GetQueryHistory 获取查询历史
// @Summary 获取查询历史
// @Description 获取当前用户的SQL查询历史记录
// @Tags SQL查询
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Param offset query int false "偏移量" default(0) minimum(0)
// @Param status query string false "查询状态" Enums(pending, success, error, timeout)
// @Param connection_id query int false "数据库连接ID"
// @Param keyword query string false "搜索关键词" maxlength(200)
// @Success 200 {object} QueryHistoryResponse "获取成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/sql/history [get]
func (h *SQLHandler) GetQueryHistory(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var params QueryHistoryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "查询参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 设置默认值
	if params.Limit == 0 {
		params.Limit = 20
	}
	
	// 根据参数调用不同的查询方法
	var queries []*repository.QueryHistory
	var err error
	
	if params.Keyword != "" {
		// 关键词搜索
		queries, err = h.queryRepo.SearchByNaturalQuery(
			c.Request.Context(), userID, params.Keyword, params.Limit, params.Offset)
	} else if params.ConnectionID > 0 {
		// 按连接ID查询
		queries, err = h.queryRepo.ListByConnection(
			c.Request.Context(), params.ConnectionID, params.Limit, params.Offset)
	} else {
		// 按用户查询
		queries, err = h.queryRepo.ListByUser(
			c.Request.Context(), userID, params.Limit, params.Offset)
	}
	
	if err != nil {
		h.logger.Error("Failed to get query history",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "查询历史获取失败",
		})
		return
	}
	
	// 获取总数
	total, err := h.queryRepo.CountByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Warn("Failed to get query count",
			zap.Error(err),
			zap.Int64("user_id", userID))
		total = 0
	}
	
	// 转换为响应格式
	items := make([]*QueryHistoryItem, len(queries))
	for i, q := range queries {
		items[i] = &QueryHistoryItem{
			ID:            q.ID,
			NaturalQuery:  q.NaturalQuery,
			GeneratedSQL:  q.GeneratedSQL,
			ExecutionTime: q.ExecutionTime,
			ResultRows:    q.ResultRows,
			Status:        q.Status,
			ErrorMessage:  q.ErrorMessage,
			ConnectionID:  q.ConnectionID,
			CreateTime:    q.CreateTime,
		}
	}
	
	response := &QueryHistoryResponse{
		Queries: items,
		Total:   total,
		Page:    (params.Offset / params.Limit) + 1,
		Limit:   params.Limit,
		HasMore: int64(params.Offset+params.Limit) < total,
	}
	
	c.JSON(http.StatusOK, response)
}

// GetQueryById 获取指定查询详情
// @Summary 获取查询详情
// @Description 根据查询ID获取详细的查询信息
// @Tags SQL查询
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "查询ID"
// @Success 200 {object} QueryHistoryItem "获取成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "查询不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/sql/history/{id} [get]
func (h *SQLHandler) GetQueryById(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	// 解析查询ID
	queryIDStr := c.Param("id")
	queryID, err := strconv.ParseInt(queryIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_QUERY_ID",
			Message: "无效的查询ID",
		})
		return
	}
	
	// 获取查询记录
	query, err := h.queryRepo.GetByID(c.Request.Context(), queryID)
	if err != nil {
		h.logger.Error("Failed to get query by ID",
			zap.Error(err),
			zap.Int64("query_id", queryID))
		
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "QUERY_NOT_FOUND",
			Message: "查询记录不存在",
		})
		return
	}
	
	// 检查权限 - 只能访问自己的查询
	if query.UserID != userID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Code:    "ACCESS_DENIED",
			Message: "无权访问该查询记录",
		})
		return
	}
	
	response := &QueryHistoryItem{
		ID:            query.ID,
		NaturalQuery:  query.NaturalQuery,
		GeneratedSQL:  query.GeneratedSQL,
		ExecutionTime: query.ExecutionTime,
		ResultRows:    query.ResultRows,
		Status:        query.Status,
		ErrorMessage:  query.ErrorMessage,
		ConnectionID:  query.ConnectionID,
		CreateTime:    query.CreateTime,
	}
	
	c.JSON(http.StatusOK, response)
}

// ValidateSQL SQL语法验证
// @Summary SQL语法验证
// @Description 验证SQL语句的语法正确性和安全性
// @Tags SQL查询
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ValidateSQLRequest true "SQL验证请求"
// @Success 200 {object} SQLValidationResult "验证完成"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Router /api/v1/sql/validate [post]
func (h *SQLHandler) ValidateSQL(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var req ValidateSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 执行SQL验证
	result := h.validateSQL(req.SQL)
	
	h.logger.Info("SQL validation completed",
		zap.Int64("user_id", userID),
		zap.Bool("is_valid", result.IsValid),
		zap.String("query_type", result.QueryType))
	
	c.JSON(http.StatusOK, result)
}

// validateSQLSecurity SQL安全验证
// 防止危险的SQL操作，只允许SELECT查询
func (h *SQLHandler) validateSQLSecurity(sql string) error {
	// 转换为大写进行关键词检查
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	
	// 禁止的SQL关键词
	forbiddenKeywords := []string{
		"DROP", "DELETE", "INSERT", "UPDATE", "CREATE", "ALTER", 
		"TRUNCATE", "GRANT", "REVOKE", "REPLACE", "MERGE",
		"CALL", "EXEC", "EXECUTE", "DECLARE", "SET",
	}
	
	for _, keyword := range forbiddenKeywords {
		if strings.Contains(upperSQL, keyword) {
			return fmt.Errorf("禁止执行 %s 操作，系统仅支持查询操作", keyword)
		}
	}
	
	// 必须以SELECT开头
	if !strings.HasPrefix(upperSQL, "SELECT") {
		return fmt.Errorf("仅支持SELECT查询语句")
	}
	
	return nil
}

// validateSQL SQL语法和安全验证
func (h *SQLHandler) validateSQL(sql string) *SQLValidationResult {
	result := &SQLValidationResult{
		IsValid:   true,
		Errors:    []string{},
		Warnings:  []string{},
		IsReadOnly: true,
	}
	
	// 安全验证
	if err := h.validateSQLSecurity(sql); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}
	
	// 检测查询类型
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	if strings.HasPrefix(upperSQL, "SELECT") {
		result.QueryType = "SELECT"
	} else {
		result.QueryType = "UNKNOWN"
		result.IsReadOnly = false
	}
	
	// 基础语法验证
	if err := h.validateSQLSyntax(sql); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}
	
	// 提取表名
	result.TablesUsed = h.extractTableNames(sql)
	
	return result
}

// executeSQL 执行SQL查询 - 调用实际的SQL执行器
func (h *SQLHandler) executeSQL(ctx context.Context, sql string, connection *repository.DatabaseConnection) *SQLExecutionResult {
	// 调用Service层的SQL执行器
	result, err := h.sqlExecutor.ExecuteQuery(ctx, sql, connection)
	if err != nil {
		// 如果result为nil，创建一个默认的错误结果
		if result == nil {
			return &SQLExecutionResult{
				ExecutionTime: 0,
				RowCount:      0,
				Status:        string(repository.QueryError),
				Error:         err.Error(),
			}
		}
		return &SQLExecutionResult{
			ExecutionTime: result.ExecutionTime,
			RowCount:      0,
			Status:        string(repository.QueryError),
			Error:         result.Error,
		}
	}
	
	// 转换Service层结果到Handler层结果格式
	return &SQLExecutionResult{
		ExecutionTime: result.ExecutionTime,
		RowCount:      result.RowCount,
		Status:        result.Status,
		Data:          result.Rows,
		Error:         result.Error,
	}
}

// validateSQLSyntax 基础SQL语法验证
func (h *SQLHandler) validateSQLSyntax(sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return fmt.Errorf("SQL语句不能为空")
	}
	
	upperSQL := strings.ToUpper(sql)
	
	// 检查基本的SQL结构
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "CREATE", "DROP", "ALTER"}
	hasValidKeyword := false
	for _, keyword := range sqlKeywords {
		if strings.HasPrefix(upperSQL, keyword+" ") || upperSQL == keyword {
			hasValidKeyword = true
			break
		}
	}
	
	if !hasValidKeyword {
		return fmt.Errorf("SQL语句必须以有效的SQL关键字开始")
	}
	
	// 检查括号匹配
	if err := h.validateParenthesesBalance(sql); err != nil {
		return err
	}
	
	// 检查引号匹配
	if err := h.validateQuotesBalance(sql); err != nil {
		return err
	}
	
	// 检查基本结构（针对SELECT语句）
	if strings.HasPrefix(upperSQL, "SELECT") {
		if !strings.Contains(upperSQL, " FROM ") {
			// 允许SELECT常量或函数，如 SELECT 1, SELECT NOW()
			if !strings.Contains(upperSQL, "(") && len(strings.Fields(sql)) == 2 {
				// SELECT <constant> 是合法的
			} else if !strings.Contains(upperSQL, "FROM") {
				return fmt.Errorf("SELECT语句通常需要包含FROM子句")
			}
		}
	}
	
	return nil
}

// validateParenthesesBalance 验证括号匹配
func (h *SQLHandler) validateParenthesesBalance(sql string) error {
	stack := 0
	inString := false
	var stringChar rune
	
	for i, char := range sql {
		switch char {
		case '\'', '"':
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				// 检查是否是转义字符
				if i == 0 || sql[i-1] != '\\' {
					inString = false
				}
			}
		case '(':
			if !inString {
				stack++
			}
		case ')':
			if !inString {
				stack--
				if stack < 0 {
					return fmt.Errorf("括号不匹配：多余的右括号")
				}
			}
		}
	}
	
	if stack != 0 {
		return fmt.Errorf("括号不匹配：未闭合的左括号")
	}
	
	return nil
}

// validateQuotesBalance 验证引号匹配
func (h *SQLHandler) validateQuotesBalance(sql string) error {
	singleQuoteCount := 0
	doubleQuoteCount := 0
	
	for i, char := range sql {
		switch char {
		case '\'':
			// 检查是否是转义字符
			if i == 0 || sql[i-1] != '\\' {
				singleQuoteCount++
			}
		case '"':
			// 检查是否是转义字符
			if i == 0 || sql[i-1] != '\\' {
				doubleQuoteCount++
			}
		}
	}
	
	if singleQuoteCount%2 != 0 {
		return fmt.Errorf("单引号不匹配")
	}
	
	if doubleQuoteCount%2 != 0 {
		return fmt.Errorf("双引号不匹配")
	}
	
	return nil
}

// extractTableNames 提取SQL中的表名
func (h *SQLHandler) extractTableNames(sql string) []string {
	// 改进的表名提取逻辑
	tables := []string{}
	
	upperSQL := strings.ToUpper(sql)
	
	// 查找FROM关键词
	fromIndex := strings.Index(upperSQL, " FROM ")
	if fromIndex == -1 {
		return tables
	}
	
	// 提取FROM后到WHERE、ORDER BY、GROUP BY等关键词之间的内容
	sqlAfterFrom := sql[fromIndex+6:]
	
	// 寻找可能的终止关键词
	stopKeywords := []string{" WHERE ", " ORDER BY ", " GROUP BY ", " HAVING ", " LIMIT ", " UNION ", " EXCEPT ", " INTERSECT ", ";"}
	stopIndex := len(sqlAfterFrom)
	
	for _, keyword := range stopKeywords {
		if idx := strings.Index(strings.ToUpper(sqlAfterFrom), keyword); idx != -1 && idx < stopIndex {
			stopIndex = idx
		}
	}
	
	tableClause := strings.TrimSpace(sqlAfterFrom[:stopIndex])
	
	// 简化处理：按逗号分割，处理JOIN
	// 移除JOIN关键词并分割表名
	tableClause = strings.ReplaceAll(strings.ToUpper(tableClause), " JOIN ", ", ")
	tableClause = strings.ReplaceAll(tableClause, " LEFT JOIN ", ", ")
	tableClause = strings.ReplaceAll(tableClause, " RIGHT JOIN ", ", ")
	tableClause = strings.ReplaceAll(tableClause, " INNER JOIN ", ", ")
	tableClause = strings.ReplaceAll(tableClause, " OUTER JOIN ", ", ")
	tableClause = strings.ReplaceAll(tableClause, " FULL JOIN ", ", ")
	
	// 按逗号分割
	tableParts := strings.Split(tableClause, ",")
	
	for _, part := range tableParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// 提取表名（移除别名）
		words := strings.Fields(part)
		if len(words) > 0 {
			tableName := words[0]
			// 移除引号
			tableName = strings.Trim(tableName, "\"'`")
			if tableName != "" && !h.containsString(tables, tableName) {
				tables = append(tables, tableName)
			}
		}
	}
	
	return tables
}

// containsString 检查slice中是否包含指定的字符串
func (h *SQLHandler) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getUserIDFromContext 从JWT中间件上下文获取用户ID
func (h *SQLHandler) getUserIDFromContext(c *gin.Context) int64 {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		return 0
	}
	return userID
}