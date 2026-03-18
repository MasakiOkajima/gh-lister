package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/MasakiOkajima/gh-lister/config"
	ghclient "github.com/MasakiOkajima/gh-lister/github"
	"github.com/MasakiOkajima/gh-lister/tui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. トークン取得
	token, err := ghclient.GetToken()
	if err != nil {
		return err
	}

	// 2. 設定ファイル読み込み
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if genErr := config.GenerateTemplate(cfgPath); genErr != nil {
				return fmt.Errorf("failed to generate config: %w", genErr)
			}
			fmt.Printf("Config template generated at: %s\nEdit it and run again.\n", cfgPath)
			return nil
		}
		return err
	}

	// 3. GitHub クライアント初期化
	client := ghclient.NewClient(token)

	// 4. PR 取得（両タブ並列）
	ctx := context.Background()

	reviewFetchFn := func() ([]ghclient.PR, error) {
		return client.FetchPendingReviews(ctx, cfg.Org, cfg.Repos)
	}
	myFetchFn := func() ([]ghclient.PR, error) {
		return client.FetchAuthoredPRs(ctx, cfg.Org, cfg.Repos)
	}

	var reviewPRs, myPRs []ghclient.PR
	var reviewErr, myErr error

	done := make(chan struct{})
	go func() {
		reviewPRs, reviewErr = reviewFetchFn()
		close(done)
	}()
	myPRs, myErr = myFetchFn()
	<-done

	// 5. エラーハンドリング
	if reviewErr != nil && myErr != nil {
		return fmt.Errorf("failed to fetch PRs: %w", errors.Join(reviewErr, myErr))
	}
	if reviewErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", reviewErr)
	}
	if myErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", myErr)
	}

	// 6. 0件チェック（両方空なら終了。エラー時は上で処理済み）
	if len(reviewPRs) == 0 && len(myPRs) == 0 {
		fmt.Println("No PRs found")
		return nil
	}

	// 7. TUI 起動
	model := tui.New(
		tui.TabData{PRs: reviewPRs, FetchFn: reviewFetchFn},
		tui.TabData{PRs: myPRs, FetchFn: myFetchFn},
	)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
