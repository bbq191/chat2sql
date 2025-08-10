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
	
	"chat2sql-go/internal/repository"
)

// SQLHandler SQL查询处理器
// 处理SQL执行、查询历史、语法验证等操作
type SQLHandler struct {
	queryRepo     repository.QueryHistoryRepository
	connectionRepo repository.ConnectionRepository
	logger        *zap.Logger
}

// NewSQLHandler 创建SQL处理器实例
func NewSQLHandler(
	queryRepo repository.QueryHistoryRepository,
	connectionRepo repository.ConnectionRepository,
	logger *zap.Logger,
) *SQLHandler {
	return &SQLHandler{
		queryRepo:      queryRepo,
		connectionRepo: connectionRepo,
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
	
	// TODO: 实际SQL执行逻辑（需要连接池管理器）
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
	
	// TODO: 根据参数调用不同的查询方法
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
	
	// TODO: 语法验证（可以集成SQL解析器）
	// TODO: 提取表名（可以使用SQL解析器）
	result.TablesUsed = h.extractTableNames(sql)
	
	return result
}

// executeSQL 执行SQL查询
// TODO: 集成实际的数据库执行器
func (h *SQLHandler) executeSQL(ctx context.Context, sql string, connection *repository.DatabaseConnection) *SQLExecutionResult {
	start := time.Now()
	
	// 模拟SQL执行
	time.Sleep(100 * time.Millisecond) // 模拟执行时间
	
	executionTime := int32(time.Since(start).Milliseconds())
	
	// TODO: 实际执行SQL并获取结果
	// 临时返回模拟结果
	return &SQLExecutionResult{
		ExecutionTime: executionTime,
		RowCount:      10, // 模拟结果行数
		Status:        string(repository.QuerySuccess),
		Data: []map[string]any{
			{"id": 1, "name": "用户1", "email": "user1@example.com"},
			{"id": 2, "name": "用户2", "email": "user2@example.com"},
		},
	}
}

// extractTableNames 提取SQL中的表名
// TODO: 集成专业的SQL解析器
func (h *SQLHandler) extractTableNames(sql string) []string {
	// 简单的表名提取逻辑
	upperSQL := strings.ToUpper(sql)
	tables := []string{}
	
	// 查找FROM关键词后的表名
	fromIndex := strings.Index(upperSQL, "FROM")
	if fromIndex != -1 {
		// 简化实现，仅提取第一个表名
		parts := strings.Fields(sql[fromIndex+4:])
		if len(parts) > 0 {
			tableName := strings.Trim(parts[0], " \t\n\r(),")
			tables = append(tables, tableName)
		}
	}
	
	return tables
}

// getUserIDFromContext 从上下文获取用户ID
// TODO: JWT中间件实现后从Token解析
func (h *SQLHandler) getUserIDFromContext(c *gin.Context) int64 {
	// 临时实现 - 从Header获取
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		return 0
	}
	
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0
	}
	
	return userID
}