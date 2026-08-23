package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// SSMS-inspired Color Palette
var (
	// Base Colors
	ColorBgDark     = lipgloss.Color("#1E1E1E")
	ColorBgLighter  = lipgloss.Color("#252526")
	ColorBgSelected = lipgloss.Color("#094771") // Classic VS / SSMS blue selection
	ColorFgLight    = lipgloss.Color("#D4D4D4")
	ColorFgMuted    = lipgloss.Color("#808080")
	ColorFgDim      = lipgloss.Color("#5A5A5A")

	// Accent Colors
	ColorPrimary   = lipgloss.Color("#007ACC") // SSMS Blue
	ColorSecondary = lipgloss.Color("#4EC9B0") // Teal / Schema
	ColorSuccess   = lipgloss.Color("#6A9955") // Green
	ColorWarning   = lipgloss.Color("#CE9178") // Orange / Strings
	ColorError     = lipgloss.Color("#F44747") // Red
	ColorSpecial   = lipgloss.Color("#C586C0") // Purple / Keywords
	ColorYellow    = lipgloss.Color("#DCDCAA") // Yellow / Functions
	ColorBorder    = lipgloss.Color("#3E3E42")
	ColorBorderAct = lipgloss.Color("#007ACC")
)

// Text Styles
var (
	StyleFgLight = lipgloss.NewStyle().Foreground(ColorFgLight)
	StyleFgMuted = lipgloss.NewStyle().Foreground(ColorFgMuted)
	StyleFgDim   = lipgloss.NewStyle().Foreground(ColorFgDim)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
)

// Styles for Panes
var (
	PaneBase = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PaneActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderAct).
			Padding(0, 1)

	PaneHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBgLighter).
			Padding(0, 1)

	PaneHeaderInactive = lipgloss.NewStyle().
				Foreground(ColorFgMuted).
				Background(ColorBgDark).
				Padding(0, 1)
)

// Header & Status Bar Styles
var (
	TopBar = lipgloss.NewStyle().
		Background(ColorBgLighter).
		Foreground(ColorFgLight).
		Padding(0, 1)

	TopBarBadge = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	TopBarDB = lipgloss.NewStyle().
			Background(ColorSecondary).
			Foreground(ColorBgDark).
			Bold(true).
			Padding(0, 1)

	StatusBar = lipgloss.NewStyle().
			Background(ColorBgLighter).
			Foreground(ColorFgLight).
			Padding(0, 1)

	StatusBadgeReady = lipgloss.NewStyle().
				Background(ColorSuccess).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	StatusBadgeExec = lipgloss.NewStyle().
			Background(ColorYellow).
			Foreground(ColorBgDark).
			Bold(true).
			Padding(0, 1)

	StatusBadgeError = lipgloss.NewStyle().
				Background(ColorError).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	StatusKey = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StatusVal = lipgloss.NewStyle().
			Foreground(ColorFgLight)
)

// Tree View Styles (Object Explorer)
var (
	TreeServerNode = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	TreeGroupNode = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	TreeDatabaseNode = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)

	TreeFolderNode = lipgloss.NewStyle().
			Foreground(ColorYellow)

	TreeTableNode = lipgloss.NewStyle().
			Foreground(ColorFgLight)

	TreeColumnNode = lipgloss.NewStyle().
			Foreground(ColorFgMuted)

	TreePKBadge = lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true)

	TreeSelected = lipgloss.NewStyle().
			Background(ColorBgSelected).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
)

// Tab Bar Styles
var (
	TabActive = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	TabInactive = lipgloss.NewStyle().
			Background(ColorBgLighter).
			Foreground(ColorFgMuted).
			Padding(0, 1)

	TabModified = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
)

// Table / Data Grid Styles
var (
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBgLighter).
			Padding(0, 1)

	TableCell = lipgloss.NewStyle().
			Foreground(ColorFgLight).
			Padding(0, 1)

	TableCellSelected = lipgloss.NewStyle().
				Background(ColorBgSelected).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	TableRowNum = lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Padding(0, 1)

	TableNull = lipgloss.NewStyle().
			Foreground(ColorFgMuted).
			Italic(true).
			Padding(0, 1)
)

// Modal Styles
var (
	ModalOverlay = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorBgDark).
			Padding(1, 2)

	ModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBgLighter).
			Padding(0, 1).
			MarginBottom(1)

	ButtonActive = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 2)

	ButtonInactive = lipgloss.NewStyle().
			Background(ColorBgLighter).
			Foreground(ColorFgMuted).
			Padding(0, 2)
)
