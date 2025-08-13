// 模型注册中心测试 - P2阶段智能路由核心组件测试
// 测试模型注册、健康检查、配置管理等核心功能

package routing

import (
	"context"
	"testing"
	"time"
)

func TestNewModelRegistry(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	
	if registry == nil {
		t.Fatal("创建模型注册中心失败")
	}
	
	if registry.models == nil {
		t.Error("models map未初始化")
	}
	
	if registry.configs == nil {
		t.Error("configs map未初始化")
	}
	
	if registry.healthChecker == nil {
		t.Error("健康检查器未初始化")
	}
	
	if registry.configManager == nil {
		t.Error("配置管理器未初始化")
	}
	
	// 清理
	defer registry.Close()
}

func TestModelRegistryRegisterModel(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 测试注册Ollama模型（不需要API密钥）
	config := &ModelConfig{
		Name:        "llama3.1:8b",
		Provider:    "ollama",
		Category:    "simple",
		Endpoint:    "http://localhost:11434",
		CostPer1K:   0.0,
		MaxTokens:   2048,
		Timeout:     30 * time.Second,
		QPS:         50,
		Accuracy:    0.75,
		AvgLatency:  3 * time.Second,
		Reliability: 0.95,
		Enabled:     true,
		Priority:    8,
	}
	
	err := registry.RegisterModel(config)
	if err != nil {
		t.Fatalf("注册模型失败: %v", err)
	}
	
	// 验证模型已注册
	model, err := registry.GetModel("llama3.1:8b")
	if err != nil {
		t.Fatalf("获取注册模型失败: %v", err)
	}
	
	if model.Name != "llama3.1:8b" {
		t.Errorf("模型名称不匹配: 期望 %s, 实际 %s", "llama3.1:8b", model.Name)
	}
	
	if model.Provider != "ollama" {
		t.Errorf("模型提供商不匹配: 期望 %s, 实际 %s", "ollama", model.Provider)
	}
	
	if model.Category != CategorySimple {
		t.Errorf("模型类别不匹配: 期望 %s, 实际 %s", CategorySimple, model.Category)
	}
}

func TestModelRegistryRegisterInvalidModel(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 测试注册无效模型（缺少名称）
	config := &ModelConfig{
		Provider:    "ollama",
		Category:    "simple",
		Endpoint:    "http://localhost:11434",
	}
	
	err := registry.RegisterModel(config)
	if err == nil {
		t.Error("应该拒绝注册无效模型")
	}
	
	// 测试注册无效模型（缺少提供商）
	config2 := &ModelConfig{
		Name:     "test-model",
		Category: "simple",
	}
	
	err = registry.RegisterModel(config2)
	if err == nil {
		t.Error("应该拒绝注册无效模型")
	}
	
	// 测试注册无效模型（无效类别）
	config3 := &ModelConfig{
		Name:     "test-model",
		Provider: "ollama",
		Category: "invalid",
	}
	
	err = registry.RegisterModel(config3)
	if err == nil {
		t.Error("应该拒绝注册无效模型")
	}
}

func TestModelRegistryGetModelsByCategory(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 注册不同类别的模型
	configs := []*ModelConfig{
		{
			Name:     "simple-model-1",
			Provider: "ollama",
			Category: "simple",
			Endpoint: "http://localhost:11434",
			Enabled:  true,
		},
		{
			Name:     "simple-model-2", 
			Provider: "ollama",
			Category: "simple",
			Endpoint: "http://localhost:11434",
			Enabled:  true,
		},
		{
			Name:     "medium-model-1",
			Provider: "ollama", 
			Category: "medium",
			Endpoint: "http://localhost:11434",
			Enabled:  true,
		},
	}
	
	for _, config := range configs {
		err := registry.RegisterModel(config)
		if err != nil {
			t.Fatalf("注册模型失败: %v", err)
		}
	}
	
	// 测试按类别获取模型
	simpleModels := registry.GetModelsByCategory(CategorySimple)
	if len(simpleModels) != 2 {
		t.Errorf("简单模型数量不匹配: 期望 2, 实际 %d", len(simpleModels))
	}
	
	mediumModels := registry.GetModelsByCategory(CategoryMedium)
	if len(mediumModels) != 1 {
		t.Errorf("中等模型数量不匹配: 期望 1, 实际 %d", len(mediumModels))
	}
	
	complexModels := registry.GetModelsByCategory(CategoryComplex)
	if len(complexModels) != 0 {
		t.Errorf("复杂模型数量不匹配: 期望 0, 实际 %d", len(complexModels))
	}
}

func TestModelRegistryGetAllModels(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 注册多个模型
	configs := []*ModelConfig{
		{
			Name:     "model-1",
			Provider: "ollama",
			Category: "simple",
			Endpoint: "http://localhost:11434",
			Enabled:  true,
		},
		{
			Name:     "model-2",
			Provider: "ollama",
			Category: "medium",
			Endpoint: "http://localhost:11434", 
			Enabled:  true,
		},
	}
	
	for _, config := range configs {
		err := registry.RegisterModel(config)
		if err != nil {
			t.Fatalf("注册模型失败: %v", err)
		}
	}
	
	// 测试获取所有模型
	allModels := registry.GetAllModels()
	if len(allModels) != 2 {
		t.Errorf("模型总数不匹配: 期望 2, 实际 %d", len(allModels))
	}
}

func TestModelRegistryUnregisterModel(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 先注册一个模型
	config := &ModelConfig{
		Name:     "test-model",
		Provider: "ollama",
		Category: "simple",
		Endpoint: "http://localhost:11434",
		Enabled:  true,
	}
	
	err := registry.RegisterModel(config)
	if err != nil {
		t.Fatalf("注册模型失败: %v", err)
	}
	
	// 验证模型已注册
	_, err = registry.GetModel("test-model")
	if err != nil {
		t.Fatalf("获取模型失败: %v", err)
	}
	
	// 注销模型
	err = registry.UnregisterModel("test-model")
	if err != nil {
		t.Fatalf("注销模型失败: %v", err)
	}
	
	// 验证模型已注销
	_, err = registry.GetModel("test-model")
	if err == nil {
		t.Error("模型应该已被注销")
	}
}

func TestModelRegistryUpdateModelConfig(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 注册初始模型
	config := &ModelConfig{
		Name:     "test-model",
		Provider: "ollama",
		Category: "simple",
		Endpoint: "http://localhost:11434",
		Enabled:  true,
		MaxTokens: 1024,
	}
	
	err := registry.RegisterModel(config)
	if err != nil {
		t.Fatalf("注册模型失败: %v", err)
	}
	
	// 更新配置
	newConfig := &ModelConfig{
		Name:     "test-model",
		Provider: "ollama",
		Category: "medium", // 类别改变
		Endpoint: "http://localhost:11434",
		Enabled:  true,
		MaxTokens: 2048, // MaxTokens改变
	}
	
	err = registry.UpdateModelConfig("test-model", newConfig)
	if err != nil {
		t.Fatalf("更新模型配置失败: %v", err)
	}
	
	// 验证配置已更新
	model, err := registry.GetModel("test-model")
	if err != nil {
		t.Fatalf("获取模型失败: %v", err)
	}
	
	if model.Category != CategoryMedium {
		t.Errorf("模型类别未更新: 期望 %s, 实际 %s", CategoryMedium, model.Category)
	}
	
	if model.Config.MaxTokens != 2048 {
		t.Errorf("MaxTokens未更新: 期望 %d, 实际 %d", 2048, model.Config.MaxTokens)
	}
}

func TestModelRegistryGetRegistryStats(t *testing.T) {
	ctx := context.Background()
	registry := NewModelRegistry(ctx)
	defer registry.Close()
	
	// 注册不同类型的模型
	configs := []*ModelConfig{
		{
			Name:     "simple-model",
			Provider: "ollama",
			Category: "simple",
			Endpoint: "http://localhost:11434",
			Enabled:  true,
		},
		{
			Name:     "openai-model",
			Provider: "openai",
			Category: "complex",
			APIKey:   "test-key",
			Enabled:  true,
		},
	}
	
	for _, config := range configs {
		err := registry.RegisterModel(config)
		if err != nil {
			t.Fatalf("注册模型失败: %v", err)
		}
	}
	
	// 获取统计信息
	stats := registry.GetRegistryStats()
	
	if stats["total_models"].(int) != 2 {
		t.Errorf("总模型数不匹配: 期望 2, 实际 %d", stats["total_models"].(int))
	}
	
	providerStats := stats["models_by_provider"].(map[string]int)
	if providerStats["ollama"] != 1 {
		t.Errorf("Ollama模型数不匹配: 期望 1, 实际 %d", providerStats["ollama"])
	}
	
	if providerStats["openai"] != 1 {
		t.Errorf("OpenAI模型数不匹配: 期望 1, 实际 %d", providerStats["openai"])
	}
	
	categoryStats := stats["models_by_category"].(map[string]int)
	if categoryStats["simple"] != 1 {
		t.Errorf("简单模型数不匹配: 期望 1, 实际 %d", categoryStats["simple"])
	}
	
	if categoryStats["complex"] != 1 {
		t.Errorf("复杂模型数不匹配: 期望 1, 实际 %d", categoryStats["complex"])
	}
}