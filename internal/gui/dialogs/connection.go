package dialogs

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dbterm/internal/config"
	"dbterm/internal/db"
)

// ConnectionDialog presents a modal dialog for managing database connections
type ConnectionDialog struct {
	window       fyne.Window
	cfg          *config.Config
	cfgPath      string
	onConnect    func(profile config.ConnectionProfile)
	onSaveConfig func()
}

func ShowConnectionDialog(w fyne.Window, cfg *config.Config, cfgPath string, onConnect func(config.ConnectionProfile), onSaveConfig func()) {
	cd := &ConnectionDialog{
		window:       w,
		cfg:          cfg,
		cfgPath:      cfgPath,
		onConnect:    onConnect,
		onSaveConfig: onSaveConfig,
	}
	cd.show()
}

func (cd *ConnectionDialog) show() {
	if cd.cfg == nil {
		cd.cfg = &config.Config{Connections: []config.ConnectionProfile{}}
	}

	list := widget.NewList(
		func() int {
			return len(cd.cfg.Connections)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.StorageIcon()),
				widget.NewLabel("Template Server Connection"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= 0 && id < len(cd.cfg.Connections) {
				c := cd.cfg.Connections[id]
				box := item.(*fyne.Container)
				label := box.Objects[1].(*widget.Label)
				label.SetText(fmt.Sprintf("%s  (%s://%s:%d/%s)", c.Name, c.Driver, c.Host, c.Port, c.Database))
			}
		},
	)

	var selectedIndex = -1
	list.OnSelected = func(id widget.ListItemID) {
		selectedIndex = id
	}

	var d dialog.Dialog

	connectBtn := widget.NewButtonWithIcon("Connect", theme.NavigateNextIcon(), func() {
		if selectedIndex >= 0 && selectedIndex < len(cd.cfg.Connections) {
			p := cd.cfg.Connections[selectedIndex]
			d.Hide()
			if cd.onConnect != nil {
				cd.onConnect(p)
			}
		} else {
			dialog.ShowInformation("Select Connection", "Please select a connection profile to connect.", cd.window)
		}
	})
	connectBtn.Importance = widget.HighImportance

	addBtn := widget.NewButtonWithIcon("Add New", theme.ContentAddIcon(), func() {
		cd.showForm(nil, func(newProfile config.ConnectionProfile) {
			cd.cfg.Connections = append(cd.cfg.Connections, newProfile)
			_ = config.SaveConfig(cd.cfg, cd.cfgPath)
			if cd.onSaveConfig != nil {
				cd.onSaveConfig()
			}
			list.Refresh()
		})
	})

	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if selectedIndex >= 0 && selectedIndex < len(cd.cfg.Connections) {
			p := cd.cfg.Connections[selectedIndex]
			cd.showForm(&p, func(updatedProfile config.ConnectionProfile) {
				cd.cfg.Connections[selectedIndex] = updatedProfile
				_ = config.SaveConfig(cd.cfg, cd.cfgPath)
				if cd.onSaveConfig != nil {
					cd.onSaveConfig()
				}
				list.Refresh()
			})
		} else {
			dialog.ShowInformation("Select Connection", "Please select a connection profile to edit.", cd.window)
		}
	})

	delBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if selectedIndex >= 0 && selectedIndex < len(cd.cfg.Connections) {
			p := cd.cfg.Connections[selectedIndex]
			dialog.ShowConfirm("Delete Connection", fmt.Sprintf("Are you sure you want to delete '%s'?", p.Name), func(ok bool) {
				if ok {
					cd.cfg.Connections = append(cd.cfg.Connections[:selectedIndex], cd.cfg.Connections[selectedIndex+1:]...)
					_ = config.SaveConfig(cd.cfg, cd.cfgPath)
					if cd.onSaveConfig != nil {
						cd.onSaveConfig()
					}
					selectedIndex = -1
					list.Refresh()
				}
			}, cd.window)
		} else {
			dialog.ShowInformation("Select Connection", "Please select a connection profile to delete.", cd.window)
		}
	})

	testBtn := widget.NewButtonWithIcon("Test", theme.ConfirmIcon(), func() {
		if selectedIndex >= 0 && selectedIndex < len(cd.cfg.Connections) {
			p := cd.cfg.Connections[selectedIndex]
			go func() {
				drv, err := db.NewDriver(&p)
				if err != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("Driver init failed: %w", err), cd.window)
					})
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				if err := drv.Connect(ctx, &p); err != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("Connection failed: %w", err), cd.window)
					})
					return
				}
				_ = drv.Close()
				fyne.Do(func() {
					dialog.ShowInformation("Success", fmt.Sprintf("✓ Successfully connected to '%s'!", p.Name), cd.window)
				})
			}()
		}
	})

	toolbar := container.NewHBox(
		connectBtn,
		widget.NewSeparator(),
		addBtn,
		editBtn,
		delBtn,
		testBtn,
	)

	headerInfo := widget.NewLabel(fmt.Sprintf("Config File: %s", cd.cfgPath))
	headerInfo.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewBorder(
		headerInfo,
		toolbar,
		nil,
		nil,
		list,
	)

	d = dialog.NewCustom("Database Server Connections (Ctrl+O)", "Close", content, cd.window)
	d.Resize(fyne.NewSize(700, 450))
	d.Show()
}

func (cd *ConnectionDialog) showForm(existing *config.ConnectionProfile, onSaved func(config.ConnectionProfile)) {
	isEdit := (existing != nil)
	title := "Add New Database Connection"
	if isEdit {
		title = "Edit Database Connection"
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g. MS SQL Production Sales")

	driverSelect := widget.NewSelect([]string{"mssql", "postgres", "oracle"}, nil)
	driverSelect.SetSelected("mssql")

	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("localhost or db.corp.example.com")
	hostEntry.SetText("localhost")

	portEntry := widget.NewEntry()
	portEntry.SetText("1433")

	driverSelect.OnChanged = func(drv string) {
		switch drv {
		case "mssql":
			portEntry.SetText("1433")
		case "postgres":
			portEntry.SetText("5432")
		case "oracle":
			portEntry.SetText("1521")
		}
	}

	dbEntry := widget.NewEntry()
	dbEntry.SetPlaceHolder("Database / SID (e.g. SalesDB)")

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("Username (e.g. sa / postgres / admin)")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Password")

	authSelect := widget.NewSelect([]string{
		"keyring (OS Keychain / Vault)",
		"sql (AES-256 Encrypted in JSON)",
		"windows (Windows Domain Auth)",
		"pass (Unix password-store)",
		"env (Environment Variable)",
	}, nil)
	authSelect.SetSelected("keyring (OS Keychain / Vault)")

	folderEntry := widget.NewEntry()
	folderEntry.SetPlaceHolder("Group / Folder (e.g. Production / Local / Staging)")

	if isEdit {
		nameEntry.SetText(existing.Name)
		driverSelect.SetSelected(existing.Driver)
		hostEntry.SetText(existing.Host)
		portEntry.SetText(strconv.Itoa(existing.Port))
		dbEntry.SetText(existing.Database)
		userEntry.SetText(existing.User)
		folderEntry.SetText(existing.Folder)
		switch existing.AuthType {
		case config.AuthTypeKeyring:
			authSelect.SetSelected("keyring (OS Keychain / Vault)")
		case config.AuthTypeSQL:
			authSelect.SetSelected("sql (AES-256 Encrypted in JSON)")
		case config.AuthTypeWindows:
			authSelect.SetSelected("windows (Windows Domain Auth)")
		case config.AuthTypePass:
			authSelect.SetSelected("pass (Unix password-store)")
		case config.AuthTypeEnv:
			authSelect.SetSelected("env (Environment Variable)")
		}
	}

	form := widget.NewForm(
		widget.NewFormItem("Profile Name", nameEntry),
		widget.NewFormItem("Driver", driverSelect),
		widget.NewFormItem("Host / Server", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("Database Name", dbEntry),
		widget.NewFormItem("Username", userEntry),
		widget.NewFormItem("Password", passEntry),
		widget.NewFormItem("Auth Type", authSelect),
		widget.NewFormItem("Folder / Group", folderEntry),
	)

	dialog.ShowCustomConfirm(title, "Save Profile", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		port, _ := strconv.Atoi(portEntry.Text)
		if port <= 0 {
			port = 1433
		}

		id := ""
		if isEdit {
			id = existing.ID
		} else {
			id = fmt.Sprintf("%s-%s-%d", driverSelect.Selected, hostEntry.Text, time.Now().Unix())
		}

		var authType config.AuthType
		switch authSelect.Selected {
		case "keyring (OS Keychain / Vault)":
			authType = config.AuthTypeKeyring
		case "sql (AES-256 Encrypted in JSON)":
			authType = config.AuthTypeSQL
		case "windows (Windows Domain Auth)":
			authType = config.AuthTypeWindows
		case "pass (Unix password-store)":
			authType = config.AuthTypePass
		case "env (Environment Variable)":
			authType = config.AuthTypeEnv
		default:
			authType = config.AuthTypeKeyring
		}

		p := config.ConnectionProfile{
			ID:       id,
			Name:     nameEntry.Text,
			Driver:   driverSelect.Selected,
			Host:     hostEntry.Text,
			Port:     port,
			Database: dbEntry.Text,
			User:     userEntry.Text,
			AuthType: authType,
			Folder:   folderEntry.Text,
		}

		// Handle password security
		if passEntry.Text != "" {
			if authType == config.AuthTypeKeyring {
				_ = config.SaveToKeyring(p.ID, passEntry.Text)
				p.Password = ""
			} else if authType == config.AuthTypeSQL {
				enc, err := config.EncryptPassword(passEntry.Text)
				if err == nil {
					p.Password = enc
				} else {
					p.Password = passEntry.Text
				}
			}
		} else if isEdit {
			p.Password = existing.Password
		}

		if onSaved != nil {
			onSaved(p)
		}
	}, cd.window)
}
