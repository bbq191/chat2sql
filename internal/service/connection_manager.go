package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"chat2sql-go/internal/repository"
)

// ConnectionManager 多数据库连接管理器
// 提供动态数据库连接管理，支持连接池复用、密码加密存储和健康检查
type ConnectionManager struct {
	// 核心组件
	systemPool     *pgxpool.Pool                 // 系统数据库连接池
	connectionRepo repository.ConnectionRepository // 连接Repository
	encryption     *AESEncryption                // 加密服务
	logger         *zap.Logger                   // 日志器
	
	// 连接池管理
	connectionPools sync.Map                     // 连接池缓存 key: connectionID, value: *ManagedPool
	poolMutex       sync.RWMutex                // 连接池锁
	
	// 配置参数
	maxPoolsPerUser    int           // 每用户最大连接池数量
	poolIdleTimeout    time.Duration // 连接池空闲超时
	connectionTimeout  time.Duration // 连接超时时间
	healthCheckInterval time.Duration // 健康检查间隔
	
	// 状态管理
	isRunning      bool          // 是否运行中
	stopCh         chan struct{} // 停止通道
	healthTicker   *time.Ticker  // 健康检查定时器
}

// ManagedPool 托管连接池
type ManagedPool struct {
	Pool         *pgxpool.Pool              // pgx连接池
	Connection   *repository.DatabaseConnection // 数据库连接配置
	LastUsed     time.Time                  // 最后使用时间
	CreatedAt    time.Time                  // 创建时间
	HealthStatus ConnectionHealthStatus     // 健康状态
	mutex        sync.RWMutex              // 读写锁
}

// ConnectionHealthStatus 连接健康状态
type ConnectionHealthStatus struct {
	IsHealthy    bool      `json:"is_healthy"`    // 是否健康
	LastCheck    time.Time `json:"last_check"`    // 最后检查时间
	ErrorMessage string    `json:"error_message"` // 错误信息
	CheckCount   int64     `json:"check_count"`   // 检查次数
}

// AESEncryption AES加密服务
type AESEncryption struct {
	key []byte // 256位密钥
}

// ConnectionManagerConfig 连接管理器配置
type ConnectionManagerConfig struct {
	EncryptionKey       []byte        `json:"-"`                    // 加密密钥（不序列化）
	MaxPoolsPerUser     int           `json:"max_pools_per_user"`   // 每用户最大连接池数，默认10
	PoolIdleTimeout     time.Duration `json:"pool_idle_timeout"`    // 连接池空闲超时，默认30分钟
	ConnectionTimeout   time.Duration `json:"connection_timeout"`   // 连接超时，默认10秒
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔，默认5分钟
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(
	systemPool *pgxpool.Pool,
	connectionRepo repository.ConnectionRepository,
	encryptionKey []byte,
	logger *zap.Logger,
) (*ConnectionManager, error) {
	config := &ConnectionManagerConfig{
		EncryptionKey:       encryptionKey,
		MaxPoolsPerUser:     10,
		PoolIdleTimeout:     30 * time.Minute,
		ConnectionTimeout:   10 * time.Second,
		HealthCheckInterval: 5 * time.Minute,
	}
	
	return NewConnectionManagerWithConfig(systemPool, connectionRepo, config, logger)
}

// NewConnectionManagerWithConfig 使用自定义配置创建连接管理器
func NewConnectionManagerWithConfig(
	systemPool *pgxpool.Pool,
	connectionRepo repository.ConnectionRepository,
	config *ConnectionManagerConfig,
	logger *zap.Logger,
) (*ConnectionManager, error) {
	if config == nil || len(config.EncryptionKey) == 0 {
		return nil, errors.New("加密密钥不能为空")
	}
	
	// 设置默认值
	if config.MaxPoolsPerUser <= 0 {
		config.MaxPoolsPerUser = 10
	}
	if config.PoolIdleTimeout <= 0 {
		config.PoolIdleTimeout = 30 * time.Minute
	}
	if config.ConnectionTimeout <= 0 {
		config.ConnectionTimeout = 10 * time.Second
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 5 * time.Minute
	}
	
	// 创建AES加密服务
	encryption, err := NewAESEncryption(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("初始化加密服务失败: %w", err)
	}
	
	manager := &ConnectionManager{
		systemPool:          systemPool,
		connectionRepo:      connectionRepo,
		encryption:          encryption,
		logger:              logger,
		maxPoolsPerUser:     config.MaxPoolsPerUser,
		poolIdleTimeout:     config.PoolIdleTimeout,
		connectionTimeout:   config.ConnectionTimeout,
		healthCheckInterval: config.HealthCheckInterval,
		stopCh:              make(chan struct{}),
	}
	
	return manager, nil
}

// Start 启动连接管理器
func (cm *ConnectionManager) Start() error {
	cm.poolMutex.Lock()
	defer cm.poolMutex.Unlock()
	
	if cm.isRunning {
		return errors.New("连接管理器已在运行")
	}
	
	cm.isRunning = true
	
	// 启动健康检查定时器
	cm.healthTicker = time.NewTicker(cm.healthCheckInterval)
	
	go cm.healthCheckRoutine()
	go cm.cleanupRoutine()
	
	cm.logger.Info("连接管理器已启动",
		zap.Duration("health_check_interval", cm.healthCheckInterval),
		zap.Duration("pool_idle_timeout", cm.poolIdleTimeout))
	
	return nil
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() error {
	cm.poolMutex.Lock()
	defer cm.poolMutex.Unlock()
	
	if !cm.isRunning {
		return nil
	}
	
	cm.isRunning = false
	close(cm.stopCh)
	
	if cm.healthTicker != nil {
		cm.healthTicker.Stop()
	}
	
	// 关闭所有连接池
	cm.connectionPools.Range(func(key, value any) bool {
		if pool, ok := value.(*ManagedPool); ok {
			pool.Pool.Close()
		}
		return true
	})
	
	cm.logger.Info("连接管理器已停止")
	return nil
}

// GetConnectionPool 获取数据库连接池
func (cm *ConnectionManager) GetConnectionPool(ctx context.Context, connectionID int64) (*pgxpool.Pool, error) {
	// 从缓存中查找
	if value, ok := cm.connectionPools.Load(connectionID); ok {
		if managedPool, ok := value.(*ManagedPool); ok {
			managedPool.mutex.Lock()
			managedPool.LastUsed = time.Now()
			managedPool.mutex.Unlock()
			return managedPool.Pool, nil
		}
	}
	
	// 缓存中不存在，创建新连接池
	return cm.createConnectionPool(ctx, connectionID)
}

// createConnectionPool 创建新的连接池
func (cm *ConnectionManager) createConnectionPool(ctx context.Context, connectionID int64) (*pgxpool.Pool, error) {
	// 获取连接配置
	connection, err := cm.connectionRepo.GetByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("获取连接配置失败: %w", err)
	}
	
	if connection.Status != string(repository.ConnectionActive) {
		return nil, fmt.Errorf("连接状态异常: %s", connection.Status)
	}
	
	// 检查用户连接池数量限制
	if err := cm.checkUserPoolLimit(connection.UserID); err != nil {
		return nil, err
	}
	
	// 解密密码
	decryptedPassword, err := cm.encryption.Decrypt(connection.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("密码解密失败: %w", err)
	}
	
	// 构建连接字符串
	connStr := cm.buildConnectionString(connection, decryptedPassword)
	
	// 解析连接配置
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("解析连接配置失败: %w", err)
	}
	
	// 设置连接池参数
	cm.configurePoolSettings(config)
	
	// 创建连接池
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}
	
	// 测试连接
	testCtx, cancel := context.WithTimeout(ctx, cm.connectionTimeout)
	defer cancel()
	
	if err := pool.Ping(testCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接测试失败: %w", err)
	}
	
	// 创建托管连接池
	managedPool := &ManagedPool{
		Pool:       pool,
		Connection: connection,
		LastUsed:   time.Now(),
		CreatedAt:  time.Now(),
		HealthStatus: ConnectionHealthStatus{
			IsHealthy: true,
			LastCheck: time.Now(),
		},
	}
	
	// 存储到缓存
	cm.connectionPools.Store(connectionID, managedPool)
	
	cm.logger.Info("创建数据库连接池",
		zap.Int64("connection_id", connectionID),
		zap.String("host", connection.Host),
		zap.String("database", connection.DatabaseName))
	
	return pool, nil
}

// buildConnectionString 构建连接字符串
func (cm *ConnectionManager) buildConnectionString(conn *repository.DatabaseConnection, password string) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=prefer connect_timeout=%d",
		conn.Host,
		conn.Port,
		conn.Username,
		password,
		conn.DatabaseName,
		int(cm.connectionTimeout.Seconds()),
	)
}

// configurePoolSettings 配置连接池设置
func (cm *ConnectionManager) configurePoolSettings(config *pgxpool.Config) {
	config.MaxConns = 10                    // 最大连接数
	config.MinConns = 2                     // 最小连接数
	config.MaxConnLifetime = 1 * time.Hour  // 连接生命周期
	config.MaxConnIdleTime = 15 * time.Minute // 最大空闲时间
}

// checkUserPoolLimit 检查用户连接池数量限制
func (cm *ConnectionManager) checkUserPoolLimit(userID int64) error {
	count := 0
	cm.connectionPools.Range(func(key, value any) bool {
		if managedPool, ok := value.(*ManagedPool); ok {
			if managedPool.Connection.UserID == userID {
				count++
			}
		}
		return true
	})
	
	if count >= cm.maxPoolsPerUser {
		return fmt.Errorf("用户连接池数量已达到限制(%d)", cm.maxPoolsPerUser)
	}
	
	return nil
}

// TestConnection 测试数据库连接
func (cm *ConnectionManager) TestConnection(ctx context.Context, connection *repository.DatabaseConnection) error {
	// 解密密码
	decryptedPassword, err := cm.encryption.Decrypt(connection.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("密码解密失败: %w", err)
	}
	
	return cm.testConnectionDirect(ctx, connection, decryptedPassword)
}

// testConnectionDirect 使用明文密码直接测试数据库连接
func (cm *ConnectionManager) testConnectionDirect(ctx context.Context, connection *repository.DatabaseConnection, plainPassword string) error {
	// 构建连接字符串
	connStr := cm.buildConnectionString(connection, plainPassword)
	
	// 创建临时连接池用于测试
	testCtx, cancel := context.WithTimeout(ctx, cm.connectionTimeout)
	defer cancel()
	
	pool, err := pgxpool.New(testCtx, connStr)
	if err != nil {
		return fmt.Errorf("创建测试连接失败: %w", err)
	}
	defer pool.Close()
	
	// 执行测试查询
	var result int
	err = pool.QueryRow(testCtx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("连接测试查询失败: %w", err)
	}
	
	if result != 1 {
		return fmt.Errorf("连接测试返回异常结果: %d", result)
	}
	
	return nil
}

// CreateConnection 创建新的数据库连接配置
func (cm *ConnectionManager) CreateConnection(ctx context.Context, connection *repository.DatabaseConnection) error {
	// 加密密码
	encryptedPassword, err := cm.encryption.Encrypt(connection.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}
	
	// 替换为加密后的密码
	originalPassword := connection.PasswordEncrypted
	connection.PasswordEncrypted = encryptedPassword
	
	// 先测试连接（使用原始明文密码，不通过TestConnection方法）
	connection.PasswordEncrypted = originalPassword
	if err := cm.testConnectionDirect(ctx, connection, originalPassword); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}
	
	// 恢复加密密码并保存
	connection.PasswordEncrypted = encryptedPassword
	if err := cm.connectionRepo.Create(ctx, connection); err != nil {
		return fmt.Errorf("保存连接配置失败: %w", err)
	}
	
	cm.logger.Info("创建数据库连接配置",
		zap.Int64("connection_id", connection.ID),
		zap.Int64("user_id", connection.UserID),
		zap.String("host", connection.Host),
		zap.String("database", connection.DatabaseName))
	
	return nil
}

// UpdateConnection 更新数据库连接配置
func (cm *ConnectionManager) UpdateConnection(ctx context.Context, connection *repository.DatabaseConnection) error {
	// 如果密码被更新，需要重新加密
	if connection.PasswordEncrypted != "" {
		encryptedPassword, err := cm.encryption.Encrypt(connection.PasswordEncrypted)
		if err != nil {
			return fmt.Errorf("密码加密失败: %w", err)
		}
		connection.PasswordEncrypted = encryptedPassword
	}
	
	// 更新配置
	if err := cm.connectionRepo.Update(ctx, connection); err != nil {
		return fmt.Errorf("更新连接配置失败: %w", err)
	}
	
	// 移除旧的连接池缓存
	cm.connectionPools.Delete(connection.ID)
	
	cm.logger.Info("更新数据库连接配置",
		zap.Int64("connection_id", connection.ID))
	
	return nil
}

// DeleteConnection 删除数据库连接配置
func (cm *ConnectionManager) DeleteConnection(ctx context.Context, connectionID int64) error {
	// 先关闭连接池
	if value, ok := cm.connectionPools.Load(connectionID); ok {
		if managedPool, ok := value.(*ManagedPool); ok {
			managedPool.Pool.Close()
		}
		cm.connectionPools.Delete(connectionID)
	}
	
	// 删除配置
	if err := cm.connectionRepo.Delete(ctx, connectionID); err != nil {
		return fmt.Errorf("删除连接配置失败: %w", err)
	}
	
	cm.logger.Info("删除数据库连接配置",
		zap.Int64("connection_id", connectionID))
	
	return nil
}

// healthCheckRoutine 健康检查例程
func (cm *ConnectionManager) healthCheckRoutine() {
	for {
		select {
		case <-cm.stopCh:
			return
		case <-cm.healthTicker.C:
			cm.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (cm *ConnectionManager) performHealthCheck() {
	cm.connectionPools.Range(func(key, value any) bool {
		if managedPool, ok := value.(*ManagedPool); ok {
			connectionID := key.(int64)
			cm.checkPoolHealth(connectionID, managedPool)
		}
		return true
	})
}

// checkPoolHealth 检查连接池健康状态
func (cm *ConnectionManager) checkPoolHealth(connectionID int64, managedPool *ManagedPool) {
	managedPool.mutex.Lock()
	defer managedPool.mutex.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), cm.connectionTimeout)
	defer cancel()
	
	// 执行健康检查
	var result int
	err := managedPool.Pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	
	managedPool.HealthStatus.LastCheck = time.Now()
	managedPool.HealthStatus.CheckCount++
	
	if err != nil || result != 1 {
		managedPool.HealthStatus.IsHealthy = false
		if err != nil {
			managedPool.HealthStatus.ErrorMessage = err.Error()
		} else {
			managedPool.HealthStatus.ErrorMessage = fmt.Sprintf("健康检查返回异常结果: %d", result)
		}
		
		cm.logger.Warn("数据库连接池健康检查失败",
			zap.Int64("connection_id", connectionID),
			zap.Error(err))
		
		// 更新连接状态为异常
		go cm.updateConnectionStatus(connectionID, repository.ConnectionError)
	} else {
		managedPool.HealthStatus.IsHealthy = true
		managedPool.HealthStatus.ErrorMessage = ""
	}
}

// cleanupRoutine 清理例程
func (cm *ConnectionManager) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute) // 每10分钟清理一次
	defer ticker.Stop()
	
	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			cm.cleanupIdlePools()
		}
	}
}

// cleanupIdlePools 清理空闲连接池
func (cm *ConnectionManager) cleanupIdlePools() {
	now := time.Now()
	
	cm.connectionPools.Range(func(key, value any) bool {
		if managedPool, ok := value.(*ManagedPool); ok {
			managedPool.mutex.RLock()
			isIdle := now.Sub(managedPool.LastUsed) > cm.poolIdleTimeout
			managedPool.mutex.RUnlock()
			
			if isIdle {
				connectionID := key.(int64)
				cm.logger.Info("清理空闲连接池",
					zap.Int64("connection_id", connectionID),
					zap.Duration("idle_time", now.Sub(managedPool.LastUsed)))
				
				managedPool.Pool.Close()
				cm.connectionPools.Delete(connectionID)
			}
		}
		return true
	})
}

// updateConnectionStatus 更新连接状态
func (cm *ConnectionManager) updateConnectionStatus(connectionID int64, status repository.ConnectionStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := cm.connectionRepo.UpdateStatus(ctx, connectionID, status); err != nil {
		cm.logger.Error("更新连接状态失败",
			zap.Int64("connection_id", connectionID),
			zap.String("status", string(status)),
			zap.Error(err))
	}
}

// GetPoolStats 获取连接池统计信息
func (cm *ConnectionManager) GetPoolStats() map[string]any {
	stats := map[string]any{
		"total_pools": 0,
		"healthy_pools": 0,
		"unhealthy_pools": 0,
		"pools": []map[string]any{},
	}
	
	totalPools := 0
	healthyPools := 0
	pools := []map[string]any{}
	
	cm.connectionPools.Range(func(key, value any) bool {
		if managedPool, ok := value.(*ManagedPool); ok {
			totalPools++
			
			managedPool.mutex.RLock()
			poolStat := managedPool.Pool.Stat()
			isHealthy := managedPool.HealthStatus.IsHealthy
			managedPool.mutex.RUnlock()
			
			if isHealthy {
				healthyPools++
			}
			
			poolInfo := map[string]any{
				"connection_id":     key,
				"total_conns":       poolStat.TotalConns(),
				"idle_conns":        poolStat.IdleConns(),
				"acquired_conns":    poolStat.AcquiredConns(),
				"is_healthy":        isHealthy,
				"last_used":         managedPool.LastUsed,
				"created_at":        managedPool.CreatedAt,
			}
			
			pools = append(pools, poolInfo)
		}
		return true
	})
	
	stats["total_pools"] = totalPools
	stats["healthy_pools"] = healthyPools
	stats["unhealthy_pools"] = totalPools - healthyPools
	stats["pools"] = pools
	
	return stats
}

// NewAESEncryption 创建AES加密服务
func NewAESEncryption(key []byte) (*AESEncryption, error) {
	// 验证密钥是否为空或nil
	if key == nil || len(key) == 0 {
		return nil, errors.New("加密密钥不能为nil或空")
	}
	
	// 只接受合理长度的密钥(16-48字节)，其他情况返回错误  
	if len(key) < 16 {
		return nil, errors.New("加密密钥长度不能小于16字节")
	}
	if len(key) > 48 {
		return nil, errors.New("加密密钥长度不能大于48字节")
	}
	
	// 确保密钥长度为256位(32字节)
	if len(key) != 32 {
		// 使用SHA-256哈希生成固定长度的密钥
		hash := sha256.Sum256(key)
		key = hash[:]
	}
	
	return &AESEncryption{key: key}, nil
}

// Encrypt 加密文本
func (e *AESEncryption) Encrypt(plaintext string) (string, error) {
	// 即使是空字符串也要加密，以确保一致的安全行为
	// 空字符串不应该直接返回空，而应该加密成有效的密文
	
	// 创建AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %w", err)
	}
	
	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}
	
	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}
	
	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// 返回base64编码的结果
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密文本
func (e *AESEncryption) Decrypt(ciphertext string) (string, error) {
	// 空密文应该返回错误，而不是直接返回空字符串
	if ciphertext == "" {
		return "", errors.New("密文不能为空")
	}
	
	// Base64解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64解码失败: %w", err)
	}
	
	// 创建AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %w", err)
	}
	
	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}
	
	// 检查数据长度
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("加密数据长度不足")
	}
	
	// 提取nonce和密文
	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	
	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	
	return string(plaintext), nil
}