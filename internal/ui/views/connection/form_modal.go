package connection

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbterm/internal/config"
	"dbterm/internal/ui/theme"
)

type FormField int

const (
	FieldName FormField = iota
	FieldGroup
	FieldDriver
	FieldHost
	FieldPort
	FieldDatabase
	FieldUser
	FieldAuthType
	FieldPassword
	FieldDomain
	FieldPassEntry
	FieldSaveButton
	FieldCancelButton
	TotalFields
)

type FormModal struct {
	Active       bool
	IsEdit       bool
	ProfileID    string
	FocusedField FormField

	Name        string
	Group       string
	DriverIdx   int // 0: mssql, 1: postgres, 2: oracle
	Host        string
	Port        string
	Database    string
	User        string
	AuthTypeIdx int // 0: keyring, 1: sql, 2: windows, 3: pass, 4: env
	Password    string
	Domain      string
	PassEntry   string

	CursorPos    int
	ErrorMessage string
	Width        int
	Height       int
}

var (
	Drivers   = []string{"mssql", "postgres", "oracle"}
	AuthTypes = []config.AuthType{
		config.AuthTypeKeyring,
		config.AuthTypeSQL,
		config.AuthTypeWindows,
		config.AuthTypePass,
		config.AuthTypeEnv,
	}
)

func NewFormModal() FormModal {
	return FormModal{
		Active: false,
	}
}

func (f *FormModal) SetSize(w, h int) {
	f.Width = w
	f.Height = h
}

func (f *FormModal) OpenNew() {
	f.Active = true
	f.IsEdit = false
	f.ProfileID = ""
	f.FocusedField = FieldName

	f.Name = "New Connection"
	f.Group = "General"
	f.DriverIdx = 0
	f.Host = "localhost"
	f.Port = "1433"
	f.Database = "SalesDB"
	f.User = "sa"
	f.AuthTypeIdx = 0 // Keyring default for best security
	f.Password = ""
	f.Domain = ""
	f.PassEntry = ""

	f.CursorPos = len(f.Name)
	f.ErrorMessage = ""
}

func (f *FormModal) OpenEdit(p *config.ConnectionProfile) {
	if p == nil {
		f.OpenNew()
		return
	}
	f.Active = true
	f.IsEdit = true
	f.ProfileID = p.ID
	f.FocusedField = FieldName

	f.Name = p.Name
	f.Group = p.GetGroup()

	// Driver idx
	f.DriverIdx = 0
	switch strings.ToLower(p.Driver) {
	case "postgres", "postgresql", "pg":
		f.DriverIdx = 1
	case "oracle", "ora":
		f.DriverIdx = 2
	}

	f.Host = p.Host
	if p.Port > 0 {
		f.Port = strconv.Itoa(p.Port)
	} else {
		f.Port = f.getDefaultPort()
	}
	f.Database = p.Database
	f.User = p.User

	// AuthType idx
	f.AuthTypeIdx = 0
	for i, at := range AuthTypes {
		if at == p.AuthType {
			f.AuthTypeIdx = i
			break
		}
	}

	f.Password = p.Password
	if p.AuthType == config.AuthTypeKeyring {
		if pass, err := config.GetFromKeyring(p.ID); err == nil {
			f.Password = pass
		}
	} else if config.IsEncrypted(p.Password) {
		if dec, err := config.DecryptPassword(p.Password); err == nil {
			f.Password = dec
		}
	}

	f.Domain = p.Domain
	f.PassEntry = p.PassEntry
	if p.PasswordEnv != "" {
		f.PassEntry = p.PasswordEnv
	}

	f.CursorPos = len(f.Name)
	f.ErrorMessage = ""
}

func (f *FormModal) Close() {
	f.Active = false
	f.ErrorMessage = ""
}

func (f *FormModal) getDefaultPort() string {
	switch f.DriverIdx {
	case 1:
		return "5432"
	case 2:
		return "1521"
	default:
		return "1433"
	}
}

func (f *FormModal) getActiveAuthType() config.AuthType {
	if f.AuthTypeIdx >= 0 && f.AuthTypeIdx < len(AuthTypes) {
		return AuthTypes[f.AuthTypeIdx]
	}
	return config.AuthTypeKeyring
}

func (f *FormModal) getFieldValue(field FormField) string {
	switch field {
	case FieldName:
		return f.Name
	case FieldGroup:
		return f.Group
	case FieldHost:
		return f.Host
	case FieldPort:
		return f.Port
	case FieldDatabase:
		return f.Database
	case FieldUser:
		return f.User
	case FieldPassword:
		return f.Password
	case FieldDomain:
		return f.Domain
	case FieldPassEntry:
		return f.PassEntry
	default:
		return ""
	}
}

func (f *FormModal) setFieldValue(field FormField, val string) {
	switch field {
	case FieldName:
		f.Name = val
	case FieldGroup:
		f.Group = val
	case FieldHost:
		f.Host = val
	case FieldPort:
		f.Port = val
	case FieldDatabase:
		f.Database = val
	case FieldUser:
		f.User = val
	case FieldPassword:
		f.Password = val
	case FieldDomain:
		f.Domain = val
	case FieldPassEntry:
		f.PassEntry = val
	}
}

func (f FormModal) Update(msg tea.Msg) (FormModal, *config.ConnectionProfile, bool) {
	if !f.Active {
		return f, nil, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			f.Close()
			return f, nil, false

		case "tab", "down":
			f.nextField()
			return f, nil, false

		case "shift+tab", "up":
			f.prevField()
			return f, nil, false

		case "enter":
			if f.FocusedField == FieldCancelButton {
				f.Close()
				return f, nil, false
			}
			if f.FocusedField == FieldSaveButton || f.FocusedField == FieldPassword {
				profile, err := f.validateAndBuildProfile()
				if err != nil {
					f.ErrorMessage = err.Error()
					return f, nil, false
				}
				f.Close()
				return f, profile, true
			}
			f.nextField()
			return f, nil, false

		case "left":
			if f.FocusedField == FieldDriver {
				f.DriverIdx = (f.DriverIdx - 1 + len(Drivers)) % len(Drivers)
				f.Port = f.getDefaultPort()
				return f, nil, false
			}
			if f.FocusedField == FieldAuthType {
				f.AuthTypeIdx = (f.AuthTypeIdx - 1 + len(AuthTypes)) % len(AuthTypes)
				return f, nil, false
			}
			if f.CursorPos > 0 {
				f.CursorPos--
			}
			return f, nil, false

		case "right", " ":
			if f.FocusedField == FieldDriver {
				f.DriverIdx = (f.DriverIdx + 1) % len(Drivers)
				f.Port = f.getDefaultPort()
				return f, nil, false
			}
			if f.FocusedField == FieldAuthType {
				f.AuthTypeIdx = (f.AuthTypeIdx + 1) % len(AuthTypes)
				return f, nil, false
			}
			val := f.getFieldValue(f.FocusedField)
			if msg.String() == "right" {
				if f.CursorPos < len(val) {
					f.CursorPos++
				}
			} else {
				// Space character insertion
				val = val[:f.CursorPos] + " " + val[f.CursorPos:]
				f.setFieldValue(f.FocusedField, val)
				f.CursorPos++
			}
			return f, nil, false

		case "backspace":
			val := f.getFieldValue(f.FocusedField)
			if len(val) > 0 && f.CursorPos > 0 {
				val = val[:f.CursorPos-1] + val[f.CursorPos:]
				f.setFieldValue(f.FocusedField, val)
				f.CursorPos--
			}
			return f, nil, false

		default:
			if len(msg.String()) == 1 {
				val := f.getFieldValue(f.FocusedField)
				val = val[:f.CursorPos] + msg.String() + val[f.CursorPos:]
				f.setFieldValue(f.FocusedField, val)
				f.CursorPos++
			}
			return f, nil, false
		}
	}

	return f, nil, false
}

func (f *FormModal) nextField() {
	f.FocusedField = (f.FocusedField + 1) % TotalFields
	f.adjustFieldVisibility(true)
	val := f.getFieldValue(f.FocusedField)
	f.CursorPos = len(val)
}

func (f *FormModal) prevField() {
	f.FocusedField = (f.FocusedField - 1 + TotalFields) % TotalFields
	f.adjustFieldVisibility(false)
	val := f.getFieldValue(f.FocusedField)
	f.CursorPos = len(val)
}

func (f *FormModal) adjustFieldVisibility(forward bool) {
	at := f.getActiveAuthType()

	if f.FocusedField == FieldDomain && at != config.AuthTypeWindows {
		if forward {
			f.FocusedField = FieldSaveButton
		} else {
			f.FocusedField = FieldPassword
		}
	}

	if f.FocusedField == FieldPassEntry && at != config.AuthTypePass && at != config.AuthTypeEnv {
		if forward {
			f.FocusedField = FieldSaveButton
		} else {
			f.FocusedField = FieldPassword
		}
	}
}

func (f *FormModal) validateAndBuildProfile() (*config.ConnectionProfile, error) {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return nil, fmt.Errorf("connection profile name cannot be empty")
	}

	host := strings.TrimSpace(f.Host)
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	portNum, err := strconv.Atoi(strings.TrimSpace(f.Port))
	if err != nil || portNum <= 0 {
		return nil, fmt.Errorf("invalid port number")
	}

	dbName := strings.TrimSpace(f.Database)
	if dbName == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	user := strings.TrimSpace(f.User)
	driver := Drivers[f.DriverIdx]
	authType := f.getActiveAuthType()

	id := f.ProfileID
	if id == "" {
		clean := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		id = fmt.Sprintf("%s-%s", driver, clean)
	}

	profile := &config.ConnectionProfile{
		ID:        id,
		Name:      name,
		Group:     strings.TrimSpace(f.Group),
		Driver:    driver,
		Host:      host,
		Port:      portNum,
		Database:  dbName,
		User:      user,
		AuthType:  authType,
		Domain:    strings.TrimSpace(f.Domain),
		PassEntry: strings.TrimSpace(f.PassEntry),
	}

	if authType == config.AuthTypeEnv {
		profile.PasswordEnv = strings.TrimSpace(f.PassEntry)
		profile.PassEntry = ""
	}

	// Handle password saving with strict encryption & keyring storage
	if f.Password != "" {
		if authType == config.AuthTypeKeyring {
			if err := config.SaveToKeyring(id, f.Password); err == nil {
				profile.Password = "" // Stored exclusively in OS Keychain!
			} else {
				// Fallback to AES encrypted if keyring failed
				enc, _ := config.EncryptPassword(f.Password)
				profile.Password = enc
			}
		} else if authType == config.AuthTypePass || authType == config.AuthTypeEnv {
			profile.Password = "" // Stored in Unix pass or environment variable
		} else {
			// SQL or Windows Auth -> Always encrypt with AES-256-GCM before writing to JSON
			enc, err := config.EncryptPassword(f.Password)
			if err == nil {
				profile.Password = enc
			} else {
				profile.Password = f.Password
			}
		}
	}

	return profile, nil
}

func (f FormModal) View() string {
	if !f.Active {
		return ""
	}

	modalWidth := 72
	if f.Width > 0 && modalWidth > f.Width-6 {
		modalWidth = f.Width - 6
	}

	titleText := " ➕ ADD SQL SERVER CONNECTION "
	if f.IsEdit {
		titleText = fmt.Sprintf(" ✏️ EDIT CONNECTION: %s ", f.Name)
	}

	var b strings.Builder
	b.WriteString(theme.ModalTitle.Render(titleText) + "\n\n")

	renderInput := func(label string, field FormField, isChoice bool, choiceVal string, isSecret bool) string {
		isFocused := (f.FocusedField == field)
		labelStr := lipgloss.NewStyle().Width(18).Bold(true).Render(label + ":")
		if isFocused {
			labelStr = lipgloss.NewStyle().Width(18).Bold(true).Foreground(theme.ColorSecondary).Render("▶ " + label + ":")
		} else {
			labelStr = "  " + labelStr
		}

		content := ""
		if isChoice {
			content = lipgloss.NewStyle().Background(theme.ColorPrimary).Foreground(lipgloss.Color("#FFF")).Padding(0, 1).Render("◀ " + choiceVal + " ▶")
		} else {
			val := f.getFieldValue(field)
			if isSecret && len(val) > 0 {
				val = strings.Repeat("•", len(val))
			}

			if isFocused {
				cursorChar := " "
				if f.CursorPos < len(val) {
					cursorChar = string(val[f.CursorPos])
				}
				styledCur := lipgloss.NewStyle().Background(theme.ColorPrimary).Foreground(lipgloss.Color("#FFF")).Render(cursorChar)
				if f.CursorPos < len(val) {
					content = val[:f.CursorPos] + styledCur + val[f.CursorPos+1:]
				} else {
					content = val + styledCur
				}
				content = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorPrimary).Padding(0, 1).Width(modalWidth - 28).Render(content)
			} else {
				if content == "" {
					content = val
				}
				content = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorBorder).Padding(0, 1).Width(modalWidth - 28).Render(content)
			}
		}

		return lipgloss.JoinHorizontal(lipgloss.Center, labelStr, " ", content) + "\n"
	}

	// 1. Name & Group
	b.WriteString(renderInput("Profile Name", FieldName, false, "", false))
	b.WriteString(renderInput("Folder / Group", FieldGroup, false, "", false))

	// 2. Driver & Host & Port
	driverDisplay := strings.ToUpper(Drivers[f.DriverIdx])
	b.WriteString(renderInput("Database Driver", FieldDriver, true, driverDisplay, false))
	b.WriteString(renderInput("Host / IP", FieldHost, false, "", false))
	b.WriteString(renderInput("Port", FieldPort, false, "", false))

	// 3. Database & User
	b.WriteString(renderInput("Database Name", FieldDatabase, false, "", false))
	b.WriteString(renderInput("Username", FieldUser, false, "", false))

	// 4. Auth & Password
	authDisplay := "Keychain (Secure OS Vault)"
	switch AuthTypes[f.AuthTypeIdx] {
	case config.AuthTypeSQL:
		authDisplay = "SQL (AES-256 Encrypted)"
	case config.AuthTypeWindows:
		authDisplay = "Windows Auth (AD/NTLM)"
	case config.AuthTypePass:
		authDisplay = "Unix 'pass' (GPG Store)"
	case config.AuthTypeEnv:
		authDisplay = "Environment Variable ($VAR)"
	}
	b.WriteString(renderInput("Auth Method", FieldAuthType, true, authDisplay, false))

	at := f.getActiveAuthType()
	if at != config.AuthTypePass && at != config.AuthTypeEnv {
		b.WriteString(renderInput("Password", FieldPassword, false, "", true))
	}

	if at == config.AuthTypeWindows {
		b.WriteString(renderInput("Domain (AD)", FieldDomain, false, "", false))
	} else if at == config.AuthTypePass {
		b.WriteString(renderInput("Pass Entry", FieldPassEntry, false, "", false))
	} else if at == config.AuthTypeEnv {
		b.WriteString(renderInput("Env Var Name", FieldPassEntry, false, "", false))
	}

	b.WriteString("\n")
	if f.ErrorMessage != "" {
		b.WriteString(theme.StatusBadgeError.Render(" "+f.ErrorMessage+" ") + "\n\n")
	}

	// Buttons
	saveStyle := theme.ButtonInactive
	cancelStyle := theme.ButtonInactive
	if f.FocusedField == FieldSaveButton {
		saveStyle = theme.ButtonActive
	} else if f.FocusedField == FieldCancelButton {
		cancelStyle = theme.ButtonActive
	}

	btnSave := saveStyle.Render(" [ Save & Store Connection ] ")
	btnCancel := cancelStyle.Render(" [ Cancel (Esc) ] ")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, "  ", btnSave, "   ", btnCancel) + "\n\n")

	b.WriteString(theme.StyleFgMuted.Render("  [Tab/Arrows: Navigate]  [Left/Right: Choice]  [Enter: Save]  [Esc: Cancel]"))

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.ColorPrimary).
		Background(theme.ColorBgDark).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
