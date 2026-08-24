package dialogs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/ai"
	"dbterm/internal/config"
	"dbterm/internal/db"
)

// ShowAIDialog presents the AI SQL Assistant dialog
func ShowAIDialog(
	w fyne.Window,
	driver db.Driver,
	aiCfg config.AIConfig,
	currentSQL string,
	lastError string,
	onApplySQL func(sql string, newTab bool),
) {
	engine := ai.NewEngine(aiCfg)

	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetPlaceHolder("Describe what data you want (e.g. 'Top 5 clients par chiffre d affaires en 2025')...")
	promptEntry.Wrapping = fyne.TextWrapWord
	promptEntry.SetMinRowsVisible(3)

	modeSelect := widget.NewRadioGroup([]string{
		"1: Text-to-SQL",
		"2: Fix Error",
		"3: Explain Query",
		"4: Optimize Query",
	}, nil)
	if lastError != "" {
		modeSelect.SetSelected("2: Fix Error")
		promptEntry.SetText(fmt.Sprintf("Fix database error: %s", lastError))
	} else {
		modeSelect.SetSelected("1: Text-to-SQL")
	}

	statusLabel := widget.NewLabel(fmt.Sprintf("AI Engine: %s", engine.GetModelInfo()))
	statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	generatedEntry := widget.NewMultiLineEntry()
	generatedEntry.SetPlaceHolder("Generated SQL will appear here...")
	generatedEntry.Wrapping = fyne.TextWrapWord
	generatedEntry.SetMinRowsVisible(6)

	explanationLabel := widget.NewLabel("")
	explanationLabel.Wrapping = fyne.TextWrapWord

	progressBar := widget.NewProgressBarInfinite()
	progressBar.Hide()

	var customDialog dialog.Dialog

	generateBtn := widget.NewButtonWithIcon("Generate SQL", theme.MediaPlayIcon(), func() {
		progressBar.Show()
		statusLabel.SetText("Analyzing database schema & generating SQL...")

		go func() {
			var mode ai.AIMode
			switch modeSelect.Selected {
			case "1: Text-to-SQL":
				mode = ai.AIModeGenerate
			case "2: Fix Error":
				mode = ai.AIModeFixError
			case "3: Explain Query":
				mode = ai.AIModeExplain
			case "4: Optimize Query":
				mode = ai.AIModeOptimize
			default:
				mode = ai.AIModeGenerate
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			req := ai.AIRequest{
				Mode:         mode,
				UserPrompt:   promptEntry.Text,
				CurrentSQL:   currentSQL,
				ErrorMessage: lastError,
				Driver:       driver,
			}

			resp, err := engine.Generate(ctx, req)
			fyne.Do(func() {
				progressBar.Hide()
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("Error: %v", err))
					dialog.ShowError(err, w)
					return
				}

				if resp != nil {
					generatedEntry.SetText(resp.GeneratedSQL)
					explanationLabel.SetText(resp.Explanation)
					statusLabel.SetText(fmt.Sprintf("✓ Generated with %s in %v", resp.ModelUsed, resp.Duration.Round(time.Millisecond)))
				}
			})
		}()
	})
	generateBtn.Importance = widget.HighImportance

	insertBtn := widget.NewButtonWithIcon("Insert in Editor", theme.DocumentSaveIcon(), func() {
		sql := strings.TrimSpace(generatedEntry.Text)
		if sql != "" {
			if onApplySQL != nil {
				onApplySQL(sql, false)
			}
			customDialog.Hide()
		}
	})

	newTabBtn := widget.NewButtonWithIcon("Open in New Tab", theme.ContentAddIcon(), func() {
		sql := strings.TrimSpace(generatedEntry.Text)
		if sql != "" {
			if onApplySQL != nil {
				onApplySQL(sql, true)
			}
			customDialog.Hide()
		}
	})

	actionButtons := container.NewHBox(
		generateBtn,
		widget.NewSeparator(),
		insertBtn,
		newTabBtn,
	)

	topBox := container.NewVBox(
		statusLabel,
		modeSelect,
		widget.NewLabel("Prompt:"),
		promptEntry,
		progressBar,
		actionButtons,
		widget.NewSeparator(),
		widget.NewLabel("Generated SQL Query:"),
	)

	content := container.NewBorder(
		topBox,
		explanationLabel,
		nil,
		nil,
		generatedEntry,
	)

	customDialog = dialog.NewCustom("🤖 AI SQL Assistant (Defog SQLCoder / Qwen / Ollama)", "Close", content, w)
	customDialog.Resize(fyne.NewSize(750, 560))
	customDialog.Show()
}
