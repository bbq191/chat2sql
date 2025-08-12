package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"chat2sql-go/internal/middleware"
	"chat2sql-go/internal/repository"
)

// ConnectionManagerInterface 连接管理器接口
type ConnectionManagerInterface interface {
	CreateConnection(ctx context.Context, connection *repository.DatabaseConnection) error
	UpdateConnection(ctx context.Context, connection *repository.DatabaseConnection) error
	TestConnection(ctx context.Context, connection *repository.DatabaseConnection) error
}

// ConnectionTestResult 连接测试结果结构
type ConnectionTestResult struct {
	Success      bool   `json:"success" example:"true"`
	Message      string `json:"message" example:"连接测试成功"`
	ResponseTime int32  `json:"response_time" example:"25"`
	Error        string `json:"error,omitempty"`
}

// ConnectionHandler 数据库连接处理器
// 处理数据库连接的创建、管理、测试等操作
type ConnectionHandler struct {
	connectionRepo    repository.ConnectionRepository
	schemaRepo        repository.SchemaRepository
	connectionManager ConnectionManagerInterface
	logger            *zap.Logger
}

// NewConnectionHandler 创建连接处理器实例
func NewConnectionHandler(
	connectionRepo repository.ConnectionRepository,
	schemaRepo repository.SchemaRepository,
	connectionManager ConnectionManagerInterface,
	logger *zap.Logger,
) *ConnectionHandler {
	return &ConnectionHandler{
		connectionRepo:    connectionRepo,
		schemaRepo:        schemaRepo,
		connectionManager: connectionManager,
		logger:            logger,
	}
}

// CreateConnectionRequest 创建连接请求结构
type CreateConnectionRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=100" example:"生产数据库"`
	Host         string `json:"host" binding:"required" example:"localhost"`
	Port         int32  `json:"port" binding:"required,min=1,max=65535" example:"5432"`
	DatabaseName string `json:"database_name" binding:"required,min=1,max=100" example:"production_db"`
	Username     string `json:"username" binding:"required,min=1,max=100" example:"db_user"`
	Password     string `json:"password" binding:"required,min=1,max=255" example:"secure_password"`
	DBType       string `json:"db_type" binding:"required,oneof=postgresql mysql sqlite oracle" example:"postgresql"`
}

// UpdateConnectionRequest 更新连接请求结构
type UpdateConnectionRequest struct {
	Name         string `json:"name" binding:"omitempty,min=1,max=100" example:"生产数据库"`
	Host         string `json:"host" binding:"omitempty" example:"localhost"`
	Port         int32  `json:"port" binding:"omitempty,min=1,max=65535" example:"5432"`
	DatabaseName string `json:"database_name" binding:"omitempty,min=1,max=100" example:"production_db"`
	Username     string `json:"username" binding:"omitempty,min=1,max=100" example:"db_user"`
	Password     string `json:"password" binding:"omitempty,min=1,max=255" example:"secure_password"`
}

// ConnectionResponse 连接响应结构
type ConnectionResponse struct {
	ID           int64     `json:"id" example:"1"`
	Name         string    `json:"name" example:"生产数据库"`
	Host         string    `json:"host" example:"localhost"`
	Port         int32     `json:"port" example:"5432"`
	DatabaseName string    `json:"database_name" example:"production_db"`
	Username     string    `json:"username" example:"db_user"`
	DBType       string    `json:"db_type" example:"postgresql"`
	Status       string    `json:"status" example:"active"`
	LastTested   *time.Time `json:"last_tested,omitempty" example:"2024-01-08T12:00:00Z"`
	CreateTime   time.Time `json:"create_time" example:"2024-01-08T10:00:00Z"`
	UpdateTime   time.Time `json:"update_time" example:"2024-01-08T11:00:00Z"`
}

// ConnectionListResponse 连接列表响应
type ConnectionListResponse struct {
	Connections []*ConnectionResponse `json:"connections"`
	Total       int64                 `json:"total" example:"3"`
}


// DatabaseSchemaResponse 数据库结构响应
type DatabaseSchemaResponse struct {
	ConnectionID int64        `json:"connection_id" example:"1"`
	Schemas      []*SchemaInfo `json:"schemas"`
	TableCount   int64        `json:"table_count" example:"15"`
	LastUpdated  time.Time    `json:"last_updated" example:"2024-01-08T12:00:00Z"`
}

// SchemaInfo 结构信息
type SchemaInfo struct {
	SchemaName string       `json:"schema_name" example:"public"`
	Tables     []*TableInfo `json:"tables"`
}

// TableInfo 表信息
type TableInfo struct {
	TableName    string        `json:"table_name" example:"users"`
	TableComment *string       `json:"table_comment,omitempty" example:"用户表"`
	Columns      []*ColumnInfo `json:"columns"`
}

// ColumnInfo 列信息
type ColumnInfo struct {
	ColumnName    string  `json:"column_name" example:"id"`
	DataType      string  `json:"data_type" example:"bigint"`
	IsNullable    bool    `json:"is_nullable" example:"false"`
	IsPrimaryKey  bool    `json:"is_primary_key" example:"true"`
	ColumnComment *string `json:"column_comment,omitempty" example:"主键ID"`
}

// CreateConnection 创建数据库连接
// @Summary 创建数据库连接
// @Description 创建新的数据库连接配置
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateConnectionRequest true "连接配置"
// @Success 201 {object} ConnectionResponse "创建成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 409 {object} ErrorResponse "连接名称已存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/connections [post]
func (h *ConnectionHandler) CreateConnection(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 检查连接名称是否已存在
	exists, err := h.connectionRepo.ExistsByUserAndName(c.Request.Context(), userID, req.Name)
	if err != nil {
		h.logger.Error("Failed to check connection name existence",
			zap.Error(err),
			zap.Int64("user_id", userID),
			zap.String("name", req.Name))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "数据库查询失败",
		})
		return
	}
	
	if exists {
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:    "CONNECTION_NAME_EXISTS",
			Message: "连接名称已存在",
		})
		return
	}
	
	// 创建连接配置（密码暂时以明文形式传递，由ConnectionManager加密）
	connection := &repository.DatabaseConnection{
		UserID:            userID,
		Name:              req.Name,
		Host:              req.Host,
		Port:              req.Port,
		DatabaseName:      req.DatabaseName,
		Username:          req.Username,
		PasswordEncrypted: req.Password, // 临时存储明文密码
		DBType:            req.DBType,
		Status:            string(repository.ConnectionActive),
	}
	
	// 使用ConnectionManager创建连接（包含密码加密和连接测试）
	if err := h.connectionManager.CreateConnection(c.Request.Context(), connection); err != nil {
		h.logger.Error("Failed to create connection",
			zap.Error(err),
			zap.Int64("user_id", userID),
			zap.String("host", req.Host),
			zap.String("database", req.DatabaseName))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "CREATE_CONNECTION_FAILED",
			Message: fmt.Sprintf("创建连接失败: %v", err),
		})
		return
	}
	
	h.logger.Info("Database connection created",
		zap.Int64("user_id", userID),
		zap.Int64("connection_id", connection.ID),
		zap.String("name", connection.Name))
	
	response := h.toConnectionResponse(connection)
	c.JSON(http.StatusCreated, response)
}

// ListConnections 获取连接列表
// @Summary 获取数据库连接列表
// @Description 获取当前用户的所有数据库连接
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ConnectionListResponse "获取成功"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/connections [get]
func (h *ConnectionHandler) ListConnections(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	// 获取用户的所有连接
	connections, err := h.connectionRepo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list connections",
			zap.Error(err),
			zap.Int64("user_id", userID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "获取连接列表失败",
		})
		return
	}
	
	// 转换为响应格式
	items := make([]*ConnectionResponse, len(connections))
	for i, conn := range connections {
		items[i] = h.toConnectionResponse(conn)
	}
	
	response := &ConnectionListResponse{
		Connections: items,
		Total:       int64(len(connections)),
	}
	
	c.JSON(http.StatusOK, response)
}

// GetConnection 获取连接详情
// @Summary 获取连接详情
// @Description 根据连接ID获取详细信息
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "连接ID"
// @Success 200 {object} ConnectionResponse "获取成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "连接不存在"
// @Router /api/v1/connections/{id} [get]
func (h *ConnectionHandler) GetConnection(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	connectionID, err := h.parseConnectionID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_CONNECTION_ID",
			Message: "无效的连接ID",
		})
		return
	}
	
	connection, err := h.getConnectionWithPermissionCheck(c.Request.Context(), connectionID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "CONNECTION_NOT_FOUND",
			Message: "连接不存在或无权访问",
		})
		return
	}
	
	response := h.toConnectionResponse(connection)
	c.JSON(http.StatusOK, response)
}

// UpdateConnection 更新连接配置
// @Summary 更新连接配置
// @Description 更新指定数据库连接的配置信息
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "连接ID"
// @Param request body UpdateConnectionRequest true "更新信息"
// @Success 200 {object} ConnectionResponse "更新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "连接不存在"
// @Router /api/v1/connections/{id} [put]
func (h *ConnectionHandler) UpdateConnection(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	connectionID, err := h.parseConnectionID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_CONNECTION_ID",
			Message: "无效的连接ID",
		})
		return
	}
	
	var req UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "请求参数格式错误",
			Details: err.Error(),
		})
		return
	}
	
	// 获取现有连接并检查权限
	connection, err := h.getConnectionWithPermissionCheck(c.Request.Context(), connectionID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "CONNECTION_NOT_FOUND",
			Message: "连接不存在或无权访问",
		})
		return
	}
	
	// 更新连接信息
	if req.Name != "" {
		connection.Name = req.Name
	}
	if req.Host != "" {
		connection.Host = req.Host
	}
	if req.Port > 0 {
		connection.Port = req.Port
	}
	if req.DatabaseName != "" {
		connection.DatabaseName = req.DatabaseName
	}
	if req.Username != "" {
		connection.Username = req.Username
	}
	if req.Password != "" {
		connection.PasswordEncrypted = req.Password // 临时存储明文密码
	}
	
	// 如果连接信息发生变化，通过ConnectionManager更新（包含加密和测试）
	if req.Host != "" || req.Port > 0 || req.Username != "" || req.Password != "" {
		if err := h.connectionManager.UpdateConnection(c.Request.Context(), connection); err != nil {
			h.logger.Error("Failed to update connection",
				zap.Error(err),
				zap.Int64("connection_id", connectionID))
			
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    "UPDATE_CONNECTION_FAILED",
				Message: fmt.Sprintf("更新连接失败: %v", err),
			})
			return
		}
	} else {
		// 只有基础信息更新，直接使用Repository
		if err := h.connectionRepo.Update(c.Request.Context(), connection); err != nil {
			h.logger.Error("Failed to update connection",
				zap.Error(err),
				zap.Int64("connection_id", connectionID))
			
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    "UPDATE_CONNECTION_FAILED",
				Message: "更新连接失败",
			})
			return
		}
	}
	
	h.logger.Info("Connection updated successfully",
		zap.Int64("user_id", userID),
		zap.Int64("connection_id", connectionID))
	
	response := h.toConnectionResponse(connection)
	c.JSON(http.StatusOK, response)
}

// DeleteConnection 删除连接
// @Summary 删除数据库连接
// @Description 软删除指定的数据库连接
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "连接ID"
// @Success 200 {object} SuccessResponse "删除成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "连接不存在"
// @Router /api/v1/connections/{id} [delete]
func (h *ConnectionHandler) DeleteConnection(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	connectionID, err := h.parseConnectionID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_CONNECTION_ID",
			Message: "无效的连接ID",
		})
		return
	}
	
	// 检查连接是否存在和权限
	_, err = h.getConnectionWithPermissionCheck(c.Request.Context(), connectionID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "CONNECTION_NOT_FOUND",
			Message: "连接不存在或无权访问",
		})
		return
	}
	
	// 软删除连接
	if err := h.connectionRepo.Delete(c.Request.Context(), connectionID); err != nil {
		h.logger.Error("Failed to delete connection",
			zap.Error(err),
			zap.Int64("connection_id", connectionID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DELETE_CONNECTION_FAILED",
			Message: "删除连接失败",
		})
		return
	}
	
	h.logger.Info("Connection deleted successfully",
		zap.Int64("user_id", userID),
		zap.Int64("connection_id", connectionID))
	
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    "CONNECTION_DELETED",
		Message: "连接删除成功",
	})
}

// TestConnection 测试连接
// @Summary 测试数据库连接
// @Description 测试指定数据库连接的可用性
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "连接ID"
// @Success 200 {object} ConnectionTestResult "测试完成"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "连接不存在"
// @Router /api/v1/connections/{id}/test [post]
func (h *ConnectionHandler) TestConnection(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	connectionID, err := h.parseConnectionID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_CONNECTION_ID",
			Message: "无效的连接ID",
		})
		return
	}
	
	// 获取连接并检查权限
	connection, err := h.getConnectionWithPermissionCheck(c.Request.Context(), connectionID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "CONNECTION_NOT_FOUND",
			Message: "连接不存在或无权访问",
		})
		return
	}
	
	// 执行连接测试
	start := time.Now()
	err = h.connectionManager.TestConnection(c.Request.Context(), connection)
	responseTime := int32(time.Since(start).Milliseconds())
	
	var result *ConnectionTestResult
	if err != nil {
		result = &ConnectionTestResult{
			Success:      false,
			Message:      "连接测试失败",
			ResponseTime: responseTime,
			Error:        err.Error(),
		}
		
		// 更新连接状态为错误
		if updateErr := h.connectionRepo.UpdateStatus(c.Request.Context(), connectionID, repository.ConnectionError); updateErr != nil {
			h.logger.Warn("Failed to update connection status after test",
				zap.Error(updateErr),
				zap.Int64("connection_id", connectionID))
		}
	} else {
		result = &ConnectionTestResult{
			Success:      true,
			Message:      "连接测试成功",
			ResponseTime: responseTime,
		}
		
		// 更新连接状态为正常
		if updateErr := h.connectionRepo.UpdateStatus(c.Request.Context(), connectionID, repository.ConnectionActive); updateErr != nil {
			h.logger.Warn("Failed to update connection status after test",
				zap.Error(updateErr),
				zap.Int64("connection_id", connectionID))
		}
	}
	
	// 更新最后测试时间
	now := time.Now()
	if err := h.connectionRepo.UpdateLastTested(c.Request.Context(), connectionID, now); err != nil {
		h.logger.Warn("Failed to update last tested time",
			zap.Error(err),
			zap.Int64("connection_id", connectionID))
	}
	
	h.logger.Info("Connection test completed",
		zap.Int64("user_id", userID),
		zap.Int64("connection_id", connectionID),
		zap.Bool("success", result.Success))
	
	c.JSON(http.StatusOK, result)
}

// GetSchema 获取数据库结构
// @Summary 获取数据库结构
// @Description 获取指定连接的数据库表结构信息
// @Tags 数据库连接
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "连接ID"
// @Success 200 {object} DatabaseSchemaResponse "获取成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权访问"
// @Failure 404 {object} ErrorResponse "连接不存在"
// @Router /api/v1/connections/{id}/schema [get]
func (h *ConnectionHandler) GetSchema(c *gin.Context) {
	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "未授权访问",
		})
		return
	}
	
	connectionID, err := h.parseConnectionID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_CONNECTION_ID",
			Message: "无效的连接ID",
		})
		return
	}
	
	// 检查连接权限
	_, err = h.getConnectionWithPermissionCheck(c.Request.Context(), connectionID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "CONNECTION_NOT_FOUND",
			Message: "连接不存在或无权访问",
		})
		return
	}
	
	// 获取元数据
	schemas, err := h.schemaRepo.ListByConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.logger.Error("Failed to get schema metadata",
			zap.Error(err),
			zap.Int64("connection_id", connectionID))
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    "DATABASE_ERROR",
			Message: "获取数据库结构失败",
		})
		return
	}
	
	// 组织结构化的响应数据
	response := &DatabaseSchemaResponse{
		ConnectionID: connectionID,
		Schemas:      h.organizeSchemaData(schemas),
		TableCount:   h.countTables(schemas),
		LastUpdated:  time.Now(), // 注意: 未来可从metadata获取实际更新时间
	}
	
	c.JSON(http.StatusOK, response)
}

// 辅助方法

// parseConnectionID 解析连接ID参数
func (h *ConnectionHandler) parseConnectionID(c *gin.Context) (int64, error) {
	idStr := c.Param("id")
	return strconv.ParseInt(idStr, 10, 64)
}

// getConnectionWithPermissionCheck 获取连接并检查权限
func (h *ConnectionHandler) getConnectionWithPermissionCheck(ctx context.Context, connectionID, userID int64) (*repository.DatabaseConnection, error) {
	connection, err := h.connectionRepo.GetByID(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	
	// 检查权限
	if connection.UserID != userID {
		return nil, fmt.Errorf("无权访问该连接")
	}
	
	return connection, nil
}

// toConnectionResponse 转换为连接响应格式
func (h *ConnectionHandler) toConnectionResponse(conn *repository.DatabaseConnection) *ConnectionResponse {
	return &ConnectionResponse{
		ID:           conn.ID,
		Name:         conn.Name,
		Host:         conn.Host,
		Port:         conn.Port,
		DatabaseName: conn.DatabaseName,
		Username:     conn.Username,
		DBType:       conn.DBType,
		Status:       conn.Status,
		LastTested:   conn.LastTested,
		CreateTime:   conn.CreateTime,
		UpdateTime:   conn.UpdateTime,
	}
}


// organizeSchemaData 组织结构化的Schema数据
// 注意: 当前为基础实现，未来可完善数据组织逻辑
func (h *ConnectionHandler) organizeSchemaData(schemas []*repository.SchemaMetadata) []*SchemaInfo {
	// 临时实现 - 简单分组
	schemaMap := make(map[string][]*repository.SchemaMetadata)
	for _, schema := range schemas {
		schemaMap[schema.SchemaName] = append(schemaMap[schema.SchemaName], schema)
	}
	
	result := []*SchemaInfo{}
	for schemaName, schemaTables := range schemaMap {
		schemaInfo := &SchemaInfo{
			SchemaName: schemaName,
			Tables:     []*TableInfo{},
		}
		
		// 按表名分组并组织列信息
		tableMap := make(map[string][]*repository.SchemaMetadata)
		for _, schema := range schemaTables {
			tableMap[schema.TableName] = append(tableMap[schema.TableName], schema)
		}
		
		for tableName, tableColumns := range tableMap {
			tableInfo := &TableInfo{
				TableName: tableName,
				Columns:   []*ColumnInfo{},
			}
			
			for _, column := range tableColumns {
				columnInfo := &ColumnInfo{
					ColumnName:    column.ColumnName,
					DataType:      column.DataType,
					IsNullable:    column.IsNullable,
					IsPrimaryKey:  column.IsPrimaryKey,
					ColumnComment: column.ColumnComment,
				}
				tableInfo.Columns = append(tableInfo.Columns, columnInfo)
			}
			
			// 设置表注释
			if len(tableColumns) > 0 {
				tableInfo.TableComment = tableColumns[0].TableComment
			}
			
			schemaInfo.Tables = append(schemaInfo.Tables, tableInfo)
		}
		
		result = append(result, schemaInfo)
	}
	
	return result
}

// countTables 统计表数量
func (h *ConnectionHandler) countTables(schemas []*repository.SchemaMetadata) int64 {
	tableSet := make(map[string]bool)
	for _, schema := range schemas {
		tableKey := schema.SchemaName + "." + schema.TableName
		tableSet[tableKey] = true
	}
	return int64(len(tableSet))
}

// getUserIDFromContext 从JWT中间件上下文获取用户ID
func (h *ConnectionHandler) getUserIDFromContext(c *gin.Context) int64 {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		return 0
	}
	return userID
}