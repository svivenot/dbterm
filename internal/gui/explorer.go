package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/config"
	"dbterm/internal/db"
)

// ExplorerController manages the tree-view of database objects and local SQL files
type ExplorerController struct {
	Container      *fyne.Container
	dbTree         *widget.Tree
	fileTree       *widget.Tree
	tabs           *container.AppTabs
	cfg            *config.Config
	driver         db.Driver
	activeProfile  *config.ConnectionProfile
	onQuerySelect  func(sql string)
	onFileOpen     func(path string, content string)
	treeNodes      map[string][]string
	nodeLabels     map[string]string
	nodeIcons      map[string]fyne.Resource
}

func NewExplorerController(
	cfg *config.Config,
	initialProfile *config.ConnectionProfile,
	driver db.Driver,
	onQuerySelect func(string),
	onFileOpen func(string, string),
) *ExplorerController {
	ec := &ExplorerController{
		cfg:           cfg,
		activeProfile: initialProfile,
		driver:        driver,
		onQuerySelect: onQuerySelect,
		onFileOpen:    onFileOpen,
		treeNodes:     make(map[string][]string),
		nodeLabels:    make(map[string]string),
		nodeIcons:     make(map[string]fyne.Resource),
	}

	ec.initDBTree()
	ec.initFileTree()

	ec.tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("Databases", theme.StorageIcon(), ec.dbTree),
		container.NewTabItemWithIcon("SQL Files", theme.FolderIcon(), ec.fileTree),
	)

	ec.Container = container.NewBorder(nil, nil, nil, nil, ec.tabs)
	ec.Refresh()
	return ec
}

func (ec *ExplorerController) SetConnection(profile *config.ConnectionProfile, driver db.Driver) {
	ec.activeProfile = profile
	ec.driver = driver
	ec.Refresh()
}

func (ec *ExplorerController) Refresh() {
	ec.rebuildDBTree()
	ec.dbTree.Refresh()
	ec.rebuildFileTree()
	ec.fileTree.Refresh()
}

func (ec *ExplorerController) initDBTree() {
	ec.dbTree = widget.NewTree(
		func(uid string) []string {
			return ec.treeNodes[uid]
		},
		func(uid string) bool {
			return len(ec.treeNodes[uid]) > 0
		},
		func(branch bool) fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.StorageIcon()),
				widget.NewLabel("Object Node"),
			)
		},
		func(uid string, branch bool, item fyne.CanvasObject) {
			box := item.(*fyne.Container)
			icon := box.Objects[0].(*widget.Icon)
			label := box.Objects[1].(*widget.Label)

			if ic, ok := ec.nodeIcons[uid]; ok {
				icon.SetResource(ic)
			} else if branch {
				icon.SetResource(theme.FolderIcon())
			} else {
				icon.SetResource(theme.FileIcon())
			}

			if txt, ok := ec.nodeLabels[uid]; ok {
				label.SetText(txt)
			} else {
				label.SetText(uid)
			}
		},
	)

	ec.dbTree.OnSelected = func(uid string) {
		if strings.HasPrefix(uid, "tbl:") {
			parts := strings.Split(strings.TrimPrefix(uid, "tbl:"), ":")
			if len(parts) >= 2 {
				tableName := parts[1]
				sql := fmt.Sprintf("SELECT TOP 100 *\nFROM %s;", tableName)
				if ec.driver != nil && strings.Contains(strings.ToLower(ec.driver.GetConnectionInfo()), "postgres") {
					sql = fmt.Sprintf("SELECT *\nFROM %s\nLIMIT 100;", tableName)
				}
				if ec.onQuerySelect != nil {
					ec.onQuerySelect(sql)
				}
			}
		}
	}
}

func (ec *ExplorerController) rebuildDBTree() {
	ec.treeNodes = make(map[string][]string)
	ec.nodeLabels = make(map[string]string)
	ec.nodeIcons = make(map[string]fyne.Resource)

	if ec.cfg == nil || len(ec.cfg.Connections) == 0 {
		ec.treeNodes[""] = []string{"empty"}
		ec.nodeLabels["empty"] = "No registered servers (Ctrl+O to add)"
		ec.nodeIcons["empty"] = theme.InfoIcon()
		return
	}

	var rootNodes []string
	for _, p := range ec.cfg.Connections {
		serverID := "srv:" + p.ID
		rootNodes = append(rootNodes, serverID)
		ec.nodeLabels[serverID] = fmt.Sprintf("%s (%s)", p.Name, p.Host)
		ec.nodeIcons[serverID] = theme.StorageIcon()

		// If connected to this server, populate live databases and tables
		if ec.activeProfile != nil && ec.activeProfile.ID == p.ID && ec.driver != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			activeDB := ec.driver.GetActiveDatabase()
			if activeDB == "" {
				activeDB = p.Database
			}
			if activeDB == "" {
				activeDB = "master"
			}

			dbID := fmt.Sprintf("db:%s:%s", p.ID, activeDB)
			ec.treeNodes[serverID] = []string{dbID}
			ec.nodeLabels[dbID] = fmt.Sprintf("🗄 %s (Connected)", activeDB)
			ec.nodeIcons[dbID] = theme.FolderOpenIcon()

			tablesID := fmt.Sprintf("tables:%s:%s", p.ID, activeDB)
			viewsID := fmt.Sprintf("views:%s:%s", p.ID, activeDB)
			ec.treeNodes[dbID] = []string{tablesID, viewsID}

			ec.nodeLabels[tablesID] = "📁 Tables"
			ec.nodeIcons[tablesID] = theme.FolderIcon()

			ec.nodeLabels[viewsID] = "📁 Views"
			ec.nodeIcons[viewsID] = theme.FolderIcon()

			tables, err := ec.driver.FetchTables(ctx, activeDB)
			cancel()
			if err == nil {
				var tableNodeIDs []string
				for _, t := range tables {
					fullTableName := t.Name
					if t.Schema != "" && !strings.Contains(t.Name, ".") {
						fullTableName = fmt.Sprintf("%s.%s", t.Schema, t.Name)
					}
					tID := fmt.Sprintf("tbl:%s:%s", p.ID, fullTableName)
					tableNodeIDs = append(tableNodeIDs, tID)
					ec.nodeLabels[tID] = fullTableName
					ec.nodeIcons[tID] = theme.DocumentIcon()
				}
				ec.treeNodes[tablesID] = tableNodeIDs
			}
		}
	}
	ec.treeNodes[""] = rootNodes
}

func (ec *ExplorerController) initFileTree() {
	ec.fileTree = widget.NewTree(
		func(uid string) []string {
			return ec.treeNodes["file:"+uid]
		},
		func(uid string) bool {
			return len(ec.treeNodes["file:"+uid]) > 0
		},
		func(branch bool) fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.FileIcon()),
				widget.NewLabel("SQL File"),
			)
		},
		func(uid string, branch bool, item fyne.CanvasObject) {
			box := item.(*fyne.Container)
			icon := box.Objects[0].(*widget.Icon)
			label := box.Objects[1].(*widget.Label)

			if branch {
				icon.SetResource(theme.FolderIcon())
			} else {
				icon.SetResource(theme.DocumentIcon())
			}
			label.SetText(filepath.Base(uid))
		},
	)

	ec.fileTree.OnSelected = func(uid string) {
		if strings.HasSuffix(strings.ToLower(uid), ".sql") {
			content, err := os.ReadFile(uid)
			if err == nil && ec.onFileOpen != nil {
				ec.onFileOpen(uid, string(content))
			}
		}
	}
}

func (ec *ExplorerController) rebuildFileTree() {
	rootPath := "."
	var files []string

	entries, err := os.ReadDir(rootPath)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				if e.Name() == "queries" || e.Name() == "sql" || e.Name() == "scripts" {
					dirPath := filepath.Join(rootPath, e.Name())
					files = append(files, dirPath)
					subEntries, _ := os.ReadDir(dirPath)
					var subFiles []string
					for _, se := range subEntries {
						if strings.HasSuffix(strings.ToLower(se.Name()), ".sql") {
							subFiles = append(subFiles, filepath.Join(dirPath, se.Name()))
						}
					}
					ec.treeNodes["file:"+dirPath] = subFiles
				}
			} else if strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
				files = append(files, filepath.Join(rootPath, e.Name()))
			}
		}
	}
	ec.treeNodes["file:"] = files
}
