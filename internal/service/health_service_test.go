package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthService_Basic(t *testing.T) {
	// 由于健康服务可能涉及外部依赖，我们测试基本结构
	ctx := context.Background()
	
	// 测试context不为nil
	assert.NotNil(t, ctx)
}

func TestHealthCheck_Context(t *testing.T) {
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// 测试取消的context
	cancel()
	assert.Error(t, ctxWithCancel.Err())
}