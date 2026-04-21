package model

import "time"

type Model struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	APIBaseURL      string    `json:"api_base_url"`
	APIKey          string    `json:"api_key"`
	ModelKey        string    `json:"model_key"`
	Description     string    `json:"description"`
	ContextWindow   int       `json:"context_window"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	Capabilities    []string  `json:"capabilities"`
	Enabled         bool      `json:"enabled"`
	IsDefault       bool      `json:"is_default"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
