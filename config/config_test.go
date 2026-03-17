package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("org: my-org\nrepos:\n  - other/repo-a\n  - other/repo-b\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Org != "my-org" {
		t.Errorf("got org %q, want %q", cfg.Org, "my-org")
	}
	if len(cfg.Repos) != 2 {
		t.Errorf("got %d repos, want 2", len(cfg.Repos))
	}
}

func TestLoad_EmptyOrg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("org: \"\"\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty org, got nil")
	}
}

func TestLoad_DuplicateRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("org: my-org\nrepos:\n  - other/repo-a\n  - other/repo-a\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Errorf("got %d repos, want 1 (duplicates removed)", len(cfg.Repos))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_ReposOptional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("org: my-org\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("got %d repos, want 0", len(cfg.Repos))
	}
}

func TestGenerateTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	err := GenerateTemplate(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("template file was not created")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("generated template is not valid: %v", err)
	}
	if cfg.Org == "" {
		t.Error("template org should not be empty")
	}
}
