package historyview

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/history"
	"dbterm/internal/ui/syntax"
	"dbterm/internal/ui/theme"
)

// Messages emitted by History modal
type SelectHistoryQueryMsg struct {
	Query  string
	NewTab bool
}

type Model struct {
	Manager   *history.Manager
	Cursor    int
	Active    bool
	Width     int
	Height    int
	Filter    string
	Filtering bool
	Filtered  []history.HistoryEntry
}

func New(mgr *history.Manager) Model {
	m := Model{
		Manager: mgr,
		Cursor:  0,
		Active:  false,
	}
	m.rebuildFiltered()
	return m
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *Model) Open() {
	m.Active = true
	m.Cursor = 0
	m.Filter = ""
	m.Filtering = false
	m.rebuildFiltered()
}

func (m *Model) Close() {
	m.Active = false
	m.Filtering = false
}

func (m *Model) Toggle() {
	if m.Active {
		m.Close()
	} else {
		m.Open()
	}
}

func (m *Model) rebuildFiltered() {
	if m.Manager == nil {
		m.Filtered = nil
		return
	}
	if m.Filter == "" {
		m.Filtered = m.Manager.Entries
		return
	}
	var res []history.HistoryEntry
	term := strings.ToLower(m.Filter)
	for _, e := range m.Manager.Entries {
		if strings.Contains(strings.ToLower(e.Query), term) || strings.Contains(strings.ToLower(e.Database), term) {
			res = append(res, e)
		}
	}
	m.Filtered = res
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = len(m.Filtered) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active {
		return *m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Filtering {
			switch msg.String() {
			case "enter", "esc":
				m.Filtering = false
				return *m, nil
			case "backspace":
				if len(m.Filter) > 0 {
					m.Filter = m.Filter[:len(m.Filter)-1]
					m.rebuildFiltered()
				}
				return *m, nil
			default:
				if len(msg.String()) == 1 {
					m.Filter += msg.String()
					m.rebuildFiltered()
				}
				return *m, nil
			}
		}

		switch msg.String() {
		case "esc", "ctrl+h":
			m.Close()
			return *m, nil
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return *m, nil
		case "down", "j":
			if m.Cursor < len(m.Filtered)-1 {
				m.Cursor++
			}
			return *m, nil
		case "/":
			m.Filtering = true
			m.Filter = ""
			m.rebuildFiltered()
			return *m, nil
		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				m.Close()
				return *m, func() tea.Msg {
					return SelectHistoryQueryMsg{Query: selected.Query, NewTab: false}
				}
			}
		case "n": // open in new tab
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				m.Close()
				return *m, func() tea.Msg {
					return SelectHistoryQueryMsg{Query: selected.Query, NewTab: true}
				}
			}
		}
	}

	return *m, nil
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	modalWidth := 84
	if m.Width > 0 && modalWidth > m.Width-6 {
		modalWidth = m.Width - 6
	}

	var b strings.Builder
	title := "QUERY EXECUTION HISTORY (Ctrl+H)"
	if m.Filtering {
		title = fmt.Sprintf("Search History: %s_", m.Filter)
	} else if m.Filter != "" {
		title = fmt.Sprintf("Search History: [%s] (/ to edit)", m.Filter)
	}
	b.WriteString(theme.ModalTitle.Render(title) + "\n\n")

	if len(m.Filtered) == 0 {
		b.WriteString(theme.StyleFgMuted.Render("  No history entries found.\n\n"))
	} else {
		maxItems := 6
		startIdx := 0
		if m.Cursor >= maxItems {
			startIdx = m.Cursor - maxItems + 1
		}
		endIdx := startIdx + maxItems
		if endIdx > len(m.Filtered) {
			endIdx = len(m.Filtered)
		}

		for i := startIdx; i < endIdx; i++ {
			entry := m.Filtered[i]
			isSel := (i == m.Cursor)

			statusBadge := theme.StatusBadgeReady.Render(" OK ")
			if !entry.Success {
				statusBadge = theme.StatusBadgeError.Render(" ERR ")
			}

			dbBadge := theme.TopBarDB.Render(" " + entry.Database + " ")
			timeStr := entry.ExecutedAt.Format("15:04:05")
			metaLine := fmt.Sprintf("%s %s %s | %v | %d rows", statusBadge, dbBadge, timeStr, entry.Duration.Round(time.Millisecond), entry.RowCount)

			// Clean query preview (one liner)
			qPreview := strings.ReplaceAll(strings.TrimSpace(entry.Query), "\n", " ")
			if len(qPreview) > modalWidth-10 {
				qPreview = qPreview[:modalWidth-13] + "..."
			}

			var queryLine string
			if isSel {
				metaLine = theme.TreeSelected.Width(modalWidth - 6).Render("▶ " + metaLine)
				queryLine = theme.TreeSelected.Width(modalWidth - 6).Render("   " + qPreview)
			} else {
				metaLine = "  " + metaLine
				queryLine = "   " + syntax.HighlightLine(qPreview)
			}

			b.WriteString(metaLine + "\n" + queryLine + "\n\n")
		}
	}

	footer := lipgloss.JoinHorizontal(
		lipgloss.Top,
		theme.ButtonActive.Render("Enter: Use Query"),
		"  ",
		theme.ButtonInactive.Render("n: Open in New Tab"),
		"  ",
		theme.ButtonInactive.Render("/: Filter"),
		"  ",
		theme.ButtonInactive.Render("Esc: Close"),
	)
	b.WriteString(footer)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
