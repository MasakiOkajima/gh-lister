package tui

import (
	"fmt"
	"strings"

	"github.com/MasakiOkajima/gh-lister/github"
	"github.com/pkg/browser"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	repoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(30)
	authorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// FetchFunc はPR取得関数の型。refresh時に再利用する。
type FetchFunc func() ([]github.PR, error)

// Model は Bubble Tea の Model。
type Model struct {
	prs      []github.PR
	cursor   int
	fetching bool
	err      error
	fetchFn  FetchFunc
	width    int
	spinner  spinner.Model
}

type fetchDoneMsg struct {
	prs []github.PR
	err error
}

// New は新しい Model を作成する。
func New(prs []github.PR, fetchFn FetchFunc) Model {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
	)
	return Model{
		prs:     prs,
		fetchFn: fetchFn,
		spinner: s,
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
		m.prs = msg.prs
		m.err = msg.err
		if m.cursor >= len(m.prs) {
			m.cursor = max(0, len(m.prs)-1)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.prs)-1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			if len(m.prs) > 0 {
				_ = browser.OpenURL(m.prs[m.cursor].URL)
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

func (m Model) doFetch() tea.Cmd {
	return func() tea.Msg {
		prs, err := m.fetchFn()
		return fetchDoneMsg{prs: prs, err: err}
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

	header := fmt.Sprintf("🔍 Pending Reviews (%d)", len(m.prs))
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	if len(m.prs) == 0 {
		b.WriteString("  No pending reviews\n")
	}

	for i, pr := range m.prs {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		repo := repoStyle.Render(pr.Repo)
		title := truncate(pr.Title, m.titleWidth())
		author := authorStyle.Render("@" + pr.Author)

		line := fmt.Sprintf("%s%s  %s  %s", cursor, repo, title, author)
		if i == m.cursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑↓: move  Enter: open in browser  r: refresh  q: quit"))
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
