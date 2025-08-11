package config

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultAppInfo 测试默认应用信息创建
func TestDefaultAppInfo(t *testing.T) {
	appInfo := DefaultAppInfo()

	assert.NotNil(t, appInfo)
	assert.Equal(t, "chat2sql-api", appInfo.Name)
	assert.Equal(t, "0.1.0", appInfo.Version)
	assert.Equal(t, runtime.Version(), appInfo.GoVersion)
	assert.Equal(t, "development", appInfo.Environment)
	assert.Equal(t, "unknown", appInfo.GitCommit)
	assert.NotEmpty(t, appInfo.BuildTime)

	// 验证BuildTime是有效的RFC3339格式
	_, err := time.Parse(time.RFC3339, appInfo.BuildTime)
	assert.NoError(t, err, "BuildTime should be in RFC3339 format")

	// 验证BuildTime是最近的时间（在测试开始前几秒内）
	buildTime, _ := time.Parse(time.RFC3339, appInfo.BuildTime)
	now := time.Now().UTC()
	timeDiff := now.Sub(buildTime)
	assert.True(t, timeDiff >= 0 && timeDiff < 10*time.Second, "BuildTime should be recent")
}

// TestNewAppInfo_AllFields 测试创建应用信息（所有字段都提供）
func TestNewAppInfo_AllFields(t *testing.T) {
	name := "test-app"
	version := "1.2.3"
	buildTime := "2024-01-08T12:00:00Z"
	gitCommit := "abc123def456"
	environment := "production"

	appInfo := NewAppInfo(name, version, buildTime, gitCommit, environment)

	assert.NotNil(t, appInfo)
	assert.Equal(t, name, appInfo.Name)
	assert.Equal(t, version, appInfo.Version)
	assert.Equal(t, buildTime, appInfo.BuildTime)
	assert.Equal(t, gitCommit, appInfo.GitCommit)
	assert.Equal(t, runtime.Version(), appInfo.GoVersion)
	assert.Equal(t, environment, appInfo.Environment)
}

// TestNewAppInfo_EmptyBuildTime 测试创建应用信息（空BuildTime）
func TestNewAppInfo_EmptyBuildTime(t *testing.T) {
	name := "test-app"
	version := "1.0.0"
	buildTime := ""
	gitCommit := "xyz789"
	environment := "staging"

	appInfo := NewAppInfo(name, version, buildTime, gitCommit, environment)

	assert.NotNil(t, appInfo)
	assert.Equal(t, name, appInfo.Name)
	assert.Equal(t, version, appInfo.Version)
	assert.NotEmpty(t, appInfo.BuildTime)
	assert.Equal(t, gitCommit, appInfo.GitCommit)
	assert.Equal(t, environment, appInfo.Environment)

	// 验证自动生成的BuildTime是有效的RFC3339格式
	_, err := time.Parse(time.RFC3339, appInfo.BuildTime)
	assert.NoError(t, err, "Auto-generated BuildTime should be in RFC3339 format")

	// 验证自动生成的BuildTime是最近的时间
	generatedBuildTime, _ := time.Parse(time.RFC3339, appInfo.BuildTime)
	now := time.Now().UTC()
	timeDiff := now.Sub(generatedBuildTime)
	assert.True(t, timeDiff >= 0 && timeDiff < 10*time.Second, "Auto-generated BuildTime should be recent")
}

// TestNewAppInfo_EmptyFields 测试创建应用信息（空字段处理）
func TestNewAppInfo_EmptyFields(t *testing.T) {
	appInfo := NewAppInfo("", "", "", "", "")

	assert.NotNil(t, appInfo)
	assert.Equal(t, "", appInfo.Name)
	assert.Equal(t, "", appInfo.Version)
	assert.NotEmpty(t, appInfo.BuildTime) // BuildTime应该被自动生成
	assert.Equal(t, "", appInfo.GitCommit)
	assert.Equal(t, runtime.Version(), appInfo.GoVersion) // GoVersion总是自动设置
	assert.Equal(t, "", appInfo.Environment)
}

// TestAppInfo_GetVersion 测试获取版本信息
func TestAppInfo_GetVersion(t *testing.T) {
	testCases := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "Normal version",
			version:  "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "Version with prefix",
			version:  "v2.0.0",
			expected: "v2.0.0",
		},
		{
			name:     "Beta version",
			version:  "1.0.0-beta.1",
			expected: "1.0.0-beta.1",
		},
		{
			name:     "Empty version",
			version:  "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appInfo := &AppInfo{Version: tc.version}
			assert.Equal(t, tc.expected, appInfo.GetVersion())
		})
	}
}

// TestAppInfo_GetBuildInfo 测试获取构建信息
func TestAppInfo_GetBuildInfo(t *testing.T) {
	appInfo := &AppInfo{
		Name:        "test-service",
		Version:     "2.1.0",
		BuildTime:   "2024-01-08T15:30:00Z",
		GitCommit:   "abcd1234",
		GoVersion:   "go1.21.0",
		Environment: "production",
	}

	buildInfo := appInfo.GetBuildInfo()

	assert.NotNil(t, buildInfo)
	assert.Len(t, buildInfo, 6) // 应该包含6个字段

	assert.Equal(t, "test-service", buildInfo["name"])
	assert.Equal(t, "2.1.0", buildInfo["version"])
	assert.Equal(t, "2024-01-08T15:30:00Z", buildInfo["build_time"])
	assert.Equal(t, "abcd1234", buildInfo["git_commit"])
	assert.Equal(t, "go1.21.0", buildInfo["go_version"])
	assert.Equal(t, "production", buildInfo["environment"])
}

// TestAppInfo_GetBuildInfo_EmptyFields 测试获取构建信息（空字段）
func TestAppInfo_GetBuildInfo_EmptyFields(t *testing.T) {
	appInfo := &AppInfo{} // 所有字段都是零值

	buildInfo := appInfo.GetBuildInfo()

	assert.NotNil(t, buildInfo)
	assert.Len(t, buildInfo, 6)

	assert.Equal(t, "", buildInfo["name"])
	assert.Equal(t, "", buildInfo["version"])
	assert.Equal(t, "", buildInfo["build_time"])
	assert.Equal(t, "", buildInfo["git_commit"])
	assert.Equal(t, "", buildInfo["go_version"])
	assert.Equal(t, "", buildInfo["environment"])
}

// TestAppInfo_GetBuildInfo_MapModification 测试构建信息Map的修改不影响原始数据
func TestAppInfo_GetBuildInfo_MapModification(t *testing.T) {
	appInfo := &AppInfo{
		Name:    "original-name",
		Version: "1.0.0",
	}

	buildInfo := appInfo.GetBuildInfo()
	
	// 修改返回的map
	buildInfo["name"] = "modified-name"
	buildInfo["new_field"] = "new_value"

	// 验证原始AppInfo没有被修改
	assert.Equal(t, "original-name", appInfo.Name)
	assert.Equal(t, "1.0.0", appInfo.Version)

	// 再次获取构建信息应该返回原始值
	newBuildInfo := appInfo.GetBuildInfo()
	assert.Equal(t, "original-name", newBuildInfo["name"])
	assert.NotContains(t, newBuildInfo, "new_field")
}

// TestAppInfo_GoVersionFormat 测试Go版本格式
func TestAppInfo_GoVersionFormat(t *testing.T) {
	appInfo := DefaultAppInfo()
	
	// Go版本应该以"go"开头
	assert.True(t, strings.HasPrefix(appInfo.GoVersion, "go"), "GoVersion should start with 'go'")
	
	// 验证版本号格式（至少包含主版本号.次版本号）
	versionPart := strings.TrimPrefix(appInfo.GoVersion, "go")
	versionParts := strings.Split(versionPart, ".")
	assert.True(t, len(versionParts) >= 2, "GoVersion should have at least major.minor version")
}

// TestAppInfo_DefaultEnvironments 测试默认环境配置
func TestAppInfo_DefaultEnvironments(t *testing.T) {
	defaultApp := DefaultAppInfo()
	assert.Equal(t, "development", defaultApp.Environment, "Default environment should be development")

	// 测试不同环境
	environments := []string{"development", "staging", "production", "test"}
	for _, env := range environments {
		appInfo := NewAppInfo("test-app", "1.0.0", "", "", env)
		assert.Equal(t, env, appInfo.Environment)
	}
}

// TestAppInfo_JSONMarshaling 测试JSON序列化（隐式测试，通过struct tags）
func TestAppInfo_JSONMarshaling(t *testing.T) {
	appInfo := &AppInfo{
		Name:        "json-test",
		Version:     "1.0.0",
		BuildTime:   "2024-01-08T12:00:00Z",
		GitCommit:   "commit123",
		GoVersion:   "go1.21.0",
		Environment: "test",
	}

	buildInfo := appInfo.GetBuildInfo()
	
	// 验证所有字段都能正确映射到JSON字段名
	expectedFields := []string{"name", "version", "build_time", "git_commit", "go_version", "environment"}
	for _, field := range expectedFields {
		assert.Contains(t, buildInfo, field, "BuildInfo should contain field: %s", field)
	}
}

// TestAppInfo_BuildTimeRFC3339Validation 测试BuildTime的RFC3339格式验证
func TestAppInfo_BuildTimeRFC3339Validation(t *testing.T) {
	testCases := []struct {
		name      string
		buildTime string
		isValid   bool
	}{
		{
			name:      "Valid RFC3339",
			buildTime: "2024-01-08T12:00:00Z",
			isValid:   true,
		},
		{
			name:      "Valid RFC3339 with timezone",
			buildTime: "2024-01-08T12:00:00+08:00",
			isValid:   true,
		},
		{
			name:      "Valid RFC3339 with microseconds",
			buildTime: "2024-01-08T12:00:00.123456Z",
			isValid:   true,
		},
		{
			name:      "Invalid format",
			buildTime: "2024-01-08 12:00:00",
			isValid:   false,
		},
		{
			name:      "Empty string",
			buildTime: "",
			isValid:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.buildTime == "" {
				// 跳过空字符串的解析测试
				return
			}
			
			_, err := time.Parse(time.RFC3339, tc.buildTime)
			if tc.isValid {
				assert.NoError(t, err, "BuildTime should be valid RFC3339")
			} else {
				assert.Error(t, err, "BuildTime should be invalid RFC3339")
			}
		})
	}
}