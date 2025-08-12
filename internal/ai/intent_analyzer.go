// Package ai 意图分析器和智能路由
package ai

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// QueryIntent 查询意图类型
type QueryIntent int

const (
	IntentUnknown QueryIntent = iota
	IntentDataQuery           // 基础数据查询
	IntentAggregation        // 聚合统计查询
	IntentJoinQuery          // 关联查询  
	IntentTimeSeriesAnalysis // 时间序列分析
	IntentComparison         // 对比分析
	IntentRanking           // 排序排名
	IntentFiltering         // 条件筛选
	IntentGrouping          // 分组查询
)

// IntentAnalyzer 意图分析器
type IntentAnalyzer struct {
	// 意图模式匹配
	patterns map[QueryIntent][]IntentPattern
	
	// 关键词权重
	keywordWeights map[string]float64
	
	// 配置参数
	config *IntentConfig
	
	// 历史分析缓存
	analysisCache map[string]*IntentResult
	
	// 用户行为学习
	userPatterns map[int64]*UserIntentProfile
}

// IntentPattern 意图模式
type IntentPattern struct {
	Keywords    []string  `json:"keywords"`    // 关键词列表
	Phrases     []string  `json:"phrases"`     // 短语模式
	Regex       string    `json:"regex"`       // 正则表达式
	Weight      float64   `json:"weight"`      // 权重
	Context     []string  `json:"context"`     // 上下文要求
	AntiPatterns []string `json:"anti_patterns"` // 反模式（存在时降低匹配度）
}

// IntentResult 意图分析结果
type IntentResult struct {
	PrimaryIntent   QueryIntent              `json:"primary_intent"`   // 主要意图
	SecondaryIntents map[QueryIntent]float64 `json:"secondary_intents"` // 次要意图和置信度
	Confidence      float64                  `json:"confidence"`       // 总体置信度
	Keywords        []string                 `json:"keywords"`         // 识别的关键词
	Entities        map[string][]string      `json:"entities"`         // 实体提取
	QueryFeatures   QueryFeatures            `json:"query_features"`   // 查询特征
	Suggestions     []string                 `json:"suggestions"`      // 优化建议
	ProcessingTime  time.Duration            `json:"processing_time"`  // 处理时间
}

// QueryFeatures 查询特征
type QueryFeatures struct {
	HasTimeReference  bool     `json:"has_time_reference"`  // 包含时间引用
	HasComparison     bool     `json:"has_comparison"`      // 包含比较
	HasAggregation    bool     `json:"has_aggregation"`     // 包含聚合
	HasGrouping       bool     `json:"has_grouping"`        // 包含分组
	HasSorting        bool     `json:"has_sorting"`         // 包含排序
	HasFiltering      bool     `json:"has_filtering"`       // 包含过滤
	HasJoin           bool     `json:"has_join"`            // 需要关联查询
	EstimatedTables   []string `json:"estimated_tables"`    // 预估涉及的表
	QueryComplexity   string   `json:"query_complexity"`    // 查询复杂度
	RequiredFunctions []string `json:"required_functions"`  // 需要的SQL函数
}

// UserIntentProfile 用户意图档案
type UserIntentProfile struct {
	UserID          int64                    `json:"user_id"`
	IntentHistory   []QueryIntent           `json:"intent_history"`
	PreferredIntents map[QueryIntent]float64 `json:"preferred_intents"`
	CommonKeywords  map[string]int           `json:"common_keywords"`
	LastUpdated     time.Time               `json:"last_updated"`
	QueryCount      int                      `json:"query_count"`
}

// IntentConfig 意图分析器配置
type IntentConfig struct {
	// 缓存配置
	EnableCache     bool          `yaml:"enable_cache"`
	CacheSize       int           `yaml:"cache_size"`
	CacheTTL        time.Duration `yaml:"cache_ttl"`
	
	// 分析配置
	MinConfidence   float64 `yaml:"min_confidence"`   // 最小置信度阈值
	MaxAlternatives int     `yaml:"max_alternatives"` // 最大候选意图数
	
	// 用户学习
	EnableUserLearning bool `yaml:"enable_user_learning"`
	UserProfileSize    int  `yaml:"user_profile_size"`
	
	// 实体识别
	EnableEntityExtraction bool     `yaml:"enable_entity_extraction"`
	CustomEntities        []string `yaml:"custom_entities"`
}

// NewIntentAnalyzer 创建新的意图分析器
func NewIntentAnalyzer() *IntentAnalyzer {
	config := &IntentConfig{
		EnableCache:            true,
		CacheSize:              1000,
		CacheTTL:               time.Hour,
		MinConfidence:          0.6,
		MaxAlternatives:        3,
		EnableUserLearning:     true,
		UserProfileSize:        100,
		EnableEntityExtraction: true,
	}
	
	ia := &IntentAnalyzer{
		patterns:      make(map[QueryIntent][]IntentPattern),
		keywordWeights: make(map[string]float64),
		config:        config,
		analysisCache: make(map[string]*IntentResult),
		userPatterns:  make(map[int64]*UserIntentProfile),
	}
	
	// 初始化意图模式
	ia.initializePatterns()
	ia.initializeKeywordWeights()
	
	return ia
}

// initializePatterns 初始化意图模式
func (ia *IntentAnalyzer) initializePatterns() {
	// 基础数据查询 (增强英文支持)
	ia.patterns[IntentDataQuery] = []IntentPattern{
		{
			Keywords: []string{"查询", "显示", "列出", "查看", "获取", "找到", "搜索"},
			Phrases:  []string{"查看信息", "显示数据", "列出所有"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"what", "show", "list", "get", "find", "display", "select", "retrieve", "fetch", "extract"},
			Phrases:  []string{"show me", "give me", "i want", "i need", "let me see", "can you show"},
			Weight:   0.9,
		},
		{
			Keywords: []string{"view", "see", "look", "check", "examine", "explore", "browse", "search for"},
			Phrases:  []string{"look at", "take a look", "have a look", "check out", "browse through"},
			Weight:   0.85,
		},
		{
			Regex: `(?i)\b(what|which|how\s+many|tell\s+me)\b.*\b(is|are|was|were)\b`,
			Weight: 1.1,
		},
		{
			Regex: `(?i)\b(please\s+)?(show|display|list|get|find|retrieve)\b`,
			Weight: 1.0,
		},
	}
	
	// 聚合统计查询 (增强英文支持)
	ia.patterns[IntentAggregation] = []IntentPattern{
		{
			Keywords: []string{"总数", "数量", "统计", "计算", "总和", "平均", "最大", "最小"},
			Phrases:  []string{"有多少", "总共有", "平均值", "最大值", "最小值"},
			Weight:   1.2,
		},
		{
			Keywords: []string{"count", "sum", "total", "average", "max", "min", "statistics", "aggregate", "calculate"},
			Phrases:  []string{"how many", "how much", "total number", "average of", "sum of", "maximum value", "minimum value"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"mean", "median", "mode", "std", "variance", "percentile", "distribution", "summary"},
			Phrases:  []string{"statistical summary", "descriptive statistics", "data summary", "aggregate data"},
			Weight:   1.0,
		},
		{
			Regex: `(?i)\b(多少|几个|数量|总计|统计)\b`,
			Weight: 1.0,
		},
		{
			Regex: `(?i)\b(how\s+many|how\s+much|total\s+number|count\s+of|sum\s+of|average\s+of)\b`,
			Weight: 1.3,
		},
		{
			Regex: `(?i)\b(what\s+is\s+the\s+(total|sum|average|mean|max|min|count))\b`,
			Weight: 1.2,
		},
	}
	
	// 关联查询 (增强英文支持)
	ia.patterns[IntentJoinQuery] = []IntentPattern{
		{
			Keywords: []string{"关联", "连接", "联合", "匹配", "对应", "相关"},
			Phrases:  []string{"和...相关", "对应的", "关联的", "匹配的"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"join", "relate", "connect", "match", "associated", "corresponding", "linked", "combined", "with", "their"},
			Phrases:  []string{"related to", "associated with", "connected to", "linked with", "combined with", "along with", "with their"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"merge", "union", "intersect", "overlap", "cross-reference", "correlate"},
			Phrases:  []string{"cross reference", "bring together", "merge data", "combine information"},
			Weight:   0.9,
		},
		{
			Regex: `(?i)\b(的|和|与).*(信息|数据|记录)\b`,
			Weight: 0.8,
		},
		{
			Regex: `(?i)\b(with|and|together\s+with|along\s+with|including)\b.*\b(data|information|records)\b`,
			Weight: 0.9,
		},
		{
			Regex: `(?i)\b(show\s+me\s+.+\s+(and|with|plus)\s+.+)\b`,
			Weight: 1.0,
		},
	}
	
	// 时间序列分析 (增强英文支持)
	ia.patterns[IntentTimeSeriesAnalysis] = []IntentPattern{
		{
			Keywords: []string{"趋势", "变化", "增长", "下降", "历史", "时间", "期间", "阶段"},
			Phrases:  []string{"随时间变化", "历史趋势", "增长趋势", "变化情况"},
			Weight:   1.2,
		},
		{
			Keywords: []string{"trend", "change", "growth", "decline", "history", "time", "period", "over time", "temporal"},
			Phrases:  []string{"over time", "time series", "historical data", "trend analysis", "temporal pattern"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"年", "月", "日", "周", "季度", "每天", "每月", "每年", "最近", "过去"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"daily", "weekly", "monthly", "yearly", "quarterly", "hourly", "recent", "past", "historical"},
			Phrases:  []string{"in the past", "last few", "previous", "recent months", "over the years", "during the period"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"evolution", "progression", "development", "fluctuation", "variation", "seasonality"},
			Phrases:  []string{"how it changed", "evolution of", "development over", "seasonal pattern", "time-based analysis"},
			Weight:   0.9,
		},
		{
			Regex: `(?i)\b(最近|过去|前)\s*\d+\s*(天|周|月|年|小时)\b`,
			Weight: 1.3,
		},
		{
			Regex: `(?i)\b(last|past|previous|recent)\s+\d+\s+(days?|weeks?|months?|years?|hours?)\b`,
			Weight: 1.3,
		},
		{
			Regex: `(?i)\b(between|from)\s+\d{4}\s+(and|to)\s+\d{4}\b`,
			Weight: 1.2,
		},
		{
			Regex: `(?i)\b(since|until|before|after)\s+\d{4}\b`,
			Weight: 1.1,
		},
	}
	
	// 对比分析 (增强英文支持)
	ia.patterns[IntentComparison] = []IntentPattern{
		{
			Keywords: []string{"比较", "对比", "差异", "区别", "相比", "vs", "对照"},
			Phrases:  []string{"相比之下", "与...相比", "两者之间", "差异在于"},
			Weight:   1.2,
		},
		{
			Keywords: []string{"compare", "versus", "difference", "between", "against", "contrast", "vs"},
			Phrases:  []string{"compared to", "in comparison", "as opposed to", "relative to", "in contrast to"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"更高", "更低", "更多", "更少", "大于", "小于", "超过", "低于"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"higher", "lower", "greater", "less", "more", "fewer", "above", "below", "exceed"},
			Phrases:  []string{"greater than", "less than", "more than", "fewer than", "better than", "worse than"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"superior", "inferior", "outperform", "underperform", "gap", "differential"},
			Phrases:  []string{"performance gap", "side by side", "head to head", "benchmark against"},
			Weight:   0.9,
		},
		{
			Regex: `(?i)\b(A\s+(vs|versus|compared\s+to|against)\s+B)\b`,
			Weight: 1.3,
		},
		{
			Regex: `(?i)\b(which\s+is\s+(better|worse|higher|lower|more|less))\b`,
			Weight: 1.2,
		},
	}
	
	// 排序排名 (增强英文支持)
	ia.patterns[IntentRanking] = []IntentPattern{
		{
			Keywords: []string{"排序", "排名", "排行", "前", "后", "最", "第一", "最后"},
			Phrases:  []string{"按...排序", "排名前", "最高的", "最低的", "前10名"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"top", "bottom", "rank", "order", "sort", "first", "last", "highest", "lowest"},
			Phrases:  []string{"top 10", "bottom 5", "rank by", "order by", "sort by", "best performing", "worst performing"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"leading", "trailing", "best", "worst", "prime", "elite", "inferior", "superior"},
			Phrases:  []string{"in descending order", "in ascending order", "ranked by", "sorted by", "arranged by"},
			Weight:   0.9,
		},
		{
			Regex: `(?i)\b(前|后|top|bottom)\s*\d+\b`,
			Weight: 1.2,
		},
		{
			Regex: `(?i)\b(top|bottom|first|last)\s+\d+\b`,
			Weight: 1.2,
		},
		{
			Regex: `(?i)\b(sort|order|rank)\s+(by|according\s+to)\b`,
			Weight: 1.1,
		},
	}
	
	// 条件筛选 (增强英文支持)
	ia.patterns[IntentFiltering] = []IntentPattern{
		{
			Keywords: []string{"筛选", "过滤", "条件", "满足", "符合", "包含", "不包含"},
			Phrases:  []string{"满足条件", "符合要求", "包含关键词", "筛选条件"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"where", "filter", "condition", "contains", "includes", "excludes", "criteria"},
			Phrases:  []string{"based on", "according to", "where the", "that meet", "with the condition"},
			Weight:   0.9,
		},
		{
			Keywords: []string{"等于", "大于", "小于", "不等于", "在...之间", "不在"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"equals", "greater", "less", "not equal", "between", "within", "outside", "matching"},
			Phrases:  []string{"equal to", "greater than", "less than", "not equal to", "in between", "within range"},
			Weight:   0.8,
		},
		{
			Keywords: []string{"subset", "superset", "distinct", "unique", "duplicate", "exclude", "omit"},
			Phrases:  []string{"that satisfy", "meeting criteria", "with properties", "having attributes"},
			Weight:   0.7,
		},
		{
			Regex: `(?i)\b(where|with|having|that\s+(are|is|have|has))\b`,
			Weight: 1.0,
		},
		{
			Regex: `(?i)\b(only\s+those|just\s+the|specifically|particularly)\b`,
			Weight: 0.9,
		},
	}
	
	// 分组查询 (增强英文支持)
	ia.patterns[IntentGrouping] = []IntentPattern{
		{
			Keywords: []string{"分组", "按", "根据", "分类", "归类", "每个", "各个"},
			Phrases:  []string{"按...分组", "根据...分类", "每个...的", "各个...的"},
			Weight:   1.1,
		},
		{
			Keywords: []string{"group", "by", "each", "per", "category", "classify", "segment", "partition"},
			Phrases:  []string{"group by", "grouped by", "categorized by", "segmented by", "broken down by"},
			Weight:   1.0,
		},
		{
			Keywords: []string{"cluster", "organize", "aggregate", "consolidate", "bucket", "bin"},
			Phrases:  []string{"for each", "per category", "by type", "according to", "organized by"},
			Weight:   0.9,
		},
		{
			Regex: `(?i)\b(按|根据|每个|各个|group\s+by)\b`,
			Weight: 1.2,
		},
		{
			Regex: `(?i)\b(group\s+by|grouped\s+by|categorize\s+by|segment\s+by)\b`,
			Weight: 1.2,
		},
		{
			Regex: `(?i)\b(for\s+each|per\s+\w+|by\s+\w+\s+type)\b`,
			Weight: 1.1,
		},
	}
}

// initializeKeywordWeights 初始化关键词权重 (增强英文支持)
func (ia *IntentAnalyzer) initializeKeywordWeights() {
	// 时间相关关键词
	timeKeywords := map[string]float64{
		"年": 0.8, "月": 0.8, "日": 0.7, "周": 0.7, "天": 0.7,
		"最近": 1.0, "过去": 1.0, "历史": 0.9, "趋势": 1.2,
		"year": 0.8, "month": 0.8, "day": 0.7, "week": 0.7,
		"recent": 1.0, "past": 1.0, "history": 0.9, "trend": 1.2,
		"yesterday": 0.9, "today": 0.9, "tomorrow": 0.9,
		"quarterly": 0.8, "annually": 0.8, "seasonal": 0.9,
		"temporal": 1.0, "chronological": 0.9, "timeline": 1.0,
	}
	
	// 聚合相关关键词
	aggKeywords := map[string]float64{
		"总数": 1.2, "数量": 1.1, "统计": 1.1, "计算": 1.0,
		"平均": 1.1, "最大": 1.0, "最小": 1.0, "总和": 1.1,
		"count": 1.2, "sum": 1.1, "avg": 1.1, "max": 1.0, "min": 1.0,
		"total": 1.2, "average": 1.1, "mean": 1.1, "median": 1.0,
		"aggregate": 1.2, "calculate": 1.0, "compute": 1.0,
		"statistics": 1.1, "summary": 1.0, "distribution": 0.9,
	}
	
	// 比较分析关键词
	comparisonKeywords := map[string]float64{
		"比较": 1.2, "对比": 1.2, "vs": 1.3, "versus": 1.3,
		"compare": 1.2, "contrast": 1.1, "difference": 1.1,
		"better": 1.0, "worse": 1.0, "superior": 0.9, "inferior": 0.9,
		"outperform": 1.0, "exceed": 1.0, "surpass": 0.9,
	}
	
	// 排序排名关键词
	rankingKeywords := map[string]float64{
		"排序": 1.1, "排名": 1.2, "top": 1.3, "bottom": 1.3,
		"rank": 1.2, "order": 1.1, "sort": 1.1, "best": 1.2,
		"worst": 1.2, "highest": 1.1, "lowest": 1.1,
		"leading": 1.0, "trailing": 1.0, "first": 1.1, "last": 1.1,
	}
	
	// 关联查询关键词
	joinKeywords := map[string]float64{
		"关联": 1.1, "连接": 1.1, "join": 1.2, "relate": 1.1,
		"connect": 1.0, "link": 1.0, "associate": 1.0,
		"combine": 1.0, "merge": 1.0, "correlate": 0.9,
	}
	
	// 分组查询关键词
	groupingKeywords := map[string]float64{
		"分组": 1.2, "group": 1.2, "category": 1.1, "classify": 1.1,
		"segment": 1.0, "partition": 1.0, "cluster": 0.9,
		"organize": 0.9, "consolidate": 0.9,
	}
	
	// 合并所有关键词权重
	allKeywords := []map[string]float64{
		timeKeywords, aggKeywords, comparisonKeywords,
		rankingKeywords, joinKeywords, groupingKeywords,
	}
	
	for _, keywords := range allKeywords {
		for k, v := range keywords {
			ia.keywordWeights[k] = v
		}
	}
}

// AnalyzeIntent 分析查询意图
func (ia *IntentAnalyzer) AnalyzeIntent(query string) QueryIntent {
	result := ia.AnalyzeIntentDetailed(query, 0)
	return result.PrimaryIntent
}

// AnalyzeIntentDetailed 详细分析查询意图
func (ia *IntentAnalyzer) AnalyzeIntentDetailed(query string, userID int64) *IntentResult {
	start := time.Now()
	
	// 检查缓存
	if ia.config.EnableCache {
		if cached, exists := ia.analysisCache[query]; exists {
			return cached
		}
	}
	
	// 文本预处理
	normalizedQuery := ia.normalizeQuery(query)
	
	// 特征提取
	features := ia.extractQueryFeatures(normalizedQuery)
	
	// 意图匹配
	intentScores := ia.calculateIntentScores(normalizedQuery, features)
	
	// 用户学习调整
	if ia.config.EnableUserLearning && userID > 0 {
		ia.adjustScoresWithUserProfile(intentScores, userID)
	}
	
	// 构建结果
	result := ia.buildIntentResult(intentScores, features, normalizedQuery)
	result.ProcessingTime = time.Since(start)
	
	// 更新用户档案
	if ia.config.EnableUserLearning && userID > 0 {
		ia.updateUserProfile(userID, result.PrimaryIntent, normalizedQuery)
	}
	
	// 缓存结果
	if ia.config.EnableCache {
		ia.cacheResult(query, result)
	}
	
	return result
}

// normalizeQuery 标准化查询文本
func (ia *IntentAnalyzer) normalizeQuery(query string) string {
	// 转小写
	normalized := strings.ToLower(query)
	
	// 移除多余空格
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	// 移除标点符号（保留一些有意义的），支持中文字符
	// 保留中文字符(\p{Han})、ASCII字母数字(\w)、空格、星号和连字符
	normalized = regexp.MustCompile(`[^\p{Han}\p{Latin}\w\s\*\-]`).ReplaceAllString(normalized, " ")
	
	// 再次移除多余空格（标点符号替换可能产生多余空格）
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	// 去除首尾空格
	normalized = strings.TrimSpace(normalized)
	
	return normalized
}

// extractQueryFeatures 提取查询特征
func (ia *IntentAnalyzer) extractQueryFeatures(query string) QueryFeatures {
	features := QueryFeatures{
		EstimatedTables:   []string{},
		RequiredFunctions: []string{},
	}
	
	// 时间引用检测 (增强中文和英文支持)
	timePatterns := []string{
		`\b\d{4}年`, `\b\d{1,2}月`, `\b\d{1,2}日`,
		`最近\d+`, `过去\d+`, `前\d+`, `last\s+\d+`, `past\s+\d+`,
		`今天`, `昨天`, `明天`, `今年`, `去年`, `明年`, `本年`, `上年`, `下年`,
		`今日`, `昨日`, `明日`, `本月`, `上月`, `下月`,
		`today`, `yesterday`, `tomorrow`, `this\s+year`, `last\s+year`, `next\s+year`,
		`(?i)\b(in|during|within|over)\s+the\s+(last|past|previous)\s+\d+\s+(days?|weeks?|months?|years?)\b`,
		`(?i)\b(from|since|until|before|after)\s+\d{4}\b`,
		`(?i)\b(this|next|last)\s+(year|month|week|quarter)\b`,
		`(?i)\b\d{4}-\d{2}-\d{2}\b`, // ISO date format
		`(?i)\b\d{1,2}/\d{1,2}/\d{4}\b`, // US date format
	}
	for _, pattern := range timePatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasTimeReference = true
			break
		}
	}
	
	// 比较检测 (增强英文支持)
	comparisonKeywords := []string{
		"比较", "对比", "相比", "compare", "versus", "vs", "大于", "小于", "更多", "更少",
		"greater", "less", "higher", "lower", "better", "worse", "exceed", "below",
		"superior", "inferior", "outperform", "underperform", "against", "contrast",
	}
	comparisonPatterns := []string{
		`(?i)\b(A\s+(vs|versus|compared\s+to|against)\s+B)\b`,
		`(?i)\b(which\s+is\s+(better|worse|higher|lower|more|less))\b`,
		`(?i)\b(more\s+than|less\s+than|greater\s+than|smaller\s+than)\b`,
		`(?i)\b(in\s+comparison\s+to|as\s+opposed\s+to|relative\s+to)\b`,
	}
	
	for _, keyword := range comparisonKeywords {
		if strings.Contains(query, keyword) {
			features.HasComparison = true
			break
		}
	}
	
	if !features.HasComparison {
		for _, pattern := range comparisonPatterns {
			if matched, _ := regexp.MatchString(pattern, query); matched {
				features.HasComparison = true
				break
			}
		}
	}
	
	// 聚合检测 (增强英文支持)
	aggregationKeywords := []string{
		"总数", "数量", "统计", "平均", "最大", "最小", "总和",
		"count", "sum", "avg", "max", "min", "total", "average", "mean",
		"aggregate", "calculate", "compute", "statistics", "summary",
		"median", "mode", "std", "variance", "percentile", "distribution",
	}
	aggregationPatterns := []string{
		`(?i)\b(how\s+many|how\s+much|total\s+number)\b`,
		`(?i)\b(what\s+is\s+the\s+(total|sum|average|mean|max|min|count))\b`,
		`(?i)\b(calculate\s+the|compute\s+the|find\s+the\s+(total|sum|average))\b`,
	}
	
	for _, keyword := range aggregationKeywords {
		if strings.Contains(query, keyword) {
			features.HasAggregation = true
			features.RequiredFunctions = append(features.RequiredFunctions, strings.ToUpper(keyword))
		}
	}
	
	for _, pattern := range aggregationPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasAggregation = true
			break
		}
	}
	
	// 分组检测 (增强英文支持)
	groupingKeywords := []string{
		"分组", "按", "根据", "每个", "各个", "group by", "each", "per",
		"category", "classify", "segment", "partition", "organize", "aggregate",
	}
	groupingPatterns := []string{
		`(?i)\b(group\s+by|grouped\s+by|categorize\s+by|segment\s+by)\b`,
		`(?i)\b(for\s+each|per\s+\w+|by\s+\w+\s+type)\b`,
		`(?i)\b(break\s+down\s+by|broken\s+down\s+by|organized\s+by)\b`,
	}
	
	for _, keyword := range groupingKeywords {
		if strings.Contains(query, keyword) {
			features.HasGrouping = true
			break
		}
	}
	
	for _, pattern := range groupingPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasGrouping = true
			break
		}
	}
	
	// 排序检测 (增强英文支持)
	sortingKeywords := []string{
		"排序", "排名", "排行", "order by", "sort", "rank", "top", "前", "后",
		"best", "worst", "highest", "lowest", "first", "last", "leading", "trailing",
	}
	sortingPatterns := []string{
		`(?i)\b(sort\s+by|order\s+by|rank\s+by|ranked\s+by)\b`,
		`(?i)\b(top\s+\d+|bottom\s+\d+|first\s+\d+|last\s+\d+)\b`,
		`(?i)\b(in\s+(ascending|descending)\s+order)\b`,
	}
	
	for _, keyword := range sortingKeywords {
		if strings.Contains(query, keyword) {
			features.HasSorting = true
			break
		}
	}
	
	for _, pattern := range sortingPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasSorting = true
			break
		}
	}
	
	// 过滤检测 (增强中文和英文支持)
	filteringKeywords := []string{
		"筛选", "过滤", "条件", "where", "满足", "符合", "包含",
		"状态", "类型", "属性", "特定", "指定", "某个", "某些",
		"活跃", "启用", "禁用", "有效", "无效", "正常", "异常",
		"filter", "condition", "criteria", "restrict", "limit", "constrain",
		"exclude", "include", "only", "specifically", "particularly",
		"active", "inactive", "enabled", "disabled", "status", "type", "property",
	}
	filteringPatterns := []string{
		`(?i)\b(where|with|having|that\s+(are|is|have|has))\b`,
		`(?i)\b(only\s+those|just\s+the|specifically|particularly)\b`,
		`(?i)\b(meeting\s+criteria|satisfying\s+condition|with\s+properties)\b`,
		`\b[\u4e00-\u9fa5]+状态的`, // 中文状态模式，如"活跃状态的"
		`\b[\u4e00-\u9fa5]+类型的`, // 中文类型模式
	}
	
	for _, keyword := range filteringKeywords {
		if strings.Contains(query, keyword) {
			features.HasFiltering = true
			break
		}
	}
	
	for _, pattern := range filteringPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasFiltering = true
			break
		}
	}
	
	// JOIN检测 (增强中文和英文支持)
	joinKeywords := []string{
		"关联", "连接", "联合", "join", "相关", "对应", "匹配",
		"relate", "connect", "link", "associate", "combine", "merge",
		"correlate", "cross-reference", "together", "along with",
		"with", "corresponding", "their", "respective", "related",
		"paired", "matched", "linked", "associated", "connected",
	}
	joinPatterns := []string{
		`(?i)\b(with|and|together\s+with|along\s+with|including)\b.*\b(data|information|records)\b`,
		`(?i)\b(show\s+me\s+.+\s+(and|with|plus)\s+.+)\b`,
		`(?i)\b(related\s+to|associated\s+with|connected\s+to|linked\s+with)\b`,
		`(?i)\b(show|display|list|get)\s+\w+\s+with\s+(their|its|corresponding|related|associated)\b`,
		`(?i)\b(users?\s+with\s+their|customers?\s+with\s+their|orders?\s+with\s+their)\b`,
		`(?i)\b\w+\s+(and|with)\s+(their|its|corresponding|related|matching)\s+\w+\b`,
		`用户.*订单`, `订单.*用户`, // 中文用户-订单关联模式
		`客户.*订单`, `订单.*客户`, // 中文客户-订单关联模式
		`每个用户的`, `每个客户的`, // "每个用户的订单"类似模式
		`用户.*的.*订单`, `客户.*的.*订单`, // 更灵活的中文关联模式
	}
	
	for _, keyword := range joinKeywords {
		if strings.Contains(query, keyword) {
			features.HasJoin = true
			break
		}
	}
	
	for _, pattern := range joinPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			features.HasJoin = true
			break
		}
	}
	
	// 复杂度评估
	complexityScore := 0
	if features.HasAggregation { complexityScore += 2 }
	if features.HasJoin { complexityScore += 3 }
	if features.HasGrouping { complexityScore += 2 }
	if features.HasTimeReference { complexityScore += 1 }
	if features.HasComparison { complexityScore += 1 }
	
	switch {
	case complexityScore <= 2:
		features.QueryComplexity = "simple"
	case complexityScore <= 5:
		features.QueryComplexity = "medium"
	default:
		features.QueryComplexity = "complex"
	}
	
	return features
}

// calculateIntentScores 计算意图分数
func (ia *IntentAnalyzer) calculateIntentScores(query string, features QueryFeatures) map[QueryIntent]float64 {
	scores := make(map[QueryIntent]float64)
	
	// 遍历所有意图模式进行匹配
	for intent, patterns := range ia.patterns {
		var intentScore float64
		
		for _, pattern := range patterns {
			patternScore := ia.calculatePatternScore(query, pattern)
			intentScore += patternScore
		}
		
		// 根据查询特征调整分数
		intentScore = ia.adjustScoreByFeatures(intent, intentScore, features)
		
		if intentScore > 0 {
			scores[intent] = intentScore
		}
	}
	
	// 如果没有匹配到任何意图，默认为数据查询
	if len(scores) == 0 {
		scores[IntentDataQuery] = 0.5
	}
	
	return scores
}

// calculatePatternScore 计算模式匹配分数
func (ia *IntentAnalyzer) calculatePatternScore(query string, pattern IntentPattern) float64 {
	var score float64
	
	// 关键词匹配
	for _, keyword := range pattern.Keywords {
		if strings.Contains(query, keyword) {
			weight := ia.keywordWeights[keyword]
			if weight == 0 {
				weight = 1.0
			}
			score += weight * pattern.Weight
		}
	}
	
	// 短语匹配
	for _, phrase := range pattern.Phrases {
		if strings.Contains(query, phrase) {
			score += 1.5 * pattern.Weight
		}
	}
	
	// 正则表达式匹配
	if pattern.Regex != "" {
		if matched, _ := regexp.MatchString(pattern.Regex, query); matched {
			score += 2.0 * pattern.Weight
		}
	}
	
	// 反模式检查（降低分数）
	for _, antiPattern := range pattern.AntiPatterns {
		if strings.Contains(query, antiPattern) {
			score *= 0.5 // 减少一半分数
		}
	}
	
	return score
}

// adjustScoreByFeatures 根据查询特征调整分数
func (ia *IntentAnalyzer) adjustScoreByFeatures(intent QueryIntent, score float64, features QueryFeatures) float64 {
	switch intent {
	case IntentAggregation:
		if features.HasAggregation {
			score *= 1.5
		}
	case IntentTimeSeriesAnalysis:
		if features.HasTimeReference {
			score *= 1.5
		}
	case IntentJoinQuery:
		if features.HasJoin {
			score *= 1.5
		}
	case IntentComparison:
		if features.HasComparison {
			score *= 1.5
		}
	case IntentRanking:
		if features.HasSorting {
			score *= 1.3
		}
	case IntentFiltering:
		if features.HasFiltering {
			score *= 1.2
		}
	case IntentGrouping:
		if features.HasGrouping {
			score *= 1.3
		}
	}
	
	return score
}

// adjustScoresWithUserProfile 使用用户档案调整分数
func (ia *IntentAnalyzer) adjustScoresWithUserProfile(scores map[QueryIntent]float64, userID int64) {
	profile := ia.getUserProfile(userID)
	if profile == nil {
		return
	}
	
	// 根据用户偏好调整分数
	for intent, preference := range profile.PreferredIntents {
		if score, exists := scores[intent]; exists {
			scores[intent] = score * (1.0 + preference*0.2) // 最多增加20%
		}
	}
}

// buildIntentResult 构建意图分析结果
func (ia *IntentAnalyzer) buildIntentResult(scores map[QueryIntent]float64, features QueryFeatures, query string) *IntentResult {
	// 按分数排序
	type intentScore struct {
		intent QueryIntent
		score  float64
	}
	
	var sortedScores []intentScore
	for intent, score := range scores {
		sortedScores = append(sortedScores, intentScore{intent, score})
	}
	
	sort.Slice(sortedScores, func(i, j int) bool {
		return sortedScores[i].score > sortedScores[j].score
	})
	
	result := &IntentResult{
		SecondaryIntents: make(map[QueryIntent]float64),
		QueryFeatures:    features,
		Keywords:         ia.extractKeywords(query),
		Entities:         ia.extractEntities(query),
		Suggestions:      []string{},
	}
	
	if len(sortedScores) > 0 {
		result.PrimaryIntent = sortedScores[0].intent
		result.Confidence = ia.normalizeConfidence(sortedScores[0].score)
		
		// 添加次要意图
		maxAlternatives := ia.config.MaxAlternatives
		if maxAlternatives > len(sortedScores)-1 {
			maxAlternatives = len(sortedScores) - 1
		}
		
		for i := 1; i <= maxAlternatives; i++ {
			if sortedScores[i].score >= sortedScores[0].score*0.3 { // 至少是主要意图分数的30%（降低阈值）
				result.SecondaryIntents[sortedScores[i].intent] = ia.normalizeConfidence(sortedScores[i].score)
			}
		}
	} else {
		result.PrimaryIntent = IntentDataQuery
		result.Confidence = 0.5
	}
	
	// 生成优化建议
	result.Suggestions = ia.generateSuggestions(result)
	
	return result
}

// normalizeConfidence 标准化置信度到0-1范围
func (ia *IntentAnalyzer) normalizeConfidence(score float64) float64 {
	// 使用sigmoid函数将分数映射到0-1区间
	confidence := 1.0 / (1.0 + exp(-score*0.5))
	
	// 确保最小置信度
	if confidence < ia.config.MinConfidence {
		confidence = ia.config.MinConfidence
	}
	
	return confidence
}

// exp 简单的指数函数实现
func exp(x float64) float64 {
	if x > 10 {
		return 22026.465794806718 // e^10
	}
	if x < -10 {
		return 0.000045399929762484845 // e^-10
	}
	
	// 泰勒级数近似
	result := 1.0
	term := 1.0
	for i := 1; i < 20; i++ {
		term *= x / float64(i)
		result += term
	}
	return result
}

// extractKeywords 提取关键词
func (ia *IntentAnalyzer) extractKeywords(query string) []string {
	var keywords []string
	
	stopWords := map[string]bool{
		// 中文停止词
		"的": true, "是": true, "在": true, "有": true, "和": true,
		"了": true, "就": true, "都": true, "而": true, "及": true,
		"与": true, "或": true, "但": true, "然": true, "即": true,
		
		// 英文停止词 (移除部分以允许更多关键词)
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"and": true, "or": true, "but": true, "in": true, "on": true,
		"at": true, "by": true, "for": true, "with": true, "to": true,
		"of": true, "as": true, "be": true, "was": true, "were": true,
		"been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "must": true, "can": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
		"its": true, "they": true, "them": true, "their": true, "there": true,
		"when": true, "why": true, "how": true, "what": true,
		"which": true, "who": true, "whom": true, "whose": true, "if": true,
		"then": true, "else": true, "than": true, "so": true, "very": true,
		"also": true, "just": true, "only": true, "even": true, "still": true,
		"now": true, "here": true, "up": true, "out": true,
		"down": true, "over": true, "under": true, "between": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
	}
	
	// 首先按空格分割，处理英文和混合文本
	words := strings.Fields(query)
	
	for _, word := range words {
		cleaned := strings.Trim(word, ".,!?;:")
		if len(cleaned) == 0 {
			continue
		}
		
		// 检查是否包含中文字符
		if ia.containsChineseChar(cleaned) {
			// 对中文进行字符级分割（简单的方法）
			chineseWords := ia.extractChineseKeywords(cleaned)
			for _, cw := range chineseWords {
				if len(cw) > 0 && !stopWords[cw] {
					keywords = append(keywords, cw)
				}
			}
		} else {
			// 英文词汇直接处理
			if len(cleaned) > 1 && !stopWords[strings.ToLower(cleaned)] {
				keywords = append(keywords, cleaned)
			}
		}
	}
	
	return keywords
}

// containsChineseChar 检查字符串是否包含中文字符
func (ia *IntentAnalyzer) containsChineseChar(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// extractChineseKeywords 简单的中文词汇提取（基于常见词汇模式）
func (ia *IntentAnalyzer) extractChineseKeywords(s string) []string {
	var words []string
	runes := []rune(s)
	
	// 简单的双字符和单字符提取策略
	i := 0
	for i < len(runes) {
		if runes[i] >= 0x4e00 && runes[i] <= 0x9fff {
			// 中文字符
			if i+1 < len(runes) && runes[i+1] >= 0x4e00 && runes[i+1] <= 0x9fff {
				// 双字符词
				words = append(words, string(runes[i:i+2]))
				i += 2
			} else {
				// 单字符
				words = append(words, string(runes[i]))
				i++
			}
		} else {
			// 非中文字符，按字符添加
			words = append(words, string(runes[i]))
			i++
		}
	}
	
	return words
}

// extractEntities 提取实体
func (ia *IntentAnalyzer) extractEntities(query string) map[string][]string {
	entities := make(map[string][]string)
	
	if !ia.config.EnableEntityExtraction {
		return entities
	}
	
	// 时间实体提取 (修复中文边界问题)
	timeRegex := regexp.MustCompile(`\d{4}年|\d{1,2}月|\d{1,2}日|最近\d+天?`)
	timeMatches := timeRegex.FindAllString(query, -1)
	if len(timeMatches) > 0 {
		entities["time"] = timeMatches
	}
	
	// 数字实体提取
	numberRegex := regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	numberMatches := numberRegex.FindAllString(query, -1)
	if len(numberMatches) > 0 {
		entities["number"] = numberMatches
	}
	
	return entities
}

// generateSuggestions 生成优化建议
func (ia *IntentAnalyzer) generateSuggestions(result *IntentResult) []string {
	var suggestions []string
	
	// 根据意图类型生成建议
	switch result.PrimaryIntent {
	case IntentAggregation:
		if !result.QueryFeatures.HasGrouping {
			suggestions = append(suggestions, "考虑添加分组条件以获得更详细的统计结果")
		}
		if !result.QueryFeatures.HasFiltering {
			suggestions = append(suggestions, "可以添加筛选条件来限制统计范围")
		}
	
	case IntentTimeSeriesAnalysis:
		if !result.QueryFeatures.HasSorting {
			suggestions = append(suggestions, "建议按时间排序以更好地展现趋势")
		}
		
	case IntentDataQuery:
		if result.Confidence < 0.7 {
			suggestions = append(suggestions, "查询意图不够明确，建议提供更具体的描述")
		}
	}
	
	// 性能建议
	if result.QueryFeatures.QueryComplexity == "complex" {
		suggestions = append(suggestions, "查询较为复杂，建议添加适当的条件来限制结果集大小")
	}
	
	return suggestions
}

// getUserProfile 获取用户档案
func (ia *IntentAnalyzer) getUserProfile(userID int64) *UserIntentProfile {
	profile, exists := ia.userPatterns[userID]
	if !exists {
		profile = &UserIntentProfile{
			UserID:          userID,
			IntentHistory:   make([]QueryIntent, 0, ia.config.UserProfileSize),
			PreferredIntents: make(map[QueryIntent]float64),
			CommonKeywords:  make(map[string]int),
			LastUpdated:     time.Now(),
		}
		ia.userPatterns[userID] = profile
	}
	
	return profile
}

// updateUserProfile 更新用户档案
func (ia *IntentAnalyzer) updateUserProfile(userID int64, intent QueryIntent, query string) {
	profile := ia.getUserProfile(userID)
	
	// 添加到历史记录
	profile.IntentHistory = append(profile.IntentHistory, intent)
	if len(profile.IntentHistory) > ia.config.UserProfileSize {
		profile.IntentHistory = profile.IntentHistory[1:] // 移除最旧的记录
	}
	
	// 更新偏好统计
	intentCount := 0
	for _, historyIntent := range profile.IntentHistory {
		if historyIntent == intent {
			intentCount++
		}
	}
	profile.PreferredIntents[intent] = float64(intentCount) / float64(len(profile.IntentHistory))
	
	// 更新关键词统计
	keywords := ia.extractKeywords(query)
	for _, keyword := range keywords {
		profile.CommonKeywords[keyword]++
	}
	
	profile.QueryCount++
	profile.LastUpdated = time.Now()
}

// cacheResult 缓存分析结果
func (ia *IntentAnalyzer) cacheResult(query string, result *IntentResult) {
	if len(ia.analysisCache) >= ia.config.CacheSize {
		// 简单的LRU清理：删除一些旧的条目
		count := 0
		for k := range ia.analysisCache {
			delete(ia.analysisCache, k)
			count++
			if count >= ia.config.CacheSize/4 { // 清理25%
				break
			}
		}
	}
	
	ia.analysisCache[query] = result
}

// GetIntentName 获取意图名称
func (ia *IntentAnalyzer) GetIntentName(intent QueryIntent) string {
	names := map[QueryIntent]string{
		IntentUnknown:            "未知意图",
		IntentDataQuery:          "数据查询",
		IntentAggregation:        "聚合统计",
		IntentJoinQuery:          "关联查询",
		IntentTimeSeriesAnalysis: "时间序列分析",
		IntentComparison:         "对比分析",
		IntentRanking:           "排序排名",
		IntentFiltering:         "条件筛选",
		IntentGrouping:          "分组查询",
	}
	
	if name, exists := names[intent]; exists {
		return name
	}
	return "未知意图"
}

// GetUserStats 获取用户统计信息
func (ia *IntentAnalyzer) GetUserStats(userID int64) *UserIntentProfile {
	return ia.getUserProfile(userID)
}

// ClearCache 清理缓存
func (ia *IntentAnalyzer) ClearCache() {
	ia.analysisCache = make(map[string]*IntentResult)
}

// GetCacheStats 获取缓存统计
func (ia *IntentAnalyzer) GetCacheStats() map[string]int {
	return map[string]int{
		"cache_size":     len(ia.analysisCache),
		"max_cache_size": ia.config.CacheSize,
		"user_profiles":  len(ia.userPatterns),
	}
}