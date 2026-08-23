package help

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/ui/theme"
)

type Model struct {
	Active bool
	Width  int
	Height int
}

func New() Model {
	return Model{Active: false}
}

func (m *Model) Open() {
	m.Active = true
}

func (m *Model) Close() {
	m.Active = false
}

func (m *Model) Toggle() {
	m.Active = !m.Active
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active {
		return *m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter", "q", "?", "f1":
			m.Close()
			return *m, nil
		}
	}

	return *m, nil
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	modalWidth := 74
	if m.Width > 0 && modalWidth > m.Width-6 {
		modalWidth = m.Width - 6
	}

	shortcuts := [][]string{
		{"F5 / Ctrl+E", "Execute query or selected text (SSMS standard)"},
		{"Ctrl+K / F4", "Open Embedded AI SQL Assistant (Offline Text-to-SQL)"},
		{"Esc (Executing)", "Cancel active running query"},
		{"F8", "Toggle Object Explorer sidebar visibility"},
		{"Tab / Shift+Tab", "Cycle focus (Explorer -> Editor -> Results)"},
		{"Alt+Left / Alt+Right", "Resize Sidebar width (or drag vertical border)"},
		{"Alt+Up / Alt+Down", "Resize Editor vs Results height (or drag border)"},
		{"Alt+=", "Reset all pane sizes to 50/50 defaults"},
		{"Shift + Arrows", "Select text across characters & lines in Editor"},
		{"F2 / Ctrl+B / Option+V", "Toggle Visual Selection Mode in Editor (macOS: F2)"},
		{"Ctrl+C / Ctrl+X / Ctrl+V", "Copy / Cut / Paste with System Clipboard"},
		{"Ctrl+A (Editor)", "Select All text in active Query Tab"},
		{"Ctrl+S (Editor)", "Save active Query to SQL File (or Save As prompt)"},
		{"Ctrl+O", "Open Connection Manager / Switch Database"},
		{"Ctrl+H", "Open Query Execution History modal"},
		{"Ctrl+N / Ctrl+W", "New Tab / Close active Query Tab"},
		{"1 / 2 (Explorer)", "Switch between [1] Database Tree and [2] SQL Files"},
		{"Enter (Files)", "Open SQL File into Editor"},
		{"Ctrl+R / Ctrl+M", "Switch between Results Grid and Messages Log"},
		{"Enter / v (Results)", "Inspect full Cell Value (JSON/XML/Text)"},
		{"e (Results)", "Open Multi-Format Export Dialog (CSV/Excel/TXT/JSON)"},
		{"o (Results)", "Toggle ascending/descending Sort on column"},
		{"/ (Results)", "Filter / Search rows in memory"},
		{"Right-Click / m (Explorer)", "Context Menu: Script as SELECT / CREATE / INSERT / Open"},
		{"r (Explorer)", "Refresh database schema or file tree"},
		{"/ (Explorer)", "Search and filter schema objects or files"},
		{"? / F1", "Toggle this Help modal"},
		{"Ctrl+Q", "Exit dbterm"},
	}

	var b strings.Builder
	b.WriteString(theme.ModalTitle.Render("KEYBOARD SHORTCUTS & ERGONOMICS (SSMS)") + "\n\n")

	for _, sc := range shortcuts {
		keyStr := theme.StatusKey.Width(22).Render(sc[0])
		descStr := theme.StyleFgLight.Render(sc[1])
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStr, descStr))
	}

	b.WriteString("\n")
	b.WriteString(theme.StyleFgMuted.Render("  Press Esc or Enter to close this help window."))

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
