package mcp

import (
	"context"
	"fmt"
	"sync"
)

// Manager MCP 管理器
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // key: tool_id
	configs map[string]*ServerConfig
}

// ServerConfig MCP 服务器配置
type ServerConfig struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Transport string            `json:"transport"` // stdio, sse
}

// NewManager 创建 MCP 管理器
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		configs: make(map[string]*ServerConfig),
	}
}

// StartServer 启动 MCP 服务器
func (m *Manager) StartServer(ctx context.Context, config *ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已启动
	if _, exists := m.clients[config.ID]; exists {
		return fmt.Errorf("server %s already started", config.ID)
	}

	client := NewClient()
	if err := client.Connect(ctx, config.Command, config.Args...); err != nil {
		return fmt.Errorf("connect to mcp server: %w", err)
	}

	m.clients[config.ID] = client
	m.configs[config.ID] = config

	return nil
}

// StopServer 停止 MCP 服务器
func (m *Manager) StopServer(serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[serverID]
	if !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	if err := client.Disconnect(); err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}

	delete(m.clients, serverID)
	delete(m.configs, serverID)

	return nil
}

// GetClient 获取 MCP 客户端
func (m *Manager) GetClient(serverID string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[serverID]
	if !exists {
		return nil, fmt.Errorf("server %s not found", serverID)
	}

	return client, nil
}

// ListServers 列出所有服务器
func (m *Manager) ListServers() []ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]ServerConfig, 0, len(m.configs))
	for _, config := range m.configs {
		configs = append(configs, *config)
	}
	return configs
}

// GetServerTools 获取服务器的工具列表
func (m *Manager) GetServerTools(serverID string) ([]ToolDefinition, error) {
	client, err := m.GetClient(serverID)
	if err != nil {
		return nil, err
	}

	return client.GetTools(), nil
}

// GetAllTools 获取所有服务器的工具
func (m *Manager) GetAllTools() map[string][]ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]ToolDefinition)
	for id, client := range m.clients {
		result[id] = client.GetTools()
	}
	return result
}

// CallTool 调用工具
func (m *Manager) CallTool(ctx context.Context, serverID, toolName string, input map[string]any) (*ToolResult, error) {
	client, err := m.GetClient(serverID)
	if err != nil {
		return nil, err
	}

	return client.CallTool(ctx, toolName, input)
}

// Close 关闭所有服务器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for id, client := range m.clients {
		if err := client.Disconnect(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", id, err))
		}
	}

	m.clients = make(map[string]*Client)
	m.configs = make(map[string]*ServerConfig)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}
