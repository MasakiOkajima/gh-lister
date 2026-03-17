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

	// 4. PR 取得
	ctx := context.Background()
	fetchFn := func() ([]ghclient.PR, error) {
		return client.FetchPendingReviews(ctx, cfg.Org, cfg.Repos)
	}

	prs, err := fetchFn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// 5. 0件チェック
	if len(prs) == 0 && err == nil {
		fmt.Println("No pending reviews")
		return nil
	}

	// 6. TUI 起動
	model := tui.New(prs, fetchFn)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
