package gui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/gui/dialogs"
)

type App struct {
	FyneApp        fyne.App
	Window         fyne.Window
	Config         *config.Config
	ConfigPath     string
	ActiveProfile  *config.ConnectionProfile
	Driver         db.Driver
	cancelExec     context.CancelFunc
	isExecuting    bool
	lastError      string

	// UI Controllers
	Explorer       *ExplorerController
	Editor         *EditorController
	Results        *ResultsController

	// Status & Toolbar widgets
	statusLabel    *widget.Label
	serverBadge    *widget.Button
	dbSelect       *widget.Select
	execBtn        *widget.Button
	stopBtn        *widget.Button
	progress       *widget.ProgressBarInfinite
}

func NewApp(cfg *config.Config, cfgPath string, initialProfile *config.ConnectionProfile, initialDriver db.Driver) *App {
	a := app.NewWithID("com.svivenot.dbterm.gui")
	a.Settings().SetTheme(&SSMSDarkTheme{})

	w := a.NewWindow("dbterm - SQL Management Studio (GUI)")
	w.Resize(fyne.NewSize(1200, 780))

	guiApp := &App{
		FyneApp:       a,
		Window:        w,
		Config:        cfg,
		ConfigPath:    cfgPath,
		ActiveProfile: initialProfile,
		Driver:        initialDriver,
	}

	guiApp.initUI()
	guiApp.setupKeybindings()

	return guiApp
}

func (a *App) initUI() {
	// 1. Initialize Sub-Controllers
	a.Explorer = NewExplorerController(
		a.Config,
		a.ActiveProfile,
		a.Driver,
		func(sql string) {
			a.Editor.InsertSQL(sql)
		},
		func(path, content string) {
			a.Editor.OpenFile(path, content)
		},
	)

	a.Editor = NewEditorController()
	a.Results = NewResultsController()

	// 2. Toolbar components
	a.execBtn = widget.NewButtonWithIcon("Execute (F5)", theme.MediaPlayIcon(), a.ExecuteCurrentQuery)
	a.execBtn.Importance = widget.HighImportance

	a.stopBtn = widget.NewButtonWithIcon("Cancel", theme.MediaStopIcon(), a.CancelExecution)
	a.stopBtn.Disable()

	connBtn := widget.NewButtonWithIcon("Connect (Ctrl+O)", theme.StorageIcon(), a.ShowConnectionDialog)

	newQueryBtn := widget.NewButtonWithIcon("New Query (Ctrl+T)", theme.ContentAddIcon(), func() {
		a.Editor.NewTab("", "")
	})

	saveBtn := widget.NewButtonWithIcon("Save (Ctrl+S)", theme.DocumentSaveIcon(), func() {
		savedPath, err := a.Editor.SaveActiveTab("")
		if err != nil {
			dialog.ShowError(err, a.Window)
		} else {
			a.SetStatus(fmt.Sprintf("✓ Saved to %s", savedPath))
		}
	})

	aiBtn := widget.NewButtonWithIcon("AI Assistant (Ctrl+K)", theme.HelpIcon(), a.ShowAIDialog)
	aiBtn.Importance = widget.MediumImportance

	exportBtn := widget.NewButtonWithIcon("Export (Ctrl+E)", theme.DownloadIcon(), a.ShowExportDialog)

	serverName := "Not Connected"
	if a.ActiveProfile != nil {
		serverName = fmt.Sprintf("%s (%s)", a.ActiveProfile.Name, a.ActiveProfile.Host)
	}
	a.serverBadge = widget.NewButtonWithIcon(serverName, theme.StorageIcon(), a.ShowConnectionDialog)

	a.dbSelect = widget.NewSelect([]string{"master"}, func(selected string) {
		if a.Driver != nil && selected != "" {
			_ = a.Driver.SwitchDatabase(context.Background(), selected)
			a.Explorer.Refresh()
			a.SetStatus(fmt.Sprintf("Switched database to '%s'", selected))
		}
	})
	if a.ActiveProfile != nil && a.ActiveProfile.Database != "" {
		a.dbSelect.SetOptions([]string{a.ActiveProfile.Database})
		a.dbSelect.SetSelected(a.ActiveProfile.Database)
	}

	toolbar := container.NewHBox(
		connBtn,
		widget.NewSeparator(),
		a.execBtn,
		a.stopBtn,
		widget.NewSeparator(),
		newQueryBtn,
		saveBtn,
		widget.NewSeparator(),
		aiBtn,
		exportBtn,
		widget.NewSeparator(),
		a.serverBadge,
		a.dbSelect,
	)

	// 3. Status Bar
	a.statusLabel = widget.NewLabel("Ready.")
	a.progress = widget.NewProgressBarInfinite()
	a.progress.Hide()

	statusBar := container.NewBorder(
		nil,
		nil,
		a.statusLabel,
		a.progress,
		widget.NewLabel(""),
	)

	// 4. Center Editor & Results Split Pane
	editorResultsSplit := container.NewVSplit(
		a.Editor.Container,
		a.Results.Container,
	)
	editorResultsSplit.SetOffset(0.55) // 55% editor, 45% results

	// 5. Main Horizontal Split (Explorer on Left, Work area on Right)
	mainSplit := container.NewHSplit(
		a.Explorer.Container,
		editorResultsSplit,
	)
	mainSplit.SetOffset(0.24) // 24% sidebar, 76% main

	// 6. Root Layout
	root := container.NewBorder(
		toolbar,
		statusBar,
		nil,
		nil,
		mainSplit,
	)

	a.Window.SetContent(root)
}

func (a *App) setupKeybindings() {
	if desk, ok := a.Window.Canvas().(desktop.Canvas); ok {
		desk.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			switch ev.Name {
			case fyne.KeyF5:
				a.ExecuteCurrentQuery()
			case fyne.KeyF4:
				a.ShowAIDialog()
			}
		})
	}
}

func (a *App) ExecuteCurrentQuery() {
	if a.isExecuting {
		return
	}

	sql := strings.TrimSpace(a.Editor.GetActiveQuery())
	if sql == "" {
		a.SetStatus("Cannot execute empty query.")
		return
	}

	if a.Driver == nil {
		a.ShowConnectionDialog()
		return
	}

	a.isExecuting = true
	a.execBtn.Disable()
	a.stopBtn.Enable()
	a.progress.Show()
	a.SetStatus("Executing query on server...")

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	a.cancelExec = cancel

	startTime := time.Now()
	go func() {
		res, err := a.Driver.ExecuteQuery(ctx, sql)
		duration := time.Since(startTime)

		a.isExecuting = false
		a.cancelExec = nil

		// Update UI on main thread
		fyne.Do(func() {
			a.execBtn.Enable()
			a.stopBtn.Disable()
			a.progress.Hide()

			if err != nil {
				a.lastError = err.Error()
				a.Results.SetError(err, duration)
				a.SetStatus(fmt.Sprintf("✗ Query failed in %v: %v", duration.Round(time.Millisecond), err))
			} else {
				a.lastError = ""
				a.Results.SetResult(res, duration)
				a.SetStatus(fmt.Sprintf("✓ Query executed successfully (%d rows affected) in %v", len(res.Rows), duration.Round(time.Millisecond)))
			}
		})
	}()
}

func (a *App) CancelExecution() {
	if a.cancelExec != nil {
		a.cancelExec()
		a.cancelExec = nil
		a.isExecuting = false
		a.execBtn.Enable()
		a.stopBtn.Disable()
		a.progress.Hide()
		a.SetStatus("Query execution cancelled.")
	}
}

func (a *App) ShowConnectionDialog() {
	dialogs.ShowConnectionDialog(
		a.Window,
		a.Config,
		a.ConfigPath,
		func(profile config.ConnectionProfile) {
			a.ConnectToProfile(profile)
		},
		func() {
			a.Explorer.Refresh()
		},
	)
}

func (a *App) ConnectToProfile(profile config.ConnectionProfile) {
	a.SetStatus(fmt.Sprintf("Connecting to %s...", profile.Name))
	go func() {
		drv, err := db.NewDriver(&profile)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, a.Window)
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := drv.Connect(ctx, &profile); err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, a.Window)
			})
			return
		}

		if a.Driver != nil {
			_ = a.Driver.Close()
		}

		a.Driver = drv
		cp := profile
		a.ActiveProfile = &cp

		// Fetch database list
		dbs, _ := drv.FetchDatabases(ctx)
		dbNames := dbs
		if len(dbNames) == 0 && profile.Database != "" {
			dbNames = []string{profile.Database}
		}

		fyne.Do(func() {
			a.serverBadge.SetText(fmt.Sprintf("%s (%s)", profile.Name, profile.Host))
			if len(dbNames) > 0 {
				a.dbSelect.SetOptions(dbNames)
				if profile.Database != "" {
					a.dbSelect.SetSelected(profile.Database)
				} else {
					a.dbSelect.SetSelected(dbNames[0])
				}
			}
			a.Explorer.SetConnection(a.ActiveProfile, a.Driver)
			a.SetStatus(fmt.Sprintf("✓ Connected to '%s' (%s)", profile.Name, profile.Driver))
		})
	}()
}

func (a *App) ShowAIDialog() {
	aiCfg := config.AIConfig{Enabled: true}
	if a.Config != nil {
		aiCfg = a.Config.AI
		if !aiCfg.Enabled {
			aiCfg.Enabled = true
		}
	}

	dialogs.ShowAIDialog(
		a.Window,
		a.Driver,
		aiCfg,
		a.Editor.GetActiveQuery(),
		a.lastError,
		func(sql string, newTab bool) {
			if newTab {
				a.Editor.NewTab("", sql)
			} else {
				a.Editor.InsertSQL(sql)
			}
			a.SetStatus("✓ SQL Query inserted from AI Assistant")
		},
	)
}

func (a *App) ShowExportDialog() {
	cols, rows := a.Results.GetCurrentData()
	if len(cols) == 0 || len(rows) == 0 {
		dialog.ShowInformation("Export", "No query results to export. Execute a query (F5) first.", a.Window)
		return
	}

	dialogs.ShowExportDialog(
		a.Window,
		cols,
		rows,
		func(path string) {
			a.SetStatus(fmt.Sprintf("✓ Exported results to %s", path))
		},
	)
}

func (a *App) SetStatus(text string) {
	a.statusLabel.SetText(text)
}

func (a *App) Run() {
	a.Window.ShowAndRun()
}
