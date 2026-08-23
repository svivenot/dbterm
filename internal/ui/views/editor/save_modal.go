package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/ui/theme"
)

// Messages emitted by SaveModal
type SaveFileSuccessMsg struct {
	FilePath string
	Content  string
}

type SaveModal struct {
	Active       bool
	Content      string
	InputPath    string
	CursorPos    int
	ErrorMessage string
	Width        int
	Height       int
}

func NewSaveModal() SaveModal {
	return SaveModal{
		Active:    false,
		InputPath: "./query.sql",
	}
}

func (m *SaveModal) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *SaveModal) Open(content, defaultName string) {
	m.Active = true
	m.Content = content
	m.ErrorMessage = ""
	if defaultName == "" {
		defaultName = "query.sql"
	}
	if !strings.HasSuffix(strings.ToLower(defaultName), ".sql") {
		defaultName += ".sql"
	}
	m.InputPath = "./" + defaultName
	m.CursorPos = len(m.InputPath)
}

func (m *SaveModal) Close() {
	m.Active = false
	m.ErrorMessage = ""
}

func (m SaveModal) Update(msg tea.Msg) (SaveModal, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return m, nil

		case "enter":
			trimmed := strings.TrimSpace(m.InputPath)
			if trimmed == "" {
				m.ErrorMessage = "Please specify a valid file path."
				return m, nil
			}

			// Ensure directory exists
			dir := filepath.Dir(trimmed)
			if dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					m.ErrorMessage = fmt.Sprintf("Failed to create directory: %v", err)
					return m, nil
				}
			}

			// Write SQL content to file
			if err := os.WriteFile(trimmed, []byte(m.Content), 0644); err != nil {
				m.ErrorMessage = fmt.Sprintf("Failed to save file: %v", err)
				return m, nil
			}

			savedPath := trimmed
			m.Close()
			return m, func() tea.Msg {
				return SaveFileSuccessMsg{
					FilePath: savedPath,
					Content:  m.Content,
				}
			}

		case "backspace":
			if len(m.InputPath) > 0 && m.CursorPos > 0 {
				m.InputPath = m.InputPath[:m.CursorPos-1] + m.InputPath[m.CursorPos:]
				m.CursorPos--
			}
			return m, nil

		case "left":
			if m.CursorPos > 0 {
				m.CursorPos--
			}
			return m, nil

		case "right":
			if m.CursorPos < len(m.InputPath) {
				m.CursorPos++
			}
			return m, nil

		default:
			if len(msg.String()) == 1 {
				m.InputPath = m.InputPath[:m.CursorPos] + msg.String() + m.InputPath[m.CursorPos:]
				m.CursorPos++
			}
			return m, nil
		}
	}

	return m, nil
}

func (m SaveModal) View() string {
	if !m.Active {
		return ""
	}

	modalWidth := 65
	if m.Width > 0 && modalWidth > m.Width-6 {
		modalWidth = m.Width - 6
	}

	var b strings.Builder
	b.WriteString(theme.ModalTitle.Render(" 💾 SAVE SQL QUERY TO FILE ") + "\n\n")
	b.WriteString(theme.StyleFgMuted.Render("Enter file path or relative name to save the current SQL query:") + "\n\n")

	// Render text input box with cursor
	inputDisplay := m.InputPath
	if m.CursorPos >= 0 && m.CursorPos <= len(m.InputPath) {
		cursorChar := " "
		if m.CursorPos < len(m.InputPath) {
			cursorChar = string(m.InputPath[m.CursorPos])
		}
		styledCursor := lipgloss.NewStyle().Background(theme.ColorPrimary).Foreground(lipgloss.Color("#FFF")).Render(cursorChar)
		if m.CursorPos < len(m.InputPath) {
			inputDisplay = m.InputPath[:m.CursorPos] + styledCursor + m.InputPath[m.CursorPos+1:]
		} else {
			inputDisplay = m.InputPath + styledCursor
		}
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(0, 1).
		Width(modalWidth - 6).
		Render(inputDisplay)

	b.WriteString(inputBox + "\n")

	cwd, _ := os.Getwd()
	b.WriteString(theme.StyleFgDim.Render(fmt.Sprintf("Current working directory: %s", cwd)) + "\n\n")

	if m.ErrorMessage != "" {
		b.WriteString(theme.StatusBadgeError.Render(" "+m.ErrorMessage+" ") + "\n\n")
	}

	footer := lipgloss.JoinHorizontal(
		lipgloss.Top,
		theme.ButtonActive.Render("Enter: Save File"),
		"  ",
		theme.ButtonInactive.Render("Esc: Cancel"),
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
