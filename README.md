# gh-lister

GitHubリポジトリから、自分がレビュー担当で未レビューの PR と自分が作成した PR を一覧表示する TUI ツール。

## インストール

```bash
go install github.com/MasakiOkajima/gh-lister@latest
```

### 前提条件

- [gh CLI](https://cli.github.com) がインストール済みで認証済みであること（`gh auth login`）

## 設定

初回実行時に `~/.config/gh-lister/config.yaml` にテンプレートが生成されます。

```yaml
# レビュー待ちPRを検索する GitHub org
org: my-org

# org 外の追加リポジトリ（owner/repo 形式）
# repos:
#   - other-org/some-repo
```

## 使い方

```bash
gh-lister
```

### キーバインド

| キー | 動作 |
|------|------|
| Tab | タブ切り替え（Review Requested / My PRs） |
| ↑/↓, j/k | カーソル移動 |
| Enter | 選択したPRをブラウザで開く |
| r | 一覧を再取得 |
| q, Ctrl+C | 終了 |
