package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider OpenAI 兼容的 LLM Provider
type OpenAIProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

// OpenAIRequest OpenAI API 请求
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// OpenAIMessage 消息
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAITool 工具定义
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction 函数定义
type OpenAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// OpenAIToolCall 工具调用
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// OpenAIResponse OpenAI API 响应
type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int            `json:"index"`
		Message      OpenAIMessage  `json:"message"`
		Delta        *OpenAIMessage `json:"delta,omitempty"`
		FinishReason string         `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// NewOpenAIProvider 创建 OpenAI Provider
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4"
	}

	return &OpenAIProvider{
		APIKey:  apiKey,
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Model:   model,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Chat 发送聊天请求
func (p *OpenAIProvider) Chat(messages []OpenAIMessage, tools []OpenAITool) (*OpenAIResponse, error) {
	req := OpenAIRequest{
		Model:       p.Model,
		Messages:    messages,
		Tools:       tools,
		Stream:      false,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result OpenAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// ChatStream 发送流式聊天请求
func (p *OpenAIProvider) ChatStream(messages []OpenAIMessage, tools []OpenAITool, callback func(chunk string)) (*OpenAIResponse, error) {
	req := OpenAIRequest{
		Model:       p.Model,
		Messages:    messages,
		Tools:       tools,
		Stream:      true,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var lastResponse *OpenAIResponse
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk OpenAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			if choice.Delta != nil && choice.Delta.Content != "" {
				callback(choice.Delta.Content)
			}
			if choice.FinishReason != "" {
				lastResponse = &chunk
			}
		}
	}

	return lastResponse, nil
}

// Plan 规划任务（实现 Provider 接口）
func (p *OpenAIProvider) Plan(prompt string) []string {
	messages := []OpenAIMessage{
		{
			Role:    "system",
			Content: "你是一个任务规划助手。根据用户的请求，分解为多个子任务。返回 JSON 数组格式的子任务列表，每个子任务是一个简短的描述。只返回 JSON 数组，不要其他内容。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	resp, err := p.Chat(messages, nil)
	if err != nil {
		// 降级到默认规划
		return []string{"analyze", "execute", "summarize"}
	}

	if len(resp.Choices) == 0 {
		return []string{"analyze", "execute", "summarize"}
	}

	content := resp.Choices[0].Message.Content

	// 尝试解析 JSON 数组
	var tasks []string
	if err := json.Unmarshal([]byte(content), &tasks); err != nil {
		// 如果解析失败，按行分割
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" {
				tasks = append(tasks, line)
			}
		}
	}

	if len(tasks) == 0 {
		return []string{"analyze", "execute", "summarize"}
	}

	return tasks
}

// Complete 完成任务（实现 Provider 接口）
func (p *OpenAIProvider) Complete(prompt string, toolOutputs []string) string {
	messages := []OpenAIMessage{
		{
			Role:    "system",
			Content: "你是一个智能助手。根据用户的请求和工具执行结果，生成最终的回答。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	if len(toolOutputs) > 0 {
		toolContext := "工具执行结果:\n"
		for i, output := range toolOutputs {
			toolContext += fmt.Sprintf("%d. %s\n", i+1, output)
		}
		messages = append(messages, OpenAIMessage{
			Role:    "user",
			Content: toolContext,
		})
	}

	resp, err := p.Chat(messages, nil)
	if err != nil {
		return fmt.Sprintf("任务完成，但生成回复时出错: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "任务完成"
	}

	return resp.Choices[0].Message.Content
}

// ChatWithTools 带工具的聊天
func (p *OpenAIProvider) ChatWithTools(messages []OpenAIMessage, tools []OpenAITool) (*OpenAIResponse, error) {
	return p.Chat(messages, tools)
}
