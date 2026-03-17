# gh-lister

A TUI tool to list GitHub pull requests pending your review across an entire org.

## Install

```bash
go install github.com/MasakiOkajima/gh-lister@latest
```

### Prerequisites

- [gh CLI](https://cli.github.com) installed and authenticated (`gh auth login`)

## Configuration

On first run, a config template is generated at `~/.config/gh-lister/config.yaml`:

```yaml
# GitHub org to search for pending reviews
org: my-org

# Additional repositories outside the org (owner/repo format)
# repos:
#   - other-org/some-repo
```

## Usage

```bash
gh-lister
```

### Keybindings

| Key | Action |
|-----|--------|
| ↑/↓, j/k | Move cursor |
| Enter | Open PR in browser |
| r | Refresh list |
| q, Ctrl+C | Quit |
