package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"chat2sql-go/internal/auth"
	"chat2sql-go/internal/config"
	"chat2sql-go/internal/repository"
	"chat2sql-go/internal/repository/postgres"
	"chat2sql-go/internal/service"
)

// RealBusinessPerformanceTestSuite 真实业务性能测试套件
// 测试核心业务功能的实际性能，而非简单的HTTP健康检查
type RealBusinessPerformanceTestSuite struct {
	// 核心服务
	systemPool        *pgxpool.Pool
	repository        repository.Repository
	jwtService        *auth.JWTService
	connectionManager *service.ConnectionManager
	sqlExecutor       *service.SQLExecutor
	
	// 测试数据
	testUser         *repository.User
	testConnection   *repository.DatabaseConnection
	testQueryHistory *repository.QueryHistory
	
	// 配置
	logger           *zap.Logger
	encryptionKey    []byte
}

// 性能指标常量定义
const (
	// 性能目标 - 基于真实业务场景
	TARGET_JWT_GENERATION_QPS    = 1000   // JWT生成目标QPS
	TARGET_JWT_VALIDATION_QPS    = 2000   // JWT验证目标QPS  
	TARGET_SQL_EXECUTION_QPS     = 100    // SQL执行目标QPS
	TARGET_REPOSITORY_CRUD_QPS   = 500    // Repository CRUD目标QPS
	TARGET_CONNECTION_POOL_QPS   = 200    // 连接池获取目标QPS
	
	// 延迟目标
	TARGET_JWT_GENERATION_LATENCY = 5 * time.Millisecond   // JWT生成延迟目标
	TARGET_JWT_VALIDATION_LATENCY = 3 * time.Millisecond   // JWT验证延迟目标
	TARGET_SQL_EXECUTION_LATENCY  = 100 * time.Millisecond // SQL执行延迟目标
	TARGET_REPOSITORY_LATENCY     = 50 * time.Millisecond  // Repository操作延迟目标
	TARGET_CONNECTION_POOL_LATENCY = 20 * time.Millisecond // 连接池延迟目标
)

// TestRealBusinessPerformanceSuite 运行真实业务性能测试套件
func TestRealBusinessPerformanceSuite(t *testing.T) {
	suite := setupRealBusinessTestSuite(t)
	defer suite.cleanup()
	
	t.Run("JWT认证性能测试", func(t *testing.T) {
		suite.testJWTPerformance(t)
	})
	
	t.Run("SQL执行器性能测试", func(t *testing.T) {
		suite.testSQLExecutorPerformance(t)
	})
	
	t.Run("数据库连接池性能测试", func(t *testing.T) {
		suite.testConnectionPoolPerformance(t)
	})
	
	t.Run("Repository层CRUD性能测试", func(t *testing.T) {
		suite.testRepositoryPerformance(t)
	})
	
	t.Run("端到端API性能测试", func(t *testing.T) {
		suite.testEndToEndAPIPerformance(t)
	})
	
	t.Run("并发业务场景压力测试", func(t *testing.T) {
		suite.testConcurrentBusinessScenario(t)
	})
	
	t.Run("数据库事务性能测试", func(t *testing.T) {
		suite.testTransactionPerformance(t)
	})
}

// setupRealBusinessTestSuite 设置真实业务测试环境
func setupRealBusinessTestSuite(t *testing.T) *RealBusinessPerformanceTestSuite {
	// 创建测试日志器
	logger := zap.NewNop()
	
	// 生成加密密钥
	encryptionKey := make([]byte, 32)
	_, err := rand.Read(encryptionKey)
	require.NoError(t, err)
	
	// 设置数据库连接 - 使用真实PostgreSQL
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		// 尝试多种连接方式，优先使用Unix socket
		dbURL = "user=postgres dbname=chat2sql_test sslmode=disable"
	}
	
	// 创建系统连接池
	systemPool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	
	// 测试数据库连接
	err = systemPool.Ping(context.Background())
	require.NoError(t, err, "无法连接到测试数据库，请确保PostgreSQL运行并可访问")
	
	// 创建Repository
	repo := postgres.NewPostgreSQLRepository(systemPool, logger)
	
	// 创建Redis客户端（用于JWT黑名单）
	redisConfig := config.DefaultRedisConfig()
	redisClient, err := config.NewRedisClient(redisConfig)
	require.NoError(t, err)
	
	// 创建JWT服务
	jwtConfig := auth.DefaultJWTConfig()
	jwtService, err := auth.NewJWTService(jwtConfig, logger, redisClient)
	require.NoError(t, err)
	
	// 创建连接管理器
	connectionManager, err := service.NewConnectionManager(systemPool, repo.ConnectionRepo(), encryptionKey, logger)
	require.NoError(t, err)
	
	// 创建SQL执行器
	sqlExecutor := service.NewSQLExecutor(systemPool, connectionManager, logger)
	
	suite := &RealBusinessPerformanceTestSuite{
		systemPool:        systemPool,
		repository:        repo,
		jwtService:        jwtService,
		connectionManager: connectionManager,
		sqlExecutor:       sqlExecutor,
		logger:           logger,
		encryptionKey:    encryptionKey,
	}
	
	// 初始化测试数据
	suite.setupTestData(t)
	
	return suite
}

// setupTestData 设置测试数据
func (s *RealBusinessPerformanceTestSuite) setupTestData(t *testing.T) {
	ctx := context.Background()
	
	// 创建测试用户
	s.testUser = &repository.User{
		Username:     fmt.Sprintf("test_user_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test_password_hash",
		Role:         string(repository.RoleUser),
		Status:       string(repository.StatusActive),
	}
	err := s.repository.UserRepo().Create(ctx, s.testUser)
	require.NoError(t, err)
	
	// 创建测试数据库连接
	s.testConnection = &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			CreateBy: &s.testUser.ID,  // 设置创建者指针
			UpdateBy: &s.testUser.ID,  // 设置更新者指针
		},
		UserID:            s.testUser.ID,
		Name:              "test_connection",
		DBType:            string(repository.DBTypePostgreSQL),
		Host:              "localhost",
		Port:              5432,
		DatabaseName:      "chat2sql_test",
		Username:          "postgres",
		PasswordEncrypted: "password", // 明文密码，将通过ConnectionManager加密
		Status:            string(repository.ConnectionActive),
	}
	// 使用ConnectionManager创建连接，确保密码被正确加密
	err = s.connectionManager.CreateConnection(ctx, s.testConnection)
	require.NoError(t, err)
	
	// 创建测试查询历史
	executionTime := int32(100)
	generatedSQL := "SELECT 1 as test"
	sqlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(generatedSQL)))
	s.testQueryHistory = &repository.QueryHistory{
		BaseModel: repository.BaseModel{
			CreateBy: &s.testUser.ID,  // 设置创建者指针
			UpdateBy: &s.testUser.ID,  // 设置更新者指针
		},
		UserID:        s.testUser.ID,
		ConnectionID:  &s.testConnection.ID,
		NaturalQuery:  "测试查询",
		GeneratedSQL:  generatedSQL,
		SQLHash:       sqlHash,
		ExecutionTime: &executionTime,
		Status:        string(repository.QuerySuccess),
	}
	err = s.repository.QueryHistoryRepo().Create(ctx, s.testQueryHistory)
	require.NoError(t, err)
}

// cleanup 清理测试环境
func (s *RealBusinessPerformanceTestSuite) cleanup() {
	if s.systemPool != nil {
		s.systemPool.Close()
	}
}

// testJWTPerformance 测试JWT认证性能
func (s *RealBusinessPerformanceTestSuite) testJWTPerformance(t *testing.T) {
	t.Log("=== 开始JWT认证性能测试 ===")
	
	// 子测试：JWT Token生成性能
	t.Run("JWT_Token_生成性能", func(t *testing.T) {
		concurrency := 50
		requestsPerWorker := 100
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var maxLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试JWT Token生成性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 生成JWT Token
					_, err := s.jwtService.GenerateTokenPair(
						s.testUser.ID,
						s.testUser.Username,
						string(s.testUser.Role),
					)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if reqLatency > maxLatency {
						maxLatency = reqLatency
					}
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("JWT Token生成性能结果:")
		t.Logf("  总耗时: %v", totalTime)
		t.Logf("  成功请求: %d/%d", successCount, totalRequests)
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  最大延迟: %v", maxLatency)
		
		// 性能断言
		assert.True(t, qps >= TARGET_JWT_GENERATION_QPS, 
			"JWT生成QPS不达标: 期望>=%d, 实际=%.2f", TARGET_JWT_GENERATION_QPS, qps)
		assert.True(t, avgLatency <= TARGET_JWT_GENERATION_LATENCY,
			"JWT生成延迟过高: 期望<=%v, 实际=%v", TARGET_JWT_GENERATION_LATENCY, avgLatency)
		assert.Equal(t, int64(totalRequests), successCount, "应该所有JWT生成都成功")
	})
	
	// 子测试：JWT Token验证性能
	t.Run("JWT_Token_验证性能", func(t *testing.T) {
		// 先生成一个Token用于验证测试
		tokenPair, err := s.jwtService.GenerateTokenPair(
			s.testUser.ID,
			s.testUser.Username, 
			string(s.testUser.Role),
		)
		require.NoError(t, err)
		
		concurrency := 100
		requestsPerWorker := 200
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试JWT Token验证性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 验证JWT Token
					_, err := s.jwtService.ValidateAccessToken(tokenPair.AccessToken)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("JWT Token验证性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		
		// 性能断言
		assert.True(t, qps >= TARGET_JWT_VALIDATION_QPS,
			"JWT验证QPS不达标: 期望>=%d, 实际=%.2f", TARGET_JWT_VALIDATION_QPS, qps)
		assert.True(t, avgLatency <= TARGET_JWT_VALIDATION_LATENCY,
			"JWT验证延迟过高: 期望<=%v, 实际=%v", TARGET_JWT_VALIDATION_LATENCY, avgLatency)
	})
}

// testSQLExecutorPerformance 测试SQL执行器性能
func (s *RealBusinessPerformanceTestSuite) testSQLExecutorPerformance(t *testing.T) {
	t.Log("=== 开始SQL执行器性能测试 ===")
	
	ctx := context.Background()
	
	// 测试简单查询性能
	t.Run("简单查询性能", func(t *testing.T) {
		concurrency := 20
		requestsPerWorker := 50
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var totalRows int64
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试SQL简单查询性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 执行简单的SQL查询
					result, err := s.sqlExecutor.ExecuteQuery(ctx, "SELECT 1 as test_col", s.testConnection)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil && result.Status == string(repository.QuerySuccess) {
						atomic.AddInt64(&successCount, 1)
						atomic.AddInt64(&totalRows, int64(result.RowCount))
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("SQL简单查询性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		t.Logf("  总返回行数: %d", totalRows)
		
		// 性能断言
		assert.True(t, qps >= TARGET_SQL_EXECUTION_QPS/2, // 简单查询目标降半
			"SQL简单查询QPS不达标: 期望>=%d, 实际=%.2f", TARGET_SQL_EXECUTION_QPS/2, qps)
		assert.True(t, avgLatency <= TARGET_SQL_EXECUTION_LATENCY,
			"SQL查询延迟过高: 期望<=%v, 实际=%v", TARGET_SQL_EXECUTION_LATENCY, avgLatency)
	})
	
	// 测试复杂查询性能
	t.Run("复杂查询性能", func(t *testing.T) {
		complexSQL := `
			WITH numbered_series AS (
				SELECT generate_series(1, 1000) as n
			)
			SELECT 
				n,
				n * 2 as doubled,
				CASE WHEN n % 2 = 0 THEN 'even' ELSE 'odd' END as parity,
				random() as rand_val
			FROM numbered_series 
			WHERE n <= 100
			ORDER BY n`
		
		concurrency := 10
		requestsPerWorker := 20
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var totalRows int64
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试SQL复杂查询性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					result, err := s.sqlExecutor.ExecuteQuery(ctx, complexSQL, s.testConnection)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil && result.Status == string(repository.QuerySuccess) {
						atomic.AddInt64(&successCount, 1)
						atomic.AddInt64(&totalRows, int64(result.RowCount))
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("SQL复杂查询性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		t.Logf("  总返回行数: %d", totalRows)
		
		// 复杂查询性能要求适当放宽
		assert.True(t, qps >= TARGET_SQL_EXECUTION_QPS/5, // 复杂查询目标降低到1/5
			"SQL复杂查询QPS不达标: 期望>=%d, 实际=%.2f", TARGET_SQL_EXECUTION_QPS/5, qps)
		assert.True(t, avgLatency <= TARGET_SQL_EXECUTION_LATENCY*3, // 复杂查询延迟允许3倍
			"SQL复杂查询延迟过高: 期望<=%v, 实际=%v", TARGET_SQL_EXECUTION_LATENCY*3, avgLatency)
	})
}

// testConnectionPoolPerformance 测试数据库连接池性能
func (s *RealBusinessPerformanceTestSuite) testConnectionPoolPerformance(t *testing.T) {
	t.Log("=== 开始数据库连接池性能测试 ===")
	
	ctx := context.Background()
	
	t.Run("连接池获取性能", func(t *testing.T) {
		concurrency := 30
		requestsPerWorker := 50
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试连接池获取性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 获取连接池
					pool, err := s.connectionManager.GetConnectionPool(ctx, s.testConnection.ID)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil && pool != nil {
						atomic.AddInt64(&successCount, 1)
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("连接池获取性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		
		// 性能断言
		assert.True(t, qps >= TARGET_CONNECTION_POOL_QPS,
			"连接池获取QPS不达标: 期望>=%d, 实际=%.2f", TARGET_CONNECTION_POOL_QPS, qps)
		assert.True(t, avgLatency <= TARGET_CONNECTION_POOL_LATENCY,
			"连接池获取延迟过高: 期望<=%v, 实际=%v", TARGET_CONNECTION_POOL_LATENCY, avgLatency)
	})
	
	t.Run("连接池并发ping测试", func(t *testing.T) {
		// 获取连接池
		pool, err := s.connectionManager.GetConnectionPool(ctx, s.testConnection.ID)
		require.NoError(t, err)
		
		concurrency := 50
		requestsPerWorker := 100
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试连接池并发ping: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 执行ping测试
					err := pool.Ping(ctx)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("连接池并发ping结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		
		// 性能断言
		assert.True(t, qps >= 1000, "连接池ping QPS应该很高") // ping操作应该很快
		assert.True(t, avgLatency <= 10*time.Millisecond, "ping延迟应该很低")
		assert.Equal(t, int64(totalRequests), successCount, "所有ping都应该成功")
	})
}

// testRepositoryPerformance 测试Repository层性能
func (s *RealBusinessPerformanceTestSuite) testRepositoryPerformance(t *testing.T) {
	t.Log("=== 开始Repository层CRUD性能测试 ===")
	
	ctx := context.Background()
	
	// 测试用户Repository性能
	t.Run("用户Repository_CRUD性能", func(t *testing.T) {
		concurrency := 20
		requestsPerWorker := 25
		totalRequests := concurrency * requestsPerWorker
		
		var createCount, readCount, updateCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试用户Repository CRUD性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// Create操作
					testUser := &repository.User{
						BaseModel: repository.BaseModel{
							CreateBy: &s.testUser.ID,  // 设置创建者指针
							UpdateBy: &s.testUser.ID,  // 设置更新者指针
						},
						Username:     fmt.Sprintf("perf_test_%d_%d_%d", workerID, j, time.Now().UnixNano()),
						Email:        fmt.Sprintf("perf_test_%d_%d_%d@test.com", workerID, j, time.Now().UnixNano()),
						PasswordHash: "test_hash",
						Role:         string(repository.RoleUser),
						Status:       string(repository.StatusActive),
					}
					
					err := s.repository.UserRepo().Create(ctx, testUser)
					createLatency := time.Since(reqStart)
					
					if err == nil {
						atomic.AddInt64(&createCount, 1)
					} else {
						// 记录第一个Create错误
						if atomic.LoadInt64(&createCount) == 0 {
							t.Logf("用户Create第一个错误: %v", err)
						}
					}
					
					// Read操作
					readStart := time.Now()
					_, err = s.repository.UserRepo().GetByID(ctx, testUser.ID)
					readLatency := time.Since(readStart)
					
					if err == nil {
						atomic.AddInt64(&readCount, 1)
					}
					
					// Update操作
					updateStart := time.Now()
					testUser.Email = fmt.Sprintf("updated_%d_%d@test.com", workerID, j)
					err = s.repository.UserRepo().Update(ctx, testUser)
					updateLatency := time.Since(updateStart)
					
					if err == nil {
						atomic.AddInt64(&updateCount, 1)
					}
					
					totalReqLatency := createLatency + readLatency + updateLatency
					
					mutex.Lock()
					totalLatency += totalReqLatency
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests*3) // 3个操作
		qps := float64(totalRequests*3) / totalTime.Seconds()
		
		t.Logf("用户Repository CRUD性能结果:")
		t.Logf("  总QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  Create成功: %d/%d", createCount, totalRequests)
		t.Logf("  Read成功: %d/%d", readCount, totalRequests)
		t.Logf("  Update成功: %d/%d", updateCount, totalRequests)
		
		// 性能断言
		assert.True(t, qps >= TARGET_REPOSITORY_CRUD_QPS,
			"Repository CRUD QPS不达标: 期望>=%d, 实际=%.2f", TARGET_REPOSITORY_CRUD_QPS, qps)
		assert.True(t, avgLatency <= TARGET_REPOSITORY_LATENCY,
			"Repository操作延迟过高: 期望<=%v, 实际=%v", TARGET_REPOSITORY_LATENCY, avgLatency)
	})
	
	// 测试查询历史Repository性能
	t.Run("查询历史Repository_批量操作性能", func(t *testing.T) {
		// 批量插入性能测试
		batchSize := 100
		batches := 10
		totalInserts := batchSize * batches
		
		var successCount int64
		
		startTime := time.Now()
		
		for i := 0; i < batches; i++ {
			// 准备批量数据
			queries := make([]*repository.QueryHistory, batchSize)
			for j := 0; j < batchSize; j++ {
				execTime := int32(50 + j)
				generatedSQL := fmt.Sprintf("SELECT %d as batch_test", j)
				sqlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(generatedSQL)))
				queries[j] = &repository.QueryHistory{
					BaseModel: repository.BaseModel{
						CreateBy: &s.testUser.ID,  // 设置创建者指针
						UpdateBy: &s.testUser.ID,  // 设置更新者指针
					},
					UserID:        s.testUser.ID,
					ConnectionID:  &s.testConnection.ID,
					NaturalQuery:  fmt.Sprintf("批量测试查询 %d-%d", i, j),
					GeneratedSQL:  generatedSQL,
					SQLHash:       sqlHash,
					ExecutionTime: &execTime,
					Status:        string(repository.QuerySuccess),
				}
			}
			
			// 批量插入
			for _, query := range queries {
				err := s.repository.QueryHistoryRepo().Create(ctx, query)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					// 记录第一个错误以便诊断
					if atomic.LoadInt64(&successCount) == 0 {
						t.Logf("批量插入第一个错误: %v", err)
					}
				}
			}
		}
		
		totalTime := time.Since(startTime)
		qps := float64(totalInserts) / totalTime.Seconds()
		
		t.Logf("查询历史批量插入性能结果:")
		t.Logf("  批量大小: %d, 批次数: %d", batchSize, batches)
		t.Logf("  总插入数: %d", totalInserts)
		t.Logf("  成功插入: %d", successCount)
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  总耗时: %v", totalTime)
		
		assert.True(t, qps >= 200, "批量插入QPS应该达到200")
		assert.Equal(t, int64(totalInserts), successCount, "所有插入都应该成功")
	})
}

// testEndToEndAPIPerformance 测试端到端API性能
func (s *RealBusinessPerformanceTestSuite) testEndToEndAPIPerformance(t *testing.T) {
	t.Log("=== 开始端到端API性能测试 ===")
	
	// 设置测试路由器（模拟真实API）
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// 添加中间件
	router.Use(gin.Recovery())
	
	// 模拟认证中间件
	router.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := s.jwtService.ValidateAccessToken(tokenString)
			if err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
			}
		}
		c.Next()
	})
	
	// 添加业务API端点
	router.POST("/api/query/execute", func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		
		// 模拟SQL执行
		result, err := s.sqlExecutor.ExecuteQuery(
			c.Request.Context(),
			"SELECT current_timestamp as now",
			s.testConnection,
		)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"result":  result,
		})
	})
	
	router.GET("/api/user/profile", func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}
		
		// 查询用户信息
		user, err := s.repository.UserRepo().GetByID(c.Request.Context(), userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"role":     user.Role,
			},
		})
	})
	
	// 生成有效的JWT Token
	tokenPair, err := s.jwtService.GenerateTokenPair(
		s.testUser.ID,
		s.testUser.Username,
		string(s.testUser.Role),
	)
	require.NoError(t, err)
	
	// 测试认证API性能
	t.Run("认证API端到端性能", func(t *testing.T) {
		concurrency := 30
		requestsPerWorker := 50
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试认证API端到端性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 构建HTTP请求
					req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
					req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
					resp := httptest.NewRecorder()
					
					// 执行请求
					router.ServeHTTP(resp, req)
					
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if resp.Code == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("认证API端到端性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		
		// 端到端API性能断言
		assert.True(t, qps >= 300, "认证API QPS应该达到300")
		assert.True(t, avgLatency <= 100*time.Millisecond, "端到端延迟应该小于100ms")
		assert.True(t, float64(successCount)/float64(totalRequests) > 0.99, "成功率应该>99%")
	})
}

// testConcurrentBusinessScenario 测试并发业务场景
func (s *RealBusinessPerformanceTestSuite) testConcurrentBusinessScenario(t *testing.T) {
	t.Log("=== 开始并发业务场景压力测试 ===")
	
	ctx := context.Background()
	
	// 模拟真实业务场景：用户同时进行多种操作
	t.Run("混合业务场景压力测试", func(t *testing.T) {
		duration := 30 * time.Second
		concurrency := 20
		
		var (
			jwtOperations    int64
			sqlOperations    int64
			repoOperations   int64
			poolOperations   int64
			errorCount       int64
		)
		
		startTime := time.Now()
		endTime := startTime.Add(duration)
		
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("混合业务场景压力测试: 并发数=%d, 持续时间=%v", concurrency, duration)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				operationCounter := 0
				for time.Now().Before(endTime) {
					operationCounter++
					scenario := operationCounter % 4 // 循环4种场景
					
					switch scenario {
					case 0: // JWT操作场景
						_, err := s.jwtService.GenerateTokenPair(
							s.testUser.ID+int64(workerID),
							fmt.Sprintf("user_%d", workerID),
							"user",
						)
						if err == nil {
							atomic.AddInt64(&jwtOperations, 1)
						} else {
							atomic.AddInt64(&errorCount, 1)
						}
						
					case 1: // SQL执行场景
						_, err := s.sqlExecutor.ExecuteQuery(
							ctx,
							fmt.Sprintf("SELECT %d as worker_id, current_timestamp as ts", workerID),
							s.testConnection,
						)
						if err == nil {
							atomic.AddInt64(&sqlOperations, 1)
						} else {
							atomic.AddInt64(&errorCount, 1)
						}
						
					case 2: // Repository操作场景
						_, err := s.repository.UserRepo().GetByID(ctx, s.testUser.ID)
						if err == nil {
							atomic.AddInt64(&repoOperations, 1)
						} else {
							atomic.AddInt64(&errorCount, 1)
						}
						
					case 3: // 连接池操作场景
						_, err := s.connectionManager.GetConnectionPool(ctx, s.testConnection.ID)
						if err == nil {
							atomic.AddInt64(&poolOperations, 1)
						} else {
							atomic.AddInt64(&errorCount, 1)
						}
					}
					
					// 短暂休眠，模拟真实用户操作间隔
					time.Sleep(time.Millisecond * 10)
				}
			}(i)
		}
		
		wg.Wait()
		actualDuration := time.Since(startTime)
		
		totalOperations := jwtOperations + sqlOperations + repoOperations + poolOperations
		totalQPS := float64(totalOperations) / actualDuration.Seconds()
		errorRate := float64(errorCount) / float64(totalOperations+errorCount) * 100
		
		t.Logf("混合业务场景压力测试结果:")
		t.Logf("  测试时长: %v", actualDuration)
		t.Logf("  JWT操作: %d", jwtOperations)
		t.Logf("  SQL操作: %d", sqlOperations) 
		t.Logf("  Repository操作: %d", repoOperations)
		t.Logf("  连接池操作: %d", poolOperations)
		t.Logf("  总操作数: %d", totalOperations)
		t.Logf("  总QPS: %.2f", totalQPS)
		t.Logf("  错误数: %d", errorCount)
		t.Logf("  错误率: %.2f%%", errorRate)
		
		// 压力测试断言
		assert.True(t, totalOperations > 1000, "总操作数应该超过1000")
		assert.True(t, totalQPS >= 100, "总QPS应该达到100")
		assert.True(t, errorRate <= 5.0, "错误率应该小于5%")
	})
}

// testTransactionPerformance 测试数据库事务性能
func (s *RealBusinessPerformanceTestSuite) testTransactionPerformance(t *testing.T) {
	t.Log("=== 开始数据库事务性能测试 ===")
	
	ctx := context.Background()
	
	t.Run("事务CRUD性能", func(t *testing.T) {
		concurrency := 10
		requestsPerWorker := 20
		totalRequests := concurrency * requestsPerWorker
		
		var successCount int64
		var totalLatency time.Duration
		var mutex sync.Mutex
		
		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)
		
		t.Logf("测试事务CRUD性能: 并发数=%d, 总请求数=%d", concurrency, totalRequests)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					reqStart := time.Now()
					
					// 开始事务
					txRepo, err := s.repository.BeginTx(ctx)
					if err != nil {
						t.Logf("开始事务失败 worker-%d req-%d: %v", workerID, j, err)
						continue
					}
					
					// 在事务中执行多个操作
					testUser := &repository.User{
						BaseModel: repository.BaseModel{
							CreateBy: &s.testUser.ID,  // 设置创建者指针
							UpdateBy: &s.testUser.ID,  // 设置更新者指针
						},
						Username:     fmt.Sprintf("tx_test_%d_%d_%d", workerID, j, time.Now().UnixNano()),
						Email:        fmt.Sprintf("tx_test_%d_%d_%d@test.com", workerID, j, time.Now().UnixNano()),
						PasswordHash: "tx_test_hash",
						Role:         string(repository.RoleUser),
						Status:       string(repository.StatusActive),
					}
					
					err = txRepo.UserRepo().Create(ctx, testUser)
					if err != nil {
						t.Logf("创建用户失败 worker-%d req-%d: %v", workerID, j, err)
						txRepo.Rollback()
						continue
					}
					
					// 创建查询历史
					execTime := int32(100)
					generatedSQL := "SELECT 1"
					sqlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(generatedSQL)))
					queryHistory := &repository.QueryHistory{
						BaseModel: repository.BaseModel{
							CreateBy: &s.testUser.ID,  // 设置创建者指针
							UpdateBy: &s.testUser.ID,  // 设置更新者指针
						},
						UserID:        testUser.ID,
						ConnectionID:  &s.testConnection.ID,
						NaturalQuery:  fmt.Sprintf("事务测试查询 %d-%d", workerID, j),
						GeneratedSQL:  generatedSQL,
						SQLHash:       sqlHash,
						ExecutionTime: &execTime,
						Status:        string(repository.QuerySuccess),
					}
					
					err = txRepo.QueryHistoryRepo().Create(ctx, queryHistory)
					if err != nil {
						t.Logf("创建查询历史失败 worker-%d req-%d: %v", workerID, j, err)
						txRepo.Rollback()
						continue
					}
					
					// 提交事务
					err = txRepo.Commit()
					reqLatency := time.Since(reqStart)
					
					mutex.Lock()
					totalLatency += reqLatency
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					} else {
						// 记录第一个事务提交错误
						if atomic.LoadInt64(&successCount) == 0 {
							t.Logf("事务提交第一个错误: %v", err)
						}
					}
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		totalTime := time.Since(startTime)
		
		avgLatency := totalLatency / time.Duration(totalRequests)
		qps := float64(totalRequests) / totalTime.Seconds()
		
		t.Logf("事务CRUD性能结果:")
		t.Logf("  QPS: %.2f", qps)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)
		
		// 事务性能断言
		assert.True(t, qps >= 50, "事务QPS应该达到50")
		assert.True(t, avgLatency <= 200*time.Millisecond, "事务平均延迟应该小于200ms")
		assert.True(t, float64(successCount)/float64(totalRequests) > 0.95, "事务成功率应该>95%")
	})
}