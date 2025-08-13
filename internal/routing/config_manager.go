// 配置管理器 - P2阶段智能路由动态配置管理组件
// 支持热重载、配置验证、版本管理和实时配置更新
// 基于文件监控和远程配置中心的统一配置管理

package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	// 配置存储
	config    *RouterConfig
	configMu  sync.RWMutex
	
	// 配置变更监听器
	listeners map[string][]ConfigChangeListener
	listenersMu sync.RWMutex
	
	// 文件监控
	configFile     string
	lastModTime    time.Time
	watcherCtx     context.Context
	watcherCancel  context.CancelFunc
	watcherWg      sync.WaitGroup
	
	// 配置历史
	configHistory []*ConfigVersion
	historyMu     sync.RWMutex
	maxHistory    int
	
	// 热重载状态
	hotReloadEnabled bool
	reloadInterval   time.Duration
	
	// 配置验证器
	validators map[string]ConfigValidator
}

// RouterConfig 路由器完整配置
type RouterConfig struct {
	// 基础配置
	Version     string    `yaml:"version" json:"version"`
	UpdatedAt   time.Time `yaml:"updated_at" json:"updated_at"`
	
	// 模型配置列表
	Models      []*ModelConfig `yaml:"models" json:"models"`
	
	// 路由策略配置
	Routing     *RoutingConfig `yaml:"routing" json:"routing"`
	
	// 健康检查配置
	HealthCheck *HealthCheckConfig `yaml:"health_check" json:"health_check"`
	
	// 负载均衡配置
	LoadBalancer *LoadBalancerConfig `yaml:"load_balancer" json:"load_balancer"`
	
	// 缓存配置
	Cache       *CacheConfig `yaml:"cache" json:"cache"`
	
	// 监控配置
	Monitoring  *MonitoringConfig `yaml:"monitoring" json:"monitoring"`
}

// RoutingConfig 路由策略配置
type RoutingConfig struct {
	// 复杂度阈值
	ComplexityThresholds struct {
		Simple  float64 `yaml:"simple" json:"simple"`
		Medium  float64 `yaml:"medium" json:"medium"`
		Complex float64 `yaml:"complex" json:"complex"`
	} `yaml:"complexity_thresholds" json:"complexity_thresholds"`
	
	// 权重配置
	Weights struct {
		Cost        float64 `yaml:"cost" json:"cost"`
		Performance float64 `yaml:"performance" json:"performance"`
		Accuracy    float64 `yaml:"accuracy" json:"accuracy"`
	} `yaml:"weights" json:"weights"`
	
	// 降级策略
	Fallback struct {
		Enabled     bool   `yaml:"enabled" json:"enabled"`
		MaxRetries  int    `yaml:"max_retries" json:"max_retries"`
		DefaultModel string `yaml:"default_model" json:"default_model"`
	} `yaml:"fallback" json:"fallback"`
	
	// 路由算法
	Algorithm   string `yaml:"algorithm" json:"algorithm"`
	
	// 预热配置
	Prewarming struct {
		Enabled  bool     `yaml:"enabled" json:"enabled"`
		Models   []string `yaml:"models" json:"models"`
		Patterns []string `yaml:"patterns" json:"patterns"`
	} `yaml:"prewarming" json:"prewarming"`
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	// QPS限制
	GlobalQPSLimit    int `yaml:"global_qps_limit" json:"global_qps_limit"`
	PerModelQPSLimit  int `yaml:"per_model_qps_limit" json:"per_model_qps_limit"`
	
	// 熔断配置
	CircuitBreaker struct {
		FailureThreshold  int           `yaml:"failure_threshold" json:"failure_threshold"`
		RecoveryTimeout   time.Duration `yaml:"recovery_timeout" json:"recovery_timeout"`
		HalfOpenMaxCalls  int           `yaml:"half_open_max_calls" json:"half_open_max_calls"`
	} `yaml:"circuit_breaker" json:"circuit_breaker"`
	
	// 负载均衡策略
	Strategy              string        `yaml:"strategy" json:"strategy"`
	HealthCheckInterval   time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// Redis配置
	Redis struct {
		Addr     string `yaml:"addr" json:"addr"`
		Password string `yaml:"password" json:"password"`
		DB       int    `yaml:"db" json:"db"`
	} `yaml:"redis" json:"redis"`
	
	// 缓存策略
	Strategy struct {
		ExactMatchTTL    time.Duration `yaml:"exact_match_ttl" json:"exact_match_ttl"`
		SimilarQueryTTL  time.Duration `yaml:"similar_query_ttl" json:"similar_query_ttl"`
		PartialResultTTL time.Duration `yaml:"partial_result_ttl" json:"partial_result_ttl"`
	} `yaml:"strategy" json:"strategy"`
	
	// 缓存大小限制
	MaxSize     int     `yaml:"max_size" json:"max_size"`
	EvictionPolicy string `yaml:"eviction_policy" json:"eviction_policy"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	// Prometheus配置
	Prometheus struct {
		Enabled bool   `yaml:"enabled" json:"enabled"`
		Port    int    `yaml:"port" json:"port"`
		Path    string `yaml:"path" json:"path"`
	} `yaml:"prometheus" json:"prometheus"`
	
	// 指标配置
	Metrics struct {
		CollectionInterval time.Duration `yaml:"collection_interval" json:"collection_interval"`
		RetentionPeriod    time.Duration `yaml:"retention_period" json:"retention_period"`
	} `yaml:"metrics" json:"metrics"`
	
	// 告警配置
	Alerts struct {
		Enabled    bool              `yaml:"enabled" json:"enabled"`
		Webhook    string            `yaml:"webhook" json:"webhook"`
		Rules      map[string]string `yaml:"rules" json:"rules"`
	} `yaml:"alerts" json:"alerts"`
}

// ConfigVersion 配置版本信息
type ConfigVersion struct {
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Changes   []string          `json:"changes"`
	Config    *RouterConfig     `json:"config"`
	Metadata  map[string]string `json:"metadata"`
}

// ConfigChangeListener 配置变更监听器
type ConfigChangeListener interface {
	OnConfigChange(oldConfig, newConfig *RouterConfig, changes []string) error
}

// ConfigChangeListenerFunc 函数式监听器
type ConfigChangeListenerFunc func(oldConfig, newConfig *RouterConfig, changes []string) error

// OnConfigChange 实现监听器接口
func (f ConfigChangeListenerFunc) OnConfigChange(oldConfig, newConfig *RouterConfig, changes []string) error {
	return f(oldConfig, newConfig, changes)
}

// ConfigValidator 配置验证器
type ConfigValidator interface {
	ValidateConfig(config *RouterConfig) error
}

// ConfigValidatorFunc 函数式验证器
type ConfigValidatorFunc func(config *RouterConfig) error

// ValidateConfig 实现验证器接口
func (f ConfigValidatorFunc) ValidateConfig(config *RouterConfig) error {
	return f(config)
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		listeners:        make(map[string][]ConfigChangeListener),
		configHistory:    make([]*ConfigVersion, 0),
		maxHistory:       10, // 保留10个历史版本
		hotReloadEnabled: true,
		reloadInterval:   10 * time.Second,
		validators:       make(map[string]ConfigValidator),
	}
}

// LoadConfigFromFile 从文件加载配置
func (cm *ConfigManager) LoadConfigFromFile(filePath string) error {
	cm.configMu.Lock()
	defer cm.configMu.Unlock()
	
	// 读取文件
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	
	// 获取文件修改时间
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}
	
	// 解析配置
	var config RouterConfig
	ext := filepath.Ext(filePath)
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &config)
	case ".json":
		err = json.Unmarshal(data, &config)
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", ext)
	}
	
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	
	// 验证配置
	if err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}
	
	// 设置配置文件路径和修改时间
	cm.configFile = filePath
	cm.lastModTime = fileInfo.ModTime()
	
	// 更新配置
	return cm.updateConfig(&config, "从文件加载配置")
}

// EnableHotReload 启用热重载
func (cm *ConfigManager) EnableHotReload(ctx context.Context) error {
	if cm.configFile == "" {
		return fmt.Errorf("配置文件路径未设置")
	}
	
	cm.watcherCtx, cm.watcherCancel = context.WithCancel(ctx)
	
	cm.watcherWg.Add(1)
	go cm.watchConfigFile()
	
	return nil
}

// watchConfigFile 监控配置文件变化
func (cm *ConfigManager) watchConfigFile() {
	defer cm.watcherWg.Done()
	
	ticker := time.NewTicker(cm.reloadInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := cm.checkAndReloadConfig(); err != nil {
				// 记录错误但继续监控
				fmt.Printf("配置热重载失败: %v\n", err)
			}
		case <-cm.watcherCtx.Done():
			return
		}
	}
}

// checkAndReloadConfig 检查并重载配置
func (cm *ConfigManager) checkAndReloadConfig() error {
	fileInfo, err := os.Stat(cm.configFile)
	if err != nil {
		return fmt.Errorf("获取配置文件信息失败: %w", err)
	}
	
	// 检查文件是否有变化
	if !fileInfo.ModTime().After(cm.lastModTime) {
		return nil // 文件未变化
	}
	
	// 重新加载配置
	return cm.LoadConfigFromFile(cm.configFile)
}

// updateConfig 更新配置
func (cm *ConfigManager) updateConfig(newConfig *RouterConfig, changeReason string) error {
	oldConfig := cm.config
	
	// 记录配置历史
	cm.saveConfigVersion(oldConfig, newConfig, changeReason)
	
	// 设置更新时间
	newConfig.UpdatedAt = time.Now()
	if newConfig.Version == "" {
		newConfig.Version = fmt.Sprintf("v%d", time.Now().Unix())
	}
	
	// 更新配置
	cm.config = newConfig
	
	// 检测变化
	changes := cm.detectConfigChanges(oldConfig, newConfig)
	
	// 通知监听器
	if err := cm.notifyListeners(oldConfig, newConfig, changes); err != nil {
		return fmt.Errorf("通知配置变更监听器失败: %w", err)
	}
	
	return nil
}

// detectConfigChanges 检测配置变化
func (cm *ConfigManager) detectConfigChanges(oldConfig, newConfig *RouterConfig) []string {
	var changes []string
	
	if oldConfig == nil {
		changes = append(changes, "初始化配置")
		return changes
	}
	
	// 检测模型配置变化
	if len(oldConfig.Models) != len(newConfig.Models) {
		changes = append(changes, "模型数量变化")
	}
	
	// 检测路由配置变化
	if oldConfig.Routing != nil && newConfig.Routing != nil {
		if oldConfig.Routing.Algorithm != newConfig.Routing.Algorithm {
			changes = append(changes, "路由算法变化")
		}
		
		if oldConfig.Routing.Weights != newConfig.Routing.Weights {
			changes = append(changes, "路由权重变化")
		}
	}
	
	// 检测健康检查配置变化
	if oldConfig.HealthCheck != nil && newConfig.HealthCheck != nil {
		if oldConfig.HealthCheck.Interval != newConfig.HealthCheck.Interval {
			changes = append(changes, "健康检查间隔变化")
		}
	}
	
	// 检测负载均衡配置变化
	if oldConfig.LoadBalancer != nil && newConfig.LoadBalancer != nil {
		if oldConfig.LoadBalancer.Strategy != newConfig.LoadBalancer.Strategy {
			changes = append(changes, "负载均衡策略变化")
		}
	}
	
	// 检测缓存配置变化
	if oldConfig.Cache != nil && newConfig.Cache != nil {
		if oldConfig.Cache.MaxSize != newConfig.Cache.MaxSize {
			changes = append(changes, "缓存大小变化")
		}
	}
	
	// 检测监控配置变化
	if oldConfig.Monitoring != nil && newConfig.Monitoring != nil {
		if oldConfig.Monitoring.Prometheus.Enabled != newConfig.Monitoring.Prometheus.Enabled {
			changes = append(changes, "Prometheus监控状态变化")
		}
	}
	
	if len(changes) == 0 {
		changes = append(changes, "配置更新")
	}
	
	return changes
}

// saveConfigVersion 保存配置版本
func (cm *ConfigManager) saveConfigVersion(oldConfig, newConfig *RouterConfig, changeReason string) {
	cm.historyMu.Lock()
	defer cm.historyMu.Unlock()
	
	if newConfig == nil {
		return
	}
	
	version := &ConfigVersion{
		Version:   newConfig.Version,
		Timestamp: time.Now(),
		Changes:   []string{changeReason},
		Config:    newConfig,
		Metadata:  make(map[string]string),
	}
	
	// 添加版本到历史
	cm.configHistory = append(cm.configHistory, version)
	
	// 保持历史版本数量限制
	if len(cm.configHistory) > cm.maxHistory {
		cm.configHistory = cm.configHistory[1:]
	}
}

// notifyListeners 通知配置变更监听器
func (cm *ConfigManager) notifyListeners(oldConfig, newConfig *RouterConfig, changes []string) error {
	cm.listenersMu.RLock()
	defer cm.listenersMu.RUnlock()
	
	for listenerType, listeners := range cm.listeners {
		for i, listener := range listeners {
			if err := listener.OnConfigChange(oldConfig, newConfig, changes); err != nil {
				return fmt.Errorf("监听器 %s[%d] 处理配置变更失败: %w", listenerType, i, err)
			}
		}
	}
	
	return nil
}

// validateConfig 验证配置
func (cm *ConfigManager) validateConfig(config *RouterConfig) error {
	// 基础验证
	if config.Models == nil || len(config.Models) == 0 {
		return fmt.Errorf("模型配置不能为空")
	}
	
	// 验证模型配置
	modelNames := make(map[string]bool)
	for i, model := range config.Models {
		if model.Name == "" {
			return fmt.Errorf("第%d个模型名称不能为空", i+1)
		}
		
		if modelNames[model.Name] {
			return fmt.Errorf("模型名称重复: %s", model.Name)
		}
		modelNames[model.Name] = true
		
		if model.Provider == "" {
			return fmt.Errorf("模型 %s 的提供商不能为空", model.Name)
		}
		
		if model.Category == "" {
			return fmt.Errorf("模型 %s 的类别不能为空", model.Name)
		}
	}
	
	// 执行自定义验证器
	for name, validator := range cm.validators {
		if err := validator.ValidateConfig(config); err != nil {
			return fmt.Errorf("验证器 %s 失败: %w", name, err)
		}
	}
	
	return nil
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *RouterConfig {
	cm.configMu.RLock()
	defer cm.configMu.RUnlock()
	
	return cm.config
}

// UpdateConfigPartial 部分更新配置
func (cm *ConfigManager) UpdateConfigPartial(updates map[string]interface{}) error {
	cm.configMu.Lock()
	defer cm.configMu.Unlock()
	
	if cm.config == nil {
		return fmt.Errorf("配置尚未初始化")
	}
	
	// 创建配置副本
	newConfig := *cm.config
	
	// 应用部分更新
	for key, value := range updates {
		if err := cm.applyPartialUpdate(&newConfig, key, value); err != nil {
			return fmt.Errorf("应用配置更新失败 %s: %w", key, err)
		}
	}
	
	// 验证更新后的配置
	if err := cm.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}
	
	return cm.updateConfig(&newConfig, "部分配置更新")
}

// applyPartialUpdate 应用部分配置更新
func (cm *ConfigManager) applyPartialUpdate(config *RouterConfig, key string, value interface{}) error {
	switch key {
	case "routing.algorithm":
		if config.Routing == nil {
			config.Routing = &RoutingConfig{}
		}
		if str, ok := value.(string); ok {
			config.Routing.Algorithm = str
		} else {
			return fmt.Errorf("routing.algorithm 必须是字符串类型")
		}
		
	case "health_check.interval":
		if config.HealthCheck == nil {
			config.HealthCheck = DefaultHealthCheckConfig()
		}
		if duration, ok := value.(time.Duration); ok {
			config.HealthCheck.Interval = duration
		} else if str, ok := value.(string); ok {
			duration, err := time.ParseDuration(str)
			if err != nil {
				return fmt.Errorf("无法解析duration: %w", err)
			}
			config.HealthCheck.Interval = duration
		} else {
			return fmt.Errorf("health_check.interval 必须是duration类型")
		}
		
	// 可以根据需要添加更多配置项的更新逻辑
	default:
		return fmt.Errorf("不支持的配置项: %s", key)
	}
	
	return nil
}

// RegisterListener 注册配置变更监听器
func (cm *ConfigManager) RegisterListener(listenerType string, listener ConfigChangeListener) {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()
	
	cm.listeners[listenerType] = append(cm.listeners[listenerType], listener)
}

// RegisterValidator 注册配置验证器
func (cm *ConfigManager) RegisterValidator(name string, validator ConfigValidator) {
	cm.validators[name] = validator
}

// GetConfigHistory 获取配置历史
func (cm *ConfigManager) GetConfigHistory() []*ConfigVersion {
	cm.historyMu.RLock()
	defer cm.historyMu.RUnlock()
	
	// 返回副本
	history := make([]*ConfigVersion, len(cm.configHistory))
	copy(history, cm.configHistory)
	
	return history
}

// RollbackToVersion 回滚到指定版本
func (cm *ConfigManager) RollbackToVersion(version string) error {
	cm.historyMu.RLock()
	var targetConfig *RouterConfig
	for _, v := range cm.configHistory {
		if v.Version == version {
			targetConfig = v.Config
			break
		}
	}
	cm.historyMu.RUnlock()
	
	if targetConfig == nil {
		return fmt.Errorf("版本 %s 不存在", version)
	}
	
	return cm.updateConfig(targetConfig, fmt.Sprintf("回滚到版本 %s", version))
}

// Close 关闭配置管理器
func (cm *ConfigManager) Close() error {
	if cm.watcherCancel != nil {
		cm.watcherCancel()
		cm.watcherWg.Wait()
	}
	
	return nil
}