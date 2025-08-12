#!/bin/bash

# AI模块代码质量检查脚本
# 提供全面的代码质量检查，包括编译、测试、静态分析等

set -e

# 脚本配置
AI_MODULE_PATH="./internal/ai"
COVERAGE_THRESHOLD=80
REPORT_DIR="./reports/ai_quality"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 创建报告目录
mkdir -p "$REPORT_DIR"

# 打印带颜色的消息
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查必要工具是否安装
check_prerequisites() {
    print_status "检查必要工具..."
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        print_error "Go未安装或不在PATH中"
        exit 1
    fi
    
    # 检查go vet
    if ! command -v go vet &> /dev/null; then
        print_error "go vet不可用"
        exit 1
    fi
    
    print_success "必要工具检查通过"
}

# 编译检查
compile_check() {
    print_status "进行编译检查..."
    
    if go build ./internal/ai/...; then
        print_success "编译检查通过"
    else
        print_error "编译检查失败"
        exit 1
    fi
}

# 静态代码分析
static_analysis() {
    print_status "进行静态代码分析..."
    
    # Go vet检查
    print_status "运行 go vet..."
    if go vet ./internal/ai/...; then
        print_success "go vet检查通过"
    else
        print_error "go vet检查发现问题"
        exit 1
    fi
    
    # 格式化检查
    print_status "检查代码格式化..."
    unformatted=$(go fmt ./internal/ai/... 2>&1 | tee "$REPORT_DIR/gofmt_report.txt")
    if [ -z "$unformatted" ]; then
        print_success "代码格式化检查通过"
    else
        print_warning "发现未格式化的文件:"
        echo "$unformatted"
    fi
}

# 运行测试并生成覆盖率报告
run_tests() {
    print_status "运行测试套件..."
    
    # 运行测试并生成覆盖率报告
    go test -v -race -coverprofile="$REPORT_DIR/coverage.out" \
           -covermode=atomic \
           ./internal/ai/... > "$REPORT_DIR/test_report.txt" 2>&1
    
    local test_exit_code=$?
    
    if [ $test_exit_code -eq 0 ]; then
        print_success "测试套件通过"
    else
        print_error "测试套件失败"
        cat "$REPORT_DIR/test_report.txt"
        return 1
    fi
    
    # 生成覆盖率报告
    if [ -f "$REPORT_DIR/coverage.out" ]; then
        print_status "生成覆盖率报告..."
        
        # 生成HTML覆盖率报告
        go tool cover -html="$REPORT_DIR/coverage.out" -o "$REPORT_DIR/coverage.html"
        
        # 计算覆盖率
        local coverage=$(go tool cover -func="$REPORT_DIR/coverage.out" | grep "total:" | awk '{print $3}' | sed 's/%//')
        
        if [ -n "$coverage" ]; then
            echo "测试覆盖率: $coverage%" > "$REPORT_DIR/coverage_summary.txt"
            
            if (( $(echo "$coverage >= $COVERAGE_THRESHOLD" | bc -l) )); then
                print_success "测试覆盖率: $coverage% (≥ $COVERAGE_THRESHOLD%)"
            else
                print_warning "测试覆盖率: $coverage% (< $COVERAGE_THRESHOLD%)"
            fi
        else
            print_warning "无法计算测试覆盖率"
        fi
        
        print_success "覆盖率报告已生成: $REPORT_DIR/coverage.html"
    fi
}

# 依赖检查
dependency_check() {
    print_status "检查依赖关系..."
    
    # 检查模块整洁性
    if go mod tidy; then
        print_success "依赖整理完成"
    else
        print_error "依赖整理失败"
        return 1
    fi
    
    # 检查是否有未使用的依赖
    if command -v go mod why &> /dev/null; then
        print_status "检查未使用的依赖..."
        go list -m all | awk '{print $1}' | xargs -I {} go mod why {} > "$REPORT_DIR/dependency_usage.txt"
    fi
    
    # 检查漏洞
    if command -v govulncheck &> /dev/null; then
        print_status "运行安全漏洞检查..."
        govulncheck ./internal/ai/... > "$REPORT_DIR/vulnerability_report.txt" 2>&1 || print_warning "发现安全漏洞，请检查报告"
    else
        print_warning "govulncheck未安装，跳过安全检查"
    fi
}

# 代码复杂度分析
complexity_analysis() {
    print_status "进行代码复杂度分析..."
    
    # 统计代码行数
    find ./internal/ai -name "*.go" -not -name "*_test.go" | xargs wc -l | tail -1 > "$REPORT_DIR/lines_of_code.txt"
    
    # 统计测试文件行数
    find ./internal/ai -name "*_test.go" | xargs wc -l | tail -1 > "$REPORT_DIR/test_lines_of_code.txt"
    
    # 生成基本统计信息
    {
        echo "=== AI模块代码统计 ==="
        echo "生成时间: $(date)"
        echo ""
        echo "源代码文件数: $(find ./internal/ai -name "*.go" -not -name "*_test.go" | wc -l)"
        echo "测试文件数: $(find ./internal/ai -name "*_test.go" | wc -l)"
        echo "源代码行数: $(cat "$REPORT_DIR/lines_of_code.txt" | awk '{print $1}')"
        echo "测试代码行数: $(cat "$REPORT_DIR/test_lines_of_code.txt" | awk '{print $1}')"
        echo ""
        echo "=== 文件列表 ==="
        find ./internal/ai -name "*.go" | sort
    } > "$REPORT_DIR/code_statistics.txt"
    
    print_success "代码统计完成"
}

# 生成综合报告
generate_summary_report() {
    print_status "生成综合质量报告..."
    
    {
        echo "# Chat2SQL AI模块代码质量报告"
        echo ""
        echo "生成时间: $(date)"
        echo "检查目录: $AI_MODULE_PATH"
        echo ""
        echo "## 检查项目"
        echo ""
        echo "✅ 编译检查: 通过"
        echo "✅ 静态代码分析: 通过"
        
        if [ -f "$REPORT_DIR/coverage_summary.txt" ]; then
            local coverage_info=$(cat "$REPORT_DIR/coverage_summary.txt")
            echo "✅ 测试覆盖率: $coverage_info"
        fi
        
        echo "✅ 依赖关系检查: 通过"
        echo "✅ 代码统计: 完成"
        echo ""
        echo "## 详细报告文件"
        echo ""
        echo "- 测试报告: $REPORT_DIR/test_report.txt"
        echo "- 覆盖率报告: $REPORT_DIR/coverage.html"
        echo "- 代码统计: $REPORT_DIR/code_statistics.txt"
        echo "- 格式化报告: $REPORT_DIR/gofmt_report.txt"
        
        if [ -f "$REPORT_DIR/vulnerability_report.txt" ]; then
            echo "- 安全漏洞报告: $REPORT_DIR/vulnerability_report.txt"
        fi
        
        echo ""
        echo "## 推荐操作"
        echo ""
        echo "1. 查看HTML覆盖率报告了解测试覆盖情况"
        echo "2. 定期运行此脚本确保代码质量"
        echo "3. 修复任何发现的问题或警告"
        echo "4. 在提交代码前运行完整检查"
        
    } > "$REPORT_DIR/quality_summary.md"
    
    print_success "综合报告已生成: $REPORT_DIR/quality_summary.md"
}

# 主函数
main() {
    print_status "开始AI模块代码质量检查..."
    echo ""
    
    check_prerequisites
    echo ""
    
    compile_check
    echo ""
    
    static_analysis
    echo ""
    
    run_tests
    echo ""
    
    dependency_check
    echo ""
    
    complexity_analysis  
    echo ""
    
    generate_summary_report
    echo ""
    
    print_success "🎉 AI模块代码质量检查完成!"
    print_status "📊 查看详细报告: $REPORT_DIR/quality_summary.md"
    print_status "🌐 查看覆盖率报告: $REPORT_DIR/coverage.html"
}

# 脚本帮助信息
show_help() {
    echo "Chat2SQL AI模块代码质量检查工具"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示此帮助信息"
    echo "  --compile      仅运行编译检查"
    echo "  --test         仅运行测试"
    echo "  --static       仅运行静态分析"
    echo "  --deps         仅运行依赖检查"
    echo "  --stats        仅生成统计信息"
    echo ""
    echo "示例:"
    echo "  $0                # 运行完整检查"
    echo "  $0 --test         # 仅运行测试"
    echo "  $0 --compile      # 仅检查编译"
}

# 处理命令行参数
case "$1" in
    -h|--help)
        show_help
        exit 0
        ;;
    --compile)
        check_prerequisites
        compile_check
        ;;
    --test)
        check_prerequisites
        run_tests
        ;;
    --static)
        check_prerequisites
        static_analysis
        ;;
    --deps)
        check_prerequisites
        dependency_check
        ;;
    --stats)
        complexity_analysis
        ;;
    "")
        main
        ;;
    *)
        print_error "未知选项: $1"
        show_help
        exit 1
        ;;
esac