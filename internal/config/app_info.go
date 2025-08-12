package config

import (
	"runtime"
	"time"
)

// AppInfo 应用信息配置
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	BuildTime   string `json:"build_time"`
	GitCommit   string `json:"git_commit"`
	GoVersion   string `json:"go_version"`
	Environment string `json:"environment"`
}

// DefaultAppInfo 返回默认的应用信息
func DefaultAppInfo() *AppInfo {
	return &AppInfo{
		Name:        "chat2sql-api",
		Version:     "0.1.0",
		BuildTime:   time.Now().UTC().Format(time.RFC3339),
		GitCommit:   "unknown",
		GoVersion:   runtime.Version(),
		Environment: "development",
	}
}

// NewAppInfo 创建新的应用信息
func NewAppInfo(name, version, buildTime, gitCommit, environment string) *AppInfo {
	if buildTime == "" {
		buildTime = time.Now().UTC().Format(time.RFC3339)
	}
	
	return &AppInfo{
		Name:        name,
		Version:     version,
		BuildTime:   buildTime,
		GitCommit:   gitCommit,
		GoVersion:   runtime.Version(),
		Environment: environment,
	}
}

// GetVersion 获取版本信息
func (a *AppInfo) GetVersion() string {
	return a.Version
}

// GetBuildInfo 获取构建信息
func (a *AppInfo) GetBuildInfo() map[string]any {
	return map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"build_time":  a.BuildTime,
		"git_commit":  a.GitCommit,
		"go_version":  a.GoVersion,
		"environment": a.Environment,
	}
}