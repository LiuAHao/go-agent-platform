package llm_test

import (
	"os"
	"testing"

	"go-agent-platform/internal/platform/llm"
)

func TestOpenAIProvider(t *testing.T) {
	// 从环境变量获取 API Key
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
	}

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 测试基本聊天
	t.Run("BasicChat", func(t *testing.T) {
		messages := []llm.OpenAIMessage{
			{
				Role:    "user",
				Content: "你好，请用一句话介绍自己。",
			},
		}

		resp, err := provider.Chat(messages, nil)
		if err != nil {
			t.Fatalf("Chat failed: %v", err)
		}

		if len(resp.Choices) == 0 {
			t.Fatal("No response choices")
		}

		t.Logf("Response: %s", resp.Choices[0].Message.Content)
	})

	// 测试 Plan 功能
	t.Run("Plan", func(t *testing.T) {
		tasks := provider.Plan("帮我分析一下当前的市场趋势")
		if len(tasks) == 0 {
			t.Fatal("No tasks returned")
		}

		t.Logf("Planned tasks:")
		for i, task := range tasks {
			t.Logf("  %d. %s", i+1, task)
		}
	})

	// 测试 Complete 功能
	t.Run("Complete", func(t *testing.T) {
		result := provider.Complete("什么是人工智能？", nil)
		if result == "" {
			t.Fatal("Empty result")
		}

		t.Logf("Complete result: %s", result)
	})
}

func TestOpenAIProviderWithTools(t *testing.T) {
	// 从环境变量获取 API Key
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set, skipping test")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
	}

	provider := llm.NewOpenAIProvider(apiKey, baseURL, model)

	// 定义工具
	tools := []llm.OpenAITool{
		{
			Type: "function",
			Function: llm.OpenAIFunction{
				Name:        "get_weather",
				Description: "获取指定城市的天气信息",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "城市名称",
						},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	// 测试带工具的聊天
	messages := []llm.OpenAIMessage{
		{
			Role:    "user",
			Content: "北京今天天气怎么样？",
		},
	}

	resp, err := provider.Chat(messages, tools)
	if err != nil {
		t.Fatalf("Chat with tools failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No response choices")
	}

	choice := resp.Choices[0]

	// 检查是否有工具调用
	if len(choice.Message.ToolCalls) > 0 {
		t.Logf("Tool calls detected:")
		for _, tc := range choice.Message.ToolCalls {
			t.Logf("  - %s(%s)", tc.Function.Name, tc.Function.Arguments)
		}
	} else {
		t.Logf("No tool calls, response: %s", choice.Message.Content)
	}
}
