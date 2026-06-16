package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MasakiOkajima/gh-lister/github"
	"github.com/pkg/browser"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	repoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(30)
	authorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Underline(true)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// FetchFunc はPR取得関数の型。refresh時に再利用する。
type FetchFunc func() ([]github.PR, error)

// TabData はコンストラクタに渡すタブの初期データ。
type TabData struct {
	PRs     []github.PR
	FetchFn FetchFunc
}

// tabState はタブごとの可変状態。
type tabState struct {
	label   string
	prs     []github.PR
	cursor  int
	fetchFn FetchFunc
}

// Model は Bubble Tea の Model。
type Model struct {
	activeTab int
	tabs      [2]tabState
	fetching  bool
	err       error
	width     int
	spinner   spinner.Model
	baseDirs  []string
}

type fetchDoneMsg struct {
	reviewPRs []github.PR
	myPRs     []github.PR
	err       error
}

// reviewDoneMsg は claude レビューセッション終了時に届く。errはプロセス起動/異常終了のみ。
type reviewDoneMsg struct {
	err error
}

// New は新しい Model を作成する。baseDirs は Claude レビュー時にローカルクローンを探す基準ディレクトリ群。
func New(reviewTab, myPRsTab TabData, baseDirs []string) Model {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
	)
	return Model{
		tabs: [2]tabState{
			{label: "Review Requested", prs: reviewTab.PRs, fetchFn: reviewTab.FetchFn},
			{label: "My PRs", prs: myPRsTab.PRs, fetchFn: myPRsTab.FetchFn},
		},
		spinner:  s,
		baseDirs: baseDirs,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case fetchDoneMsg:
		m.fetching = false
		m.err = msg.err
		m.tabs[0].prs = msg.reviewPRs
		m.tabs[1].prs = msg.myPRs
		for i := range m.tabs {
			if m.tabs[i].cursor >= len(m.tabs[i].prs) {
				m.tabs[i].cursor = max(0, len(m.tabs[i].prs)-1)
			}
		}
		return m, nil

	case reviewDoneMsg:
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			return m, nil

		case "up", "k":
			tab := &m.tabs[m.activeTab]
			if tab.cursor > 0 {
				tab.cursor--
			}
			return m, nil

		case "down", "j":
			tab := &m.tabs[m.activeTab]
			if tab.cursor < len(tab.prs)-1 {
				tab.cursor++
			}
			return m, nil

		case "enter":
			tab := &m.tabs[m.activeTab]
			if len(tab.prs) > 0 {
				_ = browser.OpenURL(tab.prs[tab.cursor].URL)
			}
			return m, nil

		case "c":
			tab := &m.tabs[m.activeTab]
			if len(tab.prs) > 0 {
				pr := tab.prs[tab.cursor]
				dir, _ := resolveRepoDir(pr.Repo, m.baseDirs)
				cmd := reviewCommand(pr.URL, dir)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return reviewDoneMsg{err: err}
				})
			}
			return m, nil

		case "r":
			if !m.fetching {
				m.fetching = true
				return m, tea.Batch(m.spinner.Tick, m.doFetch())
			}
			return m, nil
		}
	}
	return m, nil
}

// reviewCommand は claude を auto モードで起動し /code-review に PR URL を渡すコマンドを組み立てる。
// プロンプトは "/code-review <url>" を1引数として渡す(claudeは位置引数を初回入力として扱う)。
// dir が空でなければそのディレクトリで起動する(空なら呼び出し元のcwdを継承)。
func reviewCommand(url, dir string) *exec.Cmd {
	cmd := exec.Command("claude", "--permission-mode", "auto", "/code-review "+url)
	cmd.Dir = dir
	return cmd
}

// resolveRepoDir は owner/repo のローカルクローンを baseDirs から探す。
// repo 名で <base>/<repo> を順に調べ、git チェックアウト(.git が存在)であれば採用する。
// 見つからなければ ok=false を返し、呼び出し元は cwd 起動にフォールバックする。
func resolveRepoDir(repo string, baseDirs []string) (string, bool) {
	name := repo
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		name = repo[i+1:]
	}
	if name == "" {
		return "", false
	}
	for _, base := range baseDirs {
		candidate := filepath.Join(base, name)
		if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func (m Model) doFetch() tea.Cmd {
	reviewFn := m.tabs[0].fetchFn
	myFn := m.tabs[1].fetchFn
	return func() tea.Msg {
		var reviewPRs, myPRs []github.PR
		var reviewErr, myErr error

		done := make(chan struct{})
		go func() {
			reviewPRs, reviewErr = reviewFn()
			close(done)
		}()
		myPRs, myErr = myFn()
		<-done

		err := errors.Join(reviewErr, myErr)
		return fetchDoneMsg{reviewPRs: reviewPRs, myPRs: myPRs, err: err}
	}
}

func (m Model) View() tea.View {
	var b strings.Builder

	if m.fetching {
		b.WriteString(fmt.Sprintf("  %s Fetching PRs...\n", m.spinner.View()))
		fv := tea.NewView(b.String())
		fv.AltScreen = true
		return fv
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("Error: %v\n\n", m.err))
	}

	// Tab header
	for i, tab := range m.tabs {
		label := fmt.Sprintf(" %s (%d) ", tab.label, len(tab.prs))
		if i == m.activeTab {
			b.WriteString(activeTabStyle.Render(label))
		} else {
			b.WriteString(inactiveTabStyle.Render(label))
		}
		if i < len(m.tabs)-1 {
			b.WriteString(inactiveTabStyle.Render(" | "))
		}
	}
	b.WriteString("\n\n")

	// PR list for active tab
	tab := &m.tabs[m.activeTab]
	if len(tab.prs) == 0 {
		b.WriteString("  No PRs\n")
	}

	for i, pr := range tab.prs {
		cursor := "  "
		if i == tab.cursor {
			cursor = "> "
		}

		repo := repoStyle.Render(pr.Repo)
		title := truncate(pr.Title, m.titleWidth())
		author := authorStyle.Render("@" + pr.Author)

		line := fmt.Sprintf("%s%s  %s  %s", cursor, repo, title, author)
		if i == tab.cursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  Tab: switch  ↑↓: move  Enter: open  c: review  r: refresh  q: quit"))
	b.WriteString("\n")

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) titleWidth() int {
	available := m.width - 58
	if available < 20 {
		available = 40
	}
	return available
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}
