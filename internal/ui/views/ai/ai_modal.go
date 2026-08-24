package aiview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/ai"
	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/ui/syntax"
	"dbterm/internal/ui/theme"
)

// Messages emitted by AI Modal
type ApplyAISQLMsg struct {
	SQL    string
	NewTab bool
}

type AIGenerationDoneMsg struct {
	Response *ai.AIResponse
	Error    error
}

type AIDownloadProgressMsg struct {
	Progress ai.DownloadProgress
}

type AIDownloadTickMsg struct{}

type AIDownloadDoneMsg struct {
	Path  string
	Error error
}

type Model struct {
	Active        bool
	Width         int
	Height        int
	Engine        ai.Engine
	Driver        db.Driver
	Config        config.AIConfig
	Mode          ai.AIMode
	TextInput     textinput.Model
	Spinner       spinner.Model
	CurrentSQL    string
	ErrorMessage  string
	GeneratedSQL  string
	Explanation   string
	Generating    bool
	Downloading   bool
	DownloadProg  ai.DownloadProgress
	NeedsDownload bool
	CancelGen     context.CancelFunc
	CancelDown    context.CancelFunc
}

func New(cfg config.AIConfig) Model {
	ti := textinput.New()
	ti.Placeholder = "Describe the SQL query you need in plain French or English..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 64

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColorSecondary)

	engine := ai.NewEngine(cfg)
	needsDown := !ai.IsModelInstalled(ai.DefaultModel)

	return Model{
		Active:        false,
		Engine:        engine,
		Config:        cfg,
		Mode:          ai.AIModeGenerate,
		TextInput:     ti,
		Spinner:       s,
		NeedsDownload: needsDown,
	}
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	modalWidth := 80
	if w > 0 && modalWidth > w-6 {
		modalWidth = w - 6
	}
	m.TextInput.Width = modalWidth - 12
}

func (m *Model) Open(driver db.Driver, currentSQL, lastError string) {
	m.Active = true
	m.Driver = driver
	m.CurrentSQL = currentSQL
	m.ErrorMessage = lastError
	m.GeneratedSQL = ""
	m.Explanation = ""
	m.Generating = false
	m.NeedsDownload = !ai.IsModelInstalled(ai.DefaultModel)

	if m.ErrorMessage != "" {
		m.Mode = ai.AIModeFixError
		m.TextInput.Placeholder = "Explain or press Enter to automatically fix this database error..."
	} else {
		m.Mode = ai.AIModeGenerate
		m.TextInput.Placeholder = "Describe what data you want (e.g. 'Top 5 clients par chiffre d affaires en 2025')..."
	}

	m.TextInput.SetValue("")
	m.TextInput.Focus()
}

func (m *Model) Close() {
	m.Active = false
	if m.CancelGen != nil {
		m.CancelGen()
		m.CancelGen = nil
	}
	if m.CancelDown != nil {
		m.CancelDown()
		m.CancelDown = nil
	}
	m.Generating = false
	m.Downloading = false
}

func (m *Model) Toggle(driver db.Driver, currentSQL, lastError string) {
	if m.Active {
		m.Close()
	} else {
		m.Open(driver, currentSQL, lastError)
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.Generating || m.Downloading {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case AIDownloadTickMsg:
		if !m.Downloading {
			return m, nil
		}
		prog := ai.GetGlobalDownloadProgress()
		m.DownloadProg = prog
		if prog.Done {
			m.Downloading = false
			m.CancelDown = nil
			if prog.Error == nil {
				m.NeedsDownload = false
				m.Explanation = fmt.Sprintf("✓ Model %s downloaded successfully! Ready for offline SQL generation.", ai.DefaultModel.Name)
			} else {
				m.Explanation = fmt.Sprintf("Download failed: %v", prog.Error)
			}
			return m, nil
		}
		return m, tea.Batch(checkDownloadTickCmd(), m.Spinner.Tick)

	case AIDownloadProgressMsg:
		m.DownloadProg = msg.Progress
		return m, nil

	case AIDownloadDoneMsg:
		m.Downloading = false
		m.CancelDown = nil
		if msg.Error == nil {
			m.NeedsDownload = false
			m.Explanation = fmt.Sprintf("✓ Model %s downloaded successfully! Ready for offline SQL generation.", ai.DefaultModel.Name)
		} else {
			m.Explanation = fmt.Sprintf("Download failed: %v", msg.Error)
		}
		return m, nil

	case AIGenerationDoneMsg:
		m.Generating = false
		m.CancelGen = nil
		if msg.Error != nil {
			m.Explanation = fmt.Sprintf("AI Error: %v", msg.Error)
			m.GeneratedSQL = ""
		} else if msg.Response != nil {
			m.GeneratedSQL = msg.Response.GeneratedSQL
			m.Explanation = msg.Response.Explanation
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.Generating {
				if m.CancelGen != nil {
					m.CancelGen()
					m.CancelGen = nil
				}
				m.Generating = false
				return m, nil
			}
			if m.Downloading {
				if m.CancelDown != nil {
					m.CancelDown()
					m.CancelDown = nil
				}
				m.Downloading = false
				return m, nil
			}
			m.Close()
			return m, nil

		case "tab":
			if !m.NeedsDownload && !m.Generating {
				m.Mode = (m.Mode + 1) % 4
				m.GeneratedSQL = ""
				m.Explanation = ""
				switch m.Mode {
				case ai.AIModeGenerate:
					m.TextInput.Placeholder = "Describe query in natural language (e.g. 'Commandes > 500€ avec nom client')..."
				case ai.AIModeFixError:
					m.TextInput.Placeholder = "Press Enter to automatically fix current SQL error..."
				case ai.AIModeExplain:
					m.TextInput.Placeholder = "Press Enter to explain active query..."
				case ai.AIModeOptimize:
					m.TextInput.Placeholder = "Press Enter to optimize query performance..."
				}
				return m, nil
			}

		case "enter":
			if m.NeedsDownload && !m.Downloading {
				// Start real-time background download
				m.Downloading = true
				m.DownloadProg = ai.DownloadProgress{
					TotalBytes: ai.DefaultModel.SizeBytes,
					Percentage: 0,
				}
				ctx, cancel := context.WithCancel(context.Background())
				m.CancelDown = cancel
				ai.StartBackgroundDownload(ctx, ai.DefaultModel)

				return m, tea.Batch(checkDownloadTickCmd(), m.Spinner.Tick)
			}

			if m.GeneratedSQL != "" && !m.Generating {
				// Apply SQL into active editor tab
				sql := m.GeneratedSQL
				m.Close()
				return m, func() tea.Msg {
					return ApplyAISQLMsg{SQL: sql, NewTab: false}
				}
			}

			if !m.Generating && !m.Downloading {
				// Trigger Generation
				m.Generating = true
				m.GeneratedSQL = ""
				m.Explanation = ""

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				m.CancelGen = cancel

				req := ai.AIRequest{
					Mode:         m.Mode,
					UserPrompt:   m.TextInput.Value(),
					CurrentSQL:   m.CurrentSQL,
					ErrorMessage: m.ErrorMessage,
					Driver:       m.Driver,
				}

				genCmd := func() tea.Msg {
					resp, err := m.Engine.Generate(ctx, req)
					return AIGenerationDoneMsg{Response: resp, Error: err}
				}
				return m, tea.Batch(genCmd, m.Spinner.Tick)
			}

		case "ctrl+n":
			if m.GeneratedSQL != "" {
				sql := m.GeneratedSQL
				m.Close()
				return m, func() tea.Msg {
					return ApplyAISQLMsg{SQL: sql, NewTab: true}
				}
			}

		case "1", "2", "3", "4":
			if m.TextInput.Value() == "" && !m.NeedsDownload {
				switch msg.String() {
				case "1":
					m.Mode = ai.AIModeGenerate
				case "2":
					m.Mode = ai.AIModeFixError
				case "3":
					m.Mode = ai.AIModeExplain
				case "4":
					m.Mode = ai.AIModeOptimize
				}
				return m, nil
			}
		}
	}

	if !m.NeedsDownload && !m.Generating {
		var cmd tea.Cmd
		m.TextInput, cmd = m.TextInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	modalWidth := 82
	if m.Width > 0 && modalWidth > m.Width-4 {
		modalWidth = m.Width - 4
	}

	var b strings.Builder

	// Header
	dbInfo := "No Database"
	if m.Driver != nil {
		dbInfo = m.Driver.GetActiveDatabase()
	}
	headerTitle := theme.ModalTitle.Render(" 🤖 AI SQL ASSISTANT ")
	dbBadge := theme.TopBarDB.Render(" DB: " + dbInfo + " ")
	modelBadge := theme.StatusBadgeReady.Render(" " + m.Engine.GetModelInfo() + " ")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerTitle, "  ", dbBadge, "  ", modelBadge) + "\n\n")

	// State 1: Download Required
	if m.NeedsDownload {
		b.WriteString(theme.PaneHeader.Render(" OFFLINE MODEL SETUP (ONE-TIME) ") + "\n\n")
		b.WriteString(theme.StyleFgLight.Render(fmt.Sprintf("To generate SQL offline with high precision on your Intel Core Ultra 7,\ndbterm uses the local model %s (%s).\n\n", ai.DefaultModel.Name, ai.DefaultModel.SizeDisplay)))

		if m.Downloading {
			pct := m.DownloadProg.Percentage
			barWidth := modalWidth - 16
			filled := int(float64(barWidth) * (pct / 100.0))
			if filled > barWidth {
				filled = barWidth
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			barRendered := lipgloss.NewStyle().Foreground(theme.ColorSecondary).Render(bar)

			speedMB := float64(m.DownloadProg.SpeedBytesSec) / (1024 * 1024)
			downloadedMB := float64(m.DownloadProg.BytesRead) / (1024 * 1024)
			totalBytes := m.DownloadProg.TotalBytes
			if totalBytes <= 0 {
				totalBytes = ai.DefaultModel.SizeBytes
			}
			totalMB := float64(totalBytes) / (1024 * 1024)
			if pct <= 0 && totalBytes > 0 && m.DownloadProg.BytesRead > 0 {
				pct = float64(m.DownloadProg.BytesRead) / float64(totalBytes) * 100.0
			}

			b.WriteString(fmt.Sprintf("  %s Downloading %s... %.1f%%\n", m.Spinner.View(), ai.DefaultModel.Name, pct))
			b.WriteString(fmt.Sprintf("  [%s]\n", barRendered))
			b.WriteString(theme.StyleFgMuted.Render(fmt.Sprintf("  %.1f MB / %.1f MB (%.1f MB/s | ETA: %ds)\n\n", downloadedMB, totalMB, speedMB, m.DownloadProg.EtaSeconds)))
			b.WriteString(theme.StyleFgDim.Render("  Press Esc to cancel download.\n"))
		} else {
			if m.Explanation != "" {
				b.WriteString(theme.StyleError.Render("  "+m.Explanation) + "\n\n")
			}
			b.WriteString(theme.ButtonActive.Render(fmt.Sprintf(" Enter: Download %s (%s) ", ai.DefaultModel.Name, ai.DefaultModel.SizeDisplay)) + "   " + theme.ButtonInactive.Render(" Esc: Cancel ") + "\n\n")
			b.WriteString(theme.StyleFgMuted.Render("  Model will be cached locally in ~/.local/share/dbterm/models/ for instant offline access."))
		}

		return lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.ColorSecondary).
			Background(theme.ColorBgDark).
			Padding(1, 2).
			Width(modalWidth).
			Render(b.String())
	}

	// State 2: Mode Selector Bar
	modes := []string{"1: Text-to-SQL", "2: Fix Error", "3: Explain", "4: Optimize"}
	var modeHeaders []string
	for i, name := range modes {
		if int(m.Mode) == i {
			modeHeaders = append(modeHeaders, theme.ButtonActive.Render(" "+name+" "))
		} else {
			modeHeaders = append(modeHeaders, theme.ButtonInactive.Render(" "+name+" "))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, modeHeaders...) + "\n\n")

	// Prompt Input Area
	b.WriteString(theme.StyleFgMuted.Render("Prompt (French / English):") + "\n")
	b.WriteString(lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(theme.ColorPrimary).Padding(0, 1).Width(modalWidth-6).Render(m.TextInput.View()) + "\n\n")

	if m.Generating {
		b.WriteString(fmt.Sprintf("  %s Generating SQL query using local schema context...\n\n", m.Spinner.View()))
	} else if m.GeneratedSQL != "" {
		b.WriteString(theme.PaneHeader.Render(" GENERATED SQL ") + "\n")
		highlighted := syntax.HighlightSQL(m.GeneratedSQL)
		b.WriteString(lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorSecondary).
			Background(theme.ColorBgLighter).
			Padding(0, 1).
			Width(modalWidth-6).
			Render(highlighted) + "\n\n")

		if m.Explanation != "" {
			b.WriteString(theme.StyleFgLight.Render(m.Explanation) + "\n\n")
		}

		footer := lipgloss.JoinHorizontal(
			lipgloss.Top,
			theme.ButtonActive.Render("Enter: Insert in Editor"),
			"  ",
			theme.ButtonInactive.Render("Ctrl+N: Open in New Tab"),
			"  ",
			theme.ButtonInactive.Render("Esc: Close"),
		)
		b.WriteString(footer)
	} else {
		if m.Explanation != "" {
			b.WriteString(theme.StyleFgLight.Render(m.Explanation) + "\n\n")
		}
		b.WriteString(theme.StyleFgMuted.Render("Press Enter to generate | Tab to switch mode (Text-to-SQL / Fix / Explain) | Esc to close"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}

func checkDownloadTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return AIDownloadTickMsg{}
	})
}
