package github

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	gh "github.com/google/go-github/v82/github"
	"golang.org/x/sync/errgroup"
)

// PR は表示用のPR情報を表す。
type PR struct {
	Repo      string
	Title     string
	URL       string
	Author    string
	UpdatedAt int64
}

// Client は GitHub API クライアントをラップする。
type Client struct {
	gh   *gh.Client
	user string
}

// GetToken は gh auth token でトークンを取得する。
func GetToken() (string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh command not found. Install: https://cli.github.com")
	}

	out, err := exec.Command(path, "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token. Run `gh auth login` to authenticate")
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token. Run `gh auth login` to authenticate")
	}
	return token, nil
}

// NewClient は新しい Client を作成する。
func NewClient(token string) *Client {
	client := gh.NewClient(nil).WithAuthToken(token)
	return &Client{gh: client}
}

// Username はログインユーザー名を取得してキャッシュする。
func (c *Client) Username(ctx context.Context) (string, error) {
	if c.user != "" {
		return c.user, nil
	}
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("failed to get authenticated user: %w", err)
	}
	c.user = user.GetLogin()
	return c.user, nil
}

// FetchAuthoredPRs は org 横断検索 + 個別リポジトリから自分が作成した PR を取得する。
func (c *Client) FetchAuthoredPRs(ctx context.Context, org string, repos []string) ([]PR, error) {
	username, err := c.Username(ctx)
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)

	var orgPRs []PR
	var repoPRs []PR

	g.Go(func() error {
		prs, err := c.searchOrgAuthoredPRs(ctx, org, username)
		if err != nil {
			return err
		}
		orgPRs = prs
		return nil
	})

	if len(repos) > 0 {
		g.Go(func() error {
			prs, err := c.fetchRepoAuthoredPRs(ctx, repos, username)
			if err != nil {
				return err
			}
			repoPRs = prs
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		merged := MergePRs(orgPRs, repoPRs)
		sortPRs(merged)
		return merged, err
	}

	merged := MergePRs(orgPRs, repoPRs)
	sortPRs(merged)
	return merged, nil
}

// FetchPendingReviews は org 横断検索 + 個別リポジトリから未レビュー PR を取得する。
func (c *Client) FetchPendingReviews(ctx context.Context, org string, repos []string) ([]PR, error) {
	username, err := c.Username(ctx)
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)

	var orgPRs []PR
	var repoPRs []PR

	g.Go(func() error {
		prs, err := c.searchOrgPRs(ctx, org, username)
		if err != nil {
			return err
		}
		orgPRs = prs
		return nil
	})

	if len(repos) > 0 {
		g.Go(func() error {
			prs, err := c.fetchRepoPRs(ctx, repos, username)
			if err != nil {
				return err
			}
			repoPRs = prs
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		merged := MergePRs(orgPRs, repoPRs)
		sortPRs(merged)
		return merged, err
	}

	merged := MergePRs(orgPRs, repoPRs)
	sortPRs(merged)
	return merged, nil
}

func (c *Client) searchOrgPRs(ctx context.Context, org string, username string) ([]PR, error) {
	query := fmt.Sprintf("is:pr is:open review-requested:%s org:%s", username, org)
	opts := &gh.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var allPRs []PR
	for {
		result, resp, err := c.gh.Search.Issues(ctx, query, opts)
		if err != nil {
			return allPRs, fmt.Errorf("search API error: %w", err)
		}
		for _, issue := range result.Issues {
			allPRs = append(allPRs, PR{
				Repo:      repoFromURL(issue.GetRepositoryURL()),
				Title:     issue.GetTitle(),
				URL:       issue.GetHTMLURL(),
				Author:    issue.GetUser().GetLogin(),
				UpdatedAt: issue.GetUpdatedAt().Unix(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allPRs, nil
}

func (c *Client) searchOrgAuthoredPRs(ctx context.Context, org string, username string) ([]PR, error) {
	query := fmt.Sprintf("is:pr is:open author:%s org:%s", username, org)
	opts := &gh.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var allPRs []PR
	for {
		result, resp, err := c.gh.Search.Issues(ctx, query, opts)
		if err != nil {
			return allPRs, fmt.Errorf("search API error: %w", err)
		}
		for _, issue := range result.Issues {
			allPRs = append(allPRs, PR{
				Repo:      repoFromURL(issue.GetRepositoryURL()),
				Title:     issue.GetTitle(),
				URL:       issue.GetHTMLURL(),
				Author:    issue.GetUser().GetLogin(),
				UpdatedAt: issue.GetUpdatedAt().Unix(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allPRs, nil
}

func (c *Client) fetchRepoPRs(ctx context.Context, repos []string, username string) ([]PR, error) {
	var allPRs []PR
	for _, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, name := parts[0], parts[1]

		opts := &gh.PullRequestListOptions{
			State:       "open",
			ListOptions: gh.ListOptions{PerPage: 100},
		}

		for {
			pulls, resp, err := c.gh.PullRequests.List(ctx, owner, name, opts)
			if err != nil {
				break
			}
			for _, pr := range pulls {
				if isReviewRequested(pr, username) {
					allPRs = append(allPRs, PR{
						Repo:      repo,
						Title:     pr.GetTitle(),
						URL:       pr.GetHTMLURL(),
						Author:    pr.GetUser().GetLogin(),
						UpdatedAt: pr.GetUpdatedAt().Unix(),
					})
				}
			}
			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	}
	return allPRs, nil
}

func (c *Client) fetchRepoAuthoredPRs(ctx context.Context, repos []string, username string) ([]PR, error) {
	var allPRs []PR
	for _, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, name := parts[0], parts[1]

		opts := &gh.PullRequestListOptions{
			State:       "open",
			ListOptions: gh.ListOptions{PerPage: 100},
		}

		for {
			pulls, resp, err := c.gh.PullRequests.List(ctx, owner, name, opts)
			if err != nil {
				break
			}
			for _, pr := range pulls {
				if isAuthor(pr, username) {
					allPRs = append(allPRs, PR{
						Repo:      repo,
						Title:     pr.GetTitle(),
						URL:       pr.GetHTMLURL(),
						Author:    pr.GetUser().GetLogin(),
						UpdatedAt: pr.GetUpdatedAt().Unix(),
					})
				}
			}
			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	}
	return allPRs, nil
}

func isReviewRequested(pr *gh.PullRequest, username string) bool {
	for _, reviewer := range pr.RequestedReviewers {
		if strings.EqualFold(reviewer.GetLogin(), username) {
			return true
		}
	}
	return false
}

func isAuthor(pr *gh.PullRequest, username string) bool {
	return strings.EqualFold(pr.GetUser().GetLogin(), username)
}

func repoFromURL(apiURL string) string {
	parts := strings.Split(apiURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return apiURL
}

// MergePRs は2つの PR リストをマージし、URL で重複排除する。
func MergePRs(a, b []PR) []PR {
	seen := make(map[string]bool)
	var merged []PR
	for _, lists := range [][]PR{a, b} {
		for _, pr := range lists {
			if !seen[pr.URL] {
				seen[pr.URL] = true
				merged = append(merged, pr)
			}
		}
	}
	return merged
}

func sortPRs(prs []PR) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].UpdatedAt > prs[j].UpdatedAt
	})
}
