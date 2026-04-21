package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/approval"
	"go-agent-platform/internal/domain/audit"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/shared"
	"go-agent-platform/internal/domain/skill"
	"go-agent-platform/internal/domain/task"
	"go-agent-platform/internal/domain/tool"
	"go-agent-platform/internal/domain/workspace"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(cfg config.Config) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	store := &Store{pool: pool}
	if cfg.PostgresAutoMigrate {
		if err := store.RunMigrations(
			resolveMigrationPath("migrations/001_init.sql"),
			resolveMigrationPath("migrations/002_skill_and_model_registry.sql"),
			resolveMigrationPath("migrations/003_platform_catalog_and_chat.sql"),
			resolveMigrationPath("migrations/004_model_api_config.sql"),
		); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

func (s *Store) RunMigrations(paths ...string) error {
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration: %w", err)
		}
		for _, stmt := range splitStatements(string(raw)) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := s.pool.Exec(context.Background(), stmt); err != nil {
				return fmt.Errorf("run migration: %w", err)
			}
		}
	}
	return nil
}

func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	stmts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			stmts = append(stmts, part)
		}
	}
	return stmts
}

func resolveMigrationPath(relative string) string {
	candidates := []string{
		relative,
		filepath.Join("..", "..", relative),
		filepath.Join("..", relative),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return relative
}

func (s *Store) EnsureSeedData(cfg config.Config) error {
	ctx := context.Background()
	existing, err := s.FindUserByEmail(cfg.SeedAdminEmail)
	if err == nil && existing.ID != "" {
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

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `insert into users (id, email, name, password_hash, created_at) values ($1,$2,$3,$4,$5) on conflict (email) do nothing`,
		admin.ID, admin.Email, admin.Name, admin.PasswordHash, admin.CreatedAt); err != nil {
		return err
	}

	var userID string
	if err := tx.QueryRow(ctx, `select id from users where email = $1`, admin.Email).Scan(&userID); err != nil {
		return err
	}
	if userID != admin.ID {
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `insert into workspaces (id, name, created_by, created_at) values ($1,$2,$3,$4)`,
		ws.ID, ws.Name, ws.CreatedBy, ws.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into memberships (id, workspace_id, user_id, role, created_at) values ($1,$2,$3,$4,$5)`,
		member.ID, member.WorkspaceID, member.UserID, string(member.Role), member.CreatedAt); err != nil {
		return err
	}
	capabilities, err := json.Marshal(defaultModel.Capabilities)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into models (id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, capabilities, enabled, is_default, created_by, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		defaultModel.ID,
		defaultModel.WorkspaceID,
		defaultModel.Name,
		defaultModel.Provider,
		defaultModel.APIBaseURL,
		defaultModel.APIKey,
		defaultModel.ModelKey,
		defaultModel.Description,
		defaultModel.ContextWindow,
		defaultModel.MaxOutputTokens,
		capabilities,
		defaultModel.Enabled,
		defaultModel.IsDefault,
		defaultModel.CreatedBy,
		defaultModel.CreatedAt,
		defaultModel.UpdatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) FindUserByEmail(email string) (auth.User, error) {
	row := s.pool.QueryRow(context.Background(), `select id, email, name, password_hash, created_at from users where lower(email) = lower($1)`, email)
	var item auth.User
	if err := row.Scan(&item.ID, &item.Email, &item.Name, &item.PasswordHash, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, errors.New("user not found")
		}
		return auth.User{}, err
	}
	return item, nil
}

func (s *Store) FindUserByID(userID string) (auth.User, error) {
	row := s.pool.QueryRow(context.Background(), `select id, email, name, password_hash, created_at from users where id = $1`, userID)
	var item auth.User
	if err := row.Scan(&item.ID, &item.Email, &item.Name, &item.PasswordHash, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, errors.New("user not found")
		}
		return auth.User{}, err
	}
	return item, nil
}

func (s *Store) SaveSessionToken(token auth.SessionToken) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into auth_sessions (token, user_id, expires_at)
		values ($1, $2, $3)
		on conflict (token) do update set user_id = excluded.user_id, expires_at = excluded.expires_at
	`, token.Token, token.UserID, token.ExpiresAt)
	return err
}

func (s *Store) FindSessionToken(token string) (auth.SessionToken, error) {
	row := s.pool.QueryRow(context.Background(), `select token, user_id, expires_at from auth_sessions where token = $1`, token)
	var item auth.SessionToken
	if err := row.Scan(&item.Token, &item.UserID, &item.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.SessionToken{}, errors.New("token not found")
		}
		return auth.SessionToken{}, err
	}
	return item, nil
}

func (s *Store) ListWorkspacesByUser(userID string) ([]workspace.Workspace, error) {
	rows, err := s.pool.Query(context.Background(), `
		select w.id, w.name, w.created_by, w.created_at
		from workspaces w
		join memberships m on m.workspace_id = w.id
		where m.user_id = $1
		order by w.created_at asc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]workspace.Workspace, 0)
	for rows.Next() {
		var item workspace.Workspace
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UserInWorkspace(userID, workspaceID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(context.Background(), `
		select exists(select 1 from memberships where user_id = $1 and workspace_id = $2)
	`, userID, workspaceID).Scan(&exists)
	return exists, err
}

func (s *Store) CreateWorkspace(item workspace.Workspace, member workspace.Membership) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `insert into workspaces (id, name, created_by, created_at) values ($1,$2,$3,$4)`,
		item.ID, item.Name, item.CreatedBy, item.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into memberships (id, workspace_id, user_id, role, created_at) values ($1,$2,$3,$4,$5)`,
		member.ID, member.WorkspaceID, member.UserID, string(member.Role), member.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveAgent(item agent.Agent) error {
	skillPolicy, err := json.Marshal(item.SkillPolicy)
	if err != nil {
		return err
	}
	policy, err := json.Marshal(item.ToolPolicy)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into agents (id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, published_version_id, created_by, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, item.ID, item.WorkspaceID, item.Name, item.Description, item.SystemPrompt, item.Model, skillPolicy, policy, item.RuntimePolicy, nullIfEmpty(item.PublishedVerID), item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateAgent(item agent.Agent) error {
	skillPolicy, err := json.Marshal(item.SkillPolicy)
	if err != nil {
		return err
	}
	policy, err := json.Marshal(item.ToolPolicy)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		update agents
		set name = $2, description = $3, system_prompt = $4, model = $5, skill_policy = $6, tool_policy = $7, runtime_policy = $8, published_version_id = $9, updated_at = $10
		where id = $1
	`, item.ID, item.Name, item.Description, item.SystemPrompt, item.Model, skillPolicy, policy, item.RuntimePolicy, nullIfEmpty(item.PublishedVerID), item.UpdatedAt)
	return err
}

func (s *Store) FindAgentByID(agentID string) (agent.Agent, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, coalesce(published_version_id, ''), created_by, created_at, updated_at
		from agents where id = $1
	`, agentID)
	return scanAgent(row)
}

func (s *Store) ListAgents(workspaceID string) ([]agent.Agent, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, description, system_prompt, model, skill_policy, tool_policy, runtime_policy, coalesce(published_version_id, ''), created_by, created_at, updated_at
		from agents where workspace_id = $1
		order by created_at asc
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]agent.Agent, 0)
	for rows.Next() {
		item, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CountAgentVersions(agentID string) (int, error) {
	var count int
	err := s.pool.QueryRow(context.Background(), `select count(*) from agent_versions where agent_id = $1`, agentID).Scan(&count)
	return count, err
}

func (s *Store) SaveAgentVersion(item agent.Version) error {
	snapshot, err := json.Marshal(item.Snapshot)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into agent_versions (id, agent_id, version_number, snapshot, created_by, created_at)
		values ($1,$2,$3,$4,$5,$6)
	`, item.ID, item.AgentID, item.VersionNumber, snapshot, item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) FindAgentVersion(versionID string) (agent.Version, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, agent_id, version_number, snapshot, created_by, created_at
		from agent_versions where id = $1
	`, versionID)
	var item agent.Version
	var snapshot []byte
	if err := row.Scan(&item.ID, &item.AgentID, &item.VersionNumber, &snapshot, &item.CreatedBy, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return agent.Version{}, errors.New("agent version not found")
		}
		return agent.Version{}, err
	}
	if err := json.Unmarshal(snapshot, &item.Snapshot); err != nil {
		return agent.Version{}, err
	}
	return item, nil
}

func (s *Store) SaveTool(item tool.Tool) error {
	schema, err := json.Marshal(item.Schema)
	if err != nil {
		return err
	}
	configJSON, err := json.Marshal(item.Config)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into tools (id, workspace_id, name, scope, description, schema, config, requires_approval, enabled, created_by, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, item.ID, item.WorkspaceID, item.Name, item.Scope, item.Description, schema, configJSON, item.RequiresApproval, item.Enabled, item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) UpdateTool(item tool.Tool) error {
	schema, err := json.Marshal(item.Schema)
	if err != nil {
		return err
	}
	configJSON, err := json.Marshal(item.Config)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		update tools
		set name = $2, scope = $3, description = $4, schema = $5, config = $6, requires_approval = $7, enabled = $8
		where id = $1
	`, item.ID, item.Name, item.Scope, item.Description, schema, configJSON, item.RequiresApproval, item.Enabled)
	return err
}

func (s *Store) DeleteTool(workspaceID, toolID string) error {
	tag, err := s.pool.Exec(context.Background(), `
		delete from tools where id = $1 and workspace_id = $2
	`, toolID, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("tool not found")
	}
	return nil
}

func (s *Store) ListTools(workspaceID string) ([]tool.Tool, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, scope, description, schema, config, requires_approval, enabled, created_by, created_at
		from tools where workspace_id = $1
		order by created_at asc
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]tool.Tool, 0)
	for rows.Next() {
		item, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) FindToolByID(toolID string) (tool.Tool, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, name, scope, description, schema, config, requires_approval, enabled, created_by, created_at
		from tools where id = $1
	`, toolID)
	return scanTool(row)
}

func (s *Store) ListPlatformTools() ([]tool.Tool, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, scope, description, schema, config, requires_approval, enabled, created_by, created_at
		from tools where scope = $1
		order by created_at asc
	`, tool.ScopePlatform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]tool.Tool, 0)
	for rows.Next() {
		item, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListUserTools(userID string) ([]tool.Tool, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, scope, description, schema, config, requires_approval, enabled, created_by, created_at
		from tools where scope = $1 and created_by = $2
		order by created_at asc
	`, tool.ScopePersonal, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]tool.Tool, 0)
	for rows.Next() {
		item, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) InstallTool(userID, toolID string) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into user_tool_installs (user_id, tool_id, created_at)
		values ($1,$2,$3)
		on conflict (user_id, tool_id) do nothing
	`, userID, toolID, time.Now().UTC())
	return err
}

func (s *Store) UninstallTool(userID, toolID string) error {
	_, err := s.pool.Exec(context.Background(), `
		delete from user_tool_installs where user_id = $1 and tool_id = $2
	`, userID, toolID)
	return err
}

func (s *Store) ListInstalledToolIDs(userID string) ([]string, error) {
	rows, err := s.pool.Query(context.Background(), `
		select tool_id from user_tool_installs where user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (s *Store) SaveSkill(item skill.Skill) error {
	schema, err := json.Marshal(item.Schema)
	if err != nil {
		return err
	}
	configJSON, err := json.Marshal(item.Config)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into skills (id, workspace_id, name, slug, scope, description, version, entry, schema, config, enabled, created_by, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`, item.ID, item.WorkspaceID, item.Name, item.Slug, item.Scope, item.Description, item.Version, item.Entry, schema, configJSON, item.Enabled, item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateSkill(item skill.Skill) error {
	schema, err := json.Marshal(item.Schema)
	if err != nil {
		return err
	}
	configJSON, err := json.Marshal(item.Config)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		update skills
		set name = $2, slug = $3, scope = $4, description = $5, version = $6, entry = $7, schema = $8, config = $9, enabled = $10, updated_at = $11
		where id = $1
	`, item.ID, item.Name, item.Slug, item.Scope, item.Description, item.Version, item.Entry, schema, configJSON, item.Enabled, item.UpdatedAt)
	return err
}

func (s *Store) DeleteSkill(workspaceID, skillID string) error {
	tag, err := s.pool.Exec(context.Background(), `
		delete from skills where id = $1 and workspace_id = $2
	`, skillID, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("skill not found")
	}
	return nil
}

func (s *Store) FindSkillByID(skillID string) (skill.Skill, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, name, slug, scope, description, version, entry, schema, config, enabled, created_by, created_at, updated_at
		from skills where id = $1
	`, skillID)
	return scanSkill(row)
}

func (s *Store) ListSkills(workspaceID string) ([]skill.Skill, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, slug, scope, description, version, entry, schema, config, enabled, created_by, created_at, updated_at
		from skills where workspace_id = $1
		order by created_at asc
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]skill.Skill, 0)
	for rows.Next() {
		item, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListPlatformSkills() ([]skill.Skill, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, slug, scope, description, version, entry, schema, config, enabled, created_by, created_at, updated_at
		from skills where scope = $1
		order by created_at asc
	`, skill.ScopePlatform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]skill.Skill, 0)
	for rows.Next() {
		item, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListUserSkills(userID string) ([]skill.Skill, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, slug, scope, description, version, entry, schema, config, enabled, created_by, created_at, updated_at
		from skills where scope = $1 and created_by = $2
		order by created_at asc
	`, skill.ScopePersonal, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]skill.Skill, 0)
	for rows.Next() {
		item, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) InstallSkill(userID, skillID string) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into user_skill_installs (user_id, skill_id, created_at)
		values ($1,$2,$3)
		on conflict (user_id, skill_id) do nothing
	`, userID, skillID, time.Now().UTC())
	return err
}

func (s *Store) UninstallSkill(userID, skillID string) error {
	_, err := s.pool.Exec(context.Background(), `
		delete from user_skill_installs where user_id = $1 and skill_id = $2
	`, userID, skillID)
	return err
}

func (s *Store) ListInstalledSkillIDs(userID string) ([]string, error) {
	rows, err := s.pool.Query(context.Background(), `
		select skill_id from user_skill_installs where user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (s *Store) SaveModel(item model.Model) error {
	capabilities, err := json.Marshal(item.Capabilities)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into models (id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, capabilities, enabled, is_default, created_by, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, item.ID, item.WorkspaceID, item.Name, item.Provider, item.APIBaseURL, item.APIKey, item.ModelKey, item.Description, item.ContextWindow, item.MaxOutputTokens, capabilities, item.Enabled, item.IsDefault, item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateModel(item model.Model) error {
	capabilities, err := json.Marshal(item.Capabilities)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		update models
		set name = $2, provider = $3, api_base_url = $4, api_key = $5, model_key = $6, description = $7, context_window = $8, max_output_tokens = $9, capabilities = $10, enabled = $11, is_default = $12, updated_at = $13
		where id = $1
	`, item.ID, item.Name, item.Provider, item.APIBaseURL, item.APIKey, item.ModelKey, item.Description, item.ContextWindow, item.MaxOutputTokens, capabilities, item.Enabled, item.IsDefault, item.UpdatedAt)
	return err
}

func (s *Store) DeleteModel(workspaceID, modelID string) error {
	tag, err := s.pool.Exec(context.Background(), `
		delete from models where id = $1 and workspace_id = $2
	`, modelID, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("model not found")
	}
	return nil
}

func (s *Store) FindModelByID(modelID string) (model.Model, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, capabilities, enabled, is_default, created_by, created_at, updated_at
		from models where id = $1
	`, modelID)
	return scanModel(row)
}

func (s *Store) FindModelByKey(workspaceID, modelKey string) (model.Model, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, capabilities, enabled, is_default, created_by, created_at, updated_at
		from models where workspace_id = $1 and model_key = $2
	`, workspaceID, modelKey)
	return scanModel(row)
}

func (s *Store) ListModels(workspaceID string) ([]model.Model, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, name, provider, api_base_url, api_key, model_key, description, context_window, max_output_tokens, capabilities, enabled, is_default, created_by, created_at, updated_at
		from models where workspace_id = $1
		order by is_default desc, created_at asc
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.Model, 0)
	for rows.Next() {
		item, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveSession(item session.Session) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into sessions (id, workspace_id, agent_id, created_by, title, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7)
	`, item.ID, item.WorkspaceID, item.AgentID, item.CreatedBy, item.Title, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) ListSessionsByAgent(userID, agentID string) ([]session.Session, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, agent_id, created_by, title, created_at, updated_at
		from sessions
		where created_by = $1 and agent_id = $2
		order by updated_at desc, created_at desc
	`, userID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]session.Session, 0)
	for rows.Next() {
		var item session.Session
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.AgentID, &item.CreatedBy, &item.Title, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveMessage(item session.Message) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into messages (id, session_id, role, content, trace_id, created_at)
		values ($1,$2,$3,$4,$5,$6)
	`, item.ID, item.SessionID, string(item.Role), item.Content, item.TraceID, item.CreatedAt)
	return err
}

func (s *Store) ListMessages(sessionID string) ([]session.Message, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, session_id, role, content, trace_id, created_at
		from messages where session_id = $1
		order by created_at asc
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]session.Message, 0)
	for rows.Next() {
		var item session.Message
		var role string
		if err := rows.Scan(&item.ID, &item.SessionID, &role, &item.Content, &item.TraceID, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Role = session.MessageRole(role)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveSchedule(item task.Schedule) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into schedules (id, workspace_id, agent_id, name, prompt, cron, interval, next_run_at, created_by, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, item.ID, item.WorkspaceID, item.AgentID, item.Name, item.Prompt, item.Cron, item.Interval, item.NextRunAt, item.CreatedBy, item.CreatedAt)
	return err
}

func (s *Store) ListDueSchedules(now time.Time) ([]task.Schedule, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, workspace_id, agent_id, name, prompt, cron, interval, next_run_at, created_by, created_at
		from schedules where next_run_at <= $1
		order by next_run_at asc
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]task.Schedule, 0)
	for rows.Next() {
		var item task.Schedule
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.AgentID, &item.Name, &item.Prompt, &item.Cron, &item.Interval, &item.NextRunAt, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateSchedule(item task.Schedule) error {
	_, err := s.pool.Exec(context.Background(), `
		update schedules
		set name = $2, prompt = $3, cron = $4, interval = $5, next_run_at = $6
		where id = $1
	`, item.ID, item.Name, item.Prompt, item.Cron, item.Interval, item.NextRunAt)
	return err
}

func (s *Store) SaveTask(item task.Task) error {
	toolCalls, err := json.Marshal(item.ToolCalls)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into tasks (id, trace_id, workspace_id, agent_id, session_id, prompt, model, reasoning, status, result, error, created_by, approval_id, tool_calls, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, item.ID, item.TraceID, item.WorkspaceID, item.AgentID, item.SessionID, item.Prompt, item.Model, item.Reasoning, string(item.Status), item.Result, item.Error, item.CreatedBy, nullIfEmpty(item.ApprovalID), toolCalls, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateTask(item task.Task) error {
	toolCalls, err := json.Marshal(item.ToolCalls)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		update tasks
		set model = $2, reasoning = $3, status = $4, result = $5, error = $6, approval_id = $7, tool_calls = $8, updated_at = $9
		where id = $1
	`, item.ID, item.Model, item.Reasoning, string(item.Status), item.Result, item.Error, nullIfEmpty(item.ApprovalID), toolCalls, item.UpdatedAt)
	return err
}

func (s *Store) FindTaskByID(taskID string) (task.Task, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, trace_id, workspace_id, agent_id, session_id, prompt, model, reasoning, status, result, error, created_by, coalesce(approval_id, ''), tool_calls, created_at, updated_at
		from tasks where id = $1
	`, taskID)
	var item task.Task
	var status string
	var toolCalls []byte
	if err := row.Scan(&item.ID, &item.TraceID, &item.WorkspaceID, &item.AgentID, &item.SessionID, &item.Prompt, &item.Model, &item.Reasoning, &status, &item.Result, &item.Error, &item.CreatedBy, &item.ApprovalID, &toolCalls, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return task.Task{}, errors.New("task not found")
		}
		return task.Task{}, err
	}
	item.Status = task.Status(status)
	if err := json.Unmarshal(toolCalls, &item.ToolCalls); err != nil {
		return task.Task{}, err
	}
	return item, nil
}

func (s *Store) ListTasksBySession(sessionID string) ([]task.Task, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, trace_id, workspace_id, agent_id, session_id, model, reasoning, prompt, status, result, error, created_by, coalesce(approval_id, ''), tool_calls, created_at, updated_at
		from tasks where session_id = $1
		order by created_at asc
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]task.Task, 0)
	for rows.Next() {
		var item task.Task
		var status string
		var toolCalls []byte
		if err := rows.Scan(&item.ID, &item.TraceID, &item.WorkspaceID, &item.AgentID, &item.SessionID, &item.Model, &item.Reasoning, &item.Prompt, &status, &item.Result, &item.Error, &item.CreatedBy, &item.ApprovalID, &toolCalls, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Status = task.Status(status)
		if err := json.Unmarshal(toolCalls, &item.ToolCalls); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveTaskStep(item task.Step) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into task_steps (id, task_id, name, status, output, error, sequence, approval_ref, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, item.ID, item.TaskID, item.Name, string(item.Status), item.Output, item.Error, item.Sequence, item.ApprovalRef, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) SaveTaskSteps(items []task.Step) error {
	if len(items) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into task_steps (id, task_id, name, status, output, error, sequence, approval_ref, created_at, updated_at)
			values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, item.ID, item.TaskID, item.Name, string(item.Status), item.Output, item.Error, item.Sequence, item.ApprovalRef, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ReplaceTaskSteps(taskID string, items []task.Step) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from task_steps where task_id = $1`, taskID); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into task_steps (id, task_id, name, status, output, error, sequence, approval_ref, created_at, updated_at)
			values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, item.ID, item.TaskID, item.Name, string(item.Status), item.Output, item.Error, item.Sequence, item.ApprovalRef, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListTaskSteps(taskID string) ([]task.Step, error) {
	rows, err := s.pool.Query(context.Background(), `
		select id, task_id, name, status, output, error, sequence, approval_ref, created_at, updated_at
		from task_steps where task_id = $1
		order by sequence asc, created_at asc
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]task.Step, 0)
	for rows.Next() {
		var item task.Step
		var status string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Name, &status, &item.Output, &item.Error, &item.Sequence, &item.ApprovalRef, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Status = task.StepStatus(status)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveApproval(item approval.Approval) error {
	_, err := s.pool.Exec(context.Background(), `
		insert into approvals (id, workspace_id, task_id, step_id, reason, status, decision_by, decision_at, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, item.ID, item.WorkspaceID, item.TaskID, item.StepID, item.Reason, string(item.Status), item.DecisionBy, nullTime(item.DecisionAt), item.CreatedAt)
	return err
}

func (s *Store) UpdateApproval(item approval.Approval) error {
	_, err := s.pool.Exec(context.Background(), `
		update approvals
		set status = $2, decision_by = $3, decision_at = $4
		where id = $1
	`, item.ID, string(item.Status), item.DecisionBy, nullTime(item.DecisionAt))
	return err
}

func (s *Store) FindApprovalByID(approvalID string) (approval.Approval, error) {
	row := s.pool.QueryRow(context.Background(), `
		select id, workspace_id, task_id, step_id, reason, status, coalesce(decision_by, ''), decision_at, created_at
		from approvals where id = $1
	`, approvalID)
	var item approval.Approval
	var status string
	var decisionAt *time.Time
	if err := row.Scan(&item.ID, &item.WorkspaceID, &item.TaskID, &item.StepID, &item.Reason, &status, &item.DecisionBy, &decisionAt, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.Approval{}, errors.New("approval not found")
		}
		return approval.Approval{}, err
	}
	item.Status = approval.Status(status)
	if decisionAt != nil {
		item.DecisionAt = *decisionAt
	}
	return item, nil
}

func (s *Store) ListApprovals(workspaceID string) ([]approval.Approval, error) {
	query := `
		select id, workspace_id, task_id, step_id, reason, status, coalesce(decision_by, ''), decision_at, created_at
		from approvals
	`
	args := []any{}
	if workspaceID != "" {
		query += ` where workspace_id = $1`
		args = append(args, workspaceID)
	}
	query += ` order by created_at desc`

	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]approval.Approval, 0)
	for rows.Next() {
		var item approval.Approval
		var status string
		var decisionAt *time.Time
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.TaskID, &item.StepID, &item.Reason, &status, &item.DecisionBy, &decisionAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Status = approval.Status(status)
		if decisionAt != nil {
			item.DecisionAt = *decisionAt
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveAuditEvent(item audit.Event) error {
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		insert into audit_events (id, workspace_id, actor_id, action, resource, resource_id, metadata, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8)
	`, item.ID, item.WorkspaceID, item.ActorID, item.Action, item.Resource, item.ResourceID, metadata, item.CreatedAt)
	return err
}

func (s *Store) ListAuditEvents(workspaceID string) ([]audit.Event, error) {
	query := `
		select id, workspace_id, actor_id, action, resource, resource_id, metadata, created_at
		from audit_events
	`
	args := []any{}
	if workspaceID != "" {
		query += ` where workspace_id = $1`
		args = append(args, workspaceID)
	}
	query += ` order by created_at desc`

	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]audit.Event, 0)
	for rows.Next() {
		var item audit.Event
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.ActorID, &item.Action, &item.Resource, &item.ResourceID, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanAgent(scanner interface {
	Scan(dest ...any) error
}) (agent.Agent, error) {
	var item agent.Agent
	var skillPolicy []byte
	var policy []byte
	if err := scanner.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.Description, &item.SystemPrompt, &item.Model, &skillPolicy, &policy, &item.RuntimePolicy, &item.PublishedVerID, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return agent.Agent{}, errors.New("agent not found")
		}
		return agent.Agent{}, err
	}
	if err := json.Unmarshal(skillPolicy, &item.SkillPolicy); err != nil {
		return agent.Agent{}, err
	}
	if err := json.Unmarshal(policy, &item.ToolPolicy); err != nil {
		return agent.Agent{}, err
	}
	return item, nil
}

func scanTool(scanner interface {
	Scan(dest ...any) error
}) (tool.Tool, error) {
	var item tool.Tool
	var schemaRaw []byte
	var configRaw []byte
	if err := scanner.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.Scope, &item.Description, &schemaRaw, &configRaw, &item.RequiresApproval, &item.Enabled, &item.CreatedBy, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tool.Tool{}, errors.New("tool not found")
		}
		return tool.Tool{}, err
	}
	if err := json.Unmarshal(schemaRaw, &item.Schema); err != nil {
		return tool.Tool{}, err
	}
	if err := json.Unmarshal(configRaw, &item.Config); err != nil {
		return tool.Tool{}, err
	}
	return item, nil
}

func scanSkill(scanner interface {
	Scan(dest ...any) error
}) (skill.Skill, error) {
	var item skill.Skill
	var schemaRaw []byte
	var configRaw []byte
	if err := scanner.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.Slug, &item.Scope, &item.Description, &item.Version, &item.Entry, &schemaRaw, &configRaw, &item.Enabled, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return skill.Skill{}, errors.New("skill not found")
		}
		return skill.Skill{}, err
	}
	if err := json.Unmarshal(schemaRaw, &item.Schema); err != nil {
		return skill.Skill{}, err
	}
	if err := json.Unmarshal(configRaw, &item.Config); err != nil {
		return skill.Skill{}, err
	}
	return item, nil
}

func scanModel(scanner interface {
	Scan(dest ...any) error
}) (model.Model, error) {
	var item model.Model
	var capabilitiesRaw []byte
	if err := scanner.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.Provider, &item.APIBaseURL, &item.APIKey, &item.ModelKey, &item.Description, &item.ContextWindow, &item.MaxOutputTokens, &capabilitiesRaw, &item.Enabled, &item.IsDefault, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Model{}, errors.New("model not found")
		}
		return model.Model{}, err
	}
	if err := json.Unmarshal(capabilitiesRaw, &item.Capabilities); err != nil {
		return model.Model{}, err
	}
	return item, nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
