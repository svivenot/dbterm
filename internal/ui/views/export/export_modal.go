package exportview

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/export"
	"dbterm/internal/ui/theme"
)

// Messages emitted by Export Modal
type PerformExportMsg struct {
	Options export.ExportOptions
}

type ExportField int

const (
	FieldFormat ExportField = iota
	FieldFilename
	FieldHeaders
	FieldButtonExport
	FieldButtonCancel
)

type Model struct {
	Active         bool
	Width          int
	Height         int
	Formats        []export.Format
	FormatLabels   []string
	SelectedFormat int
	FilenameInput  textinput.Model
	IncludeHeaders bool
	FocusedField   ExportField
	Columns        []string
	Rows           [][]string
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "filename.csv"
	ti.CharLimit = 150
	ti.Width = 45

	formats := []export.Format{
		export.FormatCSV,
		export.FormatXLSX,
		export.FormatFixed,
		export.FormatJSON,
	}

	formatLabels := []string{
		"CSV (.csv)",
		"Excel (.xlsx)",
		"Fixed-Width Text (.txt)",
		"JSON (.json)",
	}

	return Model{
		Active:         false,
		Formats:        formats,
		FormatLabels:   formatLabels,
		SelectedFormat: 0,
		FilenameInput:  ti,
		IncludeHeaders: true,
		FocusedField:   FieldFilename,
	}
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *Model) Open(columns []string, rows [][]string) {
	m.Active = true
	m.Columns = columns
	m.Rows = rows
	m.FocusedField = FieldFilename

	// Generate default filename
	ext := m.Formats[m.SelectedFormat]
	timestamp := time.Now().Format("20060102_150405")
	m.FilenameInput.SetValue(fmt.Sprintf("query_export_%s.%s", timestamp, ext))
	m.FilenameInput.Focus()
}

func (m *Model) Close() {
	m.Active = false
	m.FilenameInput.Blur()
}

func (m *Model) updateFilenameExtension() {
	current := m.FilenameInput.Value()
	ext := fmt.Sprintf(".%s", m.Formats[m.SelectedFormat])

	if current == "" {
		m.FilenameInput.SetValue(fmt.Sprintf("query_export_%s%s", time.Now().Format("20060102_150405"), ext))
		return
	}

	base := strings.TrimSuffix(current, filepath.Ext(current))
	m.FilenameInput.SetValue(base + ext)
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active {
		return *m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return *m, nil

		case "tab":
			m.nextField()
			return *m, nil

		case "shift+tab":
			m.prevField()
			return *m, nil

		case "enter":
			if m.FocusedField == FieldButtonCancel {
				m.Close()
				return *m, nil
			}
			// Submit export
			opts := export.ExportOptions{
				Format:         m.Formats[m.SelectedFormat],
				FilePath:       m.FilenameInput.Value(),
				IncludeHeaders: m.IncludeHeaders,
				Columns:        m.Columns,
				Rows:           m.Rows,
			}
			m.Close()
			return *m, func() tea.Msg {
				return PerformExportMsg{Options: opts}
			}

		case "left", "h":
			if m.FocusedField == FieldFormat {
				if m.SelectedFormat > 0 {
					m.SelectedFormat--
					m.updateFilenameExtension()
				}
				return *m, nil
			} else if m.FocusedField == FieldButtonCancel {
				m.FocusedField = FieldButtonExport
				return *m, nil
			}

		case "right", "l":
			if m.FocusedField == FieldFormat {
				if m.SelectedFormat < len(m.Formats)-1 {
					m.SelectedFormat++
					m.updateFilenameExtension()
				}
				return *m, nil
			} else if m.FocusedField == FieldButtonExport {
				m.FocusedField = FieldButtonCancel
				return *m, nil
			}

		case " ":
			if m.FocusedField == FieldHeaders {
				m.IncludeHeaders = !m.IncludeHeaders
				return *m, nil
			}
		}
	}

	if m.FocusedField == FieldFilename {
		var cmd tea.Cmd
		m.FilenameInput, cmd = m.FilenameInput.Update(msg)
		return *m, cmd
	}

	return *m, nil
}

func (m *Model) nextField() {
	m.FilenameInput.Blur()
	m.FocusedField = (m.FocusedField + 1) % 5
	if m.FocusedField == FieldFilename {
		m.FilenameInput.Focus()
	}
}

func (m *Model) prevField() {
	m.FilenameInput.Blur()
	if m.FocusedField == 0 {
		m.FocusedField = 4
	} else {
		m.FocusedField--
	}
	if m.FocusedField == FieldFilename {
		m.FilenameInput.Focus()
	}
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	modalWidth := 68
	if m.Width > 0 && modalWidth > m.Width-6 {
		modalWidth = m.Width - 6
	}

	var b strings.Builder
	b.WriteString(theme.ModalTitle.Render("EXPORT QUERY RESULTS") + "\n\n")

	// 1. Format Selection Section
	b.WriteString(theme.StyleFgLight.Bold(true).Render("1. Choose Export Format:") + "\n")
	var formatButtons []string
	for i, lbl := range m.FormatLabels {
		isSel := (i == m.SelectedFormat)
		isFocused := (m.FocusedField == FieldFormat && isSel)

		radio := "( )"
		if isSel {
			radio = "(•)"
		}

		str := fmt.Sprintf("%s %s", radio, lbl)
		if isFocused {
			formatButtons = append(formatButtons, theme.TreeSelected.Render(" "+str+" "))
		} else if isSel {
			formatButtons = append(formatButtons, theme.ButtonActive.Render(str))
		} else {
			formatButtons = append(formatButtons, theme.ButtonInactive.Render(str))
		}
	}
	b.WriteString("   " + strings.Join(formatButtons, "  ") + "\n\n")

	// 2. Filename Input Section
	b.WriteString(theme.StyleFgLight.Bold(true).Render("2. File Path & Name:") + "\n")
	inputPrefix := "   "
	if m.FocusedField == FieldFilename {
		inputPrefix = " ▶ "
	}
	b.WriteString(inputPrefix + m.FilenameInput.View() + "\n\n")

	// 3. Options Section
	b.WriteString(theme.StyleFgLight.Bold(true).Render("3. Options:") + "\n")
	checkIcon := "[ ]"
	if m.IncludeHeaders {
		checkIcon = "[X]"
	}
	headersOption := fmt.Sprintf("%s Include Column Headers (Space to toggle)", checkIcon)
	if m.FocusedField == FieldHeaders {
		b.WriteString(" ▶ " + theme.TreeSelected.Render(" "+headersOption+" ") + "\n\n")
	} else {
		b.WriteString("   " + theme.StyleFgLight.Render(headersOption) + "\n\n")
	}

	// 4. Action Buttons
	btnExport := theme.ButtonActive.Render(" Export (Enter) ")
	if m.FocusedField == FieldButtonExport {
		btnExport = lipgloss.NewStyle().Background(theme.ColorSecondary).Foreground(theme.ColorBgDark).Bold(true).Padding(0, 2).Render(" ▶ Export (Enter) ◀ ")
	}

	btnCancel := theme.ButtonInactive.Render(" Cancel (Esc) ")
	if m.FocusedField == FieldButtonCancel {
		btnCancel = lipgloss.NewStyle().Background(theme.ColorError).Foreground(lipgloss.Color("#FFF")).Bold(true).Padding(0, 2).Render(" ▶ Cancel (Esc) ◀ ")
	}

	b.WriteString("   " + lipgloss.JoinHorizontal(lipgloss.Top, btnExport, "   ", btnCancel))

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
