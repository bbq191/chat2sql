package repository

import (
	"context"
	"time"
)

// Repository 主Repository接口，聚合所有子Repository
// 提供统一的数据访问接口，支持事务管理和批量操作
type Repository interface {
	UserRepo() UserRepository
	QueryHistoryRepo() QueryHistoryRepository
	ConnectionRepo() ConnectionRepository
	SchemaRepo() SchemaRepository
	FeedbackRepo() FeedbackRepository
	
	// 事务管理
	BeginTx(ctx context.Context) (TxRepository, error)
	Close() error
	HealthCheck(ctx context.Context) error
}

// TxRepository 事务Repository接口
// 在事务上下文中执行所有Repository操作
type TxRepository interface {
	UserRepo() UserRepository
	QueryHistoryRepo() QueryHistoryRepository
	ConnectionRepo() ConnectionRepository
	SchemaRepo() SchemaRepository
	FeedbackRepo() FeedbackRepository
	
	Commit() error
	Rollback() error
}

// UserRepository 用户Repository接口
// 提供用户管理的所有数据库操作，支持RBAC权限控制
type UserRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id int64) error // 软删除
	
	// 查询操作
	List(ctx context.Context, limit, offset int) ([]*User, error)
	ListByRole(ctx context.Context, role UserRole, limit, offset int) ([]*User, error)
	ListByStatus(ctx context.Context, status UserStatus, limit, offset int) ([]*User, error)
	Count(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status UserStatus) (int64, error)
	
	// 认证相关
	ValidateCredentials(ctx context.Context, username, passwordHash string) (*User, error)
	UpdatePassword(ctx context.Context, userID int64, newPasswordHash string) error
	UpdateLastLogin(ctx context.Context, userID int64, loginTime time.Time) error
	
	// 状态管理
	UpdateStatus(ctx context.Context, userID int64, status UserStatus) error
	BatchUpdateStatus(ctx context.Context, userIDs []int64, status UserStatus) error
	
	// 检查操作
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// QueryHistoryRepository 查询历史Repository接口
// 提供SQL查询历史的管理，支持查询分析和性能统计
type QueryHistoryRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, query *QueryHistory) error
	GetByID(ctx context.Context, id int64) (*QueryHistory, error)
	Update(ctx context.Context, query *QueryHistory) error
	Delete(ctx context.Context, id int64) error // 软删除
	
	// 查询操作
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*QueryHistory, error)
	ListByConnection(ctx context.Context, connectionID int64, limit, offset int) ([]*QueryHistory, error)
	ListByStatus(ctx context.Context, status QueryStatus, limit, offset int) ([]*QueryHistory, error)
	ListRecent(ctx context.Context, userID int64, hours int, limit int) ([]*QueryHistory, error)
	
	// 统计操作
	CountByUser(ctx context.Context, userID int64) (int64, error)
	CountByStatus(ctx context.Context, status QueryStatus) (int64, error)
	GetExecutionStats(ctx context.Context, userID int64, days int) (*QueryExecutionStats, error)
	GetPopularQueries(ctx context.Context, limit int, days int) ([]*PopularQuery, error)
	GetSlowQueries(ctx context.Context, minExecutionTime int32, limit int) ([]*QueryHistory, error)
	
	// 搜索操作
	SearchByNaturalQuery(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*QueryHistory, error)
	SearchBySQL(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*QueryHistory, error)
	
	// 批量操作
	BatchUpdateStatus(ctx context.Context, queryIDs []int64, status QueryStatus) error
	CleanupOldQueries(ctx context.Context, beforeDate time.Time) (int64, error)
}

// ConnectionRepository 数据库连接Repository接口
// 提供多数据库连接管理，支持连接测试和状态监控
type ConnectionRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, conn *DatabaseConnection) error
	GetByID(ctx context.Context, id int64) (*DatabaseConnection, error)
	Update(ctx context.Context, conn *DatabaseConnection) error
	Delete(ctx context.Context, id int64) error // 软删除
	
	// 查询操作
	ListByUser(ctx context.Context, userID int64) ([]*DatabaseConnection, error)
	ListByType(ctx context.Context, dbType DatabaseType) ([]*DatabaseConnection, error)
	ListByStatus(ctx context.Context, status ConnectionStatus) ([]*DatabaseConnection, error)
	GetByUserAndName(ctx context.Context, userID int64, name string) (*DatabaseConnection, error)
	
	// 统计操作
	CountByUser(ctx context.Context, userID int64) (int64, error)
	CountByStatus(ctx context.Context, status ConnectionStatus) (int64, error)
	CountByType(ctx context.Context, dbType DatabaseType) (int64, error)
	
	// 状态管理
	UpdateStatus(ctx context.Context, connectionID int64, status ConnectionStatus) error
	UpdateLastTested(ctx context.Context, connectionID int64, testTime time.Time) error
	BatchUpdateStatus(ctx context.Context, connectionIDs []int64, status ConnectionStatus) error
	
	// 检查操作
	ExistsByUserAndName(ctx context.Context, userID int64, name string) (bool, error)
	GetActiveConnections(ctx context.Context) ([]*DatabaseConnection, error)
}

// SchemaRepository 数据库元数据Repository接口
// 提供数据库表结构元数据管理，支持结构缓存和更新
type SchemaRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, schema *SchemaMetadata) error
	GetByID(ctx context.Context, id int64) (*SchemaMetadata, error)
	Update(ctx context.Context, schema *SchemaMetadata) error
	Delete(ctx context.Context, id int64) error // 软删除
	
	// 查询操作
	ListByConnection(ctx context.Context, connectionID int64) ([]*SchemaMetadata, error)
	ListByTable(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*SchemaMetadata, error)
	GetTableStructure(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*SchemaMetadata, error)
	ListTables(ctx context.Context, connectionID int64, schemaName string) ([]string, error)
	ListSchemas(ctx context.Context, connectionID int64) ([]string, error)
	
	// 批量操作
	BatchCreate(ctx context.Context, schemas []*SchemaMetadata) error
	BatchDelete(ctx context.Context, connectionID int64) error // 删除指定连接的所有元数据
	RefreshConnectionMetadata(ctx context.Context, connectionID int64, schemas []*SchemaMetadata) error
	
	// 搜索操作
	SearchTables(ctx context.Context, connectionID int64, keyword string) ([]*TableInfo, error)
	SearchColumns(ctx context.Context, connectionID int64, keyword string) ([]*ColumnInfo, error)
	GetRelatedTables(ctx context.Context, connectionID int64, tableName string) ([]*TableRelation, error)
	
	// 统计操作
	CountByConnection(ctx context.Context, connectionID int64) (int64, error)
	GetTableCount(ctx context.Context, connectionID int64) (int64, error)
	GetColumnCount(ctx context.Context, connectionID int64) (int64, error)
}

// 统计和分析相关的数据结构

// QueryExecutionStats 查询执行统计信息
type QueryExecutionStats struct {
	UserID              int64   `json:"user_id"`
	TotalQueries        int64   `json:"total_queries"`        // 总查询数
	SuccessfulQueries   int64   `json:"successful_queries"`   // 成功查询数
	FailedQueries       int64   `json:"failed_queries"`       // 失败查询数
	AverageExecutionTime float64 `json:"avg_execution_time"`   // 平均执行时间(毫秒)
	TotalExecutionTime  int64   `json:"total_execution_time"` // 总执行时间(毫秒)
	SuccessRate         float64 `json:"success_rate"`         // 成功率
}

// PopularQuery 热门查询信息
type PopularQuery struct {
	NaturalQuery string `json:"natural_query"` // 自然语言查询
	QueryCount   int64  `json:"query_count"`   // 查询次数
	SuccessRate  float64 `json:"success_rate"`  // 成功率
	AvgExecTime  float64 `json:"avg_exec_time"` // 平均执行时间
}

// TableInfo 表信息
type TableInfo struct {
	SchemaName   string `json:"schema_name"`   // 模式名
	TableName    string `json:"table_name"`    // 表名
	TableComment string `json:"table_comment"` // 表注释
	ColumnCount  int32  `json:"column_count"`  // 列数
}

// ColumnInfo 列信息
type ColumnInfo struct {
	SchemaName    string  `json:"schema_name"`    // 模式名
	TableName     string  `json:"table_name"`     // 表名
	ColumnName    string  `json:"column_name"`    // 列名
	DataType      string  `json:"data_type"`      // 数据类型
	IsNullable    bool    `json:"is_nullable"`    // 是否可空
	IsPrimaryKey  bool    `json:"is_primary_key"` // 是否主键
	ColumnComment *string `json:"column_comment"` // 列注释
}

// TableRelation 表关系信息
type TableRelation struct {
	FromTable    string `json:"from_table"`    // 源表
	ToTable      string `json:"to_table"`      // 目标表
	FromColumn   string `json:"from_column"`   // 源列
	ToColumn     string `json:"to_column"`     // 目标列
	RelationType string `json:"relation_type"` // 关系类型：foreign_key等
}

// PaginationParams 分页参数
type PaginationParams struct {
	Limit  int `json:"limit"`  // 每页大小
	Offset int `json:"offset"` // 偏移量
}

// Validate 验证分页参数
func (p *PaginationParams) Validate() error {
	if p.Limit <= 0 {
		p.Limit = 20 // 默认每页20条
	}
	if p.Limit > 1000 {
		p.Limit = 1000 // 最大每页1000条
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// GetPage 计算页码（从1开始）
func (p *PaginationParams) GetPage() int {
	return (p.Offset / p.Limit) + 1
}

// FeedbackRepository 用户反馈Repository接口
// 提供用户反馈的管理，支持统计分析和准确率监控
type FeedbackRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, feedback *Feedback) error
	GetByID(ctx context.Context, id int64) (*Feedback, error)
	GetByQueryID(ctx context.Context, queryID string) (*Feedback, error)
	Update(ctx context.Context, feedback *Feedback) error
	Delete(ctx context.Context, id int64) error // 软删除
	
	// 查询操作
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Feedback, error)
	ListByTimeRange(ctx context.Context, startTime, endTime time.Time, limit, offset int) ([]*Feedback, error)
	ListByCorrectness(ctx context.Context, isCorrect bool, limit, offset int) ([]*Feedback, error)
	ListByRating(ctx context.Context, minRating int, limit, offset int) ([]*Feedback, error)
	ListByCategory(ctx context.Context, category string, limit, offset int) ([]*Feedback, error)
	ListByModel(ctx context.Context, model string, limit, offset int) ([]*Feedback, error)
	
	// 统计操作
	CountByUser(ctx context.Context, userID int64) (int64, error)
	CountByCorrectness(ctx context.Context, isCorrect bool) (int64, error)
	CountByTimeRange(ctx context.Context, startTime, endTime time.Time) (int64, error)
	GetAccuracyStats(ctx context.Context, startTime, endTime time.Time) (*AccuracyStats, error)
	GetRatingStats(ctx context.Context, startTime, endTime time.Time) (*RatingStats, error)
	GetCategoryStats(ctx context.Context, startTime, endTime time.Time) ([]*CategoryFeedbackStats, error)
	GetModelStats(ctx context.Context, startTime, endTime time.Time) ([]*ModelFeedbackStats, error)
	GetUserStats(ctx context.Context, limit int) ([]*UserFeedbackStats, error)
	GetErrorStats(ctx context.Context, startTime, endTime time.Time, limit int) ([]*ErrorStats, error)
	
	// 搜索操作
	SearchByQuery(ctx context.Context, keyword string, limit, offset int) ([]*Feedback, error)
	SearchByFeedback(ctx context.Context, keyword string, limit, offset int) ([]*Feedback, error)
	
	// 批量操作
	BatchCreate(ctx context.Context, feedbacks []*Feedback) error
	BatchUpdateProcessed(ctx context.Context, feedbackIDs []int64) error
	CleanupOldFeedbacks(ctx context.Context, beforeDate time.Time) (int64, error)
}

// 反馈统计相关的数据结构

// AccuracyStats 准确率统计
type AccuracyStats struct {
	TotalQueries    int64   `json:"total_queries"`    // 总查询数
	CorrectQueries  int64   `json:"correct_queries"`  // 正确查询数
	AccuracyRate    float64 `json:"accuracy_rate"`    // 准确率
	ErrorRate       float64 `json:"error_rate"`       // 错误率
	ImprovementRate float64 `json:"improvement_rate"` // 相比上期改进率
}

// RatingStats 评分统计
type RatingStats struct {
	TotalRatings int64            `json:"total_ratings"`  // 总评分数
	AverageRating float64         `json:"average_rating"` // 平均评分
	RatingDistribution map[int]int64 `json:"rating_distribution"` // 评分分布 {评分: 数量}
}

// CategoryFeedbackStats 类别反馈统计
type CategoryFeedbackStats struct {
	Category       string  `json:"category"`        // 查询类别
	TotalQueries   int64   `json:"total_queries"`   // 总查询数
	CorrectQueries int64   `json:"correct_queries"` // 正确查询数
	AccuracyRate   float64 `json:"accuracy_rate"`   // 准确率
	AverageRating  float64 `json:"average_rating"`  // 平均评分
}

// ModelFeedbackStats 模型反馈统计
type ModelFeedbackStats struct {
	ModelName      string  `json:"model_name"`      // 模型名称
	TotalQueries   int64   `json:"total_queries"`   // 总查询数
	CorrectQueries int64   `json:"correct_queries"` // 正确查询数
	AccuracyRate   float64 `json:"accuracy_rate"`   // 准确率
	AverageRating  float64 `json:"average_rating"`  // 平均评分
	AvgTokensUsed  float64 `json:"avg_tokens_used"` // 平均Token使用数
	AvgProcessTime float64 `json:"avg_process_time"` // 平均处理时间（毫秒）
}

// UserFeedbackStats 用户反馈统计
type UserFeedbackStats struct {
	UserID         int64   `json:"user_id"`         // 用户ID
	TotalQueries   int64   `json:"total_queries"`   // 总查询数
	CorrectQueries int64   `json:"correct_queries"` // 正确查询数
	AccuracyRate   float64 `json:"accuracy_rate"`   // 准确率
	AverageRating  float64 `json:"average_rating"`  // 平均评分
	FeedbackCount  int64   `json:"feedback_count"`  // 反馈数量
	LastFeedback   *time.Time `json:"last_feedback"` // 最后反馈时间
}

// ErrorStats 错误统计
type ErrorStats struct {
	ErrorType    string `json:"error_type"`    // 错误类型
	ErrorCount   int64  `json:"error_count"`   // 错误次数
	ExampleQuery string `json:"example_query"` // 示例查询
	ExampleError string `json:"example_error"` // 示例错误信息
}