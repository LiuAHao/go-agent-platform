package memory

import (
	"errors"
	"strings"
	"sync"
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/approval"
	"go-agent-platform/internal/domain/audit"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/settings"
	"go-agent-platform/internal/domain/shared"
	"go-agent-platform/internal/domain/skill"
	"go-agent-platform/internal/domain/task"
	"go-agent-platform/internal/domain/tool"
	"go-agent-platform/internal/domain/workspace"
)

type Store struct {
	mu sync.RWMutex

	users           map[string]auth.User
	usersByEmail    map[string]string
	tokens          map[string]auth.SessionToken
	workspaces      map[string]workspace.Workspace
	memberships     map[string]workspace.Membership
	agents          map[string]agent.Agent
	agentVersions   map[string]agent.Version
	tools           map[string]tool.Tool
	skills          map[string]skill.Skill
	models          map[string]model.Model
	installedTools  map[string]map[string]struct{}
	installedSkills map[string]map[string]struct{}
	sessions        map[string]session.Session
	messages        map[string][]session.Message
	tasks           map[string]task.Task
	taskSteps       map[string][]task.Step
	approvals       map[string]approval.Approval
	auditEvents     []audit.Event
	schedules       map[string]task.Schedule
}

func NewStore() *Store {
	return &Store{
		users:           make(map[string]auth.User),
		usersByEmail:    make(map[string]string),
		tokens:          make(map[string]auth.SessionToken),
		workspaces:      make(map[string]workspace.Workspace),
		memberships:     make(map[string]workspace.Membership),
		agents:          make(map[string]agent.Agent),
		agentVersions:   make(map[string]agent.Version),
		tools:           make(map[string]tool.Tool),
		skills:          make(map[string]skill.Skill),
		models:          make(map[string]model.Model),
		installedTools:  make(map[string]map[string]struct{}),
		installedSkills: make(map[string]map[string]struct{}),
		sessions:        make(map[string]session.Session),
		messages:        make(map[string][]session.Message),
		tasks:           make(map[string]task.Task),
		taskSteps:       make(map[string][]task.Step),
		approvals:       make(map[string]approval.Approval),
		auditEvents:     make([]audit.Event, 0),
		schedules:       make(map[string]task.Schedule),
	}
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) EnsureSeedData(cfg config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	email := strings.ToLower(cfg.SeedAdminEmail)
	if userID, ok := s.usersByEmail[email]; ok {
		s.ensureDefaultModelLocked(userID)
		return nil
	}

	now := time.Now().UTC()
	admin := auth.User{
		ID:           shared.NewID("user"),
		Email:        cfg.SeedAdminEmail,
		Name:         "Platform Admin",
		PasswordHash: shared.HashPassword(cfg.SeedAdminPassword),
		CreatedAt:    now,
	}
	ws := workspace.Workspace{
		ID:        shared.NewID("ws"),
		Name:      "Default Workspace",
		CreatedBy: admin.ID,
		CreatedAt: now,
	}
	member := workspace.Membership{
		ID:          shared.NewID("member"),
		WorkspaceID: ws.ID,
		UserID:      admin.ID,
		Role:        workspace.RoleOwner,
		CreatedAt:   now,
	}
	defaultModel := model.Model{
		ID:              shared.NewID("model"),
		WorkspaceID:     ws.ID,
		Name:            "Mock Provider",
		Provider:        "mock",
		APIBaseURL:      "",
		APIKey:          "",
		ModelKey:        "mock-1",
		Description:     "默认开发模型，用于本地调试和首轮联调。",
		ContextWindow:   8192,
		MaxOutputTokens: 2048,
		Capabilities:    []string{"chat", "tools"},
		Enabled:         true,
		IsDefault:       true,
		CreatedBy:       admin.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	s.users[admin.ID] = admin
	s.usersByEmail[email] = admin.ID
	s.workspaces[ws.ID] = ws
	s.memberships[member.ID] = member
	s.models[defaultModel.ID] = defaultModel
	return nil
}

func (s *Store) ensureDefaultModelLocked(userID string) {
	var workspaceID string
	for _, membership := range s.memberships {
		if membership.UserID == userID {
			workspaceID = membership.WorkspaceID
			break
		}
	}
	if workspaceID == "" {
		return
	}
	for _, item := range s.models {
		if item.WorkspaceID == workspaceID && item.ModelKey == "mock-1" {
			return
		}
	}
	now := time.Now().UTC()
	modelID := shared.NewID("model")
	s.models[modelID] = model.Model{
		ID:              modelID,
		WorkspaceID:     workspaceID,
		Name:            "Mock Provider",
		Provider:        "mock",
		APIBaseURL:      "",
		APIKey:          "",
		ModelKey:        "mock-1",
		Description:     "默认开发模型，用于本地调试和首轮联调。",
		ContextWindow:   8192,
		MaxOutputTokens: 2048,
		Capabilities:    []string{"chat", "tools"},
		Enabled:         true,
		IsDefault:       true,
		CreatedBy:       userID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (s *Store) FindUserByEmail(email string) (auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id, ok := s.usersByEmail[strings.ToLower(email)]; ok {
		return s.users[id], nil
	}
	return auth.User{}, errors.New("user not found")
}

func (s *Store) FindUserByID(userID string) (auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.users[userID]
	if !ok {
		return auth.User{}, errors.New("user not found")
	}
	return item, nil
}

func (s *Store) SaveSessionToken(token auth.SessionToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
	return nil
}

func (s *Store) FindSessionToken(token string) (auth.SessionToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.tokens[token]
	if !ok {
		return auth.SessionToken{}, errors.New("token not found")
	}
	return item, nil
}

func (s *Store) ListWorkspacesByUser(userID string) ([]workspace.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]workspace.Workspace, 0)
	for _, membership := range s.memberships {
		if membership.UserID == userID {
			if ws, ok := s.workspaces[membership.WorkspaceID]; ok {
				items = append(items, ws)
			}
		}
	}
	return items, nil
}

func (s *Store) UserInWorkspace(userID, workspaceID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, membership := range s.memberships {
		if membership.UserID == userID && membership.WorkspaceID == workspaceID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) CreateWorkspace(item workspace.Workspace, member workspace.Membership) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.workspaces[item.ID] = item
	s.memberships[member.ID] = member
	return nil
}

func (s *Store) SaveAgent(item agent.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[item.ID] = item
	return nil
}

func (s *Store) UpdateAgent(item agent.Agent) error {
	return s.SaveAgent(item)
}

func (s *Store) FindAgentByID(agentID string) (agent.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.agents[agentID]
	if !ok {
		return agent.Agent{}, errors.New("agent not found")
	}
	return item, nil
}

func (s *Store) ListAgents(workspaceID string) ([]agent.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]agent.Agent, 0)
	for _, item := range s.agents {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) CountAgentVersions(agentID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, item := range s.agentVersions {
		if item.AgentID == agentID {
			count++
		}
	}
	return count, nil
}

func (s *Store) SaveAgentVersion(item agent.Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentVersions[item.ID] = item
	return nil
}

func (s *Store) FindAgentVersion(versionID string) (agent.Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.agentVersions[versionID]
	if !ok {
		return agent.Version{}, errors.New("agent version not found")
	}
	return item, nil
}

func (s *Store) SaveTool(item tool.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[item.ID] = item
	return nil
}

func (s *Store) UpdateTool(item tool.Tool) error {
	return s.SaveTool(item)
}

func (s *Store) DeleteTool(workspaceID, toolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.tools[toolID]
	if !ok || item.WorkspaceID != workspaceID {
		return errors.New("tool not found")
	}
	delete(s.tools, toolID)
	return nil
}

func (s *Store) ListPlatformTools() ([]tool.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]tool.Tool, 0)
	for _, item := range s.tools {
		if item.Scope == tool.ScopePlatform {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) ListUserTools(userID string) ([]tool.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]tool.Tool, 0)
	for _, item := range s.tools {
		if item.Scope == tool.ScopePersonal && item.CreatedBy == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) InstallTool(userID, toolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tools[toolID]; !ok {
		return errors.New("tool not found")
	}
	if _, ok := s.installedTools[userID]; !ok {
		s.installedTools[userID] = make(map[string]struct{})
	}
	s.installedTools[userID][toolID] = struct{}{}
	return nil
}

func (s *Store) UninstallTool(userID, toolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.installedTools[userID]; ok {
		delete(s.installedTools[userID], toolID)
	}
	return nil
}

func (s *Store) ListInstalledToolIDs(userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]string, 0)
	for id := range s.installedTools[userID] {
		items = append(items, id)
	}
	return items, nil
}

func (s *Store) SaveSkill(item skill.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skills[item.ID] = item
	return nil
}

func (s *Store) ListPlatformSkills() ([]skill.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]skill.Skill, 0)
	for _, item := range s.skills {
		if item.Scope == skill.ScopePlatform {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) ListUserSkills(userID string) ([]skill.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]skill.Skill, 0)
	for _, item := range s.skills {
		if item.Scope == skill.ScopePersonal && item.CreatedBy == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) InstallSkill(userID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skills[skillID]; !ok {
		return errors.New("skill not found")
	}
	if _, ok := s.installedSkills[userID]; !ok {
		s.installedSkills[userID] = make(map[string]struct{})
	}
	s.installedSkills[userID][skillID] = struct{}{}
	return nil
}

func (s *Store) UninstallSkill(userID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.installedSkills[userID]; ok {
		delete(s.installedSkills[userID], skillID)
	}
	return nil
}

func (s *Store) ListInstalledSkillIDs(userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]string, 0)
	for id := range s.installedSkills[userID] {
		items = append(items, id)
	}
	return items, nil
}

func (s *Store) UpdateSkill(item skill.Skill) error {
	return s.SaveSkill(item)
}

func (s *Store) DeleteSkill(workspaceID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.skills[skillID]
	if !ok || item.WorkspaceID != workspaceID {
		return errors.New("skill not found")
	}
	delete(s.skills, skillID)
	return nil
}

func (s *Store) FindSkillByID(skillID string) (skill.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.skills[skillID]
	if !ok {
		return skill.Skill{}, errors.New("skill not found")
	}
	return item, nil
}

func (s *Store) ListSkills(workspaceID string) ([]skill.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]skill.Skill, 0)
	for _, item := range s.skills {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) SaveModel(item model.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[item.ID] = item
	return nil
}

func (s *Store) UpdateModel(item model.Model) error {
	return s.SaveModel(item)
}

func (s *Store) DeleteModel(workspaceID, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.models[modelID]
	if !ok || item.WorkspaceID != workspaceID {
		return errors.New("model not found")
	}
	delete(s.models, modelID)
	return nil
}

func (s *Store) FindModelByID(modelID string) (model.Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.models[modelID]
	if !ok {
		return model.Model{}, errors.New("model not found")
	}
	return item, nil
}

func (s *Store) FindModelByKey(workspaceID, modelKey string) (model.Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.models {
		if item.WorkspaceID == workspaceID && item.ModelKey == modelKey {
			return item, nil
		}
	}
	return model.Model{}, errors.New("model not found")
}

func (s *Store) ListModels(workspaceID string) ([]model.Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]model.Model, 0)
	for _, item := range s.models {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) ListTools(workspaceID string) ([]tool.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]tool.Tool, 0)
	for _, item := range s.tools {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) FindToolByID(toolID string) (tool.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.tools[toolID]
	if !ok {
		return tool.Tool{}, errors.New("tool not found")
	}
	return item, nil
}

func (s *Store) SaveSession(item session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[item.ID] = item
	return nil
}

func (s *Store) ListSessionsByAgent(userID, agentID string) ([]session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]session.Session, 0)
	for _, item := range s.sessions {
		if item.AgentID == agentID && item.CreatedBy == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) SaveMessage(item session.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[item.SessionID] = append(s.messages[item.SessionID], item)
	return nil
}

func (s *Store) ListMessages(sessionID string) ([]session.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := append([]session.Message(nil), s.messages[sessionID]...)
	return items, nil
}

func (s *Store) SaveSchedule(item task.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedules[item.ID] = item
	return nil
}

func (s *Store) ListDueSchedules(now time.Time) ([]task.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]task.Schedule, 0)
	for _, item := range s.schedules {
		if !item.NextRunAt.After(now) {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) UpdateSchedule(item task.Schedule) error {
	return s.SaveSchedule(item)
}

func (s *Store) SaveTask(item task.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[item.ID] = item
	return nil
}

func (s *Store) UpdateTask(item task.Task) error {
	return s.SaveTask(item)
}

func (s *Store) FindTaskByID(taskID string) (task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.tasks[taskID]
	if !ok {
		return task.Task{}, errors.New("task not found")
	}
	return item, nil
}

func (s *Store) ListTasksBySession(sessionID string) ([]task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]task.Task, 0)
	for _, item := range s.tasks {
		if item.SessionID == sessionID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) SaveTaskStep(item task.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskSteps[item.TaskID] = append(s.taskSteps[item.TaskID], item)
	return nil
}

func (s *Store) SaveTaskSteps(items []task.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.taskSteps[item.TaskID] = append(s.taskSteps[item.TaskID], item)
	}
	return nil
}

func (s *Store) ReplaceTaskSteps(taskID string, items []task.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskSteps[taskID] = append([]task.Step(nil), items...)
	return nil
}

func (s *Store) ListTaskSteps(taskID string) ([]task.Step, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := append([]task.Step(nil), s.taskSteps[taskID]...)
	return items, nil
}

func (s *Store) SaveApproval(item approval.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals[item.ID] = item
	return nil
}

func (s *Store) UpdateApproval(item approval.Approval) error {
	return s.SaveApproval(item)
}

func (s *Store) FindApprovalByID(approvalID string) (approval.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.approvals[approvalID]
	if !ok {
		return approval.Approval{}, errors.New("approval not found")
	}
	return item, nil
}

func (s *Store) ListApprovals(workspaceID string) ([]approval.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]approval.Approval, 0)
	for _, item := range s.approvals {
		if workspaceID == "" || item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) SaveAuditEvent(item audit.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditEvents = append(s.auditEvents, item)
	return nil
}

func (s *Store) ListAuditEvents(workspaceID string) ([]audit.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]audit.Event, 0)
	for _, item := range s.auditEvents {
		if workspaceID == "" || item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}

// 删除会话
func (s *Store) DeleteSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return errors.New("session not found")
	}

	delete(s.sessions, sessionID)
	delete(s.messages, sessionID)
	return nil
}

// 删除消息
func (s *Store) DeleteMessage(messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, messages := range s.messages {
		for i, msg := range messages {
			if msg.ID == messageID {
				s.messages[sessionID] = append(messages[:i], messages[i+1:]...)
				return nil
			}
		}
	}

	return errors.New("message not found")
}

// 用户设置存储
var userSettingsStore = make(map[string]settings.UserSettings)

func (s *Store) GetUserSettings(userID string) (settings.UserSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userSettings, ok := userSettingsStore[userID]
	if !ok {
		// 返回默认设置
		return settings.DefaultSettings(userID), nil
	}

	return userSettings, nil
}

func (s *Store) SaveUserSettings(userSettings settings.UserSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userSettingsStore[userSettings.UserID] = userSettings
	return nil
}
