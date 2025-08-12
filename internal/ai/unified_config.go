// AI模块统一配置管理 - 整合所有AI相关配置到单一文件
// 避免配置重复，提供统一的配置入口

package ai

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// AIModuleConfig AI模块统一配置 - 作为顶层配置容器
type AIModuleConfig struct {
	// LLM配置
	LLMConfig *LLMRouterConfig `json:"llm_config" yaml:"llm_config"`
	
	// 性能配置
	PerformanceConfig *PerformanceConfig `json:"performance_config" yaml:"performance_config"`
	
	// 成本控制配置
	CostConfig *ExtendedCostConfig `json:"cost_config" yaml:"cost_config"`
	
	// 准确率监控配置
	AccuracyConfig *AccuracyConfig `json:"accuracy_config" yaml:"accuracy_config"`
	
	// 意图分析配置
	IntentConfig *IntentConfig `json:"intent_config" yaml:"intent_config"`
	
	// 全局设置
	GlobalConfig *GlobalConfig `json:"global_config" yaml:"global_config"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	// 环境标识
	Environment string `json:"environment" yaml:"environment"` // dev, test, prod
	
	// 调试模式
	DebugMode bool `json:"debug_mode" yaml:"debug_mode"`
	
	// 日志级别
	LogLevel string `json:"log_level" yaml:"log_level"` // debug, info, warn, error
	
	// 服务标识
	ServiceName string `json:"service_name" yaml:"service_name"`
	ServiceVersion string `json:"service_version" yaml:"service_version"`
	
	// 超时设置
	DefaultTimeout time.Duration `json:"default_timeout" yaml:"default_timeout"`
}

// LoadAIModuleConfig 加载完整的AI模块配置
func LoadAIModuleConfig() (*AIModuleConfig, error) {
	config := &AIModuleConfig{}
	
	// 加载全局配置
	config.GlobalConfig = loadGlobalConfig()
	
	// 加载LLM配置
	llmConfig, err := LoadLLMConfig()
	if err != nil {
		return nil, fmt.Errorf("加载LLM配置失败: %w", err)
	}
	config.LLMConfig = llmConfig
	
	// 加载性能配置
	config.PerformanceConfig = LoadPerformanceConfig()
	
	// 加载成本配置
	config.CostConfig = loadExtendedCostConfig()
	
	// 加载准确率监控配置
	config.AccuracyConfig = DefaultAccuracyConfig()
	
	// 加载意图分析配置
	config.IntentConfig = loadIntentConfig()
	
	return config, nil
}

// LoadPerformanceConfig 统一的性能配置加载函数（替代之前的重复定义）
func LoadPerformanceConfig() *PerformanceConfig {
	config := DefaultPerformanceConfig()
	
	// 从环境变量覆盖配置
	if workers := os.Getenv("AI_WORKERS"); workers != "" {
		if w, err := strconv.Atoi(workers); err == nil {
			config.Workers = w
		}
	}
	
	if queueSize := os.Getenv("AI_QUEUE_SIZE"); queueSize != "" {
		if qs, err := strconv.Atoi(queueSize); err == nil {
			config.QueueSize = qs
		}
	}
	
	if rateLimit := os.Getenv("AI_RATE_LIMIT"); rateLimit != "" {
		if rl, err := strconv.Atoi(rateLimit); err == nil {
			config.RateLimit = rl
		}
	}
	
	// 缓存配置
	if enableCache := os.Getenv("ENABLE_AI_CACHE"); enableCache != "" {
		config.EnableCache = strings.ToLower(enableCache) == "true"
	}
	
	if cacheTTL := os.Getenv("AI_CACHE_TTL_MINUTES"); cacheTTL != "" {
		if ttl, err := strconv.Atoi(cacheTTL); err == nil {
			config.CacheTTL = time.Duration(ttl) * time.Minute
		}
	}
	
	if cacheSize := os.Getenv("AI_CACHE_SIZE"); cacheSize != "" {
		if cs, err := strconv.Atoi(cacheSize); err == nil {
			config.CacheSize = cs
		}
	}
	
	// HTTP连接池配置
	if maxIdleConns := os.Getenv("AI_MAX_IDLE_CONNS"); maxIdleConns != "" {
		if mic, err := strconv.Atoi(maxIdleConns); err == nil {
			config.MaxIdleConns = mic
		}
	}
	
	if maxIdleConnsPerHost := os.Getenv("AI_MAX_IDLE_CONNS_PER_HOST"); maxIdleConnsPerHost != "" {
		if micph, err := strconv.Atoi(maxIdleConnsPerHost); err == nil {
			config.MaxIdleConnsPerHost = micph
		}
	}
	
	if idleConnTimeout := os.Getenv("AI_IDLE_CONN_TIMEOUT_SECONDS"); idleConnTimeout != "" {
		if ict, err := strconv.Atoi(idleConnTimeout); err == nil {
			config.IdleConnTimeout = time.Duration(ict) * time.Second
		}
	}
	
	return config
}

// loadGlobalConfig 加载全局配置
func loadGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Environment:    getEnvWithDefault("AI_ENVIRONMENT", "development"),
		DebugMode:      strings.ToLower(getEnvWithDefault("AI_DEBUG_MODE", "false")) == "true",
		LogLevel:       getEnvWithDefault("AI_LOG_LEVEL", "info"),
		ServiceName:    getEnvWithDefault("AI_SERVICE_NAME", "chat2sql-ai"),
		ServiceVersion: getEnvWithDefault("AI_SERVICE_VERSION", "1.0.0"),
		DefaultTimeout: time.Duration(getIntEnvWithDefault("AI_DEFAULT_TIMEOUT_SECONDS", 30)) * time.Second,
	}
}

// loadIntentConfig 加载意图分析配置
func loadIntentConfig() *IntentConfig {
	return &IntentConfig{
		EnableCache:            strings.ToLower(getEnvWithDefault("AI_INTENT_CACHE_ENABLED", "true")) == "true",
		CacheSize:              getIntEnvWithDefault("AI_INTENT_CACHE_SIZE", 1000),
		CacheTTL:               time.Duration(getIntEnvWithDefault("AI_INTENT_CACHE_TTL_MINUTES", 60)) * time.Minute,
		MinConfidence:          getFloatEnvWithDefault("AI_INTENT_MIN_CONFIDENCE", 0.6),
		MaxAlternatives:        getIntEnvWithDefault("AI_INTENT_MAX_ALTERNATIVES", 3),
		EnableUserLearning:     strings.ToLower(getEnvWithDefault("AI_INTENT_USER_LEARNING", "true")) == "true",
		UserProfileSize:        getIntEnvWithDefault("AI_INTENT_USER_PROFILE_SIZE", 100),
		EnableEntityExtraction: strings.ToLower(getEnvWithDefault("AI_INTENT_ENTITY_EXTRACTION", "true")) == "true",
		CustomEntities:         strings.Split(getEnvWithDefault("AI_INTENT_CUSTOM_ENTITIES", ""), ","),
	}
}

// ValidateConfig 验证配置的完整性和有效性
func (config *AIModuleConfig) ValidateConfig() error {
	if config.LLMConfig == nil {
		return fmt.Errorf("LLM配置不能为空")
	}
	
	if config.PerformanceConfig == nil {
		return fmt.Errorf("性能配置不能为空")
	}
	
	// 验证LLM配置
	if config.LLMConfig.PrimaryProvider == "" {
		return fmt.Errorf("主要LLM提供商不能为空")
	}
	
	if config.LLMConfig.PrimaryConfig == nil {
		return fmt.Errorf("主要LLM配置不能为空")
	}
	
	// 验证性能配置合理性
	if config.PerformanceConfig.Workers < 0 {
		return fmt.Errorf("工作线程数不能为负数")
	}
	
	if config.PerformanceConfig.QueueSize <= 0 {
		return fmt.Errorf("队列大小必须大于0")
	}
	
	if config.PerformanceConfig.RateLimit <= 0 {
		return fmt.Errorf("限流配置必须大于0")
	}
	
	return nil
}

// GetEnvironmentInfo 获取环境信息摘要
func (config *AIModuleConfig) GetEnvironmentInfo() map[string]any {
	info := make(map[string]any)
	
	if config.GlobalConfig != nil {
		info["environment"] = config.GlobalConfig.Environment
		info["service_name"] = config.GlobalConfig.ServiceName
		info["service_version"] = config.GlobalConfig.ServiceVersion
		info["debug_mode"] = config.GlobalConfig.DebugMode
		info["log_level"] = config.GlobalConfig.LogLevel
	}
	
	if config.LLMConfig != nil {
		info["primary_llm"] = string(config.LLMConfig.PrimaryProvider)
		info["fallback_llm"] = string(config.LLMConfig.FallbackProvider)
		
		if config.LLMConfig.LocalProvider != "" {
			info["local_llm"] = string(config.LLMConfig.LocalProvider)
		}
	}
	
	if config.PerformanceConfig != nil {
		info["workers"] = config.PerformanceConfig.Workers
		info["queue_size"] = config.PerformanceConfig.QueueSize
		info["rate_limit"] = config.PerformanceConfig.RateLimit
		info["cache_enabled"] = config.PerformanceConfig.EnableCache
	}
	
	return info
}

// IsProduction 判断是否为生产环境
func (config *AIModuleConfig) IsProduction() bool {
	if config.GlobalConfig == nil {
		return false
	}
	return strings.ToLower(config.GlobalConfig.Environment) == "production" || 
		   strings.ToLower(config.GlobalConfig.Environment) == "prod"
}

// IsDevelopment 判断是否为开发环境  
func (config *AIModuleConfig) IsDevelopment() bool {
	if config.GlobalConfig == nil {
		return true
	}
	env := strings.ToLower(config.GlobalConfig.Environment)
	return env == "development" || env == "dev" || env == ""
}

// GetHTTPClient 获取优化的HTTP客户端（统一入口）
func (config *AIModuleConfig) GetHTTPClient() *HTTPClient {
	if config.PerformanceConfig == nil {
		config.PerformanceConfig = DefaultPerformanceConfig()
	}
	return NewHTTPClient(config.PerformanceConfig)
}

// 配置导出功能
func (config *AIModuleConfig) ExportConfig() map[string]any {
	result := make(map[string]any)
	
	result["global"] = config.GlobalConfig
	result["performance"] = config.PerformanceConfig
	result["cost"] = config.CostConfig
	result["accuracy"] = config.AccuracyConfig
	result["intent"] = config.IntentConfig
	
	// 不导出敏感的LLM配置（API密钥等）
	if config.LLMConfig != nil {
		llmSafe := map[string]any{
			"primary_provider":  config.LLMConfig.PrimaryProvider,
			"fallback_provider": config.LLMConfig.FallbackProvider,
			"local_provider":    config.LLMConfig.LocalProvider,
			"request_timeout":   config.LLMConfig.RequestTimeout,
		}
		result["llm"] = llmSafe
	}
	
	return result
}