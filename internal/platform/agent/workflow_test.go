package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"go-agent-platform/internal/platform/agent"
	"go-agent-platform/internal/platform/llm"
)

func TestWorkflowLogging(t *testing.T) {
	apiKey := "sk-b5ec6cbd1b6742e48c401bbd1372bc7a"
	baseURL := "https://api.deepseek.com"
	model := "deepseek-v4-flash"

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 定义工具集 - 模拟文件系统操作
	tools := []agent.Tool{
		{
			Name:        "list_files",
			Description: "列出目录下的所有文件，参数: directory (目录路径)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				directory, _ := input["directory"].(string)
				t.Logf("  [TOOL EXEC] list_files(directory=%s)", directory)
				
				// 模拟文件系统
				files := map[string][]string{
					"/home": {"documents", "pictures", "music"},
					"/home/documents": {"report.pdf", "notes.txt", "data.csv"},
					"/home/pictures": {"photo1.jpg", "screenshot.png"},
				}
				
				if result, ok := files[directory]; ok {
					return strings.Join(result, "\n"), nil
				}
				return "目录为空或不存在", nil
			},
		},
		{
			Name:        "read_file",
			Description: "读取文件内容，参数: path (文件路径)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				path, _ := input["path"].(string)
				t.Logf("  [TOOL EXEC] read_file(path=%s)", path)
				
				contents := map[string]string{
					"/home/documents/report.pdf": "这是2024年度销售报告，总销售额100万元",
					"/home/documents/notes.txt":  "待办事项：1. 完成报告 2. 发送邮件",
					"/home/documents/data.csv":   "月份,销售额\n1月,10万\n2月,15万",
				}
				
				if content, ok := contents[path]; ok {
					return content, nil
				}
				return "", fmt.Errorf("文件不存在: %s", path)
			},
		},
		{
			Name:        "search_files",
			Description: "搜索包含关键词的文件，参数: keyword (关键词), directory (搜索目录)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				keyword, _ := input["keyword"].(string)
				directory, _ := input["directory"].(string)
				t.Logf("  [TOOL EXEC] search_files(keyword=%s, directory=%s)", keyword, directory)
				
				// 模拟搜索
				results := []string{
					fmt.Sprintf("/home/documents/report.pdf: 包含 '%s'", keyword),
					fmt.Sprintf("/home/documents/notes.txt: 包含 '%s'", keyword),
				}
				return strings.Join(results, "\n"), nil
			},
		},
		{
			Name:        "summarize",
			Description: "总结文本内容，参数: text (要总结的文本)",
			Execute: func(ctx context.Context, input map[string]any) (string, error) {
				text, _ := input["text"].(string)
				t.Logf("  [TOOL EXEC] summarize(text=%s...)", text[:min(50, len(text))])
				return "总结: " + text[:min(100, len(text))] + "...", nil
			},
		},
	}

	// 创建 Planner 和 Executor
	planner := agent.NewLLMPlanner(provider)
	executor := agent.NewReActExecutor(provider)
	_ = agent.NewAgent(planner, executor, tools)

	// 测试模糊任务
	testCases := []struct {
		name    string
		request string
	}{
		{
			name:    "模糊文件查询",
			request: "帮我看看我有哪些文件",
		},
		{
			name:    "模糊数据分析",
			request: "分析一下我的文档",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("========================================")
			t.Logf("任务: %s", tc.request)
			t.Logf("========================================")

			ctx := context.Background()

			// 先测试规划
			t.Logf("\n[PHASE 1] Plan - 规划阶段")
			plan, err := planner.Plan(ctx, tc.request, tools)
			if err != nil {
				t.Fatalf("Plan failed: %v", err)
			}

			t.Logf("生成 %d 个子任务:", len(plan.Tasks))
			for _, task := range plan.Tasks {
				t.Logf("  任务 %d: %s", task.ID, task.Task)
				t.Logf("    预筛选工具: %v", task.Tools)
				t.Logf("    依赖: %v", task.Deps)
			}

			// 测试单个任务的 ReAct 执行
			if len(plan.Tasks) > 0 {
				t.Logf("\n[PHASE 2] Execute - 执行第一个任务")
				firstTask := plan.Tasks[0]
				
				// 获取预筛选的工具
				taskTools := filterTools(tools, firstTask.Tools)
				t.Logf("可用工具 (%d个): %v", len(taskTools), firstTask.Tools)

				result, err := executor.ExecuteTask(ctx, firstTask, taskTools)
				if err != nil {
					t.Fatalf("ExecuteTask failed: %v", err)
				}

				t.Logf("\n[RESULT] 执行结果:")
				t.Logf("  最终答案: %s", result.Answer)
				t.Logf("  ReAct 步骤数: %d", len(result.Steps))
				
				for i, step := range result.Steps {
					t.Logf("\n  --- Step %d ---", i+1)
					t.Logf("  Thought: %s", step.Thought)
					t.Logf("  Action: %s", step.Action)
					if step.Action_input != nil {
						inputJSON, _ := json.Marshal(step.Action_input)
						t.Logf("  Action Input: %s", string(inputJSON))
					}
					if step.Observation != "" {
						t.Logf("  Observation: %s", step.Observation)
					}
				}
			}
		})
	}
}

// filterTools 根据工具名称列表筛选工具
func filterTools(tools []agent.Tool, toolNames []string) []agent.Tool {
	toolMap := make(map[string]agent.Tool)
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	result := make([]agent.Tool, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			result = append(result, tool)
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
