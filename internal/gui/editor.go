package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type EditorTab struct {
	ID       string
	Title    string
	FilePath string
	Entry    *widget.Entry
	Item     *container.TabItem
}

type EditorController struct {
	Container *fyne.Container
	tabs      *container.DocTabs
	tabList   []*EditorTab
	tabCount  int
}

func NewEditorController() *EditorController {
	ec := &EditorController{
		tabs: container.NewDocTabs(),
	}

	ec.Container = container.NewBorder(nil, nil, nil, nil, ec.tabs)
	ec.NewTab("Query 1.sql", "SELECT TOP 100 *\nFROM SalesDB.sales.Customers;\n")
	return ec
}

func (ec *EditorController) NewTab(title string, initialSQL string) *EditorTab {
	ec.tabCount++
	if title == "" {
		title = fmt.Sprintf("Query %d.sql", ec.tabCount)
	}

	entry := widget.NewMultiLineEntry()
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.Wrapping = fyne.TextWrapOff
	entry.SetText(initialSQL)

	tab := &EditorTab{
		ID:    fmt.Sprintf("tab-%d", ec.tabCount),
		Title: title,
		Entry: entry,
	}

	tab.Item = container.NewTabItemWithIcon(title, theme.DocumentIcon(), container.NewBorder(nil, nil, nil, nil, entry))
	ec.tabList = append(ec.tabList, tab)
	ec.tabs.Append(tab.Item)
	ec.tabs.Select(tab.Item)

	return tab
}

func (ec *EditorController) GetActiveQuery() string {
	selected := ec.tabs.Selected()
	if selected == nil {
		return ""
	}
	for _, t := range ec.tabList {
		if t.Item == selected {
			return t.Entry.Text
		}
	}
	return ""
}

func (ec *EditorController) InsertSQL(sql string) {
	selected := ec.tabs.Selected()
	if selected == nil {
		ec.NewTab("", sql)
		return
	}
	for _, t := range ec.tabList {
		if t.Item == selected {
			if t.Entry.Text == "" {
				t.Entry.SetText(sql)
			} else {
				t.Entry.SetText(t.Entry.Text + "\n\n" + sql)
			}
			return
		}
	}
}

func (ec *EditorController) OpenFile(path string, content string) {
	title := filepath.Base(path)
	tab := ec.NewTab(title, content)
	tab.FilePath = path
}

func (ec *EditorController) SaveActiveTab(customPath string) (string, error) {
	selected := ec.tabs.Selected()
	if selected == nil {
		return "", fmt.Errorf("no active editor tab")
	}

	var activeTab *EditorTab
	for _, t := range ec.tabList {
		if t.Item == selected {
			activeTab = t
			break
		}
	}
	if activeTab == nil {
		return "", fmt.Errorf("active tab not found")
	}

	targetPath := activeTab.FilePath
	if customPath != "" {
		targetPath = customPath
	}
	if targetPath == "" {
		_ = os.MkdirAll("queries", 0755)
		targetPath = filepath.Join("queries", activeTab.Title)
	}

	if err := os.WriteFile(targetPath, []byte(activeTab.Entry.Text), 0644); err != nil {
		return "", err
	}

	activeTab.FilePath = targetPath
	activeTab.Title = filepath.Base(targetPath)
	activeTab.Item.Text = activeTab.Title
	ec.tabs.Refresh()

	return targetPath, nil
}
