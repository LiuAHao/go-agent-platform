package executor

import "context"

// SkillExecutor Skill 执行接口
type SkillExecutor interface {
	// ID 返回 Skill ID
	ID() string

	// Name 返回 Skill 名称
	Name() string

	// Execute 执行 Skill
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)

	// Schema 返回 Skill 的输入输出 Schema
	Schema() SkillSchema

	// HealthCheck 健康检查
	HealthCheck() error

	// Close 关闭执行器
	Close() error
}

// SkillSchema Skill 的 Schema 定义
type SkillSchema struct {
	Input  map[string]any `json:"input"`
	Output map[string]any `json:"output"`
}

// MCPExecutor MCP 工具执行接口
type MCPExecutor interface {
	// ID 返回 MCP ID
	ID() string

	// Name 返回 MCP 名称
	Name() string

	// Connect 连接到 MCP Server
	Connect(ctx context.Context) error

	// CallTool 调用 MCP 工具
	CallTool(ctx context.Context, toolName string, input map[string]any) (map[string]any, error)

	// ListTools 列出可用工具
	ListTools(ctx context.Context) ([]MCPTool, error)

	// Disconnect 断开连接
	Disconnect() error

	// IsConnected 是否已连接
	IsConnected() bool
}

// MCPTool MCP 工具定义
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ExecutorManager 执行器管理器
type ExecutorManager interface {
	// RegisterSkill 注册 Skill 执行器
	RegisterSkill(executor SkillExecutor) error

	// UnregisterSkill 注销 Skill 执行器
	UninstallSkill(skillID string) error

	// GetSkill 获取 Skill 执行器
	GetSkill(skillID string) (SkillExecutor, error)

	// ListSkills 列出所有已注册的 Skill
	ListSkills() []SkillExecutor

	// RegisterMCP 注册 MCP 执行器
	RegisterMCP(executor MCPExecutor) error

	// UnregisterMCP 注销 MCP 执行器
	UninstallMCP(mcpID string) error

	// GetMCP 获取 MCP 执行器
	GetMCP(mcpID string) (MCPExecutor, error)

	// ListMCPs 列出所有已注册的 MCP
	ListMCPs() []MCPExecutor

	// Close 关闭所有执行器
	Close() error
}
