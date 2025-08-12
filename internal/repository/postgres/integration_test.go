package postgres

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chat2sql-go/internal/config"
	"chat2sql-go/internal/database"
	"chat2sql-go/internal/repository"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// PostgreSQLRepositoryTestSuite PostgreSQL Repository 集成测试套件
// 使用本地PostgreSQL环境进行测试
type PostgreSQLRepositoryTestSuite struct {
	suite.Suite
	pool       *database.Manager
	repository repository.Repository
	logger     *zap.Logger
}

// SetupSuite 设置测试套件，使用本地PostgreSQL环境
func (suite *PostgreSQLRepositoryTestSuite) SetupSuite() {
	// 创建logger
	suite.logger = zap.NewNop()
	
	// 使用本地PostgreSQL配置
	dbConfig := &config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "",  // 本地环境通常无密码或使用默认配置
		Database:        "chat2sql_test",
		SSLMode:         "disable",  // 本地环境禁用SSL
		MaxConns:        50,  // 增加连接数以支持并发测试
		MinConns:        5,   // 增加最小连接数
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		HealthCheckPeriod: 5 * time.Minute,  // 健康检查周期
		ConnectTimeout:  30 * time.Second,   // 连接超时
		QueryTimeout:    30 * time.Second,   // 查询超时
		PreparedStatementCacheSize: 100,     // 预处理语句缓存
		ApplicationName: "chat2sql-test",    // 应用名称
		SearchPath:      "public",           // 搜索路径
	}
	
	// 创建数据库管理器
	manager, err := database.NewManager(dbConfig, suite.logger)
	require.NoError(suite.T(), err)
	suite.pool = manager
	
	// 创建Repository
	suite.repository = NewPostgreSQLRepository(manager.GetPool(), suite.logger)
	
	// 测试数据库连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = suite.pool.GetPool().Ping(ctx)
	require.NoError(suite.T(), err, "无法连接到本地PostgreSQL数据库，请确保PostgreSQL运行并可访问")
	
	// 清理现有数据库结构，然后执行迁移
	err = suite.cleanDatabase(ctx)
	require.NoError(suite.T(), err, "清理数据库失败")
	
	// 执行数据库迁移，确保测试表结构存在
	err = suite.runMigrations(ctx)
	require.NoError(suite.T(), err, "数据库迁移执行失败")
	
	suite.T().Logf("已连接到本地PostgreSQL数据库: %s@%s:%d/%s", 
		dbConfig.User, dbConfig.Host, dbConfig.Port, dbConfig.Database)
}

// TearDownSuite 清理测试套件，关闭数据库连接
func (suite *PostgreSQLRepositoryTestSuite) TearDownSuite() {
	if suite.pool != nil {
		suite.pool.Close()
	}
}

// cleanDatabase 清理测试数据库
func (suite *PostgreSQLRepositoryTestSuite) cleanDatabase(ctx context.Context) error {
	// 清理SQL - 按依赖顺序删除
	cleanupSQL := `
		-- 删除触发器
		DROP TRIGGER IF EXISTS tr_users_update_time ON users;
		DROP TRIGGER IF EXISTS tr_query_history_update_time ON query_history;
		DROP TRIGGER IF EXISTS tr_database_connections_update_time ON database_connections;
		DROP TRIGGER IF EXISTS tr_schema_metadata_update_time ON schema_metadata;
		
		-- 删除函数
		DROP FUNCTION IF EXISTS update_timestamp_trigger();
		
		-- 删除表（按依赖顺序）
		DROP TABLE IF EXISTS schema_metadata CASCADE;
		DROP TABLE IF EXISTS query_history CASCADE;
		DROP TABLE IF EXISTS database_connections CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
	`
	
	_, err := suite.pool.GetPool().Exec(ctx, cleanupSQL)
	if err != nil {
		return fmt.Errorf("清理数据库失败: %w", err)
	}
	
	suite.T().Logf("数据库清理完成")
	return nil
}

// runMigrations 执行数据库迁移
func (suite *PostgreSQLRepositoryTestSuite) runMigrations(ctx context.Context) error {
	// 构建迁移文件路径
	migrationPath := filepath.Join("..", "..", "..", "migrations", "001_create_tables.sql")
	
	// 读取迁移文件
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}
	
	// 处理SQL内容，移除CONCURRENTLY关键字（测试环境不需要并发创建索引）
	sqlContent := string(migrationSQL)
	sqlContent = strings.ReplaceAll(sqlContent, "CREATE INDEX CONCURRENTLY", "CREATE INDEX")
	
	// 执行迁移SQL
	_, err = suite.pool.GetPool().Exec(ctx, sqlContent)
	if err != nil {
		return fmt.Errorf("执行迁移SQL失败: %w", err)
	}
	
	suite.T().Logf("数据库迁移执行成功")
	return nil
}

// SetupTest 每个测试前的设置
func (suite *PostgreSQLRepositoryTestSuite) SetupTest() {
	// 清理数据（可选，根据需要决定是否每次都清理）
}

// TestUserRepository_CRUD 测试用户Repository的CRUD操作
func (suite *PostgreSQLRepositoryTestSuite) TestUserRepository_CRUD() {
	ctx := context.Background()
	userRepo := suite.repository.UserRepo()

	t := suite.T()
	
	// 测试创建用户
	user := &repository.User{
		Username:     "testuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password_123",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}

	start := time.Now()
	err := userRepo.Create(ctx, user)
	createDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Greater(t, user.ID, int64(0))
	assert.Less(t, createDuration, 200*time.Millisecond, "用户创建时间应小于200ms")

	t.Logf("用户创建成功，ID: %d, 耗时: %v", user.ID, createDuration)
	
	// 测试通过ID获取用户
	start = time.Now()
	foundUser, err := userRepo.GetByID(ctx, user.ID)
	getDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.NotNil(t, foundUser)
	assert.Equal(t, user.Username, foundUser.Username)
	assert.Equal(t, user.Email, foundUser.Email)
	assert.Equal(t, user.Role, foundUser.Role)
	assert.Equal(t, user.Status, foundUser.Status)
	assert.Less(t, getDuration, 100*time.Millisecond, "用户查询时间应小于100ms")

	t.Logf("用户查询成功，耗时: %v", getDuration)
	
	// 测试通过用户名获取用户
	foundUser2, err := userRepo.GetByUsername(ctx, user.Username)
	require.NoError(t, err)
	assert.Equal(t, user.ID, foundUser2.ID)
	
	// 测试用户名存在性检查
	exists, err := userRepo.ExistsByUsername(ctx, user.Username)
	require.NoError(t, err)
	assert.True(t, exists)
	
	// 测试不存在的用户名
	exists, err = userRepo.ExistsByUsername(ctx, "nonexistent_user")
	require.NoError(t, err)
	assert.False(t, exists)
	
	// 测试更新用户
	user.Role = string(repository.RoleAdmin)
	user.Status = string(repository.StatusInactive)
	
	start = time.Now()
	err = userRepo.Update(ctx, user)
	updateDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, updateDuration, 200*time.Millisecond, "用户更新时间应小于200ms")
	
	// 验证更新是否生效
	updatedUser, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, string(repository.RoleAdmin), updatedUser.Role)
	assert.Equal(t, string(repository.StatusInactive), updatedUser.Status)
	
	t.Logf("用户更新成功，耗时: %v", updateDuration)
	
	// 测试软删除
	start = time.Now()
	err = userRepo.Delete(ctx, user.ID)
	deleteDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, deleteDuration, 200*time.Millisecond, "用户删除时间应小于200ms")
	
	// 验证软删除后用户不能通过常规方法查询到
	_, err = userRepo.GetByID(ctx, user.ID)
	assert.Error(t, err)
	assert.True(t, repository.IsNotFound(err), "应该返回记录不存在错误")
	
	t.Logf("用户软删除成功，耗时: %v", deleteDuration)
}

// TestUserRepository_List 测试用户列表查询
func (suite *PostgreSQLRepositoryTestSuite) TestUserRepository_List() {
	ctx := context.Background()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 创建测试用户
	users := make([]*repository.User, 3)
	for i := 0; i < 3; i++ {
		user := &repository.User{
			Username:     fmt.Sprintf("listuser_%d_%d", i, time.Now().UnixNano()),
			Email:        fmt.Sprintf("listuser_%d_%d@example.com", i, time.Now().UnixNano()),
			PasswordHash: "hashed_password",
			Role:         string(repository.RoleUser),
			Status:       string(repository.StatusActive),
		}
		err := userRepo.Create(ctx, user)
		require.NoError(t, err)
		users[i] = user
	}
	
	// 测试列表查询
	start := time.Now()
	userList, err := userRepo.List(ctx, 10, 0)
	listDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(userList), 3)
	assert.Less(t, listDuration, 200*time.Millisecond, "用户列表查询时间应小于200ms")
	
	t.Logf("用户列表查询成功，返回%d个用户，耗时: %v", len(userList), listDuration)
	
	// 测试分页
	userList1, err := userRepo.List(ctx, 2, 0)
	require.NoError(t, err)
	
	userList2, err := userRepo.List(ctx, 2, 2)
	require.NoError(t, err)
	
	// 验证分页结果不重复
	if len(userList1) >= 2 && len(userList2) >= 1 {
		assert.NotEqual(t, userList1[0].ID, userList2[0].ID)
	}
	
	// 测试用户数量统计
	count, err := userRepo.Count(ctx)
	require.NoError(t, err)
	assert.Greater(t, count, int64(0))
	
	t.Logf("用户数量统计: %d", count)
}

// TestUserRepository_Authentication 测试用户认证相关功能
func (suite *PostgreSQLRepositoryTestSuite) TestUserRepository_Authentication() {
	ctx := context.Background()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 创建测试用户
	user := &repository.User{
		Username:     "authuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("authuser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "correct_hash_123",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	
	// 测试有效凭据验证
	validUser, err := userRepo.ValidateCredentials(ctx, user.Username, "correct_hash_123")
	require.NoError(t, err)
	assert.NotNil(t, validUser)
	assert.Equal(t, user.ID, validUser.ID)
	
	// 测试无效凭据验证
	invalidUser, err := userRepo.ValidateCredentials(ctx, user.Username, "wrong_hash")
	assert.Error(t, err)
	assert.Nil(t, invalidUser)
	assert.Equal(t, repository.ErrInvalidCredentials, err)
	
	// 测试更新密码
	newPasswordHash := "new_hash_456"
	err = userRepo.UpdatePassword(ctx, user.ID, newPasswordHash)
	require.NoError(t, err)
	
	// 验证新密码可以通过认证
	updatedUser, err := userRepo.ValidateCredentials(ctx, user.Username, newPasswordHash)
	require.NoError(t, err)
	assert.Equal(t, user.ID, updatedUser.ID)
	
	// 验证旧密码不能通过认证
	_, err = userRepo.ValidateCredentials(ctx, user.Username, "correct_hash_123")
	assert.Error(t, err)
	
	t.Logf("用户认证测试完成")
}

// TestQueryHistoryRepository_CRUD 测试查询历史Repository的CRUD操作
func (suite *PostgreSQLRepositoryTestSuite) TestQueryHistoryRepository_CRUD() {
	ctx := context.Background()
	queryRepo := suite.repository.QueryHistoryRepo()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 首先创建一个测试用户
	user := &repository.User{
		Username:     "queryuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("queryuser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	
	// 测试创建查询历史
	executionTime := int32(150)
	resultRows := int32(25)
	errorMsg := ""
	generatedSQL := "SELECT * FROM users WHERE status = 'active'"
	
	query := &repository.QueryHistory{
		BaseModel: repository.BaseModel{
			CreateBy: &user.ID,
			UpdateBy: &user.ID,
		},
		UserID:        user.ID,
		ConnectionID:  nil, // 设置为nil避免外键约束问题
		NaturalQuery:  "查询所有活跃用户",
		GeneratedSQL:  generatedSQL,
		SQLHash:       generateSQLHash(generatedSQL),
		Status:        string(repository.QuerySuccess),
		ExecutionTime: &executionTime,
		ResultRows:    &resultRows,
		ErrorMessage:  &errorMsg,
	}

	start := time.Now()
	err = queryRepo.Create(ctx, query)
	createDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Greater(t, query.ID, int64(0))
	assert.Less(t, createDuration, 200*time.Millisecond, "查询历史创建时间应小于200ms")

	t.Logf("查询历史创建成功，ID: %d, 耗时: %v", query.ID, createDuration)
	
	// 测试通过ID获取查询历史
	start = time.Now()
	foundQuery, err := queryRepo.GetByID(ctx, query.ID)
	getDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.NotNil(t, foundQuery)
	assert.Equal(t, query.NaturalQuery, foundQuery.NaturalQuery)
	assert.Equal(t, query.GeneratedSQL, foundQuery.GeneratedSQL)
	assert.Equal(t, query.Status, foundQuery.Status)
	assert.Equal(t, *query.ExecutionTime, *foundQuery.ExecutionTime)
	assert.Equal(t, *query.ResultRows, *foundQuery.ResultRows)
	assert.Less(t, getDuration, 100*time.Millisecond, "查询历史查询时间应小于100ms")

	t.Logf("查询历史查询成功，耗时: %v", getDuration)
	
	// 测试更新查询历史状态
	query.Status = string(repository.QueryError)
	newErrorMsg := "执行失败：语法错误"
	query.ErrorMessage = &newErrorMsg
	
	err = queryRepo.Update(ctx, query)
	require.NoError(t, err)
	
	// 验证更新
	updatedQuery, err := queryRepo.GetByID(ctx, query.ID)
	require.NoError(t, err)
	assert.Equal(t, string(repository.QueryError), updatedQuery.Status)
	assert.Equal(t, newErrorMsg, *updatedQuery.ErrorMessage)
}

// TestQueryHistoryRepository_ListAndSearch 测试查询历史列表和搜索功能
func (suite *PostgreSQLRepositoryTestSuite) TestQueryHistoryRepository_ListAndSearch() {
	ctx := context.Background()
	queryRepo := suite.repository.QueryHistoryRepo()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 创建测试用户
	user := &repository.User{
		Username:     "searchuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("searchuser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	
	// 创建多个查询历史记录
	queries := []struct {
		natural string
		sql     string
		status  repository.QueryStatus
	}{
		{"查询用户信息", "SELECT * FROM users", repository.QuerySuccess},
		{"查询订单数据", "SELECT * FROM orders", repository.QuerySuccess},
		{"用户统计分析", "SELECT COUNT(*) FROM users GROUP BY status", repository.QuerySuccess},
	}
	
	createdQueries := make([]*repository.QueryHistory, len(queries))
	for i, q := range queries {
		executionTime := int32(100 + i*10)
		resultRows := int32(10 + i*5)
		errorMsg := ""
		
		query := &repository.QueryHistory{
			BaseModel: repository.BaseModel{
				CreateBy: &user.ID,
				UpdateBy: &user.ID,
			},
			UserID:        user.ID,
			ConnectionID:  nil, // 设置为nil避免外键约束问题
			NaturalQuery:  q.natural,
			GeneratedSQL:  q.sql,
			SQLHash:       generateSQLHash(q.sql),
			Status:        string(q.status),
			ExecutionTime: &executionTime,
			ResultRows:    &resultRows,
			ErrorMessage:  &errorMsg,
		}
		
		err := queryRepo.Create(ctx, query)
		require.NoError(t, err)
		createdQueries[i] = query
	}
	
	// 测试用户查询历史列表
	start := time.Now()
	userQueries, err := queryRepo.ListByUser(ctx, user.ID, 10, 0)
	listDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(userQueries), 3)
	assert.Less(t, listDuration, 200*time.Millisecond, "用户查询历史列表查询时间应小于200ms")
	
	t.Logf("用户查询历史列表查询成功，返回%d条记录，耗时: %v", len(userQueries), listDuration)
	
	// 测试自然语言查询搜索
	searchResults, err := queryRepo.SearchByNaturalQuery(ctx, user.ID, "用户", 5, 0)
	require.NoError(t, err)
	
	// 验证搜索结果包含关键词
	assert.Greater(t, len(searchResults), 0)
	for _, result := range searchResults {
		assert.Contains(t, result.NaturalQuery, "用户")
	}
	
	t.Logf("自然语言查询搜索成功，找到%d条匹配记录", len(searchResults))
	
	// 测试SQL查询搜索
	sqlSearchResults, err := queryRepo.SearchBySQL(ctx, user.ID, "SELECT", 5, 0)
	require.NoError(t, err)
	
	// 验证搜索结果
	assert.Greater(t, len(sqlSearchResults), 0)
	for _, result := range sqlSearchResults {
		assert.Contains(t, result.GeneratedSQL, "SELECT")
	}
	
	t.Logf("SQL查询搜索成功，找到%d条匹配记录", len(sqlSearchResults))
}

// TestConnectionRepository_CRUD 测试数据库连接Repository的CRUD操作
func (suite *PostgreSQLRepositoryTestSuite) TestConnectionRepository_CRUD() {
	ctx := context.Background()
	connRepo := suite.repository.ConnectionRepo()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 首先创建一个测试用户
	user := &repository.User{
		Username:     "connuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("connuser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	
	// 测试创建数据库连接
	conn := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			CreateBy: &user.ID,
			UpdateBy: &user.ID,
		},
		UserID:            user.ID,
		Name:              "测试PostgreSQL连接_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Host:              "localhost",
		Port:              5432,
		DatabaseName:      "testdb",
		Username:          "testuser",
		PasswordEncrypted: "encrypted_password_123",
		DBType:            string(repository.DBTypePostgreSQL),
		Status:            string(repository.ConnectionActive),
	}

	start := time.Now()
	err = connRepo.Create(ctx, conn)
	createDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Greater(t, conn.ID, int64(0))
	assert.Less(t, createDuration, 200*time.Millisecond, "连接创建时间应小于200ms")

	t.Logf("数据库连接创建成功，ID: %d, 耗时: %v", conn.ID, createDuration)
	
	// 测试通过ID获取连接
	start = time.Now()
	foundConn, err := connRepo.GetByID(ctx, conn.ID)
	getDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.NotNil(t, foundConn)
	assert.Equal(t, conn.Name, foundConn.Name)
	assert.Equal(t, conn.Host, foundConn.Host)
	assert.Equal(t, conn.Port, foundConn.Port)
	assert.Equal(t, conn.DatabaseName, foundConn.DatabaseName)
	assert.Equal(t, conn.DBType, foundConn.DBType)
	assert.Equal(t, conn.Status, foundConn.Status)
	assert.Less(t, getDuration, 100*time.Millisecond, "连接查询时间应小于100ms")

	t.Logf("数据库连接查询成功，耗时: %v", getDuration)
	
	// 测试获取用户的连接列表
	userConnections, err := connRepo.ListByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(userConnections), 1)
	
	// 验证列表中包含我们创建的连接
	found := false
	for _, c := range userConnections {
		if c.ID == conn.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "用户连接列表应包含创建的连接")
	
	// 测试更新连接
	conn.Status = string(repository.ConnectionInactive)
	conn.Name = "更新后的连接名"
	
	start = time.Now()
	err = connRepo.Update(ctx, conn)
	updateDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, updateDuration, 200*time.Millisecond, "连接更新时间应小于200ms")
	
	// 验证更新是否生效
	updatedConn, err := connRepo.GetByID(ctx, conn.ID)
	require.NoError(t, err)
	assert.Equal(t, string(repository.ConnectionInactive), updatedConn.Status)
	assert.Equal(t, "更新后的连接名", updatedConn.Name)
	
	t.Logf("数据库连接更新成功，耗时: %v", updateDuration)
	
	// 测试软删除
	start = time.Now()
	err = connRepo.Delete(ctx, conn.ID)
	deleteDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, deleteDuration, 200*time.Millisecond, "连接删除时间应小于200ms")
	
	// 验证软删除后连接不能通过常规方法查询到
	_, err = connRepo.GetByID(ctx, conn.ID)
	assert.Error(t, err)
	assert.True(t, repository.IsNotFound(err), "应该返回记录不存在错误")
	
	t.Logf("数据库连接软删除成功，耗时: %v", deleteDuration)
}

// TestSchemaRepository_CRUD 测试Schema元数据Repository的CRUD操作
func (suite *PostgreSQLRepositoryTestSuite) TestSchemaRepository_CRUD() {
	ctx := context.Background()
	schemaRepo := suite.repository.SchemaRepo()
	connRepo := suite.repository.ConnectionRepo()
	userRepo := suite.repository.UserRepo()
	
	t := suite.T()
	
	// 创建测试用户
	user := &repository.User{
		Username:     "schemauser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("schemauser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	
	// 创建测试连接
	conn := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			CreateBy: &user.ID,
			UpdateBy: &user.ID,
		},
		UserID:            user.ID,
		Name:              "Schema测试连接",
		Host:              "localhost",
		Port:              5432,
		DatabaseName:      "testdb",
		Username:          "testuser",
		PasswordEncrypted: "encrypted_password",
		DBType:            string(repository.DBTypePostgreSQL),
		Status:            string(repository.ConnectionActive),
	}
	err = connRepo.Create(ctx, conn)
	require.NoError(t, err)
	
	// 测试创建Schema元数据
	tableComment := "用户表"
	columnComment := "用户ID主键"
	
	schema := &repository.SchemaMetadata{
		BaseModel: repository.BaseModel{
			CreateBy: &user.ID,
			UpdateBy: &user.ID,
		},
		ConnectionID:    conn.ID,
		SchemaName:      "public",
		TableName:       "users",
		ColumnName:      "id",
		DataType:        "bigint",
		IsNullable:      false,
		IsPrimaryKey:    true,
		IsForeignKey:    false,
		OrdinalPosition: 1,
		TableComment:    &tableComment,
		ColumnComment:   &columnComment,
	}

	start := time.Now()
	err = schemaRepo.Create(ctx, schema)
	createDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Greater(t, schema.ID, int64(0))
	assert.Less(t, createDuration, 200*time.Millisecond, "元数据创建时间应小于200ms")

	t.Logf("Schema元数据创建成功，ID: %d, 耗时: %v", schema.ID, createDuration)
	
	// 测试通过ID获取Schema元数据
	start = time.Now()
	foundSchema, err := schemaRepo.GetByID(ctx, schema.ID)
	getDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.NotNil(t, foundSchema)
	assert.Equal(t, schema.ConnectionID, foundSchema.ConnectionID)
	assert.Equal(t, schema.SchemaName, foundSchema.SchemaName)
	assert.Equal(t, schema.TableName, foundSchema.TableName)
	assert.Equal(t, schema.ColumnName, foundSchema.ColumnName)
	assert.Equal(t, schema.DataType, foundSchema.DataType)
	assert.Equal(t, schema.IsPrimaryKey, foundSchema.IsPrimaryKey)
	assert.Less(t, getDuration, 100*time.Millisecond, "元数据查询时间应小于100ms")

	t.Logf("Schema元数据查询成功，耗时: %v", getDuration)
	
	// 添加更多列的元数据
	schemas := []*repository.SchemaMetadata{
		{
			BaseModel: repository.BaseModel{
				CreateBy: &user.ID,
				UpdateBy: &user.ID,
			},
			ConnectionID:    conn.ID,
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "username",
			DataType:        "varchar(50)",
			IsNullable:      false,
			IsPrimaryKey:    false,
			IsForeignKey:    false,
			OrdinalPosition: 2,
			ColumnComment:   &[]string{"用户名"}[0],
		},
		{
			BaseModel: repository.BaseModel{
				CreateBy: &user.ID,
				UpdateBy: &user.ID,
			},
			ConnectionID:    conn.ID,
			SchemaName:      "public",
			TableName:       "users",
			ColumnName:      "email",
			DataType:        "varchar(255)",
			IsNullable:      false,
			IsPrimaryKey:    false,
			IsForeignKey:    false,
			OrdinalPosition: 3,
			ColumnComment:   &[]string{"邮箱地址"}[0],
		},
	}
	
	// 批量创建元数据
	for _, s := range schemas {
		err := schemaRepo.Create(ctx, s)
		require.NoError(t, err)
	}
	
	// 测试获取连接的Schema列表
	start = time.Now()
	connectionSchemas, err := schemaRepo.ListByConnection(ctx, conn.ID)
	listDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(connectionSchemas), 3)
	assert.Less(t, listDuration, 200*time.Millisecond, "元数据列表查询时间应小于200ms")
	
	t.Logf("连接Schema列表查询成功，返回%d条记录，耗时: %v", len(connectionSchemas), listDuration)
	
	// 测试按表名获取Schema
	tableSchemas, err := schemaRepo.ListByTable(ctx, conn.ID, "public", "users")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tableSchemas), 3)
	
	// 验证所有列都属于users表
	for _, s := range tableSchemas {
		assert.Equal(t, "users", s.TableName)
		assert.Equal(t, "public", s.SchemaName)
	}
	
	// 测试获取表列表
	tables, err := schemaRepo.ListTables(ctx, conn.ID, "public")
	require.NoError(t, err)
	assert.Contains(t, tables, "users")
	
	// 测试更新Schema元数据
	schema.ColumnComment = &[]string{"更新后的用户ID注释"}[0]
	
	start = time.Now()
	err = schemaRepo.Update(ctx, schema)
	updateDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, updateDuration, 200*time.Millisecond, "元数据更新时间应小于200ms")
	
	// 验证更新是否生效
	updatedSchema, err := schemaRepo.GetByID(ctx, schema.ID)
	require.NoError(t, err)
	assert.Equal(t, "更新后的用户ID注释", *updatedSchema.ColumnComment)
	
	t.Logf("Schema元数据更新成功，耗时: %v", updateDuration)
}

// TestRepository_Transaction 测试事务管理
func (suite *PostgreSQLRepositoryTestSuite) TestRepository_Transaction() {
	ctx := context.Background()
	
	t := suite.T()
	
	// 测试事务开始
	start := time.Now()
	txRepo, err := suite.repository.BeginTx(ctx)
	txStartDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.NotNil(t, txRepo)
	assert.Less(t, txStartDuration, 50*time.Millisecond, "事务开始时间应小于50ms")
	
	// 在事务中创建用户
	user := &repository.User{
		Username:     "txuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("txuser_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	
	err = txRepo.UserRepo().Create(ctx, user)
	require.NoError(t, err)
	assert.Greater(t, user.ID, int64(0))
	
	// 测试事务提交
	start = time.Now()
	err = txRepo.Commit()
	commitDuration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, commitDuration, 50*time.Millisecond, "事务提交时间应小于50ms")
	
	// 验证事务提交后数据存在
	foundUser, err := suite.repository.UserRepo().GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Username, foundUser.Username)
	
	t.Logf("事务管理测试完成 - 开始: %v, 提交: %v", txStartDuration, commitDuration)
}

// TestRepository_HealthCheck 测试健康检查
func (suite *PostgreSQLRepositoryTestSuite) TestRepository_HealthCheck() {
	ctx := context.Background()
	
	t := suite.T()
	
	start := time.Now()
	err := suite.repository.HealthCheck(ctx)
	duration := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, duration, 50*time.Millisecond, "健康检查时间应小于50ms")
	
	t.Logf("健康检查完成，耗时: %v", duration)
}

// TestRepository_Performance 性能基准测试
func (suite *PostgreSQLRepositoryTestSuite) TestRepository_Performance() {
	if testing.Short() {
		suite.T().Skip("跳过性能测试")
	}

	ctx := context.Background()
	userRepo := suite.repository.UserRepo()
	
	// 并发创建用户测试
	suite.T().Run("并发用户创建", func(t *testing.T) {
		concurrency := 10
		usersPerGoroutine := 10
		
		start := time.Now()
		
		// 创建channel收集结果
		results := make(chan error, concurrency*usersPerGoroutine)
		
		// 启动多个goroutine并发创建用户
		for i := 0; i < concurrency; i++ {
			go func(goroutineID int) {
				for j := 0; j < usersPerGoroutine; j++ {
					user := &repository.User{
						Username:     fmt.Sprintf("perfuser_%d_%d_%d", goroutineID, j, time.Now().UnixNano()),
						Email:        fmt.Sprintf("perfuser_%d_%d_%d@example.com", goroutineID, j, time.Now().UnixNano()),
						PasswordHash: "hashed_password",
						Role:         string(repository.RoleUser),
						Status:       string(repository.StatusActive),
					}
					
					results <- userRepo.Create(ctx, user)
				}
			}(i)
		}
		
		// 收集所有结果
		totalOperations := concurrency * usersPerGoroutine
		var errorCount int
		for i := 0; i < totalOperations; i++ {
			if err := <-results; err != nil {
				errorCount++
				t.Logf("创建用户失败: %v", err)
			}
		}
		
		totalDuration := time.Since(start)
		successfulOperations := totalOperations - errorCount
		
		if successfulOperations > 0 {
			avgDuration := totalDuration / time.Duration(successfulOperations)
			assert.Less(t, avgDuration, 200*time.Millisecond, 
				"平均用户创建时间 %v 超过性能指标 200ms", avgDuration)
			
			t.Logf("并发创建测试完成 - 成功: %d/%d, 总时间: %v, 平均: %v", 
				successfulOperations, totalOperations, totalDuration, avgDuration)
		}
		
		assert.Less(t, errorCount, totalOperations/10, "错误率应小于10%")
	})
}

// generateSQLHash 生成SQL语句的SHA-256哈希值
func generateSQLHash(sql string) string {
	h := sha256.New()
	h.Write([]byte(sql))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestSuite 运行测试套件
func TestPostgreSQLRepositoryTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}
	
	suite.Run(t, new(PostgreSQLRepositoryTestSuite))
}