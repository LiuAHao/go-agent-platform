package local

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/approval"
	"go-agent-platform/internal/domain/audit"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/skill"
	"go-agent-platform/internal/domain/task"
	"go-agent-platform/internal/domain/tool"
	"go-agent-platform/internal/domain/workspace"

	_ "modernc.org/sqlite"
)

// Store 是本地 SQLite 存储实现
type Store struct {
	db   *sql.DB
	home *Home
}

// NewStore 创建本地 SQLite 存储
func NewStore(home *Home) (*Store, error) {
	dbPath := home.DBPath()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// 启用 WAL 模式提高并发性能
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db, home: home}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// EnsureSeedData 创建初始数据
func (s *Store) EnsureSeedData(cfg config.Config) error {
	// 检查是否已有管理员用户
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", cfg.SeedAdminEmail).Scan(&count)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if count > 0 {
		return nil
	}

	// 创建管理员用户
	now := time.Now().UTC()
	_, err = s.db.Exec(`
		INSERT INTO users (id, email, name, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, "admin-001", cfg.SeedAdminEmail, "Admin", hashPassword(cfg.SeedAdminPassword), now)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	// 创建默认工作区
	_, err = s.db.Exec(`
		INSERT INTO workspaces (id, name, created_by, created_at)
		VALUES (?, ?, ?, ?)
	`, "default", "Default", "admin-001", now)
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// 添加管理员到工作区
	_, err = s.db.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		VALUES (?, ?, ?, ?)
	`, "default", "admin-001", "owner", now)
	if err != nil {
		return fmt.Errorf("add admin to workspace: %w", err)
	}

	return nil
}

// hashPassword 简单的密码哈希 (MVP 阶段，生产环境应使用 bcrypt)
func hashPassword(password string) string {
	return fmt.Sprintf("hashed:%s", password)
}

// --- 用户相关 ---

func (s *Store) FindUserByEmail(email string) (auth.User, error) {
	var u auth.User
	err := s.db.QueryRow(`
		SELECT id, email, name, password_hash, created_at
		FROM users WHERE email = ?
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (s *Store) FindUserByID(userID string) (auth.User, error) {
	var u auth.User
	err := s.db.QueryRow(`
		SELECT id, email, name, password_hash, created_at
		FROM users WHERE id = ?
	`, userID).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (s *Store) SaveSessionToken(token auth.SessionToken) error {
	_, err := s.db.Exec(`
		INSERT INTO session_tokens (token, user_id, expires_at)
		VALUES (?, ?, ?)
	`, token.Token, token.UserID, token.ExpiresAt)
	return err
}

func (s *Store) FindSessionToken(token string) (auth.SessionToken, error) {
	var t auth.SessionToken
	err := s.db.QueryRow(`
		SELECT token, user_id, expires_at
		FROM session_tokens WHERE token = ?
	`, token).Scan(&t.Token, &t.UserID, &t.ExpiresAt)
	return t, err
}

// --- 工作区相关 ---

func (s *Store) ListWorkspacesByUser(userID string) ([]workspace.Workspace, error) {
	rows, err := s.db.Query(`
		SELECT w.id, w.name, w.created_by, w.created_at
		FROM workspaces w
		JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []workspace.Workspace
	for rows.Next() {
		var w workspace.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedBy, &w.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, w)
	}
	return items, nil
}

func (s *Store) UserInWorkspace(userID, workspaceID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM workspace_members
		WHERE user_id = ? AND workspace_id = ?
	`, userID, workspaceID).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateWorkspace(item workspace.Workspace, member workspace.Membership) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO workspaces (id, name, created_by, created_at)
		VALUES (?, ?, ?, ?)
	`, item.ID, item.Name, item.CreatedBy, item.CreatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
		VALUES (?, ?, ?, ?)
	`, member.WorkspaceID, member.UserID, string(member.Role), member.CreatedAt)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// --- Agent 相关 ---

func (s *Store) SaveAgent(item agent.Agent) error {
	_, err := s.db.Exec(`
		INSERT INTO agents (id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, published_version_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.Name, item.Description, item.SystemPrompt, item.Model,
		toJSON(item.SkillPolicy), toJSON(item.ToolPolicy), item.RuntimePolicy,
		item.PublishedVerID, item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateAgent(item agent.Agent) error {
	_, err := s.db.Exec(`
		UPDATE agents
		SET name = ?, description = ?, system_prompt = ?, model = ?, skill_policy = ?, tool_policy = ?, runtime_policy = ?, published_version_id = ?, updated_at = ?
		WHERE id = ?
	`, item.Name, item.Description, item.SystemPrompt, item.Model,
		toJSON(item.SkillPolicy), toJSON(item.ToolPolicy), item.RuntimePolicy,
		item.PublishedVerID, item.UpdatedAt, item.ID)
	return err
}

func (s *Store) FindAgentByID(agentID string) (agent.Agent, error) {
	var a agent.Agent
	var skillPolicy, toolPolicy []byte
	err := s.db.QueryRow(`
		SELECT id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, published_version_id, created_by, created_at, updated_at
		FROM agents WHERE id = ?
	`, agentID).Scan(&a.ID, &a.WorkspaceID, &a.Name, &a.Description, &a.SystemPrompt, &a.Model,
		&skillPolicy, &toolPolicy, &a.RuntimePolicy,
		&a.PublishedVerID, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return agent.Agent{}, err
	}
	json.Unmarshal(skillPolicy, &a.SkillPolicy)
	json.Unmarshal(toolPolicy, &a.ToolPolicy)
	return a, nil
}

func (s *Store) ListAgents(workspaceID string) ([]agent.Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, published_version_id, created_by, created_at, updated_at
		FROM agents WHERE workspace_id = ?
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []agent.Agent
	for rows.Next() {
		var a agent.Agent
		var skillPolicy, toolPolicy []byte
		if err := rows.Scan(&a.ID, &a.WorkspaceID, &a.Name, &a.Description, &a.SystemPrompt, &a.Model,
			&skillPolicy, &toolPolicy, &a.RuntimePolicy,
			&a.PublishedVerID, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(skillPolicy, &a.SkillPolicy)
		json.Unmarshal(toolPolicy, &a.ToolPolicy)
		items = append(items, a)
	}
	return items, nil
}

func (s *Store) CountAgentVersions(agentID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM agent_versions WHERE agent_id = ?", agentID).Scan(&count)
	return count, err
}

func (s *Store) SaveAgentVersion(item agent.Version) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_versions (id, agent_id, version, description, snapshot, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.AgentID, item.VersionNumber, "", toJSON(item.Snapshot), item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) FindAgentVersion(versionID string) (agent.Version, error) {
	var v agent.Version
	var snapshot []byte
	err := s.db.QueryRow(`
		SELECT id, agent_id, version, description, snapshot, created_by, created_at
		FROM agent_versions WHERE id = ?
	`, versionID).Scan(&v.ID, &v.AgentID, &v.VersionNumber, new(string), &snapshot, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return agent.Version{}, err
	}
	json.Unmarshal(snapshot, &v.Snapshot)
	return v, nil
}

// --- Tool/MCP 相关 ---

func (s *Store) SaveTool(item tool.Tool) error {
	_, err := s.db.Exec(`
		INSERT INTO tools (id, workspace_id, name, scope, description, schema_data, config, enabled, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.Name, item.Scope, item.Description,
		toJSON(item.Schema), toJSON(item.Config), item.Enabled,
		item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) UpdateTool(item tool.Tool) error {
	_, err := s.db.Exec(`
		UPDATE tools
		SET name = ?, description = ?, schema_data = ?, config = ?, enabled = ?
		WHERE id = ?
	`, item.Name, item.Description,
		toJSON(item.Schema), toJSON(item.Config), item.Enabled, item.ID)
	return err
}

func (s *Store) DeleteTool(workspaceID, toolID string) error {
	_, err := s.db.Exec("DELETE FROM tools WHERE id = ? AND workspace_id = ?", toolID, workspaceID)
	return err
}

func (s *Store) ListTools(workspaceID string) ([]tool.Tool, error) {
	return s.listToolsByQuery("SELECT * FROM tools WHERE workspace_id = ?", workspaceID)
}

func (s *Store) ListPlatformTools() ([]tool.Tool, error) {
	return s.listToolsByQuery("SELECT * FROM tools WHERE scope = 'platform'")
}

func (s *Store) ListUserTools(userID string) ([]tool.Tool, error) {
	return s.listToolsByQuery("SELECT * FROM tools WHERE scope = 'personal' AND created_by = ?", userID)
}

func (s *Store) listToolsByQuery(query string, args ...any) ([]tool.Tool, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []tool.Tool
	for rows.Next() {
		var t tool.Tool
		var configData, schemaData []byte
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Scope, &t.Description,
			&schemaData, &configData, &t.Enabled,
			&t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(configData, &t.Config)
		json.Unmarshal(schemaData, &t.Schema)
		items = append(items, t)
	}
	return items, nil
}

func (s *Store) FindToolByID(toolID string) (tool.Tool, error) {
	var t tool.Tool
	var configData, schemaData []byte
	err := s.db.QueryRow(`
		SELECT id, workspace_id, name, scope, description, schema_data, config, enabled, created_by, created_at
		FROM tools WHERE id = ?
	`, toolID).Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Scope, &t.Description,
		&schemaData, &configData, &t.Enabled,
		&t.CreatedBy, &t.CreatedAt)
	if err != nil {
		return tool.Tool{}, err
	}
	json.Unmarshal(configData, &t.Config)
	json.Unmarshal(schemaData, &t.Schema)
	return t, nil
}

func (s *Store) InstallTool(userID, toolID string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_tool_installs (user_id, tool_id, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id, tool_id) DO NOTHING
	`, userID, toolID, time.Now().UTC())
	return err
}

func (s *Store) UninstallTool(userID, toolID string) error {
	_, err := s.db.Exec("DELETE FROM user_tool_installs WHERE user_id = ? AND tool_id = ?", userID, toolID)
	return err
}

func (s *Store) ListInstalledToolIDs(userID string) ([]string, error) {
	rows, err := s.db.Query("SELECT tool_id FROM user_tool_installs WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- Skill 相关 ---

func (s *Store) SaveSkill(item skill.Skill) error {
	_, err := s.db.Exec(`
		INSERT INTO skills (id, workspace_id, name, slug, scope, description, version, entry, schema_data, config, enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.Name, item.Slug, item.Scope, item.Description, item.Version,
		item.Entry, toJSON(item.Schema), toJSON(item.Config), item.Enabled,
		item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateSkill(item skill.Skill) error {
	_, err := s.db.Exec(`
		UPDATE skills
		SET name = ?, description = ?, version = ?, entry = ?, schema_data = ?, config = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, item.Name, item.Description, item.Version, item.Entry,
		toJSON(item.Schema), toJSON(item.Config), item.Enabled, item.UpdatedAt, item.ID)
	return err
}

func (s *Store) DeleteSkill(workspaceID, skillID string) error {
	_, err := s.db.Exec("DELETE FROM skills WHERE id = ? AND workspace_id = ?", skillID, workspaceID)
	return err
}

func (s *Store) FindSkillByID(skillID string) (skill.Skill, error) {
	var sk skill.Skill
	var schemaData, configData []byte
	err := s.db.QueryRow(`
		SELECT id, workspace_id, name, slug, scope, description, version, entry, schema_data, config, enabled, created_by, created_at, updated_at
		FROM skills WHERE id = ?
	`, skillID).Scan(&sk.ID, &sk.WorkspaceID, &sk.Name, &sk.Slug, &sk.Scope, &sk.Description, &sk.Version,
		&sk.Entry, &schemaData, &configData, &sk.Enabled,
		&sk.CreatedBy, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return skill.Skill{}, err
	}
	json.Unmarshal(schemaData, &sk.Schema)
	json.Unmarshal(configData, &sk.Config)
	return sk, nil
}

func (s *Store) ListSkills(workspaceID string) ([]skill.Skill, error) {
	return s.listSkillsByQuery("SELECT * FROM skills WHERE workspace_id = ?", workspaceID)
}

func (s *Store) ListPlatformSkills() ([]skill.Skill, error) {
	return s.listSkillsByQuery("SELECT * FROM skills WHERE scope = 'platform'")
}

func (s *Store) ListUserSkills(userID string) ([]skill.Skill, error) {
	return s.listSkillsByQuery("SELECT * FROM skills WHERE scope = 'personal' AND created_by = ?", userID)
}

func (s *Store) listSkillsByQuery(query string, args ...any) ([]skill.Skill, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []skill.Skill
	for rows.Next() {
		var sk skill.Skill
		var schemaData, configData []byte
		if err := rows.Scan(&sk.ID, &sk.WorkspaceID, &sk.Name, &sk.Slug, &sk.Scope, &sk.Description, &sk.Version,
			&sk.Entry, &schemaData, &configData, &sk.Enabled,
			&sk.CreatedBy, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(schemaData, &sk.Schema)
		json.Unmarshal(configData, &sk.Config)
		items = append(items, sk)
	}
	return items, nil
}

func (s *Store) InstallSkill(userID, skillID string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_skill_installs (user_id, skill_id, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id, skill_id) DO NOTHING
	`, userID, skillID, time.Now().UTC())
	return err
}

func (s *Store) UninstallSkill(userID, skillID string) error {
	_, err := s.db.Exec("DELETE FROM user_skill_installs WHERE user_id = ? AND skill_id = ?", userID, skillID)
	return err
}

func (s *Store) ListInstalledSkillIDs(userID string) ([]string, error) {
	rows, err := s.db.Query("SELECT skill_id FROM user_skill_installs WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- Model 相关 ---

func (s *Store) SaveModel(item model.Model) error {
	_, err := s.db.Exec(`
		INSERT INTO models (id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, is_default, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.Name, item.Provider, item.APIBaseURL,
		item.APIKey, item.ModelKey, item.Description, item.ContextWindow, item.MaxOutputTokens, item.IsDefault,
		item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateModel(item model.Model) error {
	_, err := s.db.Exec(`
		UPDATE models
		SET name = ?, provider = ?, api_base_url = ?, api_key = ?, model_key = ?, description = ?, context_window = ?, max_output_tokens = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`, item.Name, item.Provider, item.APIBaseURL, item.APIKey, item.ModelKey,
		item.Description, item.ContextWindow, item.MaxOutputTokens, item.IsDefault, item.UpdatedAt, item.ID)
	return err
}

func (s *Store) DeleteModel(workspaceID, modelID string) error {
	_, err := s.db.Exec("DELETE FROM models WHERE id = ? AND workspace_id = ?", modelID, workspaceID)
	return err
}

func (s *Store) FindModelByID(modelID string) (model.Model, error) {
	var m model.Model
	err := s.db.QueryRow(`
		SELECT id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, is_default, created_by, created_at, updated_at
		FROM models WHERE id = ?
	`, modelID).Scan(&m.ID, &m.WorkspaceID, &m.Name, &m.Provider, &m.APIBaseURL,
		&m.APIKey, &m.ModelKey, &m.Description, &m.ContextWindow, &m.MaxOutputTokens, &m.IsDefault,
		&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (s *Store) FindModelByKey(workspaceID, modelKey string) (model.Model, error) {
	var m model.Model
	err := s.db.QueryRow(`
		SELECT id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, is_default, created_by, created_at, updated_at
		FROM models WHERE workspace_id = ? AND model_key = ?
	`, workspaceID, modelKey).Scan(&m.ID, &m.WorkspaceID, &m.Name, &m.Provider, &m.APIBaseURL,
		&m.APIKey, &m.ModelKey, &m.Description, &m.ContextWindow, &m.MaxOutputTokens, &m.IsDefault,
		&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (s *Store) ListModels(workspaceID string) ([]model.Model, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, is_default, created_by, created_at, updated_at
		FROM models WHERE workspace_id = ?
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Model
	for rows.Next() {
		var m model.Model
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.Name, &m.Provider, &m.APIBaseURL,
			&m.APIKey, &m.ModelKey, &m.Description, &m.ContextWindow, &m.MaxOutputTokens, &m.IsDefault,
			&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, nil
}

// --- Session 相关 ---

func (s *Store) SaveSession(item session.Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, workspace_id, agent_id, created_by, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.AgentID, item.CreatedBy, item.Title, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) ListSessionsByAgent(userID, agentID string) ([]session.Session, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, agent_id, created_by, title, created_at, updated_at
		FROM sessions WHERE created_by = ? AND agent_id = ?
	`, userID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []session.Session
	for rows.Next() {
		var sess session.Session
		if err := rows.Scan(&sess.ID, &sess.WorkspaceID, &sess.AgentID, &sess.CreatedBy, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, sess)
	}
	return items, nil
}

func (s *Store) SaveMessage(item session.Message) error {
	_, err := s.db.Exec(`
		INSERT INTO messages (id, session_id, role, content, trace_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, item.ID, item.SessionID, string(item.Role), item.Content, item.TraceID, item.CreatedAt)
	return err
}

func (s *Store) ListMessages(sessionID string) ([]session.Message, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, trace_id, created_at
		FROM messages WHERE session_id = ?
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []session.Message
	for rows.Next() {
		var m session.Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.TraceID, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, nil
}

// --- Task 相关 ---

func (s *Store) SaveSchedule(item task.Schedule) error {
	_, err := s.db.Exec(`
		INSERT INTO schedules (id, workspace_id, agent_id, name, prompt, cron, interval, next_run_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.AgentID, item.Name, item.Prompt,
		item.Cron, item.Interval, item.NextRunAt, item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) ListDueSchedules(now time.Time) ([]task.Schedule, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, agent_id, name, prompt, cron, interval, next_run_at, created_by, created_at
		FROM schedules WHERE next_run_at <= ?
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []task.Schedule
	for rows.Next() {
		var t task.Schedule
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.AgentID, &t.Name, &t.Prompt,
			&t.Cron, &t.Interval, &t.NextRunAt, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, nil
}

func (s *Store) UpdateSchedule(item task.Schedule) error {
	_, err := s.db.Exec(`
		UPDATE schedules
		SET name = ?, prompt = ?, cron = ?, interval = ?, next_run_at = ?
		WHERE id = ?
	`, item.Name, item.Prompt, item.Cron, item.Interval, item.NextRunAt, item.ID)
	return err
}

func (s *Store) SaveTask(item task.Task) error {
	_, err := s.db.Exec(`
		INSERT INTO tasks (id, workspace_id, agent_id, session_id, model, reasoning, prompt, status, result, error, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.AgentID, item.SessionID, item.Model, item.Reasoning,
		item.Prompt, string(item.Status), item.Result, item.Error,
		item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateTask(item task.Task) error {
	_, err := s.db.Exec(`
		UPDATE tasks
		SET status = ?, result = ?, error = ?, updated_at = ?
		WHERE id = ?
	`, string(item.Status), item.Result, item.Error, item.UpdatedAt, item.ID)
	return err
}

func (s *Store) FindTaskByID(taskID string) (task.Task, error) {
	var t task.Task
	err := s.db.QueryRow(`
		SELECT id, workspace_id, agent_id, session_id, model, reasoning, prompt, status, result, error, created_by, created_at, updated_at
		FROM tasks WHERE id = ?
	`, taskID).Scan(&t.ID, &t.WorkspaceID, &t.AgentID, &t.SessionID, &t.Model, &t.Reasoning,
		&t.Prompt, &t.Status, &t.Result, &t.Error,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *Store) ListTasksBySession(sessionID string) ([]task.Task, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, agent_id, session_id, model, reasoning, prompt, status, result, error, created_by, created_at, updated_at
		FROM tasks WHERE session_id = ?
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []task.Task
	for rows.Next() {
		var t task.Task
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.AgentID, &t.SessionID, &t.Model, &t.Reasoning,
			&t.Prompt, &t.Status, &t.Result, &t.Error,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, nil
}

func (s *Store) SaveTaskStep(item task.Step) error {
	_, err := s.db.Exec(`
		INSERT INTO task_steps (id, task_id, name, status, output, error, sequence, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.TaskID, item.Name, string(item.Status), item.Output, item.Error,
		item.Sequence, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) SaveTaskSteps(items []task.Step) error {
	for _, item := range items {
		if err := s.SaveTaskStep(item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReplaceTaskSteps(taskID string, items []task.Step) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM task_steps WHERE task_id = ?", taskID)
	if err != nil {
		return err
	}

	for _, item := range items {
		_, err = tx.Exec(`
			INSERT INTO task_steps (id, task_id, name, status, output, error, sequence, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.ID, item.TaskID, item.Name, string(item.Status), item.Output, item.Error,
			item.Sequence, item.CreatedAt, item.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListTaskSteps(taskID string) ([]task.Step, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, name, status, output, error, sequence, created_at, updated_at
		FROM task_steps WHERE task_id = ?
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []task.Step
	for rows.Next() {
		var st task.Step
		if err := rows.Scan(&st.ID, &st.TaskID, &st.Name, &st.Status, &st.Output, &st.Error,
			&st.Sequence, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, st)
	}
	return items, nil
}

// --- Approval 相关 ---

func (s *Store) SaveApproval(item approval.Approval) error {
	_, err := s.db.Exec(`
		INSERT INTO approvals (id, workspace_id, task_id, step_id, reason, status, decision_by, decision_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.TaskID, item.StepID, item.Reason,
		string(item.Status), item.DecisionBy, item.DecisionAt, item.CreatedAt)
	return err
}

func (s *Store) UpdateApproval(item approval.Approval) error {
	_, err := s.db.Exec(`
		UPDATE approvals
		SET status = ?, decision_by = ?, decision_at = ?
		WHERE id = ?
	`, string(item.Status), item.DecisionBy, item.DecisionAt, item.ID)
	return err
}

func (s *Store) FindApprovalByID(approvalID string) (approval.Approval, error) {
	var a approval.Approval
	err := s.db.QueryRow(`
		SELECT id, workspace_id, task_id, step_id, reason, status, decision_by, decision_at, created_at
		FROM approvals WHERE id = ?
	`, approvalID).Scan(&a.ID, &a.WorkspaceID, &a.TaskID, &a.StepID, &a.Reason,
		&a.Status, &a.DecisionBy, &a.DecisionAt, &a.CreatedAt)
	return a, err
}

func (s *Store) ListApprovals(workspaceID string) ([]approval.Approval, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, task_id, step_id, reason, status, decision_by, decision_at, created_at
		FROM approvals WHERE workspace_id = ?
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []approval.Approval
	for rows.Next() {
		var a approval.Approval
		if err := rows.Scan(&a.ID, &a.WorkspaceID, &a.TaskID, &a.StepID, &a.Reason,
			&a.Status, &a.DecisionBy, &a.DecisionAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, nil
}

// --- Audit 相关 ---

func (s *Store) SaveAuditEvent(item audit.Event) error {
	_, err := s.db.Exec(`
		INSERT INTO audit_events (id, workspace_id, actor_id, action, resource, resource_id, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.WorkspaceID, item.ActorID, item.Action, item.Resource,
		item.ResourceID, toJSON(item.Metadata), item.CreatedAt)
	return err
}

func (s *Store) ListAuditEvents(workspaceID string) ([]audit.Event, error) {
	rows, err := s.db.Query(`
		SELECT id, workspace_id, actor_id, action, resource, resource_id, metadata, created_at
		FROM audit_events WHERE workspace_id = ?
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []audit.Event
	for rows.Next() {
		var e audit.Event
		var metadata []byte
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.ActorID, &e.Action, &e.Resource,
			&e.ResourceID, &metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(metadata, &e.Metadata)
		items = append(items, e)
	}
	return items, nil
}

// toJSON 将对象序列化为 JSON 字节
func toJSON(v any) []byte {
	if v == nil {
		return nil
	}
	data, _ := json.Marshal(v)
	return data
}
