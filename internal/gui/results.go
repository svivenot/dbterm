package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/db"
)

type ResultsController struct {
	Container      *fyne.Container
	tabs           *container.AppTabs
	table          *widget.Table
	messagesEntry  *widget.Entry
	columns        []string
	rows           [][]string
	activeResult   *db.QueryResult
	statusLabel    *widget.Label
}

func NewResultsController() *ResultsController {
	rc := &ResultsController{
		messagesEntry: widget.NewMultiLineEntry(),
		statusLabel:   widget.NewLabel("Ready."),
	}
	rc.messagesEntry.TextStyle = fyne.TextStyle{Monospace: true}
	rc.messagesEntry.Wrapping = fyne.TextWrapWord
	rc.messagesEntry.SetText("Execute a query (F5) to view results here.\n")

	rc.initTable()

	tableContainer := container.NewBorder(nil, nil, nil, nil, rc.table)

	rc.tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("Results (Grid)", theme.ListIcon(), tableContainer),
		container.NewTabItemWithIcon("Messages", theme.InfoIcon(), rc.messagesEntry),
	)

	rc.Container = container.NewBorder(nil, nil, nil, nil, rc.tabs)
	return rc
}

func (rc *ResultsController) initTable() {
	rc.table = widget.NewTable(
		func() (int, int) {
			if len(rc.columns) == 0 {
				return 0, 0
			}
			return len(rc.rows) + 1, len(rc.columns) + 1
		},
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("Cell Value Placeholder")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(id widget.TableCellID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			if id.Row == 0 {
				// Header row
				if id.Col == 0 {
					label.TextStyle = fyne.TextStyle{Bold: true}
					label.SetText("#")
				} else if id.Col-1 < len(rc.columns) {
					label.TextStyle = fyne.TextStyle{Bold: true}
					label.SetText(rc.columns[id.Col-1])
				}
			} else {
				// Data row
				rowIdx := id.Row - 1
				if id.Col == 0 {
					label.TextStyle = fyne.TextStyle{Monospace: true}
					label.SetText(fmt.Sprintf("%d", id.Row))
				} else if rowIdx < len(rc.rows) && id.Col-1 < len(rc.columns) {
					label.TextStyle = fyne.TextStyle{}
					val := rc.rows[rowIdx][id.Col-1]
					if val == "" {
						label.SetText("NULL")
					} else {
						label.SetText(val)
					}
				}
			}
		},
	)

	// Set column widths
	rc.table.SetColumnWidth(0, 50)
	for i := 1; i < 20; i++ {
		rc.table.SetColumnWidth(i, 160)
	}
}

func (rc *ResultsController) SetResult(res *db.QueryResult, duration time.Duration) {
	rc.activeResult = res
	if res == nil {
		rc.columns = nil
		rc.rows = nil
		rc.table.Refresh()
		return
	}

	rc.columns = res.Columns
	rc.rows = res.Rows

	// Adjust column widths based on content
	rc.table.SetColumnWidth(0, 50)
	for cIdx, colName := range res.Columns {
		maxLen := len(colName)
		sampleRows := 30
		if len(res.Rows) < sampleRows {
			sampleRows = len(res.Rows)
		}
		for r := 0; r < sampleRows; r++ {
			if cIdx < len(res.Rows[r]) && len(res.Rows[r][cIdx]) > maxLen {
				maxLen = len(res.Rows[r][cIdx])
			}
		}
		w := float32(maxLen*9 + 30)
		if w < 100 {
			w = 100
		}
		if w > 400 {
			w = 400
		}
		rc.table.SetColumnWidth(cIdx+1, w)
	}

	rc.table.Refresh()
	rc.tabs.SelectIndex(0)

	msg := fmt.Sprintf("(%d rows affected)\nCompletion time: %s\nExecution time: %v\n", len(res.Rows), time.Now().Format("2006-01-02 15:04:05"), duration.Round(time.Millisecond))
	rc.messagesEntry.SetText(msg)
}

func (rc *ResultsController) SetError(err error, duration time.Duration) {
	rc.columns = nil
	rc.rows = nil
	rc.table.Refresh()
	rc.tabs.SelectIndex(1)

	msg := fmt.Sprintf("Msg 50000, Level 16, State 1, Line 1\n%v\n\nExecution time: %v\n", err, duration.Round(time.Millisecond))
	rc.messagesEntry.SetText(msg)
}

func (rc *ResultsController) GetCurrentData() ([]string, [][]string) {
	return rc.columns, rc.rows
}
