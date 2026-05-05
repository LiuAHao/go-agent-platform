package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// JSONRPCRequest JSON-RPC 2.0 请求
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse JSON-RPC 2.0 响应
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError JSON-RPC 错误
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolDefinition MCP 工具定义
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolResult MCP 工具调用结果
type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError"`
}

// ToolContent 工具内容
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Client MCP 客户端
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	stderr    io.ReadCloser
	mu        sync.Mutex
	nextID    int
	connected bool
	tools     []ToolDefinition
}

// NewClient 创建 MCP 客户端
func NewClient() *Client {
	return &Client{
		nextID: 1,
	}
}

// Connect 连接到 MCP Server
func (c *Client) Connect(ctx context.Context, command string, args ...string) error {
	c.cmd = exec.CommandContext(ctx, command, args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}

	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	c.stdout = bufio.NewScanner(stdoutPipe)

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start mcp server: %w", err)
	}

	c.connected = true

	// 发送 initialize 请求
	if err := c.initialize(ctx); err != nil {
		c.Disconnect()
		return fmt.Errorf("initialize: %w", err)
	}

	// 获取工具列表
	if err := c.listTools(ctx); err != nil {
		c.Disconnect()
		return fmt.Errorf("list tools: %w", err)
	}

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	if !c.connected {
		return nil
	}

	c.connected = false
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

// IsConnected 是否已连接
func (c *Client) IsConnected() bool {
	return c.connected
}

// GetTools 获取工具列表
func (c *Client) GetTools() []ToolDefinition {
	return c.tools
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, input map[string]any) (*ToolResult, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	params := map[string]any{
		"name":      name,
		"arguments": input,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error: %s", resp.Error.Message)
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &result, nil
}

// initialize 发送初始化请求
func (c *Client) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "go-agent-platform",
			"version": "1.0.0",
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// 发送 initialized 通知
	notification := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notification)
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	return nil
}

// listTools 获取工具列表
func (c *Client) listTools(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []ToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("unmarshal tools: %w", err)
	}

	c.tools = result.Tools
	return nil
}

// sendRequest 发送请求
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// 读取响应
	done := make(chan *JSONRPCResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		for c.stdout.Scan() {
			line := c.stdout.Text()
			if line == "" {
				continue
			}

			var response JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &response); err != nil {
				continue
			}

			if response.ID == id {
				done <- &response
				return
			}
		}
		errCh <- fmt.Errorf("connection closed")
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case resp := <-done:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}
