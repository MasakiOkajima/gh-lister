package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReviewCommand(t *testing.T) {
	url := "https://github.com/org/repo/pull/123"
	cmd := reviewCommand(url, "/some/dir")

	if !strings.HasSuffix(cmd.Path, "claude") {
		t.Errorf("expected claude binary, got path %q", cmd.Path)
	}

	want := []string{"claude", "--permission-mode", "auto", "/code-review " + url}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("unexpected args:\n got: %q\nwant: %q", cmd.Args, want)
	}

	if cmd.Dir != "/some/dir" {
		t.Errorf("got Dir %q, want %q", cmd.Dir, "/some/dir")
	}
}

func TestReviewCommand_EmptyDirInheritsCwd(t *testing.T) {
	cmd := reviewCommand("https://github.com/org/repo/pull/1", "")
	if cmd.Dir != "" {
		t.Errorf("got Dir %q, want empty (inherit cwd)", cmd.Dir)
	}
}

func TestResolveRepoDir_FoundInFirstMatchingBase(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "repo-a")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveRepoDir("org/repo-a", []string{base})
	if !ok {
		t.Fatal("expected to resolve repo dir, got ok=false")
	}
	if got != repoDir {
		t.Errorf("got %q, want %q", got, repoDir)
	}
}

func TestResolveRepoDir_ChecksBasesInOrder(t *testing.T) {
	first := t.TempDir()  // 一致なし
	second := t.TempDir() // ここに clone がある
	repoDir := filepath.Join(second, "repo-b")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveRepoDir("org/repo-b", []string{first, second})
	if !ok || got != repoDir {
		t.Errorf("got (%q, %v), want (%q, true)", got, ok, repoDir)
	}
}

func TestResolveRepoDir_NotGitCheckout(t *testing.T) {
	base := t.TempDir()
	// .git のない同名ディレクトリは採用しない(偶然の同名ディレクトリ対策)
	if err := os.MkdirAll(filepath.Join(base, "repo-c"), 0755); err != nil {
		t.Fatal(err)
	}

	if _, ok := resolveRepoDir("org/repo-c", []string{base}); ok {
		t.Error("expected ok=false for non-git directory")
	}
}

func TestResolveRepoDir_NotFound(t *testing.T) {
	if _, ok := resolveRepoDir("org/missing", []string{t.TempDir()}); ok {
		t.Error("expected ok=false when no base contains the repo")
	}
}

func TestResolveRepoDir_NoBaseDirs(t *testing.T) {
	if _, ok := resolveRepoDir("org/repo", nil); ok {
		t.Error("expected ok=false with no base dirs")
	}
}
