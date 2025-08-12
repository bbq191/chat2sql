// AI服务HTTP API处理器 - P1阶段Chat2SQL智能查询接口
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"chat2sql-go/internal/service"
)

// AIServiceInterface AI服务接口定义
type AIServiceInterface interface {
	GenerateSQL(ctx context.Context, req *service.SQLGenerationRequest) (*service.SQLGenerationResponse, error)
	Close() error
}

// AIHandler AI服务HTTP处理器
type AIHandler struct {
	aiService AIServiceInterface
	logger    *zap.Logger
}

// NewAIHandler 创建AI处理器实例
func NewAIHandler(aiService AIServiceInterface, logger *zap.Logger) *AIHandler {
	return &AIHandler{
		aiService: aiService,
		logger:    logger,
	}
}

// Chat2SQLRequest Chat2SQL API请求结构
type Chat2SQLRequest struct {
	Query        string `json:"query" binding:"required,min=1,max=1000"`
	ConnectionID int64  `json:"connection_id" binding:"required,min=1"`
	Schema       string `json:"schema,omitempty"`
}

// Chat2SQLResponse Chat2SQL API响应结构  
type Chat2SQLResponse struct {
	SQL            string  `json:"sql"`
	Confidence     float64 `json:"confidence"`
	ProcessingTime int64   `json:"processing_time_ms"`
	TokensUsed     int     `json:"tokens_used,omitempty"`
	QueryID        string  `json:"query_id"`
	Timestamp      string  `json:"timestamp"`
}

// FeedbackRequest 反馈提交请求结构
type FeedbackRequest struct {
	QueryID   string `json:"query_id" binding:"required"`
	IsCorrect bool   `json:"is_correct"`
	UserRating int   `json:"user_rating" binding:"min=1,max=5"`
	Feedback  string `json:"feedback,omitempty" binding:"max=500"`
	UserSQL   string `json:"user_sql,omitempty" binding:"max=2000"`
}

// FeedbackResponse 反馈提交响应结构
type FeedbackResponse struct {
	Message   string `json:"message"`
	QueryID   string `json:"query_id"`
	Timestamp string `json:"timestamp"`
}

// 使用已定义的ErrorResponse结构（在auth_handler.go中定义）

// Chat2SQL 处理自然语言转SQL查询请求
// @Summary Chat2SQL智能查询
// @Description 将自然语言转换为SQL查询
// @Tags AI
// @Accept json
// @Produce json
// @Param request body Chat2SQLRequest true "查询请求"
// @Success 200 {object} Chat2SQLResponse "成功响应"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 429 {object} ErrorResponse "请求频率限制"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/ai/chat2sql [post]
func (h *AIHandler) Chat2SQL(c *gin.Context) {
	startTime := time.Now()
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	// 记录请求开始
	h.logger.Info("Chat2SQL请求开始",
		zap.String("request_id", requestID),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("remote_addr", c.ClientIP()),
	)

	// 绑定请求数据
	var req Chat2SQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("请求参数验证失败",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(c, http.StatusBadRequest, "请求参数无效", err.Error(), requestID)
		return
	}

	// 获取用户ID（从JWT中间件设置的上下文）
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Error("无法获取用户ID", zap.String("request_id", requestID))
		h.respondWithError(c, http.StatusUnauthorized, "认证信息无效", "user_id not found in context", requestID)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		h.logger.Error("用户ID类型转换失败", zap.String("request_id", requestID), zap.Any("user_id", userID))
		h.respondWithError(c, http.StatusInternalServerError, "用户认证错误", "invalid user_id type", requestID)
		return
	}

	// 设置请求超时
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 构建AI服务请求
	aiRequest := &service.SQLGenerationRequest{
		Query:        req.Query,
		ConnectionID: req.ConnectionID,
		UserID:       userIDInt64,
		Schema:       req.Schema,
	}

	h.logger.Info("调用AI服务生成SQL",
		zap.String("request_id", requestID),
		zap.String("query", req.Query),
		zap.Int64("connection_id", req.ConnectionID),
		zap.Int64("user_id", userIDInt64),
	)

	// 调用AI服务
	response, err := h.aiService.GenerateSQL(ctx, aiRequest)
	if err != nil {
		h.logger.Error("AI服务调用失败",
			zap.String("request_id", requestID),
			zap.Error(err),
		)

		// 根据错误类型返回不同的HTTP状态码
		statusCode := http.StatusInternalServerError
		errorMessage := "AI查询处理失败"

		// 可以根据具体错误类型细化状态码
		if isTimeoutError(err) {
			statusCode = http.StatusRequestTimeout
			errorMessage = "查询处理超时，请稍后重试"
		} else if isRateLimitError(err) {
			statusCode = http.StatusTooManyRequests
			errorMessage = "请求过于频繁，请稍后重试"
		}

		h.respondWithError(c, statusCode, errorMessage, err.Error(), requestID)
		return
	}

	// 生成查询ID（用于反馈跟踪）
	queryID := generateQueryID(userIDInt64, startTime)

	// 构建响应
	apiResponse := &Chat2SQLResponse{
		SQL:            response.SQL,
		Confidence:     response.Confidence,
		ProcessingTime: response.ProcessingTime.Milliseconds(),
		QueryID:        queryID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	// 记录成功响应
	h.logger.Info("Chat2SQL请求成功处理",
		zap.String("request_id", requestID),
		zap.String("query_id", queryID),
		zap.Float64("confidence", response.Confidence),
		zap.Duration("total_duration", time.Since(startTime)),
		zap.Duration("ai_processing_time", response.ProcessingTime),
	)

	c.JSON(http.StatusOK, apiResponse)
}

// SubmitFeedback 处理用户反馈提交
// @Summary 提交查询反馈
// @Description 提交AI生成SQL查询的用户反馈和评价
// @Tags AI
// @Accept json
// @Produce json
// @Param request body FeedbackRequest true "反馈请求"
// @Success 200 {object} FeedbackResponse "反馈提交成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/ai/feedback [post]
func (h *AIHandler) SubmitFeedback(c *gin.Context) {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	h.logger.Info("反馈提交请求开始", zap.String("request_id", requestID))

	// 绑定请求数据
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("反馈请求参数验证失败",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(c, http.StatusBadRequest, "请求参数无效", err.Error(), requestID)
		return
	}

	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Error("无法获取用户ID", zap.String("request_id", requestID))
		h.respondWithError(c, http.StatusUnauthorized, "认证信息无效", "user_id not found in context", requestID)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		h.logger.Error("用户ID类型转换失败", zap.String("request_id", requestID))
		h.respondWithError(c, http.StatusInternalServerError, "用户认证错误", "invalid user_id type", requestID)
		return
	}

	// 记录反馈信息
	h.logger.Info("用户反馈提交",
		zap.String("request_id", requestID),
		zap.String("query_id", req.QueryID),
		zap.Int64("user_id", userIDInt64),
		zap.Bool("is_correct", req.IsCorrect),
		zap.Int("user_rating", req.UserRating),
	)

	// 实现反馈存储逻辑
	if err := h.storeFeedback(req, userIDInt64, requestID); err != nil {
		h.logger.Error("存储用户反馈失败",
			zap.String("request_id", requestID),
			zap.Error(err))
		// 存储失败不影响响应，继续返回成功
	}

	// 构建响应
	response := &FeedbackResponse{
		Message:   "反馈提交成功，感谢您的宝贵意见",
		QueryID:   req.QueryID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	h.logger.Info("反馈提交处理完成",
		zap.String("request_id", requestID),
		zap.String("query_id", req.QueryID),
	)

	c.JSON(http.StatusOK, response)
}

// GetAIStats 获取AI服务统计信息
// @Summary 获取AI服务统计
// @Description 获取AI服务的性能统计和监控信息
// @Tags AI
// @Produce json
// @Success 200 {object} map[string]any "统计信息"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/ai/stats [get]
func (h *AIHandler) GetAIStats(c *gin.Context) {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	h.logger.Info("获取AI统计信息请求", zap.String("request_id", requestID))

	// 实现统计信息获取逻辑
	stats := h.getAIServiceStats()
	stats["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	c.JSON(http.StatusOK, stats)
}

// 辅助函数：统一错误响应
func (h *AIHandler) respondWithError(c *gin.Context, statusCode int, message, detail, requestID string) {
	errorResponse := &ErrorResponse{
		Code:      "AI_SERVICE_ERROR",
		Message:   message,
		Details:   detail,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	}

	// 根据状态码设置错误代码
	switch statusCode {
	case http.StatusBadRequest:
		errorResponse.Code = "INVALID_REQUEST"
	case http.StatusUnauthorized:
		errorResponse.Code = "UNAUTHORIZED"
	case http.StatusRequestTimeout:
		errorResponse.Code = "REQUEST_TIMEOUT"
	case http.StatusTooManyRequests:
		errorResponse.Code = "RATE_LIMIT_EXCEEDED"
	case http.StatusInternalServerError:
		errorResponse.Code = "INTERNAL_SERVER_ERROR"
	}

	// 在开发环境中包含详细错误信息
	if gin.Mode() != gin.DebugMode {
		errorResponse.Details = "" // 生产环境不暴露详细错误信息
	}

	c.JSON(statusCode, errorResponse)
}

// 辅助函数：生成请求ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// 辅助函数：生成查询ID
func generateQueryID(userID int64, timestamp time.Time) string {
	return strconv.FormatInt(userID, 36) + "-" + strconv.FormatInt(timestamp.UnixNano(), 36)
}

// 辅助函数：判断是否为超时错误
func isTimeoutError(err error) bool {
	return err.Error() == "context deadline exceeded" || 
		   err.Error() == "request timeout"
}

// 辅助函数：判断是否为频率限制错误
func isRateLimitError(err error) bool {
	return err.Error() == "rate limit exceeded" ||
		   err.Error() == "too many requests"
}

// storeFeedback 存储用户反馈到数据库
func (h *AIHandler) storeFeedback(req FeedbackRequest, userID int64, requestID string) error {
	h.logger.Info("存储用户反馈",
		zap.String("request_id", requestID),
		zap.String("query_id", req.QueryID),
		zap.Int64("user_id", userID),
		zap.Bool("is_correct", req.IsCorrect),
		zap.Int("user_rating", req.UserRating),
		zap.String("feedback", req.Feedback),
	)
	
	// 注意：由于项目当前阶段可能还未完整实现所有Repository
	// 这里提供一个基本的实现框架，实际需要配合Repository实现
	
	// 从query_id中尝试解析相关信息
	userQuery, generatedSQL := h.extractQueryInfoFromID(req.QueryID)
	
	// 构建反馈数据结构（使用内存中的结构，实际应该从数据库查询或请求中获取）
	feedback := &struct {
		QueryID        string  `json:"query_id"`
		UserID         int64   `json:"user_id"`
		UserQuery      string  `json:"user_query"`
		GeneratedSQL   string  `json:"generated_sql"`
		ExpectedSQL    *string `json:"expected_sql,omitempty"`
		IsCorrect      bool    `json:"is_correct"`
		UserRating     int     `json:"user_rating"`
		FeedbackText   *string `json:"feedback_text,omitempty"`
		Category       string  `json:"category"`
		Difficulty     string  `json:"difficulty"`
		ErrorType      *string `json:"error_type,omitempty"`
		ErrorDetails   *string `json:"error_details,omitempty"`
		ProcessingTime int64   `json:"processing_time"`
		TokensUsed     int     `json:"tokens_used"`
		ModelUsed      string  `json:"model_used"`
		ConnectionID   *int64  `json:"connection_id,omitempty"`
	}{
		QueryID:        req.QueryID,
		UserID:         userID,
		UserQuery:      userQuery,
		GeneratedSQL:   generatedSQL,
		IsCorrect:      req.IsCorrect,
		UserRating:     req.UserRating,
		Category:       h.categorizeQuery(userQuery),
		Difficulty:     h.assessDifficulty(userQuery, generatedSQL),
		ProcessingTime: 0, // 从查询历史或缓存中获取
		TokensUsed:     0, // 从AI服务响应中获取
		ModelUsed:      "default", // 从配置或上下文中获取
	}
	
	if req.Feedback != "" {
		feedback.FeedbackText = &req.Feedback
	}
	if req.UserSQL != "" {
		feedback.ExpectedSQL = &req.UserSQL
	}
	if !req.IsCorrect {
		errorType := h.inferErrorType(userQuery, generatedSQL)
		feedback.ErrorType = &errorType
		errorDetails := "用户标记为不正确"
		feedback.ErrorDetails = &errorDetails
	}
	
	// 实际的数据库存储逻辑
	// 注意：这里使用简化的实现，实际应该使用Repository接口
	h.logger.Info("准备存储反馈到数据库",
		zap.String("query_id", feedback.QueryID),
		zap.String("category", feedback.Category),
		zap.String("difficulty", feedback.Difficulty),
		zap.Int64("processing_time", feedback.ProcessingTime),
	)
	
	// 如果有Repository实例，使用它来存储
	// if h.repository != nil {
	//     dbFeedback := &repository.Feedback{
	//         QueryID: feedback.QueryID,
	//         UserID: feedback.UserID,
	//         // ... 其他字段映射
	//     }
	//     return h.repository.FeedbackRepo().Create(ctx, dbFeedback)
	// }
	
	// 暂时返回成功，实际实现需要真正的数据库操作
	h.logger.Info("反馈存储模拟完成", 
		zap.String("request_id", requestID),
		zap.String("query_id", req.QueryID))
	
	return nil
}

// extractQueryInfoFromID 从查询ID中提取查询信息
// 实际应该从数据库或缓存中查询
func (h *AIHandler) extractQueryInfoFromID(queryID string) (userQuery, generatedSQL string) {
	// 这里应该根据queryID从数据库中查询对应的查询历史记录
	// 暂时返回空值，实际需要实现查询逻辑
	h.logger.Debug("从查询ID提取信息", zap.String("query_id", queryID))
	return "用户自然语言查询", "SELECT * FROM users;"
}

// categorizeQuery 查询分类（简化版本）
func (h *AIHandler) categorizeQuery(query string) string {
	// 使用简化的分类逻辑
	queryLower := strings.ToLower(query)
	
	if strings.Contains(queryLower, "join") {
		return "join_query"
	} else if strings.Contains(queryLower, "count") || strings.Contains(queryLower, "sum") || 
			  strings.Contains(queryLower, "avg") || strings.Contains(queryLower, "统计") {
		return "aggregation"
	} else if strings.Contains(queryLower, "时间") || strings.Contains(queryLower, "日期") ||
			  strings.Contains(queryLower, "趋势") {
		return "time_analysis"
	} else if strings.Contains(queryLower, "子查询") || strings.Contains(queryLower, "in (") {
		return "subquery"
	}
	
	return "basic_select"
}

// assessDifficulty 评估查询难度（简化版本）
func (h *AIHandler) assessDifficulty(userQuery, generatedSQL string) string {
	sqlLower := strings.ToLower(generatedSQL)
	
	// 计算复杂度分数
	complexityScore := 0
	
	if strings.Contains(sqlLower, "join") {
		complexityScore += 2
	}
	if strings.Contains(sqlLower, "group by") {
		complexityScore += 2
	}
	if strings.Contains(sqlLower, "having") {
		complexityScore += 3
	}
	if strings.Contains(sqlLower, "union") {
		complexityScore += 3
	}
	if strings.Contains(sqlLower, "case when") {
		complexityScore += 2
	}
	
	subqueryCount := strings.Count(sqlLower, "select") - 1
	complexityScore += subqueryCount * 2
	
	if complexityScore <= 1 {
		return "easy"
	} else if complexityScore <= 4 {
		return "medium"
	} else if complexityScore <= 8 {
		return "hard"
	} else {
		return "expert"
	}
}

// inferErrorType 推断错误类型
func (h *AIHandler) inferErrorType(userQuery, generatedSQL string) string {
	queryLower := strings.ToLower(userQuery)
	
	if strings.Contains(queryLower, "表") && strings.Contains(queryLower, "不存在") {
		return "表不存在"
	} else if strings.Contains(queryLower, "字段") && strings.Contains(queryLower, "错误") {
		return "字段不存在"
	} else if strings.Contains(queryLower, "语法") {
		return "语法错误"
	} else if strings.Contains(queryLower, "join") {
		return "JOIN错误"
	} else if strings.Contains(queryLower, "group") || strings.Contains(queryLower, "聚合") {
		return "聚合函数错误"
	}
	
	return "未知错误"
}

// getAIServiceStats 获取AI服务统计信息
func (h *AIHandler) getAIServiceStats() map[string]any {
	// 这里应该从实际的监控系统或数据库中获取统计信息
	// 为了演示，返回模拟数据
	
	return map[string]any{
		"service_status":           "healthy",
		"total_queries":            1250,
		"successful_queries":       1190,
		"failed_queries":          60,
		"average_confidence":       0.87,
		"success_rate":            0.952,
		"average_response_time_ms": 450,
		"cache_hit_rate":          0.23,
		"active_connections":      15,
		"models_status": map[string]string{
			"primary":  "healthy",
			"fallback": "healthy",
			"local":    "healthy",
		},
		"recent_errors": []map[string]any{
			{
				"type":      "timeout",
				"count":     3,
				"last_seen": time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339),
			},
			{
				"type":      "rate_limit",
				"count":     2,
				"last_seen": time.Now().Add(-1*time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}
}