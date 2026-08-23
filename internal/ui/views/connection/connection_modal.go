package connection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/ui/theme"
)

// Messages emitted by Connection modal
type ConnectProfileMsg struct {
	Profile *config.ConnectionProfile
}

type ConfigUpdatedMsg struct {
	Config *config.Config
}

type NodeType int

const (
	NodeTypeFolder NodeType = iota
	NodeTypeConnection
)

type ConnTreeNode struct {
	Type        NodeType
	Name        string
	GroupPath   string
	Profile     *config.ConnectionProfile
	Depth       int
	Expanded    bool
	ServerCount int
	Children    []*ConnTreeNode
}

type Model struct {
	Config            *config.Config
	ConfigPath        string
	RootNodes         []*ConnTreeNode
	VisibleNodes      []*ConnTreeNode
	Cursor            int
	Width             int
	Height            int
	Active            bool
	Testing           bool
	TestResult        string
	TestResultErr     bool
	TestResultTime    time.Time
	Filter            string
	Filtering         bool
	FormModal         FormModal
	DeleteConfirmID   string
	DeleteConfirmName string
	lastClickIdx      int
	lastClickTime     time.Time
}

func New(cfg *config.Config, configPath string) Model {
	m := Model{
		Config:       cfg,
		ConfigPath:   configPath,
		FormModal:    NewFormModal(),
		Cursor:       0,
		Active:       false,
		lastClickIdx: -1,
	}
	m.rebuildTree()
	return m
}

func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	m.FormModal.SetSize(w, h)
}

func (m *Model) Open() {
	m.Active = true
	m.TestResult = ""
	m.Filter = ""
	m.Filtering = false
	m.DeleteConfirmID = ""
	m.DeleteConfirmName = ""
	m.FormModal.Close()
	m.rebuildTree()
}

func (m *Model) Close() {
	m.Active = false
	m.Filtering = false
	m.FormModal.Close()
	m.DeleteConfirmID = ""
}

func (m *Model) rebuildTree() {
	if m.Config == nil || len(m.Config.Connections) == 0 {
		m.RootNodes = nil
		m.VisibleNodes = nil
		m.Cursor = 0
		return
	}

	// Preserve expanded states
	expandedState := make(map[string]bool)
	for _, n := range m.VisibleNodes {
		if n.Type == NodeTypeFolder {
			expandedState[n.GroupPath] = n.Expanded
		}
	}

	// Group connections by folder
	groupMap := make(map[string][]*config.ConnectionProfile)
	var groupOrder []string

	for i := range m.Config.Connections {
		c := &m.Config.Connections[i]

		// Apply search filter if active
		if m.Filter != "" {
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

	var rootNodes []*ConnTreeNode
	for _, grpName := range groupOrder {
		profiles := groupMap[grpName]

		isExpanded := true
		if val, ok := expandedState[grpName]; ok {
			isExpanded = val
		}

		groupNode := &ConnTreeNode{
			Type:        NodeTypeFolder,
			Name:        grpName,
			GroupPath:   grpName,
			Depth:       0,
			Expanded:    isExpanded,
			ServerCount: len(profiles),
			Children:    make([]*ConnTreeNode, 0, len(profiles)),
		}

		for _, p := range profiles {
			connNode := &ConnTreeNode{
				Type:      NodeTypeConnection,
				Name:      p.Name,
				GroupPath: grpName,
				Profile:   p,
				Depth:     1,
			}
			groupNode.Children = append(groupNode.Children, connNode)
		}

		rootNodes = append(rootNodes, groupNode)
	}

	m.RootNodes = rootNodes
	m.rebuildVisible()
}

func (m *Model) rebuildVisible() {
	var visible []*ConnTreeNode
	for _, root := range m.RootNodes {
		visible = append(visible, root)
		if root.Expanded {
			visible = append(visible, root.Children...)
		}
	}
	m.VisibleNodes = visible

	if m.Cursor >= len(m.VisibleNodes) {
		m.Cursor = len(m.VisibleNodes) - 1
	}
	if m.Cursor < 0 && len(m.VisibleNodes) > 0 {
		m.Cursor = 0
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	// 1. Sub-modal (Form Modal) has precedence
	if m.FormModal.Active {
		var updatedForm FormModal
		var savedProfile *config.ConnectionProfile
		var saved bool

		updatedForm, savedProfile, saved = m.FormModal.Update(msg)
		m.FormModal = updatedForm

		if saved && savedProfile != nil {
			config.UpsertConnection(m.Config, *savedProfile)
			_ = config.SaveConfig(m.Config, m.ConfigPath)
			m.rebuildTree()
			m.TestResult = fmt.Sprintf("✓ Connection '%s' saved successfully!", savedProfile.Name)
			m.TestResultErr = false
			cfg := m.Config
			return m, func() tea.Msg {
				return ConfigUpdatedMsg{Config: cfg}
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			modalWidth := 78
			if m.Width > 0 && modalWidth > m.Width-6 {
				modalWidth = m.Width - 6
			}
			headerOffset := 4 // modal title + subtitle + spacing

			clickRow := msg.Y - ((m.Height - 20) / 2) - headerOffset
			if clickRow >= 0 && clickRow < len(m.VisibleNodes) {
				m.Cursor = clickRow
				node := m.VisibleNodes[clickRow]

				if node.Type == NodeTypeFolder {
					node.Expanded = !node.Expanded
					m.rebuildVisible()
					return m, nil
				}

				// Double-click on server connection to connect immediately
				now := time.Now()
				if m.lastClickIdx == clickRow && now.Sub(m.lastClickTime) < 400*time.Millisecond {
					m.Close()
					profile := node.Profile
					return m, func() tea.Msg {
						return ConnectProfileMsg{Profile: profile}
					}
				}
				m.lastClickIdx = clickRow
				m.lastClickTime = now
				return m, nil
			}
		}

	case tea.KeyMsg:
		// Delete confirmation prompt
		if m.DeleteConfirmID != "" {
			switch msg.String() {
			case "y", "Y":
				idToDelete := m.DeleteConfirmID
				nameDeleted := m.DeleteConfirmName
				m.DeleteConfirmID = ""
				m.DeleteConfirmName = ""
				config.DeleteConnection(m.Config, idToDelete)
				_ = config.DeleteFromKeyring(idToDelete)
				_ = config.SaveConfig(m.Config, m.ConfigPath)
				m.rebuildTree()
				m.TestResult = fmt.Sprintf("✓ Deleted connection '%s'.", nameDeleted)
				m.TestResultErr = false
				cfg := m.Config
				return m, func() tea.Msg {
					return ConfigUpdatedMsg{Config: cfg}
				}
			case "n", "N", "esc":
				m.DeleteConfirmID = ""
				m.DeleteConfirmName = ""
				m.TestResult = "Deletion cancelled."
				m.TestResultErr = false
				return m, nil
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
					m.rebuildTree()
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.Filter += msg.String()
					m.rebuildTree()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "esc", "ctrl+o":
			m.Close()
			return m, nil

		case "/":
			m.Filtering = true
			m.Filter = ""
			m.rebuildTree()
			return m, nil

		case "a", "n", "A", "N": // Add New Connection Profile
			m.FormModal.OpenNew()
			return m, nil

		case "e", "E": // Edit Selected Connection Profile
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeConnection && node.Profile != nil {
					m.FormModal.OpenEdit(node.Profile)
					return m, nil
				}
			}

		case "d", "D", "delete", "x": // Delete Selected Connection Profile
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeConnection && node.Profile != nil {
					m.DeleteConfirmID = node.Profile.ID
					m.DeleteConfirmName = node.Profile.Name
					return m, nil
				}
			}

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
				m.TestResult = ""
			}
			return m, nil

		case "down", "j":
			if m.Cursor < len(m.VisibleNodes)-1 {
				m.Cursor++
				m.TestResult = ""
			}
			return m, nil

		case " ", "left", "right", "h", "l":
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeFolder {
					if msg.String() == "left" || msg.String() == "h" {
						node.Expanded = false
					} else if msg.String() == "right" || msg.String() == "l" {
						node.Expanded = true
					} else {
						node.Expanded = !node.Expanded
					}
					m.rebuildVisible()
					return m, nil
				}
			}

		case "enter":
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeFolder {
					node.Expanded = !node.Expanded
					m.rebuildVisible()
					return m, nil
				}

				// Connection node -> Connect!
				selected := node.Profile
				m.Close()
				return m, func() tea.Msg {
					return ConnectProfileMsg{Profile: selected}
				}
			}

		case "t": // Test connection
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeFolder {
					m.TestResult = "Select a specific server to test connection."
					m.TestResultErr = false
					return m, nil
				}

				selected := node.Profile
				m.Testing = true
				m.TestResult = "Testing connection..."
				m.TestResultErr = false
				m.TestResultTime = time.Now()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				driver, err := db.NewDriver(selected)
				if err != nil {
					m.Testing = false
					m.TestResult = fmt.Sprintf("Driver Error: %v", err)
					m.TestResultErr = true
					return m, nil
				}

				if err := driver.Connect(ctx, selected); err != nil {
					m.Testing = false
					m.TestResult = fmt.Sprintf("Connection Failed: %v", err)
					m.TestResultErr = true
					return m, nil
				}
				_ = driver.Close()
				m.Testing = false
				m.TestResult = "✓ Connection Successful!"
				m.TestResultErr = false
				return m, nil
			}

		case "s": // Secure password to OS Keychain
			if len(m.VisibleNodes) > 0 && m.Cursor < len(m.VisibleNodes) {
				node := m.VisibleNodes[m.Cursor]
				if node.Type == NodeTypeFolder {
					m.TestResult = "Select a database server to secure password."
					m.TestResultErr = false
					return m, nil
				}

				selected := node.Profile
				pass, err := selected.ResolvePassword(context.Background())
				if err != nil || pass == "" {
					m.TestResult = "No password found to store in Keychain."
					m.TestResultErr = true
					return m, nil
				}
				if err := config.SaveToKeyring(selected.ID, pass); err != nil {
					m.TestResult = fmt.Sprintf("Keychain error: %v", err)
					m.TestResultErr = true
					return m, nil
				}
				selected.AuthType = config.AuthTypeKeyring
				selected.Password = "" // Strip plaintext password from JSON
				if err := config.SaveConfig(m.Config, m.ConfigPath); err != nil {
					m.TestResult = fmt.Sprintf("Saved to Keychain, but config save failed: %v", err)
					m.TestResultErr = true
					return m, nil
				}
				m.TestResult = "✓ Password moved to OS Keychain (stripped from JSON)!"
				m.TestResultErr = false
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if !m.Active {
		return ""
	}

	// Sub-modal (Form Modal) rendered on top
	if m.FormModal.Active {
		return m.FormModal.View()
	}

	modalWidth := 78
	if m.Width > 0 && modalWidth > m.Width-6 {
		modalWidth = m.Width - 6
	}

	var b strings.Builder
	title := theme.ModalTitle.Render(" 🗄 REGISTERED DATABASE SERVERS ")
	filterBadge := ""
	if m.Filtering {
		filterBadge = theme.TopBarBadge.Render(fmt.Sprintf(" Filter: %s_ ", m.Filter))
	} else if m.Filter != "" {
		filterBadge = theme.TopBarBadge.Render(fmt.Sprintf(" Filter: [%s] ", m.Filter))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", filterBadge) + "\n\n")

	if m.Config == nil || len(m.Config.Connections) == 0 {
		b.WriteString(theme.StyleError.Render("No connections defined in configuration.\nPress 'a' to add a new connection profile.") + "\n\n")
	} else if len(m.VisibleNodes) == 0 {
		b.WriteString(theme.StyleFgMuted.Render(fmt.Sprintf("No servers match filter '%s'. Press / to change filter.", m.Filter)) + "\n\n")
	} else {
		for i, node := range m.VisibleNodes {
			isSel := (i == m.Cursor)

			if node.Type == NodeTypeFolder {
				// Folder / Group Node
				icon := "▼ 📁 "
				if !node.Expanded {
					icon = "▶ 📁 "
				}
				folderName := fmt.Sprintf("%s%s (%d)", icon, node.Name, node.ServerCount)
				line := theme.PaneHeader.Render(folderName)

				if isSel {
					line = theme.TreeSelected.Width(modalWidth - 6).Render("▶ " + folderName)
				} else {
					line = "  " + line
				}
				b.WriteString(line + "\n")
			} else {
				// Connection Profile Node
				c := node.Profile
				driverBadge := ""
				switch strings.ToLower(c.Driver) {
				case "mssql":
					driverBadge = lipgloss.NewStyle().Background(theme.ColorPrimary).Foreground(lipgloss.Color("#FFF")).Bold(true).Render(" MSSQL ")
				case "postgres", "postgresql":
					driverBadge = lipgloss.NewStyle().Background(lipgloss.Color("#336791")).Foreground(lipgloss.Color("#FFF")).Bold(true).Render(" PG ")
				case "oracle":
					driverBadge = lipgloss.NewStyle().Background(lipgloss.Color("#F80000")).Foreground(lipgloss.Color("#FFF")).Bold(true).Render(" ORA ")
				default:
					driverBadge = lipgloss.NewStyle().Background(theme.ColorFgMuted).Foreground(lipgloss.Color("#000")).Render(" " + c.Driver + " ")
				}

				authInfo := "SQL"
				if c.AuthType == config.AuthTypeKeyring {
					authInfo = "Keychain"
				} else if config.IsEncrypted(c.Password) {
					authInfo = "AES-256"
				} else if c.AuthType == config.AuthTypeWindows {
					authInfo = "Windows"
				} else if c.AuthType == config.AuthTypePass || c.PassEntry != "" {
					authInfo = "Pass GPG"
				} else if c.AuthType == config.AuthTypeEnv || c.PasswordEnv != "" {
					authInfo = "$" + c.PasswordEnv
				} else if c.Password != "" {
					authInfo = "Plaintext"
				}

				serverStr := fmt.Sprintf("  🖥  %s %s", driverBadge, c.Name)
				metaStr := fmt.Sprintf("      %s@%s:%d/%s | Auth: %s", c.User, c.Host, c.Port, c.Database, authInfo)

				if isSel {
					serverLine := theme.TreeSelected.Width(modalWidth - 6).Render("▶" + serverStr)
					metaLine := theme.TreeSelected.Width(modalWidth - 6).Render(metaStr)
					b.WriteString(serverLine + "\n" + metaLine + "\n")
				} else {
					serverLine := " " + serverStr
					metaLine := theme.StyleFgMuted.Render(metaStr)
					b.WriteString(serverLine + "\n" + metaLine + "\n")
				}
			}
		}
	}

	b.WriteString("\n")

	// Delete confirmation prompt bar
	if m.DeleteConfirmID != "" {
		warnText := fmt.Sprintf(" ⚠️ Delete connection '%s'? Press 'y' to confirm, 'n' to cancel ", m.DeleteConfirmName)
		b.WriteString(theme.StatusBadgeError.Render(warnText) + "\n\n")
	} else if m.TestResult != "" {
		if m.TestResultErr {
			b.WriteString(theme.StatusBadgeError.Render(" "+m.TestResult+" ") + "\n\n")
		} else {
			b.WriteString(theme.StatusBadgeReady.Render(" "+m.TestResult+" ") + "\n\n")
		}
	}

	footer := lipgloss.JoinHorizontal(
		lipgloss.Top,
		theme.ButtonActive.Render("a: Add"),
		" ",
		theme.ButtonActive.Render("e: Edit"),
		" ",
		theme.ButtonInactive.Render("d: Delete"),
		" ",
		theme.ButtonInactive.Render("t: Test"),
		" ",
		theme.ButtonInactive.Render("Enter: Connect"),
		" ",
		theme.ButtonInactive.Render("/: Filter"),
		" ",
		theme.ButtonInactive.Render("Esc: Close"),
	)
	b.WriteString(footer)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
