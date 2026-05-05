package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-platform/internal/platform/llm"
)

// LLMPlanner 使用 LLM 的规划器实现
type LLMPlanner struct {
	Provider *llm.OpenAIProvider
}

// NewLLMPlanner 创建 LLM 规划器
func NewLLMPlanner(provider *llm.OpenAIProvider) *LLMPlanner {
	return &LLMPlanner{
		Provider: provider,
	}
}

// planResponse 规划响应格式
type planResponse struct {
	Tasks []struct {
		ID    int      `json:"id"`
		Task  string   `json:"task"`
		Tools []string `json:"tools"`
		Deps  []int    `json:"deps"`
	} `json:"tasks"`
}

// Plan 根据用户请求生成执行计划
func (p *LLMPlanner) Plan(ctx context.Context, request string, tools []Tool) (*Plan, error) {
	// 构建工具描述
	toolDesc := p.buildToolDescription(tools)

	// 构建提示词
	prompt := fmt.Sprintf(`你是一个任务规划专家。根据用户的请求，分解为可执行的子任务。

可用工具列表：
%s

用户请求：
%s

要求：
1. 将请求分解为 2-5 个子任务
2. 为每个子任务选择 2-5 个最相关的工具
3. 如果任务之间有依赖关系，通过 deps 字段指定
4. 返回 JSON 格式的任务列表

输出格式（只返回 JSON，不要其他内容）：
{
  "tasks": [
    {"id": 1, "task": "任务描述", "tools": ["tool1", "tool2"], "deps": []},
    {"id": 2, "task": "任务描述", "tools": ["tool3"], "deps": [1]}
  ]
}`, toolDesc, request)

	messages := []llm.OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	resp, err := p.Provider.Chat(messages, nil)
	if err != nil {
		return nil, fmt.Errorf("plan failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no plan generated")
	}

	// 解析响应
	content := resp.Choices[0].Message.Content
	content = extractJSON(content)

	var planResp planResponse
	if err := json.Unmarshal([]byte(content), &planResp); err != nil {
		return nil, fmt.Errorf("parse plan failed: %w, content: %s", err, content)
	}

	// 转换为 Plan 结构
	plan := &Plan{
		Original: request,
		Tasks:    make([]Task, 0, len(planResp.Tasks)),
	}

	for _, t := range planResp.Tasks {
		plan.Tasks = append(plan.Tasks, Task{
			ID:     t.ID,
			Task:   t.Task,
			Tools:  t.Tools,
			Deps:   t.Deps,
			Status: "pending",
		})
	}

	return plan, nil
}

// RePlan 根据当前状态重新规划
func (p *LLMPlanner) RePlan(ctx context.Context, request string, plan *Plan, completedTasks []Task, tools []Tool) (*Plan, error) {
	toolDesc := p.buildToolDescription(tools)

	// 构建已完成任务的描述
	completedDesc := ""
	for _, t := range completedTasks {
		completedDesc += fmt.Sprintf("- 任务 %d (%s): %s\n", t.ID, t.Status, t.Result)
	}

	// 构建剩余任务的描述
	remainingDesc := ""
	for _, t := range plan.Tasks {
		remainingDesc += fmt.Sprintf("- 任务 %d: %s\n", t.ID, t.Task)
	}

	prompt := fmt.Sprintf(`你是一个任务规划专家。根据当前执行状态，重新评估和调整剩余任务。

原始请求：
%s

已完成的任务：
%s

剩余任务：
%s

可用工具：
%s

要求：
1. 评估已完成任务的结果
2. 决定是否需要调整剩余任务
3. 如果需要，可以添加、删除或修改任务
4. 返回调整后的任务列表（只包含剩余任务）

输出格式（只返回 JSON）：
{
  "tasks": [
    {"id": 1, "task": "任务描述", "tools": ["tool1"], "deps": []}
  ]
}`, request, completedDesc, remainingDesc, toolDesc)

	messages := []llm.OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	resp, err := p.Provider.Chat(messages, nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return plan, nil
	}

	content := resp.Choices[0].Message.Content
	content = extractJSON(content)

	var planResp planResponse
	if err := json.Unmarshal([]byte(content), &planResp); err != nil {
		return plan, nil
	}

	newPlan := &Plan{
		Original: request,
		Tasks:    make([]Task, 0, len(planResp.Tasks)),
	}

	for _, t := range planResp.Tasks {
		newPlan.Tasks = append(newPlan.Tasks, Task{
			ID:     t.ID,
			Task:   t.Task,
			Tools:  t.Tools,
			Deps:   t.Deps,
			Status: "pending",
		})
	}

	return newPlan, nil
}

// buildToolDescription 构建工具描述
func (p *LLMPlanner) buildToolDescription(tools []Tool) string {
	var sb strings.Builder
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}
	return sb.String()
}

// extractJSON 从响应中提取 JSON
func extractJSON(content string) string {
	// 尝试直接解析
	if json.Valid([]byte(content)) {
		return content
	}

	// 查找 JSON 块
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}

	return content
}
