package mcp_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-agent-platform/internal/platform/mcp"
)

func TestMCPClient(t *testing.T) {
	// 跳过测试如果没有安装 npx
	if testing.Short() {
		t.Skip("skipping mcp test in short mode")
	}

	// 使用用户 home 目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient()

	// 连接到 filesystem MCP server
	err = client.Connect(ctx, "npx", "-y", "@modelcontextprotocol/server-filesystem", homeDir)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Disconnect()

	// 检查连接状态
	if !client.IsConnected() {
		t.Fatal("should be connected")
	}

	// 获取工具列表
	tools := client.GetTools()
	if len(tools) == 0 {
		t.Fatal("should have tools")
	}

	t.Logf("Found %d tools:", len(tools))
	for _, tool := range tools {
		t.Logf("  - %s: %s", tool.Name, tool.Description)
	}

	// 调用一个工具 (list_directory)
	result, err := client.CallTool(ctx, "list_directory", map[string]any{
		"path": homeDir,
	})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	t.Logf("Tool result:")
	for _, content := range result.Content {
		t.Logf("  %s: %s", content.Type, content.Text)
	}
}

func TestMCPManager(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mcp test in short mode")
	}

	// 使用用户 home 目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := mcp.NewManager()
	defer manager.Close()

	// 启动服务器
	config := &mcp.ServerConfig{
		ID:      "test-filesystem",
		Name:    "Filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", homeDir},
	}

	if err := manager.StartServer(ctx, config); err != nil {
		t.Fatalf("start server failed: %v", err)
	}

	// 列出服务器
	servers := manager.ListServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	// 获取工具
	tools, err := manager.GetServerTools("test-filesystem")
	if err != nil {
		t.Fatalf("get tools failed: %v", err)
	}

	if len(tools) == 0 {
		t.Fatal("should have tools")
	}

	// 调用工具
	result, err := manager.CallTool(ctx, "test-filesystem", "list_directory", map[string]any{
		"path": homeDir,
	})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	t.Logf("Tool call succeeded")
	for _, content := range result.Content {
		t.Logf("  %s: %s", content.Type, content.Text)
	}
}
