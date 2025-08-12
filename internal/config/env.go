// 环境变量配置加载器
// 支持从.env文件加载配置

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnv 从.env文件加载环境变量
func LoadEnv(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Warning: %s file not found, using system environment variables\n", filepath)
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", filepath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析键值对
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: invalid line %d in %s: %s\n", lineNum, filepath, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 只有当环境变量未设置时才设置
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading %s: %w", filepath, err)
	}

	fmt.Printf("✅ Loaded environment variables from %s\n", filepath)
	return nil
}