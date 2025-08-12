package config

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestNewPgxZapLogger 测试创建PGX Zap日志适配器
func TestNewPgxZapLogger(t *testing.T) {
	logger := zap.NewNop()
	
	testCases := []struct {
		name          string
		inputLogger   *zap.Logger
		level         string
		expectedLevel tracelog.LogLevel
	}{
		{
			name:          "With valid logger and trace level",
			inputLogger:   logger,
			level:         "trace",
			expectedLevel: tracelog.LogLevelTrace,
		},
		{
			name:          "With valid logger and debug level",
			inputLogger:   logger,
			level:         "debug",
			expectedLevel: tracelog.LogLevelDebug,
		},
		{
			name:          "With valid logger and info level",
			inputLogger:   logger,
			level:         "info",
			expectedLevel: tracelog.LogLevelInfo,
		},
		{
			name:          "With valid logger and warn level",
			inputLogger:   logger,
			level:         "warn",
			expectedLevel: tracelog.LogLevelWarn,
		},
		{
			name:          "With valid logger and error level",
			inputLogger:   logger,
			level:         "error",
			expectedLevel: tracelog.LogLevelError,
		},
		{
			name:          "With valid logger and none level",
			inputLogger:   logger,
			level:         "none",
			expectedLevel: tracelog.LogLevelNone,
		},
		{
			name:          "With invalid level defaults to warn",
			inputLogger:   logger,
			level:         "invalid",
			expectedLevel: tracelog.LogLevelWarn,
		},
		{
			name:          "With nil logger creates nop logger",
			inputLogger:   nil,
			level:         "info",
			expectedLevel: tracelog.LogLevelInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pgxLogger := NewPgxZapLogger(tc.inputLogger, tc.level)
			
			assert.NotNil(t, pgxLogger)
			assert.Equal(t, tc.expectedLevel, pgxLogger.level)
			assert.NotNil(t, pgxLogger.logger)
		})
	}
}

// TestParsePgxLogLevel 测试日志级别解析
func TestParsePgxLogLevel(t *testing.T) {
	testCases := []struct {
		input    string
		expected tracelog.LogLevel
	}{
		{"trace", tracelog.LogLevelTrace},
		{"debug", tracelog.LogLevelDebug},
		{"info", tracelog.LogLevelInfo},
		{"warn", tracelog.LogLevelWarn},
		{"error", tracelog.LogLevelError},
		{"none", tracelog.LogLevelNone},
		{"", tracelog.LogLevelWarn}, // 默认值
		{"invalid", tracelog.LogLevelWarn}, // 默认值
		{"TRACE", tracelog.LogLevelWarn}, // 大小写敏感，默认值
		{"Info", tracelog.LogLevelWarn}, // 大小写敏感，默认值
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := parsePgxLogLevel(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestPgxZapLogger_GetLogLevel 测试获取日志级别
func TestPgxZapLogger_GetLogLevel(t *testing.T) {
	logger := zap.NewNop()
	pgxLogger := NewPgxZapLogger(logger, "debug")
	
	assert.Equal(t, tracelog.LogLevelDebug, pgxLogger.GetLogLevel())
}

// TestPgxZapLogger_SetLogLevel 测试设置日志级别
func TestPgxZapLogger_SetLogLevel(t *testing.T) {
	logger := zap.NewNop()
	pgxLogger := NewPgxZapLogger(logger, "info")
	
	// 初始级别应该是info
	assert.Equal(t, tracelog.LogLevelInfo, pgxLogger.GetLogLevel())
	
	// 设置为error级别
	pgxLogger.SetLogLevel(tracelog.LogLevelError)
	assert.Equal(t, tracelog.LogLevelError, pgxLogger.GetLogLevel())
	
	// 设置为trace级别
	pgxLogger.SetLogLevel(tracelog.LogLevelTrace)
	assert.Equal(t, tracelog.LogLevelTrace, pgxLogger.GetLogLevel())
}

// TestPgxZapLogger_Log_LevelFiltering 测试日志级别过滤
func TestPgxZapLogger_Log_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "warn")
	ctx := context.Background()
	
	testCases := []struct {
		name           string
		level          tracelog.LogLevel
		shouldBeLogged bool
	}{
		{"Trace level should be filtered", tracelog.LogLevelTrace, false},
		{"Debug level should be filtered", tracelog.LogLevelDebug, false},
		{"Info level should be filtered", tracelog.LogLevelInfo, false},
		{"Warn level should be logged", tracelog.LogLevelWarn, true},
		{"Error level should be logged", tracelog.LogLevelError, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			
			pgxLogger.Log(ctx, tc.level, "test message", map[string]interface{}{
				"key": "value",
			})

			if tc.shouldBeLogged {
				assert.NotEmpty(t, buf.String(), "Log should be written")
				assert.Contains(t, buf.String(), "test message")
			} else {
				assert.Empty(t, buf.String(), "Log should be filtered out")
			}
		})
	}
}

// TestPgxZapLogger_Log_MessageMapping 测试日志消息映射
func TestPgxZapLogger_Log_MessageMapping(t *testing.T) {
	var buf bytes.Buffer
	// 使用生产配置确保JSON字段名正确
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "trace") // 允许所有级别
	ctx := context.Background()

	testCases := []struct {
		name         string
		level        tracelog.LogLevel
		expectedZapLevel string
	}{
		{"Trace maps to debug", tracelog.LogLevelTrace, "debug"},
		{"Debug maps to debug", tracelog.LogLevelDebug, "debug"},
		{"Info maps to info", tracelog.LogLevelInfo, "info"},
		{"Warn maps to warn", tracelog.LogLevelWarn, "warn"},
		{"Error maps to error", tracelog.LogLevelError, "error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			
			pgxLogger.Log(ctx, tc.level, "test message", map[string]interface{}{})

			logOutput := buf.String()
			assert.NotEmpty(t, logOutput)

			// 解析JSON日志输出
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedZapLevel, logEntry["level"])
			assert.Equal(t, "test message", logEntry["msg"])
		})
	}
}

// TestPgxZapLogger_Log_DataTypes 测试不同数据类型的处理
func TestPgxZapLogger_Log_DataTypes(t *testing.T) {
	var buf bytes.Buffer
	// 使用生产配置确保JSON字段名正确
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "info")
	ctx := context.Background()

	testData := map[string]interface{}{
		"string_field":  "test_string",
		"int_field":     int(42),
		"int32_field":   int32(32),
		"int64_field":   int64(64),
		"float32_field": float32(3.14),
		"float64_field": float64(2.718),
		"bool_field":    true,
		"error_field":   assert.AnError,
		"any_field":     []int{1, 2, 3},
	}

	pgxLogger.Log(ctx, tracelog.LogLevelInfo, "test data types", testData)

	logOutput := buf.String()
	assert.NotEmpty(t, logOutput)

	// 解析JSON日志输出
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	// 验证各种数据类型都被正确处理
	assert.Equal(t, "test_string", logEntry["string_field"])
	assert.Equal(t, float64(42), logEntry["int_field"]) // JSON数字被解析为float64
	assert.Equal(t, float64(32), logEntry["int32_field"])
	assert.Equal(t, float64(64), logEntry["int64_field"])
	assert.InDelta(t, 3.14, logEntry["float32_field"], 0.01)
	assert.InDelta(t, 2.718, logEntry["float64_field"], 0.001)
	assert.Equal(t, true, logEntry["bool_field"])
	assert.Equal(t, assert.AnError.Error(), logEntry["error"])
	assert.NotNil(t, logEntry["any_field"])
}

// TestPgxZapLogger_Log_EmptyData 测试空数据处理
func TestPgxZapLogger_Log_EmptyData(t *testing.T) {
	var buf bytes.Buffer
	// 使用生产配置确保JSON字段名正确
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "info")
	ctx := context.Background()

	// 测试nil数据
	pgxLogger.Log(ctx, tracelog.LogLevelInfo, "test message", nil)
	assert.NotEmpty(t, buf.String())
	buf.Reset()

	// 测试空map
	pgxLogger.Log(ctx, tracelog.LogLevelInfo, "test message", map[string]interface{}{})
	assert.NotEmpty(t, buf.String())
	
	logOutput := buf.String()
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
}

// TestPgxZapLogger_Log_SpecialCharacters 测试特殊字符处理
func TestPgxZapLogger_Log_SpecialCharacters(t *testing.T) {
	var buf bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "info")
	ctx := context.Background()

	specialData := map[string]interface{}{
		"unicode":     "测试中文字符 🚀",
		"quotes":      `"quoted string"`,
		"newlines":    "line1\nline2\n",
		"tabs":        "col1\tcol2",
		"backslash":   "path\\to\\file",
		"empty":       "",
		"null_value":  nil,
	}

	pgxLogger.Log(ctx, tracelog.LogLevelInfo, "test special characters", specialData)

	logOutput := buf.String()
	assert.NotEmpty(t, logOutput)

	// 验证JSON格式正确
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)
	
	// 验证特殊字符被正确处理
	assert.Equal(t, "测试中文字符 🚀", logEntry["unicode"])
	assert.Equal(t, `"quoted string"`, logEntry["quotes"])
	assert.Equal(t, "line1\nline2\n", logEntry["newlines"])
}

// TestPgxZapLogger_Log_ContextCancellation 测试上下文取消（虽然当前实现不使用context）
func TestPgxZapLogger_Log_ContextCancellation(t *testing.T) {
	var buf bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "info")
	
	// 创建取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 即使上下文被取消，日志仍应该正常工作
	pgxLogger.Log(ctx, tracelog.LogLevelInfo, "test message", map[string]interface{}{
		"key": "value",
	})

	assert.NotEmpty(t, buf.String())
}

// TestPgxZapLogger_Log_DefaultLevel 测试默认日志级别映射
func TestPgxZapLogger_Log_DefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	// 使用生产配置确保JSON字段名正确
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "trace")
	ctx := context.Background()

	// 使用一个不在已知级别中但不会被级别检查过滤的值
	// tracelog级别：Error=1, Warn=2, Info=3, Debug=4, Trace=5
	// 使用0作为未知级别，它会通过级别检查（0 <= 5）
	unknownLevel := tracelog.LogLevel(0)
	
	pgxLogger.Log(ctx, unknownLevel, "unknown level test", map[string]interface{}{})

	logOutput := buf.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	// 未知级别应该映射到info级别
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "unknown level test", logEntry["msg"])
}

// TestPgxZapLogger_Integration 集成测试
func TestPgxZapLogger_Integration(t *testing.T) {
	var buf bytes.Buffer
	// 使用生产配置确保JSON字段名正确
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	))

	pgxLogger := NewPgxZapLogger(logger, "debug")
	ctx := context.Background()

	// 模拟一系列数据库操作日志
	operations := []struct {
		level   tracelog.LogLevel
		message string
		data    map[string]interface{}
	}{
		{
			level:   tracelog.LogLevelInfo,
			message: "connection established",
			data:    map[string]interface{}{"host": "localhost", "port": 5432},
		},
		{
			level:   tracelog.LogLevelDebug,
			message: "executing query",
			data:    map[string]interface{}{"sql": "SELECT * FROM users", "duration": 150},
		},
		{
			level:   tracelog.LogLevelWarn,
			message: "slow query detected",
			data:    map[string]interface{}{"sql": "SELECT * FROM large_table", "duration": 5000},
		},
		{
			level:   tracelog.LogLevelError,
			message: "query failed",
			data:    map[string]interface{}{"error": "syntax error", "sql": "SELCT * FROM users"},
		},
	}

	for _, op := range operations {
		pgxLogger.Log(ctx, op.level, op.message, op.data)
	}

	logOutput := buf.String()
	assert.NotEmpty(t, logOutput)

	// 验证所有操作都被记录（因为级别设置为debug）
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")
	assert.Len(t, lines, len(operations))

	// 验证每条日志都是有效的JSON
	for i, line := range lines {
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(line), &logEntry)
		assert.NoError(t, err, "Log line %d should be valid JSON", i)
		assert.Equal(t, operations[i].message, logEntry["msg"])
	}
}

// BenchmarkPgxZapLogger_Log 性能基准测试
func BenchmarkPgxZapLogger_Log(b *testing.B) {
	logger := zap.NewNop() // 使用NOP logger避免IO开销
	pgxLogger := NewPgxZapLogger(logger, "info")
	ctx := context.Background()
	
	data := map[string]interface{}{
		"sql":      "SELECT * FROM users WHERE id = $1",
		"args":     []interface{}{123},
		"duration": 250,
		"rows":     1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pgxLogger.Log(ctx, tracelog.LogLevelInfo, "query executed", data)
	}
}

// BenchmarkPgxZapLogger_LogFiltered 过滤日志的性能基准测试
func BenchmarkPgxZapLogger_LogFiltered(b *testing.B) {
	logger := zap.NewNop()
	pgxLogger := NewPgxZapLogger(logger, "error") // 只记录error级别
	ctx := context.Background()
	
	data := map[string]interface{}{
		"sql": "SELECT * FROM users",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 这些debug级别的日志应该被过滤掉
		pgxLogger.Log(ctx, tracelog.LogLevelDebug, "debug message", data)
	}
}