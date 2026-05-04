package local

import (
	"fmt"
	"sync"

	"go-agent-platform/internal/domain/executor"
)

// ExecutorManager 本地执行器管理器实现
type ExecutorManager struct {
	mu       sync.RWMutex
	skills   map[string]executor.SkillExecutor
	mcps     map[string]executor.MCPExecutor
}

// NewExecutorManager 创建执行器管理器
func NewExecutorManager() *ExecutorManager {
	return &ExecutorManager{
		skills: make(map[string]executor.SkillExecutor),
		mcps:   make(map[string]executor.MCPExecutor),
	}
}

// RegisterSkill 注册 Skill 执行器
func (em *ExecutorManager) RegisterSkill(exec executor.SkillExecutor) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	id := exec.ID()
	if _, exists := em.skills[id]; exists {
		return fmt.Errorf("skill %s already registered", id)
	}

	em.skills[id] = exec
	return nil
}

// UninstallSkill 注销 Skill 执行器
func (em *ExecutorManager) UninstallSkill(skillID string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	exec, exists := em.skills[skillID]
	if !exists {
		return fmt.Errorf("skill %s not found", skillID)
	}

	if err := exec.Close(); err != nil {
		return fmt.Errorf("close skill executor: %w", err)
	}

	delete(em.skills, skillID)
	return nil
}

// GetSkill 获取 Skill 执行器
func (em *ExecutorManager) GetSkill(skillID string) (executor.SkillExecutor, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	exec, exists := em.skills[skillID]
	if !exists {
		return nil, fmt.Errorf("skill %s not found", skillID)
	}

	return exec, nil
}

// ListSkills 列出所有已注册的 Skill
func (em *ExecutorManager) ListSkills() []executor.SkillExecutor {
	em.mu.RLock()
	defer em.mu.RUnlock()

	result := make([]executor.SkillExecutor, 0, len(em.skills))
	for _, exec := range em.skills {
		result = append(result, exec)
	}
	return result
}

// RegisterMCP 注册 MCP 执行器
func (em *ExecutorManager) RegisterMCP(exec executor.MCPExecutor) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	id := exec.ID()
	if _, exists := em.mcps[id]; exists {
		return fmt.Errorf("mcp %s already registered", id)
	}

	em.mcps[id] = exec
	return nil
}

// UninstallMCP 注销 MCP 执行器
func (em *ExecutorManager) UninstallMCP(mcpID string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	exec, exists := em.mcps[mcpID]
	if !exists {
		return fmt.Errorf("mcp %s not found", mcpID)
	}

	if err := exec.Disconnect(); err != nil {
		return fmt.Errorf("disconnect mcp: %w", err)
	}

	delete(em.mcps, mcpID)
	return nil
}

// GetMCP 获取 MCP 执行器
func (em *ExecutorManager) GetMCP(mcpID string) (executor.MCPExecutor, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	exec, exists := em.mcps[mcpID]
	if !exists {
		return nil, fmt.Errorf("mcp %s not found", mcpID)
	}

	return exec, nil
}

// ListMCPs 列出所有已注册的 MCP
func (em *ExecutorManager) ListMCPs() []executor.MCPExecutor {
	em.mu.RLock()
	defer em.mu.RUnlock()

	result := make([]executor.MCPExecutor, 0, len(em.mcps))
	for _, exec := range em.mcps {
		result = append(result, exec)
	}
	return result
}

// Close 关闭所有执行器
func (em *ExecutorManager) Close() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	var errs []error

	for id, exec := range em.skills {
		if err := exec.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close skill %s: %w", id, err))
		}
	}

	for id, exec := range em.mcps {
		if err := exec.Disconnect(); err != nil {
			errs = append(errs, fmt.Errorf("disconnect mcp %s: %w", id, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close executors: %v", errs)
	}

	return nil
}
