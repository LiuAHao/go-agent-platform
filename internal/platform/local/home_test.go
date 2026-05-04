package local

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHome(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()
	home, err := NewHome(tmpDir)
	if err != nil {
		t.Fatalf("NewHome failed: %v", err)
	}

	// 验证根目录
	if home.Root() != tmpDir {
		t.Errorf("Root() = %s, want %s", home.Root(), tmpDir)
	}

	// 验证子目录已创建
	dirs := []string{
		home.DBPath(),
		home.SkillsDir(),
		home.MCPsDir(),
		home.ModelsDir(),
		home.AgentsDir(),
		home.LogsDir(),
		home.CacheDir(),
	}

	for _, dir := range dirs {
		parent := filepath.Dir(dir)
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			t.Errorf("Directory %s not created", parent)
		}
	}
}

func TestHomePaths(t *testing.T) {
	tmpDir := t.TempDir()
	home, err := NewHome(tmpDir)
	if err != nil {
		t.Fatalf("NewHome failed: %v", err)
	}

	// 测试 DB 路径
	dbPath := home.DBPath()
	if filepath.Ext(dbPath) != ".db" {
		t.Errorf("DBPath() should end with .db, got %s", dbPath)
	}

	// 测试 Skill 安装目录
	skillDir := home.SkillInstallDir("test-skill")
	expected := filepath.Join(tmpDir, "skills", "test-skill")
	if skillDir != expected {
		t.Errorf("SkillInstallDir() = %s, want %s", skillDir, expected)
	}
}

func TestCleanCache(t *testing.T) {
	tmpDir := t.TempDir()
	home, err := NewHome(tmpDir)
	if err != nil {
		t.Fatalf("NewHome failed: %v", err)
	}

	// 创建缓存文件
	cacheDir := home.CacheDir()
	testFile := filepath.Join(cacheDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 清理缓存
	if err := home.CleanCache(); err != nil {
		t.Fatalf("CleanCache failed: %v", err)
	}

	// 验证缓存目录已删除
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory should be deleted")
	}
}
