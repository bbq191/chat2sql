# 🔴 Redis语义缓存系统指南

<div align="center">

![Redis](https://img.shields.io/badge/Redis-7.4+-red.svg)
![Cache](https://img.shields.io/badge/Cache-Semantic-blue.svg)
![HitRate](https://img.shields.io/badge/Hit_Rate->75%25-green.svg)

**Chat2SQL P2阶段 - 高性能语义缓存完整实现方案**

</div>

## 📋 概述

本文档专门针对Chat2SQL系统的Redis语义缓存实现，提供从架构设计到代码实现的完整技术方案，实现高命中率的智能缓存系统，大幅提升查询性能并降低AI调用成本。

## 🎯 核心功能

### 缓存能力
- ✅ **精确匹配缓存**：查询字符串完全匹配的高速缓存
- ✅ **语义相似缓存**：基于编辑距离和语义相似度的智能缓存
- ✅ **分层缓存架构**：L1内存缓存 + L2 Redis缓存
- ✅ **动态TTL管理**：根据查询频率和质量动态调整过期时间

### 性能指标
| 缓存类型 | 命中率目标 | 响应时间 | 成本节省 |
|---------|-----------|---------|----------|
| **精确匹配** | > 40% | < 1ms | 100% |
| **语义相似** | > 35% | < 5ms | 100% |
| **部分结果** | > 20% | < 10ms | 60% |

---

## 🏗️ 缓存系统架构

### 📦 核心组件设计

```go
// internal/cache/semantic_cache.go
package cache

import (
    "context"
    "crypto/md5"
    "encoding/json"
    "fmt"
    "sort"
    "sync"
    "time"
    
    "github.com/go-redis/redis/v9"
)

type SemanticCache struct {
    // Redis客户端
    redisClient    redis.UniversalClient
    
    // 内存缓存 (L1)
    memoryCache    *MemoryCache
    
    // 缓存策略
    strategy       *CacheStrategy
    
    // 相似度计算
    similarityCalculator *SimilarityCalculator
    
    // 配置
    config         *CacheConfig
    
    // 监控
    metrics        *CacheMetrics
    
    // 并发控制
    mu             sync.RWMutex
}

type CacheConfig struct {
    // Redis配置
    Redis struct {
        Addrs        []string      `yaml:"addrs"`          // ["localhost:6379"]
        Password     string        `yaml:"password"`       // ""
        DB           int           `yaml:"db"`             // 0
        PoolSize     int           `yaml:"pool_size"`      // 100
        MinIdleConns int           `yaml:"min_idle_conns"` // 20
    } `yaml:"redis"`
    
    // 缓存策略
    Strategy struct {
        // TTL配置
        ExactMatchTTL    time.Duration `yaml:"exact_match_ttl"`    // 24h
        SimilarTTL       time.Duration `yaml:"similar_ttl"`        // 12h
        PartialTTL       time.Duration `yaml:"partial_ttl"`        // 6h
        
        // 相似度阈值
        SimilarityThreshold float64 `yaml:"similarity_threshold"` // 0.85
        EditDistanceThreshold int   `yaml:"edit_distance_threshold"` // 5
        
        // 缓存大小
        MaxMemoryCacheSize int `yaml:"max_memory_cache_size"` // 10000
        MaxRedisCacheSize  int `yaml:"max_redis_cache_size"`  // 100000
    } `yaml:"strategy"`
    
    // 预热配置
    Warmup struct {
        Enabled       bool     `yaml:"enabled"`        // true
        CommonQueries []string `yaml:"common_queries"` // 常用查询列表
        WarmupOnStart bool     `yaml:"warmup_on_start"` // true
    } `yaml:"warmup"`
}

type CacheEntry struct {
    // 基础信息
    Key           string                 `json:"key"`
    Query         string                 `json:"query"`
    Result        *QueryResult           `json:"result"`
    
    // 时间信息
    CreatedAt     time.Time              `json:"created_at"`
    LastAccessed  time.Time              `json:"last_accessed"`
    ExpiresAt     time.Time              `json:"expires_at"`
    
    // 统计信息
    HitCount      int64                  `json:"hit_count"`
    Quality       float64                `json:"quality"`      // 结果质量评分
    
    // 元数据
    Metadata      map[string]interface{} `json:"metadata"`
    
    // 相似度信息
    SimilarQueries []SimilarQuery        `json:"similar_queries,omitempty"`
}

type QueryResult struct {
    SQL           string                 `json:"sql"`
    Explanation   string                 `json:"explanation,omitempty"`
    Tables        []string               `json:"tables"`
    Confidence    float64                `json:"confidence"`
    ModelUsed     string                 `json:"model_used"`
    GeneratedAt   time.Time              `json:"generated_at"`
    TokensUsed    int                    `json:"tokens_used"`
    Cost          float64                `json:"cost"`
}

type SimilarQuery struct {
    Query      string  `json:"query"`
    Similarity float64 `json:"similarity"`
    LastUsed   time.Time `json:"last_used"`
}
```

### 🔧 缓存系统初始化

```go
func NewSemanticCache(config *CacheConfig) (*SemanticCache, error) {
    // 创建Redis客户端
    rdb := redis.NewUniversalClient(&redis.UniversalOptions{
        Addrs:        config.Redis.Addrs,
        Password:     config.Redis.Password,
        DB:           config.Redis.DB,
        PoolSize:     config.Redis.PoolSize,
        MinIdleConns: config.Redis.MinIdleConns,
    })
    
    // 测试连接
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("Redis连接失败: %w", err)
    }
    
    cache := &SemanticCache{
        redisClient:          rdb,
        memoryCache:          NewMemoryCache(config.Strategy.MaxMemoryCacheSize),
        strategy:            NewCacheStrategy(config),
        similarityCalculator: NewSimilarityCalculator(),
        config:              config,
        metrics:             NewCacheMetrics(),
    }
    
    // 预热缓存
    if config.Warmup.Enabled && config.Warmup.WarmupOnStart {
        go cache.warmupCache()
    }
    
    // 启动后台任务
    go cache.startBackgroundTasks()
    
    return cache, nil
}

func (sc *SemanticCache) warmupCache() {
    log.Info("开始缓存预热...")
    
    for _, query := range sc.config.Warmup.CommonQueries {
        // 预热常用查询的向量化结果
        if err := sc.precomputeSimilarity(query); err != nil {
            log.Warn("预热查询失败", zap.String("query", query), zap.Error(err))
        }
    }
    
    log.Info("缓存预热完成", zap.Int("queries", len(sc.config.Warmup.CommonQueries)))
}

func (sc *SemanticCache) startBackgroundTasks() {
    // 定期清理过期缓存
    go sc.startExpirationCleanup()
    
    // 定期收集指标
    go sc.startMetricsCollection()
    
    // 定期优化缓存
    go sc.startCacheOptimization()
}
```

---

## 🎯 精确匹配缓存实现

### 高速精确匹配

```go
// internal/cache/exact_cache.go
package cache

import (
    "context"
    "crypto/md5"
    "fmt"
    "time"
)

func (sc *SemanticCache) GetExact(ctx context.Context, query string) (*QueryResult, error) {
    start := time.Now()
    defer func() {
        sc.metrics.RecordCacheLatency("exact", time.Since(start))
    }()
    
    // 1. 生成缓存键
    cacheKey := sc.generateExactCacheKey(query)
    
    // 2. 先查L1内存缓存
    if entry := sc.memoryCache.Get(cacheKey); entry != nil {
        sc.metrics.RecordCacheHit("exact", "memory")
        sc.updateAccessInfo(entry)
        return entry.Result, nil
    }
    
    // 3. 查L2 Redis缓存
    entry, err := sc.getFromRedis(ctx, cacheKey)
    if err != nil {
        sc.metrics.RecordCacheError("exact", "redis")
        return nil, err
    }
    
    if entry != nil {
        sc.metrics.RecordCacheHit("exact", "redis")
        
        // 回填到内存缓存
        sc.memoryCache.Set(cacheKey, entry)
        sc.updateAccessInfo(entry)
        
        return entry.Result, nil
    }
    
    // 4. 缓存未命中
    sc.metrics.RecordCacheMiss("exact")
    return nil, nil
}

func (sc *SemanticCache) SetExact(
    ctx context.Context, 
    query string, 
    result *QueryResult) error {
    
    start := time.Now()
    defer func() {
        sc.metrics.RecordCacheLatency("exact_set", time.Since(start))
    }()
    
    cacheKey := sc.generateExactCacheKey(query)
    
    entry := &CacheEntry{
        Key:       cacheKey,
        Query:     query,
        Result:    result,
        CreatedAt: time.Now(),
        LastAccessed: time.Now(),
        ExpiresAt: time.Now().Add(sc.config.Strategy.ExactMatchTTL),
        HitCount:  0,
        Quality:   sc.calculateResultQuality(result),
        Metadata:  make(map[string]interface{}),
    }
    
    // 同时写入内存和Redis
    sc.memoryCache.Set(cacheKey, entry)
    
    if err := sc.setToRedis(ctx, cacheKey, entry); err != nil {
        sc.metrics.RecordCacheError("exact_set", "redis")
        return fmt.Errorf("Redis写入失败: %w", err)
    }
    
    sc.metrics.RecordCacheSet("exact")
    return nil
}

func (sc *SemanticCache) generateExactCacheKey(query string) string {
    // 标准化查询字符串
    normalized := sc.normalizeQuery(query)
    
    // 生成MD5哈希
    hash := md5.Sum([]byte(normalized))
    return fmt.Sprintf("exact:%x", hash)
}

func (sc *SemanticCache) normalizeQuery(query string) string {
    // 1. 转换为小写
    normalized := strings.ToLower(query)
    
    // 2. 去除多余空格
    normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
    
    // 3. 去除首尾空格
    normalized = strings.TrimSpace(normalized)
    
    // 4. 去除标点符号差异
    normalized = regexp.MustCompile(`[，。！？；：]/g`).ReplaceAllString(normalized, "")
    
    return normalized
}

func (sc *SemanticCache) calculateResultQuality(result *QueryResult) float64 {
    quality := 0.0
    
    // 基于信心度 (50%)
    quality += result.Confidence * 0.5
    
    // 基于SQL复杂度 (20%)
    sqlComplexity := sc.analyzeSQLComplexity(result.SQL)
    quality += sqlComplexity * 0.2
    
    // 基于模型质量 (20%)
    modelQuality := sc.getModelQuality(result.ModelUsed)
    quality += modelQuality * 0.2
    
    // 基于响应时间 (10%)
    responseTime := time.Since(result.GeneratedAt).Seconds()
    timeScore := math.Max(0, 1.0-responseTime/10.0) // 10秒内响应得满分
    quality += timeScore * 0.1
    
    return math.Min(quality, 1.0)
}
```

---

## 🧠 语义相似缓存实现

### 智能相似度匹配

```go
// internal/cache/similarity_cache.go
package cache

import (
    "context"
    "math"
    "sort"
    "strings"
    "time"
)

type SimilarityCalculator struct {
    // 编辑距离计算器
    editDistanceCalculator *EditDistanceCalculator
    
    // 语义相似度计算器
    semanticCalculator     *SemanticSimilarityCalculator
    
    // 缓存
    similarityCache        map[string]float64
    mu                     sync.RWMutex
}

type SimilarityResult struct {
    Query           string  `json:"query"`
    CacheKey        string  `json:"cache_key"`
    Similarity      float64 `json:"similarity"`
    EditDistance    int     `json:"edit_distance"`
    SemanticScore   float64 `json:"semantic_score"`
    OverallScore    float64 `json:"overall_score"`
}

func (sc *SemanticCache) GetSimilar(
    ctx context.Context, 
    query string, 
    threshold float64) (*QueryResult, error) {
    
    start := time.Now()
    defer func() {
        sc.metrics.RecordCacheLatency("similar", time.Since(start))
    }()
    
    // 1. 查找相似查询
    similarQueries, err := sc.findSimilarQueries(ctx, query, threshold)
    if err != nil {
        return nil, err
    }
    
    if len(similarQueries) == 0 {
        sc.metrics.RecordCacheMiss("similar")
        return nil, nil
    }
    
    // 2. 选择最佳匹配
    bestMatch := sc.selectBestMatch(similarQueries)
    
    // 3. 获取缓存结果
    entry, err := sc.getFromRedis(ctx, bestMatch.CacheKey)
    if err != nil {
        return nil, err
    }
    
    if entry == nil {
        sc.metrics.RecordCacheMiss("similar")
        return nil, nil
    }
    
    sc.metrics.RecordCacheHit("similar", "redis")
    
    // 4. 更新相似查询信息
    sc.updateSimilarQueryInfo(entry, query, bestMatch.Similarity)
    
    return entry.Result, nil
}

func (sc *SemanticCache) findSimilarQueries(
    ctx context.Context, 
    query string, 
    threshold float64) ([]*SimilarityResult, error) {
    
    // 1. 获取候选查询列表
    candidates, err := sc.getCandidateQueries(ctx, query)
    if err != nil {
        return nil, err
    }
    
    var results []*SimilarityResult
    
    // 2. 计算相似度
    for _, candidate := range candidates {
        similarity := sc.calculateSimilarity(query, candidate.Query)
        
        if similarity >= threshold {
            results = append(results, &SimilarityResult{
                Query:        candidate.Query,
                CacheKey:     candidate.Key,
                Similarity:   similarity,
                OverallScore: similarity,
            })
        }
    }
    
    // 3. 按相似度排序
    sort.Slice(results, func(i, j int) bool {
        return results[i].OverallScore > results[j].OverallScore
    })
    
    return results, nil
}

func (sc *SemanticCache) calculateSimilarity(query1, query2 string) float64 {
    // 1. 标准化查询
    norm1 := sc.normalizeQuery(query1)
    norm2 := sc.normalizeQuery(query2)
    
    if norm1 == norm2 {
        return 1.0
    }
    
    // 2. 编辑距离相似度 (40%)
    editDistance := sc.calculateEditDistance(norm1, norm2)
    maxLen := math.Max(float64(len(norm1)), float64(len(norm2)))
    editSimilarity := 1.0 - float64(editDistance)/maxLen
    
    // 3. 字符级相似度 (30%)
    charSimilarity := sc.calculateCharSimilarity(norm1, norm2)
    
    // 4. 词级相似度 (30%)
    wordSimilarity := sc.calculateWordSimilarity(norm1, norm2)
    
    // 加权平均
    overall := editSimilarity*0.4 + charSimilarity*0.3 + wordSimilarity*0.3
    
    return overall
}

func (sc *SemanticCache) calculateEditDistance(s1, s2 string) int {
    len1, len2 := len(s1), len(s2)
    
    // 创建DP表
    dp := make([][]int, len1+1)
    for i := range dp {
        dp[i] = make([]int, len2+1)
    }
    
    // 初始化
    for i := 0; i <= len1; i++ {
        dp[i][0] = i
    }
    for j := 0; j <= len2; j++ {
        dp[0][j] = j
    }
    
    // 动态规划计算
    for i := 1; i <= len1; i++ {
        for j := 1; j <= len2; j++ {
            if s1[i-1] == s2[j-1] {
                dp[i][j] = dp[i-1][j-1]
            } else {
                dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
            }
        }
    }
    
    return dp[len1][len2]
}

func (sc *SemanticCache) calculateCharSimilarity(s1, s2 string) float64 {
    if len(s1) == 0 && len(s2) == 0 {
        return 1.0
    }
    if len(s1) == 0 || len(s2) == 0 {
        return 0.0
    }
    
    // 计算字符交集
    chars1 := make(map[rune]int)
    chars2 := make(map[rune]int)
    
    for _, char := range s1 {
        chars1[char]++
    }
    for _, char := range s2 {
        chars2[char]++
    }
    
    // 计算交集大小
    intersection := 0
    for char, count1 := range chars1 {
        if count2, exists := chars2[char]; exists {
            intersection += min(count1, count2)
        }
    }
    
    // 计算并集大小
    union := len(s1) + len(s2) - intersection
    
    return float64(intersection) / float64(union)
}

func (sc *SemanticCache) calculateWordSimilarity(s1, s2 string) float64 {
    words1 := strings.Fields(s1)
    words2 := strings.Fields(s2)
    
    if len(words1) == 0 && len(words2) == 0 {
        return 1.0
    }
    if len(words1) == 0 || len(words2) == 0 {
        return 0.0
    }
    
    // 计算词汇交集
    wordSet1 := make(map[string]bool)
    wordSet2 := make(map[string]bool)
    
    for _, word := range words1 {
        wordSet1[word] = true
    }
    for _, word := range words2 {
        wordSet2[word] = true
    }
    
    // 计算Jaccard相似度
    intersection := 0
    for word := range wordSet1 {
        if wordSet2[word] {
            intersection++
        }
    }
    
    union := len(wordSet1) + len(wordSet2) - intersection
    return float64(intersection) / float64(union)
}

func (sc *SemanticCache) getCandidateQueries(
    ctx context.Context, 
    query string) ([]*CacheEntry, error) {
    
    // 1. 基于查询长度筛选候选
    queryLen := len(query)
    pattern := fmt.Sprintf("query_len:%d-*", queryLen/10*10) // 按10字符分桶
    
    keys, err := sc.redisClient.Keys(ctx, pattern).Result()
    if err != nil {
        return nil, err
    }
    
    // 2. 限制候选数量，避免计算过载
    maxCandidates := 100
    if len(keys) > maxCandidates {
        keys = keys[:maxCandidates]
    }
    
    var candidates []*CacheEntry
    
    // 3. 批量获取候选条目
    if len(keys) > 0 {
        results := sc.redisClient.MGet(ctx, keys...)
        values, err := results.Result()
        if err != nil {
            return nil, err
        }
        
        for _, value := range values {
            if value != nil {
                var entry CacheEntry
                if err := json.Unmarshal([]byte(value.(string)), &entry); err == nil {
                    candidates = append(candidates, &entry)
                }
            }
        }
    }
    
    return candidates, nil
}

func (sc *SemanticCache) selectBestMatch(results []*SimilarityResult) *SimilarityResult {
    if len(results) == 0 {
        return nil
    }
    
    // 已经按相似度排序，返回第一个（最佳）
    return results[0]
}

func min(a, b, c int) int {
    if a < b {
        if a < c {
            return a
        }
        return c
    }
    if b < c {
        return b
    }
    return c
}
```

---

## 📊 分层缓存策略

### L1内存缓存 + L2 Redis缓存

```go
// internal/cache/memory_cache.go
package cache

import (
    "container/list"
    "sync"
    "time"
)

type MemoryCache struct {
    // LRU缓存
    capacity int
    cache    map[string]*list.Element
    lruList  *list.List
    
    // 并发控制
    mu       sync.RWMutex
    
    // 统计信息
    hits     int64
    misses   int64
    sets     int64
}

type memoryCacheItem struct {
    key       string
    entry     *CacheEntry
    createdAt time.Time
    lastAccess time.Time
}

func NewMemoryCache(capacity int) *MemoryCache {
    return &MemoryCache{
        capacity: capacity,
        cache:    make(map[string]*list.Element),
        lruList:  list.New(),
    }
}

func (mc *MemoryCache) Get(key string) *CacheEntry {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    if element, exists := mc.cache[key]; exists {
        // 检查过期
        item := element.Value.(*memoryCacheItem)
        if time.Now().After(item.entry.ExpiresAt) {
            mc.removeElement(element)
            mc.misses++
            return nil
        }
        
        // 移动到链表头部 (LRU)
        mc.lruList.MoveToFront(element)
        item.lastAccess = time.Now()
        
        mc.hits++
        return item.entry
    }
    
    mc.misses++
    return nil
}

func (mc *MemoryCache) Set(key string, entry *CacheEntry) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    now := time.Now()
    
    // 如果key已存在，更新值
    if element, exists := mc.cache[key]; exists {
        item := element.Value.(*memoryCacheItem)
        item.entry = entry
        item.lastAccess = now
        mc.lruList.MoveToFront(element)
        mc.sets++
        return
    }
    
    // 检查容量，如有必要移除最久未使用的项
    if mc.lruList.Len() >= mc.capacity {
        mc.removeOldest()
    }
    
    // 添加新项
    item := &memoryCacheItem{
        key:        key,
        entry:      entry,
        createdAt:  now,
        lastAccess: now,
    }
    
    element := mc.lruList.PushFront(item)
    mc.cache[key] = element
    mc.sets++
}

func (mc *MemoryCache) Delete(key string) bool {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    if element, exists := mc.cache[key]; exists {
        mc.removeElement(element)
        return true
    }
    
    return false
}

func (mc *MemoryCache) removeOldest() {
    element := mc.lruList.Back()
    if element != nil {
        mc.removeElement(element)
    }
}

func (mc *MemoryCache) removeElement(element *list.Element) {
    item := element.Value.(*memoryCacheItem)
    delete(mc.cache, item.key)
    mc.lruList.Remove(element)
}

func (mc *MemoryCache) Stats() map[string]interface{} {
    mc.mu.RLock()
    defer mc.mu.RUnlock()
    
    total := mc.hits + mc.misses
    hitRate := 0.0
    if total > 0 {
        hitRate = float64(mc.hits) / float64(total)
    }
    
    return map[string]interface{}{
        "capacity":   mc.capacity,
        "size":      mc.lruList.Len(),
        "hits":      mc.hits,
        "misses":    mc.misses,
        "sets":      mc.sets,
        "hit_rate":  hitRate,
    }
}

func (mc *MemoryCache) Clear() {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    
    mc.cache = make(map[string]*list.Element)
    mc.lruList = list.New()
    mc.hits = 0
    mc.misses = 0
    mc.sets = 0
}
```

### Redis缓存操作

```go
// internal/cache/redis_operations.go
package cache

import (
    "context"
    "encoding/json"
    "time"
)

func (sc *SemanticCache) getFromRedis(ctx context.Context, key string) (*CacheEntry, error) {
    result := sc.redisClient.Get(ctx, key)
    if result.Err() == redis.Nil {
        return nil, nil // 缓存未命中
    }
    if result.Err() != nil {
        return nil, result.Err()
    }
    
    var entry CacheEntry
    if err := json.Unmarshal([]byte(result.Val()), &entry); err != nil {
        return nil, err
    }
    
    // 检查过期时间
    if time.Now().After(entry.ExpiresAt) {
        // 异步删除过期条目
        go sc.redisClient.Del(context.Background(), key)
        return nil, nil
    }
    
    return &entry, nil
}

func (sc *SemanticCache) setToRedis(ctx context.Context, key string, entry *CacheEntry) error {
    data, err := json.Marshal(entry)
    if err != nil {
        return err
    }
    
    // 计算TTL
    ttl := time.Until(entry.ExpiresAt)
    if ttl <= 0 {
        return nil // 已过期，不存储
    }
    
    return sc.redisClient.Set(ctx, key, data, ttl).Err()
}

func (sc *SemanticCache) existsInRedis(ctx context.Context, key string) (bool, error) {
    result := sc.redisClient.Exists(ctx, key)
    if result.Err() != nil {
        return false, result.Err()
    }
    
    return result.Val() > 0, nil
}

func (sc *SemanticCache) deleteFromRedis(ctx context.Context, key string) error {
    return sc.redisClient.Del(ctx, key).Err()
}

func (sc *SemanticCache) getAllKeysFromRedis(ctx context.Context, pattern string) ([]string, error) {
    return sc.redisClient.Keys(ctx, pattern).Result()
}

// 批量操作
func (sc *SemanticCache) batchGetFromRedis(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
    if len(keys) == 0 {
        return make(map[string]*CacheEntry), nil
    }
    
    results := sc.redisClient.MGet(ctx, keys...)
    values, err := results.Result()
    if err != nil {
        return nil, err
    }
    
    entries := make(map[string]*CacheEntry)
    
    for i, value := range values {
        if value != nil {
            var entry CacheEntry
            if err := json.Unmarshal([]byte(value.(string)), &entry); err == nil {
                // 检查过期
                if time.Now().Before(entry.ExpiresAt) {
                    entries[keys[i]] = &entry
                }
            }
        }
    }
    
    return entries, nil
}

func (sc *SemanticCache) batchSetToRedis(ctx context.Context, entries map[string]*CacheEntry) error {
    if len(entries) == 0 {
        return nil
    }
    
    pipe := sc.redisClient.Pipeline()
    
    for key, entry := range entries {
        data, err := json.Marshal(entry)
        if err != nil {
            continue
        }
        
        ttl := time.Until(entry.ExpiresAt)
        if ttl > 0 {
            pipe.Set(ctx, key, data, ttl)
        }
    }
    
    _, err := pipe.Exec(ctx)
    return err
}
```

---

## 🕐 动态TTL管理

### 智能过期时间调整

```go
// internal/cache/ttl_manager.go
package cache

import (
    "context"
    "math"
    "time"
)

type TTLManager struct {
    baseConfig     *CacheConfig
    metrics        *CacheMetrics
    
    // TTL策略
    strategy       *TTLStrategy
}

type TTLStrategy struct {
    // 基础TTL
    BaseTTLs map[string]time.Duration
    
    // 调整因子
    QualityFactor    float64 // 质量因子
    AccessFactor     float64 // 访问频率因子
    SuccessFactor    float64 // 成功率因子
    
    // 限制
    MinTTL          time.Duration
    MaxTTL          time.Duration
}

func NewTTLManager(config *CacheConfig) *TTLManager {
    return &TTLManager{
        baseConfig: config,
        strategy: &TTLStrategy{
            BaseTTLs: map[string]time.Duration{
                "exact":   config.Strategy.ExactMatchTTL,
                "similar": config.Strategy.SimilarTTL,
                "partial": config.Strategy.PartialTTL,
            },
            QualityFactor:  1.5,
            AccessFactor:   2.0,
            SuccessFactor:  1.2,
            MinTTL:        1 * time.Hour,
            MaxTTL:        7 * 24 * time.Hour,
        },
    }
}

func (tm *TTLManager) CalculateOptimalTTL(
    cacheType string,
    entry *CacheEntry,
    stats *CacheStats) time.Duration {
    
    // 获取基础TTL
    baseTTL, exists := tm.strategy.BaseTTLs[cacheType]
    if !exists {
        baseTTL = 12 * time.Hour
    }
    
    // 计算调整因子
    adjustmentFactor := 1.0
    
    // 1. 质量因子调整
    if entry.Quality > 0.8 {
        adjustmentFactor *= tm.strategy.QualityFactor
    } else if entry.Quality < 0.5 {
        adjustmentFactor *= 0.7
    }
    
    // 2. 访问频率调整
    if stats != nil {
        accessRate := float64(entry.HitCount) / math.Max(1, time.Since(entry.CreatedAt).Hours())
        if accessRate > 1 { // 每小时访问超过1次
            adjustmentFactor *= tm.strategy.AccessFactor
        } else if accessRate < 0.1 { // 每小时访问少于0.1次
            adjustmentFactor *= 0.5
        }
    }
    
    // 3. 模型成功率调整
    if entry.Result != nil && entry.Result.Confidence > 0.9 {
        adjustmentFactor *= tm.strategy.SuccessFactor
    }
    
    // 4. 时间衰减因子
    age := time.Since(entry.CreatedAt)
    if age > 24*time.Hour {
        decayFactor := math.Exp(-age.Hours() / (24 * 7)) // 一周衰减
        adjustmentFactor *= decayFactor
    }
    
    // 计算最终TTL
    finalTTL := time.Duration(float64(baseTTL) * adjustmentFactor)
    
    // 应用限制
    if finalTTL < tm.strategy.MinTTL {
        finalTTL = tm.strategy.MinTTL
    }
    if finalTTL > tm.strategy.MaxTTL {
        finalTTL = tm.strategy.MaxTTL
    }
    
    return finalTTL
}

func (tm *TTLManager) UpdateTTL(ctx context.Context, key string, newTTL time.Duration) error {
    // 实现TTL更新逻辑
    return nil
}

// 批量TTL优化
func (sc *SemanticCache) optimizeTTLs(ctx context.Context) error {
    // 获取需要优化的缓存条目
    keys, err := sc.redisClient.Keys(ctx, "*").Result()
    if err != nil {
        return err
    }
    
    // 批量获取条目
    entries, err := sc.batchGetFromRedis(ctx, keys)
    if err != nil {
        return err
    }
    
    // 为每个条目计算新TTL
    updates := make(map[string]time.Duration)
    
    for key, entry := range entries {
        // 判断缓存类型
        cacheType := "exact"
        if strings.Contains(key, "similar:") {
            cacheType = "similar"
        } else if strings.Contains(key, "partial:") {
            cacheType = "partial"
        }
        
        // 计算新TTL
        newTTL := sc.ttlManager.CalculateOptimalTTL(cacheType, entry, nil)
        
        // 如果TTL变化显著，记录更新
        currentRemainingTTL := time.Until(entry.ExpiresAt)
        if math.Abs(newTTL.Seconds()-currentRemainingTTL.Seconds()) > 3600 { // 差异超过1小时
            updates[key] = newTTL
        }
    }
    
    // 批量更新TTL
    if len(updates) > 0 {
        pipe := sc.redisClient.Pipeline()
        for key, ttl := range updates {
            pipe.Expire(ctx, key, ttl)
        }
        _, err = pipe.Exec(ctx)
    }
    
    return err
}
```

---

## 📊 缓存性能监控

### 详细指标收集

```go
// internal/cache/metrics.go
package cache

import (
    "sync"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type CacheMetrics struct {
    // 缓存命中率
    CacheHits    *prometheus.CounterVec
    CacheMisses  *prometheus.CounterVec
    CacheSets    *prometheus.CounterVec
    
    // 缓存延迟
    CacheLatency *prometheus.HistogramVec
    
    // 缓存大小
    CacheSize    *prometheus.GaugeVec
    CacheMemory  *prometheus.GaugeVec
    
    // 过期和清理
    CacheEvictions *prometheus.CounterVec
    CacheExpired   *prometheus.CounterVec
    
    // 相似度分析
    SimilarityScores *prometheus.HistogramVec
    SimilarityTime   *prometheus.HistogramVec
    
    // 错误监控
    CacheErrors  *prometheus.CounterVec
    
    // 本地统计
    localStats   map[string]*LocalCacheStats
    mu           sync.RWMutex
}

type LocalCacheStats struct {
    HitCount      int64     `json:"hit_count"`
    MissCount     int64     `json:"miss_count"`
    SetCount      int64     `json:"set_count"`
    ErrorCount    int64     `json:"error_count"`
    TotalLatency  time.Duration `json:"total_latency"`
    LastUpdated   time.Time     `json:"last_updated"`
}

func NewCacheMetrics() *CacheMetrics {
    return &CacheMetrics{
        CacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "chat2sql_cache_hits_total",
            Help: "缓存命中总数",
        }, []string{"type", "layer"}),
        
        CacheMisses: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "chat2sql_cache_misses_total",
            Help: "缓存未命中总数",
        }, []string{"type"}),
        
        CacheSets: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "chat2sql_cache_sets_total",
            Help: "缓存写入总数",
        }, []string{"type"}),
        
        CacheLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name: "chat2sql_cache_latency_seconds",
            Help: "缓存操作延迟分布",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        }, []string{"operation", "type"}),
        
        CacheSize: promauto.NewGaugeVec(prometheus.GaugeOpts{
            Name: "chat2sql_cache_size_entries",
            Help: "缓存条目数量",
        }, []string{"type", "layer"}),
        
        CacheMemory: promauto.NewGaugeVec(prometheus.GaugeOpts{
            Name: "chat2sql_cache_memory_bytes",
            Help: "缓存内存使用量",
        }, []string{"type"}),
        
        SimilarityScores: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name: "chat2sql_similarity_scores",
            Help: "相似度分数分布",
            Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
        }, []string{"algorithm"}),
        
        CacheErrors: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "chat2sql_cache_errors_total",
            Help: "缓存错误总数",
        }, []string{"type", "operation"}),
        
        localStats: make(map[string]*LocalCacheStats),
    }
}

func (cm *CacheMetrics) RecordCacheHit(cacheType, layer string) {
    cm.CacheHits.WithLabelValues(cacheType, layer).Inc()
    cm.updateLocalStats(cacheType, "hit", 0)
}

func (cm *CacheMetrics) RecordCacheMiss(cacheType string) {
    cm.CacheMisses.WithLabelValues(cacheType).Inc()
    cm.updateLocalStats(cacheType, "miss", 0)
}

func (cm *CacheMetrics) RecordCacheSet(cacheType string) {
    cm.CacheSets.WithLabelValues(cacheType).Inc()
    cm.updateLocalStats(cacheType, "set", 0)
}

func (cm *CacheMetrics) RecordCacheLatency(operation string, latency time.Duration) {
    cm.CacheLatency.WithLabelValues(operation, "").Observe(latency.Seconds())
}

func (cm *CacheMetrics) RecordCacheError(cacheType, operation string) {
    cm.CacheErrors.WithLabelValues(cacheType, operation).Inc()
    cm.updateLocalStats(cacheType, "error", 0)
}

func (cm *CacheMetrics) RecordSimilarityScore(algorithm string, score float64) {
    cm.SimilarityScores.WithLabelValues(algorithm).Observe(score)
}

func (cm *CacheMetrics) updateLocalStats(cacheType, operation string, latency time.Duration) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    key := fmt.Sprintf("%s_%s", cacheType, operation)
    stats, exists := cm.localStats[key]
    if !exists {
        stats = &LocalCacheStats{}
        cm.localStats[key] = stats
    }
    
    switch operation {
    case "hit":
        stats.HitCount++
    case "miss":
        stats.MissCount++
    case "set":
        stats.SetCount++
    case "error":
        stats.ErrorCount++
    }
    
    stats.TotalLatency += latency
    stats.LastUpdated = time.Now()
}

func (cm *CacheMetrics) GetCacheStats() map[string]*LocalCacheStats {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    
    result := make(map[string]*LocalCacheStats)
    for key, stats := range cm.localStats {
        result[key] = &LocalCacheStats{
            HitCount:     stats.HitCount,
            MissCount:    stats.MissCount,
            SetCount:     stats.SetCount,
            ErrorCount:   stats.ErrorCount,
            TotalLatency: stats.TotalLatency,
            LastUpdated:  stats.LastUpdated,
        }
    }
    
    return result
}

func (cm *CacheMetrics) CalculateHitRate(cacheType string) float64 {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    
    hitKey := fmt.Sprintf("%s_hit", cacheType)
    missKey := fmt.Sprintf("%s_miss", cacheType)
    
    hitStats := cm.localStats[hitKey]
    missStats := cm.localStats[missKey]
    
    hits := int64(0)
    misses := int64(0)
    
    if hitStats != nil {
        hits = hitStats.HitCount
    }
    if missStats != nil {
        misses = missStats.MissCount
    }
    
    total := hits + misses
    if total == 0 {
        return 0.0
    }
    
    return float64(hits) / float64(total)
}
```

---

## 🛠️ 配置和部署

### 缓存配置示例

```yaml
# config/cache_config.yaml
cache:
  # Redis配置
  redis:
    addrs: ["localhost:6379"]
    password: ""
    db: 0
    pool_size: 100
    min_idle_conns: 20
    dial_timeout: "5s"
    read_timeout: "3s"
    write_timeout: "3s"
    
  # 缓存策略
  strategy:
    # TTL配置
    exact_match_ttl: "24h"
    similar_ttl: "12h"
    partial_ttl: "6h"
    
    # 相似度阈值
    similarity_threshold: 0.85
    edit_distance_threshold: 5
    
    # 缓存大小限制
    max_memory_cache_size: 10000
    max_redis_cache_size: 100000
    
    # 清理策略
    cleanup_interval: "1h"
    max_idle_time: "24h"
    
  # 预热配置
  warmup:
    enabled: true
    warmup_on_start: true
    common_queries:
      - "查询用户信息"
      - "获取订单列表"
      - "统计销售数据"
      - "分析用户行为"
      - "生成报表"
      
  # 监控配置
  monitoring:
    metrics_interval: "30s"
    detailed_logging: true
    slow_query_threshold: "100ms"
```

### Docker部署脚本

```bash
#!/bin/bash
# deploy/deploy_cache.sh

set -e

echo "🔴 部署Redis缓存系统..."

# 1. 创建Redis集群
echo "📦 启动Redis集群..."
docker network create chat2sql-net 2>/dev/null || true

# 主Redis实例
docker run -d --name chat2sql-redis-master \
  --network chat2sql-net \
  -p 6379:6379 \
  -v redis-master-data:/data \
  -e REDIS_PASSWORD=${REDIS_PASSWORD:-} \
  redis:7.4-alpine \
  redis-server --appendonly yes --maxmemory 2gb --maxmemory-policy allkeys-lru

# Redis Sentinel (高可用)
docker run -d --name chat2sql-redis-sentinel \
  --network chat2sql-net \
  -p 26379:26379 \
  -v redis-sentinel-conf:/etc/redis \
  redis:7.4-alpine \
  redis-sentinel /etc/redis/sentinel.conf

# 2. 部署缓存服务配置
echo "⚙️ 部署缓存配置..."
cp config/cache_config.yaml /etc/chat2sql/

# 3. 启动缓存预热
echo "🔥 开始缓存预热..."
go run tools/cache_warmup.go \
  --config=/etc/chat2sql/cache_config.yaml \
  --redis-addr=localhost:6379

echo "✅ 缓存系统部署完成！"
echo "🔴 Redis: localhost:6379"
echo "📊 缓存监控: http://localhost:9090/metrics"
```

---

<div align="center">

**🔴 语义缓存成功关键：精确匹配 + 智能相似 + 动态优化**

</div>