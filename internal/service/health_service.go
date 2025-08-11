package service

import (
	"context"
	"fmt"
	"time"

	"chat2sql-go/internal/config"
	"chat2sql-go/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// HealthServiceInterface 健康检查服务接口，用于支持测试和依赖注入
type HealthServiceInterface interface {
	CheckHealth(ctx context.Context) *HealthCheckResult
	CheckReadiness(ctx context.Context) *ReadinessResult
	GetVersionInfo() map[string]interface{}
}

// HealthService 健康检查服务
type HealthService struct {
	repo        repository.Repository
	redisClient redis.UniversalClient
	appInfo     *config.AppInfo
	logger      *zap.Logger
}

// NewHealthService 创建健康检查服务
func NewHealthService(
	repo repository.Repository,
	redisClient redis.UniversalClient,
	appInfo *config.AppInfo,
	logger *zap.Logger,
) *HealthService {
	return &HealthService{
		repo:        repo,
		redisClient: redisClient,
		appInfo:     appInfo,
		logger:      logger,
	}
}

// HealthStatus 健康状态枚举
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
)

// ComponentStatus 组件状态
type ComponentStatus struct {
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Duration  string       `json:"duration,omitempty"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status      HealthStatus                `json:"status"`
	Timestamp   time.Time                   `json:"timestamp"`
	Service     string                      `json:"service"`
	Version     string                      `json:"version"`
	Environment string                      `json:"environment"`
	Components  map[string]ComponentStatus  `json:"components"`
	BuildInfo   map[string]interface{}      `json:"build_info,omitempty"`
}

// ReadinessResult 就绪检查结果
type ReadinessResult struct {
	Status     HealthStatus                `json:"status"`
	Timestamp  time.Time                   `json:"timestamp"`
	Components map[string]ComponentStatus  `json:"components"`
}

// CheckHealth 执行健康检查
func (h *HealthService) CheckHealth(ctx context.Context) *HealthCheckResult {
	now := time.Now()
	components := make(map[string]ComponentStatus)
	overallStatus := HealthStatusHealthy

	// 检查数据库连接
	dbStatus := h.checkDatabase(ctx)
	components["database"] = dbStatus
	if dbStatus.Status != HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// 检查Redis连接
	if h.redisClient != nil {
		redisStatus := h.checkRedis(ctx)
		components["redis"] = redisStatus
		if redisStatus.Status != HealthStatusHealthy {
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusDegraded
			}
		}
	}

	return &HealthCheckResult{
		Status:      overallStatus,
		Timestamp:   now,
		Service:     h.appInfo.Name,
		Version:     h.appInfo.Version,
		Environment: h.appInfo.Environment,
		Components:  components,
		BuildInfo:   h.appInfo.GetBuildInfo(),
	}
}

// CheckReadiness 执行就绪检查
func (h *HealthService) CheckReadiness(ctx context.Context) *ReadinessResult {
	now := time.Now()
	components := make(map[string]ComponentStatus)
	overallStatus := HealthStatusHealthy

	// 就绪检查比健康检查更严格
	// 数据库必须正常
	dbStatus := h.checkDatabase(ctx)
	components["database"] = dbStatus
	if dbStatus.Status != HealthStatusHealthy {
		overallStatus = HealthStatusUnhealthy
	}

	// Redis也必须正常（如果配置了）
	if h.redisClient != nil {
		redisStatus := h.checkRedis(ctx)
		components["redis"] = redisStatus
		if redisStatus.Status != HealthStatusHealthy {
			overallStatus = HealthStatusUnhealthy
		}
	}

	return &ReadinessResult{
		Status:     overallStatus,
		Timestamp:  now,
		Components: components,
	}
}

// checkDatabase 检查数据库连接
func (h *HealthService) checkDatabase(ctx context.Context) ComponentStatus {
	start := time.Now()
	
	if h.repo == nil {
		return ComponentStatus{
			Status:    HealthStatusUnhealthy,
			Message:   "数据库连接未配置",
			Timestamp: time.Now(),
		}
	}

	// 使用超时上下文防止长时间阻塞
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := h.repo.HealthCheck(timeoutCtx)
	duration := time.Since(start)

	if err != nil {
		h.logger.Error("数据库健康检查失败", zap.Error(err))
		return ComponentStatus{
			Status:    HealthStatusUnhealthy,
			Message:   fmt.Sprintf("数据库连接失败: %v", err),
			Timestamp: time.Now(),
			Duration:  duration.String(),
		}
	}

	status := HealthStatusHealthy
	message := "数据库连接正常"

	// 如果响应时间过长，标记为降级
	if duration > 2*time.Second {
		status = HealthStatusDegraded
		message = "数据库响应较慢"
	}

	return ComponentStatus{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Duration:  duration.String(),
	}
}

// checkRedis 检查Redis连接
func (h *HealthService) checkRedis(ctx context.Context) ComponentStatus {
	start := time.Now()

	// 使用超时上下文防止长时间阻塞
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := h.redisClient.Ping(timeoutCtx).Err()
	duration := time.Since(start)

	if err != nil {
		h.logger.Error("Redis健康检查失败", zap.Error(err))
		return ComponentStatus{
			Status:    HealthStatusUnhealthy,
			Message:   fmt.Sprintf("Redis连接失败: %v", err),
			Timestamp: time.Now(),
			Duration:  duration.String(),
		}
	}

	status := HealthStatusHealthy
	message := "Redis连接正常"

	// 如果响应时间过长，标记为降级
	if duration > 1*time.Second {
		status = HealthStatusDegraded
		message = "Redis响应较慢"
	}

	return ComponentStatus{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Duration:  duration.String(),
	}
}

// GetVersionInfo 获取版本信息
func (h *HealthService) GetVersionInfo() map[string]interface{} {
	return h.appInfo.GetBuildInfo()
}