package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"go-agent-platform/internal/platform/agent"
	"go-agent-platform/internal/platform/llm"
)

func TestSimpleToolCall(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 简单工具集
	tools := []agent.Tool{
		{
			Name:        "calculator",
			Description: "计算数学表达式，参数: expression (数学表达式)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				expr, _ := input["expression"].(string)
				// 简单计算
				if expr == "2+3" {
					return "5", nil
				}
				if expr == "10*5" {
					return "50", nil
				}
				return fmt.Sprintf("计算结果: %s", expr), nil
			},
		},
		{
			Name:        "string_length",
			Description: "计算字符串长度，参数: text (输入文本)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				text, _ := input["text"].(string)
				return fmt.Sprintf("字符串长度: %d", len(text)), nil
			},
		},
	}

	planner := agent.NewLLMPlanner(provider)
	executor := agent.NewReActExecutor(provider)
	ag := agent.NewAgent(planner, executor, tools)

	t.Run("SimpleCalculation", func(t *testing.T) {
		ctx := context.Background()
		result, err := ag.Run(ctx, "请计算 2+3 的结果")
		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}
		t.Logf("结果: %s", result)
	})
}

func TestTwoStepTask(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	tools := []agent.Tool{
		{
			Name:        "read_file",
			Description: "读取文件内容，参数: path (文件路径)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				path, _ := input["path"].(string)
				if path == "/data.txt" {
					return "Hello World", nil
				}
				return "", fmt.Errorf("文件不存在: %s", path)
			},
		},
		{
			Name:        "count_chars",
			Description: "统计字符数，参数: text (输入文本)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				text, _ := input["text"].(string)
				return fmt.Sprintf("字符数: %d", len(text)), nil
			},
		},
	}

	planner := agent.NewLLMPlanner(provider)
	executor := agent.NewReActExecutor(provider)
	ag := agent.NewAgent(planner, executor, tools)

	t.Run("ReadAndCount", func(t *testing.T) {
		ctx := context.Background()
		result, err := ag.Run(ctx, "请读取 /data.txt 文件，然后统计其字符数")
		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}
		t.Logf("结果: %s", result)
	})
}

func TestToolSelection(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 多个工具，测试预筛选
	tools := []agent.Tool{
		{Name: "read_file", Description: "读取文件", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "file content", nil }},
		{Name: "write_file", Description: "写入文件", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "written", nil }},
		{Name: "list_dir", Description: "列出目录", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "dir listing", nil }},
		{Name: "search", Description: "搜索", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "search results", nil }},
		{Name: "analyze", Description: "分析", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "analysis", nil }},
		{Name: "report", Description: "生成报告", Execute: func(ctx context.Context, input map[string]any) (string, error) { return "report", nil }},
	}

	planner := agent.NewLLMPlanner(provider)
	// executor := agent.NewReActExecutor(provider)
	// ag := agent.NewAgent(planner, nil, tools)

	t.Run("ToolSelection", func(t *testing.T) {
		ctx := context.Background()

		// 先测试规划器的工具预筛选
		plan, err := planner.Plan(ctx, "请读取文件并分析内容", tools)
		if err != nil {
			t.Fatalf("Plan failed: %v", err)
		}

		t.Logf("=== 执行计划 ===")
		for _, task := range plan.Tasks {
			t.Logf("任务 %d: %s", task.ID, task.Task)
			t.Logf("  预筛选工具: %v", task.Tools)
			t.Logf("  依赖: %v", task.Deps)
		}
	})
}

func TestReActLoop(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	tools := []agent.Tool{
		{
			Name:        "get_weather",
			Description: "获取天气信息，参数: city (城市名称)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				city, _ := input["city"].(string)
				weather := map[string]any{
					"city":      city,
					"temp":      "25°C",
					"condition": "晴朗",
				}
				jsonData, _ := json.Marshal(weather)
				return string(jsonData), nil
			},
		},
	}

	executor := agent.NewReActExecutor(provider)

	task := agent.Task{
		ID:   1,
		Task: "获取北京的天气信息",
	}

	ctx := context.Background()
	result, err := executor.ExecuteTask(ctx, task, tools)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	t.Logf("=== ReAct 执行结果 ===")
	t.Logf("最终答案: %s", result.Answer)
	t.Logf("执行步骤: %d", len(result.Steps))
	for i, step := range result.Steps {
		t.Logf("步骤 %d:", i+1)
		t.Logf("  思考: %s", step.Thought)
		t.Logf("  行动: %s", step.Action)
		if step.Action_input != nil {
			t.Logf("  输入: %v", step.Action_input)
		}
		if step.Observation != "" {
			t.Logf("  观察: %s", step.Observation)
		}
	}
}

func TestPlannerToolFiltering(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)
	planner := agent.NewLLMPlanner(provider)

	// 大量工具
	tools := []agent.Tool{
		{Name: "read_file", Description: "读取文件内容"},
		{Name: "write_file", Description: "写入文件"},
		{Name: "delete_file", Description: "删除文件"},
		{Name: "list_dir", Description: "列出目录"},
		{Name: "create_dir", Description: "创建目录"},
		{Name: "search_files", Description: "搜索文件"},
		{Name: "copy_file", Description: "复制文件"},
		{Name: "move_file", Description: "移动文件"},
		{Name: "get_weather", Description: "获取天气"},
		{Name: "send_email", Description: "发送邮件"},
		{Name: "calculate", Description: "计算"},
		{Name: "translate", Description: "翻译"},
		{Name: "summarize", Description: "总结"},
		{Name: "analyze", Description: "分析"},
		{Name: "generate_report", Description: "生成报告"},
	}

	ctx := context.Background()

	// 测试不同任务的工具预筛选
	testCases := []struct {
		name    string
		request string
	}{
		{"文件操作", "请读取 config.json 文件"},
		{"数据分析", "请分析数据并生成报告"},
		{"天气查询", "请获取上海的天气"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planner.Plan(ctx, tc.request, tools)
			if err != nil {
				t.Fatalf("Plan failed: %v", err)
			}

			t.Logf("请求: %s", tc.request)
			for _, task := range plan.Tasks {
				t.Logf("  任务: %s", task.Task)
				t.Logf("  预筛选工具 (%d个): %v", len(task.Tools), task.Tools)

				// 验证工具数量在合理范围内
				if len(task.Tools) > 5 {
					t.Logf("  警告: 工具数量过多 (%d)，建议限制在 2-5 个", len(task.Tools))
				}
			}
		})
	}
}

func TestStringOperations(t *testing.T) {
	// 测试字符串处理功能
	testCases := []struct {
		input    string
		expected int
	}{
		{"Hello", 5},
		{"Hello World", 11},
		{"你好世界", 12}, // UTF-8 编码
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("len_%s", tc.input), func(t *testing.T) {
			result := len(tc.input)
			t.Logf("字符串 '%s' 长度: %d", tc.input, result)
		})
	}

	// 测试 JSON 处理
	t.Run("JSONProcessing", func(t *testing.T) {
		data := map[string]any{
			"name":  "test",
			"value": 42,
			"items": []string{"a", "b", "c"},
		}

		jsonBytes, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("JSON marshal failed: %v", err)
		}

		t.Logf("JSON 输出: %s", string(jsonBytes))

		// 解析 JSON
		var parsed map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("JSON unmarshal failed: %v", err)
		}

		t.Logf("解析结果: %v", parsed)
	})

	// 测试字符串处理
	t.Run("StringProcessing", func(t *testing.T) {
		text := "  Hello, World!  "
		t.Logf("原始: '%s'", text)
		t.Logf("去空格: '%s'", strings.TrimSpace(text))
		t.Logf("大写: '%s'", strings.ToUpper(text))
		t.Logf("包含 World: %v", strings.Contains(text, "World"))
		t.Logf("替换: '%s'", strings.ReplaceAll(text, "World", "Go"))
	})
}
