package agent

import (
	"context"
)

// Task 表示一个子任务
type Task struct {
	ID      int      `json:"id"`
	Task    string   `json:"task"`
	Tools   []string `json:"tools"`   // 预筛选的工具列表
	Deps    []int    `json:"deps"`    // 依赖的任务 ID
	Status  string   `json:"status"`  // pending, running, completed, failed
	Result  string   `json:"result"`
}

// Plan 表示执行计划
type Plan struct {
	Tasks    []Task `json:"tasks"`
	Original string `json:"original"` // 原始用户请求
}

// Tool 表示一个可用工具
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Execute     func(ctx context.Context, input map[string]any) (string, error)
}

// ReActStep 表示 ReAct 循环的一步
type ReActStep struct {
	Thought      string         `json:"thought"`
	Action       string         `json:"action"`       // 工具名称或 "final_answer"
	Action_input map[string]any `json:"action_input"` // 工具输入
	Observation  string         `json:"observation"`
}

// ReActResult 表示 ReAct 循环的结果
type ReActResult struct {
	Answer string      `json:"answer"`
	Steps  []ReActStep `json:"steps"`
}

// Planner 规划器接口
type Planner interface {
	// Plan 根据用户请求生成执行计划
	Plan(ctx context.Context, request string, tools []Tool) (*Plan, error)

	// RePlan 根据当前状态重新规划
	RePlan(ctx context.Context, request string, plan *Plan, completedTasks []Task, tools []Tool) (*Plan, error)
}

// Executor 执行器接口
type Executor interface {
	// ExecuteTask 执行单个任务
	ExecuteTask(ctx context.Context, task Task, tools []Tool) (*ReActResult, error)
}

// Agent 是主 Agent 结构
type Agent struct {
	Planner  Planner
	Executor Executor
	Tools    []Tool
	MaxRePlans int // 最大重新规划次数
}

// NewAgent 创建新的 Agent
func NewAgent(planner Planner, executor Executor, tools []Tool) *Agent {
	return &Agent{
		Planner:    planner,
		Executor:   executor,
		Tools:      tools,
		MaxRePlans: 3,
	}
}

// Run 执行用户请求
func (a *Agent) Run(ctx context.Context, request string) (string, error) {
	// 阶段 1: Plan - 生成执行计划
	plan, err := a.Planner.Plan(ctx, request, a.Tools)
	if err != nil {
		return "", err
	}

	// 阶段 2: Execute - 逐步执行
	completedTasks := make([]Task, 0)
	replanCount := 0

	for len(plan.Tasks) > 0 {
		// 找到下一个可执行的任务（依赖已完成）
		task, idx := a.findNextTask(plan.Tasks, completedTasks)
		if task == nil {
			break
		}

		// 预筛选工具
		taskTools := a.filterTools(task.Tools)

		// 使用 ReAct 循环执行任务
		result, err := a.Executor.ExecuteTask(ctx, *task, taskTools)
		if err != nil {
			task.Status = "failed"
			task.Result = err.Error()
		} else {
			task.Status = "completed"
			task.Result = result.Answer
		}

		completedTasks = append(completedTasks, *task)

		// 从计划中移除已完成的任务
		plan.Tasks = append(plan.Tasks[:idx], plan.Tasks[idx+1:]...)

		// 动态重新规划（如果还有任务未完成）
		if len(plan.Tasks) > 0 && replanCount < a.MaxRePlans {
			newPlan, err := a.Planner.RePlan(ctx, request, plan, completedTasks, a.Tools)
			if err == nil && newPlan != nil {
				plan = newPlan
				replanCount++
			}
		}
	}

	// 阶段 3: 汇总结果
	return a.summarizeResults(ctx, request, completedTasks)
}

// findNextTask 找到下一个可执行的任务
func (a *Agent) findNextTask(tasks []Task, completed []Task) (*Task, int) {
	completedMap := make(map[int]bool)
	for _, t := range completed {
		completedMap[t.ID] = true
	}

	for i, task := range tasks {
		if task.Status != "" && task.Status != "pending" {
			continue
		}

		// 检查依赖是否都已完成
		allDepsCompleted := true
		for _, dep := range task.Deps {
			if !completedMap[dep] {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			return &tasks[i], i
		}
	}

	return nil, -1
}

// filterTools 根据工具名称列表筛选工具
func (a *Agent) filterTools(toolNames []string) []Tool {
	toolMap := make(map[string]Tool)
	for _, t := range a.Tools {
		toolMap[t.Name] = t
	}

	result := make([]Tool, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			result = append(result, tool)
		}
	}

	return result
}

// summarizeResults 汇总所有任务结果
func (a *Agent) summarizeResults(ctx context.Context, request string, tasks []Task) (string, error) {
	// 如果只有一个任务，直接返回结果
	if len(tasks) == 1 {
		return tasks[0].Result, nil
	}

	// 多个任务，汇总结果
	summary := ""
	for _, task := range tasks {
		summary += task.Task + ": " + task.Result + "\n"
	}

	return summary, nil
}
