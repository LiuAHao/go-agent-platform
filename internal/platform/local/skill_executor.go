package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go-agent-platform/internal/domain/executor"
)

// LocalSkillExecutor 本地 Skill 执行器
// 从本地文件夹加载并执行 Skill
type LocalSkillExecutor struct {
	id         string
	name       string
	folderPath string
	entry      string
	schema     executor.SkillSchema
	config     map[string]any
}

// NewLocalSkillExecutor 创建本地 Skill 执行器
func NewLocalSkillExecutor(id, name, folderPath, entry string, schema executor.SkillSchema, config map[string]any) *LocalSkillExecutor {
	return &LocalSkillExecutor{
		id:         id,
		name:       name,
		folderPath: folderPath,
		entry:      entry,
		schema:     schema,
		config:     config,
	}
}

// ID 返回 Skill ID
func (e *LocalSkillExecutor) ID() string {
	return e.id
}

// Name 返回 Skill 名称
func (e *LocalSkillExecutor) Name() string {
	return e.name
}

// Execute 执行 Skill
func (e *LocalSkillExecutor) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 构建输入 JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	// 构建执行命令
	entryPath := filepath.Join(e.folderPath, e.entry)
	cmd := exec.CommandContext(ctx, "node", entryPath)
	cmd.Dir = e.folderPath
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), fmt.Sprintf("SKILL_INPUT=%s", string(inputJSON)))

	// 添加配置到环境变量
	for k, v := range e.config {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SKILL_%s=%v", k, v))
	}

	// 执行并获取输出
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("skill execution failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("execute skill: %w", err)
	}

	// 解析输出
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		// 如果不是 JSON，返回原始输出
		return map[string]any{
			"output": string(output),
		}, nil
	}

	return result, nil
}

// Schema 返回 Skill 的 Schema
func (e *LocalSkillExecutor) Schema() executor.SkillSchema {
	return e.schema
}

// HealthCheck 健康检查
func (e *LocalSkillExecutor) HealthCheck() error {
	// 检查入口文件是否存在
	entryPath := filepath.Join(e.folderPath, e.entry)
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		return fmt.Errorf("entry file not found: %s", entryPath)
	}
	return nil
}

// Close 关闭执行器
func (e *LocalSkillExecutor) Close() error {
	// 本地 Skill 执行器没有需要清理的资源
	return nil
}

// LoadLocalSkill 从本地文件夹加载 Skill
func LoadLocalSkill(id, name, folderPath string, config map[string]any) (*LocalSkillExecutor, error) {
	// 读取 manifest.json
	manifestPath := filepath.Join(folderPath, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// 解析 manifest
	var manifest struct {
		Entry  string           `json:"entry"`
		Schema executor.SkillSchema `json:"schema"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// 默认入口文件
	entry := manifest.Entry
	if entry == "" {
		entry = "index.js"
	}

	return NewLocalSkillExecutor(id, name, folderPath, entry, manifest.Schema, config), nil
}
