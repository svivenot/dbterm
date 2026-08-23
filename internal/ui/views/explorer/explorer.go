package explorer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/ui/theme"
)

// Messages emitted by Explorer
type ScriptTableMsg struct {
	Query string
}

type NodeSelectedMsg struct {
	Node *db.TreeNode
}

type ConnectServerMsg struct {
	Profile *config.ConnectionProfile
}

type SwitchDatabaseMsg struct {
	Database string
}

type OpenFileMsg struct {
	FilePath string
	Content  string
	FileName string
}

type ContextMenuItem struct {
	Title  string
	Action string
}

type ExplorerTab int

const (
	TabDatabases ExplorerTab = 0
	TabFiles     ExplorerTab = 1
)

type Model struct {
	Config            *config.Config
	ActiveProfile     *config.ConnectionProfile
	ActiveDriver      db.Driver
	ActiveDB          string
	ActiveTab         ExplorerTab
	RootNodes         []*db.TreeNode
	Flattened         []*db.TreeNode
	Cursor            int
	FileRootNodes     []*db.TreeNode
	FileFlattened     []*db.TreeNode
	FileCursor        int
	RootDirectory     string
	Width             int
	Height            int
	Filter            string
	Filtering         bool
	ErrorMessage      string
	Visible           bool
	ContextMenuOpen   bool
	ContextMenuCursor int
	ContextMenuRelY   int
	MenuItems         []ContextMenuItem
	lastClickIdx      int
	lastClickTime     time.Time
}

func New(cfg *config.Config, activeProfile *config.ConnectionProfile, activeDriver db.Driver, activeDB string) Model {
	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}

	m := Model{
		Config:        cfg,
		ActiveProfile: activeProfile,
		ActiveDriver:  activeDriver,
		ActiveDB:      activeDB,
		ActiveTab:     TabDatabases,
		RootDirectory: cwd,
		Cursor:        0,
		FileCursor:    0,
		Visible:       true,
		lastClickIdx:  -1,
	}
	m.Refresh()
	m.RefreshFiles()
	return m
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
}

func (m *Model) SetActiveConnection(profile *config.ConnectionProfile, driver db.Driver, activeDB string) {
	m.ActiveProfile = profile
	m.ActiveDriver = driver
	m.ActiveDB = activeDB
	m.Refresh()
}

// -------------------------------------------------------------
// Database Server Tree Logic (Tab 0)
// -------------------------------------------------------------

func (m *Model) Refresh() {
	expandedState := make(map[string]bool)
	for _, n := range m.Flattened {
		expandedState[n.ID] = n.Expanded
	}

	if m.Config == nil || len(m.Config.Connections) == 0 {
		serverName := "Local Server"
		if m.ActiveProfile != nil {
			serverName = m.ActiveProfile.Name
		}
		serverNode := &db.TreeNode{
			ID:        "server_standalone",
			Name:      serverName,
			Type:      db.NodeServer,
			Connected: (m.ActiveDriver != nil),
			Expanded:  true,
			Loaded:    true,
		}
		if m.ActiveDriver != nil {
			m.loadServerDatabases(serverNode, expandedState)
		}
		m.RootNodes = []*db.TreeNode{serverNode}
		m.rebuildFlattened()
		return
	}

	groupMap := make(map[string][]*config.ConnectionProfile)
	var groupOrder []string

	for i := range m.Config.Connections {
		c := &m.Config.Connections[i]

		if m.ActiveTab == TabDatabases && m.Filter != "" {
			query := strings.ToLower(m.Filter)
			match := strings.Contains(strings.ToLower(c.Name), query) ||
				strings.Contains(strings.ToLower(c.Host), query) ||
				strings.Contains(strings.ToLower(c.Database), query) ||
				strings.Contains(strings.ToLower(c.Driver), query) ||
				strings.Contains(strings.ToLower(c.GetGroup()), query)
			if !match {
				continue
			}
		}

		grp := c.GetGroup()
		if _, exists := groupMap[grp]; !exists {
			groupOrder = append(groupOrder, grp)
		}
		groupMap[grp] = append(groupMap[grp], c)
	}

	sort.Strings(groupOrder)

	var rootNodes []*db.TreeNode
	for _, grpName := range groupOrder {
		profiles := groupMap[grpName]

		grpID := "grp_" + grpName
		isGrpExpanded := true
		if val, ok := expandedState[grpID]; ok {
			isGrpExpanded = val
		}

		groupNode := &db.TreeNode{
			ID:        grpID,
			Name:      grpName,
			Type:      db.NodeGroup,
			GroupPath: grpName,
			Expanded:  isGrpExpanded,
			Loaded:    true,
		}

		for _, p := range profiles {
			isConnected := (m.ActiveProfile != nil && m.ActiveProfile.ID == p.ID && m.ActiveDriver != nil)
			srvID := "srv_" + p.ID

			isSrvExpanded := isConnected
			if val, ok := expandedState[srvID]; ok {
				isSrvExpanded = val
			}

			serverNode := db.TreeNode{
				ID:         srvID,
				Name:       p.Name,
				Type:       db.NodeServer,
				GroupPath:  grpName,
				ProfileID:  p.ID,
				DriverName: p.Driver,
				Connected:  isConnected,
				Expanded:   isSrvExpanded,
				Loaded:     isConnected,
			}

			if isConnected {
				m.loadServerDatabases(&serverNode, expandedState)
			}

			groupNode.Children = append(groupNode.Children, serverNode)
		}

		rootNodes = append(rootNodes, groupNode)
	}

	m.RootNodes = rootNodes
	m.rebuildFlattened()
}

func (m *Model) loadServerDatabases(serverNode *db.TreeNode, expandedState map[string]bool) {
	if m.ActiveDriver == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbs, err := m.ActiveDriver.FetchDatabases(ctx)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Error fetching DBs: %v", err)
		return
	}

	dbFolderID := serverNode.ID + "_databases"
	dbFolderExpanded := true
	if val, ok := expandedState[dbFolderID]; ok {
		dbFolderExpanded = val
	}

	dbFolder := db.TreeNode{
		ID:       dbFolderID,
		Name:     "Databases",
		Type:     db.NodeDatabases,
		Expanded: dbFolderExpanded,
		Loaded:   true,
	}

	for _, dbName := range dbs {
		isCurrent := (dbName == m.ActiveDB)
		dbNodeID := serverNode.ID + "_db_" + dbName

		isDbExpanded := isCurrent
		if val, ok := expandedState[dbNodeID]; ok {
			isDbExpanded = val
		}

		dbNode := db.TreeNode{
			ID:       dbNodeID,
			Name:     dbName,
			Type:     db.NodeDatabase,
			Catalog:  dbName,
			Expanded: isDbExpanded,
			Loaded:   false,
		}

		if isDbExpanded {
			m.loadDatabaseChildren(&dbNode, expandedState)
		}

		dbFolder.Children = append(dbFolder.Children, dbNode)
	}

	serverNode.Children = []db.TreeNode{dbFolder}
	serverNode.Loaded = true
}

func (m *Model) loadDatabaseChildren(dbNode *db.TreeNode, expandedState map[string]bool) {
	if m.ActiveDriver == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tablesFolderID := dbNode.ID + "_tables"
	tblFolderExpanded := true
	if val, ok := expandedState[tablesFolderID]; ok {
		tblFolderExpanded = val
	}

	tablesFolder := db.TreeNode{
		ID:       tablesFolderID,
		Name:     "Tables",
		Type:     db.NodeFolderTables,
		Catalog:  dbNode.Name,
		Expanded: tblFolderExpanded,
		Loaded:   true,
	}

	tables, _ := m.ActiveDriver.FetchTables(ctx, dbNode.Name)
	for _, t := range tables {
		displayName := fmt.Sprintf("%s.%s", t.Schema, t.Name)
		tblID := fmt.Sprintf("%s_tbl_%s_%s", dbNode.ID, t.Schema, t.Name)

		tblExpanded := false
		if val, ok := expandedState[tblID]; ok {
			tblExpanded = val
		}

		tableNode := db.TreeNode{
			ID:       tblID,
			Name:     displayName,
			Type:     db.NodeTable,
			Catalog:  dbNode.Name,
			Schema:   t.Schema,
			Expanded: tblExpanded,
			Loaded:   false,
		}

		if tblExpanded {
			m.loadColumns(&tableNode)
		}

		tablesFolder.Children = append(tablesFolder.Children, tableNode)
	}

	viewsFolderID := dbNode.ID + "_views"
	viewsFolderExpanded := false
	if val, ok := expandedState[viewsFolderID]; ok {
		viewsFolderExpanded = val
	}

	viewsFolder := db.TreeNode{
		ID:       viewsFolderID,
		Name:     "Views",
		Type:     db.NodeFolderViews,
		Catalog:  dbNode.Name,
		Expanded: viewsFolderExpanded,
		Loaded:   true,
	}

	views, _ := m.ActiveDriver.FetchViews(ctx, dbNode.Name)
	for _, v := range views {
		displayName := fmt.Sprintf("%s.%s", v.Schema, v.Name)
		viewNode := db.TreeNode{
			ID:       fmt.Sprintf("%s_vw_%s_%s", dbNode.ID, v.Schema, v.Name),
			Name:     displayName,
			Type:     db.NodeView,
			Catalog:  dbNode.Name,
			Schema:   v.Schema,
			Expanded: false,
			Loaded:   false,
		}
		viewsFolder.Children = append(viewsFolder.Children, viewNode)
	}

	procsFolderID := dbNode.ID + "_procs"
	procsFolderExpanded := false
	if val, ok := expandedState[procsFolderID]; ok {
		procsFolderExpanded = val
	}

	procsFolder := db.TreeNode{
		ID:       procsFolderID,
		Name:     "Programmability",
		Type:     db.NodeFolderProcs,
		Catalog:  dbNode.Name,
		Expanded: procsFolderExpanded,
		Loaded:   true,
	}

	procs, _ := m.ActiveDriver.FetchProcedures(ctx, dbNode.Name)
	for _, p := range procs {
		procNode := db.TreeNode{
			ID:      fmt.Sprintf("%s_prc_%s", dbNode.ID, p),
			Name:    p,
			Type:    db.NodeProcedure,
			Catalog: dbNode.Name,
		}
		procsFolder.Children = append(procsFolder.Children, procNode)
	}

	dbNode.Children = []db.TreeNode{tablesFolder, viewsFolder, procsFolder}
	dbNode.Loaded = true
}

func (m *Model) loadColumns(tableNode *db.TreeNode) {
	if m.ActiveDriver == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	parts := strings.Split(tableNode.Name, ".")
	schema := tableNode.Schema
	tableName := tableNode.Name
	if len(parts) == 2 {
		schema = parts[0]
		tableName = parts[1]
	}

	cols, err := m.ActiveDriver.FetchColumns(ctx, tableNode.Catalog, schema, tableName)
	if err != nil {
		return
	}

	tableNode.Children = nil
	for _, c := range cols {
		nullStr := "null"
		if !c.IsNullable {
			nullStr = "not null"
		}
		colDisplay := fmt.Sprintf("%s (%s, %s)", c.Name, c.DataType, nullStr)
		colNode := db.TreeNode{
			ID:           fmt.Sprintf("col_%s_%s", tableNode.ID, c.Name),
			Name:         colDisplay,
			Type:         db.NodeColumn,
			Catalog:      tableNode.Catalog,
			Schema:       schema,
			DataType:     c.DataType,
			IsPrimaryKey: c.IsPrimaryKey,
			IsNullable:   c.IsNullable,
		}
		tableNode.Children = append(tableNode.Children, colNode)
	}
	tableNode.Loaded = true
}

func (m *Model) rebuildFlattened() {
	var list []*db.TreeNode
	for _, root := range m.RootNodes {
		m.flattenNode(root, &list)
	}
	m.Flattened = list

	if m.Cursor >= len(m.Flattened) {
		m.Cursor = len(m.Flattened) - 1
	}
	if m.Cursor < 0 && len(m.Flattened) > 0 {
		m.Cursor = 0
	}
}

// -------------------------------------------------------------
// Local SQL File Explorer Tree Logic (Tab 1)
// -------------------------------------------------------------

func (m *Model) RefreshFiles() {
	expandedState := make(map[string]bool)
	for _, n := range m.FileFlattened {
		expandedState[n.FilePath] = n.Expanded
	}

	rootDir := m.RootDirectory
	if rootDir == "" {
		rootDir = "."
	}

	isRootExpanded := true
	if val, ok := expandedState[rootDir]; ok {
		isRootExpanded = val
	}

	rootNode := &db.TreeNode{
		ID:       "file_root",
		Name:     filepath.Base(rootDir),
		Type:     db.NodeFileDir,
		FilePath: rootDir,
		Expanded: isRootExpanded,
		Loaded:   false,
	}

	m.loadDirectoryChildren(rootNode, expandedState)
	m.FileRootNodes = []*db.TreeNode{rootNode}
	m.rebuildFileFlattened()
}

func (m *Model) loadDirectoryChildren(dirNode *db.TreeNode, expandedState map[string]bool) {
	entries, err := os.ReadDir(dirNode.FilePath)
	if err != nil {
		dirNode.Loaded = true
		return
	}

	var dirs []db.TreeNode
	var sqlFiles []db.TreeNode
	var otherFiles []db.TreeNode

	for _, e := range entries {
		name := e.Name()
		// Ignore hidden directories and build outputs
		if strings.HasPrefix(name, ".") || name == "bin" || name == "node_modules" || name == "vendor" {
			continue
		}

		fullPath := filepath.Join(dirNode.FilePath, name)

		if m.ActiveTab == TabFiles && m.Filter != "" {
			if !strings.Contains(strings.ToLower(name), strings.ToLower(m.Filter)) {
				continue
			}
		}

		if e.IsDir() {
			isExpanded := false
			if val, ok := expandedState[fullPath]; ok {
				isExpanded = val
			}
			dirChild := db.TreeNode{
				ID:       "fdir_" + fullPath,
				Name:     name + "/",
				Type:     db.NodeFileDir,
				FilePath: fullPath,
				Expanded: isExpanded,
				Loaded:   false,
			}
			if isExpanded {
				m.loadDirectoryChildren(&dirChild, expandedState)
			}
			dirs = append(dirs, dirChild)
		} else {
			info, _ := e.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}

			if strings.HasSuffix(strings.ToLower(name), ".sql") {
				sqlFiles = append(sqlFiles, db.TreeNode{
					ID:       "fsql_" + fullPath,
					Name:     name,
					Type:     db.NodeFileSQL,
					FilePath: fullPath,
					FileSize: size,
				})
			} else if strings.HasSuffix(strings.ToLower(name), ".json") || strings.HasSuffix(strings.ToLower(name), ".md") || strings.HasSuffix(strings.ToLower(name), ".txt") {
				otherFiles = append(otherFiles, db.TreeNode{
					ID:       "foth_" + fullPath,
					Name:     name,
					Type:     db.NodeFileOther,
					FilePath: fullPath,
					FileSize: size,
				})
			}
		}
	}

	// Sort directories and SQL files alphabetically
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(sqlFiles, func(i, j int) bool { return sqlFiles[i].Name < sqlFiles[j].Name })
	sort.Slice(otherFiles, func(i, j int) bool { return otherFiles[i].Name < otherFiles[j].Name })

	dirNode.Children = append(dirs, append(sqlFiles, otherFiles...)...)
	dirNode.Loaded = true
}

func (m *Model) rebuildFileFlattened() {
	var list []*db.TreeNode
	for _, root := range m.FileRootNodes {
		m.flattenNode(root, &list)
	}
	m.FileFlattened = list

	if m.FileCursor >= len(m.FileFlattened) {
		m.FileCursor = len(m.FileFlattened) - 1
	}
	if m.FileCursor < 0 && len(m.FileFlattened) > 0 {
		m.FileCursor = 0
	}
}

func (m *Model) flattenNode(node *db.TreeNode, list *[]*db.TreeNode) {
	*list = append(*list, node)
	if node.Expanded {
		for i := range node.Children {
			m.flattenNode(&node.Children[i], list)
		}
	}
}

func (m *Model) findProfileByID(profileID string) *config.ConnectionProfile {
	if m.Config == nil {
		return nil
	}
	for i := range m.Config.Connections {
		if m.Config.Connections[i].ID == profileID {
			return &m.Config.Connections[i]
		}
	}
	return nil
}

func (m *Model) openContextMenu(relY int) {
	if m.ActiveTab == TabDatabases {
		if len(m.Flattened) == 0 || m.Cursor >= len(m.Flattened) {
			return
		}
		curr := m.Flattened[m.Cursor]
		var items []ContextMenuItem

		switch curr.Type {
		case db.NodeServer:
			if curr.Connected {
				items = []ContextMenuItem{
					{Title: "🔄 Refresh Databases", Action: "refresh_server"},
					{Title: "📝 New SQL Query Tab", Action: "new_query"},
				}
			} else {
				items = []ContextMenuItem{
					{Title: "▶ Connect to this Server", Action: "connect_server"},
					{Title: "📝 New SQL Query Tab", Action: "new_query"},
				}
			}
		case db.NodeDatabase:
			items = []ContextMenuItem{
				{Title: "▶ Set Active DB (USE)", Action: "use_db"},
				{Title: "🔄 Refresh Objects", Action: "refresh_db"},
				{Title: "📝 New Query for this DB", Action: "new_query"},
			}
		case db.NodeTable:
			items = []ContextMenuItem{
				{Title: "▶ Script as SELECT TOP 100", Action: "select"},
				{Title: "🛠 Script as CREATE (DDL)", Action: "create"},
				{Title: "📝 Script as INSERT Template", Action: "insert"},
				{Title: "📋 Copy Name", Action: "copy"},
			}
		case db.NodeView:
			items = []ContextMenuItem{
				{Title: "▶ Script as SELECT TOP 100", Action: "select"},
				{Title: "🛠 Script as CREATE (DDL)", Action: "create"},
				{Title: "📋 Copy Name", Action: "copy"},
			}
		case db.NodeProcedure, db.NodeFunction:
			items = []ContextMenuItem{
				{Title: "🛠 Script as CREATE (DDL)", Action: "create"},
				{Title: "📋 Copy Name", Action: "copy"},
			}
		default:
			items = []ContextMenuItem{
				{Title: "🔄 Refresh", Action: "refresh"},
			}
		}

		m.MenuItems = items
		m.ContextMenuCursor = 0
		m.ContextMenuRelY = relY
		m.ContextMenuOpen = true
	} else {
		// Files Tab Context Menu
		if len(m.FileFlattened) == 0 || m.FileCursor >= len(m.FileFlattened) {
			return
		}
		curr := m.FileFlattened[m.FileCursor]
		var items []ContextMenuItem

		if curr.Type == db.NodeFileSQL || curr.Type == db.NodeFileOther {
			items = []ContextMenuItem{
				{Title: "▶ Open in Editor", Action: "open_file"},
				{Title: "📋 Copy Path", Action: "copy_path"},
				{Title: "🔄 Refresh Files", Action: "refresh_files"},
			}
		} else {
			items = []ContextMenuItem{
				{Title: "🔄 Refresh Files", Action: "refresh_files"},
			}
		}

		m.MenuItems = items
		m.ContextMenuCursor = 0
		m.ContextMenuRelY = relY
		m.ContextMenuOpen = true
	}
}

func (m *Model) executeContextMenuAction(action string) tea.Cmd {
	m.ContextMenuOpen = false

	if m.ActiveTab == TabDatabases {
		if len(m.Flattened) == 0 || m.Cursor >= len(m.Flattened) {
			return nil
		}
		curr := m.Flattened[m.Cursor]

		switch action {
		case "connect_server":
			if prof := m.findProfileByID(curr.ProfileID); prof != nil {
				return func() tea.Msg {
					return ConnectServerMsg{Profile: prof}
				}
			}

		case "use_db":
			m.ActiveDB = curr.Name
			return func() tea.Msg {
				return SwitchDatabaseMsg{Database: curr.Name}
			}

		case "select":
			if m.ActiveDriver != nil {
				parts := strings.Split(curr.Name, ".")
				schema := curr.Schema
				tbl := curr.Name
				if len(parts) == 2 {
					schema = parts[0]
					tbl = parts[1]
				}
				query := m.ActiveDriver.GenerateSelectQuery(schema, tbl, 100)
				return func() tea.Msg {
					return ScriptTableMsg{Query: query}
				}
			}

		case "create":
			if m.ActiveDriver != nil {
				parts := strings.Split(curr.Name, ".")
				schema := curr.Schema
				tbl := curr.Name
				if len(parts) == 2 {
					schema = parts[0]
					tbl = parts[1]
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				ddl, err := m.ActiveDriver.GenerateDDL(ctx, curr.Catalog, schema, tbl, curr.Type)
				if err != nil {
					ddl = fmt.Sprintf("-- Error generating DDL: %v", err)
				}
				return func() tea.Msg {
					return ScriptTableMsg{Query: ddl}
				}
			}

		case "insert":
			if m.ActiveDriver != nil {
				if !curr.Loaded {
					m.loadColumns(curr)
				}
				var cols []db.ColumnInfo
				for _, c := range curr.Children {
					cols = append(cols, db.ColumnInfo{
						Name:       strings.Split(c.Name, " ")[0],
						DataType:   c.DataType,
						IsNullable: c.IsNullable,
					})
				}
				parts := strings.Split(curr.Name, ".")
				schema := curr.Schema
				tbl := curr.Name
				if len(parts) == 2 {
					schema = parts[0]
					tbl = parts[1]
				}
				query := m.ActiveDriver.GenerateInsertQuery(schema, tbl, cols)
				return func() tea.Msg {
					return ScriptTableMsg{Query: query}
				}
			}

		case "copy":
			return func() tea.Msg {
				return ScriptTableMsg{Query: fmt.Sprintf("-- Selected: %s\n", curr.Name)}
			}

		case "refresh", "refresh_server", "refresh_db", "refresh_tbl":
			m.Refresh()
			return nil
		}
	} else {
		// File actions
		if len(m.FileFlattened) == 0 || m.FileCursor >= len(m.FileFlattened) {
			return nil
		}
		curr := m.FileFlattened[m.FileCursor]

		switch action {
		case "open_file":
			if curr.Type == db.NodeFileSQL || curr.Type == db.NodeFileOther {
				data, err := os.ReadFile(curr.FilePath)
				if err == nil {
					content := string(data)
					path := curr.FilePath
					name := curr.Name
					return func() tea.Msg {
						return OpenFileMsg{
							FilePath: path,
							Content:  content,
							FileName: name,
						}
					}
				}
			}

		case "copy_path":
			return func() tea.Msg {
				return ScriptTableMsg{Query: fmt.Sprintf("-- File: %s\n", curr.FilePath)}
			}

		case "refresh_files":
			m.RefreshFiles()
			return nil
		}
	}

	return nil
}

func (m Model) HandleMouse(msg tea.MouseMsg, relY int) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	// 1. Clicked on Header Tabs
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && relY == 0 {
		mid := m.Width / 2
		if msg.X < mid {
			m.ActiveTab = TabDatabases
		} else {
			m.ActiveTab = TabFiles
		}
		return m, nil
	}

	// 2. Context menu clicks
	if m.ContextMenuOpen && len(m.MenuItems) > 0 {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			menuRelY := relY - m.ContextMenuRelY
			if menuRelY >= 0 && menuRelY < len(m.MenuItems) {
				return m, m.executeContextMenuAction(m.MenuItems[menuRelY].Action)
			}
			m.ContextMenuOpen = false
			return m, nil
		}
	}

	// 3. Right-click context menu
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
		listLen := len(m.Flattened)
		if m.ActiveTab == TabFiles {
			listLen = len(m.FileFlattened)
		}
		itemIdx := relY - 1
		if itemIdx >= 0 && itemIdx < listLen {
			if m.ActiveTab == TabDatabases {
				m.Cursor = itemIdx
			} else {
				m.FileCursor = itemIdx
			}
			m.openContextMenu(relY)
			return m, nil
		}
	}

	// 4. Scroll Wheel
	if msg.Button == tea.MouseButtonWheelUp {
		if m.ActiveTab == TabDatabases {
			if m.Cursor > 0 {
				m.Cursor--
			}
		} else {
			if m.FileCursor > 0 {
				m.FileCursor--
			}
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		if m.ActiveTab == TabDatabases {
			if m.Cursor < len(m.Flattened)-1 {
				m.Cursor++
			}
		} else {
			if m.FileCursor < len(m.FileFlattened)-1 {
				m.FileCursor++
			}
		}
		return m, nil
	}

	// 5. Left-click item
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		itemIdx := relY - 1

		if m.ActiveTab == TabDatabases {
			if itemIdx >= 0 && itemIdx < len(m.Flattened) {
				m.Cursor = itemIdx
				curr := m.Flattened[itemIdx]

				if curr.Type == db.NodeServer && !curr.Connected {
					if prof := m.findProfileByID(curr.ProfileID); prof != nil {
						return m, func() tea.Msg {
							return ConnectServerMsg{Profile: prof}
						}
					}
				}

				if curr.Type == db.NodeGroup {
					curr.Expanded = !curr.Expanded
					m.rebuildFlattened()
					return m, nil
				}

				if curr.Type == db.NodeServer {
					if curr.Connected {
						curr.Expanded = !curr.Expanded
						m.rebuildFlattened()
					}
					return m, nil
				}

				if curr.Type == db.NodeDatabase && !curr.Loaded {
					m.loadDatabaseChildren(curr, make(map[string]bool))
					curr.Expanded = true
				} else if curr.Type == db.NodeTable && !curr.Loaded {
					m.loadColumns(curr)
					curr.Expanded = true
				} else if len(curr.Children) > 0 {
					curr.Expanded = !curr.Expanded
				}
				m.rebuildFlattened()
				return m, nil
			}
		} else {
			// TabFiles Left Click
			if itemIdx >= 0 && itemIdx < len(m.FileFlattened) {
				m.FileCursor = itemIdx
				curr := m.FileFlattened[itemIdx]

				if curr.Type == db.NodeFileSQL || curr.Type == db.NodeFileOther {
					// Open file on click
					data, err := os.ReadFile(curr.FilePath)
					if err == nil {
						content := string(data)
						path := curr.FilePath
						name := curr.Name
						return m, func() tea.Msg {
							return OpenFileMsg{
								FilePath: path,
								Content:  content,
								FileName: name,
							}
						}
					}
				}

				if curr.Type == db.NodeFileDir {
					if !curr.Loaded {
						m.loadDirectoryChildren(curr, make(map[string]bool))
						curr.Expanded = true
					} else {
						curr.Expanded = !curr.Expanded
					}
					m.rebuildFileFlattened()
					return m, nil
				}
			}
		}
	}

	return m, nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.ContextMenuOpen {
			switch msg.String() {
			case "esc":
				m.ContextMenuOpen = false
				return m, nil
			case "up", "k":
				if m.ContextMenuCursor > 0 {
					m.ContextMenuCursor--
				}
				return m, nil
			case "down", "j":
				if m.ContextMenuCursor < len(m.MenuItems)-1 {
					m.ContextMenuCursor++
				}
				return m, nil
			case "enter":
				if len(m.MenuItems) > 0 && m.ContextMenuCursor < len(m.MenuItems) {
					return m, m.executeContextMenuAction(m.MenuItems[m.ContextMenuCursor].Action)
				}
			}
			return m, nil
		}

		if m.Filtering {
			switch msg.String() {
			case "enter", "esc":
				m.Filtering = false
				return m, nil
			case "backspace":
				if len(m.Filter) > 0 {
					m.Filter = m.Filter[:len(m.Filter)-1]
					if m.ActiveTab == TabDatabases {
						m.rebuildFlattened()
					} else {
						m.RefreshFiles()
					}
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.Filter += msg.String()
					if m.ActiveTab == TabDatabases {
						m.rebuildFlattened()
					} else {
						m.RefreshFiles()
					}
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "1":
			m.ActiveTab = TabDatabases
			return m, nil

		case "2":
			m.ActiveTab = TabFiles
			return m, nil

		case "tab":
			if m.ActiveTab == TabDatabases {
				m.ActiveTab = TabFiles
			} else {
				m.ActiveTab = TabDatabases
			}
			return m, nil

		case "up", "k":
			if m.ActiveTab == TabDatabases {
				if m.Cursor > 0 {
					m.Cursor--
				}
			} else {
				if m.FileCursor > 0 {
					m.FileCursor--
				}
			}
			return m, nil

		case "down", "j":
			if m.ActiveTab == TabDatabases {
				if m.Cursor < len(m.Flattened)-1 {
					m.Cursor++
				}
			} else {
				if m.FileCursor < len(m.FileFlattened)-1 {
					m.FileCursor++
				}
			}
			return m, nil

		case "right", "enter", "space":
			if m.ActiveTab == TabDatabases {
				if len(m.Flattened) > 0 && m.Cursor < len(m.Flattened) {
					curr := m.Flattened[m.Cursor]

					if curr.Type == db.NodeServer && !curr.Connected {
						if prof := m.findProfileByID(curr.ProfileID); prof != nil {
							return m, func() tea.Msg {
								return ConnectServerMsg{Profile: prof}
							}
						}
					}

					if curr.Type == db.NodeGroup {
						curr.Expanded = !curr.Expanded
						m.rebuildFlattened()
						return m, nil
					}

					if curr.Type == db.NodeServer && curr.Connected {
						curr.Expanded = !curr.Expanded
						m.rebuildFlattened()
						return m, nil
					}

					if curr.Type == db.NodeDatabase && !curr.Loaded {
						m.loadDatabaseChildren(curr, make(map[string]bool))
						curr.Expanded = true
					} else if curr.Type == db.NodeTable && !curr.Loaded {
						m.loadColumns(curr)
						curr.Expanded = true
					} else if len(curr.Children) > 0 {
						curr.Expanded = !curr.Expanded
					}
					m.rebuildFlattened()
				}
			} else {
				// TabFiles Enter/Space
				if len(m.FileFlattened) > 0 && m.FileCursor < len(m.FileFlattened) {
					curr := m.FileFlattened[m.FileCursor]
					if curr.Type == db.NodeFileSQL || curr.Type == db.NodeFileOther {
						data, err := os.ReadFile(curr.FilePath)
						if err == nil {
							content := string(data)
							path := curr.FilePath
							name := curr.Name
							return m, func() tea.Msg {
								return OpenFileMsg{
									FilePath: path,
									Content:  content,
									FileName: name,
								}
							}
						}
					}

					if curr.Type == db.NodeFileDir {
						if !curr.Loaded {
							m.loadDirectoryChildren(curr, make(map[string]bool))
							curr.Expanded = true
						} else {
							curr.Expanded = !curr.Expanded
						}
						m.rebuildFileFlattened()
						return m, nil
					}
				}
			}
			return m, nil

		case "left":
			if m.ActiveTab == TabDatabases {
				if len(m.Flattened) > 0 && m.Cursor < len(m.Flattened) {
					curr := m.Flattened[m.Cursor]
					if curr.Expanded {
						curr.Expanded = false
						m.rebuildFlattened()
					}
				}
			} else {
				if len(m.FileFlattened) > 0 && m.FileCursor < len(m.FileFlattened) {
					curr := m.FileFlattened[m.FileCursor]
					if curr.Expanded {
						curr.Expanded = false
						m.rebuildFileFlattened()
					}
				}
			}
			return m, nil

		case "m":
			curIdx := m.Cursor
			if m.ActiveTab == TabFiles {
				curIdx = m.FileCursor
			}
			m.openContextMenu(curIdx + 1)
			return m, nil

		case "s":
			if m.ActiveTab == TabDatabases {
				return m, m.executeContextMenuAction("select")
			}

		case "d":
			if m.ActiveTab == TabDatabases {
				return m, m.executeContextMenuAction("create")
			}

		case "i":
			if m.ActiveTab == TabDatabases {
				return m, m.executeContextMenuAction("insert")
			}

		case "r":
			if m.ActiveTab == TabDatabases {
				m.Refresh()
			} else {
				m.RefreshFiles()
			}
			return m, nil

		case "/":
			m.Filtering = true
			m.Filter = ""
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Render Tabs Header: [ 🗄 Databases (1) | 📁 SQL Files (2) ]
	tab1Style := theme.TabInactive
	tab2Style := theme.TabInactive
	if m.ActiveTab == TabDatabases {
		tab1Style = theme.TabActive
	} else {
		tab2Style = theme.TabActive
	}

	tab1 := tab1Style.Render("🗄 DBs [1]")
	tab2 := tab2Style.Render("📁 Files [2]")
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tab1, " ", tab2)

	if m.Filtering {
		b.WriteString(theme.PaneHeader.Render(fmt.Sprintf(" Filter: %s_", m.Filter)) + "\n")
	} else {
		b.WriteString(tabBar + "\n")
	}

	currentList := m.Flattened
	currentCursor := m.Cursor
	if m.ActiveTab == TabFiles {
		currentList = m.FileFlattened
		currentCursor = m.FileCursor
	}

	if len(currentList) == 0 {
		if m.ActiveTab == TabDatabases {
			b.WriteString(theme.StyleFgMuted.Render("  No servers/objects found.\n  Press 'r' to refresh."))
		} else {
			b.WriteString(theme.StyleFgMuted.Render("  No SQL files found in directory.\n  Press 'r' to refresh."))
		}
		return b.String()
	}

	startIdx := 0
	maxVisible := m.Height - 3
	if maxVisible < 5 {
		maxVisible = 5
	}

	if currentCursor >= maxVisible {
		startIdx = currentCursor - maxVisible + 1
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(currentList) {
		endIdx = len(currentList)
	}

	for i := startIdx; i < endIdx; i++ {
		node := currentList[i]
		isSel := (i == currentCursor)

		prefix := ""
		icon := ""
		driverBadge := ""

		switch node.Type {
		case db.NodeGroup:
			icon = "📁 "
			if node.Expanded {
				icon = "📂 "
			}
			prefix = ""

		case db.NodeServer:
			icon = "🖥 "
			switch strings.ToLower(node.DriverName) {
			case "mssql":
				driverBadge = "[MSSQL] "
			case "postgres", "postgresql":
				driverBadge = "[PG] "
			case "oracle":
				driverBadge = "[ORA] "
			}
			prefix = "  "

		case db.NodeDatabases:
			icon = "🗄 "
			prefix = "    "

		case db.NodeDatabase:
			icon = "📁 "
			if node.Expanded {
				icon = "📂 "
			}
			prefix = "      "

		case db.NodeFolderTables, db.NodeFolderViews, db.NodeFolderProcs:
			icon = "📁 "
			if node.Expanded {
				icon = "📂 "
			}
			prefix = "        "

		case db.NodeTable:
			icon = "📋 "
			prefix = "          "

		case db.NodeView:
			icon = "👁 "
			prefix = "          "

		case db.NodeProcedure:
			icon = "⚙️ "
			prefix = "          "

		case db.NodeColumn:
			if node.IsPrimaryKey {
				icon = "🔑 "
			} else {
				icon = "🔹 "
			}
			prefix = "            "

		case db.NodeFileDir:
			icon = "📁 "
			if node.Expanded {
				icon = "📂 "
			}
			prefix = ""

		case db.NodeFileSQL:
			icon = "📄 "
			prefix = "  "

		case db.NodeFileOther:
			icon = "📜 "
			prefix = "  "
		}

		expandIcon := ""
		if len(node.Children) > 0 || (node.Type == db.NodeDatabase && !node.Loaded) || (node.Type == db.NodeTable && !node.Loaded) || node.Type == db.NodeGroup || (node.Type == db.NodeFileDir && !node.Loaded) {
			if node.Expanded {
				expandIcon = "▼ "
			} else {
				expandIcon = "▶ "
			}
		} else {
			expandIcon = "  "
		}

		displayName := node.Name
		if node.Type == db.NodeServer {
			displayName = driverBadge + node.Name
			if node.Connected {
				displayName += " 🟢"
			} else {
				displayName += " ⚪"
			}
		}

		lineContent := fmt.Sprintf("%s%s%s%s", prefix, expandIcon, icon, displayName)
		if node.Type == db.NodeColumn && node.IsPrimaryKey {
			lineContent += " [PK]"
		}

		maxWidth := m.Width - 4
		if maxWidth > 0 && len(lineContent) > maxWidth {
			lineContent = lineContent[:maxWidth] + "…"
		}

		if isSel {
			lineContent = theme.TreeSelected.Width(m.Width - 2).Render(lineContent)
		} else {
			switch node.Type {
			case db.NodeGroup, db.NodeFileDir:
				lineContent = theme.TreeGroupNode.Render(lineContent)
			case db.NodeServer:
				lineContent = theme.TreeServerNode.Render(lineContent)
			case db.NodeDatabase:
				lineContent = theme.TreeDatabaseNode.Render(lineContent)
			case db.NodeFolderTables, db.NodeFolderViews, db.NodeFolderProcs:
				lineContent = theme.TreeFolderNode.Render(lineContent)
			case db.NodeTable, db.NodeView, db.NodeFileSQL:
				lineContent = theme.TreeTableNode.Render(lineContent)
			case db.NodeColumn, db.NodeFileOther:
				lineContent = theme.TreeColumnNode.Render(lineContent)
			}
		}

		b.WriteString(lineContent)
		b.WriteString("\n")
	}

	mainView := lipgloss.NewStyle().Width(m.Width).Height(m.Height).Render(b.String())

	if m.ContextMenuOpen && len(m.MenuItems) > 0 {
		var menuBuilder strings.Builder
		menuBuilder.WriteString(theme.PaneHeader.Render(" CONTEXT MENU ") + "\n")
		for i, item := range m.MenuItems {
			isSel := (i == m.ContextMenuCursor)
			line := "  " + item.Title
			if isSel {
				line = theme.TreeSelected.Render("▶ " + item.Title)
			}
			menuBuilder.WriteString(line + "\n")
		}

		menuBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorSecondary).
			Background(theme.ColorBgDark).
			Padding(0, 1).
			Width(28).
			Render(menuBuilder.String())

		return lipgloss.Place(
			m.Width,
			m.Height,
			lipgloss.Left,
			lipgloss.Center,
			menuBox,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	return mainView
}
