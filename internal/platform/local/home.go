package local

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// DefaultHomeName 是本地存储的默认目录名
	DefaultHomeName = ".agent-platform"

	// DirDB 存放本地数据库
	DirDB = "db"

	// DirSkills 存放已安装的平台 Skill
	DirSkills = "skills"

	// DirMCPs 存放 MCP 配置
	DirMCPs = "mcps"

	// DirModels 存放模型配置 (含敏感信息)
	DirModels = "models"

	// DirAgents 存放 Agent 运行时配置
	DirAgents = "agents"

	// DirLogs 存放执行日志
	DirLogs = "logs"

	// DirCache 存放临时缓存
	DirCache = "cache"
)

// Home 管理本地存储目录结构
type Home struct {
	root string
}

// NewHome 创建 Home 实例
// 默认使用 ~/.agent-platform/
func NewHome(customRoot string) (*Home, error) {
	root := customRoot
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get user home dir: %w", err)
		}
		root = filepath.Join(home, DefaultHomeName)
	}

	h := &Home{root: root}
	if err := h.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure dirs: %w", err)
	}

	return h, nil
}

// Root 返回根目录
func (h *Home) Root() string {
	return h.root
}

// DBPath 返回数据库文件路径
func (h *Home) DBPath() string {
	return filepath.Join(h.root, DirDB, "local.db")
}

// SkillsDir 返回 Skill 安装目录
func (h *Home) SkillsDir() string {
	return filepath.Join(h.root, DirSkills)
}

// MCPsDir 返回 MCP 配置目录
func (h *Home) MCPsDir() string {
	return filepath.Join(h.root, DirMCPs, "configs")
}

// ModelsDir 返回模型配置目录
func (h *Home) ModelsDir() string {
	return filepath.Join(h.root, DirModels)
}

// AgentsDir 返回 Agent 配置目录
func (h *Home) AgentsDir() string {
	return filepath.Join(h.root, DirAgents)
}

// LogsDir 返回日志目录
func (h *Home) LogsDir() string {
	return filepath.Join(h.root, DirLogs)
}

// CacheDir 返回缓存目录
func (h *Home) CacheDir() string {
	return filepath.Join(h.root, DirCache)
}

// SkillInstallDir 返回指定 Skill 的安装目录
func (h *Home) SkillInstallDir(skillID string) string {
	return filepath.Join(h.SkillsDir(), skillID)
}

// EnsureDirs 创建所有必要的目录
func (h *Home) EnsureDirs() error {
	dirs := []string{
		h.root,
		filepath.Join(h.root, DirDB),
		h.SkillsDir(),
		h.MCPsDir(),
		h.ModelsDir(),
		h.AgentsDir(),
		h.LogsDir(),
		h.CacheDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	return nil
}

// CleanCache 清理缓存目录
func (h *Home) CleanCache() error {
	return os.RemoveAll(h.CacheDir())
}

// Platform 返回当前平台标识
func Platform() string {
	return runtime.GOOS
}

// IsWindows 返回是否是 Windows 系统
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
