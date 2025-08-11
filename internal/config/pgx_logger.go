package config

import (
	"context"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
)

// PgxZapLogger pgx和zap的适配器
// 将pgx的日志输出重定向到zap日志系统
type PgxZapLogger struct {
	logger *zap.Logger
	level  tracelog.LogLevel
}

// NewPgxZapLogger 创建新的PGX Zap日志适配器
func NewPgxZapLogger(logger *zap.Logger, level string) *PgxZapLogger {
	if logger == nil {
		logger = zap.NewNop()
	}

	pgxLevel := parsePgxLogLevel(level)
	
	return &PgxZapLogger{
		logger: logger,
		level:  pgxLevel,
	}
}

// Log 实现tracelog.Logger接口
func (l *PgxZapLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	// 检查日志级别
	if level < l.level {
		return
	}

	fields := make([]zap.Field, 0, len(data))
	for key, value := range data {
		switch v := value.(type) {
		case string:
			fields = append(fields, zap.String(key, v))
		case int:
			fields = append(fields, zap.Int(key, v))
		case int32:
			fields = append(fields, zap.Int32(key, v))
		case int64:
			fields = append(fields, zap.Int64(key, v))
		case float32:
			fields = append(fields, zap.Float32(key, v))
		case float64:
			fields = append(fields, zap.Float64(key, v))
		case bool:
			fields = append(fields, zap.Bool(key, v))
		case error:
			fields = append(fields, zap.Error(v))
		default:
			fields = append(fields, zap.Any(key, v))
		}
	}

	// 根据pgx日志级别映射到zap日志级别
	switch level {
	case tracelog.LogLevelTrace:
		l.logger.Debug(msg, fields...)
	case tracelog.LogLevelDebug:
		l.logger.Debug(msg, fields...)
	case tracelog.LogLevelInfo:
		l.logger.Info(msg, fields...)
	case tracelog.LogLevelWarn:
		l.logger.Warn(msg, fields...)
	case tracelog.LogLevelError:
		l.logger.Error(msg, fields...)
	default:
		l.logger.Info(msg, fields...)
	}
}

// parsePgxLogLevel 解析字符串日志级别到pgx LogLevel
func parsePgxLogLevel(level string) tracelog.LogLevel {
	switch level {
	case "trace":
		return tracelog.LogLevelTrace
	case "debug":
		return tracelog.LogLevelDebug
	case "info":
		return tracelog.LogLevelInfo
	case "warn":
		return tracelog.LogLevelWarn
	case "error":
		return tracelog.LogLevelError
	case "none":
		return tracelog.LogLevelNone
	default:
		return tracelog.LogLevelWarn // 默认为warn级别
	}
}

// GetLogLevel 获取当前日志级别
func (l *PgxZapLogger) GetLogLevel() tracelog.LogLevel {
	return l.level
}

// SetLogLevel 设置日志级别
func (l *PgxZapLogger) SetLogLevel(level tracelog.LogLevel) {
	l.level = level
}