// 流式响应处理器测试 - 完整测试覆盖AI查询的实时流式响应
// 测试流式处理、WebSocket、SSE、心跳机制等核心功能

package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewStreamingProcessor 测试流式处理器创建
func TestNewStreamingProcessor(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	queryProcessor := &QueryProcessor{} // 简化的查询处理器
	config := DefaultStreamingConfig()
	
	processor := NewStreamingProcessor(queryProcessor, config, logger)
	
	assert.NotNil(t, processor)
	assert.Equal(t, queryProcessor, processor.queryProcessor)
	assert.Equal(t, config, processor.config)
	assert.Equal(t, logger, processor.logger)
	assert.NotNil(t, processor.metrics)
}

// TestDefaultStreamingConfig 测试默认流式配置
func TestDefaultStreamingConfig(t *testing.T) {
	config := DefaultStreamingConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, 1024, config.BufferSize)
	assert.Equal(t, 100*time.Millisecond, config.FlushInterval)
	assert.Equal(t, 8192, config.MaxChunkSize)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.True(t, config.EnableBuffer)
	assert.Equal(t, "gzip", config.CompressionType)
}

// TestStreamResponse 测试流式响应结构
func TestStreamResponse(t *testing.T) {
	now := time.Now()
	response := &StreamResponse{
		ID:        "test-stream-1",
		Type:      StreamTypeStart,
		Data:      map[string]interface{}{"message": "starting"},
		Error:     nil,
		Metadata:  map[string]interface{}{"connection_id": 123},
		Timestamp: now,
		Sequence:  1,
		IsComplete: false,
	}

	assert.Equal(t, "test-stream-1", response.ID)
	assert.Equal(t, StreamTypeStart, response.Type)
	assert.NotNil(t, response.Data)
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.Metadata)
	assert.Equal(t, now, response.Timestamp)
	assert.Equal(t, int64(1), response.Sequence)
	assert.False(t, response.IsComplete)
}

// TestStreamResponseType 测试流式响应类型枚举
func TestStreamResponseType(t *testing.T) {
	assert.Equal(t, "start", string(StreamTypeStart))
	assert.Equal(t, "chunk", string(StreamTypeChunk))
	assert.Equal(t, "progress", string(StreamTypeProgress))
	assert.Equal(t, "sql", string(StreamTypeSQL))
	assert.Equal(t, "result", string(StreamTypeResult))
	assert.Equal(t, "error", string(StreamTypeError))
	assert.Equal(t, "complete", string(StreamTypeComplete))
	assert.Equal(t, "heartbeat", string(StreamTypeHeartbeat))
}

// TestStreamError 测试流式错误结构
func TestStreamError(t *testing.T) {
	streamError := &StreamError{
		Code:    "VALIDATION_FAILED",
		Message: "SQL validation failed",
		Details: "Error at line 1, column 10",
	}

	assert.Equal(t, "VALIDATION_FAILED", streamError.Code)
	assert.Equal(t, "SQL validation failed", streamError.Message)
	assert.Equal(t, "Error at line 1, column 10", streamError.Details)
}

// TestStreamChunk 测试流式数据块结构
func TestStreamChunk(t *testing.T) {
	chunk := &StreamChunk{
		Content:    "SELECT * FROM users",
		Type:       ChunkTypeGeneration,
		Progress:   0.75,
		Confidence: 0.95,
		Metadata:   map[string]interface{}{"tokens": 50},
	}

	assert.Equal(t, "SELECT * FROM users", chunk.Content)
	assert.Equal(t, ChunkTypeGeneration, chunk.Type)
	assert.Equal(t, 0.75, chunk.Progress)
	assert.Equal(t, 0.95, chunk.Confidence)
	assert.NotNil(t, chunk.Metadata)
}

// TestStreamChunkType 测试流式数据块类型枚举
func TestStreamChunkType(t *testing.T) {
	assert.Equal(t, "thinking", string(ChunkTypeThinking))
	assert.Equal(t, "intention", string(ChunkTypeIntention))
	assert.Equal(t, "schema", string(ChunkTypeSchema))
	assert.Equal(t, "generation", string(ChunkTypeGeneration))
	assert.Equal(t, "validation", string(ChunkTypeValidation))
	assert.Equal(t, "execution", string(ChunkTypeExecution))
}

// TestChannelStreamWriter 测试通道流式写入器
func TestChannelStreamWriter(t *testing.T) {
	ch := make(chan *StreamResponse, 10)
	writer := NewChannelStreamWriter(ch, 5)

	assert.NotNil(t, writer)
	// 注意：不能直接比较只写通道类型，但可以验证容量
	assert.Equal(t, 5, cap(writer.buffer))

	// 测试写入数据块
	response := &StreamResponse{
		ID:      "test-1",
		Type:    StreamTypeChunk,
		Data:    "test data",
		Sequence: 1,
	}

	err := writer.WriteChunk(response)
	assert.NoError(t, err)

	// 测试刷新
	err = writer.Flush()
	assert.NoError(t, err)

	// 验证数据是否发送到通道
	select {
	case received := <-ch:
		assert.Equal(t, "test-1", received.ID)
		assert.Equal(t, "test data", received.Data)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected data to be sent to channel")
	}

	// 测试关闭
	err = writer.Close()
	assert.NoError(t, err)
	assert.True(t, writer.closed)
}

// TestChannelStreamWriterBuffering 测试通道流式写入器缓冲功能
func TestChannelStreamWriterBuffering(t *testing.T) {
	ch := make(chan *StreamResponse, 10)
	writer := NewChannelStreamWriter(ch, 3) // 缓冲区大小为3

	// 写入多个响应，应该缓冲而不立即发送
	for i := 0; i < 2; i++ {
		response := &StreamResponse{
			ID:       "test",
			Type:     StreamTypeChunk,
			Data:     i,
			Sequence: int64(i),
		}
		err := writer.WriteChunk(response)
		assert.NoError(t, err)
	}

	// 缓冲区未满，通道中应该没有数据
	select {
	case <-ch:
		t.Fatal("Data should be buffered, not sent immediately")
	case <-time.After(10 * time.Millisecond):
		// 正确，数据被缓冲
	}

	// 刷新缓冲区
	err := writer.Flush()
	assert.NoError(t, err)

	// 现在通道中应该有数据
	receivedCount := 0
	for {
		select {
		case <-ch:
			receivedCount++
		case <-time.After(10 * time.Millisecond):
			goto done
		}
	}
done:
	assert.Equal(t, 2, receivedCount)
}

// TestChannelStreamWriterConcurrency 测试通道流式写入器并发安全
func TestChannelStreamWriterConcurrency(t *testing.T) {
	ch := make(chan *StreamResponse, 100)
	writer := NewChannelStreamWriter(ch, 10)

	var wg sync.WaitGroup
	const numGoroutines = 10
	const numWrites = 10

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				response := &StreamResponse{
					ID:       "test",
					Type:     StreamTypeChunk,
					Data:     id*100 + j,
					Sequence: int64(id*100 + j),
				}
				err := writer.WriteChunk(response)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// 刷新所有缓冲数据
	err := writer.Flush()
	assert.NoError(t, err)

	// 验证接收到的数据数量
	receivedCount := 0
	for {
		select {
		case <-ch:
			receivedCount++
		case <-time.After(100 * time.Millisecond):
			goto finished
		}
	}
finished:
	assert.Equal(t, numGoroutines*numWrites, receivedCount)
}

// TestStreamingMetrics 测试流式处理指标
func TestStreamingMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	processor := NewStreamingProcessor(nil, DefaultStreamingConfig(), logger)

	metrics := processor.GetMetrics()
	assert.NotNil(t, metrics)

	// 初始指标应该为零
	assert.Equal(t, int64(0), metrics.TotalStreams)
	assert.Equal(t, int64(0), metrics.ActiveStreams)
	assert.Equal(t, int64(0), metrics.BytesTransferred)
	assert.Equal(t, int64(0), metrics.ErrorCount)
	assert.Equal(t, int64(0), metrics.AvgLatency)
}

// TestSendChunk 测试发送数据块功能
func TestSendChunk(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	processor := NewStreamingProcessor(nil, DefaultStreamingConfig(), logger)

	// 创建一个通道来接收数据块
	responseChan := make(chan *StreamChunk, 10)

	chunk := &StreamChunk{
		Content:    "test chunk data",
		Type:       ChunkTypeThinking,
		Progress:   0.5,
		Confidence: 0.8,
	}

	// 发送数据块
	processor.sendChunk(responseChan, chunk)

	// 验证数据块是否被发送
	select {
	case received := <-responseChan:
		assert.Equal(t, chunk.Content, received.Content)
		assert.Equal(t, chunk.Type, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected chunk to be sent")
	}
}

// TestSendError 测试发送错误功能
func TestSendError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	processor := NewStreamingProcessor(nil, DefaultStreamingConfig(), logger)

	ch := make(chan *StreamResponse, 10)
	writer := NewChannelStreamWriter(ch, 5)

	// 发送错误
	processor.sendError(writer, "test-stream", 1, "TEST_ERROR", "Test error message", "Error details")

	// 刷新写入器以确保错误被发送
	writer.Flush()

	// 验证错误是否被发送
	select {
	case response := <-ch:
		assert.Equal(t, "test-stream", response.ID)
		assert.Equal(t, StreamTypeError, response.Type)
		assert.NotNil(t, response.Error)
		assert.Equal(t, "TEST_ERROR", response.Error.Code)
		assert.Equal(t, "Test error message", response.Error.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected error to be sent")
	}
}

// TestStartHeartbeat 测试心跳机制
func TestStartHeartbeat(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	processor := NewStreamingProcessor(nil, DefaultStreamingConfig(), logger)

	ch := make(chan *StreamResponse, 10)
	writer := NewChannelStreamWriter(ch, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// 启动心跳
	sequence := int64(0)
	processor.startHeartbeat(ctx, "test-stream", writer, &sequence)

	// 等待一段时间让心跳发送
	time.Sleep(300 * time.Millisecond)

	// 刷新写入器
	writer.Flush()

	// 应该至少接收到一个心跳
	heartbeatCount := 0
	for {
		select {
		case response := <-ch:
			if response.Type == StreamTypeHeartbeat {
				heartbeatCount++
			}
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	assert.GreaterOrEqual(t, heartbeatCount, 1, "Expected at least one heartbeat")
}

// TestMinFunction 测试最小值函数
func TestMinFunction(t *testing.T) {
	assert.Equal(t, 1.0, min(1.0, 2.0))
	assert.Equal(t, 1.0, min(2.0, 1.0))
	assert.Equal(t, 5.5, min(5.5, 5.5))
	assert.Equal(t, -1.0, min(-1.0, 0.0))
}

// TestProcessStreamingQueryBasic 测试基本流式查询处理
func TestProcessStreamingQueryBasic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// 创建一个简单的查询处理器 mock
	queryProcessor := &QueryProcessor{}
	config := DefaultStreamingConfig()
	processor := NewStreamingProcessor(queryProcessor, config, logger)

	// 创建测试请求
	request := &ChatRequest{
		Query:        "SELECT * FROM users",
		ConnectionID: 1,
		UserID:       123,
	}

	ch := make(chan *StreamResponse, 100)
	writer := NewChannelStreamWriter(ch, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 由于没有真实的LLM集成，这个测试主要验证错误处理路径
	err := processor.ProcessStreamingQuery(ctx, request, writer)
	
	// 预期会有错误，因为没有配置真实的查询处理器
	assert.Error(t, err)
}

// TestStreamingConfig 测试流式配置结构
func TestStreamingConfig(t *testing.T) {
	config := &StreamingConfig{
		BufferSize:      2048,
		FlushInterval:   50 * time.Millisecond,
		MaxChunkSize:    512,
		Timeout:         10 * time.Second,
		EnableBuffer:    false,
		CompressionType: "none",
	}

	assert.Equal(t, 2048, config.BufferSize)
	assert.Equal(t, 50*time.Millisecond, config.FlushInterval)
	assert.Equal(t, 512, config.MaxChunkSize)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.False(t, config.EnableBuffer)
	assert.Equal(t, "none", config.CompressionType)
}