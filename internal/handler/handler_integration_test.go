package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"chat2sql-go/internal/auth"
	"chat2sql-go/internal/repository"
	"chat2sql-go/internal/service"
)

// MockSQLExecutor Mock SQL执行器
type MockSQLExecutor struct {
	mock.Mock
}

func (m *MockSQLExecutor) ExecuteQuery(ctx context.Context, sql string, connection *repository.DatabaseConnection) (*service.QueryResult, error) {
	args := m.Called(ctx, sql, connection)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*service.QueryResult), args.Error(1)
}

// MockAuthMiddleware Mock认证中间件
type MockAuthMiddleware struct {
	shouldAuthenticate bool
	userID             int64
}

func NewMockAuthMiddleware(shouldAuthenticate bool, userID int64) *MockAuthMiddleware {
	return &MockAuthMiddleware{
		shouldAuthenticate: shouldAuthenticate,
		userID:             userID,
	}
}

func (m *MockAuthMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.shouldAuthenticate {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		// 设置用户ID到上下文
		c.Set("user_id", m.userID)
		c.Next()
	}
}

// MockQueryHistoryRepository Mock查询历史仓库
type MockQueryHistoryRepository struct {
	mock.Mock
}

func (m *MockQueryHistoryRepository) Create(ctx context.Context, query *repository.QueryHistory) error {
	args := m.Called(ctx, query)
	return args.Error(0)
}

func (m *MockQueryHistoryRepository) Update(ctx context.Context, query *repository.QueryHistory) error {
	args := m.Called(ctx, query)
	return args.Error(0)
}

func (m *MockQueryHistoryRepository) GetByID(ctx context.Context, id int64) (*repository.QueryHistory, error) {
	args := m.Called(ctx, id)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQueryHistoryRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) ListByConnection(ctx context.Context, connectionID int64, limit, offset int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, connectionID, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) ListByStatus(ctx context.Context, status repository.QueryStatus, limit, offset int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, status, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) ListRecent(ctx context.Context, userID int64, hours int, limit int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, userID, hours, limit)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueryHistoryRepository) CountByStatus(ctx context.Context, status repository.QueryStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueryHistoryRepository) GetExecutionStats(ctx context.Context, userID int64, days int) (*repository.QueryExecutionStats, error) {
	args := m.Called(ctx, userID, days)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.QueryExecutionStats), args.Error(1)
}

func (m *MockQueryHistoryRepository) GetPopularQueries(ctx context.Context, limit int, days int) ([]*repository.PopularQuery, error) {
	args := m.Called(ctx, limit, days)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.PopularQuery), args.Error(1)
}

func (m *MockQueryHistoryRepository) GetSlowQueries(ctx context.Context, minExecutionTime int32, limit int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, minExecutionTime, limit)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) SearchByNaturalQuery(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, userID, keyword, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) SearchBySQL(ctx context.Context, userID int64, keyword string, limit, offset int) ([]*repository.QueryHistory, error) {
	args := m.Called(ctx, userID, keyword, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.QueryHistory), args.Error(1)
}

func (m *MockQueryHistoryRepository) BatchUpdateStatus(ctx context.Context, queryIDs []int64, status repository.QueryStatus) error {
	args := m.Called(ctx, queryIDs, status)
	return args.Error(0)
}

func (m *MockQueryHistoryRepository) CleanupOldQueries(ctx context.Context, beforeDate time.Time) (int64, error) {
	args := m.Called(ctx, beforeDate)
	return args.Get(0).(int64), args.Error(1)
}

// MockConnectionRepository Mock连接仓库
type MockConnectionRepository struct {
	mock.Mock
}

func (m *MockConnectionRepository) Create(ctx context.Context, connection *repository.DatabaseConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockConnectionRepository) GetByID(ctx context.Context, id int64) (*repository.DatabaseConnection, error) {
	args := m.Called(ctx, id)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) Update(ctx context.Context, connection *repository.DatabaseConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockConnectionRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConnectionRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx, userID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) ListByType(ctx context.Context, dbType repository.DatabaseType) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx, dbType)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) ListByStatus(ctx context.Context, status repository.ConnectionStatus) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx, status)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) GetByUserAndName(ctx context.Context, userID int64, name string) (*repository.DatabaseConnection, error) {
	args := m.Called(ctx, userID, name)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) CountByStatus(ctx context.Context, status repository.ConnectionStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) CountByType(ctx context.Context, dbType repository.DatabaseType) (int64, error) {
	args := m.Called(ctx, dbType)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockConnectionRepository) UpdateStatus(ctx context.Context, connectionID int64, status repository.ConnectionStatus) error {
	args := m.Called(ctx, connectionID, status)
	return args.Error(0)
}

func (m *MockConnectionRepository) UpdateLastTested(ctx context.Context, connectionID int64, testTime time.Time) error {
	args := m.Called(ctx, connectionID, testTime)
	return args.Error(0)
}

func (m *MockConnectionRepository) BatchUpdateStatus(ctx context.Context, connectionIDs []int64, status repository.ConnectionStatus) error {
	args := m.Called(ctx, connectionIDs, status)
	return args.Error(0)
}

func (m *MockConnectionRepository) ExistsByUserAndName(ctx context.Context, userID int64, name string) (bool, error) {
	args := m.Called(ctx, userID, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockConnectionRepository) GetActiveConnections(ctx context.Context) ([]*repository.DatabaseConnection, error) {
	args := m.Called(ctx)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.DatabaseConnection), args.Error(1)
}

// MockHealthService Mock健康检查服务
type MockHealthService struct {
	mock.Mock
}

func (m *MockHealthService) CheckHealth(ctx context.Context) *service.HealthCheckResult {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*service.HealthCheckResult)
}

func (m *MockHealthService) CheckReadiness(ctx context.Context) *service.ReadinessResult {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*service.ReadinessResult)
}

func (m *MockHealthService) GetVersionInfo() map[string]interface{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]interface{})
}

func (m *MockHealthService) GetApplicationStatus(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

// MockUserRepository Mock用户Repository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *repository.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	args := m.Called(ctx, id)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	args := m.Called(ctx, username)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	args := m.Called(ctx, email)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *repository.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	args := m.Called(ctx, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.User), args.Error(1)
}

func (m *MockUserRepository) ListByRole(ctx context.Context, role repository.UserRole, limit, offset int) ([]*repository.User, error) {
	args := m.Called(ctx, role, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.User), args.Error(1)
}

func (m *MockUserRepository) ListByStatus(ctx context.Context, status repository.UserStatus, limit, offset int) ([]*repository.User, error) {
	args := m.Called(ctx, status, limit, offset)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.User), args.Error(1)
}

func (m *MockUserRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockUserRepository) CountByStatus(ctx context.Context, status repository.UserStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockUserRepository) ValidateCredentials(ctx context.Context, username, passwordHash string) (*repository.User, error) {
	args := m.Called(ctx, username, passwordHash)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.User), args.Error(1)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, userID int64, newPasswordHash string) error {
	args := m.Called(ctx, userID, newPasswordHash)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, userID int64, loginTime time.Time) error {
	args := m.Called(ctx, userID, loginTime)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateStatus(ctx context.Context, userID int64, status repository.UserStatus) error {
	args := m.Called(ctx, userID, status)
	return args.Error(0)
}

func (m *MockUserRepository) BatchUpdateStatus(ctx context.Context, userIDs []int64, status repository.UserStatus) error {
	args := m.Called(ctx, userIDs, status)
	return args.Error(0)
}

func (m *MockUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

// MockJWTService Mock JWT服务
type MockJWTService struct {
	mock.Mock
}

func (m *MockJWTService) GenerateTokenPair(userID int64, username, role string) (*auth.TokenPair, error) {
	args := m.Called(userID, username, role)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.TokenPair), args.Error(1)
}

func (m *MockJWTService) ValidateRefreshToken(tokenString string) (*auth.CustomClaims, error) {
	args := m.Called(tokenString)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.CustomClaims), args.Error(1)
}

func (m *MockJWTService) ValidateTokenFromRequest(authHeader string) (*auth.CustomClaims, error) {
	args := m.Called(authHeader)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.CustomClaims), args.Error(1)
}

func (m *MockJWTService) RefreshTokenPair(refreshTokenString string) (*auth.TokenPair, error) {
	args := m.Called(refreshTokenString)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*auth.TokenPair), args.Error(1)
}

// MockSchemaRepository Mock Schema Repository
type MockSchemaRepository struct {
	mock.Mock
}

func (m *MockSchemaRepository) Create(ctx context.Context, schema *repository.SchemaMetadata) error {
	args := m.Called(ctx, schema)
	return args.Error(0)
}

func (m *MockSchemaRepository) GetByID(ctx context.Context, id int64) (*repository.SchemaMetadata, error) {
	args := m.Called(ctx, id)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) Update(ctx context.Context, schema *repository.SchemaMetadata) error {
	args := m.Called(ctx, schema)
	return args.Error(0)
}

func (m *MockSchemaRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSchemaRepository) ListByConnection(ctx context.Context, connectionID int64) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) ListByTable(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID, schemaName, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) GetTableStructure(ctx context.Context, connectionID int64, schemaName, tableName string) ([]*repository.SchemaMetadata, error) {
	args := m.Called(ctx, connectionID, schemaName, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.SchemaMetadata), args.Error(1)
}

func (m *MockSchemaRepository) ListTables(ctx context.Context, connectionID int64, schemaName string) ([]string, error) {
	args := m.Called(ctx, connectionID, schemaName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]string), args.Error(1)
}

func (m *MockSchemaRepository) ListSchemas(ctx context.Context, connectionID int64) ([]string, error) {
	args := m.Called(ctx, connectionID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]string), args.Error(1)
}

func (m *MockSchemaRepository) BatchCreate(ctx context.Context, schemas []*repository.SchemaMetadata) error {
	args := m.Called(ctx, schemas)
	return args.Error(0)
}

func (m *MockSchemaRepository) BatchDelete(ctx context.Context, connectionID int64) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockSchemaRepository) RefreshConnectionMetadata(ctx context.Context, connectionID int64, schemas []*repository.SchemaMetadata) error {
	args := m.Called(ctx, connectionID, schemas)
	return args.Error(0)
}

func (m *MockSchemaRepository) SearchTables(ctx context.Context, connectionID int64, keyword string) ([]*repository.TableInfo, error) {
	args := m.Called(ctx, connectionID, keyword)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.TableInfo), args.Error(1)
}

func (m *MockSchemaRepository) SearchColumns(ctx context.Context, connectionID int64, keyword string) ([]*repository.ColumnInfo, error) {
	args := m.Called(ctx, connectionID, keyword)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.ColumnInfo), args.Error(1)
}

func (m *MockSchemaRepository) GetRelatedTables(ctx context.Context, connectionID int64, tableName string) ([]*repository.TableRelation, error) {
	args := m.Called(ctx, connectionID, tableName)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]*repository.TableRelation), args.Error(1)
}

func (m *MockSchemaRepository) CountByConnection(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSchemaRepository) GetTableCount(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSchemaRepository) GetColumnCount(ctx context.Context, connectionID int64) (int64, error) {
	args := m.Called(ctx, connectionID)
	return args.Get(0).(int64), args.Error(1)
}

// MockConnectionManager Mock连接管理器
type MockConnectionManager struct {
	mock.Mock
}

func (m *MockConnectionManager) TestConnection(ctx context.Context, conn *repository.DatabaseConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockConnectionManager) CreateConnection(ctx context.Context, conn *repository.DatabaseConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockConnectionManager) GetConnection(ctx context.Context, connectionID int64) (*repository.DatabaseConnection, error) {
	args := m.Called(ctx, connectionID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*repository.DatabaseConnection), args.Error(1)
}

func (m *MockConnectionManager) CloseConnection(ctx context.Context, connectionID int64) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockConnectionManager) UpdateConnection(ctx context.Context, conn *repository.DatabaseConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

// HandlerIntegrationTestSuite Handler集成测试套件
type HandlerIntegrationTestSuite struct {
	router              *gin.Engine
	mockSQLExecutor     *MockSQLExecutor
	mockQueryRepo       *MockQueryHistoryRepository
	mockConnectionRepo  *MockConnectionRepository
	mockHealthService   *MockHealthService
	mockAuthMiddleware  *MockAuthMiddleware
	sqlHandler          *SQLHandler
	logger              *zap.Logger
}

// setupTestRouter 设置测试路由
func (suite *HandlerIntegrationTestSuite) setupTestRouter(authenticated bool, userID int64) *gin.Engine {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	
	suite.logger = zaptest.NewLogger(&testing.T{})
	suite.mockSQLExecutor = new(MockSQLExecutor)
	suite.mockQueryRepo = new(MockQueryHistoryRepository)
	suite.mockConnectionRepo = new(MockConnectionRepository)
	// 只有在mockHealthService为nil时才创建新的
	if suite.mockHealthService == nil {
		suite.mockHealthService = new(MockHealthService)
	}
	suite.mockAuthMiddleware = NewMockAuthMiddleware(authenticated, userID)
	
	// 创建SQLHandler
	suite.sqlHandler = NewSQLHandler(
		suite.mockQueryRepo,
		suite.mockConnectionRepo,
		suite.mockSQLExecutor,
		suite.logger,
	)
	
	// 创建路由配置
	routerConfig := &RouterConfig{
		SQLHandler:      suite.sqlHandler,
		AuthMiddleware:  suite.mockAuthMiddleware,
		HealthService:   suite.mockHealthService,
	}
	
	router := gin.New()
	SetupRoutes(router, routerConfig)
	
	return router
}

// TestSQLHandler_ExecuteSQL_Success 测试SQL执行成功场景
func TestSQLHandler_ExecuteSQL_Success(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	// 准备Mock响应
	mockConnection := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		UserID:       1,
		Name:         "test_db",
		Host:         "localhost",
		Port:         5432,
		DatabaseName: "testdb",
	}
	
	mockQueryResult := &service.QueryResult{
		ExecutionTime: 150,
		RowCount:     10,
		Status:       string(repository.QuerySuccess),
		Rows: []map[string]any{
			{"id": 1, "name": "John"},
			{"id": 2, "name": "Jane"},
		},
	}
	
	suite.mockConnectionRepo.On("GetByID", mock.Anything, int64(1)).Return(mockConnection, nil)
	suite.mockSQLExecutor.On("ExecuteQuery", mock.Anything, "SELECT * FROM users LIMIT 10", mockConnection).Return(mockQueryResult, nil)
	suite.mockQueryRepo.On("Create", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).Return(nil)
	suite.mockQueryRepo.On("Update", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).Return(nil)
	
	// 准备请求数据
	requestBody := ExecuteSQLRequest{
		SQL:          "SELECT * FROM users LIMIT 10",
		NaturalQuery: "获取前10个用户",
		ConnectionID: 1,
	}
	jsonData, _ := json.Marshal(requestBody)
	
	// 发送请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response SQLExecutionResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int32(150), response.ExecutionTime)
	assert.Equal(t, int32(10), response.RowCount)
	assert.Equal(t, string(repository.QuerySuccess), response.Status)
	assert.Len(t, response.Data, 2)
	
	// 验证Mock调用
	suite.mockConnectionRepo.AssertExpectations(t)
	suite.mockSQLExecutor.AssertExpectations(t)
	suite.mockQueryRepo.AssertExpectations(t)
}

// TestSQLHandler_ExecuteSQL_Unauthorized 测试未授权访问
func TestSQLHandler_ExecuteSQL_Unauthorized(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(false, 0) // 未认证
	
	requestBody := ExecuteSQLRequest{
		SQL:          "SELECT * FROM users",
		ConnectionID: 1,
	}
	jsonData, _ := json.Marshal(requestBody)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestSQLHandler_ExecuteSQL_SQLSecurityValidationFailed 测试SQL安全验证失败
func TestSQLHandler_ExecuteSQL_SQLSecurityValidationFailed(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	requestBody := ExecuteSQLRequest{
		SQL:          "DROP TABLE users", // 危险SQL
		ConnectionID: 1,
	}
	jsonData, _ := json.Marshal(requestBody)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusForbidden, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "SQL_FORBIDDEN", response.Code)
}

// TestSQLHandler_ExecuteSQL_InvalidConnectionAccess 测试无效连接访问
func TestSQLHandler_ExecuteSQL_InvalidConnectionAccess(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	// 模拟连接属于其他用户
	mockConnection := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		UserID: 2, // 不同的用户ID
		Name:   "test_db",
	}
	
	suite.mockConnectionRepo.On("GetByID", mock.Anything, int64(1)).Return(mockConnection, nil)
	
	requestBody := ExecuteSQLRequest{
		SQL:          "SELECT * FROM users",
		ConnectionID: 1,
	}
	jsonData, _ := json.Marshal(requestBody)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusForbidden, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "CONNECTION_FORBIDDEN", response.Code)
	
	suite.mockConnectionRepo.AssertExpectations(t)
}

// TestSQLHandler_GetQueryHistory_Success 测试获取查询历史成功
func TestSQLHandler_GetQueryHistory_Success(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	now := time.Now()
	mockQueries := []*repository.QueryHistory{
		{
			BaseModel: repository.BaseModel{
				ID:         1,
				CreateTime: now,
			},
			UserID:       1,
			NaturalQuery: "获取用户列表",
			GeneratedSQL: "SELECT * FROM users",
			Status:       string(repository.QuerySuccess),
		},
		{
			BaseModel: repository.BaseModel{
				ID:         2,
				CreateTime: now.Add(-time.Hour),
			},
			UserID:       1,
			NaturalQuery: "获取订单统计",
			GeneratedSQL: "SELECT COUNT(*) FROM orders",
			Status:       string(repository.QuerySuccess),
		},
	}
	
	suite.mockQueryRepo.On("ListByUser", mock.Anything, int64(1), 20, 0).Return(mockQueries, nil)
	suite.mockQueryRepo.On("CountByUser", mock.Anything, int64(1)).Return(int64(2), nil)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response QueryHistoryResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Queries, 2)
	assert.Equal(t, int64(2), response.Total)
	assert.Equal(t, 1, response.Page)
	assert.False(t, response.HasMore)
	
	suite.mockQueryRepo.AssertExpectations(t)
}

// TestSQLHandler_GetQueryHistory_WithSearchKeyword 测试使用搜索关键词查询历史
func TestSQLHandler_GetQueryHistory_WithSearchKeyword(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	now := time.Now()
	mockQueries := []*repository.QueryHistory{
		{
			BaseModel: repository.BaseModel{
				ID:         1,
				CreateTime: now,
			},
			UserID:       1,
			NaturalQuery: "获取用户列表",
			GeneratedSQL: "SELECT * FROM users",
			Status:       string(repository.QuerySuccess),
		},
	}
	
	suite.mockQueryRepo.On("SearchByNaturalQuery", mock.Anything, int64(1), "用户", 20, 0).Return(mockQueries, nil)
	suite.mockQueryRepo.On("CountByUser", mock.Anything, int64(1)).Return(int64(1), nil)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history?keyword=用户", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response QueryHistoryResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Queries, 1)
	assert.Contains(t, response.Queries[0].NaturalQuery, "用户")
	
	suite.mockQueryRepo.AssertExpectations(t)
}

// TestSQLHandler_GetQueryById_Success 测试根据ID获取查询成功
func TestSQLHandler_GetQueryById_Success(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	now := time.Now()
	mockQuery := &repository.QueryHistory{
		BaseModel: repository.BaseModel{
			ID:         123,
			CreateTime: now,
		},
		UserID:       1,
		NaturalQuery: "获取用户详情",
		GeneratedSQL: "SELECT * FROM users WHERE id = 1",
		Status:       string(repository.QuerySuccess),
	}
	
	suite.mockQueryRepo.On("GetByID", mock.Anything, int64(123)).Return(mockQuery, nil)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history/123", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response QueryHistoryItem
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), response.ID)
	assert.Equal(t, "获取用户详情", response.NaturalQuery)
	assert.Equal(t, "SELECT * FROM users WHERE id = 1", response.GeneratedSQL)
	
	suite.mockQueryRepo.AssertExpectations(t)
}

// TestSQLHandler_GetQueryById_AccessDenied 测试访问其他用户的查询被拒绝
func TestSQLHandler_GetQueryById_AccessDenied(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	// 模拟查询属于其他用户
	mockQuery := &repository.QueryHistory{
		BaseModel: repository.BaseModel{
			ID: 123,
		},
		UserID:       2, // 不同的用户ID
		NaturalQuery: "获取用户详情",
		GeneratedSQL: "SELECT * FROM users WHERE id = 1",
	}
	
	suite.mockQueryRepo.On("GetByID", mock.Anything, int64(123)).Return(mockQuery, nil)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history/123", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusForbidden, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ACCESS_DENIED", response.Code)
	
	suite.mockQueryRepo.AssertExpectations(t)
}

// TestSQLHandler_ValidateSQL_Success 测试SQL验证成功
func TestSQLHandler_ValidateSQL_Success(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	requestBody := ValidateSQLRequest{
		SQL: "SELECT * FROM users WHERE status = 'active'",
	}
	jsonData, _ := json.Marshal(requestBody)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/validate", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response SQLValidationResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.IsValid)
	assert.Equal(t, "SELECT", response.QueryType)
	assert.True(t, response.IsReadOnly)
}

// TestSQLHandler_ValidateSQL_InvalidSQL 测试无效SQL验证
func TestSQLHandler_ValidateSQL_InvalidSQL(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	requestBody := ValidateSQLRequest{
		SQL: "DELETE FROM users WHERE id = 1", // 不允许的操作
	}
	jsonData, _ := json.Marshal(requestBody)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/validate", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response SQLValidationResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.IsValid)
	assert.NotEmpty(t, response.Errors)
}

// TestHealthEndpoints 测试健康检查端点
func TestHealthEndpoints(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	
	// 测试有HealthService的情况
	suite.mockHealthService = new(MockHealthService)
	mockHealthResult := &service.HealthCheckResult{
		Status:      service.HealthStatusHealthy,
		Timestamp:   time.Now(),
		Service:     "chat2sql-api",
		Version:     "0.1.0",
		Environment: "test",
		Components: map[string]service.ComponentStatus{
			"database": {
				Status:    service.HealthStatusHealthy,
				Message:   "Database connection is healthy",
				Timestamp: time.Now(),
			},
		},
	}
	
	mockReadinessResult := &service.ReadinessResult{
		Status:    service.HealthStatusHealthy,
		Timestamp: time.Now(),
		Components: map[string]service.ComponentStatus{
			"database": {
				Status:    service.HealthStatusHealthy,
				Message:   "Database connection is healthy",
				Timestamp: time.Now(),
			},
		},
	}
	
	mockVersionInfo := map[string]interface{}{
		"version":    "0.1.0",
		"git_commit": "abc123",
		"build_time": time.Now().Format(time.RFC3339),
	}
	
	suite.mockHealthService.On("CheckHealth", mock.Anything).Return(mockHealthResult)
	suite.mockHealthService.On("CheckReadiness", mock.Anything).Return(mockReadinessResult)
	suite.mockHealthService.On("GetVersionInfo").Return(mockVersionInfo)
	
	router := suite.setupTestRouter(false, 0) // 健康检查不需要认证
	
	// 测试 /health
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// 测试 /ready
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// 测试 /version
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/version", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	suite.mockHealthService.AssertExpectations(t)
}

// TestSimpleHealthEndpoints 测试简单健康检查端点（没有HealthService）
func TestSimpleHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// 创建没有HealthService的路由配置
	routerConfig := &RouterConfig{
		HealthService: nil, // 没有健康服务
	}
	
	router := gin.New()
	SetupRoutes(router, routerConfig)
	
	// 测试 /health - 应该使用简单实现
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	var healthResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &healthResponse)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", healthResponse["status"])
	assert.Equal(t, "chat2sql-api", healthResponse["service"])
	
	// 测试 /ready
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// 测试 /version
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/version", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterMiddleware 测试路由中间件
func TestRouterMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// 设置全局中间件
	setupGlobalMiddleware(router)
	
	// 添加测试路由
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	// 验证CORS头
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	
	// 验证安全头
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

// TestOPTIONSRequest 测试OPTIONS预检请求
func TestOPTIONSRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	setupGlobalMiddleware(router)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/api/v1/sql/execute", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

// TestInvalidJSONRequest 测试无效JSON请求
func TestInvalidJSONRequest(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	// 发送无效JSON
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", response.Code)
}

// TestSQLHandler_InvalidQueryID 测试无效查询ID
func TestSQLHandler_InvalidQueryID(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history/invalid", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_QUERY_ID", response.Code)
}

// TestSQLHandler_QueryHistoryPagination 测试查询历史分页
func TestSQLHandler_QueryHistoryPagination(t *testing.T) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	mockQueries := []*repository.QueryHistory{
		{
			BaseModel: repository.BaseModel{
				ID:         1,
				CreateTime: time.Now(),
			},
			UserID:       1,
			NaturalQuery: "Query 1",
		},
		{
			BaseModel: repository.BaseModel{
				ID:         2,
				CreateTime: time.Now(),
			},
			UserID:       1,
			NaturalQuery: "Query 2",
		},
	}
	
	suite.mockQueryRepo.On("ListByUser", mock.Anything, int64(1), 10, 10).Return(mockQueries, nil)
	suite.mockQueryRepo.On("CountByUser", mock.Anything, int64(1)).Return(int64(25), nil)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sql/history?limit=10&offset=10", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response QueryHistoryResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, response.Page)
	assert.Equal(t, int64(25), response.Total)
	assert.True(t, response.HasMore)
	
	suite.mockQueryRepo.AssertExpectations(t)
}

// BenchmarkSQLHandler_ExecuteSQL 性能测试：SQL执行
func BenchmarkSQLHandler_ExecuteSQL(b *testing.B) {
	suite := &HandlerIntegrationTestSuite{}
	router := suite.setupTestRouter(true, 1)
	
	// 设置Mock响应
	mockConnection := &repository.DatabaseConnection{
		BaseModel: repository.BaseModel{
			ID: 1,
		},
		UserID: 1,
		Name:   "bench_db",
	}
	mockResult := &service.QueryResult{
		ExecutionTime: 10,
		RowCount:     1,
		Status:       string(repository.QuerySuccess),
		Rows:         []map[string]any{{"result": "ok"}},
	}
	
	suite.mockConnectionRepo.On("GetByID", mock.Anything, int64(1)).Return(mockConnection, nil)
	suite.mockSQLExecutor.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mockConnection).Return(mockResult, nil)
	suite.mockQueryRepo.On("Create", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).Return(nil)
	suite.mockQueryRepo.On("Update", mock.Anything, mock.AnythingOfType("*repository.QueryHistory")).Return(nil)
	
	requestBody := ExecuteSQLRequest{
		SQL:          "SELECT 1",
		ConnectionID: 1,
	}
	jsonData, _ := json.Marshal(requestBody)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/sql/execute", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}