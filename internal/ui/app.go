package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/export"
	"dbterm/internal/history"
	"dbterm/internal/ui/theme"
	aiview "dbterm/internal/ui/views/ai"
	"dbterm/internal/ui/views/connection"
	"dbterm/internal/ui/views/editor"
	exportview "dbterm/internal/ui/views/export"
	"dbterm/internal/ui/views/explorer"
	"dbterm/internal/ui/views/help"
	historyview "dbterm/internal/ui/views/history"
	"dbterm/internal/ui/views/results"
)

type FocusArea int

const (
	FocusExplorer FocusArea = iota
	FocusEditor
	FocusResults
)

type QueryFinishedMsg struct {
	Query    string
	Result   *db.QueryResult
	Error    error
	Duration time.Duration
}

type Model struct {
	Config             *config.Config
	ConfigPath         string
	ActiveProfile      *config.ConnectionProfile
	Driver             db.Driver
	Focus              FocusArea
	Explorer           explorer.Model
	Editor             editor.Model
	Results            results.Model
	ConnModal          connection.Model
	HelpModal          help.Model
	HistoryModal       historyview.Model
	ExportModal        exportview.Model
	AIModal            aiview.Model
	SaveModal          editor.SaveModal
	HistoryManager     *history.Manager
	Spinner            spinner.Model
	Executing          bool
	ExecutionQuery     string
	cancelExec         context.CancelFunc
	Width              int
	Height             int
	CustomSidebarWidth int
	CustomEditorHeight int
	DraggingVSplitter  bool
	DraggingHSplitter  bool
	LastExecDuration   time.Duration
	LastRowCount       int
	ErrorMessage       string
	StatusToast        string
	StatusToastTime    time.Time
}

func NewApp(cfg *config.Config, configPath string, initialProfile *config.ConnectionProfile) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColorYellow)

	var activeDriver db.Driver
	if initialProfile != nil {
		driver, err := db.NewDriver(initialProfile)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := driver.Connect(ctx, initialProfile); err == nil {
				activeDriver = driver
			}
		}
	}

	activeDB := "master"
	if initialProfile != nil {
		activeDB = initialProfile.Database
	}

	histMgr := history.NewManager(200)
	expl := explorer.New(cfg, initialProfile, activeDriver, activeDB)
	ed := editor.New("")
	res := results.New()
	connMod := connection.New(cfg, configPath)
	helpMod := help.New()
	histMod := historyview.New(histMgr)
	exportMod := exportview.New()
	saveMod := editor.NewSaveModal()

	aiCfg := config.AIConfig{Enabled: true}
	if cfg != nil {
		aiCfg = cfg.AI
		if !aiCfg.Enabled {
			aiCfg.Enabled = true // default enabled
		}
	}
	aiMod := aiview.New(aiCfg)

	app := Model{
		Config:         cfg,
		ConfigPath:     configPath,
		ActiveProfile:  initialProfile,
		Driver:         activeDriver,
		Focus:          FocusEditor,
		Explorer:       expl,
		Editor:         ed,
		Results:        res,
		ConnModal:      connMod,
		HelpModal:      helpMod,
		HistoryModal:   histMod,
		ExportModal:    exportMod,
		AIModal:        aiMod,
		SaveModal:      saveMod,
		HistoryManager: histMgr,
		Spinner:        s,
		Executing:      false,
	}

	if initialProfile == nil || activeDriver == nil {
		app.ConnModal.Open()
	}

	return app
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
	)
}

func (m *Model) getSidebarWidth() int {
	if !m.Explorer.Visible {
		return 0
	}
	if m.CustomSidebarWidth > 0 {
		w := m.CustomSidebarWidth
		if w < 16 {
			w = 16
		}
		if m.Width > 40 && w > m.Width-30 {
			w = m.Width - 30
		}
		return w
	}
	w := 32
	if m.Width < 100 {
		w = 26
	}
	return w
}

func (m *Model) getEditorHeight() int {
	topBarHeight := 1
	statusBarHeight := 1
	borderRows := 4 // 2 rows for editor borders + 2 rows for results borders
	innerAvailableHeight := m.Height - topBarHeight - statusBarHeight - borderRows
	if innerAvailableHeight < 4 {
		return 2
	}
	if m.CustomEditorHeight > 0 {
		h := m.CustomEditorHeight
		if h < 2 {
			h = 2
		}
		if h > innerAvailableHeight-2 {
			h = innerAvailableHeight - 2
		}
		return h
	}
	return innerAvailableHeight / 2
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.updateLayout()
		return m, nil

	case spinner.TickMsg:
		if m.Executing {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		if m.AIModal.Active {
			var cmd tea.Cmd
			m.AIModal, cmd = m.AIModal.Update(msg)
			cmds = append(cmds, cmd)
		}

	case aiview.ApplyAISQLMsg:
		if msg.NewTab {
			m.Editor.NewTab("", msg.SQL)
		} else {
			m.Editor.InsertQuery(msg.SQL)
		}
		m.Focus = FocusEditor
		m.Editor.Focus()
		m.StatusToast = "✓ Query inserted from AI Assistant"
		m.StatusToastTime = time.Now()
		return m, nil

	case aiview.AIDownloadTickMsg, aiview.AIDownloadProgressMsg, aiview.AIDownloadDoneMsg, aiview.AIGenerationDoneMsg:
		if m.AIModal.Active {
			var cmd tea.Cmd
			m.AIModal, cmd = m.AIModal.Update(msg)
			return m, cmd
		}

	case editor.ExecuteQueryMsg:
		if m.Driver == nil {
			m.ErrorMessage = "Not connected to any database. Press Ctrl+O to connect."
			return m, nil
		}
		if strings.TrimSpace(msg.Query) == "" {
			return m, nil
		}
		m.Executing = true
		m.ErrorMessage = ""
		m.ExecutionQuery = msg.Query

		execCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		m.cancelExec = cancel

		startTime := time.Now()
		execCmd := func() tea.Msg {
			res, err := m.Driver.ExecuteQuery(execCtx, msg.Query)
			return QueryFinishedMsg{
				Query:    msg.Query,
				Result:   res,
				Error:    err,
				Duration: time.Since(startTime),
			}
		}
		return m, tea.Batch(execCmd, m.Spinner.Tick)

	case QueryFinishedMsg:
		m.Executing = false
		m.cancelExec = nil
		m.LastExecDuration = msg.Duration

		dbName := "unknown"
		if m.Driver != nil {
			dbName = m.Driver.GetActiveDatabase()
		}
		serverName := "localhost"
		if m.ActiveProfile != nil {
			serverName = m.ActiveProfile.Host
		}

		if msg.Result != nil {
			m.LastRowCount = len(msg.Result.Rows)
			m.Results.SetResult(msg.Result)
		}

		success := (msg.Error == nil)
		errStr := ""
		if msg.Error != nil {
			errStr = msg.Error.Error()
			m.ErrorMessage = errStr
		} else {
			m.ErrorMessage = ""
		}

		// Record in history
		m.HistoryManager.Add(history.HistoryEntry{
			Query:      msg.Query,
			Database:   dbName,
			Server:     serverName,
			RowCount:   m.LastRowCount,
			Duration:   msg.Duration,
			Success:    success,
			ErrorMsg:   errStr,
			ExecutedAt: time.Now(),
		})

		return m, nil

	case explorer.ScriptTableMsg:
		m.Editor.InsertQuery(msg.Query)
		m.Focus = FocusEditor
		m.Editor.Focus()
		return m, nil

	case results.OpenExportModalMsg:
		if m.Results.Result != nil && len(m.Results.FilteredRows) > 0 {
			m.ExportModal.Open(m.Results.Result.Columns, m.Results.FilteredRows)
		}
		return m, nil

	case editor.EditorToastMsg:
		m.StatusToast = msg.Message
		m.StatusToastTime = time.Now()
		return m, nil

	case exportview.PerformExportMsg:
		destPath, err := export.Export(msg.Options)
		if err != nil {
			m.ErrorMessage = fmt.Sprintf("Export failed: %v", err)
		} else {
			m.StatusToast = fmt.Sprintf("✓ Exported %d rows to %s", len(msg.Options.Rows), filepath.Base(destPath))
			m.StatusToastTime = time.Now()
			m.ErrorMessage = ""
		}
		return m, nil

	case historyview.SelectHistoryQueryMsg:
		if msg.NewTab {
			m.Editor.NewTab("", msg.Query)
		} else {
			m.Editor.InsertQuery(msg.Query)
		}
		m.Focus = FocusEditor
		m.Editor.Focus()
		return m, nil

	case connection.ConnectProfileMsg:
		return m.handleConnectProfile(msg.Profile)

	case connection.ConfigUpdatedMsg:
		m.Config = msg.Config
		m.Explorer.Config = msg.Config
		m.Explorer.Refresh()
		return m, nil

	case explorer.ConnectServerMsg:
		return m.handleConnectProfile(msg.Profile)

	case explorer.SwitchDatabaseMsg:
		if m.Driver != nil {
			_ = m.Driver.SwitchDatabase(context.Background(), msg.Database)
			m.StatusToast = fmt.Sprintf("Active database: %s", msg.Database)
			m.StatusToastTime = time.Now()
		}
	case explorer.OpenFileMsg:
		m.Editor.OpenFile(msg.FilePath, msg.Content)
		m.Focus = FocusEditor
		m.Editor.Focus()
		m.StatusToast = fmt.Sprintf("✓ Opened %s", filepath.Base(msg.FilePath))
		m.StatusToastTime = time.Now()
		return m, nil

	case editor.PromptSaveFileMsg:
		m.SaveModal.Open(msg.Content, msg.DefaultName)
		return m, nil

	case editor.SaveFileSuccessMsg:
		m.Editor.SetSavedPath(msg.FilePath)
		m.Explorer.RefreshFiles()
		m.StatusToast = fmt.Sprintf("✓ Query saved to %s", msg.FilePath)
		m.StatusToastTime = time.Now()
		return m, nil

	case tea.MouseMsg:
		// 1. Modals have precedence if active
		if m.SaveModal.Active {
			var cmd tea.Cmd
			m.SaveModal, cmd = m.SaveModal.Update(msg)
			return m, cmd
		}
		if m.AIModal.Active {
			var cmd tea.Cmd
			m.AIModal, cmd = m.AIModal.Update(msg)
			return m, cmd
		}
		if m.ExportModal.Active {
			var cmd tea.Cmd
			m.ExportModal, cmd = m.ExportModal.Update(msg)
			return m, cmd
		}
		if m.HelpModal.Active {
			if msg.Action == tea.MouseActionPress {
				m.HelpModal.Close()
			}
			return m, nil
		}
		if m.ConnModal.Active {
			var cmd tea.Cmd
			m.ConnModal, cmd = m.ConnModal.Update(msg)
			return m, cmd
		}
		if m.HistoryModal.Active {
			var cmd tea.Cmd
			m.HistoryModal, cmd = m.HistoryModal.Update(msg)
			return m, cmd
		}
		if m.Results.InspectorOpen {
			var cmd tea.Cmd
			m.Results, cmd = m.Results.Update(msg)
			return m, cmd
		}

		sidebarWidth := m.getSidebarWidth()
		availableHeight := m.Height - 2
		editorInnerHeight := m.getEditorHeight()
		editorOuterHeight := editorInnerHeight + 2
		vSplitPos := sidebarWidth + 3
		hSplitPos := 1 + editorOuterHeight

		// 2. Active Splitter Dragging (highest priority)
		if m.DraggingVSplitter {
			if msg.Action == tea.MouseActionRelease {
				m.DraggingVSplitter = false
				return m, nil
			}
			newWidth := msg.X - 3
			if newWidth < 16 {
				newWidth = 16
			}
			if m.Width > 50 && newWidth > m.Width-35 {
				newWidth = m.Width - 35
			}
			m.CustomSidebarWidth = newWidth
			m.StatusToast = fmt.Sprintf("Sidebar width: %d cols", newWidth)
			m.StatusToastTime = time.Now()
			m.updateLayout()
			return m, nil
		}

		if m.DraggingHSplitter {
			if msg.Action == tea.MouseActionRelease {
				m.DraggingHSplitter = false
				return m, nil
			}
			newHeight := msg.Y - 2
			if newHeight < 2 {
				newHeight = 2
			}
			if newHeight > availableHeight-6 {
				newHeight = availableHeight - 6
			}
			m.CustomEditorHeight = newHeight
			m.StatusToast = fmt.Sprintf("Editor: %d rows | Results: %d rows", newHeight, availableHeight-newHeight)
			m.StatusToastTime = time.Now()
			m.updateLayout()
			return m, nil
		}

		// 3. Top Bar Clicks (Y == 0)
		if msg.Y == 0 && msg.Action == tea.MouseActionPress {
			if msg.X >= m.Width-75 {
				if strings.Contains(m.renderTopBar(), "AI") && msg.X >= m.Width-75 && msg.X < m.Width-60 {
					m.AIModal.Toggle(m.Driver, m.Editor.GetCurrentQuery(), m.ErrorMessage)
					return m, nil
				}
				if strings.Contains(m.renderTopBar(), "Connect") && msg.X >= m.Width-55 && msg.X < m.Width-40 {
					m.ConnModal.Open()
					return m, nil
				}
				if msg.X >= m.Width-40 && msg.X < m.Width-25 {
					if m.Results.Result != nil && len(m.Results.FilteredRows) > 0 {
						m.ExportModal.Open(m.Results.Result.Columns, m.Results.FilteredRows)
						return m, nil
					}
				}
				if msg.X >= m.Width-15 {
					m.HelpModal.Toggle()
					return m, nil
				}
			}
		}

		// 4. Detect Start of Splitter Dragging (generous hit zone for trackpad)
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Check vertical splitter between Explorer and Editor/Results
			if m.Explorer.Visible && msg.X >= vSplitPos-3 && msg.X <= vSplitPos+3 && msg.Y >= 1 && msg.Y < m.Height-1 {
				m.DraggingVSplitter = true
				m.Editor.MouseDown = false
				return m, nil
			}

			// Check horizontal splitter between Editor and Results
			if msg.X >= vSplitPos-1 && (msg.Y >= hSplitPos-1 && msg.Y <= hSplitPos+2) {
				m.DraggingHSplitter = true
				m.Editor.MouseDown = false
				return m, nil
			}
		}

		// If user is currently dragging inside editor, continue sending mouse events to editor
		if m.Editor.MouseDown {
			var cmd tea.Cmd
			m.Editor, cmd = m.Editor.HandleMouse(msg, msg.X-sidebarWidth-2, msg.Y-2)
			return m, cmd
		}

		// 5. Object Explorer (Left Panel)
		if m.Explorer.Visible && msg.X < vSplitPos-1 && msg.Y >= 1 && msg.Y < m.Height-1 {
			m.Focus = FocusExplorer
			m.Editor.Blur()
			m.Results.Blur()
			var cmd tea.Cmd
			m.Explorer, cmd = m.Explorer.HandleMouse(msg, msg.Y-2)
			return m, cmd
		}

		// 6. Editor Pane (Top Right)
		if msg.X >= sidebarWidth && msg.Y >= 1 && msg.Y < 1+editorOuterHeight {
			m.Focus = FocusEditor
			m.Editor.Focus()
			m.Results.Blur()
			var cmd tea.Cmd
			m.Editor, cmd = m.Editor.HandleMouse(msg, msg.X-sidebarWidth-2, msg.Y-2)
			return m, cmd
		}

		// 7. Results Pane (Bottom Right)
		if msg.X >= sidebarWidth && msg.Y >= 1+editorOuterHeight && msg.Y < m.Height-1 {
			m.Focus = FocusResults
			m.Editor.Blur()
			m.Results.Focus()
			var cmd tea.Cmd
			m.Results, cmd = m.Results.HandleMouse(msg, msg.X-sidebarWidth-1, msg.Y-(1+editorOuterHeight)-1)
			return m, cmd
		}

	case tea.KeyMsg:
		// Cancel active query with Esc
		if m.Executing && msg.String() == "esc" {
			if m.cancelExec != nil {
				m.cancelExec()
				m.Executing = false
				m.ErrorMessage = "Query execution cancelled by user."
				return m, nil
			}
		}

		// Modal handling
		if m.SaveModal.Active {
			var cmd tea.Cmd
			m.SaveModal, cmd = m.SaveModal.Update(msg)
			return m, cmd
		}

		if m.AIModal.Active {
			var cmd tea.Cmd
			m.AIModal, cmd = m.AIModal.Update(msg)
			return m, cmd
		}

		if m.ExportModal.Active {
			var cmd tea.Cmd
			m.ExportModal, cmd = m.ExportModal.Update(msg)
			return m, cmd
		}

		if m.HelpModal.Active {
			var cmd tea.Cmd
			m.HelpModal, cmd = m.HelpModal.Update(msg)
			return m, cmd
		}

		if m.ConnModal.Active {
			var cmd tea.Cmd
			m.ConnModal, cmd = m.ConnModal.Update(msg)
			return m, cmd
		}

		if m.HistoryModal.Active {
			var cmd tea.Cmd
			m.HistoryModal, cmd = m.HistoryModal.Update(msg)
			return m, cmd
		}

		if m.Results.InspectorOpen {
			var cmd tea.Cmd
			m.Results, cmd = m.Results.Update(msg)
			return m, cmd
		}

		// Global Keybindings
		switch msg.String() {
		case "ctrl+q":
			return m, tea.Quit
		case "ctrl+c":
			if m.Focus != FocusEditor {
				return m, tea.Quit
			}
			// When in editor, route Ctrl+C to Editor (Copy)
			var cmd tea.Cmd
			m.Editor, cmd = m.Editor.Update(msg)
			return m, cmd
		case "ctrl+k", "f4":
			m.AIModal.Toggle(m.Driver, m.Editor.GetCurrentQuery(), m.ErrorMessage)
			return m, nil
		case "ctrl+o":
			m.ConnModal.Open()
			return m, nil
		case "ctrl+h":
			m.HistoryModal.Toggle()
			return m, nil
		case "ctrl+s":
			if m.Results.Result != nil && len(m.Results.FilteredRows) > 0 {
				m.ExportModal.Open(m.Results.Result.Columns, m.Results.FilteredRows)
				return m, nil
			}
		case "f1":
			m.HelpModal.Toggle()
			return m, nil
		case "?":
			if m.Focus != FocusEditor {
				m.HelpModal.Toggle()
				return m, nil
			}
		case "f8":
			m.Explorer.Visible = !m.Explorer.Visible
			m.updateLayout()
			return m, nil

		// Pane Resizing Shortcuts
		case "alt+left", "ctrl+shift+left":
			w := m.getSidebarWidth() - 3
			if w < 16 {
				w = 16
			}
			m.CustomSidebarWidth = w
			m.updateLayout()
			return m, nil

		case "alt+right", "ctrl+shift+right":
			w := m.getSidebarWidth() + 3
			if m.Width > 40 && w > m.Width-30 {
				w = m.Width - 30
			}
			m.CustomSidebarWidth = w
			m.updateLayout()
			return m, nil

		case "alt+up", "ctrl+shift+up":
			h := m.getEditorHeight() - 2
			if h < 4 {
				h = 4
			}
			m.CustomEditorHeight = h
			m.updateLayout()
			return m, nil

		case "alt+down", "ctrl+shift+down":
			h := m.getEditorHeight() + 2
			availableHeight := m.Height - 2
			if h > availableHeight-4 {
				h = availableHeight - 4
			}
			m.CustomEditorHeight = h
			m.updateLayout()
			return m, nil

		case "alt+=", "alt+-", "ctrl+=":
			// Reset layout splits to default
			m.CustomSidebarWidth = 0
			m.CustomEditorHeight = 0
			m.updateLayout()
			m.StatusToast = "Layout splits reset to default (50/50)"
			m.StatusToastTime = time.Now()
			return m, nil

		case "tab":
			if m.Focus == FocusExplorer {
				m.Focus = FocusEditor
				m.Editor.Focus()
				m.Results.Blur()
			} else if m.Focus == FocusEditor {
				m.Focus = FocusResults
				m.Editor.Blur()
				m.Results.Focus()
			} else {
				if m.Explorer.Visible {
					m.Focus = FocusExplorer
					m.Editor.Blur()
					m.Results.Blur()
				} else {
					m.Focus = FocusEditor
					m.Editor.Focus()
					m.Results.Blur()
				}
			}
			return m, nil
		case "shift+tab":
			if m.Focus == FocusResults {
				m.Focus = FocusEditor
				m.Editor.Focus()
				m.Results.Blur()
			} else if m.Focus == FocusEditor {
				if m.Explorer.Visible {
					m.Focus = FocusExplorer
					m.Editor.Blur()
					m.Results.Blur()
				} else {
					m.Focus = FocusResults
					m.Editor.Blur()
					m.Results.Focus()
				}
			} else {
				m.Focus = FocusResults
				m.Editor.Blur()
				m.Results.Focus()
			}
			return m, nil
		}
	}

	// Route keys to active pane
	switch m.Focus {
	case FocusExplorer:
		var cmd tea.Cmd
		m.Explorer, cmd = m.Explorer.Update(msg)
		cmds = append(cmds, cmd)
	case FocusEditor:
		var cmd tea.Cmd
		m.Editor, cmd = m.Editor.Update(msg)
		cmds = append(cmds, cmd)
	case FocusResults:
		var cmd tea.Cmd
		m.Results, cmd = m.Results.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateLayout() {
	if m.Width == 0 || m.Height == 0 {
		return
	}

	topBarHeight := 1
	statusBarHeight := 1
	availableOuterHeight := m.Height - topBarHeight - statusBarHeight
	if availableOuterHeight < 6 {
		availableOuterHeight = 6
	}

	sidebarOuterWidth := m.getSidebarWidth()
	sidebarInnerWidth := sidebarOuterWidth - 4 // 2 borders + 2 padding
	if sidebarInnerWidth < 10 {
		sidebarInnerWidth = 10
	}

	var mainInnerWidth int
	if m.Explorer.Visible {
		mainOuterWidth := m.Width - sidebarOuterWidth
		mainInnerWidth = mainOuterWidth - 4 // 2 borders + 2 padding
	} else {
		mainInnerWidth = m.Width - 4
	}
	if mainInnerWidth < 20 {
		mainInnerWidth = 20
	}

	explorerInnerHeight := availableOuterHeight - 2 // 2 borders (top+bottom)
	if explorerInnerHeight < 2 {
		explorerInnerHeight = 2
	}

	innerAvailableHeight := availableOuterHeight - 4 // 2 borders for editor + 2 for results
	if innerAvailableHeight < 4 {
		innerAvailableHeight = 4
	}

	editorInnerHeight := m.getEditorHeight()
	resultsInnerHeight := innerAvailableHeight - editorInnerHeight
	if resultsInnerHeight < 2 {
		resultsInnerHeight = 2
	}

	m.Explorer.SetSize(sidebarInnerWidth, explorerInnerHeight)
	m.Editor.SetSize(mainInnerWidth, editorInnerHeight)
	m.Results.SetSize(mainInnerWidth, resultsInnerHeight)
	m.ConnModal.SetSize(m.Width, m.Height)
	m.HistoryModal.SetSize(m.Width, m.Height)
	m.ExportModal.SetSize(m.Width, m.Height)
	m.AIModal.SetSize(m.Width, m.Height)
	m.SaveModal.SetSize(m.Width, m.Height)
	m.HelpModal.Width = m.Width
	m.HelpModal.Height = m.Height
}

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing dbterm..."
	}

	topBar := m.renderTopBar()

	var mainLayout string
	var rightPanes string

	editorStyle := theme.PaneBase
	if m.Focus == FocusEditor {
		editorStyle = theme.PaneActive
	}
	editorBox := editorStyle.Render(m.Editor.View())

	resultsStyle := theme.PaneBase
	if m.Focus == FocusResults {
		resultsStyle = theme.PaneActive
	}
	resultsBox := resultsStyle.Render(m.Results.View())

	rightPanes = lipgloss.JoinVertical(lipgloss.Left, editorBox, resultsBox)

	if m.Explorer.Visible {
		explorerStyle := theme.PaneBase
		if m.Focus == FocusExplorer {
			explorerStyle = theme.PaneActive
		}
		explorerBox := explorerStyle.Render(m.Explorer.View())
		mainLayout = lipgloss.JoinHorizontal(lipgloss.Top, explorerBox, rightPanes)
	} else {
		mainLayout = rightPanes
	}

	statusBar := m.renderStatusBar()

	screen := lipgloss.JoinVertical(lipgloss.Left, topBar, mainLayout, statusBar)

	// Modal Overlays
	if m.AIModal.Active {
		return m.placeOverlay(screen, m.AIModal.View())
	}
	if m.ExportModal.Active {
		return m.placeOverlay(screen, m.ExportModal.View())
	}
	if m.HelpModal.Active {
		return m.placeOverlay(screen, m.HelpModal.View())
	}
	if m.ConnModal.Active {
		return m.placeOverlay(screen, m.ConnModal.View())
	}
	if m.HistoryModal.Active {
		return m.placeOverlay(screen, m.HistoryModal.View())
	}
	if m.SaveModal.Active {
		return m.placeOverlay(screen, m.SaveModal.View())
	}

	return screen
}

func (m Model) renderTopBar() string {
	logo := theme.TopBarBadge.Render(" dbterm ")

	connInfo := "Disconnected (Press Ctrl+O)"
	if m.Driver != nil && m.ActiveProfile != nil {
		auth := "SQL"
		if m.ActiveProfile.AuthType == config.AuthTypeWindows {
			auth = "Windows"
		}
		connInfo = fmt.Sprintf(" %s:%d | User: %s (%s) ", m.ActiveProfile.Host, m.ActiveProfile.Port, m.ActiveProfile.User, auth)
	}
	connBadge := theme.TopBar.Render(connInfo)

	dbName := "master"
	if m.ActiveProfile != nil && m.ActiveProfile.Database != "" {
		dbName = m.ActiveProfile.Database
	}
	dbBadge := theme.TopBarDB.Render(" DB: " + dbName + " ")

	rightHints := theme.TopBar.Render("[Ctrl+K: AI]  [Ctrl+O: Connect]  [Ctrl+S: Export]  [Ctrl+H: History]  [F1/?: Help] ")

	leftPart := lipgloss.JoinHorizontal(lipgloss.Top, logo, connBadge, dbBadge)
	spaceLen := m.Width - lipgloss.Width(leftPart) - lipgloss.Width(rightHints)
	if spaceLen < 0 {
		spaceLen = 0
	}
	spacer := theme.TopBar.Render(strings.Repeat(" ", spaceLen))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, spacer, rightHints)
}

func (m Model) renderStatusBar() string {
	var statusBadge string
	if m.Executing {
		statusBadge = theme.StatusBadgeExec.Render(fmt.Sprintf(" %s Executing (Esc to cancel)... ", m.Spinner.View()))
	} else if m.ErrorMessage != "" {
		statusBadge = theme.StatusBadgeError.Render(" ERROR ")
	} else if m.StatusToast != "" && time.Since(m.StatusToastTime) < 5*time.Second {
		statusBadge = theme.StatusBadgeReady.Render(" " + m.StatusToast + " ")
	} else {
		statusBadge = theme.StatusBadgeReady.Render(" Ready ")
	}

	var statsInfo string
	if m.ErrorMessage != "" {
		statsInfo = fmt.Sprintf(" %s ", m.ErrorMessage)
		if len(statsInfo) > 50 {
			statsInfo = statsInfo[:47] + "..."
		}
	} else if m.StatusToast != "" && time.Since(m.StatusToastTime) < 5*time.Second {
		statsInfo = ""
	} else if m.LastExecDuration > 0 {
		statsInfo = fmt.Sprintf(" Exec: %v | Rows: %d ", m.LastExecDuration.Round(time.Millisecond), m.LastRowCount)
	}

	line, col := m.Editor.GetCursorPosition()
	posInfo := fmt.Sprintf(" Ln %d, Col %d ", line, col)

	navHints := "[F5: Run | Ctrl+K: AI | Tab: Pane | Alt+Arrows: Resize | F1: Help] "

	leftPart := lipgloss.JoinHorizontal(lipgloss.Top, statusBadge, theme.StatusBar.Render(statsInfo))
	rightPart := theme.StatusBar.Render(posInfo + " " + navHints)

	spaceLen := m.Width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
	if spaceLen < 0 {
		spaceLen = 0
	}
	spacer := theme.StatusBar.Render(strings.Repeat(" ", spaceLen))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, spacer, rightPart)
}

func (m Model) placeOverlay(background, overlay string) string {
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	posX := (m.Width - overlayWidth) / 2
	posY := (m.Height - overlayHeight) / 2
	if posX < 0 {
		posX = 0
	}
	if posY < 0 {
		posY = 0
	}

	return lipgloss.Place(
		m.Width,
		m.Height,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(theme.ColorBgDark),
	)
}

func (m Model) handleConnectProfile(profile *config.ConnectionProfile) (tea.Model, tea.Cmd) {
	if profile == nil {
		return m, nil
	}
	m.ActiveProfile = profile
	m.ErrorMessage = ""
	if m.Driver != nil {
		_ = m.Driver.Close()
	}

	driver, err := db.NewDriver(profile)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Driver error: %v", err)
		return m, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := driver.Connect(ctx, profile); err != nil {
		m.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return m, nil
	}

	m.Driver = driver
	m.Explorer.SetActiveConnection(profile, driver, profile.Database)
	m.StatusToast = fmt.Sprintf("✓ Connected to %s (%s)", profile.Name, profile.Database)
	m.StatusToastTime = time.Now()
	m.updateLayout()
	return m, nil
}
