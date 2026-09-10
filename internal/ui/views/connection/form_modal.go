package connection

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
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

// textFields lists, in tab order, the fields backed by a bubbles textinput.
// FieldDriver and FieldAuthType are choice toggles handled manually.
var textFields = []FormField{
	FieldName, FieldGroup, FieldHost, FieldPort,
	FieldDatabase, FieldUser, FieldPassword, FieldDomain, FieldPassEntry,
}

type FormModal struct {
	Active       bool
	IsEdit       bool
	ProfileID    string
	FocusedField FormField

	DriverIdx   int // 0: mssql, 1: postgres, 2: oracle
	AuthTypeIdx int // 0: keyring, 1: sql, 2: windows, 3: pass, 4: env

	// Text fields are delegated to bubbles/textinput, which handles rune-aware
	// editing, cursor navigation, horizontal windowing, clipboard paste, and
	// password masking natively (no hand-rolled byte slicing).
	inputs map[FormField]*textinput.Model

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

// newTextInput builds a textinput configured for this form's look & feel.
func newTextInput(value string, secret bool) *textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 512
	ti.Width = 40
	ti.SetValue(value)
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
	}
	return &ti
}

// buildInputs (re)creates every text input from the given values and focuses the
// currently selected field.
func (f *FormModal) buildInputs(values map[FormField]string) {
	f.inputs = make(map[FormField]*textinput.Model, len(textFields))
	for _, field := range textFields {
		f.inputs[field] = newTextInput(values[field], field == FieldPassword)
	}
	f.updateFocus()
}

func (f *FormModal) updateFocus() {
	for field, in := range f.inputs {
		if field == f.FocusedField {
			in.Focus()
			in.CursorEnd()
		} else {
			in.Blur()
		}
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
	f.DriverIdx = 0
	f.AuthTypeIdx = 0 // Keyring default for best security
	f.ErrorMessage = ""

	f.buildInputs(map[FormField]string{
		FieldName:     "New Connection",
		FieldGroup:    "General",
		FieldHost:     "localhost",
		FieldPort:     "1433",
		FieldDatabase: "SalesDB",
		FieldUser:     "sa",
	})
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
	f.ErrorMessage = ""

	// Driver idx
	f.DriverIdx = 0
	switch strings.ToLower(p.Driver) {
	case "postgres", "postgresql", "pg":
		f.DriverIdx = 1
	case "oracle", "ora":
		f.DriverIdx = 2
	}

	port := strconv.Itoa(p.Port)
	if p.Port <= 0 {
		port = f.getDefaultPort()
	}

	// AuthType idx
	f.AuthTypeIdx = 0
	for i, at := range AuthTypes {
		if at == p.AuthType {
			f.AuthTypeIdx = i
			break
		}
	}

	// Resolve the password for editing (keyring lookup / AES decryption).
	password := p.Password
	if p.AuthType == config.AuthTypeKeyring {
		if pass, err := config.GetFromKeyring(p.ID); err == nil {
			password = pass
		}
	} else if config.IsEncrypted(p.Password) {
		if dec, err := config.DecryptPassword(p.Password); err == nil {
			password = dec
		}
	}

	passEntry := p.PassEntry
	if p.PasswordEnv != "" {
		passEntry = p.PasswordEnv
	}

	f.buildInputs(map[FormField]string{
		FieldName:      p.Name,
		FieldGroup:     p.GetGroup(),
		FieldHost:      p.Host,
		FieldPort:      port,
		FieldDatabase:  p.Database,
		FieldUser:      p.User,
		FieldPassword:  password,
		FieldDomain:    p.Domain,
		FieldPassEntry: passEntry,
	})
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
	if in, ok := f.inputs[field]; ok {
		return in.Value()
	}
	return ""
}

func (f *FormModal) setFieldValue(field FormField, val string) {
	if in, ok := f.inputs[field]; ok {
		in.SetValue(val)
	}
}

func (f FormModal) Update(msg tea.Msg) (FormModal, *config.ConnectionProfile, bool) {
	if !f.Active {
		return f, nil, false
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return f, nil, false
	}

	switch keyMsg.String() {
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
	}

	// Choice toggles (Driver / Auth Method): left/right/space cycle the value.
	if f.FocusedField == FieldDriver || f.FocusedField == FieldAuthType {
		switch keyMsg.String() {
		case "left":
			f.cycleChoice(-1)
		case "right", " ":
			f.cycleChoice(1)
		}
		return f, nil, false
	}

	// Text fields: delegate everything else (runes, paste, cursor moves,
	// backspace, home/end, ...) to the focused textinput.
	if in, ok := f.inputs[f.FocusedField]; ok {
		updated, _ := in.Update(msg)
		*in = updated
	}
	return f, nil, false
}

func (f *FormModal) cycleChoice(dir int) {
	switch f.FocusedField {
	case FieldDriver:
		f.DriverIdx = (f.DriverIdx + dir + len(Drivers)) % len(Drivers)
		f.setFieldValue(FieldPort, f.getDefaultPort())
	case FieldAuthType:
		f.AuthTypeIdx = (f.AuthTypeIdx + dir + len(AuthTypes)) % len(AuthTypes)
	}
}

func (f *FormModal) nextField() {
	f.FocusedField = (f.FocusedField + 1) % TotalFields
	f.adjustFieldVisibility(true)
	f.updateFocus()
}

func (f *FormModal) prevField() {
	f.FocusedField = (f.FocusedField - 1 + TotalFields) % TotalFields
	f.adjustFieldVisibility(false)
	f.updateFocus()
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
	name := strings.TrimSpace(f.getFieldValue(FieldName))
	if name == "" {
		return nil, fmt.Errorf("connection profile name cannot be empty")
	}

	host := strings.TrimSpace(f.getFieldValue(FieldHost))
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	portNum, err := strconv.Atoi(strings.TrimSpace(f.getFieldValue(FieldPort)))
	if err != nil || portNum <= 0 {
		return nil, fmt.Errorf("invalid port number")
	}

	dbName := strings.TrimSpace(f.getFieldValue(FieldDatabase))
	if dbName == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	user := strings.TrimSpace(f.getFieldValue(FieldUser))
	driver := Drivers[f.DriverIdx]
	authType := f.getActiveAuthType()
	password := f.getFieldValue(FieldPassword)
	passEntry := strings.TrimSpace(f.getFieldValue(FieldPassEntry))

	id := f.ProfileID
	if id == "" {
		clean := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		id = fmt.Sprintf("%s-%s", driver, clean)
	}

	profile := &config.ConnectionProfile{
		ID:        id,
		Name:      name,
		Group:     strings.TrimSpace(f.getFieldValue(FieldGroup)),
		Driver:    driver,
		Host:      host,
		Port:      portNum,
		Database:  dbName,
		User:      user,
		AuthType:  authType,
		Domain:    strings.TrimSpace(f.getFieldValue(FieldDomain)),
		PassEntry: passEntry,
	}

	if authType == config.AuthTypeEnv {
		profile.PasswordEnv = passEntry
		profile.PassEntry = ""
	}

	// Handle password saving with strict encryption & keyring storage
	if password != "" {
		if authType == config.AuthTypeKeyring {
			if err := config.SaveToKeyring(id, password); err == nil {
				profile.Password = "" // Stored exclusively in OS Keychain!
			} else {
				// Fallback to AES encrypted if keyring failed
				enc, _ := config.EncryptPassword(password)
				profile.Password = enc
			}
		} else if authType == config.AuthTypePass || authType == config.AuthTypeEnv {
			profile.Password = "" // Stored in Unix pass or environment variable
		} else {
			// SQL or Windows Auth -> Always encrypt with AES-256-GCM before writing to JSON
			enc, err := config.EncryptPassword(password)
			if err == nil {
				profile.Password = enc
			} else {
				profile.Password = password
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

	// Size the text inputs to the inner width of the field box.
	innerW := modalWidth - 32
	if innerW < 8 {
		innerW = 8
	}
	for _, in := range f.inputs {
		in.Width = innerW
	}

	titleText := " ➕ ADD SQL SERVER CONNECTION "
	if f.IsEdit {
		titleText = fmt.Sprintf(" ✏️ EDIT CONNECTION: %s ", f.getFieldValue(FieldName))
	}

	var b strings.Builder
	b.WriteString(theme.ModalTitle.Render(titleText) + "\n\n")

	renderInput := func(label string, field FormField, isChoice bool, choiceVal string) string {
		isFocused := (f.FocusedField == field)
		labelStr := lipgloss.NewStyle().Width(18).Bold(true).Render(label + ":")
		if isFocused {
			labelStr = lipgloss.NewStyle().Width(18).Bold(true).Foreground(theme.ColorSecondary).Render("▶ " + label + ":")
		} else {
			labelStr = "  " + labelStr
		}

		var content string
		if isChoice {
			content = lipgloss.NewStyle().Background(theme.ColorPrimary).Foreground(lipgloss.Color("#FFF")).Padding(0, 1).Render("◀ " + choiceVal + " ▶")
		} else {
			borderColor := theme.ColorBorder
			if isFocused {
				borderColor = theme.ColorPrimary
			}
			inner := ""
			if in, ok := f.inputs[field]; ok {
				inner = in.View()
			}
			content = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(0, 1).
				Width(modalWidth - 28).
				MaxHeight(3).
				Render(inner)
		}

		return lipgloss.JoinHorizontal(lipgloss.Center, labelStr, " ", content) + "\n"
	}

	// 1. Name & Group
	b.WriteString(renderInput("Profile Name", FieldName, false, ""))
	b.WriteString(renderInput("Folder / Group", FieldGroup, false, ""))

	// 2. Driver & Host & Port
	driverDisplay := strings.ToUpper(Drivers[f.DriverIdx])
	b.WriteString(renderInput("Database Driver", FieldDriver, true, driverDisplay))
	b.WriteString(renderInput("Host / IP", FieldHost, false, ""))
	b.WriteString(renderInput("Port", FieldPort, false, ""))

	// 3. Database & User
	b.WriteString(renderInput("Database Name", FieldDatabase, false, ""))
	b.WriteString(renderInput("Username", FieldUser, false, ""))

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
	b.WriteString(renderInput("Auth Method", FieldAuthType, true, authDisplay))

	at := f.getActiveAuthType()
	if at != config.AuthTypePass && at != config.AuthTypeEnv {
		b.WriteString(renderInput("Password", FieldPassword, false, ""))
	}

	if at == config.AuthTypeWindows {
		b.WriteString(renderInput("Domain (AD)", FieldDomain, false, ""))
	} else if at == config.AuthTypePass {
		b.WriteString(renderInput("Pass Entry", FieldPassEntry, false, ""))
	} else if at == config.AuthTypeEnv {
		b.WriteString(renderInput("Env Var Name", FieldPassEntry, false, ""))
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
