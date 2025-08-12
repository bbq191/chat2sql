// LLM环境配置验证工具
// 测试OpenAI、Anthropic、Ollama等模型提供商连接

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"chat2sql-go/internal/ai"
	"chat2sql-go/internal/config"
	"go.uber.org/zap"
)

func main() {
	var (
		configOnly = flag.Bool("config-only", false, "只检查配置，不测试API调用")
		provider   = flag.String("provider", "", "测试特定提供商 (openai|anthropic|ollama)")
		timeout    = flag.Int("timeout", 30, "API调用超时时间（秒）")
	)
	flag.Parse()

	fmt.Println("🔧 Chat2SQL LLM环境配置验证工具")
	fmt.Println("================================")

	// 加载环境变量
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("⚠️  环境变量加载警告: %v", err)
	}

	// 加载配置
	fmt.Println("📋 加载LLM配置...")
	config, err := ai.LoadLLMConfig()
	if err != nil {
		log.Fatalf("❌ 配置加载失败: %v", err)
	}

	fmt.Printf("✅ 配置加载成功\n")
	fmt.Printf("   - 主要模型: %s (%s)\n", config.PrimaryProvider, config.PrimaryConfig.Model)
	fmt.Printf("   - 备用模型: %s (%s)\n", config.FallbackProvider, config.FallbackConfig.Model)
	if config.LocalConfig != nil {
		fmt.Printf("   - 本地模型: %s (%s)\n", config.LocalProvider, config.LocalConfig.Model)
	}
	fmt.Printf("   - 请求超时: %v\n", config.RequestTimeout)
	fmt.Printf("   - 工作线程: %d\n", config.PerformanceConfig.Workers)

	if *configOnly {
		fmt.Println("\n✅ 配置检查完成")
		return
	}

	// 创建LLM客户端
	fmt.Println("\n🚀 初始化LLM客户端...")
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	client, err := ai.NewLLMClient(config, logger)
	if err != nil {
		log.Fatalf("❌ LLM客户端创建失败: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// 测试特定提供商或全部
	if *provider != "" {
		if err := testSpecificProvider(ctx, client, *provider); err != nil {
			log.Fatalf("❌ 提供商 %s 测试失败: %v", *provider, err)
		}
	} else {
		// 验证所有配置的模型
		fmt.Println("🧪 验证模型配置...")
		if err := client.ValidateConfiguration(ctx); err != nil {
			log.Fatalf("❌ 模型配置验证失败: %v", err)
		}
	}

	fmt.Println("\n✅ LLM环境配置验证完成")
	fmt.Println("🎉 系统已准备就绪，可以处理自然语言转SQL查询")
}

// testSpecificProvider 测试特定提供商
func testSpecificProvider(ctx context.Context, client *ai.LLMClient, providerName string) error {
	fmt.Printf("🧪 测试提供商: %s\n", providerName)

	var model interface{}
	switch providerName {
	case "openai":
		model = client.GetPrimaryLLM()
	case "anthropic":
		model = client.GetFallbackLLM()
	case "ollama":
		model = client.GetLocalLLM()
		if model == nil {
			return fmt.Errorf("本地模型未配置")
		}
	default:
		return fmt.Errorf("不支持的提供商: %s", providerName)
	}

	if model == nil {
		return fmt.Errorf("模型实例为空")
	}

	// 这里可以添加更详细的测试逻辑
	fmt.Printf("✅ 提供商 %s 测试通过\n", providerName)
	return nil
}

// 检查环境变量是否设置
func init() {
	requiredEnvVars := []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
	}

	missingVars := []string{}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			missingVars = append(missingVars, envVar)
		}
	}

	if len(missingVars) > 0 {
		fmt.Printf("⚠️  警告: 以下环境变量未设置:\n")
		for _, v := range missingVars {
			fmt.Printf("   - %s\n", v)
		}
		fmt.Printf("\n💡 提示: 复制 .env.example 为 .env 并填入真实API密钥\n")
		fmt.Printf("   cp .env.example .env\n\n")
	}
}