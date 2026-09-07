package dialogs

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/lsp"
)

// ShowCompletionDialog opens an autocompletion and schema suggestion dialog
func ShowCompletionDialog(window fyne.Window, lspClient *lsp.Client, currentSQL string, onInsert func(text string)) {
	if lspClient == nil {
		return
	}

	items := lspClient.GetCompletions("file://gui_query.sql", currentSQL, 0, len(currentSQL))
	if len(items) == 0 {
		dialog.ShowInformation("Autocomplete", "No suggestions available for current query context.", window)
		return
	}

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Type to filter tables, columns, keywords, functions...")

	filteredItems := items

	var list *widget.List
	docLabel := widget.NewLabel("Select an item to view documentation and details.")
	docLabel.Wrapping = fyne.TextWrapWord

	list = widget.NewList(
		func() int {
			return len(filteredItems)
		},
		func() fyne.CanvasObject {
			badge := widget.NewLabel("[SQL]")
			badge.TextStyle = fyne.TextStyle{Bold: true}
			label := widget.NewLabel("Item Name")
			detail := widget.NewLabel("Detail")
			return container.NewHBox(badge, label, widget.NewLabel("-"), detail)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(filteredItems) {
				return
			}
			item := filteredItems[id]
			box := obj.(*fyne.Container)
			badge := box.Objects[0].(*widget.Label)
			label := box.Objects[1].(*widget.Label)
			detail := box.Objects[3].(*widget.Label)

			switch item.Kind {
			case lsp.CompletionItemKindClass:
				badge.SetText("[TBL]")
			case lsp.CompletionItemKindInterface:
				badge.SetText("[VIEW]")
			case lsp.CompletionItemKindField:
				badge.SetText("[COL]")
			case lsp.CompletionItemKindKeyword:
				badge.SetText("[KWD]")
			case lsp.CompletionItemKindFunction:
				badge.SetText("[FUNC]")
			case lsp.CompletionItemKindModule:
				badge.SetText("[SCH]")
			case lsp.CompletionItemKindSnippet:
				badge.SetText("[SNP]")
			default:
				badge.SetText("[SQL]")
			}

			label.SetText(item.Label)
			detail.SetText(item.Detail)
		},
	)

	filterItems := func(query string) {
		q := strings.ToLower(strings.TrimSpace(query))
		if q == "" {
			filteredItems = items
		} else {
			var res []lsp.CompletionItem
			for _, it := range items {
				if strings.Contains(strings.ToLower(it.Label), q) || strings.Contains(strings.ToLower(it.Detail), q) {
					res = append(res, it)
				}
			}
			filteredItems = res
		}
		list.Refresh()
		if len(filteredItems) > 0 {
			list.Select(0)
		} else {
			docLabel.SetText("No matching suggestions.")
		}
	}

	searchEntry.OnChanged = filterItems

	selectedItem := -1
	list.OnSelected = func(id widget.ListItemID) {
		if id < len(filteredItems) {
			selectedItem = id
			it := filteredItems[id]
			docText := fmt.Sprintf("Symbol: %s\nDetail: %s\n", it.Label, it.Detail)
			if it.Documentation != nil {
				docText += "\n" + it.Documentation.Value
			}
			docLabel.SetText(docText)
		}
	}

	var customDialog dialog.Dialog

	insertAction := func() {
		if selectedItem >= 0 && selectedItem < len(filteredItems) {
			it := filteredItems[selectedItem]
			val := it.InsertText
			if val == "" {
				val = it.Label
			}
			onInsert(val)
			if customDialog != nil {
				customDialog.Hide()
			}
		}
	}

	insertBtn := widget.NewButtonWithIcon("Insert Selected", theme.ContentAddIcon(), insertAction)
	insertBtn.Importance = widget.HighImportance

	closeBtn := widget.NewButton("Cancel", func() {
		if customDialog != nil {
			customDialog.Hide()
		}
	})

	rightDocPane := container.NewVScroll(docLabel)
	split := container.NewHSplit(list, rightDocPane)
	split.SetOffset(0.55)

	content := container.NewBorder(
		searchEntry,
		container.NewHBox(insertBtn, closeBtn),
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(750, 420), split),
	)

	customDialog = dialog.NewCustomWithoutButtons("SQL Autocomplete & Schema Inspector", content, window)
	customDialog.Resize(fyne.NewSize(800, 500))
	customDialog.Show()

	if len(filteredItems) > 0 {
		list.Select(0)
	}
	window.Canvas().Focus(searchEntry)
}
