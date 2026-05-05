package agent_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go-agent-platform/internal/platform/agent"
	"go-agent-platform/internal/platform/llm"
)

func TestAgentFramework(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 定义测试工具
	tools := []agent.Tool{
		{
			Name:        "read_file",
			Description: "读取文件内容",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				path, _ := input["path"].(string)
				return fmt.Sprintf("文件内容 of %s: 这是一个测试文件", path), nil
			},
		},
		{
			Name:        "list_directory",
			Description: "列出目录内容",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				path, _ := input["path"].(string)
				return fmt.Sprintf("目录 %s 包含: file1.txt, file2.py, dir1/", path), nil
			},
		},
		{
			Name:        "write_file",
			Description: "写入文件",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				path, _ := input["path"].(string)
				content, _ := input["content"].(string)
				return fmt.Sprintf("已写入文件 %s, 内容长度: %d", path, len(content)), nil
			},
		},
	}

	// 创建 Planner 和 Executor
	planner := agent.NewLLMPlanner(provider)
	executor := agent.NewReActExecutor(provider)

	// 创建 Agent
	ag := agent.NewAgent(planner, executor, tools)

	// 测试简单任务
	t.Run("SimpleTask", func(t *testing.T) {
		ctx := context.Background()
		result, err := ag.Run(ctx, "请列出 /tmp 目录的内容")
		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		t.Logf("Agent result: %s", result)
	})

	// 测试多步骤任务
	t.Run("MultiStepTask", func(t *testing.T) {
		ctx := context.Background()
		result, err := ag.Run(ctx, "请读取 /tmp/test.txt 文件，然后将内容写入 /tmp/backup.txt")
		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		t.Logf("Agent result: %s", result)
	})
}

func TestPlanner(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)
	planner := agent.NewLLMPlanner(provider)

	tools := []agent.Tool{
		{Name: "read_file", Description: "读取文件内容"},
		{Name: "list_directory", Description: "列出目录内容"},
		{Name: "write_file", Description: "写入文件"},
		{Name: "search_files", Description: "搜索文件"},
	}

	ctx := context.Background()
	plan, err := planner.Plan(ctx, "请帮我分析项目结构，找出所有的 Python 文件", tools)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	t.Logf("Plan generated with %d tasks:", len(plan.Tasks))
	for _, task := range plan.Tasks {
		t.Logf("  Task %d: %s (tools: %v, deps: %v)", task.ID, task.Task, task.Tools, task.Deps)
	}
}

func TestReActExecutor(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}
	baseURL := envOrDefault("LLM_BASE_URL", "https://api.deepseek.com")
	model := envOrDefault("LLM_MODEL", "deepseek-v4-flash")

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)
	executor := agent.NewReActExecutor(provider)

	tools := []agent.Tool{
		{
			Name:        "calculator",
			Description: "计算数学表达式",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				expr, _ := input["expression"].(string)
				return fmt.Sprintf("计算结果: %s = 42", expr), nil
			},
		},
	}

	task := agent.Task{
		ID:   1,
		Task: "计算 2+2 的结果",
	}

	ctx := context.Background()
	result, err := executor.ExecuteTask(ctx, task, tools)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	t.Logf("ReAct result: %s", result.Answer)
	t.Logf("Steps taken: %d", len(result.Steps))
	for i, step := range result.Steps {
		t.Logf("  Step %d: Thought=%s, Action=%s", i+1, step.Thought, step.Action)
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
