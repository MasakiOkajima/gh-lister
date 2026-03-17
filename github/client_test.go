package github

import (
	"context"
	"testing"
)

func TestMergePRs_Dedup(t *testing.T) {
	a := []PR{
		{Repo: "org/repo-a", Title: "PR 1", URL: "https://github.com/org/repo-a/pull/1", Author: "alice"},
		{Repo: "org/repo-b", Title: "PR 2", URL: "https://github.com/org/repo-b/pull/2", Author: "bob"},
	}
	b := []PR{
		{Repo: "org/repo-a", Title: "PR 1", URL: "https://github.com/org/repo-a/pull/1", Author: "alice"},
		{Repo: "org/repo-c", Title: "PR 3", URL: "https://github.com/org/repo-c/pull/3", Author: "charlie"},
	}

	merged := MergePRs(a, b)
	if len(merged) != 3 {
		t.Errorf("got %d PRs, want 3 (duplicate removed)", len(merged))
	}
}

func TestMergePRs_Empty(t *testing.T) {
	merged := MergePRs(nil, nil)
	if len(merged) != 0 {
		t.Errorf("got %d PRs, want 0", len(merged))
	}
}

func TestRepoFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://api.github.com/repos/my-org/my-repo", "my-org/my-repo"},
		{"https://api.github.com/repos/owner/name", "owner/name"},
		{"short", "short"},
	}
	for _, tt := range tests {
		got := repoFromURL(tt.input)
		if got != tt.want {
			t.Errorf("repoFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("fake-token")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFetchPendingReviews_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	token, err := GetToken()
	if err != nil {
		t.Skipf("gh auth token not available: %v", err)
	}

	client := NewClient(token)
	prs, err := client.FetchPendingReviews(context.Background(), "golang", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("found %d pending reviews in golang org", len(prs))
}
