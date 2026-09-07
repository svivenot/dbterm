package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/lsp"
	"dbterm/internal/ui/syntax"
	"dbterm/internal/ui/theme"
)

// Messages emitted by Editor
type ExecuteQueryMsg struct {
	Query string
}

type EditorToastMsg struct {
	Message string
}

type PromptSaveFileMsg struct {
	Content     string
	DefaultName string
}

type TabInfo struct {
	ID             string
	Title          string
	FilePath       string // Associated local file path
	Textarea       textarea.Model
	Modified       bool
	Selecting      bool
	VisualMode     bool
	SelStartLine   int
	SelStartCol    int
	SelEndLine     int
	SelEndCol      int
	SelectionExist bool
}

type Model struct {
	Tabs               []TabInfo
	ActiveIndex        int
	Width              int
	Height             int
	Focused            bool
	MouseDown          bool
	LSPClient          *lsp.Client
	CompletionActive   bool
	Completions        []lsp.CompletionItem
	SelectedCompletion int
	CompletionLine     int
	CompletionCol      int
}

func New(initialQuery string) Model {
	ta := textarea.New()
	ta.Placeholder = "-- Write SQL query here and press F5 or Ctrl+E to execute..."
	ta.ShowLineNumbers = true
	ta.Focus()
	if initialQuery != "" {
		ta.SetValue(initialQuery)
	} else {
		ta.SetValue("SELECT * FROM sales.Customers;\n\n-- Run custom queries here:\n-- SELECT * FROM inventory.Products;\n-- SELECT * FROM sales.v_OrderSummary;")
	}

	tab := TabInfo{
		ID:       "tab_1",
		Title:    "Query-1.sql",
		Textarea: ta,
		Modified: false,
	}

	return Model{
		Tabs:        []TabInfo{tab},
		ActiveIndex: 0,
		Focused:     true,
	}
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h

	taHeight := h - 3
	if taHeight < 3 {
		taHeight = 3
	}

	for i := range m.Tabs {
		m.Tabs[i].Textarea.SetWidth(w - 4)
		m.Tabs[i].Textarea.SetHeight(taHeight)
	}
}

func (m *Model) Focus() {
	m.Focused = true
	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		m.Tabs[m.ActiveIndex].Textarea.Focus()
	}
}

func (m *Model) Blur() {
	m.Focused = false
	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		m.Tabs[m.ActiveIndex].Textarea.Blur()
	}
}

func (m *Model) NewTab(title, content string) {
	if title == "" {
		title = fmt.Sprintf("Query-%d.sql", len(m.Tabs)+1)
	}
	ta := textarea.New()
	ta.Placeholder = "-- Write SQL query here and press F5 or Ctrl+E to execute..."
	ta.ShowLineNumbers = true
	ta.SetWidth(m.Width - 4)
	taHeight := m.Height - 3
	if taHeight < 3 {
		taHeight = 3
	}
	ta.SetHeight(taHeight)
	if content != "" {
		ta.SetValue(content)
	}
	if m.Focused {
		ta.Focus()
	}

	tab := TabInfo{
		ID:       fmt.Sprintf("tab_%d", len(m.Tabs)+1),
		Title:    title,
		Textarea: ta,
		Modified: false,
	}
	m.Tabs = append(m.Tabs, tab)
	m.ActiveIndex = len(m.Tabs) - 1
}

func (m *Model) CloseActiveTab() {
	if len(m.Tabs) <= 1 {
		m.Tabs[0].Textarea.SetValue("")
		m.Tabs[0].Modified = false
		m.Tabs[0].SelectionExist = false
		m.Tabs[0].VisualMode = false
		return
	}

	m.Tabs = append(m.Tabs[:m.ActiveIndex], m.Tabs[m.ActiveIndex+1:]...)
	if m.ActiveIndex >= len(m.Tabs) {
		m.ActiveIndex = len(m.Tabs) - 1
	}
	if m.Focused {
		m.Tabs[m.ActiveIndex].Textarea.Focus()
	}
}

func (m *Model) OpenFile(filePath, content string) {
	title := filepath.Base(filePath)
	if len(m.Tabs) == 1 && !m.Tabs[0].Modified && m.Tabs[0].FilePath == "" {
		// Reuse initial unmodified tab
		m.Tabs[0].Title = title
		m.Tabs[0].FilePath = filePath
		m.Tabs[0].Textarea.SetValue(content)
		m.Tabs[0].Modified = false
		m.ActiveIndex = 0
		return
	}

	// Check if already open
	for i, t := range m.Tabs {
		if t.FilePath == filePath {
			m.ActiveIndex = i
			m.Tabs[i].Textarea.SetValue(content)
			m.Tabs[i].Modified = false
			return
		}
	}

	m.NewTab(title, content)
	m.Tabs[m.ActiveIndex].FilePath = filePath
	m.Tabs[m.ActiveIndex].Modified = false
}

func (m *Model) SetSavedPath(filePath string) {
	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		m.Tabs[m.ActiveIndex].FilePath = filePath
		m.Tabs[m.ActiveIndex].Title = filepath.Base(filePath)
		m.Tabs[m.ActiveIndex].Modified = false
	}
}

func (m *Model) InsertQuery(query string) {
	if len(m.Tabs) == 0 {
		m.NewTab("", query)
		return
	}
	currentVal := m.Tabs[m.ActiveIndex].Textarea.Value()
	if strings.TrimSpace(currentVal) == "" {
		m.Tabs[m.ActiveIndex].Textarea.SetValue(query)
	} else {
		m.NewTab("", query)
	}
}

// GetCurrentQuery returns selected text if a selection exists (SSMS style), or entire buffer
func (m *Model) GetCurrentQuery() string {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return ""
	}
	tab := &m.Tabs[m.ActiveIndex]
	if tab.SelectionExist {
		sel := m.GetSelectedText()
		if strings.TrimSpace(sel) != "" {
			return sel
		}
	}
	return tab.Textarea.Value()
}

func (m *Model) SetLSPClient(client *lsp.Client) {
	m.LSPClient = client
}

// TriggerCompletion fetches autocompletions from LSP
func (m *Model) TriggerCompletion(explicit bool) {
	if m.LSPClient == nil || len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return
	}
	tab := &m.Tabs[m.ActiveIndex]
	sql := tab.Textarea.Value()
	curLine := tab.Textarea.Line()
	curCol := tab.Textarea.LineInfo().ColumnOffset

	lines := strings.Split(sql, "\n")
	if curLine < len(lines) {
		lineText := lines[curLine]
		if curCol > len(lineText) {
			curCol = len(lineText)
		}
		textBefore := lineText[:curCol]

		if !explicit {
			trimmed := strings.TrimRight(textBefore, " \t")
			isDot := strings.HasSuffix(trimmed, ".")
			// Find word before cursor
			startCol := curCol
			for startCol > 0 {
				r := rune(lineText[startCol-1])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '#' && r != '$' {
					break
				}
				startCol--
			}
			word := lineText[startCol:curCol]
			if !isDot && len(word) < 2 {
				m.CompletionActive = false
				m.Completions = nil
				return
			}
		}
	}

	items := m.LSPClient.GetCompletions("file://current_query.sql", sql, curLine, curCol)
	if len(items) == 0 {
		m.CompletionActive = false
		m.Completions = nil
		return
	}

	m.Completions = items
	m.SelectedCompletion = 0
	m.CompletionActive = true
	m.CompletionLine = curLine
	m.CompletionCol = curCol
}

// AcceptCompletion applies the selected autocompletion item into the active textarea
func (m *Model) AcceptCompletion() bool {
	if !m.CompletionActive || len(m.Completions) == 0 || m.SelectedCompletion >= len(m.Completions) {
		return false
	}
	tab := &m.Tabs[m.ActiveIndex]
	item := m.Completions[m.SelectedCompletion]

	insertVal := item.InsertText
	if insertVal == "" {
		insertVal = item.Label
	}

	// Strip snippet placeholders like ${1:table}
	reSnippet := regexp.MustCompile(`\$\{\d+:([^}]+)\}`)
	insertVal = reSnippet.ReplaceAllString(insertVal, "$1")
	insertVal = strings.ReplaceAll(insertVal, "${1:*}", "*")
	insertVal = strings.ReplaceAll(insertVal, "$1", "")
	insertVal = strings.ReplaceAll(insertVal, "$2", "")
	insertVal = strings.ReplaceAll(insertVal, "$3", "")
	insertVal = strings.ReplaceAll(insertVal, "$4", "")

	lines := strings.Split(tab.Textarea.Value(), "\n")
	curLine := tab.Textarea.Line()
	curCol := tab.Textarea.LineInfo().ColumnOffset

	if curLine < len(lines) {
		lineText := lines[curLine]
		if curCol > len(lineText) {
			curCol = len(lineText)
		}

		// Find word prefix before cursor on current line to replace
		startCol := curCol
		for startCol > 0 {
			r := rune(lineText[startCol-1])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '#' && r != '$' {
				break
			}
			startCol--
		}

		newLine := lineText[:startCol] + insertVal + lineText[curCol:]
		lines[curLine] = newLine
		tab.Textarea.SetValue(strings.Join(lines, "\n"))

		// Move cursor to target column
		targetCol := startCol + len(insertVal)
		tab.Textarea.SetCursor(targetCol)
	}

	m.CompletionActive = false
	m.Completions = nil
	m.SelectedCompletion = 0
	tab.Modified = true
	return true
}

func (m Model) renderCompletionPopup() string {
	if !m.CompletionActive || len(m.Completions) == 0 {
		return ""
	}

	maxVisible := 6
	total := len(m.Completions)
	startIdx := 0
	if m.SelectedCompletion >= maxVisible {
		startIdx = m.SelectedCompletion - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > total {
		endIdx = total
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(theme.ColorSecondary).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#264F78")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	var rows []string
	rows = append(rows, headerStyle.Render("── LSP Suggestions (Tab/Enter ↵) ──"))

	for i := startIdx; i < endIdx; i++ {
		item := m.Completions[i]

		var badge string
		switch item.Kind {
		case lsp.CompletionItemKindClass:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#569CD6")).Bold(true).Render("[TBL]")
		case lsp.CompletionItemKindInterface:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0")).Bold(true).Render("[VIEW]")
		case lsp.CompletionItemKindField:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#B5CEA8")).Bold(true).Render("[COL]")
		case lsp.CompletionItemKindKeyword:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#C586C0")).Bold(true).Render("[KWD]")
		case lsp.CompletionItemKindFunction:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#DCDCAA")).Bold(true).Render("[FUNC]")
		case lsp.CompletionItemKindModule:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#CE9178")).Bold(true).Render("[SCH]")
		case lsp.CompletionItemKindSnippet:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CDCFE")).Bold(true).Render("[SNP]")
		default:
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("[SQL]")
		}

		label := item.Label
		if len(label) > 22 {
			label = label[:19] + "..."
		}
		detail := item.Detail
		if len(detail) > 18 {
			detail = detail[:15] + "..."
		}

		lineContent := fmt.Sprintf("%s %-22s %s", badge, label, detail)

		if i == m.SelectedCompletion {
			rows = append(rows, selectedStyle.Render(" ▶ "+lineContent+" "))
		} else {
			rows = append(rows, normalStyle.Render("   "+lineContent+" "))
		}
	}

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")).
		Render(fmt.Sprintf(" [Esc: Close]          ▲ %d/%d ▼ ", m.SelectedCompletion+1, total))
	rows = append(rows, footer)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSecondary).
		Background(lipgloss.Color("#1E1E1E")).
		Padding(0, 1)

	return boxStyle.Render(strings.Join(rows, "\n"))
}

func (m *Model) GetCursorPosition() (int, int) {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return 1, 1
	}
	ta := m.Tabs[m.ActiveIndex].Textarea
	return ta.Line() + 1, ta.LineInfo().ColumnOffset + 1
}

// Selection helpers
func (m *Model) HasSelection() bool {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return false
	}
	return m.Tabs[m.ActiveIndex].SelectionExist
}

func (m *Model) ClearSelection() {
	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		m.Tabs[m.ActiveIndex].SelectionExist = false
		m.Tabs[m.ActiveIndex].Selecting = false
		m.Tabs[m.ActiveIndex].VisualMode = false
	}
}

func (m *Model) SelectAll() {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return
	}
	tab := &m.Tabs[m.ActiveIndex]
	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		return
	}
	tab.SelStartLine = 0
	tab.SelStartCol = 0
	tab.SelEndLine = len(lines) - 1
	tab.SelEndCol = len(lines[len(lines)-1])
	tab.SelectionExist = true
}

func (m *Model) GetSelectedText() string {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return ""
	}
	tab := &m.Tabs[m.ActiveIndex]
	if !tab.SelectionExist {
		return ""
	}

	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		return ""
	}

	sLine, sCol := tab.SelStartLine, tab.SelStartCol
	eLine, eCol := tab.SelEndLine, tab.SelEndCol

	if sLine > eLine || (sLine == eLine && sCol > eCol) {
		sLine, eLine = eLine, sLine
		sCol, eCol = eCol, sCol
	}

	if sLine >= len(lines) {
		sLine = len(lines) - 1
	}
	if eLine >= len(lines) {
		eLine = len(lines) - 1
	}
	if sLine < 0 {
		sLine = 0
	}
	if eLine < 0 {
		eLine = 0
	}

	if sLine == eLine {
		line := lines[sLine]
		if sCol > len(line) {
			sCol = len(line)
		}
		if eCol > len(line) {
			eCol = len(line)
		}
		if sCol > eCol {
			sCol, eCol = eCol, sCol
		}
		return line[sCol:eCol]
	}

	var result []string
	firstLine := lines[sLine]
	if sCol > len(firstLine) {
		sCol = len(firstLine)
	}
	result = append(result, firstLine[sCol:])

	for i := sLine + 1; i < eLine; i++ {
		result = append(result, lines[i])
	}

	lastLine := lines[eLine]
	if eCol > len(lastLine) {
		eCol = len(lastLine)
	}
	result = append(result, lastLine[:eCol])

	return strings.Join(result, "\n")
}

func (m *Model) DeleteSelectedText() {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return
	}
	tab := &m.Tabs[m.ActiveIndex]
	if !tab.SelectionExist {
		return
	}

	lines := strings.Split(tab.Textarea.Value(), "\n")
	sLine, sCol := tab.SelStartLine, tab.SelStartCol
	eLine, eCol := tab.SelEndLine, tab.SelEndCol

	if sLine > eLine || (sLine == eLine && sCol > eCol) {
		sLine, eLine = eLine, sLine
		sCol, eCol = eCol, sCol
	}

	if sLine >= len(lines) {
		sLine = len(lines) - 1
	}
	if eLine >= len(lines) {
		eLine = len(lines) - 1
	}

	var newLines []string
	for i := 0; i < sLine; i++ {
		newLines = append(newLines, lines[i])
	}

	prefix := ""
	if sLine < len(lines) && sCol <= len(lines[sLine]) {
		prefix = lines[sLine][:sCol]
	}

	suffix := ""
	if eLine < len(lines) && eCol <= len(lines[eLine]) {
		suffix = lines[eLine][eCol:]
	}

	newLines = append(newLines, prefix+suffix)

	for i := eLine + 1; i < len(lines); i++ {
		newLines = append(newLines, lines[i])
	}

	tab.Textarea.SetValue(strings.Join(newLines, "\n"))
	tab.SelectionExist = false
	tab.Selecting = false
	tab.VisualMode = false
	tab.Modified = true
}

func (m *Model) Copy() (string, error) {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return "", nil
	}
	tab := &m.Tabs[m.ActiveIndex]

	textToCopy := ""
	if tab.SelectionExist {
		textToCopy = m.GetSelectedText()
	} else {
		textToCopy = tab.Textarea.Value()
	}

	if textToCopy != "" {
		err := clipboard.WriteAll(textToCopy)
		return textToCopy, err
	}
	return "", nil
}

func (m *Model) Cut() (string, error) {
	if !m.HasSelection() {
		return "", nil
	}
	text, err := m.Copy()
	if err == nil {
		m.DeleteSelectedText()
	}
	return text, err
}

func (m *Model) Paste() error {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return nil
	}
	clipText, err := clipboard.ReadAll()
	if err != nil || clipText == "" {
		return err
	}

	if m.HasSelection() {
		m.DeleteSelectedText()
	}

	tab := &m.Tabs[m.ActiveIndex]
	tab.Textarea.InsertString(clipText)
	tab.Modified = true
	return nil
}

func (m *Model) HandleMouse(msg tea.MouseMsg, relX, relY int) (Model, tea.Cmd) {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return *m, nil
	}
	tab := &m.Tabs[m.ActiveIndex]

	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	gutterWidth := 6 // "  1 │ "

	calcLineCol := func(rx, ry int) (int, int) {
		lineIdx := ry - 1
		if lineIdx < 0 {
			lineIdx = 0
		}
		if lineIdx >= len(lines) {
			lineIdx = len(lines) - 1
		}

		colIdx := rx - gutterWidth
		if colIdx < 0 {
			colIdx = 0
		}
		if colIdx > len(lines[lineIdx]) {
			colIdx = len(lines[lineIdx])
		}
		return lineIdx, colIdx
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		if relY == 0 {
			// Clicked on Tab Bar
			currentX := 0
			for i, t := range m.Tabs {
				tabWidth := len(t.Title) + 4
				if t.Modified {
					tabWidth++
				}
				if relX >= currentX && relX < currentX+tabWidth {
					m.ActiveIndex = i
					m.Focus()
					return *m, nil
				}
				currentX += tabWidth
			}
			return *m, nil
		}

		// Click inside text area -> Start mouse selection
		lineIdx, colIdx := calcLineCol(relX, relY)
		tab.SelStartLine = lineIdx
		tab.SelStartCol = colIdx
		tab.SelEndLine = lineIdx
		tab.SelEndCol = colIdx
		tab.Selecting = true
		tab.SelectionExist = false
		m.MouseDown = true
		return *m, nil

	} else if msg.Action == tea.MouseActionMotion && m.MouseDown {
		// Mouse Dragging -> Extend selection
		lineIdx, colIdx := calcLineCol(relX, relY)
		tab.SelEndLine = lineIdx
		tab.SelEndCol = colIdx
		if tab.SelStartLine != tab.SelEndLine || tab.SelStartCol != tab.SelEndCol {
			tab.SelectionExist = true
			tab.Selecting = true
		}
		return *m, nil

	} else if msg.Action == tea.MouseActionRelease {
		if m.MouseDown {
			lineIdx, colIdx := calcLineCol(relX, relY)
			tab.SelEndLine = lineIdx
			tab.SelEndCol = colIdx
			m.MouseDown = false
			tab.Selecting = false
			if tab.SelStartLine != tab.SelEndLine || tab.SelStartCol != tab.SelEndCol {
				tab.SelectionExist = true
			}
		}
		return *m, nil
	}

	var cmd tea.Cmd
	tab.Textarea, cmd = tab.Textarea.Update(msg)
	return *m, cmd
}

func (m *Model) handleShiftArrow(direction string) {
	if len(m.Tabs) == 0 || m.ActiveIndex >= len(m.Tabs) {
		return
	}
	tab := &m.Tabs[m.ActiveIndex]

	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		return
	}

	if !tab.SelectionExist {
		curLine := tab.Textarea.Line()
		curCol := tab.Textarea.LineInfo().ColumnOffset
		tab.SelStartLine = curLine
		tab.SelStartCol = curCol
		tab.SelEndLine = curLine
		tab.SelEndCol = curCol
		tab.SelectionExist = true
	}

	switch direction {
	case "down":
		if tab.SelEndLine < len(lines)-1 {
			tab.SelEndLine++
			tab.Textarea.CursorDown()
			if tab.SelEndCol > len(lines[tab.SelEndLine]) {
				tab.SelEndCol = len(lines[tab.SelEndLine])
			}
		} else {
			// At the bottom line: select to the end of the line
			tab.SelEndCol = len(lines[len(lines)-1])
		}

	case "up":
		if tab.SelEndLine > 0 {
			tab.SelEndLine--
			tab.Textarea.CursorUp()
			if tab.SelEndCol > len(lines[tab.SelEndLine]) {
				tab.SelEndCol = len(lines[tab.SelEndLine])
			}
		} else {
			// At the top line: select to beginning of line
			tab.SelEndCol = 0
		}

	case "right":
		if tab.SelEndCol < len(lines[tab.SelEndLine]) {
			tab.SelEndCol++
		} else if tab.SelEndLine < len(lines)-1 {
			tab.SelEndLine++
			tab.SelEndCol = 0
			tab.Textarea.CursorDown()
		}

	case "left":
		if tab.SelEndCol > 0 {
			tab.SelEndCol--
		} else if tab.SelEndLine > 0 {
			tab.SelEndLine--
			tab.SelEndCol = len(lines[tab.SelEndLine])
			tab.Textarea.CursorUp()
		}

	case "home":
		tab.SelEndCol = 0

	case "end":
		if tab.SelEndLine < len(lines) {
			tab.SelEndCol = len(lines[tab.SelEndLine])
		}
	}

	tab.SelectionExist = true
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.HandleMouse(msg, msg.X, msg.Y)

	case tea.KeyMsg:
		tab := &m.Tabs[m.ActiveIndex]
		keyStr := msg.String()

		switch keyStr {
		case "shift+down":
			m.handleShiftArrow("down")
			return *m, nil

		case "shift+up":
			m.handleShiftArrow("up")
			return *m, nil

		case "shift+left":
			m.handleShiftArrow("left")
			return *m, nil

		case "shift+right":
			m.handleShiftArrow("right")
			return *m, nil

		case "shift+home":
			m.handleShiftArrow("home")
			return *m, nil

		case "shift+end":
			m.handleShiftArrow("end")
			return *m, nil

		case "ctrl+b", "ctrl+g", "alt+v", "√":
			// Toggle Visual Selection Mode
			tab.VisualMode = !tab.VisualMode
			if tab.VisualMode {
				curLine := tab.Textarea.Line()
				curCol := tab.Textarea.LineInfo().ColumnOffset
				tab.SelStartLine = curLine
				tab.SelStartCol = curCol
				tab.SelEndLine = curLine
				tab.SelEndCol = curCol
				tab.SelectionExist = true
			} else {
				tab.SelectionExist = false
			}
			return *m, nil

		case "ctrl+space", "ctrl+@", "ctrl+.", "alt+.", "ctrl+j", "alt+space", "f2":
			m.TriggerCompletion(true)
			return *m, nil

		case "up", "ctrl+p":
			if m.CompletionActive && len(m.Completions) > 0 {
				m.SelectedCompletion--
				if m.SelectedCompletion < 0 {
					m.SelectedCompletion = len(m.Completions) - 1
				}
				return *m, nil
			}
			if tab.VisualMode {
				m.handleShiftArrow("up")
				return *m, nil
			}
			if tab.SelectionExist {
				tab.SelectionExist = false
			}

		case "down":
			if m.CompletionActive && len(m.Completions) > 0 {
				m.SelectedCompletion++
				if m.SelectedCompletion >= len(m.Completions) {
					m.SelectedCompletion = 0
				}
				return *m, nil
			}
			if tab.VisualMode {
				m.handleShiftArrow("down")
				return *m, nil
			}
			if tab.SelectionExist {
				tab.SelectionExist = false
			}

		case "tab", "enter":
			if m.CompletionActive && len(m.Completions) > 0 {
				if m.AcceptCompletion() {
					return *m, nil
				}
			}

		case "f5", "ctrl+e":
			query := m.GetCurrentQuery()
			return *m, func() tea.Msg {
				return ExecuteQueryMsg{Query: query}
			}

		case "ctrl+a":
			m.SelectAll()
			return *m, nil

		case "ctrl+c":
			copied, err := m.Copy()
			if err == nil && copied != "" {
				return *m, func() tea.Msg {
					return EditorToastMsg{Message: fmt.Sprintf("Copied %d characters to clipboard", len(copied))}
				}
			}
			return *m, nil

		case "ctrl+x":
			cutText, err := m.Cut()
			if err == nil && cutText != "" {
				return *m, func() tea.Msg {
					return EditorToastMsg{Message: fmt.Sprintf("Cut %d characters to clipboard", len(cutText))}
				}
			}
			return *m, nil

		case "ctrl+v":
			_ = m.Paste()
			return *m, func() tea.Msg {
				return EditorToastMsg{Message: "Pasted from clipboard"}
			}

		case "esc":
			if m.CompletionActive {
				m.CompletionActive = false
				m.Completions = nil
				return *m, nil
			}
			if tab.SelectionExist || tab.VisualMode {
				m.ClearSelection()
				return *m, nil
			}

		case "ctrl+n":
			if m.CompletionActive && len(m.Completions) > 0 {
				m.SelectedCompletion++
				if m.SelectedCompletion >= len(m.Completions) {
					m.SelectedCompletion = 0
				}
				return *m, nil
			}
			m.NewTab("", "")
			return *m, nil

		case "ctrl+s":
			if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
				tab := &m.Tabs[m.ActiveIndex]
				if tab.FilePath != "" {
					if err := os.WriteFile(tab.FilePath, []byte(tab.Textarea.Value()), 0644); err == nil {
						tab.Modified = false
						return *m, func() tea.Msg {
							return EditorToastMsg{Message: fmt.Sprintf("✓ Saved to %s", filepath.Base(tab.FilePath))}
						}
					}
				}
				// Open Save Prompt for unsaved queries
				return *m, func() tea.Msg {
					return PromptSaveFileMsg{
						Content:     tab.Textarea.Value(),
						DefaultName: tab.Title,
					}
				}
			}
			return *m, nil

		case "ctrl+w":
			m.CloseActiveTab()
			return *m, nil

		case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5":
			tabNum := int(msg.Runes[0] - '0')
			if tabNum >= 1 && tabNum <= len(m.Tabs) {
				m.ActiveIndex = tabNum - 1
				m.Focus()
			}
			return *m, nil
		}
	}

	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		var cmd tea.Cmd
		m.Tabs[m.ActiveIndex].Textarea, cmd = m.Tabs[m.ActiveIndex].Textarea.Update(msg)
		cmds = append(cmds, cmd)

		// Auto-trigger completion on '.' or while typing identifiers
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			keyStr := keyMsg.String()
			if keyStr == "." {
				m.TriggerCompletion(false)
			} else if len(keyStr) == 1 && (unicode.IsLetter(rune(keyStr[0])) || unicode.IsDigit(rune(keyStr[0])) || keyStr[0] == '_') {
				m.TriggerCompletion(false)
			} else if m.CompletionActive {
				if keyStr == "backspace" {
					m.TriggerCompletion(false)
				} else if keyStr == " " || keyStr == "\n" {
					m.CompletionActive = false
					m.Completions = nil
				}
			}
		}
	}

	return *m, tea.Batch(cmds...)
}

func (m Model) renderHighlightedView(tab *TabInfo) string {
	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		return ""
	}

	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	activeLineNumStyle := lipgloss.NewStyle().Foreground(theme.ColorSecondary).Bold(true)
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	var renderedLines []string
	curLine := tab.Textarea.Line()
	curCol := tab.Textarea.LineInfo().ColumnOffset

	for i, line := range lines {
		// Line numbering prefix
		var numStr string
		if i == curLine {
			numStr = activeLineNumStyle.Render(fmt.Sprintf("%3d │ ", i+1))
		} else {
			numStr = lineNumStyle.Render(fmt.Sprintf("%3d │ ", i+1))
		}

		if i == curLine && m.Focused {
			// Render cursor inside line
			runes := []rune(line)
			if curCol >= len(runes) {
				renderedLines = append(renderedLines, numStr+syntax.HighlightLine(line)+cursorStyle.Render(" "))
			} else {
				before := string(runes[:curCol])
				cChar := string(runes[curCol : curCol+1])
				after := string(runes[curCol+1:])
				renderedLines = append(renderedLines, numStr+syntax.HighlightLine(before)+cursorStyle.Render(cChar)+syntax.HighlightLine(after))
			}

			// Render inline autocomplete popup directly under current line
			if m.CompletionActive && len(m.Completions) > 0 {
				popup := m.renderCompletionPopup()
				renderedLines = append(renderedLines, popup)
			}
		} else {
			renderedLines = append(renderedLines, numStr+syntax.HighlightLine(line))
		}
	}

	return strings.Join(renderedLines, "\n")
}

func (m Model) renderSelectionView(tab *TabInfo) string {
	lines := strings.Split(tab.Textarea.Value(), "\n")
	if len(lines) == 0 {
		return ""
	}

	sLine, sCol := tab.SelStartLine, tab.SelStartCol
	eLine, eCol := tab.SelEndLine, tab.SelEndCol

	if sLine > eLine || (sLine == eLine && sCol > eCol) {
		sLine, eLine = eLine, sLine
		sCol, eCol = eCol, sCol
	}

	selStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#264F78")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	activeLineNumStyle := lipgloss.NewStyle().Foreground(theme.ColorSecondary).Bold(true)

	var renderedLines []string
	curLine := tab.Textarea.Line()

	for i, line := range lines {
		// Line numbering prefix
		var numStr string
		if i == curLine {
			numStr = activeLineNumStyle.Render(fmt.Sprintf("%3d │ ", i+1))
		} else {
			numStr = lineNumStyle.Render(fmt.Sprintf("%3d │ ", i+1))
		}

		if i < sLine || i > eLine {
			// Outside selection -> Syntax highlighted
			renderedLines = append(renderedLines, numStr+syntax.HighlightLine(line))
			continue
		}

		if i == sLine && i == eLine {
			// Selection starts and ends on this line
			start := sCol
			end := eCol
			if start > len(line) {
				start = len(line)
			}
			if end > len(line) {
				end = len(line)
			}
			if start > end {
				start, end = end, start
			}

			p1 := syntax.HighlightLine(line[:start])
			selectedChunk := line[start:end]
			if selectedChunk == "" {
				selectedChunk = " "
			}
			p2 := selStyle.Render(selectedChunk)
			p3 := syntax.HighlightLine(line[end:])
			renderedLines = append(renderedLines, numStr+p1+p2+p3)
		} else if i == sLine {
			// Selection starts on this line
			start := sCol
			if start > len(line) {
				start = len(line)
			}
			p1 := syntax.HighlightLine(line[:start])
			selectedChunk := line[start:]
			if selectedChunk == "" {
				selectedChunk = " "
			}
			p2 := selStyle.Render(selectedChunk)
			renderedLines = append(renderedLines, numStr+p1+p2)
		} else if i == eLine {
			// Selection ends on this line
			end := eCol
			if end > len(line) {
				end = len(line)
			}
			selectedChunk := line[:end]
			if selectedChunk == "" {
				selectedChunk = " "
			}
			p1 := selStyle.Render(selectedChunk)
			p2 := syntax.HighlightLine(line[end:])
			renderedLines = append(renderedLines, numStr+p1+p2)
		} else {
			// Full line is inside selection
			selectedChunk := line
			if selectedChunk == "" {
				selectedChunk = " "
			}
			renderedLines = append(renderedLines, numStr+selStyle.Render(selectedChunk))
		}
	}

	content := strings.Join(renderedLines, "\n")
	modeIndicator := "Selection"
	if tab.VisualMode {
		modeIndicator = "Visual Mode (Arrows to extend)"
	}
	infoBar := theme.StyleFgMuted.Render(fmt.Sprintf(" [%s: %d characters | F5: Execute Selection | Ctrl+C: Copy | Esc: Deselect]", modeIndicator, len(m.GetSelectedText())))
	return content + "\n" + infoBar
}

func (m Model) View() string {
	var b strings.Builder

	// Tab Bar Header
	var tabHeaders []string
	for i, tab := range m.Tabs {
		title := tab.Title
		if tab.Modified {
			title += "*"
		}
		if tab.VisualMode {
			title += " [VISUAL]"
		} else if tab.SelectionExist {
			title += " [SELECTED]"
		}
		if i == m.ActiveIndex {
			tabHeaders = append(tabHeaders, theme.TabActive.Render(fmt.Sprintf(" %s ", title)))
		} else {
			tabHeaders = append(tabHeaders, theme.TabInactive.Render(fmt.Sprintf(" %s ", title)))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabHeaders...)
	b.WriteString(tabBar)
	b.WriteString("\n")

	// Highlighted Textarea View
	if len(m.Tabs) > 0 && m.ActiveIndex < len(m.Tabs) {
		tab := &m.Tabs[m.ActiveIndex]
		if tab.SelectionExist {
			b.WriteString(m.renderSelectionView(tab))
		} else {
			b.WriteString(m.renderHighlightedView(tab))
		}
	}

	return lipgloss.NewStyle().Width(m.Width).Height(m.Height).Render(b.String())
}
