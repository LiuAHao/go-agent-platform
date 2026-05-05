package app

import (
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/approval"
	"go-agent-platform/internal/domain/audit"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/settings"
	"go-agent-platform/internal/domain/skill"
	"go-agent-platform/internal/domain/task"
	"go-agent-platform/internal/domain/tool"
	"go-agent-platform/internal/domain/workspace"
)

// Store 定义应用层依赖的持久化能力，屏蔽 memory/postgres 差异。
type Store interface {
	Close() error
	EnsureSeedData(cfg config.Config) error
	FindUserByEmail(email string) (auth.User, error)
	FindUserByID(userID string) (auth.User, error)
	SaveSessionToken(token auth.SessionToken) error
	FindSessionToken(token string) (auth.SessionToken, error)
	ListWorkspacesByUser(userID string) ([]workspace.Workspace, error)
	UserInWorkspace(userID, workspaceID string) (bool, error)
	CreateWorkspace(item workspace.Workspace, member workspace.Membership) error
	SaveAgent(item agent.Agent) error
	UpdateAgent(item agent.Agent) error
	FindAgentByID(agentID string) (agent.Agent, error)
	ListAgents(workspaceID string) ([]agent.Agent, error)
	CountAgentVersions(agentID string) (int, error)
	SaveAgentVersion(item agent.Version) error
	FindAgentVersion(versionID string) (agent.Version, error)
	SaveTool(item tool.Tool) error
	UpdateTool(item tool.Tool) error
	DeleteTool(workspaceID, toolID string) error
	ListTools(workspaceID string) ([]tool.Tool, error)
	ListPlatformTools() ([]tool.Tool, error)
	ListUserTools(userID string) ([]tool.Tool, error)
	FindToolByID(toolID string) (tool.Tool, error)
	InstallTool(userID, toolID string) error
	UninstallTool(userID, toolID string) error
	ListInstalledToolIDs(userID string) ([]string, error)
	SaveSkill(item skill.Skill) error
	UpdateSkill(item skill.Skill) error
	DeleteSkill(workspaceID, skillID string) error
	FindSkillByID(skillID string) (skill.Skill, error)
	ListSkills(workspaceID string) ([]skill.Skill, error)
	ListPlatformSkills() ([]skill.Skill, error)
	ListUserSkills(userID string) ([]skill.Skill, error)
	InstallSkill(userID, skillID string) error
	UninstallSkill(userID, skillID string) error
	ListInstalledSkillIDs(userID string) ([]string, error)
	SaveModel(item model.Model) error
	UpdateModel(item model.Model) error
	DeleteModel(workspaceID, modelID string) error
	FindModelByID(modelID string) (model.Model, error)
	FindModelByKey(workspaceID, modelKey string) (model.Model, error)
	ListModels(workspaceID string) ([]model.Model, error)
	SaveSession(item session.Session) error
	DeleteSession(sessionID string) error
	ListSessionsByAgent(userID, agentID string) ([]session.Session, error)
	SaveMessage(item session.Message) error
	DeleteMessage(messageID string) error
	ListMessages(sessionID string) ([]session.Message, error)
	SaveSchedule(item task.Schedule) error
	ListDueSchedules(now time.Time) ([]task.Schedule, error)
	UpdateSchedule(item task.Schedule) error
	SaveTask(item task.Task) error
	UpdateTask(item task.Task) error
	FindTaskByID(taskID string) (task.Task, error)
	ListTasksBySession(sessionID string) ([]task.Task, error)
	SaveTaskStep(item task.Step) error
	SaveTaskSteps(items []task.Step) error
	ReplaceTaskSteps(taskID string, items []task.Step) error
	ListTaskSteps(taskID string) ([]task.Step, error)
	SaveApproval(item approval.Approval) error
	UpdateApproval(item approval.Approval) error
	FindApprovalByID(approvalID string) (approval.Approval, error)
	ListApprovals(workspaceID string) ([]approval.Approval, error)
	SaveAuditEvent(item audit.Event) error
	ListAuditEvents(workspaceID string) ([]audit.Event, error)
	GetUserSettings(userID string) (settings.UserSettings, error)
	SaveUserSettings(settings settings.UserSettings) error
}
