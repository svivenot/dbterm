package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AuthType represents the authentication method
type AuthType string

const (
	AuthTypeSQL     AuthType = "sql"     // Standard username/password
	AuthTypeKeyring AuthType = "keyring" // Stored in OS secure Keychain / Credential Vault
	AuthTypeWindows AuthType = "windows" // Windows / NTLM / Kerberos Authentication
	AuthTypePass    AuthType = "pass"    // Password retrieved dynamically via Unix 'pass'
	AuthTypeEnv     AuthType = "env"     // Password retrieved from an environment variable
)

// ConnectionProfile defines a database connection
type ConnectionProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Group       string            `json:"group,omitempty"`  // Hierarchical folder/group (e.g. "Production/Europe", "Local/Docker")
	Folder      string            `json:"folder,omitempty"` // Alias for group
	Driver      string            `json:"driver"`           // mssql, postgres, oracle
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Database    string            `json:"database"`
	User        string            `json:"user"`
	AuthType    AuthType          `json:"auth_type"` // sql, keyring, windows, pass, env
	Password    string            `json:"password,omitempty"`
	PassEntry   string            `json:"pass_entry,omitempty"`   // e.g. "databases/mssql/sa"
	PasswordEnv string            `json:"password_env,omitempty"` // e.g. "MSSQL_PASSWORD"
	Domain      string            `json:"domain,omitempty"`       // Windows Auth Domain (e.g. "CORP")
	SSLMode     string            `json:"ssl_mode,omitempty"`     // disable, require, etc.
	ExtraParams map[string]string `json:"extra_params,omitempty"`
}

// GetGroup returns the group/folder name, defaulting to "Default" if none specified
func (p *ConnectionProfile) GetGroup() string {
	if p.Group != "" {
		return p.Group
	}
	if p.Folder != "" {
		return p.Folder
	}
	return "General"
}

// AIProvider defines the LLM inference provider
type AIProvider string

const (
	AIProviderEmbedded     AIProvider = "embedded"      // Self-contained GGUF runner (default)
	AIProviderOllama       AIProvider = "ollama"        // Local Ollama instance
	AIProviderOpenAICompat AIProvider = "openai_compat" // LocalAI / llama.cpp server / OpenVINO / vLLM
	AIProviderOpenAI       AIProvider = "openai"        // Remote OpenAI API
	AIProviderAnthropic    AIProvider = "anthropic"     // Remote Anthropic API
	AIProviderGemini       AIProvider = "gemini"        // Remote Google Gemini API
)

// AIConfig defines settings for the AI SQL Assistant
type AIConfig struct {
	Enabled         bool       `json:"enabled"`                    // Toggle AI features (default: true)
	Provider        AIProvider `json:"provider,omitempty"`         // embedded (default), ollama, openai_compat, etc.
	ModelName       string     `json:"model_name,omitempty"`       // default: "qwen2.5-coder-1.5b-instruct"
	ModelPath       string     `json:"model_path,omitempty"`       // optional custom path to .gguf model file
	Endpoint        string     `json:"endpoint,omitempty"`         // optional endpoint (e.g. "http://localhost:11434")
	APIKey          string     `json:"api_key,omitempty"`          // optional API key
	Temperature     float64    `json:"temperature,omitempty"`      // default: 0.1
	MaxTokens       int        `json:"max_tokens,omitempty"`       // default: 1024
	IncludeSchema   bool       `json:"include_schema"`             // default: true
	MaxSchemaTables int        `json:"max_schema_tables,omitempty"` // default: 30
}

// Config represents the root configuration file
type Config struct {
	ActiveConnectionID string              `json:"active_connection_id,omitempty"`
	Connections        []ConnectionProfile `json:"connections"`
	AI                 AIConfig            `json:"ai,omitempty"`
}

// GetDefaultConfigPath returns ~/.config/dbterm/connections.json
func GetDefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "dbterm", "connections.json"), nil
}

// LoadConfig loads the configuration from the preferred paths:
// 1. Path specified
// 2. ./connections.json (workspace local)
// 3. ~/.config/dbterm/connections.json
func LoadConfig(customPath string) (*Config, string, error) {
	paths := []string{}
	if customPath != "" {
		paths = append(paths, customPath)
	}
	paths = append(paths, "connections.json")

	defaultPath, err := GetDefaultConfigPath()
	if err == nil {
		paths = append(paths, defaultPath)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, p, fmt.Errorf("failed to read config file %s: %w", p, err)
			}
			var cfg Config
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, p, fmt.Errorf("failed to parse JSON from %s: %w", p, err)
			}
			return &cfg, p, nil
		}
	}

	// Default fallback config if none exists
	defaultCfg := &Config{
		Connections: []ConnectionProfile{
			{
				ID:       "mssql-local-sales",
				Name:     "MS SQL - SalesDB (Local Docker)",
				Driver:   "mssql",
				Host:     "localhost",
				Port:     1433,
				Database: "SalesDB",
				User:     "sa",
				AuthType: AuthTypeSQL,
				Password: "Password123!",
			},
			{
				ID:        "mssql-windows-pass-sample",
				Name:      "MS SQL - Windows Auth (via pass)",
				Driver:    "mssql",
				Host:      "sql-corp.example.com",
				Port:      1433,
				Database:  "SalesDB",
				User:      "sylvain",
				Domain:    "CORP",
				AuthType:  AuthTypeWindows,
				PassEntry: "corp/ad/sylvain",
			},
		},
	}
	return defaultCfg, defaultPath, nil
}

// SaveConfig saves the configuration to the specified JSON path
func SaveConfig(cfg *Config, targetPath string) error {
	if targetPath == "" {
		var err error
		targetPath, err = GetDefaultConfigPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON config: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", targetPath, err)
	}

	return nil
}

// UpsertConnection inserts or updates a connection profile by its ID
func UpsertConnection(cfg *Config, profile ConnectionProfile) {
	if cfg == nil {
		return
	}
	if profile.ID == "" {
		cleanName := strings.ToLower(strings.ReplaceAll(profile.Name, " ", "-"))
		profile.ID = fmt.Sprintf("%s-%s-%d", profile.Driver, cleanName, time.Now().Unix()%10000)
	}
	for i, c := range cfg.Connections {
		if c.ID == profile.ID {
			cfg.Connections[i] = profile
			return
		}
	}
	cfg.Connections = append(cfg.Connections, profile)
}

// DeleteConnection removes a connection profile by its ID
func DeleteConnection(cfg *Config, profileID string) bool {
	if cfg == nil {
		return false
	}
	for i, c := range cfg.Connections {
		if c.ID == profileID {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return true
		}
	}
	return false
}

// ResolvePassword dynamically retrieves the password based on profile settings:
// 1. Direct password if set
// 2. 'pass' CLI tool if PassEntry is set (or AuthType is pass)
// 3. Environment variable if PasswordEnv is set
func (p *ConnectionProfile) ResolvePassword(ctx context.Context) (string, error) {
	// 1. OS Native Keyring / Keychain
	if p.AuthType == AuthTypeKeyring {
		pass, err := GetFromKeyring(p.ID)
		if err == nil && pass != "" {
			return pass, nil
		}
		if p.Password != "" {
			if IsEncrypted(p.Password) {
				return DecryptPassword(p.Password)
			}
			return p.Password, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to retrieve password from OS keychain for '%s': %w", p.ID, err)
		}
	}

	// Try OS Keyring opportunistic lookup if plain password is not provided
	if p.Password == "" && p.PassEntry == "" && p.PasswordEnv == "" && p.AuthType != AuthTypeWindows {
		if pass, err := GetFromKeyring(p.ID); err == nil && pass != "" {
			return pass, nil
		}
	}

	// 2. Check PassEntry first if specified
	if p.PassEntry != "" || p.AuthType == AuthTypePass {
		entry := p.PassEntry
		if entry == "" {
			entry = fmt.Sprintf("databases/%s/%s", p.Driver, p.User)
		}
		pass, err := GetPasswordFromPass(ctx, entry)
		if err == nil && pass != "" {
			return pass, nil
		}
		// If pass failed and there's a fallback password, use it
		if p.Password != "" {
			if IsEncrypted(p.Password) {
				return DecryptPassword(p.Password)
			}
			return p.Password, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to get password from pass entry '%s': %w", entry, err)
		}
	}

	// 3. Environment variable
	if p.PasswordEnv != "" {
		if val := os.Getenv(p.PasswordEnv); val != "" {
			return val, nil
		}
	}

	// 4. In-file Password field (Encrypted or Plain)
	if p.Password != "" {
		if IsEncrypted(p.Password) {
			return DecryptPassword(p.Password)
		}
		// Expand env var syntax like ${MY_PASS}
		if strings.HasPrefix(p.Password, "${") && strings.HasSuffix(p.Password, "}") {
			envKey := strings.TrimSuffix(strings.TrimPrefix(p.Password, "${"), "}")
			return os.Getenv(envKey), nil
		}
		return p.Password, nil
	}

	return "", nil
}

// MigrateToKeyring migrates all plaintext/encrypted passwords in connections to the OS native Keyring
func MigrateToKeyring(cfg *Config, configPath string) (int, error) {
	if cfg == nil {
		return 0, fmt.Errorf("nil config provided")
	}

	migratedCount := 0
	ctx := context.Background()

	for i := range cfg.Connections {
		c := &cfg.Connections[i]
		if c.AuthType == AuthTypeWindows {
			continue
		}

		rawPass, err := c.ResolvePassword(ctx)
		if err == nil && rawPass != "" {
			if err := SaveToKeyring(c.ID, rawPass); err != nil {
				return migratedCount, fmt.Errorf("failed to save password for %s to keychain: %w", c.ID, err)
			}
			c.AuthType = AuthTypeKeyring
			c.Password = "" // Remove plain password from JSON!
			migratedCount++
		}
	}

	if migratedCount > 0 {
		if err := SaveConfig(cfg, configPath); err != nil {
			return migratedCount, fmt.Errorf("failed to save updated config: %w", err)
		}
	}

	return migratedCount, nil
}

// EncryptAllPasswords encrypts all plaintext passwords in connections.json with AES-256-GCM
func EncryptAllPasswords(cfg *Config, configPath string) (int, error) {
	if cfg == nil {
		return 0, fmt.Errorf("nil config provided")
	}

	encryptedCount := 0
	for i := range cfg.Connections {
		c := &cfg.Connections[i]
		if c.Password != "" && !IsEncrypted(c.Password) && !strings.HasPrefix(c.Password, "${") {
			enc, err := EncryptPassword(c.Password)
			if err != nil {
				return encryptedCount, fmt.Errorf("failed to encrypt password for %s: %w", c.ID, err)
			}
			c.Password = enc
			encryptedCount++
		}
	}

	if encryptedCount > 0 {
		if err := SaveConfig(cfg, configPath); err != nil {
			return encryptedCount, fmt.Errorf("failed to save encrypted config: %w", err)
		}
	}

	return encryptedCount, nil
}

// GetPasswordFromPass executes `pass show <entry>` and returns the first line (the password)
func GetPasswordFromPass(ctx context.Context, entry string) (string, error) {
	if entry == "" {
		return "", fmt.Errorf("empty pass entry name")
	}

	// 5-second timeout to avoid blocking if gpg pinentry prompts in background
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "pass", "show", entry)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(errOut.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("pass error: %s", errMsg)
	}

	lines := strings.Split(strings.ReplaceAll(out.String(), "\r\n", "\n"), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "", fmt.Errorf("empty output from pass show %s", entry)
}
