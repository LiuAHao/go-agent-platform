package local

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go-agent-platform/internal/domain/skill"
)

// SkillInstaller 平台 Skill 安装器
type SkillInstaller struct {
	home       *Home
	httpClient *http.Client
}

// NewSkillInstaller 创建 Skill 安装器
func NewSkillInstaller(home *Home) *SkillInstaller {
	return &SkillInstaller{
		home: home,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// InstallResult 安装结果
type InstallResult struct {
	SkillID     string    `json:"skill_id"`
	Version     string    `json:"version"`
	InstallPath string    `json:"install_path"`
	InstalledAt time.Time `json:"installed_at"`
}

// Install 从 URL 下载并安装 Skill
func (si *SkillInstaller) Install(skillID, version, downloadURL, checksum string) (*InstallResult, error) {
	// 创建临时目录
	tmpDir := filepath.Join(si.home.CacheDir(), fmt.Sprintf("skill-%s-%s", skillID, version))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 下载文件
	zipPath := filepath.Join(tmpDir, "skill.zip")
	if err := si.download(downloadURL, zipPath); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	// 校验 checksum
	if checksum != "" {
		if err := si.verifyChecksum(zipPath, checksum); err != nil {
			return nil, fmt.Errorf("verify checksum: %w", err)
		}
	}

	// 安装目录
	installDir := si.home.SkillInstallDir(skillID)

	// 如果已存在，先删除
	if _, err := os.Stat(installDir); err == nil {
		if err := os.RemoveAll(installDir); err != nil {
			return nil, fmt.Errorf("remove old installation: %w", err)
		}
	}

	// 解压到安装目录
	if err := si.unzip(zipPath, installDir); err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}

	// 写入安装元数据
	result := &InstallResult{
		SkillID:     skillID,
		Version:     version,
		InstallPath: installDir,
		InstalledAt: time.Now().UTC(),
	}

	if err := si.writeInstallMeta(installDir, result); err != nil {
		return nil, fmt.Errorf("write install meta: %w", err)
	}

	return result, nil
}

// Uninstall 卸载 Skill
func (si *SkillInstaller) Uninstall(skillID string) error {
	installDir := si.home.SkillInstallDir(skillID)
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("uninstall skill: %w", err)
	}
	return nil
}

// IsInstalled 检查 Skill 是否已安装
func (si *SkillInstaller) IsInstalled(skillID string) bool {
	installDir := si.home.SkillInstallDir(skillID)
	_, err := os.Stat(installDir)
	return err == nil
}

// GetInstallPath 获取 Skill 安装路径
func (si *SkillInstaller) GetInstallPath(skillID string) string {
	return si.home.SkillInstallDir(skillID)
}

// ListInstalled 列出已安装的 Skill
func (si *SkillInstaller) ListInstalled() ([]InstallResult, error) {
	skillsDir := si.home.SkillsDir()
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var results []InstallResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		installDir := filepath.Join(skillsDir, entry.Name())
		metaPath := filepath.Join(installDir, "install-meta.json")

		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var result InstallResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// download 下载文件
func (si *SkillInstaller) download(url, destPath string) error {
	resp, err := si.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// verifyChecksum 校验文件 checksum
func (si *SkillInstaller) verifyChecksum(filePath, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// unzip 解压 zip 文件
func (si *SkillInstaller) unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		// 防止 zip slip
		if filepath.Dir(path) != dest && !isSubpath(dest, path) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(path)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// isSubpath 检查 child 是否是 parent 的子路径
func isSubpath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return len(rel) > 0 && rel[0] != '.'
}

// writeInstallMeta 写入安装元数据
func (si *SkillInstaller) writeInstallMeta(installDir string, result *InstallResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	metaPath := filepath.Join(installDir, "install-meta.json")
	return os.WriteFile(metaPath, data, 0644)
}

// LoadPlatformSkill 从安装目录加载平台 Skill
func LoadPlatformSkill(home *Home, skillMeta skill.Skill) (*LocalSkillExecutor, error) {
	installDir := home.SkillInstallDir(skillMeta.ID)

	// 检查是否已安装
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("skill %s not installed", skillMeta.ID)
	}

	return LoadLocalSkill(skillMeta.ID, skillMeta.Name, installDir, nil)
}
