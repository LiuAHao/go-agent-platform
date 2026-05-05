package app

import (
	"context"
	"fmt"

	"go-agent-platform/internal/platform/mcp"
)

// StartMCPServer 启动 MCP 服务器
func (a *Application) StartMCPServer(config *mcp.ServerConfig) error {
	if config.ID == "" {
		return fmt.Errorf("server id is required")
	}
	if config.Command == "" {
		return fmt.Errorf("command is required")
	}

	ctx := context.Background()
	return a.MCPManager.StartServer(ctx, config)
}

// StopMCPServer 停止 MCP 服务器
func (a *Application) StopMCPServer(serverID string) error {
	return a.MCPManager.StopServer(serverID)
}

// ListMCPServers 列出 MCP 服务器
func (a *Application) ListMCPServers() []mcp.ServerConfig {
	return a.MCPManager.ListServers()
}

// GetMCPServerTools 获取 MCP 服务器工具
func (a *Application) GetMCPServerTools(serverID string) ([]mcp.ToolDefinition, error) {
	return a.MCPManager.GetServerTools(serverID)
}

// GetAllMCPTools 获取所有 MCP 工具
func (a *Application) GetAllMCPTools() map[string][]mcp.ToolDefinition {
	return a.MCPManager.GetAllTools()
}

// CallMCPTool 调用 MCP 工具
func (a *Application) CallMCPTool(serverID, toolName string, input map[string]any) (*mcp.ToolResult, error) {
	ctx := context.Background()
	return a.MCPManager.CallTool(ctx, serverID, toolName, input)
}
