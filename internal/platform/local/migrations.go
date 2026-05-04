package local

import "fmt"

// migrate 创建本地 SQLite 数据库表结构
func (s *Store) migrate() error {
	migrations := []string{
		// 用户表
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,

		// 会话令牌表
		`CREATE TABLE IF NOT EXISTS session_tokens (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			expires_at DATETIME NOT NULL
		)`,

		// 工作区表
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL
		)`,

		// 工作区成员表
		`CREATE TABLE IF NOT EXISTS workspace_members (
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			user_id TEXT NOT NULL REFERENCES users(id),
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME NOT NULL,
			PRIMARY KEY (workspace_id, user_id)
		)`,

		// Agent 表
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			skill_policy TEXT,
			tool_policy TEXT,
			runtime_policy TEXT NOT NULL DEFAULT '',
			published_version_id TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Agent 版本表
		`CREATE TABLE IF NOT EXISTS agent_versions (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL REFERENCES agents(id),
			version INTEGER NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			snapshot TEXT,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL
		)`,

		// Tool/MCP 表
		`CREATE TABLE IF NOT EXISTS tools (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			name TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT 'platform',
			description TEXT NOT NULL DEFAULT '',
			schema_data TEXT,
			config TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL
		)`,

		// 用户 Tool 安装表
		`CREATE TABLE IF NOT EXISTS user_tool_installs (
			user_id TEXT NOT NULL REFERENCES users(id),
			tool_id TEXT NOT NULL REFERENCES tools(id),
			created_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, tool_id)
		)`,

		// Skill 表
		`CREATE TABLE IF NOT EXISTS skills (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT 'platform',
			description TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '1.0.0',
			entry TEXT NOT NULL DEFAULT '',
			schema_data TEXT,
			config TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// 用户 Skill 安装表
		`CREATE TABLE IF NOT EXISTS user_skill_installs (
			user_id TEXT NOT NULL REFERENCES users(id),
			skill_id TEXT NOT NULL REFERENCES skills(id),
			created_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, skill_id)
		)`,

		// Model 表
		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			name TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			api_base_url TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			model_key TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			context_window INTEGER NOT NULL DEFAULT 0,
			max_output_tokens INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Session 表
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			agent_id TEXT NOT NULL REFERENCES agents(id),
			created_by TEXT NOT NULL REFERENCES users(id),
			title TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Message 表
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id),
			role TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			trace_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		)`,

		// Schedule 表
		`CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			agent_id TEXT NOT NULL REFERENCES agents(id),
			name TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			cron TEXT NOT NULL DEFAULT '',
			interval TEXT NOT NULL DEFAULT '',
			next_run_at DATETIME,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL
		)`,

		// Task 表
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			agent_id TEXT NOT NULL REFERENCES agents(id),
			session_id TEXT NOT NULL REFERENCES sessions(id),
			model TEXT NOT NULL DEFAULT '',
			reasoning TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			result TEXT,
			error TEXT,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Task Step 表
		`CREATE TABLE IF NOT EXISTS task_steps (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL REFERENCES tasks(id),
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			output TEXT,
			error TEXT,
			sequence INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Approval 表
		`CREATE TABLE IF NOT EXISTS approvals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			task_id TEXT NOT NULL REFERENCES tasks(id),
			step_id TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			decision_by TEXT,
			decision_at DATETIME,
			created_at DATETIME NOT NULL
		)`,

		// Audit Event 表
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			actor_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metadata TEXT,
			created_at DATETIME NOT NULL
		)`,

		// 同步元数据表
		`CREATE TABLE IF NOT EXISTS sync_metadata (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			last_synced_at DATETIME,
			is_dirty INTEGER NOT NULL DEFAULT 0,
			deleted_at DATETIME,
			PRIMARY KEY (entity_type, entity_id)
		)`,

		// 创建索引
		`CREATE INDEX IF NOT EXISTS idx_agents_workspace ON agents(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tools_workspace ON tools(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_workspace ON skills(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_models_workspace ON models(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_workspace ON sessions(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_session ON tasks(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_dirty ON sync_metadata(is_dirty)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w\nSQL: %s", err, m)
		}
	}

	return nil
}
