package repository

import (
	"time"
)

// BaseModel 所有数据模型的基础结构
// 包含统一的基础字段：创建信息、更新信息、软删除标记
type BaseModel struct {
	ID         int64     `json:"id" db:"id"`                   // 主键ID，自增长整型
	CreateBy   *int64    `json:"create_by" db:"create_by"`     // 创建者ID，可为空（系统创建的记录）
	CreateTime time.Time `json:"create_time" db:"create_time"` // 创建时间，使用UTC时区
	UpdateBy   *int64    `json:"update_by" db:"update_by"`     // 最后更新者ID，可为空
	UpdateTime time.Time `json:"update_time" db:"update_time"` // 最后更新时间，使用UTC时区
	IsDeleted  bool      `json:"is_deleted" db:"is_deleted"`   // 软删除标记，false=正常，true=已删除
}

// User 用户模型
// 支持基于角色的权限控制(RBAC)，包含用户基本信息和状态管理
type User struct {
	BaseModel
	Username      string     `json:"username" db:"username"`               // 用户名，唯一，3-50字符
	Email         string     `json:"email" db:"email"`                     // 邮箱地址，唯一，用于登录和通知
	PasswordHash  string     `json:"-" db:"password_hash"`                 // 密码哈希，使用bcrypt加密，不返回给前端
	Role          string     `json:"role" db:"role"`                       // 用户角色：user/admin/manager
	Status        string     `json:"status" db:"status"`                   // 用户状态：active/inactive/locked
	LastLoginTime *time.Time `json:"last_login_time" db:"last_login_time"` // 最后登录时间
}

// QueryHistory SQL查询历史记录
// 记录用户的自然语言查询和AI生成的SQL语句，支持查询分析和优化
type QueryHistory struct {
	BaseModel
	UserID        int64   `json:"user_id" db:"user_id"`               // 查询用户ID，外键关联users表
	NaturalQuery  string  `json:"natural_query" db:"natural_query"`   // 用户输入的自然语言查询
	GeneratedSQL  string  `json:"generated_sql" db:"generated_sql"`   // AI生成的SQL语句
	SQLHash       string  `json:"sql_hash" db:"sql_hash"`             // SQL语句SHA-256哈希，用于去重和缓存
	ExecutionTime *int32  `json:"execution_time" db:"execution_time"` // SQL执行时间，单位毫秒，可为空
	ResultRows    *int32  `json:"result_rows" db:"result_rows"`       // 查询结果行数，可为空
	Status        string  `json:"status" db:"status"`                 // 执行状态：pending/success/error/timeout
	ErrorMessage  *string `json:"error_message" db:"error_message"`   // 错误信息，执行失败时记录
	ConnectionID  *int64  `json:"connection_id" db:"connection_id"`   // 使用的数据库连接ID，可为空
}

// DatabaseConnection 数据库连接配置
// 支持多数据库连接管理，密码加密存储，连接状态监控
type DatabaseConnection struct {
	BaseModel
	UserID            int64      `json:"user_id" db:"user_id"`                       // 连接所属用户ID
	Name              string     `json:"name" db:"name"`                             // 连接名称，用户自定义
	Host              string     `json:"host" db:"host"`                             // 数据库主机地址
	Port              int32      `json:"port" db:"port"`                             // 数据库端口号
	DatabaseName      string     `json:"database_name" db:"database_name"`           // 数据库名称
	Username          string     `json:"username" db:"username"`                     // 数据库用户名
	PasswordEncrypted string     `json:"-" db:"password_encrypted"`                 // AES加密存储的密码，不返回给前端
	DBType            string     `json:"db_type" db:"db_type"`                       // 数据库类型：postgresql/mysql/sqlite/oracle
	Status            string     `json:"status" db:"status"`                         // 连接状态：active/inactive/error
	LastTested        *time.Time `json:"last_tested" db:"last_tested"`               // 最后测试连接时间
}

// SchemaMetadata 数据库表结构元数据
// 缓存目标数据库的表结构信息，用于AI模型理解数据库结构
type SchemaMetadata struct {
	BaseModel
	ConnectionID     int64   `json:"connection_id" db:"connection_id"`           // 关联的数据库连接ID
	SchemaName       string  `json:"schema_name" db:"schema_name"`               // 模式名（如PostgreSQL的schema）
	TableName        string  `json:"table_name" db:"table_name"`                 // 表名
	ColumnName       string  `json:"column_name" db:"column_name"`               // 列名
	DataType         string  `json:"data_type" db:"data_type"`                   // 数据类型（如varchar、int等）
	IsNullable       bool    `json:"is_nullable" db:"is_nullable"`               // 是否允许NULL值
	ColumnDefault    *string `json:"column_default" db:"column_default"`         // 列默认值
	IsPrimaryKey     bool    `json:"is_primary_key" db:"is_primary_key"`         // 是否为主键
	IsForeignKey     bool    `json:"is_foreign_key" db:"is_foreign_key"`         // 是否为外键
	ForeignTable     *string `json:"foreign_table" db:"foreign_table"`           // 外键引用的表名
	ForeignColumn    *string `json:"foreign_column" db:"foreign_column"`         // 外键引用的列名
	TableComment     *string `json:"table_comment" db:"table_comment"`           // 表注释/说明
	ColumnComment    *string `json:"column_comment" db:"column_comment"`         // 列注释/说明
	OrdinalPosition  int32   `json:"ordinal_position" db:"ordinal_position"`     // 列在表中的位置序号
}

// UserRole 用户角色枚举
type UserRole string

const (
	RoleUser    UserRole = "user"    // 普通用户：可以执行查询，管理自己的连接
	RoleManager UserRole = "manager" // 管理员：可以查看团队查询历史，管理团队连接
	RoleAdmin   UserRole = "admin"   // 系统管理员：完全访问权限
)

// UserStatus 用户状态枚举
type UserStatus string

const (
	StatusActive   UserStatus = "active"   // 活跃状态：正常使用
	StatusInactive UserStatus = "inactive" // 非活跃状态：暂时禁用
	StatusLocked   UserStatus = "locked"   // 锁定状态：因安全原因锁定
)

// QueryStatus 查询状态枚举
type QueryStatus string

const (
	QueryPending QueryStatus = "pending" // 等待执行
	QuerySuccess QueryStatus = "success" // 执行成功
	QueryError   QueryStatus = "error"   // 执行失败
	QueryTimeout QueryStatus = "timeout" // 执行超时
)

// ConnectionStatus 连接状态枚举
type ConnectionStatus string

const (
	ConnectionActive   ConnectionStatus = "active"   // 连接正常
	ConnectionInactive ConnectionStatus = "inactive" // 连接未激活
	ConnectionError    ConnectionStatus = "error"    // 连接异常
)

// DatabaseType 数据库类型枚举
type DatabaseType string

const (
	DBTypePostgreSQL DatabaseType = "postgresql" // PostgreSQL数据库
	DBTypeMySQL      DatabaseType = "mysql"      // MySQL数据库
	DBTypeSQLite     DatabaseType = "sqlite"     // SQLite数据库
	DBTypeOracle     DatabaseType = "oracle"     // Oracle数据库
)

// IsValidRole 验证用户角色是否有效
func (r UserRole) IsValid() bool {
	return r == RoleUser || r == RoleManager || r == RoleAdmin
}

// IsValidStatus 验证用户状态是否有效
func (s UserStatus) IsValid() bool {
	return s == StatusActive || s == StatusInactive || s == StatusLocked
}

// IsValidQueryStatus 验证查询状态是否有效
func (s QueryStatus) IsValid() bool {
	return s == QueryPending || s == QuerySuccess || s == QueryError || s == QueryTimeout
}

// IsValidConnectionStatus 验证连接状态是否有效
func (s ConnectionStatus) IsValid() bool {
	return s == ConnectionActive || s == ConnectionInactive || s == ConnectionError
}

// IsValidDatabaseType 验证数据库类型是否有效
func (t DatabaseType) IsValid() bool {
	return t == DBTypePostgreSQL || t == DBTypeMySQL || t == DBTypeSQLite || t == DBTypeOracle
}

// HasPermission 检查用户是否有指定权限
// 基于角色的权限检查，支持层级权限管理
func (u *User) HasPermission(permission string) bool {
	// 管理员拥有所有权限
	if u.Role == string(RoleAdmin) {
		return true
	}
	
	// 根据不同权限类型进行检查
	switch permission {
	case "query:execute":
		return u.Role == string(RoleUser) || u.Role == string(RoleManager)
	case "connection:manage":
		return u.Role == string(RoleUser) || u.Role == string(RoleManager)
	case "history:view_team":
		return u.Role == string(RoleManager)
	case "user:manage":
		return u.Role == string(RoleAdmin)
	default:
		return false
	}
}

// IsActive 检查用户是否处于活跃状态
func (u *User) IsActive() bool {
	return u.Status == string(StatusActive) && !u.IsDeleted
}

// IsQuerySuccessful 检查查询是否执行成功
func (qh *QueryHistory) IsQuerySuccessful() bool {
	return qh.Status == string(QuerySuccess)
}

// IsConnectionHealthy 检查数据库连接是否健康
func (dc *DatabaseConnection) IsConnectionHealthy() bool {
	return dc.Status == string(ConnectionActive) && !dc.IsDeleted
}

// GetTableKey 获取表的唯一标识符
// 格式：schema_name.table_name
func (sm *SchemaMetadata) GetTableKey() string {
	if sm.SchemaName != "" {
		return sm.SchemaName + "." + sm.TableName
	}
	return sm.TableName
}