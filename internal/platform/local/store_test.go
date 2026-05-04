package local

import (
	"testing"
	"time"

	"go-agent-platform/internal/config"
	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/auth"
	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/session"
	"go-agent-platform/internal/domain/skill"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()

	tmpDir := t.TempDir()
	home, err := NewHome(tmpDir)
	if err != nil {
		t.Fatalf("NewHome failed: %v", err)
	}

	store, err := NewStore(home)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func TestStore_EnsureSeedData(t *testing.T) {
	store := setupTestStore(t)

	cfg := config.Config{
		SeedAdminEmail:    "test@example.com",
		SeedAdminPassword: "test123",
	}

	// 首次创建种子数据
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	// 验证管理员用户已创建
	user, err := store.FindUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("user.Email = %s, want test@example.com", user.Email)
	}

	// 再次调用应该幂等
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData should be idempotent: %v", err)
	}
}

func TestStore_UserOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建用户
	cfg := config.Config{
		SeedAdminEmail:    "user@example.com",
		SeedAdminPassword: "password",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	// 查找用户
	found, err := store.FindUserByEmail("user@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}

	if found.Email != "user@example.com" {
		t.Errorf("FindUserByEmail: got %s, want user@example.com", found.Email)
	}

	// 通过 ID 查找
	foundByID, err := store.FindUserByID(found.ID)
	if err != nil {
		t.Fatalf("FindUserByID failed: %v", err)
	}

	if foundByID.ID != found.ID {
		t.Errorf("FindUserByID: got %s, want %s", foundByID.ID, found.ID)
	}
}

func TestStore_SessionTokenOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建用户
	cfg := config.Config{
		SeedAdminEmail:    "admin@example.com",
		SeedAdminPassword: "password",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	user, err := store.FindUserByEmail("admin@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}

	// 保存会话令牌
	token := auth.SessionToken{
		Token:     "test-token-123",
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := store.SaveSessionToken(token); err != nil {
		t.Fatalf("SaveSessionToken failed: %v", err)
	}

	// 查找会话令牌
	found, err := store.FindSessionToken("test-token-123")
	if err != nil {
		t.Fatalf("FindSessionToken failed: %v", err)
	}

	if found.UserID != user.ID {
		t.Errorf("FindSessionToken: got UserID %s, want %s", found.UserID, user.ID)
	}
}

func TestStore_AgentOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建种子数据
	cfg := config.Config{
		SeedAdminEmail:    "admin@example.com",
		SeedAdminPassword: "admin",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	user, _ := store.FindUserByEmail("admin@example.com")

	// 创建 Agent
	now := time.Now().UTC()
	a := agent.Agent{
		ID:          "agent-001",
		WorkspaceID: "default",
		Name:        "Test Agent",
		Description: "A test agent",
		SystemPrompt: "You are a test agent",
		Model:       "gpt-4",
		CreatedBy:   user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := store.SaveAgent(a); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// 查找 Agent
	found, err := store.FindAgentByID("agent-001")
	if err != nil {
		t.Fatalf("FindAgentByID failed: %v", err)
	}

	if found.Name != a.Name {
		t.Errorf("FindAgentByID: got %s, want %s", found.Name, a.Name)
	}

	// 列出 Agents
	agents, err := store.ListAgents("default")
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(agents) != 1 {
		t.Errorf("ListAgents: got %d agents, want 1", len(agents))
	}

	// 更新 Agent
	a.Name = "Updated Agent"
	if err := store.UpdateAgent(a); err != nil {
		t.Fatalf("UpdateAgent failed: %v", err)
	}

	found, err = store.FindAgentByID("agent-001")
	if err != nil {
		t.Fatalf("FindAgentByID failed: %v", err)
	}

	if found.Name != "Updated Agent" {
		t.Errorf("UpdateAgent: got %s, want Updated Agent", found.Name)
	}
}

func TestStore_SkillOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建种子数据
	cfg := config.Config{
		SeedAdminEmail:    "admin@example.com",
		SeedAdminPassword: "admin",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	user, _ := store.FindUserByEmail("admin@example.com")

	// 创建 Skill
	now := time.Now().UTC()
	sk := skill.Skill{
		ID:          "skill-001",
		WorkspaceID: "default",
		Name:        "Test Skill",
		Slug:        "test-skill",
		Scope:       "platform",
		Description: "A test skill",
		Version:     "1.0.0",
		Entry:       "index.js",
		Enabled:     true,
		CreatedBy:   user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := store.SaveSkill(sk); err != nil {
		t.Fatalf("SaveSkill failed: %v", err)
	}

	// 查找 Skill
	found, err := store.FindSkillByID("skill-001")
	if err != nil {
		t.Fatalf("FindSkillByID failed: %v", err)
	}

	if found.Name != sk.Name {
		t.Errorf("FindSkillByID: got %s, want %s", found.Name, sk.Name)
	}

	// 列出平台 Skills
	skills, err := store.ListPlatformSkills()
	if err != nil {
		t.Fatalf("ListPlatformSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ListPlatformSkills: got %d skills, want 1", len(skills))
	}

	// 安装 Skill
	if err := store.InstallSkill(user.ID, "skill-001"); err != nil {
		t.Fatalf("InstallSkill failed: %v", err)
	}

	// 列出已安装的 Skill
	installed, err := store.ListInstalledSkillIDs(user.ID)
	if err != nil {
		t.Fatalf("ListInstalledSkillIDs failed: %v", err)
	}

	if len(installed) != 1 || installed[0] != "skill-001" {
		t.Errorf("ListInstalledSkillIDs: got %v, want [skill-001]", installed)
	}

	// 卸载 Skill
	if err := store.UninstallSkill(user.ID, "skill-001"); err != nil {
		t.Fatalf("UninstallSkill failed: %v", err)
	}

	installed, err = store.ListInstalledSkillIDs(user.ID)
	if err != nil {
		t.Fatalf("ListInstalledSkillIDs failed: %v", err)
	}

	if len(installed) != 0 {
		t.Errorf("ListInstalledSkillIDs after uninstall: got %d, want 0", len(installed))
	}
}

func TestStore_ModelOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建种子数据
	cfg := config.Config{
		SeedAdminEmail:    "admin@example.com",
		SeedAdminPassword: "admin",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	user, _ := store.FindUserByEmail("admin@example.com")

	// 创建 Model
	now := time.Now().UTC()
	m := model.Model{
		ID:              "model-001",
		WorkspaceID:     "default",
		Name:            "GPT-4",
		Provider:        "openai",
		APIBaseURL:      "https://api.openai.com/v1",
		APIKey:          "sk-test-key", // 本地存储
		ModelKey:        "gpt-4",
		Description:     "GPT-4 model",
		ContextWindow:   8192,
		MaxOutputTokens: 4096,
		IsDefault:       true,
		CreatedBy:       user.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := store.SaveModel(m); err != nil {
		t.Fatalf("SaveModel failed: %v", err)
	}

	// 查找 Model
	found, err := store.FindModelByID("model-001")
	if err != nil {
		t.Fatalf("FindModelByID failed: %v", err)
	}

	if found.Name != m.Name {
		t.Errorf("FindModelByID: got %s, want %s", found.Name, m.Name)
	}

	// 验证 API Key 存储在本地
	if found.APIKey != "sk-test-key" {
		t.Errorf("APIKey should be stored locally, got %s", found.APIKey)
	}

	// 列出 Models
	models, err := store.ListModels("default")
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("ListModels: got %d models, want 1", len(models))
	}
}

func TestStore_SessionAndMessageOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建种子数据
	cfg := config.Config{
		SeedAdminEmail:    "admin@example.com",
		SeedAdminPassword: "admin",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	user, _ := store.FindUserByEmail("admin@example.com")

	// 创建 Agent
	now := time.Now().UTC()
	a := agent.Agent{
		ID:          "agent-001",
		WorkspaceID: "default",
		Name:        "Test Agent",
		CreatedBy:   user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveAgent(a); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// 创建 Session
	sess := session.Session{
		ID:          "session-001",
		WorkspaceID: "default",
		AgentID:     "agent-001",
		CreatedBy:   user.ID,
		Title:       "Test Session",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// 创建 Messages
	msg1 := session.Message{
		ID:        "msg-001",
		SessionID: "session-001",
		Role:      session.RoleUser,
		Content:   "Hello",
		CreatedAt: now,
	}
	msg2 := session.Message{
		ID:        "msg-002",
		SessionID: "session-001",
		Role:      session.RoleAssistant,
		Content:   "Hi there!",
		CreatedAt: now.Add(time.Second),
	}

	if err := store.SaveMessage(msg1); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}
	if err := store.SaveMessage(msg2); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	// 列出 Messages
	messages, err := store.ListMessages("session-001")
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("ListMessages: got %d messages, want 2", len(messages))
	}

	// 验证消息顺序
	if messages[0].Content != "Hello" {
		t.Errorf("First message content: got %s, want Hello", messages[0].Content)
	}
	if messages[1].Content != "Hi there!" {
		t.Errorf("Second message content: got %s, want Hi there!", messages[1].Content)
	}

	// 列出 Sessions by Agent
	sessions, err := store.ListSessionsByAgent(user.ID, "agent-001")
	if err != nil {
		t.Fatalf("ListSessionsByAgent failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("ListSessionsByAgent: got %d sessions, want 1", len(sessions))
	}
}

func TestStore_WorkspaceOperations(t *testing.T) {
	store := setupTestStore(t)

	// 创建用户
	cfg := config.Config{
		SeedAdminEmail:    "user@example.com",
		SeedAdminPassword: "password",
	}
	if err := store.EnsureSeedData(cfg); err != nil {
		t.Fatalf("EnsureSeedData failed: %v", err)
	}

	// 获取用户
	user, err := store.FindUserByEmail("user@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}

	// 列出用户的工作区
	workspaces, err := store.ListWorkspacesByUser(user.ID)
	if err != nil {
		t.Fatalf("ListWorkspacesByUser failed: %v", err)
	}

	if len(workspaces) == 0 {
		t.Error("User should have at least one workspace")
	}

	// 检查用户是否在工作区中
	inWorkspace, err := store.UserInWorkspace(user.ID, workspaces[0].ID)
	if err != nil {
		t.Fatalf("UserInWorkspace failed: %v", err)
	}

	if !inWorkspace {
		t.Error("User should be in workspace")
	}
}
