package postgres

import (
	"context"
	"testing"
	"time"

	"chat2sql-go/internal/config"
	"chat2sql-go/internal/database"
	"chat2sql-go/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TestPostgreSQLRepository PostgreSQL Repository集成测试
// 验证所有Repository功能和P0性能指标
func TestPostgreSQLRepository(t *testing.T) {
	// 跳过集成测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 设置测试数据库连接
	pool, cleanup := setupTestDatabase(t)
	defer cleanup()

	// 创建Repository实例
	logger := zap.NewNop()
	repo := NewPostgreSQLRepository(pool, logger)
	
	// 执行健康检查
	t.Run("健康检查", func(t *testing.T) {
		testHealthCheck(t, repo)
	})
	
	// 测试用户Repository
	t.Run("用户Repository", func(t *testing.T) {
		testUserRepository(t, repo)
	})
	
	// 测试查询历史Repository
	t.Run("查询历史Repository", func(t *testing.T) {
		testQueryHistoryRepository(t, repo)
	})
	
	// 测试连接Repository
	t.Run("连接Repository", func(t *testing.T) {
		testConnectionRepository(t, repo)
	})
	
	// 测试元数据Repository
	t.Run("元数据Repository", func(t *testing.T) {
		testSchemaRepository(t, repo)
	})
	
	// 测试事务功能
	t.Run("事务管理", func(t *testing.T) {
		testTransactionManagement(t, repo)
	})
	
	// 性能基准测试
	t.Run("性能测试", func(t *testing.T) {
		testPerformanceBenchmarks(t, repo)
	})
}

// setupTestDatabase 设置测试数据库
func setupTestDatabase(t *testing.T) (*pgxpool.Pool, func()) {
	// 连接到本地测试数据库
	dbConfig := &config.DatabaseConfig{
		Host:            "localhost", 
		Port:            5432,
		User:            "postgres",
		Password:        "",  // 本地环境通常无密码或使用默认配置
		Database:        "chat2sql_test",
		SSLMode:         "disable",  // 本地环境禁用SSL
		MaxConns:        100,
		MinConns:        10,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		HealthCheckPeriod: 5 * time.Minute,  // 健康检查周期
		ConnectTimeout:  30 * time.Second,   // 连接超时
		QueryTimeout:    30 * time.Second,   // 查询超时
		PreparedStatementCacheSize: 100,     // 预处理语句缓存
		ApplicationName: "chat2sql-test",    // 应用名称
		SearchPath:      "public",           // 搜索路径
	}
	
	logger := zap.NewNop()
	
	manager, err := database.NewManager(dbConfig, logger)
	if err != nil {
		t.Skipf("无法连接到测试数据库: %v", err)
	}
	
	pool := manager.GetPool()
	
	// 清理函数
	cleanup := func() {
		manager.Close()
	}
	
	return pool, cleanup
}

// testHealthCheck 测试健康检查功能
func testHealthCheck(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	
	start := time.Now()
	err := repo.HealthCheck(ctx)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("健康检查失败: %v", err)
	}
	
	// P0性能指标：健康检查响应时间 < 50ms
	if duration > 50*time.Millisecond {
		t.Errorf("健康检查响应时间过长: %v, 期望 < 50ms", duration)
	}
	
	t.Logf("健康检查通过，响应时间: %v", duration)
}

// testUserRepository 测试用户Repository
func testUserRepository(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	userRepo := repo.UserRepo()
	
	// 测试用户创建
	user := &repository.User{
		Username:     "test_user_" + time.Now().Format("20060102150405"),
		Email:        "test_" + time.Now().Format("20060102150405") + "@example.com",
		PasswordHash: "hashed_password",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	
	start := time.Now()
	err := userRepo.Create(ctx, user)
	createDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	
	// P0性能指标：用户创建响应时间 < 200ms
	if createDuration > 200*time.Millisecond {
		t.Errorf("用户创建响应时间过长: %v, 期望 < 200ms", createDuration)
	}
	
	// 测试用户查询
	start = time.Now()
	foundUser, err := userRepo.GetByID(ctx, user.ID)
	queryDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	
	// P0性能指标：用户查询响应时间 < 100ms
	if queryDuration > 100*time.Millisecond {
		t.Errorf("用户查询响应时间过长: %v, 期望 < 100ms", queryDuration)
	}
	
	if foundUser.Username != user.Username {
		t.Errorf("查询到的用户名不匹配: got %s, want %s", foundUser.Username, user.Username)
	}
	
	// 测试用户存在性检查
	exists, err := userRepo.ExistsByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("检查用户存在性失败: %v", err)
	}
	
	if !exists {
		t.Error("用户应该存在但检查结果为不存在")
	}
	
	t.Logf("用户Repository测试通过 - 创建: %v, 查询: %v", createDuration, queryDuration)
}

// testQueryHistoryRepository 测试查询历史Repository
func testQueryHistoryRepository(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	queryRepo := repo.QueryHistoryRepo()
	
	// 创建测试查询历史
	query := &repository.QueryHistory{
		UserID:       1, // 假设存在用户ID 1
		NaturalQuery: "查询所有用户",
		GeneratedSQL: "SELECT * FROM users",
		Status:       string(repository.QuerySuccess),
		ExecutionTime: new(int32),
		ResultRows:   new(int32),
	}
	*query.ExecutionTime = 50
	*query.ResultRows = 10
	
	start := time.Now()
	err := queryRepo.Create(ctx, query)
	createDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("创建查询历史失败: %v", err)
	}
	
	// P0性能指标：查询历史创建响应时间 < 200ms
	if createDuration > 200*time.Millisecond {
		t.Errorf("查询历史创建响应时间过长: %v, 期望 < 200ms", createDuration)
	}
	
	// 测试查询历史列表
	start = time.Now()
	queries, err := queryRepo.ListByUser(ctx, 1, 10, 0)
	listDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("查询历史列表失败: %v", err)
	}
	
	// P0性能指标：查询历史列表响应时间 < 200ms
	if listDuration > 200*time.Millisecond {
		t.Errorf("查询历史列表响应时间过长: %v, 期望 < 200ms", listDuration)
	}
	
	if len(queries) == 0 {
		t.Error("应该至少有一条查询历史记录")
	}
	
	t.Logf("查询历史Repository测试通过 - 创建: %v, 列表: %v", createDuration, listDuration)
}

// testConnectionRepository 测试连接Repository
func testConnectionRepository(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	connRepo := repo.ConnectionRepo()
	
	// 创建测试连接
	conn := &repository.DatabaseConnection{
		UserID:            1, // 假设存在用户ID 1
		Name:              "test_conn_" + time.Now().Format("20060102150405"),
		Host:              "localhost",
		Port:              5432,
		DatabaseName:      "test_db",
		Username:          "test_user",
		PasswordEncrypted: "encrypted_password",
		DBType:            string(repository.DBTypePostgreSQL),
		Status:            string(repository.ConnectionActive),
	}
	
	start := time.Now()
	err := connRepo.Create(ctx, conn)
	createDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("创建连接配置失败: %v", err)
	}
	
	// P0性能指标：连接配置创建响应时间 < 200ms
	if createDuration > 200*time.Millisecond {
		t.Errorf("连接配置创建响应时间过长: %v, 期望 < 200ms", createDuration)
	}
	
	// 测试连接查询
	start = time.Now()
	foundConn, err := connRepo.GetByID(ctx, conn.ID)
	queryDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("查询连接配置失败: %v", err)
	}
	
	// P0性能指标：连接配置查询响应时间 < 100ms  
	if queryDuration > 100*time.Millisecond {
		t.Errorf("连接配置查询响应时间过长: %v, 期望 < 100ms", queryDuration)
	}
	
	if foundConn.Name != conn.Name {
		t.Errorf("查询到的连接名不匹配: got %s, want %s", foundConn.Name, conn.Name)
	}
	
	t.Logf("连接Repository测试通过 - 创建: %v, 查询: %v", createDuration, queryDuration)
}

// testSchemaRepository 测试元数据Repository
func testSchemaRepository(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	schemaRepo := repo.SchemaRepo()
	
	// 创建测试元数据
	schema := &repository.SchemaMetadata{
		ConnectionID:     1, // 假设存在连接ID 1
		SchemaName:       "public",
		TableName:        "test_table",
		ColumnName:       "id",
		DataType:         "bigint",
		IsNullable:       false,
		IsPrimaryKey:     true,
		OrdinalPosition:  1,
		ColumnComment:    new(string),
	}
	*schema.ColumnComment = "主键ID"
	
	start := time.Now()
	err := schemaRepo.Create(ctx, schema)
	createDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("创建元数据失败: %v", err)
	}
	
	// P0性能指标：元数据创建响应时间 < 200ms
	if createDuration > 200*time.Millisecond {
		t.Errorf("元数据创建响应时间过长: %v, 期望 < 200ms", createDuration)
	}
	
	// 测试元数据查询
	start = time.Now()
	schemas, err := schemaRepo.ListByConnection(ctx, 1)
	queryDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("查询元数据列表失败: %v", err)
	}
	
	// P0性能指标：元数据查询响应时间 < 200ms
	if queryDuration > 200*time.Millisecond {
		t.Errorf("元数据查询响应时间过长: %v, 期望 < 200ms", queryDuration)
	}
	
	if len(schemas) == 0 {
		t.Error("应该至少有一条元数据记录")
	}
	
	t.Logf("元数据Repository测试通过 - 创建: %v, 查询: %v", createDuration, queryDuration)
}

// testTransactionManagement 测试事务管理
func testTransactionManagement(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	
	// 测试事务开始
	start := time.Now()
	txRepo, err := repo.BeginTx(ctx)
	txStartDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	
	// P0性能指标：事务开始响应时间 < 50ms
	if txStartDuration > 50*time.Millisecond {
		t.Errorf("事务开始响应时间过长: %v, 期望 < 50ms", txStartDuration)
	}
	
	// 测试事务提交
	start = time.Now()
	err = txRepo.Commit()
	commitDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("事务提交失败: %v", err)
	}
	
	// P0性能指标：事务提交响应时间 < 50ms
	if commitDuration > 50*time.Millisecond {
		t.Errorf("事务提交响应时间过长: %v, 期望 < 50ms", commitDuration)
	}
	
	t.Logf("事务管理测试通过 - 开始: %v, 提交: %v", txStartDuration, commitDuration)
}

// testPerformanceBenchmarks 性能基准测试
func testPerformanceBenchmarks(t *testing.T, repo repository.Repository) {
	ctx := context.Background()
	
	// 并发连接测试
	t.Run("并发连接测试", func(t *testing.T) {
		const concurrency = 100
		const iterations = 10
		
		start := time.Now()
		
		// 创建多个goroutine并发执行健康检查
		done := make(chan bool, concurrency)
		errors := make(chan error, concurrency*iterations)
		
		for i := 0; i < concurrency; i++ {
			go func() {
				defer func() { done <- true }()
				
				for j := 0; j < iterations; j++ {
					if err := repo.HealthCheck(ctx); err != nil {
						errors <- err
						return
					}
				}
			}()
		}
		
		// 等待所有goroutine完成
		for i := 0; i < concurrency; i++ {
			<-done
		}
		
		totalDuration := time.Since(start)
		
		// 检查错误
		select {
		case err := <-errors:
			t.Fatalf("并发测试中发生错误: %v", err)
		default:
		}
		
		totalOperations := concurrency * iterations
		avgDuration := totalDuration / time.Duration(totalOperations)
		
		// P0性能指标：平均响应时间 < 200ms，总体完成时间合理
		if avgDuration > 200*time.Millisecond {
			t.Errorf("并发测试平均响应时间过长: %v, 期望 < 200ms", avgDuration)
		}
		
		// P0性能指标：支持1000+并发连接（这里测试100个并发）
		if totalDuration > 30*time.Second {
			t.Errorf("并发测试总时间过长: %v, 期望 < 30s", totalDuration)
		}
		
		t.Logf("并发测试通过 - 并发数: %d, 迭代: %d, 总时间: %v, 平均: %v", 
			concurrency, iterations, totalDuration, avgDuration)
	})
}

// BenchmarkRepository Repository性能基准测试
func BenchmarkRepository(b *testing.B) {
	if testing.Short() {
		b.Skip("跳过基准测试")
	}
	
	pool, cleanup := setupBenchDatabase(b)
	defer cleanup()
	
	logger := zap.NewNop()
	repo := NewPostgreSQLRepository(pool, logger)
	ctx := context.Background()
	
	b.Run("HealthCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = repo.HealthCheck(ctx)
		}
	})
	
	b.Run("UserQuery", func(b *testing.B) {
		userRepo := repo.UserRepo()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = userRepo.GetByID(ctx, 1) // 假设存在用户ID 1
		}
	})
}

// setupBenchDatabase 设置基准测试数据库
func setupBenchDatabase(b *testing.B) (*pgxpool.Pool, func()) {
	// 复用测试数据库配置
	dbConfig := &config.DatabaseConfig{
		Host:            "localhost", 
		Port:            5432,
		User:            "postgres",
		Password:        "password",
		Database:        "chat2sql_test",
		MaxConns:        100,
		MinConns:        10,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
	
	logger := zap.NewNop()
	
	manager, err := database.NewManager(dbConfig, logger)
	if err != nil {
		b.Skipf("无法连接到基准测试数据库: %v", err)
	}
	
	pool := manager.GetPool()
	
	cleanup := func() {
		manager.Close()
	}
	
	return pool, cleanup
}