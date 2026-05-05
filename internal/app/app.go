package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/approval"
	"go-agent-platform/internal/domain/audit"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/shared"
	"go-agent-platform/internal/domain/task"
	"go-agent-platform/internal/domain/tool"
	"go-agent-platform/internal/domain/workspace"
	"go-agent-platform/internal/platform/events"
	"go-agent-platform/internal/platform/llm"
	"go-agent-platform/internal/platform/mcp"
	"go-agent-platform/internal/platform/memory"
	mysqlstore "go-agent-platform/internal/platform/mysql"
	"go-agent-platform/internal/platform/postgres"
)

type Application struct {
	Config     config.Config
	Store      Store
	Events     *events.Hub
	Provider   llm.Provider
	MCPManager *mcp.Manager
}

func New(cfg config.Config) (*Application, error) {
	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		_ = store.Close()
		return nil, err
	}

	// 创建 LLM Provider
	provider := createLLMProvider(cfg)

	return &Application{
		Config:     cfg,
		Store:      store,
		Events:     events.NewHub(),
		Provider:   provider,
		MCPManager: mcp.NewManager(),
	}, nil
}

func createLLMProvider(cfg config.Config) llm.Provider {
	// 如果配置了 API Key，使用真实的 LLM Provider
	if cfg.LLMAPIKey != "" {
		return llm.NewOpenAIProvider(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel)
	}
	// 否则使用 Mock Provider
	return llm.MockProvider{}
}

func newStore(cfg config.Config) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.StorageDriver)) {
	case "", "memory":
		return memory.NewStore(), nil
	case "postgres":
		return postgres.New(cfg)
	case "mysql":
		return mysqlstore.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.StorageDriver)
	}
}

func (a *Application) Close() error {
	if a.MCPManager != nil {
		a.MCPManager.Close()
	}
	if a.Store == nil {
		return nil
	}
	return a.Store.Close()
}

func (a *Application) Login(email, password string) (map[string]any, error) {
	user, err := a.Store.FindUserByEmail(email)
	if err != nil || user.PasswordHash != shared.HashPassword(password) {
		return nil, errors.New("invalid credentials")
	}

	token := auth.SessionToken{
		Token:     shared.NewID("token"),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := a.Store.SaveSessionToken(token); err != nil {
		return nil, err
	}
	a.appendAudit("", user.ID, "auth.login", "user", user.ID, nil)
	return map[string]any{"token": token.Token, "user": user}, nil
}

func (a *Application) Authenticate(token string) (auth.User, error) {
	sessionToken, err := a.Store.FindSessionToken(token)
	if err != nil || sessionToken.ExpiresAt.Before(time.Now()) {
		return auth.User{}, errors.New("unauthorized")
	}
	user, err := a.Store.FindUserByID(sessionToken.UserID)
	if err != nil {
		return auth.User{}, errors.New("unauthorized")
	}
	return user, nil
}

func (a *Application) ListWorkspaces(userID string) []workspace.Workspace {
	items, err := a.Store.ListWorkspacesByUser(userID)
	if err != nil {
		return []workspace.Workspace{}
	}
	return items
}

func (a *Application) CreateWorkspace(userID, name string) workspace.Workspace {
	now := time.Now().UTC()
	item := workspace.Workspace{
		ID:        shared.NewID("ws"),
		Name:      name,
		CreatedBy: userID,
		CreatedAt: now,
	}
	member := workspace.Membership{
		ID:          shared.NewID("member"),
		WorkspaceID: item.ID,
		UserID:      userID,
		Role:        workspace.RoleOwner,
		CreatedAt:   now,
	}
	_ = a.Store.CreateWorkspace(item, member)
	a.appendAudit(item.ID, userID, "workspace.create", "workspace", item.ID, map[string]any{"name": name})
	return item
}

func (a *Application) CreateAgent(userID string, req CreateAgentRequest) (agent.Agent, error) {
	workspaceID, err := a.resolveWorkspaceID(userID, req.WorkspaceID)
	if err != nil {
		return agent.Agent{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return agent.Agent{}, errors.New("workspace access denied")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Model = strings.TrimSpace(req.Model)
	if req.Name == "" {
		return agent.Agent{}, errors.New("agent name is required")
	}
	if req.Model == "" {
		return agent.Agent{}, errors.New("model is required")
	}
	if _, err := a.Store.FindModelByKey(workspaceID, req.Model); err != nil {
		return agent.Agent{}, errors.New("model not found in workspace registry")
	}
	if err := a.validateSkillPolicy(userID, req.SkillPolicy); err != nil {
		return agent.Agent{}, err
	}
	if err := a.validateToolPolicy(userID, req.ToolPolicy); err != nil {
		return agent.Agent{}, err
	}
	now := time.Now().UTC()
	item := agent.Agent{
		ID:            shared.NewID("agent"),
		WorkspaceID:   workspaceID,
		Name:          req.Name,
		Description:   req.Description,
		SystemPrompt:  req.SystemPrompt,
		Model:         req.Model,
		SkillPolicy:   req.SkillPolicy,
		ToolPolicy:    req.ToolPolicy,
		RuntimePolicy: req.RuntimePolicy,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := a.Store.SaveAgent(item); err != nil {
		return agent.Agent{}, err
	}
	a.appendAudit(workspaceID, userID, "agent.create", "agent", item.ID, map[string]any{"name": item.Name})
	return item, nil
}

func (a *Application) ListAgents(workspaceID string) []agent.Agent {
	items, err := a.Store.ListAgents(workspaceID)
	if err != nil {
		return []agent.Agent{}
	}
	return items
}

func (a *Application) CreateVersion(userID, agentID string) (agent.Version, error) {
	current, err := a.Store.FindAgentByID(agentID)
	if err != nil {
		return agent.Version{}, errors.New("agent not found")
	}
	count, err := a.Store.CountAgentVersions(agentID)
	if err != nil {
		return agent.Version{}, err
	}
	snapshot := map[string]any{
		"name":           current.Name,
		"description":    current.Description,
		"system_prompt":  current.SystemPrompt,
		"model":          current.Model,
		"skill_policy":   current.SkillPolicy,
		"tool_policy":    current.ToolPolicy,
		"runtime_policy": current.RuntimePolicy,
	}
	version := agent.Version{
		ID:            shared.NewID("agentver"),
		AgentID:       agentID,
		VersionNumber: count + 1,
		Snapshot:      snapshot,
		CreatedBy:     userID,
		CreatedAt:     time.Now().UTC(),
	}
	if err := a.Store.SaveAgentVersion(version); err != nil {
		return agent.Version{}, err
	}
	a.appendAudit(current.WorkspaceID, userID, "agent.version.create", "agent_version", version.ID, map[string]any{"agent_id": agentID})
	return version, nil
}

func (a *Application) PublishVersion(userID, agentID, versionID string) error {
	current, err := a.Store.FindAgentByID(agentID)
	if err != nil {
		return errors.New("agent or version not found")
	}
	version, err := a.Store.FindAgentVersion(versionID)
	if err != nil || version.AgentID != agentID {
		return errors.New("agent or version not found")
	}
	current.PublishedVerID = versionID
	current.UpdatedAt = time.Now().UTC()
	if err := a.Store.UpdateAgent(current); err != nil {
		return err
	}
	a.appendAudit(current.WorkspaceID, userID, "agent.publish", "agent", agentID, map[string]any{"version_id": versionID})
	return nil
}

func (a *Application) CreateTool(userID string, req CreateToolRequest) (tool.Tool, error) {
	workspaceID, err := a.resolveWorkspaceID(userID, req.WorkspaceID)
	if err != nil {
		return tool.Tool{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return tool.Tool{}, errors.New("workspace access denied")
	}
	scope := normalizeToolScope(req.Scope)
	if scope == tool.ScopePlatform && !a.isPlatformAdmin(userID) {
		return tool.Tool{}, errors.New("only platform admin can create platform mcp")
	}
	item := tool.Tool{
		ID:               shared.NewID("tool"),
		WorkspaceID:      workspaceID,
		Name:             req.Name,
		Scope:            scope,
		Description:      req.Description,
		Schema:           req.Schema,
		Config:           req.Config,
		RequiresApproval: req.RequiresApproval,
		Enabled:          true,
		CreatedBy:        userID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := a.Store.SaveTool(item); err != nil {
		return tool.Tool{}, err
	}
	a.appendAudit(workspaceID, userID, "tool.create", "tool", item.ID, map[string]any{"name": item.Name, "scope": item.Scope})
	return item, nil
}

func (a *Application) ListTools(workspaceID string) []tool.Tool {
	items, err := a.Store.ListTools(workspaceID)
	if err != nil {
		return []tool.Tool{}
	}
	return items
}

func (a *Application) CreateSession(userID string, req CreateSessionRequest) (session.Session, error) {
	workspaceID, err := a.resolveAgentWorkspace(userID, req.WorkspaceID, req.AgentID)
	if err != nil {
		return session.Session{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return session.Session{}, errors.New("workspace access denied")
	}
	if item, err := a.Store.FindAgentByID(req.AgentID); err != nil || item.WorkspaceID != workspaceID {
		return session.Session{}, errors.New("agent not found")
	}
	item := session.Session{
		ID:          shared.NewID("session"),
		WorkspaceID: workspaceID,
		AgentID:     req.AgentID,
		CreatedBy:   userID,
		Title:       req.Title,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := a.Store.SaveSession(item); err != nil {
		return session.Session{}, err
	}
	a.appendAudit(workspaceID, userID, "session.create", "session", item.ID, nil)
	return item, nil
}

func (a *Application) ListMessages(sessionID string) []session.Message {
	items, err := a.Store.ListMessages(sessionID)
	if err != nil {
		return []session.Message{}
	}
	return items
}

func (a *Application) CreateSchedule(userID string, req CreateScheduleRequest) (task.Schedule, error) {
	workspaceID, err := a.resolveWorkspaceID(userID, req.WorkspaceID)
	if err != nil {
		return task.Schedule{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return task.Schedule{}, errors.New("workspace access denied")
	}
	nextRunAt, interval, err := parseSchedule(req.Cron)
	if err != nil {
		return task.Schedule{}, err
	}
	item := task.Schedule{
		ID:          shared.NewID("schedule"),
		WorkspaceID: workspaceID,
		AgentID:     req.AgentID,
		Name:        req.Name,
		Prompt:      req.Prompt,
		Cron:        req.Cron,
		Interval:    interval.String(),
		NextRunAt:   nextRunAt,
		CreatedBy:   userID,
		CreatedAt:   time.Now().UTC(),
	}
	if err := a.Store.SaveSchedule(item); err != nil {
		return task.Schedule{}, err
	}
	a.appendAudit(workspaceID, userID, "schedule.create", "schedule", item.ID, map[string]any{"cron": req.Cron})
	return item, nil
}

func parseSchedule(expr string) (time.Time, time.Duration, error) {
	if !strings.HasPrefix(expr, "@every ") {
		return time.Time{}, 0, errors.New("only '@every <duration>' schedules are supported in v1")
	}
	d, err := time.ParseDuration(strings.TrimPrefix(expr, "@every "))
	if err != nil || d <= 0 {
		return time.Time{}, 0, errors.New("invalid duration")
	}
	return time.Now().UTC().Add(d), d, nil
}

func (a *Application) RunDueSchedules() {
	now := time.Now().UTC()
	due, err := a.Store.ListDueSchedules(now)
	if err != nil {
		return
	}
	for _, schedule := range due {
		_, _ = a.ExecuteTask(schedule.CreatedBy, ExecuteTaskRequest{
			WorkspaceID: schedule.WorkspaceID,
			AgentID:     schedule.AgentID,
			Prompt:      schedule.Prompt,
			Async:       true,
		})
		dur, err := time.ParseDuration(schedule.Interval)
		if err != nil {
			continue
		}
		schedule.NextRunAt = time.Now().UTC().Add(dur)
		_ = a.Store.UpdateSchedule(schedule)
	}
}

func (a *Application) ExecuteTask(userID string, req ExecuteTaskRequest) (task.Task, error) {
	workspaceID, err := a.resolveAgentWorkspace(userID, req.WorkspaceID, req.AgentID)
	if err != nil {
		return task.Task{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return task.Task{}, errors.New("workspace access denied")
	}
	agentRecord, err := a.Store.FindAgentByID(req.AgentID)
	if err != nil {
		return task.Task{}, errors.New("agent not found")
	}
	if agentRecord.PublishedVerID == "" {
		return task.Task{}, errors.New("agent has no published version")
	}

	now := time.Now().UTC()
	sessionID := req.SessionID
	if sessionID == "" {
		created, err := a.CreateSession(userID, CreateSessionRequest{
			WorkspaceID: workspaceID,
			AgentID:     req.AgentID,
			Title:       firstNonEmpty(req.SessionTitle, req.Prompt),
		})
		if err != nil {
			return task.Task{}, err
		}
		sessionID = created.ID
	}

	job := task.Task{
		ID:          shared.NewID("task"),
		TraceID:     shared.NewID("trace"),
		WorkspaceID: workspaceID,
		AgentID:     req.AgentID,
		SessionID:   sessionID,
		Model:       firstNonEmpty(strings.TrimSpace(req.Model), agentRecord.Model),
		Reasoning:   firstNonEmpty(strings.TrimSpace(req.Reasoning), "standard"),
		Prompt:      req.Prompt,
		Status:      task.StatusPending,
		CreatedBy:   userID,
		ToolCalls:   req.ToolCalls,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := a.Store.SaveTask(job); err != nil {
		return task.Task{}, err
	}
	if err := a.Store.SaveMessage(session.Message{
		ID:        shared.NewID("msg"),
		SessionID: sessionID,
		Role:      session.RoleUser,
		Content:   req.Prompt,
		TraceID:   job.TraceID,
		CreatedAt: now,
	}); err != nil {
		return task.Task{}, err
	}

	a.appendAudit(workspaceID, userID, "task.create", "task", job.ID, map[string]any{"agent_id": req.AgentID, "model": job.Model, "reasoning": job.Reasoning})
	if req.Async {
		go a.runTask(job.ID)
		return job, nil
	}
	a.runTask(job.ID)
	return a.GetTask(job.ID)
}

func (a *Application) runTask(taskID string) {
	job, err := a.GetTask(taskID)
	if err != nil {
		return
	}
	job.Status = task.StatusRunning
	job.UpdatedAt = time.Now().UTC()
	_ = a.Store.UpdateTask(job)
	a.publishEvent("task.started", job.ID, job.TraceID, map[string]any{"task_id": job.ID})

	seq := 1
	for _, name := range a.Provider.Plan(job.Prompt) {
		step := task.Step{
			ID:        shared.NewID("step"),
			TaskID:    job.ID,
			Name:      name,
			Status:    task.StepCompleted,
			Output:    "planner completed",
			Sequence:  seq,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		_ = a.Store.SaveTaskStep(step)
		a.publishEvent("task.step.completed", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "step_name": name})
		seq++
	}

	toolOutputs := make([]string, 0, len(job.ToolCalls))
	for _, call := range job.ToolCalls {
		t, err := a.Store.FindToolByID(call.ToolID)
		if err != nil || !t.Enabled {
			a.failTask(job.ID, fmt.Sprintf("tool %s not available", call.ToolID))
			return
		}
		step := task.Step{
			ID:        shared.NewID("step"),
			TaskID:    job.ID,
			Name:      "tool:" + t.Name,
			Status:    task.StepRunning,
			Sequence:  seq,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		seq++
		a.publishEvent("task.step.started", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "step_name": step.Name})

		if t.RequiresApproval {
			step.Status = task.StepWaiting
			appr := approval.Approval{
				ID:          shared.NewID("approval"),
				WorkspaceID: job.WorkspaceID,
				TaskID:      job.ID,
				StepID:      step.ID,
				Reason:      "high risk tool requires approval: " + t.Name,
				Status:      approval.StatusPending,
				CreatedAt:   time.Now().UTC(),
			}
			step.ApprovalRef = appr.ID
			job.Status = task.StatusWaitingApproval
			job.ApprovalID = appr.ID
			job.UpdatedAt = time.Now().UTC()

			_ = a.Store.SaveTaskStep(step)
			_ = a.Store.SaveApproval(appr)
			_ = a.Store.UpdateTask(job)

			a.appendAudit(job.WorkspaceID, job.CreatedBy, "approval.create", "approval", appr.ID, map[string]any{"tool_id": t.ID, "task_id": job.ID})
			a.publishEvent("task.waiting_approval", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "approval_id": appr.ID, "reason": appr.Reason})
			return
		}

		output, outputJSON := a.executeTool(t, call.Input)
		step.Status = task.StepCompleted
		step.Output = output
		step.UpdatedAt = time.Now().UTC()
		toolOutputs = append(toolOutputs, output)
		_ = a.Store.SaveTaskStep(step)
		a.publishEvent("task.stream.delta", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "delta": outputJSON})
	}

	// 使用 LLM 生成最终结果
	result := a.completeWithLLM(job)
	job.Status = task.StatusCompleted
	job.Result = result
	job.UpdatedAt = time.Now().UTC()
	_ = a.Store.UpdateTask(job)
	_ = a.Store.SaveMessage(session.Message{
		ID:        shared.NewID("msg"),
		SessionID: job.SessionID,
		Role:      session.RoleAssistant,
		Content:   result,
		TraceID:   job.TraceID,
		CreatedAt: time.Now().UTC(),
	})
	a.publishEvent("task.completed", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "result": result})
}

// completeWithLLM 使用 LLM 生成最终结果
func (a *Application) completeWithLLM(job task.Task) string {
	// 检查是否是 OpenAI Provider
	if openaiProvider, ok := a.Provider.(*llm.OpenAIProvider); ok {
		// 构建消息
		messages := []llm.OpenAIMessage{
			{
				Role:    "system",
				Content: "你是一个智能助手。根据用户的请求和工具执行结果，生成最终的回答。请用中文回复。",
			},
			{
				Role:    "user",
				Content: job.Prompt,
			},
		}

		// 添加工具执行结果
		if len(job.ToolCalls) > 0 {
			toolContext := "工具执行结果:\n"
			for i, call := range job.ToolCalls {
				tool, err := a.Store.FindToolByID(call.ToolID)
				if err == nil {
					toolContext += fmt.Sprintf("%d. %s: 已执行\n", i+1, tool.Name)
				}
			}
			messages = append(messages, llm.OpenAIMessage{
				Role:    "user",
				Content: toolContext,
			})
		}

		// 调用 LLM
		resp, err := openaiProvider.Chat(messages, nil)
		if err != nil {
			return fmt.Sprintf("任务完成，但生成回复时出错: %v", err)
		}

		if len(resp.Choices) > 0 {
			return resp.Choices[0].Message.Content
		}

		return "任务完成"
	}

	// 降级到 Mock Provider
	toolOutputs := make([]string, 0)
	for _, call := range job.ToolCalls {
		tool, err := a.Store.FindToolByID(call.ToolID)
		if err == nil {
			output, _ := a.executeTool(tool, call.Input)
			toolOutputs = append(toolOutputs, output)
		}
	}
	return a.Provider.Complete(job.Prompt, toolOutputs)
}

func (a *Application) Approve(userID, approvalID string, approved bool) (approval.Approval, error) {
	item, err := a.Store.FindApprovalByID(approvalID)
	if err != nil {
		return approval.Approval{}, errors.New("approval not found")
	}
	if item.Status != approval.StatusPending {
		return approval.Approval{}, errors.New("approval already decided")
	}
	if approved {
		item.Status = approval.StatusApproved
	} else {
		item.Status = approval.StatusRejected
	}
	item.DecisionBy = userID
	item.DecisionAt = time.Now().UTC()
	if err := a.Store.UpdateApproval(item); err != nil {
		return approval.Approval{}, err
	}
	if approved {
		a.resumeApprovedTask(item.TaskID, item.StepID)
	} else {
		a.failTask(item.TaskID, "approval rejected")
	}
	a.appendAudit(item.WorkspaceID, userID, "approval.decide", "approval", item.ID, map[string]any{"approved": approved})
	return item, nil
}

func (a *Application) resumeApprovedTask(taskID, stepID string) {
	job, err := a.GetTask(taskID)
	if err != nil {
		return
	}
	steps, err := a.Store.ListTaskSteps(job.ID)
	if err != nil {
		return
	}
	for idx := range steps {
		if steps[idx].ID == stepID {
			steps[idx].Status = task.StepCompleted
			steps[idx].Output = "approval granted and tool executed"
			steps[idx].UpdatedAt = time.Now().UTC()
		}
	}
	job.Status = task.StatusCompleted
	job.Result = "task resumed after approval"
	job.UpdatedAt = time.Now().UTC()
	_ = a.Store.ReplaceTaskSteps(job.ID, steps)
	_ = a.Store.UpdateTask(job)
	_ = a.Store.SaveMessage(session.Message{
		ID:        shared.NewID("msg"),
		SessionID: job.SessionID,
		Role:      session.RoleAssistant,
		Content:   job.Result,
		TraceID:   job.TraceID,
		CreatedAt: time.Now().UTC(),
	})
	a.publishEvent("task.completed", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "result": job.Result})
}

func (a *Application) CancelTask(userID, taskID string) (task.Task, error) {
	job, err := a.GetTask(taskID)
	if err != nil {
		return task.Task{}, err
	}
	job.Status = task.StatusCanceled
	job.UpdatedAt = time.Now().UTC()
	if err := a.Store.UpdateTask(job); err != nil {
		return task.Task{}, err
	}
	a.appendAudit(job.WorkspaceID, userID, "task.cancel", "task", taskID, nil)
	return job, nil
}

func (a *Application) GetTask(taskID string) (task.Task, error) {
	item, err := a.Store.FindTaskByID(taskID)
	if err != nil {
		return task.Task{}, errors.New("task not found")
	}
	return item, nil
}

func (a *Application) GetTaskSteps(taskID string) []task.Step {
	items, err := a.Store.ListTaskSteps(taskID)
	if err != nil {
		return []task.Step{}
	}
	return items
}

func (a *Application) UpdateTool(userID, toolID string, req UpdateToolRequest) (tool.Tool, error) {
	item, err := a.Store.FindToolByID(toolID)
	if err != nil {
		return tool.Tool{}, errors.New("tool not found")
	}
	if ok, err := a.userInWorkspace(userID, item.WorkspaceID); err != nil || !ok {
		return tool.Tool{}, errors.New("workspace access denied")
	}
	if item.Scope == tool.ScopePlatform && !a.isPlatformAdmin(userID) {
		return tool.Tool{}, errors.New("only platform admin can update platform mcp")
	}
	if item.Scope == tool.ScopePersonal && item.CreatedBy != userID {
		return tool.Tool{}, errors.New("mcp access denied")
	}

	if value := strings.TrimSpace(req.Name); value != "" {
		item.Name = value
	}
	if value := strings.TrimSpace(req.Description); value != "" {
		item.Description = value
	}
	if req.Schema != nil {
		item.Schema = req.Schema
	}
	if req.Config != nil {
		item.Config = req.Config
	}
	if req.RequiresApproval != nil {
		item.RequiresApproval = *req.RequiresApproval
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if err := a.Store.UpdateTool(item); err != nil {
		return tool.Tool{}, err
	}
	a.appendAudit(item.WorkspaceID, userID, "tool.update", "tool", item.ID, map[string]any{"enabled": item.Enabled})
	return item, nil
}

func (a *Application) DeleteTool(userID, toolID string) error {
	item, err := a.Store.FindToolByID(toolID)
	if err != nil {
		return errors.New("tool not found")
	}
	if ok, err := a.userInWorkspace(userID, item.WorkspaceID); err != nil || !ok {
		return errors.New("workspace access denied")
	}
	if item.Scope == tool.ScopePlatform && !a.isPlatformAdmin(userID) {
		return errors.New("only platform admin can delete platform mcp")
	}
	if item.Scope == tool.ScopePersonal && item.CreatedBy != userID {
		return errors.New("mcp access denied")
	}
	if err := a.Store.DeleteTool(item.WorkspaceID, toolID); err != nil {
		return err
	}
	a.appendAudit(item.WorkspaceID, userID, "tool.delete", "tool", toolID, nil)
	return nil
}

func (a *Application) ListApprovals(workspaceID string) []approval.Approval {
	items, err := a.Store.ListApprovals(workspaceID)
	if err != nil {
		return []approval.Approval{}
	}
	return items
}

func (a *Application) ListAuditEvents(workspaceID string) []audit.Event {
	items, err := a.Store.ListAuditEvents(workspaceID)
	if err != nil {
		return []audit.Event{}
	}
	return items
}

func (a *Application) executeTool(t tool.Tool, input map[string]any) (string, string) {
	// 检查是否是 MCP 工具
	if t.Config != nil {
		if mcpID, ok := t.Config["mcp_id"].(string); ok {
			if toolName, ok := t.Config["mcp_tool"].(string); ok {
				// 调用 MCP 工具
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				result, err := a.MCPManager.CallTool(ctx, mcpID, toolName, input)
				if err != nil {
					errMsg := fmt.Sprintf("MCP tool error: %v", err)
					return errMsg, errMsg
				}

				// 提取结果文本
				var resultTexts []string
				for _, content := range result.Content {
					resultTexts = append(resultTexts, content.Text)
				}
				resultStr := strings.Join(resultTexts, "\n")
				return resultStr, resultStr
			}
		}
	}

	// 默认行为：模拟执行
	payload, _ := json.Marshal(input)
	result := fmt.Sprintf("executed tool %s with input %s", t.Name, string(payload))
	return result, result
}

func (a *Application) failTask(taskID, reason string) {
	job, err := a.GetTask(taskID)
	if err != nil {
		return
	}
	job.Status = task.StatusFailed
	job.Error = reason
	job.UpdatedAt = time.Now().UTC()
	_ = a.Store.UpdateTask(job)
	a.publishEvent("task.failed", job.ID, job.TraceID, map[string]any{"task_id": job.ID, "error": reason})
}

func (a *Application) userInWorkspace(userID, workspaceID string) (bool, error) {
	return a.Store.UserInWorkspace(userID, workspaceID)
}

func (a *Application) appendAudit(workspaceID, actorID, action, resource, resourceID string, metadata map[string]any) {
	entry := audit.Event{
		ID:          shared.NewID("audit"),
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		Action:      action,
		Resource:    resource,
		ResourceID:  resourceID,
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC(),
	}
	_ = a.Store.SaveAuditEvent(entry)
}

func (a *Application) publishEvent(eventType, taskID, traceID string, payload map[string]any) {
	packet := map[string]any{
		"type":      eventType,
		"task_id":   taskID,
		"trace_id":  traceID,
		"payload":   payload,
		"timestamp": time.Now().UTC(),
	}
	if raw, err := json.Marshal(packet); err == nil {
		a.Events.Publish(raw)
	}
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return "Untitled Session"
}

func (a *Application) validateSkillPolicy(userID string, skillIDs []string) error {
	for _, skillID := range normalizeStringList(skillIDs) {
		item, err := a.Store.FindSkillByID(skillID)
		if err != nil || !item.Enabled || !a.skillAccessible(userID, item) {
			return fmt.Errorf("skill %s not available", skillID)
		}
	}
	return nil
}

func (a *Application) validateToolPolicy(userID string, toolIDs []string) error {
	for _, toolID := range normalizeStringList(toolIDs) {
		item, err := a.Store.FindToolByID(toolID)
		if err != nil || !item.Enabled || !a.toolAccessible(userID, item) {
			return fmt.Errorf("tool %s not available", toolID)
		}
	}
	return nil
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

type CreateAgentRequest struct {
	WorkspaceID   string   `json:"workspace_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	SystemPrompt  string   `json:"system_prompt"`
	Model         string   `json:"model"`
	SkillPolicy   []string `json:"skill_policy"`
	ToolPolicy    []string `json:"tool_policy"`
	RuntimePolicy string   `json:"runtime_policy"`
}

type CreateToolRequest struct {
	WorkspaceID      string         `json:"workspace_id"`
	Name             string         `json:"name"`
	Scope            string         `json:"scope"`
	Description      string         `json:"description"`
	Schema           map[string]any `json:"schema"`
	Config           map[string]any `json:"config"`
	RequiresApproval bool           `json:"requires_approval"`
}

type UpdateToolRequest struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Schema           map[string]any `json:"schema"`
	Config           map[string]any `json:"config"`
	RequiresApproval *bool          `json:"requires_approval"`
	Enabled          *bool          `json:"enabled"`
}

type CreateSessionRequest struct {
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	Title       string `json:"title"`
}

type ExecuteTaskRequest struct {
	WorkspaceID  string          `json:"workspace_id"`
	AgentID      string          `json:"agent_id"`
	SessionID    string          `json:"session_id"`
	SessionTitle string          `json:"session_title"`
	Model        string          `json:"model"`
	Reasoning    string          `json:"reasoning"`
	Prompt       string          `json:"prompt"`
	Async        bool            `json:"async"`
	ToolCalls    []tool.CallSpec `json:"tool_calls"`
}

type CreateScheduleRequest struct {
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	Name        string `json:"name"`
	Prompt      string `json:"prompt"`
	Cron        string `json:"cron"`
}
