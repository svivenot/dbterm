package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/db"
	"dbterm/internal/ui/theme"
)

type ActiveTab int

const (
	TabResults ActiveTab = iota
	TabMessages
)

// OpenExportModalMsg requests the app to show the interactive export modal
type OpenExportModalMsg struct{}

type Model struct {
	Result         *db.QueryResult
	ActiveTab      ActiveTab
	SelectedRow    int
	SelectedCol    int
	ScrollRow      int
	ScrollCol      int
	Width          int
	Height         int
	Focused        bool
	InspectorOpen  bool
	InspectorText  string
	StatusMessage  string
	StatusMsgTimer time.Time
	SortCol        int
	SortAsc        bool
	Filter         string
	Filtering      bool
	FilteredRows   [][]string
}

func New() Model {
	return Model{
		ActiveTab:   TabResults,
		SelectedRow: 0,
		SelectedCol: 0,
		ScrollRow:   0,
		ScrollCol:   0,
		SortCol:     -1,
		SortAsc:     true,
		Focused:     false,
	}
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *Model) SetResult(res *db.QueryResult) {
	m.Result = res
	m.SelectedRow = 0
	m.SelectedCol = 0
	m.ScrollRow = 0
	m.ScrollCol = 0
	m.SortCol = -1
	m.SortAsc = true
	m.Filter = ""
	m.Filtering = false

	if res != nil {
		m.FilteredRows = res.Rows
	} else {
		m.FilteredRows = nil
	}

	if res != nil && res.Error != nil {
		m.ActiveTab = TabMessages
	} else {
		m.ActiveTab = TabResults
	}
}

func (m *Model) Focus() {
	m.Focused = true
}

func (m *Model) Blur() {
	m.Focused = false
	m.InspectorOpen = false
	m.Filtering = false
}

func (m *Model) applyFilterAndSort() {
	if m.Result == nil {
		m.FilteredRows = nil
		return
	}

	// 1. Filter
	var rows [][]string
	if m.Filter == "" {
		rows = make([][]string, len(m.Result.Rows))
		copy(rows, m.Result.Rows)
	} else {
		term := strings.ToLower(m.Filter)
		for _, row := range m.Result.Rows {
			match := false
			for _, cell := range row {
				if strings.Contains(strings.ToLower(cell), term) {
					match = true
					break
				}
			}
			if match {
				rows = append(rows, row)
			}
		}
	}

	// 2. Sort
	if m.SortCol >= 0 && m.SortCol < len(m.Result.Columns) {
		col := m.SortCol
		asc := m.SortAsc
		sort.SliceStable(rows, func(i, j int) bool {
			valI := rows[i][col]
			valJ := rows[j][col]
			if asc {
				return valI < valJ
			}
			return valI > valJ
		})
	}

	m.FilteredRows = rows
	if m.SelectedRow >= len(m.FilteredRows) {
		m.SelectedRow = len(m.FilteredRows) - 1
	}
	if m.SelectedRow < 0 {
		m.SelectedRow = 0
	}
}

func (m *Model) ExportCSV() (string, error) {
	if m.Result == nil || len(m.FilteredRows) == 0 {
		return "", fmt.Errorf("no data to export")
	}

	filename := fmt.Sprintf("query_export_%s.csv", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(m.Result.Columns); err != nil {
		return "", err
	}

	for _, row := range m.FilteredRows {
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	absPath, _ := filepath.Abs(filename)
	return absPath, nil
}

func (m *Model) ExportJSON() (string, error) {
	if m.Result == nil || len(m.FilteredRows) == 0 {
		return "", fmt.Errorf("no data to export")
	}

	filename := fmt.Sprintf("query_export_%s.json", time.Now().Format("20060102_150405"))

	var records []map[string]any
	for _, row := range m.FilteredRows {
		rec := make(map[string]any)
		for c, colName := range m.Result.Columns {
			if c < len(row) {
				rec[colName] = row[c]
			}
		}
		records = append(records, rec)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", err
	}

	absPath, _ := filepath.Abs(filename)
	return absPath, nil
}

func (m *Model) HandleMouse(msg tea.MouseMsg, relX, relY int) (Model, tea.Cmd) {
	if m.InspectorOpen {
		if msg.Action == tea.MouseActionPress {
			m.InspectorOpen = false
		}
		return *m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.ActiveTab == TabResults && len(m.FilteredRows) > 0 {
			m.SelectedRow -= 3
			if m.SelectedRow < 0 {
				m.SelectedRow = 0
			}
			if m.SelectedRow < m.ScrollRow {
				m.ScrollRow = m.SelectedRow
			}
		}
		return *m, nil

	case tea.MouseButtonWheelDown:
		if m.ActiveTab == TabResults && len(m.FilteredRows) > 0 {
			m.SelectedRow += 3
			if m.SelectedRow >= len(m.FilteredRows) {
				m.SelectedRow = len(m.FilteredRows) - 1
			}
			maxVisible := m.Height - 5
			if maxVisible > 0 && m.SelectedRow >= m.ScrollRow+maxVisible {
				m.ScrollRow = m.SelectedRow - maxVisible + 1
			}
		}
		return *m, nil

	case tea.MouseButtonWheelLeft:
		if m.ActiveTab == TabResults && m.Result != nil {
			if m.SelectedCol > 0 {
				m.SelectedCol--
				if m.SelectedCol < m.ScrollCol {
					m.ScrollCol = m.SelectedCol
				}
			}
		}
		return *m, nil

	case tea.MouseButtonWheelRight:
		if m.ActiveTab == TabResults && m.Result != nil {
			if m.SelectedCol < len(m.Result.Columns)-1 {
				m.SelectedCol++
			}
		}
		return *m, nil

	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			if relY == 0 {
				// Clicked on Tab Headers: Results (left) vs Messages (right)
				if relX < 22 {
					m.ActiveTab = TabResults
				} else {
					m.ActiveTab = TabMessages
				}
				return *m, nil
			}

			if m.ActiveTab == TabResults && m.Result != nil {
				// Row number column width
				rowNumWidth := len(fmt.Sprintf("%d", len(m.FilteredRows))) + 1
				if rowNumWidth < 3 {
					rowNumWidth = 3
				}

				// Find which visible column was clicked
				colWidths := make([]int, len(m.Result.Columns))
				for i, col := range m.Result.Columns {
					colWidths[i] = len(col)
				}
				for r := 0; r < 50 && r < len(m.FilteredRows); r++ {
					for c, val := range m.FilteredRows[r] {
						if len(val) > colWidths[c] {
							colWidths[c] = len(val)
						}
					}
				}
				for i := range colWidths {
					if colWidths[i] > 30 {
						colWidths[i] = 30
					}
					if colWidths[i] < 6 {
						colWidths[i] = 6
					}
				}

				currX := rowNumWidth
				clickedCol := m.ScrollCol
				for c := m.ScrollCol; c < len(m.Result.Columns); c++ {
					cw := colWidths[c] + 2
					if relX >= currX && relX < currX+cw {
						clickedCol = c
						break
					}
					currX += cw
				}

				if relY == 1 {
					// Clicked Header -> Sort by column
					if clickedCol >= 0 && clickedCol < len(m.Result.Columns) {
						if m.SortCol == clickedCol {
							m.SortAsc = !m.SortAsc
						} else {
							m.SortCol = clickedCol
							m.SortAsc = true
						}
						m.applyFilterAndSort()
					}
					return *m, nil
				}

				if relY >= 2 {
					// Clicked a Data Row -> Select row & col
					targetRow := m.ScrollRow + (relY - 2)
					if targetRow >= 0 && targetRow < len(m.FilteredRows) {
						m.SelectedRow = targetRow
						m.SelectedCol = clickedCol
					}
					return *m, nil
				}
			}
		}
	}

	return *m, nil
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.HandleMouse(msg, msg.X, msg.Y)
	case tea.KeyMsg:
		if m.InspectorOpen {
			switch msg.String() {
			case "esc", "enter", "q":
				m.InspectorOpen = false
				return *m, nil
			}
			return *m, nil
		}

		if m.Filtering {
			switch msg.String() {
			case "enter", "esc":
				m.Filtering = false
				return *m, nil
			case "backspace":
				if len(m.Filter) > 0 {
					m.Filter = m.Filter[:len(m.Filter)-1]
					m.applyFilterAndSort()
				}
				return *m, nil
			default:
				if len(msg.String()) == 1 {
					m.Filter += msg.String()
					m.applyFilterAndSort()
				}
				return *m, nil
			}
		}

		switch msg.String() {
		case "ctrl+r":
			m.ActiveTab = TabResults
			return *m, nil
		case "ctrl+m":
			m.ActiveTab = TabMessages
			return *m, nil
		case "up", "k":
			if m.ActiveTab == TabResults && len(m.FilteredRows) > 0 {
				if m.SelectedRow > 0 {
					m.SelectedRow--
					if m.SelectedRow < m.ScrollRow {
						m.ScrollRow = m.SelectedRow
					}
				}
			}
			return *m, nil
		case "down", "j":
			if m.ActiveTab == TabResults && len(m.FilteredRows) > 0 {
				if m.SelectedRow < len(m.FilteredRows)-1 {
					m.SelectedRow++
					maxVisible := m.Height - 5
					if maxVisible > 0 && m.SelectedRow >= m.ScrollRow+maxVisible {
						m.ScrollRow = m.SelectedRow - maxVisible + 1
					}
				}
			}
			return *m, nil
		case "left", "h":
			if m.ActiveTab == TabResults && m.Result != nil {
				if m.SelectedCol > 0 {
					m.SelectedCol--
					if m.SelectedCol < m.ScrollCol {
						m.ScrollCol = m.SelectedCol
					}
				}
			}
			return *m, nil
		case "right", "l":
			if m.ActiveTab == TabResults && m.Result != nil {
				if m.SelectedCol < len(m.Result.Columns)-1 {
					m.SelectedCol++
				}
			}
			return *m, nil
		case "home", "^":
			if m.ActiveTab == TabResults && m.Result != nil {
				m.SelectedCol = 0
				m.ScrollCol = 0
			}
			return *m, nil
		case "end", "$":
			if m.ActiveTab == TabResults && m.Result != nil && len(m.Result.Columns) > 0 {
				m.SelectedCol = len(m.Result.Columns) - 1
			}
			return *m, nil
		case "enter", "v":
			// Open Cell Inspector
			if m.ActiveTab == TabResults && m.Result != nil && len(m.FilteredRows) > 0 {
				if m.SelectedRow < len(m.FilteredRows) && m.SelectedCol < len(m.Result.Columns) {
					cellVal := m.FilteredRows[m.SelectedRow][m.SelectedCol]
					colName := m.Result.Columns[m.SelectedCol]

					// Attempt to pretty-print JSON if applicable
					formattedVal := cellVal
					if strings.HasPrefix(strings.TrimSpace(cellVal), "{") || strings.HasPrefix(strings.TrimSpace(cellVal), "[") {
						var parsed any
						if err := json.Unmarshal([]byte(cellVal), &parsed); err == nil {
							if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
								formattedVal = string(pretty)
							}
						}
					}

					m.InspectorText = fmt.Sprintf("Column: %s (Row %d)\n\n%s", colName, m.SelectedRow+1, formattedVal)
					m.InspectorOpen = true
				}
			}
			return *m, nil
		case "o": // Sort by current column
			if m.ActiveTab == TabResults && m.Result != nil && len(m.Result.Columns) > 0 {
				if m.SortCol == m.SelectedCol {
					m.SortAsc = !m.SortAsc
				} else {
					m.SortCol = m.SelectedCol
					m.SortAsc = true
				}
				m.applyFilterAndSort()
			}
			return *m, nil
		case "/": // Filter rows
			if m.ActiveTab == TabResults {
				m.Filtering = true
				m.Filter = ""
				m.applyFilterAndSort()
			}
			return *m, nil
		case "ctrl+s", "e": // Open Export Modal
			if m.Result != nil && len(m.FilteredRows) > 0 {
				return *m, func() tea.Msg {
					return OpenExportModalMsg{}
				}
			}
			return *m, nil
		}
	}

	return *m, nil
}

func (m Model) View() string {
	if m.InspectorOpen {
		return m.renderInspector()
	}

	var b strings.Builder

	// Header Tabs
	resultsTitle := " Results "
	if m.Result != nil && len(m.Result.Rows) > 0 {
		if len(m.FilteredRows) != len(m.Result.Rows) {
			resultsTitle = fmt.Sprintf(" Results (%d/%d rows) ", len(m.FilteredRows), len(m.Result.Rows))
		} else {
			resultsTitle = fmt.Sprintf(" Results (%d rows) ", len(m.Result.Rows))
		}
	}
	messagesTitle := " Messages "

	var tabHeaders []string
	if m.ActiveTab == TabResults {
		tabHeaders = append(tabHeaders, theme.TabActive.Render(resultsTitle))
		tabHeaders = append(tabHeaders, theme.TabInactive.Render(messagesTitle))
	} else {
		tabHeaders = append(tabHeaders, theme.TabInactive.Render(resultsTitle))
		tabHeaders = append(tabHeaders, theme.TabActive.Render(messagesTitle))
	}

	headerBar := lipgloss.JoinHorizontal(lipgloss.Top, tabHeaders...)

	// Filter badge if filtering
	if m.Filtering {
		headerBar += "  " + theme.TopBarBadge.Render(fmt.Sprintf("Filter: %s_", m.Filter))
	} else if m.Filter != "" {
		headerBar += "  " + theme.TopBarBadge.Render(fmt.Sprintf("Filter: [%s]", m.Filter))
	}

	if m.StatusMessage != "" && time.Since(m.StatusMsgTimer) < 5*time.Second {
		headerBar += "  " + theme.TopBarBadge.Render(m.StatusMessage)
	}

	b.WriteString(headerBar)
	b.WriteString("\n")

	if m.ActiveTab == TabMessages {
		b.WriteString(m.renderMessages())
	} else {
		b.WriteString(m.renderGrid())
	}

	return lipgloss.NewStyle().Width(m.Width).Height(m.Height).Render(b.String())
}

func (m Model) renderGrid() string {
	if m.Result == nil {
		return theme.StyleFgMuted.Render("\n  No query executed yet. Press F5 or Ctrl+E to run.")
	}

	if len(m.Result.Columns) == 0 {
		return theme.StyleFgMuted.Render("\n  Query executed successfully. (0 result columns)")
	}

	if len(m.FilteredRows) == 0 {
		return theme.StyleFgMuted.Render("\n  (0 rows returned)")
	}

	// 1. Calculate column widths
	colWidths := make([]int, len(m.Result.Columns))
	for i, col := range m.Result.Columns {
		colWidths[i] = len(col)
	}

	sampleLimit := 100
	if len(m.FilteredRows) < sampleLimit {
		sampleLimit = len(m.FilteredRows)
	}

	for r := 0; r < sampleLimit; r++ {
		row := m.FilteredRows[r]
		for c, val := range row {
			if len(val) > colWidths[c] {
				colWidths[c] = len(val)
			}
		}
	}

	for i := range colWidths {
		if colWidths[i] > 30 {
			colWidths[i] = 30
		}
		if colWidths[i] < 6 {
			colWidths[i] = 6
		}
	}

	// 2. Row number column width
	rowNumWidth := len(fmt.Sprintf("%d", len(m.FilteredRows))) + 1
	if rowNumWidth < 3 {
		rowNumWidth = 3
	}

	// 3. Compute available width for horizontal scrolling of columns
	availWidth := m.Width - 4 - rowNumWidth
	if availWidth < 10 {
		availWidth = 10
	}

	// Compute visible column slice [startCol, endCol] based on m.ScrollCol and m.SelectedCol
	scrollCol := m.ScrollCol
	if m.SelectedCol < scrollCol {
		scrollCol = m.SelectedCol
	}

	computeVisibleCols := func(start int) []int {
		var cols []int
		used := 0
		for c := start; c < len(m.Result.Columns); c++ {
			cw := colWidths[c] + 2
			if used+cw > availWidth && len(cols) > 0 {
				break
			}
			cols = append(cols, c)
			used += cw
		}
		return cols
	}

	visibleCols := computeVisibleCols(scrollCol)

	// Scroll right if SelectedCol is to the right of the visible window
	for len(visibleCols) > 0 && m.SelectedCol > visibleCols[len(visibleCols)-1] {
		scrollCol++
		visibleCols = computeVisibleCols(scrollCol)
	}

	if len(visibleCols) == 0 && len(m.Result.Columns) > 0 {
		visibleCols = []int{m.SelectedCol}
	}

	m.ScrollCol = scrollCol

	var gridBuilder strings.Builder

	// Render column scroll indicators info if some columns are hidden
	hasLeftHidden := scrollCol > 0
	hasRightHidden := len(visibleCols) > 0 && visibleCols[len(visibleCols)-1] < len(m.Result.Columns)-1

	// Header row
	var headerCells []string
	headerCells = append(headerCells, theme.TableHeader.Width(rowNumWidth).Render("#"))

	for _, c := range visibleCols {
		displayName := m.Result.Columns[c]
		if c == m.SortCol {
			if m.SortAsc {
				displayName += " ▲"
			} else {
				displayName += " ▼"
			}
		}
		headerCells = append(headerCells, theme.TableHeader.Width(colWidths[c]+2).Render(displayName))
	}

	if hasRightHidden {
		moreCount := len(m.Result.Columns) - 1 - visibleCols[len(visibleCols)-1]
		headerCells = append(headerCells, theme.TableHeader.Render(fmt.Sprintf(" ▶ (+%d)", moreCount)))
	} else if hasLeftHidden {
		headerCells = append(headerCells, theme.TableHeader.Render(" ◀"))
	}

	gridBuilder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))
	gridBuilder.WriteString("\n")

	// Render visible rows (single-line guaranteed)
	maxVisibleRows := m.Height - 4
	if maxVisibleRows < 3 {
		maxVisibleRows = 3
	}

	startRow := m.ScrollRow
	endRow := startRow + maxVisibleRows
	if endRow > len(m.FilteredRows) {
		endRow = len(m.FilteredRows)
	}

	for r := startRow; r < endRow; r++ {
		var rowCells []string
		isRowSel := (r == m.SelectedRow)

		rowNumStyle := theme.TableRowNum.Width(rowNumWidth)
		if isRowSel {
			rowNumStyle = theme.TableCellSelected.Width(rowNumWidth)
		}
		rowCells = append(rowCells, rowNumStyle.Render(fmt.Sprintf("%d", r+1)))

		row := m.FilteredRows[r]
		for _, c := range visibleCols {
			val := ""
			if c < len(row) {
				val = row[c]
			}
			isCellSel := (r == m.SelectedRow && c == m.SelectedCol)
			cellWidth := colWidths[c] + 2

			displayVal := val
			if len(displayVal) > colWidths[c] {
				displayVal = displayVal[:colWidths[c]-1] + "…"
			}

			var cellStyle lipgloss.Style
			if isCellSel {
				cellStyle = theme.TableCellSelected.Width(cellWidth)
			} else if val == "NULL" {
				cellStyle = theme.TableNull.Width(cellWidth)
			} else {
				cellStyle = theme.TableCell.Width(cellWidth)
			}
			rowCells = append(rowCells, cellStyle.Render(displayVal))
		}

		if hasRightHidden {
			rowCells = append(rowCells, theme.TableCell.Render(" …"))
		}

		gridBuilder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
		gridBuilder.WriteString("\n")
	}

	return gridBuilder.String()
}

func (m Model) renderMessages() string {
	if m.Result == nil {
		return theme.StyleFgMuted.Render("\n  No execution messages.")
	}

	var b strings.Builder
	b.WriteString("\n")

	if m.Result.Error != nil {
		b.WriteString(theme.StatusBadgeError.Render(" ERROR ") + " " + theme.StyleError.Render(m.Result.Error.Error()))
		b.WriteString("\n\n")
	}

	for _, msg := range m.Result.Messages {
		if strings.HasPrefix(msg, "Msg:") || strings.HasPrefix(msg, "Error:") {
			b.WriteString(theme.StyleError.Render(msg) + "\n")
		} else {
			b.WriteString(theme.StyleFgLight.Render(msg) + "\n")
		}
	}

	return b.String()
}

func (m Model) renderInspector() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(m.Width - 6).
		Height(m.Height - 4)

	content := fmt.Sprintf("%s\n\n%s\n\n%s",
		theme.ModalTitle.Render("CELL VALUE INSPECTOR"),
		m.InspectorText,
		theme.StyleFgMuted.Render("[Esc/Enter: Close]"),
	)

	return box.Render(content)
}
