package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config はアプリケーション設定を表す。
type Config struct {
	Org   string   `yaml:"org"`
	Repos []string `yaml:"repos"`
	// BaseDirs はローカルクローンを探索するディレクトリ群。
	// Claudeレビュー時に owner/repo の repo 名で <base>/<repo> を探し、見つかればそこで起動する。
	BaseDirs []string `yaml:"base_dirs"`
}

// DefaultPath は設定ファイルのデフォルトパスを返す。
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gh-lister", "config.yaml")
}

// Load は指定パスから設定を読み込む。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	config.dedupRepos()
	config.expandBaseDirs()
	return config, nil
}

// expandBaseDirs は base_dirs 内の先頭 ~ をホームディレクトリへ展開する。
// yaml はシェル展開しないため、設定に書かれた ~/work などを自前で絶対パス化する。
func (c *Config) expandBaseDirs() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for i, dir := range c.BaseDirs {
		if dir == "~" {
			c.BaseDirs[i] = home
		} else if strings.HasPrefix(dir, "~/") {
			c.BaseDirs[i] = filepath.Join(home, dir[2:])
		}
	}
}

func (c *Config) validate() error {
	if c.Org == "" {
		return errors.New("org is required in config")
	}
	return nil
}

func (c *Config) dedupRepos() {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(c.Repos))
	for _, repo := range c.Repos {
		if !seen[repo] {
			seen[repo] = true
			unique = append(unique, repo)
		}
	}
	c.Repos = unique
}

// GenerateTemplate は設定ファイルのテンプレートを生成する。
func GenerateTemplate(path string) error {
	template := `# GitHub org to search for pending reviews
org: my-org

# Additional repositories outside the org (owner/repo format)
# repos:
#   - other-org/some-repo

# Base directories that hold local clones (for the 'c' Claude review key).
# For a PR in owner/repo, <base>/<repo> is searched in order; the first existing
# git checkout is used as the working directory. Falls back to the current
# directory when none match. '~' is expanded to your home directory.
# base_dirs:
#   - ~/repos
#   - ~/work
`
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}
	return nil
}
