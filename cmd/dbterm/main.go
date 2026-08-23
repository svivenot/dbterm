package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"dbterm/internal/config"
	"dbterm/internal/ui"
)

func main() {
	configPath := flag.String("config", "", "Path to connections.json")
	profileID := flag.String("profile", "", "Connection profile ID to connect to on startup")
	driverFlag := flag.String("driver", "", "Database driver (mssql, postgres, oracle)")
	hostFlag := flag.String("host", "", "Database host")
	portFlag := flag.Int("port", 0, "Database port")
	dbFlag := flag.String("db", "", "Database name")
	userFlag := flag.String("user", "", "Database username")
	authFlag := flag.String("auth", "", "Authentication type (sql, keyring, windows, pass, env)")
	passEntryFlag := flag.String("pass-entry", "", "Password store pass entry (e.g. databases/mssql/sa)")
	secureFlag := flag.Bool("secure-passwords", false, "Migrate all passwords to OS Keychain/Keyring and strip from JSON")
	encryptFlag := flag.Bool("encrypt-passwords", false, "Encrypt all plaintext passwords in connections.json with AES-256-GCM")
	keyringSetFlag := flag.String("keyring-set", "", "Store a password in OS Keychain for specified profile ID (prompts on stdin)")
	flag.Parse()

	// Load configuration
	cfg, actualConfigPath, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
	}

	// CLI Management commands
	if *keyringSetFlag != "" {
		fmt.Printf("Enter password to store in OS Keychain for profile '%s': ", *keyringSetFlag)
		var pass string
		fmt.Scanln(&pass)
		if err := config.SaveToKeyring(*keyringSetFlag, pass); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving to Keychain: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Password securely saved to OS Keychain for '%s'.\n", *keyringSetFlag)
		os.Exit(0)
	}

	if *secureFlag {
		if cfg == nil {
			fmt.Fprintln(os.Stderr, "Error: No configuration file found to migrate.")
			os.Exit(1)
		}
		n, err := config.MigrateToKeyring(cfg, actualConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during Keychain migration: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Successfully migrated %d connection passwords to OS Keychain/Keyring.\n", n)
		fmt.Printf("✓ Plaintext passwords removed from %s (file mode 0600).\n", actualConfigPath)
		os.Exit(0)
	}

	if *encryptFlag {
		if cfg == nil {
			fmt.Fprintln(os.Stderr, "Error: No configuration file found to encrypt.")
			os.Exit(1)
		}
		n, err := config.EncryptAllPasswords(cfg, actualConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encrypting passwords: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Successfully encrypted %d passwords with AES-256-GCM in %s.\n", n, actualConfigPath)
		os.Exit(0)
	}

	var initialProfile *config.ConnectionProfile

	// 1. CLI flag overrides
	if *driverFlag != "" && *hostFlag != "" {
		port := *portFlag
		if port == 0 {
			switch *driverFlag {
			case "mssql":
				port = 1433
			case "postgres":
				port = 5432
			case "oracle":
				port = 1521
			}
		}

		authType := config.AuthTypeSQL
		if *authFlag == "windows" {
			authType = config.AuthTypeWindows
		} else if *authFlag == "pass" {
			authType = config.AuthTypePass
		}

		initialProfile = &config.ConnectionProfile{
			ID:        "cli-adhoc",
			Name:      fmt.Sprintf("%s Ad-hoc (%s)", *driverFlag, *hostFlag),
			Driver:    *driverFlag,
			Host:      *hostFlag,
			Port:      port,
			Database:  *dbFlag,
			User:      *userFlag,
			AuthType:  authType,
			PassEntry: *passEntryFlag,
		}
	} else if *profileID != "" && cfg != nil {
		for _, c := range cfg.Connections {
			if c.ID == *profileID {
				cp := c
				initialProfile = &cp
				break
			}
		}
	} else if cfg != nil && cfg.ActiveConnectionID != "" {
		for _, c := range cfg.Connections {
			if c.ID == cfg.ActiveConnectionID {
				cp := c
				initialProfile = &cp
				break
			}
		}
	}

	app := ui.NewApp(cfg, actualConfigPath, initialProfile)
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running dbterm: %v\n", err)
		os.Exit(1)
	}
}
