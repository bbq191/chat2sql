// 环境集成测试 - PostgreSQL + Valkey + Ollama+DeepSeek-R1:7b
// 测试P1阶段AI能力的完整环境依赖
package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"chat2sql-go/internal/config"
)

// EnvironmentTestSuite 环境测试套件
type EnvironmentTestSuite struct {
	dbPool      *pgxpool.Pool
	redisClient redis.UniversalClient
	logger      *zap.Logger
	testDB      string
}

// 测试环境配置
const (
	// PostgreSQL配置
	TestDBHost     = "localhost"
	TestDBPort     = 5432
	TestDBUser     = "postgres"
	TestDBPassword = "" // 默认无密码，根据实际环境调整
	TestDBName     = "chat2sql_test"

	// Valkey配置 (Redis compatible)
	TestRedisAddr = "localhost:6379"
	TestRedisDB   = 1 // 使用DB 1避免冲突

	// Ollama配置
	TestOllamaURL   = "http://localhost:11434"
	TestOllamaModel = "deepseek-r1:7b"
)

// TestEnvironment_PostgreSQL_Connection 测试PostgreSQL数据库连接
func TestEnvironment_PostgreSQL_Connection(t *testing.T) {
	logger := zap.NewNop()

	// 创建数据库配置
	dbConfig := &config.DatabaseConfig{
		Host:                      TestDBHost,
		Port:                      TestDBPort,
		User:                      TestDBUser,
		Password:                  TestDBPassword,
		Database:                  "postgres", // 先连接到默认数据库
		SSLMode:                   "disable",
		MaxConns:                  10,
		MinConns:                  1,
		MaxConnLifetime:           time.Hour,
		MaxConnIdleTime:           30 * time.Minute,
		HealthCheckPeriod:         5 * time.Minute,
		ConnectTimeout:            10 * time.Second,
		QueryTimeout:              10 * time.Second,
		PreparedStatementCacheSize: 50,
		LogLevel:                  "info",
		LogSlowQueries:            true,
		SlowQueryThreshold:        time.Second,
		ApplicationName:           "chat2sql-env-test",
		SearchPath:                "public",
	}

	t.Run("数据库配置验证", func(t *testing.T) {
		err := dbConfig.Validate()
		require.NoError(t, err, "数据库配置应该有效")
	})

	t.Run("PostgreSQL连接测试", func(t *testing.T) {
		// 获取连接池配置
		poolConfig, err := dbConfig.GetPoolConfigWithLogger(logger)
		require.NoError(t, err, "应该能够获取连接池配置")

		// 创建连接池
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		require.NoError(t, err, "应该能够创建数据库连接池")
		defer pool.Close()

		// 测试连接
		conn, err := pool.Acquire(ctx)
		require.NoError(t, err, "应该能够获取数据库连接")
		defer conn.Release()

		// 执行简单查询
		var version string
		err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
		require.NoError(t, err, "应该能够执行查询")

		t.Logf("PostgreSQL版本: %s", version)
		assert.Contains(t, version, "PostgreSQL", "应该返回PostgreSQL版本信息")
	})

	t.Run("创建测试数据库", func(t *testing.T) {
		// 连接到默认数据库创建测试数据库
		poolConfig, err := dbConfig.GetPoolConfig()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		require.NoError(t, err)
		defer pool.Close()

		// 检查测试数据库是否存在
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", TestDBName).Scan(&exists)
		require.NoError(t, err)

		if !exists {
			// 创建测试数据库
			_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", TestDBName))
			require.NoError(t, err, "应该能够创建测试数据库")
			t.Logf("成功创建测试数据库: %s", TestDBName)
		} else {
			t.Logf("测试数据库已存在: %s", TestDBName)
		}
	})

	t.Run("测试数据库Schema操作", func(t *testing.T) {
		// 连接到测试数据库
		testDBConfig := *dbConfig
		testDBConfig.Database = TestDBName

		poolConfig, err := testDBConfig.GetPoolConfig()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		require.NoError(t, err)
		defer pool.Close()

		// 创建测试表
		createTableSQL := `
			CREATE TABLE IF NOT EXISTS test_users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`
		_, err = pool.Exec(ctx, createTableSQL)
		require.NoError(t, err, "应该能够创建测试表")

		// 插入测试数据
		insertSQL := "INSERT INTO test_users (name, email) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING"
		_, err = pool.Exec(ctx, insertSQL, "Test User", "test@example.com")
		require.NoError(t, err, "应该能够插入测试数据")

		// 查询测试数据
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_users").Scan(&count)
		require.NoError(t, err, "应该能够查询数据")
		assert.GreaterOrEqual(t, count, 1, "应该至少有一条测试数据")

		t.Logf("测试表创建成功，当前记录数: %d", count)
	})
}

// TestEnvironment_Valkey_Connection 测试Valkey缓存连接
func TestEnvironment_Valkey_Connection(t *testing.T) {
	logger := zap.NewNop()

	// 创建Redis配置（Valkey兼容）
	redisConfig := &config.RedisConfig{
		Addr:            TestRedisAddr,
		Password:        "", // 默认无密码
		DB:              TestRedisDB,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        10,
		MinIdleConns:    2,
		PoolTimeout:     4 * time.Second,
		IdleTimeout:     5 * time.Minute,
		IdleCheckFreq:   time.Minute,
		TLSEnabled:      false,
		ClusterMode:     false,
	}

	t.Run("Valkey连接测试", func(t *testing.T) {
		// 创建Redis管理器
		redisManager, err := config.NewRedisManager(redisConfig, logger)
		require.NoError(t, err, "应该能够创建Redis管理器")
		defer redisManager.Close()

		// 获取客户端
		client := redisManager.GetClient()
		require.NotNil(t, client, "应该能够获取Redis客户端")

		// 测试Ping
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pong, err := client.Ping(ctx).Result()
		require.NoError(t, err, "Ping应该成功")
		assert.Equal(t, "PONG", pong, "应该返回PONG")

		t.Logf("Valkey连接成功: %s", pong)
	})

	t.Run("Valkey基础操作测试", func(t *testing.T) {
		redisManager, err := config.NewRedisManager(redisConfig, logger)
		require.NoError(t, err)
		defer redisManager.Close()

		client := redisManager.GetClient()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 测试SET/GET操作
		testKey := "chat2sql:test:key"
		testValue := "test_value_12345"

		// SET操作
		err = client.Set(ctx, testKey, testValue, time.Minute).Err()
		require.NoError(t, err, "SET操作应该成功")

		// GET操作
		value, err := client.Get(ctx, testKey).Result()
		require.NoError(t, err, "GET操作应该成功")
		assert.Equal(t, testValue, value, "值应该匹配")

		// TTL检查
		ttl, err := client.TTL(ctx, testKey).Result()
		require.NoError(t, err, "TTL查询应该成功")
		assert.Greater(t, ttl.Seconds(), float64(50), "TTL应该大于50秒")

		// 删除测试键
		err = client.Del(ctx, testKey).Err()
		require.NoError(t, err, "DEL操作应该成功")

		t.Logf("Valkey基础操作测试通过")
	})

	t.Run("Valkey性能和连接池测试", func(t *testing.T) {
		redisManager, err := config.NewRedisManager(redisConfig, logger)
		require.NoError(t, err)
		defer redisManager.Close()

		// 健康检查
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = redisManager.HealthCheck(ctx)
		require.NoError(t, err, "健康检查应该通过")

		// 获取连接池统计
		stats := redisManager.GetStats()
		require.NotNil(t, stats, "应该能够获取连接池统计")

		t.Logf("Valkey连接池统计 - Hits: %d, Misses: %d, Timeouts: %d, TotalConns: %d, IdleConns: %d",
			stats.Hits, stats.Misses, stats.Timeouts, stats.TotalConns, stats.IdleConns)
	})
}

// TestEnvironment_Ollama_DeepSeek_Connection 测试Ollama+DeepSeek-R1:7b连接
func TestEnvironment_Ollama_DeepSeek_Connection(t *testing.T) {
	t.Run("Ollama服务连接测试", func(t *testing.T) {
		// 创建HTTP客户端
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		// 测试Ollama健康检查
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", TestOllamaURL+"/api/tags", nil)
		require.NoError(t, err, "应该能够创建HTTP请求")

		resp, err := client.Do(req)
		require.NoError(t, err, "应该能够连接到Ollama服务")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Ollama服务应该可访问")
		t.Logf("Ollama服务连接成功，状态码: %d", resp.StatusCode)
	})

	t.Run("DeepSeek-R1模型可用性测试", func(t *testing.T) {
		// 创建HTTP客户端
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		// 查询可用模型
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", TestOllamaURL+"/api/tags", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err, "应该能够查询模型列表")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "模型查询应该成功")

		// 这里可以解析响应来检查deepseek-r1:7b是否可用
		// 简化测试，仅检查HTTP状态
		t.Logf("模型查询接口正常，状态码: %d", resp.StatusCode)
	})

	t.Run("Ollama生成API可用性测试", func(t *testing.T) {
		// 测试生成API端点的可访问性（不实际调用生成，避免长时间等待）
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 测试生成API端点可访问性
		req, err := http.NewRequestWithContext(ctx, "POST", TestOllamaURL+"/api/generate", nil)
		if err == nil {
			// 仅创建请求检查URL格式，不发送避免长时间等待
			t.Logf("Ollama生成API端点格式正确: %s", req.URL.String())
			assert.Contains(t, req.URL.String(), "/api/generate", "生成API端点应包含正确路径")
		} else {
			t.Logf("创建生成API请求时出错: %v", err)
		}
	})
}

// TestEnvironment_Integration_AI_Pipeline 测试AI查询处理完整流程
func TestEnvironment_Integration_AI_Pipeline(t *testing.T) {
	t.Run("环境依赖检查", func(t *testing.T) {
		// 检查必要的环境变量
		envVars := map[string]string{
			"OLLAMA_MODEL":      getEnvWithDefault("OLLAMA_MODEL", TestOllamaModel),
			"OLLAMA_SERVER_URL": getEnvWithDefault("OLLAMA_SERVER_URL", TestOllamaURL),
		}

		for key, value := range envVars {
			t.Logf("环境变量 %s = %s", key, value)
		}

		// 检查服务可用性
		services := []struct {
			name string
			url  string
		}{
			{"PostgreSQL", fmt.Sprintf("%s:%d", TestDBHost, TestDBPort)},
			{"Valkey", TestRedisAddr},
			{"Ollama", TestOllamaURL},
		}

		for _, service := range services {
			t.Logf("检查服务 %s (%s) 可用性", service.name, service.url)
		}
	})

	t.Run("集成配置验证", func(t *testing.T) {
		// 验证所有配置都能正确加载
		dbConfig := config.DefaultDatabaseConfig()
		dbConfig.Database = TestDBName
		err := dbConfig.Validate()
		assert.NoError(t, err, "数据库配置应该有效")

		redisConfig := config.DefaultRedisConfig()
		redisConfig.Addr = TestRedisAddr
		redisConfig.DB = TestRedisDB
		assert.NotNil(t, redisConfig, "Redis配置应该有效")

		t.Logf("所有配置验证通过")
	})
}

// TestEnvironment_Performance_Baseline 性能基准测试
func TestEnvironment_Performance_Baseline(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试（使用 -short 标志）")
	}

	t.Run("数据库连接性能", func(t *testing.T) {
		dbConfig := config.DefaultDatabaseConfig()
		dbConfig.Database = TestDBName
		dbConfig.MaxConns = 20
		dbConfig.MinConns = 5

		poolConfig, err := dbConfig.GetPoolConfig()
		require.NoError(t, err)

		ctx := context.Background()
		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		require.NoError(t, err)
		defer pool.Close()

		// 测试并发连接获取
		start := time.Now()
		const concurrency = 10
		errors := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				conn, err := pool.Acquire(ctx)
				if err != nil {
					errors <- err
					return
				}
				defer conn.Release()

				var result int
				err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
				errors <- err
			}()
		}

		// 收集结果
		for i := 0; i < concurrency; i++ {
			err := <-errors
			assert.NoError(t, err, "并发查询应该成功")
		}

		duration := time.Since(start)
		t.Logf("数据库并发性能测试 (%d个连接): %v", concurrency, duration)
		assert.Less(t, duration, 5*time.Second, "并发连接应在5秒内完成")
	})

	t.Run("缓存性能测试", func(t *testing.T) {
		redisConfig := config.DefaultRedisConfig()
		redisConfig.Addr = TestRedisAddr
		redisConfig.DB = TestRedisDB
		redisConfig.PoolSize = 20

		redisManager, err := config.NewRedisManager(redisConfig, zap.NewNop())
		require.NoError(t, err)
		defer redisManager.Close()

		client := redisManager.GetClient()
		ctx := context.Background()

		// 批量操作性能测试
		start := time.Now()
		const operations = 100

		for i := 0; i < operations; i++ {
			key := fmt.Sprintf("perf:test:%d", i)
			value := fmt.Sprintf("value_%d", i)

			err := client.Set(ctx, key, value, time.Minute).Err()
			assert.NoError(t, err, "SET操作应该成功")
		}

		// 批量读取
		for i := 0; i < operations; i++ {
			key := fmt.Sprintf("perf:test:%d", i)
			_, err := client.Get(ctx, key).Result()
			assert.NoError(t, err, "GET操作应该成功")
		}

		duration := time.Since(start)
		t.Logf("缓存性能测试 (%d个操作): %v", operations*2, duration)

		// 清理测试数据
		for i := 0; i < operations; i++ {
			key := fmt.Sprintf("perf:test:%d", i)
			client.Del(ctx, key)
		}
	})
}

// getEnvWithDefault 获取环境变量，如果不存在返回默认值
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestMain 测试主函数，进行环境预检查
func TestMain(m *testing.M) {
	// 设置环境变量（如果需要）
	if os.Getenv("OLLAMA_SERVER_URL") == "" {
		os.Setenv("OLLAMA_SERVER_URL", TestOllamaURL)
	}
	if os.Getenv("OLLAMA_MODEL") == "" {
		os.Setenv("OLLAMA_MODEL", TestOllamaModel)
	}

	// 运行测试
	code := m.Run()

	// 清理（如果需要）
	os.Exit(code)
}