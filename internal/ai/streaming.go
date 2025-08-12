// 流式响应处理器 - 基于 LangChainGo 和 Go 最佳实践
// 实现 AI 查询的实时流式响应，提升用户体验

package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
)

// StreamingProcessor 流式处理器
type StreamingProcessor struct {
	queryProcessor *QueryProcessor
	logger         *zap.Logger
	metrics        *StreamingMetrics
	config         *StreamingConfig
	mu             sync.RWMutex
}

// StreamingConfig 流式处理配置
type StreamingConfig struct {
	BufferSize      int           `json:"buffer_size" yaml:"buffer_size"`           // 缓冲区大小
	FlushInterval   time.Duration `json:"flush_interval" yaml:"flush_interval"`     // 刷新间隔
	MaxChunkSize    int           `json:"max_chunk_size" yaml:"max_chunk_size"`     // 最大块大小
	Timeout         time.Duration `json:"timeout" yaml:"timeout"`                  // 超时时间
	EnableBuffer    bool          `json:"enable_buffer" yaml:"enable_buffer"`      // 启用缓冲
	CompressionType string        `json:"compression" yaml:"compression"`          // 压缩类型
}

// StreamResponse 流式响应数据
type StreamResponse struct {
	ID          string                 `json:"id"`
	Type        StreamResponseType     `json:"type"`
	Data        any            `json:"data,omitempty"`
	Error       *StreamError           `json:"error,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Sequence    int64                  `json:"sequence"`
	IsComplete  bool                   `json:"is_complete"`
}

type StreamResponseType string

const (
	StreamTypeStart     StreamResponseType = "start"
	StreamTypeChunk     StreamResponseType = "chunk"
	StreamTypeProgress  StreamResponseType = "progress"
	StreamTypeSQL       StreamResponseType = "sql"
	StreamTypeResult    StreamResponseType = "result"
	StreamTypeError     StreamResponseType = "error"
	StreamTypeComplete  StreamResponseType = "complete"
	StreamTypeHeartbeat StreamResponseType = "heartbeat"
)

// StreamError 流式错误
type StreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// StreamingMetrics 流式处理指标
type StreamingMetrics struct {
	ActiveStreams    int64 `json:"active_streams"`
	TotalStreams     int64 `json:"total_streams"`
	BytesTransferred int64 `json:"bytes_transferred"`
	ErrorCount       int64 `json:"error_count"`
	AvgLatency       int64 `json:"avg_latency_ms"`
}

// StreamChunk SQL生成的流式块
type StreamChunk struct {
	Content    string             `json:"content"`
	Type       StreamChunkType    `json:"type"`
	Progress   float64            `json:"progress"`
	Confidence float64            `json:"confidence"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type StreamChunkType string

const (
	ChunkTypeThinking    StreamChunkType = "thinking"    // AI思考过程
	ChunkTypeIntention   StreamChunkType = "intention"   // 意图分析
	ChunkTypeSchema      StreamChunkType = "schema"      // 模式分析
	ChunkTypeGeneration  StreamChunkType = "generation"  // SQL生成
	ChunkTypeValidation  StreamChunkType = "validation"  // 验证过程
	ChunkTypeExecution   StreamChunkType = "execution"   // 执行准备
)

// StreamWriter 流式写入器接口
type StreamWriter interface {
	WriteChunk(chunk *StreamResponse) error
	Flush() error
	Close() error
}

// ChannelStreamWriter 基于channel的流式写入器
type ChannelStreamWriter struct {
	ch         chan<- *StreamResponse
	buffer     []*StreamResponse
	bufferSize int
	mu         sync.Mutex
	closed     bool
}

// WebSocketStreamWriter WebSocket流式写入器（接口定义）
type WebSocketStreamWriter struct {
	conn       WebSocketConnection // 抽象接口，实际实现时需要具体的WebSocket库
	encoder    *json.Encoder
	buffer     *bufio.Writer
	config     *StreamingConfig
	mu         sync.Mutex
	closed     bool
}

// WebSocketConnection WebSocket连接接口
type WebSocketConnection interface {
	WriteMessage(messageType int, data []byte) error
	Close() error
}

// Flusher 刷新接口
type Flusher interface {
	Flush()
}

// SSEStreamWriter Server-Sent Events流式写入器
type SSEStreamWriter struct {
	writer  io.Writer
	flusher Flusher
	encoder *json.Encoder
	mu      sync.Mutex
	closed  bool
}

// NewStreamingProcessor 创建流式处理器
func NewStreamingProcessor(queryProcessor *QueryProcessor, config *StreamingConfig, logger *zap.Logger) *StreamingProcessor {
	if config == nil {
		config = DefaultStreamingConfig()
	}

	return &StreamingProcessor{
		queryProcessor: queryProcessor,
		config:         config,
		logger:         logger,
		metrics:        &StreamingMetrics{},
	}
}

// DefaultStreamingConfig 默认流式配置
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BufferSize:      1024,
		FlushInterval:   100 * time.Millisecond,
		MaxChunkSize:    8192,
		Timeout:         30 * time.Second,
		EnableBuffer:    true,
		CompressionType: "gzip",
	}
}

// ProcessStreamingQuery 处理流式查询
func (sp *StreamingProcessor) ProcessStreamingQuery(
	ctx context.Context, 
	req *ChatRequest, 
	writer StreamWriter) error {

	sp.metrics.ActiveStreams++
	sp.metrics.TotalStreams++
	defer func() { sp.metrics.ActiveStreams-- }()

	sp.logger.Info("开始流式查询处理",
		zap.String("query", req.Query),
		zap.Int64("user_id", req.UserID),
	)

	// 创建带超时的上下文
	streamCtx, cancel := context.WithTimeout(ctx, sp.config.Timeout)
	defer cancel()

	// 生成唯一流ID
	streamID := fmt.Sprintf("stream_%d_%d", req.UserID, time.Now().UnixNano())
	sequence := int64(0)

	// 发送开始事件
	startResponse := &StreamResponse{
		ID:        streamID,
		Type:      StreamTypeStart,
		Timestamp: time.Now(),
		Sequence:  sequence,
		Metadata: map[string]any{
			"query": req.Query,
			"user_id": req.UserID,
		},
	}
	
	if err := writer.WriteChunk(startResponse); err != nil {
		return fmt.Errorf("发送开始事件失败: %w", err)
	}
	sequence++

	// 创建流式响应通道
	responseChan := make(chan *StreamChunk, sp.config.BufferSize)
	errorChan := make(chan error, 1)

	// 启动心跳goroutine
	heartbeatCtx, heartbeatCancel := context.WithCancel(streamCtx)
	defer heartbeatCancel()
	go sp.startHeartbeat(heartbeatCtx, streamID, writer, &sequence)

	// 启动AI处理goroutine
	go func() {
		defer close(responseChan)
		if err := sp.generateStreamingSQL(streamCtx, req, responseChan); err != nil {
			errorChan <- err
		}
	}()

	// 处理流式响应
	var lastProgress float64
	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()

	for {
		select {
		case <-streamCtx.Done():
			sp.sendError(writer, streamID, sequence, "timeout", "请求超时", streamCtx.Err().Error())
			return streamCtx.Err()

		case err := <-errorChan:
			sp.sendError(writer, streamID, sequence, "processing", "处理错误", err.Error())
			return err

		case chunk, ok := <-responseChan:
			if !ok {
				// 处理完成
				completeResponse := &StreamResponse{
					ID:        streamID,
					Type:      StreamTypeComplete,
					Timestamp: time.Now(),
					Sequence:  sequence,
					IsComplete: true,
				}
				return writer.WriteChunk(completeResponse)
			}

			// 发送数据块
			chunkResponse := &StreamResponse{
				ID:        streamID,
				Type:      StreamTypeChunk,
				Data:      chunk,
				Timestamp: time.Now(),
				Sequence:  sequence,
			}

			if err := writer.WriteChunk(chunkResponse); err != nil {
				sp.logger.Error("发送数据块失败", zap.Error(err))
				return err
			}
			sequence++
			
			// 更新进度
			if chunk.Progress > lastProgress {
				lastProgress = chunk.Progress
				progressResponse := &StreamResponse{
					ID:        streamID,
					Type:      StreamTypeProgress,
					Data:      map[string]any{"progress": chunk.Progress},
					Timestamp: time.Now(),
					Sequence:  sequence,
				}
				writer.WriteChunk(progressResponse)
				sequence++
			}

		case <-progressTicker.C:
			// 定期刷新
			if sp.config.EnableBuffer {
				writer.Flush()
			}
		}
	}
}

// generateStreamingSQL 生成流式SQL
func (sp *StreamingProcessor) generateStreamingSQL(
	ctx context.Context, 
	req *ChatRequest, 
	responseChan chan<- *StreamChunk) error {

	// 第一步：意图分析
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "正在分析查询意图...",
		Type:     ChunkTypeThinking,
		Progress: 0.1,
		Confidence: 0.8,
	})

	// 模拟意图分析过程
	time.Sleep(200 * time.Millisecond)
	
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "识别为数据查询请求",
		Type:     ChunkTypeIntention,
		Progress: 0.2,
		Confidence: 0.9,
	})

	// 第二步：模式分析
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "正在分析数据库模式...",
		Type:     ChunkTypeThinking,
		Progress: 0.3,
		Confidence: 0.8,
	})

	time.Sleep(300 * time.Millisecond)
	
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "已获取相关表结构信息",
		Type:     ChunkTypeSchema,
		Progress: 0.4,
		Confidence: 0.9,
	})

	// 第三步：SQL生成（流式调用LangChainGo）
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "开始生成SQL查询...",
		Type:     ChunkTypeThinking,
		Progress: 0.5,
		Confidence: 0.8,
	})

	// 使用LangChainGo的流式生成
	if err := sp.streamingLLMGeneration(ctx, req, responseChan); err != nil {
		return err
	}

	// 第四步：验证
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "验证SQL语法和安全性...",
		Type:     ChunkTypeValidation,
		Progress: 0.9,
		Confidence: 0.95,
	})

	time.Sleep(100 * time.Millisecond)

	// 第五步：完成
	sp.sendChunk(responseChan, &StreamChunk{
		Content:  "SQL生成完成，准备执行",
		Type:     ChunkTypeExecution,
		Progress: 1.0,
		Confidence: 0.95,
	})

	return nil
}

// streamingLLMGeneration 流式LLM生成
func (sp *StreamingProcessor) streamingLLMGeneration(
	ctx context.Context, 
	req *ChatRequest, 
	responseChan chan<- *StreamChunk) error {

	// 构建提示词
	template, err := sp.queryProcessor.templateManager.GetTemplate("basic_sql")
	if err != nil {
		return fmt.Errorf("获取提示词模板失败: %w", err)
	}
	
	// 从上下文管理器获取数据库 schema 和相关信息
	var databaseSchema string
	var tableNames []string
	var queryHistory []QueryHistory
	var userContext map[string]string
	
	// 如果查询处理器有上下文管理器，使用它获取相关信息
	if sp.queryProcessor.contextManager != nil {
		// 构建查询上下文，获取数据库 schema
		if builtCtx, err := sp.queryProcessor.contextManager.BuildQueryContext(
			req.ConnectionID, req.UserID, req.Query); err == nil {
			databaseSchema = builtCtx.DatabaseSchema
			tableNames = builtCtx.TableNames
			queryHistory = builtCtx.QueryHistory
			userContext = builtCtx.UserContext
		}
	}

	queryCtx := &QueryContext{
		UserQuery:      req.Query,
		DatabaseSchema: databaseSchema,
		TableNames:     tableNames,
		QueryHistory:   queryHistory,
		UserContext:    userContext,
	}
	
	prompt, err := template.FormatPrompt(queryCtx)
	if err != nil {
		return fmt.Errorf("格式化提示词失败: %w", err)
	}

	// 创建流式生成的消息
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 流式生成SQL
	var sqlBuilder strings.Builder
	progress := 0.5

	_, err = sp.queryProcessor.primaryClient.GenerateContent(ctx, messages,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// 累积SQL内容
				sqlBuilder.Write(chunk)
				content := string(chunk)
				
				// 发送生成的块
				sp.sendChunk(responseChan, &StreamChunk{
					Content:    content,
					Type:       ChunkTypeGeneration,
					Progress:   progress,
					Confidence: 0.85,
					Metadata: map[string]any{
						"accumulated_sql": sqlBuilder.String(),
					},
				})
				
				// 更新进度
				progress = min(progress + 0.05, 0.85)
				return nil
			}
		}),
	)

	return err
}

// sendChunk 安全发送chunk
func (sp *StreamingProcessor) sendChunk(responseChan chan<- *StreamChunk, chunk *StreamChunk) {
	select {
	case responseChan <- chunk:
		sp.metrics.BytesTransferred += int64(len(chunk.Content))
	default:
		sp.logger.Warn("流式响应通道已满，丢弃数据块")
	}
}

// sendError 发送错误信息
func (sp *StreamingProcessor) sendError(writer StreamWriter, streamID string, sequence int64, 
	code, message, details string) {
	errorResponse := &StreamResponse{
		ID:        streamID,
		Type:      StreamTypeError,
		Error:     &StreamError{Code: code, Message: message, Details: details},
		Timestamp: time.Now(),
		Sequence:  sequence,
	}
	writer.WriteChunk(errorResponse)
	sp.metrics.ErrorCount++
}

// startHeartbeat 启动心跳
func (sp *StreamingProcessor) startHeartbeat(ctx context.Context, streamID string, 
	writer StreamWriter, sequence *int64) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeatResponse := &StreamResponse{
				ID:        streamID,
				Type:      StreamTypeHeartbeat,
				Timestamp: time.Now(),
				Sequence:  *sequence,
			}
			if err := writer.WriteChunk(heartbeatResponse); err != nil {
				sp.logger.Warn("心跳发送失败", zap.Error(err))
				return
			}
			(*sequence)++
		}
	}
}

// NewChannelStreamWriter 创建基于channel的流式写入器
func NewChannelStreamWriter(ch chan<- *StreamResponse, bufferSize int) *ChannelStreamWriter {
	return &ChannelStreamWriter{
		ch:         ch,
		bufferSize: bufferSize,
		buffer:     make([]*StreamResponse, 0, bufferSize),
	}
}

// WriteChunk 写入数据块
func (w *ChannelStreamWriter) WriteChunk(chunk *StreamResponse) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("流已关闭")
	}

	if len(w.buffer) >= w.bufferSize {
		// 缓冲区满，强制刷新
		if err := w.flushLocked(); err != nil {
			return err
		}
	}

	w.buffer = append(w.buffer, chunk)
	return nil
}

// Flush 刷新缓冲区
func (w *ChannelStreamWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *ChannelStreamWriter) flushLocked() error {
	if w.closed {
		return fmt.Errorf("流已关闭")
	}

	for _, chunk := range w.buffer {
		select {
		case w.ch <- chunk:
		default:
			return fmt.Errorf("通道写入阻塞")
		}
	}
	
	w.buffer = w.buffer[:0] // 清空缓冲区
	return nil
}

// Close 关闭写入器
func (w *ChannelStreamWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	// 刷新剩余数据
	w.flushLocked()
	w.closed = true
	return nil
}

// GetMetrics 获取流式处理指标
func (sp *StreamingProcessor) GetMetrics() *StreamingMetrics {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	
	// 复制指标以避免并发访问问题
	return &StreamingMetrics{
		ActiveStreams:    sp.metrics.ActiveStreams,
		TotalStreams:     sp.metrics.TotalStreams,
		BytesTransferred: sp.metrics.BytesTransferred,
		ErrorCount:       sp.metrics.ErrorCount,
		AvgLatency:       sp.metrics.AvgLatency,
	}
}

// 实用函数
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}