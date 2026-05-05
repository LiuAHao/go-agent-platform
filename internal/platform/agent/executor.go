package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-platform/internal/platform/llm"
)

// ReActExecutor 使用 ReAct 循环的执行器
type ReActExecutor struct {
	Provider   *llm.OpenAIProvider
	MaxSteps   int // 最大 ReAct 步数
}

// NewReActExecutor 创建 ReAct 执行器
func NewReActExecutor(provider *llm.OpenAIProvider) *ReActExecutor {
	return &ReActExecutor{
		Provider: provider,
		MaxSteps: 10,
	}
}

// ExecuteTask 使用 ReAct 循环执行单个任务
func (e *ReActExecutor) ExecuteTask(ctx context.Context, task Task, tools []Tool) (*ReActResult, error) {
	// 构建工具描述
	toolDesc := buildToolDescription(tools)

	// 构建系统提示
	systemPrompt := fmt.Sprintf(`你是一个智能助手，正在执行一个任务。使用 ReAct（思考-行动-观察）循环来完成任务。

当前任务：%s

可用工具：
%s

规则：
1. 先思考（Thought）：分析当前状态，决定下一步
2. 选择行动（Action）：选择一个工具调用，或直接给出最终答案
3. 观察结果（Observation）：根据工具返回结果继续思考

输出格式（每一步都返回 JSON）：
{
  "thought": "你的思考过程",
  "action": "工具名称或 final_answer",
  "action_input": {"参数名": "参数值"}
}

当任务完成时，使用：
{
  "thought": "任务已完成",
  "action": "final_answer",
  "action_input": {"answer": "最终答案"}
}`, task.Task, toolDesc)

	messages := []llm.OpenAIMessage{
		{Role: "system", Content: systemPrompt},
	}

	result := &ReActResult{
		Steps: make([]ReActStep, 0),
	}

	// ReAct 循环
	for step := 0; step < e.MaxSteps; step++ {
		// 调用 LLM
		resp, err := e.Provider.Chat(messages, nil)
		if err != nil {
			return nil, fmt.Errorf("react step %d failed: %w", step, err)
		}

		if len(resp.Choices) == 0 {
			break
		}

		content := resp.Choices[0].Message.Content

		// 解析响应
		var reactStep ReActStep
		if err := parseReActResponse(content, &reactStep); err != nil {
			// 解析失败，尝试直接作为最终答案
			result.Answer = content
			break
		}

		// 检查是否是最终答案
		if reactStep.Action == "final_answer" {
			if answer, ok := reactStep.Action_input["answer"].(string); ok {
				result.Answer = answer
			} else {
				result.Answer = fmt.Sprintf("%v", reactStep.Action_input)
			}
			result.Steps = append(result.Steps, reactStep)
			break
		}

		// 执行工具
		observation := e.executeTool(ctx, tools, reactStep.Action, reactStep.Action_input)
		reactStep.Observation = observation

		result.Steps = append(result.Steps, reactStep)

		// 添加到消息历史
		messages = append(messages, llm.OpenAIMessage{
			Role:    "assistant",
			Content: content,
		})
		messages = append(messages, llm.OpenAIMessage{
			Role:    "user",
			Content: fmt.Sprintf("Observation: %s", observation),
		})
	}

	if result.Answer == "" {
		result.Answer = "任务执行完成，但未能生成明确答案"
	}

	return result, nil
}

// executeTool 执行工具
func (e *ReActExecutor) executeTool(ctx context.Context, tools []Tool, toolName string, input map[string]any) string {
	// 查找工具
	for _, tool := range tools {
		if tool.Name == toolName {
			output, err := tool.Execute(ctx, input)
			if err != nil {
				return fmt.Sprintf("工具执行错误: %v", err)
			}
			return output
		}
	}

	return fmt.Sprintf("未找到工具: %s", toolName)
}

// buildToolDescription 构建工具描述
func buildToolDescription(tools []Tool) string {
	var sb strings.Builder
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}
	return sb.String()
}

// parseReActResponse 解析 ReAct 响应
func parseReActResponse(content string, step *ReActStep) error {
	// 清理内容
	content = strings.TrimSpace(content)

	// 提取 JSON
	jsonStr := extractJSONFromResponse(content)

	// 解析 JSON
	var parsed struct {
		Thought     string         `json:"thought"`
		Action      string         `json:"action"`
		ActionInput map[string]any `json:"action_input"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}

	step.Thought = parsed.Thought
	step.Action = parsed.Action
	step.Action_input = parsed.ActionInput

	return nil
}

// extractJSONFromResponse 从响应中提取 JSON
func extractJSONFromResponse(content string) string {
	// 查找 JSON 块
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	return content
}
