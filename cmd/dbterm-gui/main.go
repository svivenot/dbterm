package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"dbterm/internal/config"
	"dbterm/internal/db"
	"dbterm/internal/gui"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
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
	versionFlag := flag.Bool("version", false, "Print dbterm-gui version and build info")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dbterm-gui version %s (commit: %s, built: %s)\n", Version, Commit, Date)
		os.Exit(0)
	}

	// Load configuration
	cfg, actualConfigPath, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
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
		} else if *authFlag == "keyring" {
			authType = config.AuthTypeKeyring
		}

		initialProfile = &config.ConnectionProfile{
			ID:       "cli-adhoc",
			Name:     fmt.Sprintf("%s Ad-hoc (%s)", *driverFlag, *hostFlag),
			Driver:   *driverFlag,
			Host:     *hostFlag,
			Port:     port,
			Database: *dbFlag,
			User:     *userFlag,
			AuthType: authType,
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

	var initialDriver db.Driver
	if initialProfile != nil {
		drv, err := db.NewDriver(initialProfile)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := drv.Connect(ctx, initialProfile); err == nil {
				initialDriver = drv
			}
		}
	}

	guiApp := gui.NewApp(cfg, actualConfigPath, initialProfile, initialDriver)
	guiApp.Run()
}
