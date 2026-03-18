package tui

import (
	"errors"
	"fmt"
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
}

type fetchDoneMsg struct {
	reviewPRs []github.PR
	myPRs     []github.PR
	err       error
}

// New は新しい Model を作成する。
func New(reviewTab, myPRsTab TabData) Model {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
	)
	return Model{
		tabs: [2]tabState{
			{label: "Review Requested", prs: reviewTab.PRs, fetchFn: reviewTab.FetchFn},
			{label: "My PRs", prs: myPRsTab.PRs, fetchFn: myPRsTab.FetchFn},
		},
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
		m.err = msg.err
		m.tabs[0].prs = msg.reviewPRs
		m.tabs[1].prs = msg.myPRs
		for i := range m.tabs {
			if m.tabs[i].cursor >= len(m.tabs[i].prs) {
				m.tabs[i].cursor = max(0, len(m.tabs[i].prs)-1)
			}
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
	b.WriteString(helpStyle.Render("  Tab: switch  ↑↓: move  Enter: open  r: refresh  q: quit"))
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
