package app

import (
	"errors"
	"strings"
	"time"

	"go-agent-platform/internal/domain/model"
	"go-agent-platform/internal/domain/shared"
)

type CreateModelRequest struct {
	WorkspaceID     string   `json:"workspace_id"`
	Name            string   `json:"name"`
	Provider        string   `json:"provider"`
	APIBaseURL      string   `json:"api_base_url"`
	APIKey          string   `json:"api_key"`
	ModelKey        string   `json:"model_key"`
	Description     string   `json:"description"`
	ContextWindow   int      `json:"context_window"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Capabilities    []string `json:"capabilities"`
	Enabled         bool     `json:"enabled"`
	IsDefault       bool     `json:"is_default"`
}

type UpdateModelRequest struct {
	Name            string   `json:"name"`
	Provider        string   `json:"provider"`
	APIBaseURL      string   `json:"api_base_url"`
	APIKey          string   `json:"api_key"`
	ModelKey        string   `json:"model_key"`
	Description     string   `json:"description"`
	ContextWindow   int      `json:"context_window"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Capabilities    []string `json:"capabilities"`
	Enabled         *bool    `json:"enabled"`
	IsDefault       *bool    `json:"is_default"`
}

func (a *Application) CreateModel(userID string, req CreateModelRequest) (model.Model, error) {
	workspaceID, err := a.resolveWorkspaceID(userID, req.WorkspaceID)
	if err != nil {
		return model.Model{}, err
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return model.Model{}, errors.New("workspace access denied")
	}

	name := strings.TrimSpace(req.Name)
	modelKey := strings.TrimSpace(req.ModelKey)
	if name == "" || modelKey == "" {
		return model.Model{}, errors.New("model name and model key are required")
	}
	if _, err := a.Store.FindModelByKey(workspaceID, modelKey); err == nil {
		return model.Model{}, errors.New("model key already exists in workspace")
	}

	now := time.Now().UTC()
	item := model.Model{
		ID:              shared.NewID("model"),
		WorkspaceID:     workspaceID,
		Name:            name,
		Provider:        strings.TrimSpace(req.Provider),
		APIBaseURL:      strings.TrimSpace(req.APIBaseURL),
		APIKey:          strings.TrimSpace(req.APIKey),
		ModelKey:        modelKey,
		Description:     strings.TrimSpace(req.Description),
		ContextWindow:   req.ContextWindow,
		MaxOutputTokens: req.MaxOutputTokens,
		Capabilities:    compactStrings(req.Capabilities),
		Enabled:         true,
		IsDefault:       req.IsDefault,
		CreatedBy:       userID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if req.Enabled {
		item.Enabled = true
	}

	if item.IsDefault {
		if err := a.clearDefaultModel(workspaceID, item.ID); err != nil {
			return model.Model{}, err
		}
	}
	if err := a.Store.SaveModel(item); err != nil {
		return model.Model{}, err
	}
	a.appendAudit(workspaceID, userID, "model.create", "model", item.ID, map[string]any{"model_key": item.ModelKey})
	return item, nil
}

func (a *Application) UpdateModel(userID, modelID string, req UpdateModelRequest) (model.Model, error) {
	item, err := a.Store.FindModelByID(modelID)
	if err != nil {
		return model.Model{}, errors.New("model not found")
	}
	if ok, err := a.userInWorkspace(userID, item.WorkspaceID); err != nil || !ok {
		return model.Model{}, errors.New("workspace access denied")
	}

	if value := strings.TrimSpace(req.Name); value != "" {
		item.Name = value
	}
	if value := strings.TrimSpace(req.Provider); value != "" {
		item.Provider = value
	}
	if value := strings.TrimSpace(req.APIBaseURL); value != "" {
		item.APIBaseURL = value
	}
	if value := strings.TrimSpace(req.APIKey); value != "" {
		item.APIKey = value
	}
	if value := strings.TrimSpace(req.ModelKey); value != "" {
		if value != item.ModelKey {
			if _, err := a.Store.FindModelByKey(item.WorkspaceID, value); err == nil {
				return model.Model{}, errors.New("model key already exists in workspace")
			}
		}
		item.ModelKey = value
	}
	if req.Description != "" {
		item.Description = strings.TrimSpace(req.Description)
	}
	if req.ContextWindow > 0 {
		item.ContextWindow = req.ContextWindow
	}
	if req.MaxOutputTokens > 0 {
		item.MaxOutputTokens = req.MaxOutputTokens
	}
	if req.Capabilities != nil {
		item.Capabilities = compactStrings(req.Capabilities)
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if req.IsDefault != nil {
		item.IsDefault = *req.IsDefault
		if item.IsDefault {
			if err := a.clearDefaultModel(item.WorkspaceID, item.ID); err != nil {
				return model.Model{}, err
			}
			item.IsDefault = true
		}
	}
	item.UpdatedAt = time.Now().UTC()

	if err := a.Store.UpdateModel(item); err != nil {
		return model.Model{}, err
	}
	a.appendAudit(item.WorkspaceID, userID, "model.update", "model", item.ID, map[string]any{"enabled": item.Enabled, "is_default": item.IsDefault})
	return item, nil
}

func (a *Application) DeleteModel(userID, workspaceID, modelID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		item, err := a.Store.FindModelByID(modelID)
		if err != nil {
			return errors.New("model not found")
		}
		workspaceID = item.WorkspaceID
	}
	if ok, err := a.userInWorkspace(userID, workspaceID); err != nil || !ok {
		return errors.New("workspace access denied")
	}
	if err := a.Store.DeleteModel(workspaceID, modelID); err != nil {
		return err
	}
	a.appendAudit(workspaceID, userID, "model.delete", "model", modelID, nil)
	return nil
}

func (a *Application) ListModels(workspaceID string) []model.Model {
	items, err := a.Store.ListModels(workspaceID)
	if err != nil {
		return []model.Model{}
	}
	return items
}

func (a *Application) clearDefaultModel(workspaceID string, exceptIDs ...string) error {
	excluded := make(map[string]struct{}, len(exceptIDs))
	for _, id := range exceptIDs {
		excluded[id] = struct{}{}
	}
	items, err := a.Store.ListModels(workspaceID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, skip := excluded[item.ID]; skip {
			continue
		}
		if !item.IsDefault {
			continue
		}
		item.IsDefault = false
		item.UpdatedAt = time.Now().UTC()
		if err := a.Store.UpdateModel(item); err != nil {
			return err
		}
	}
	return nil
}

func compactStrings(items []string) []string {
	compacted := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(item); value != "" {
			compacted = append(compacted, value)
		}
	}
	return compacted
}

func defaultMap(item map[string]any) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return item
}

func slugify(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "--", "-")
	return strings.Trim(normalized, "-")
}
