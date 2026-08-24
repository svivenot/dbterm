package config

import (
	"context"
	"testing"
)

func TestConfigLoadDefault(t *testing.T) {
	cfg, _, err := LoadConfig("../../connections.example.json")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Connections) == 0 {
		t.Fatalf("Expected at least one connection, got 0")
	}

	foundMSSQL := false
	for _, c := range cfg.Connections {
		if c.Driver == "mssql" {
			foundMSSQL = true
			break
		}
	}
	if !foundMSSQL {
		t.Errorf("Expected to find MSSQL connection profile in config")
	}
}

func TestResolvePasswordDirect(t *testing.T) {
	p := ConnectionProfile{
		Password: "SecretPassword123!",
	}
	pass, err := p.ResolvePassword(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error resolving direct password: %v", err)
	}
	if pass != "SecretPassword123!" {
		t.Errorf("Expected 'SecretPassword123!', got '%s'", pass)
	}
}

func TestAESVaultEncryption(t *testing.T) {
	original := "SuperSecretPassword2026!#$%"

	encrypted, err := EncryptPassword(original)
	if err != nil {
		t.Fatalf("EncryptPassword failed: %v", err)
	}

	if !IsEncrypted(encrypted) {
		t.Errorf("Expected encrypted string to have prefix %s, got: %s", EncryptedPrefix, encrypted)
	}

	decrypted, err := DecryptPassword(encrypted)
	if err != nil {
		t.Fatalf("DecryptPassword failed: %v", err)
	}

	if decrypted != original {
		t.Errorf("Decryption mismatch: expected '%s', got '%s'", original, decrypted)
	}

	// Test ResolvePassword with encrypted password
	p := ConnectionProfile{
		Password: encrypted,
	}
	resolved, err := p.ResolvePassword(context.Background())
	if err != nil {
		t.Fatalf("ResolvePassword with encrypted password failed: %v", err)
	}
	if resolved != original {
		t.Errorf("ResolvePassword mismatch: expected '%s', got '%s'", original, resolved)
	}
}

func TestKeyringStorage(t *testing.T) {
	testID := "__test_dbterm_profile__"
	testPass := "KeyringSecretPassword456!"

	err := SaveToKeyring(testID, testPass)
	if err != nil {
		t.Skipf("Skipping Keyring test (no native keychain available in this environment): %v", err)
		return
	}
	defer DeleteFromKeyring(testID)

	got, err := GetFromKeyring(testID)
	if err != nil {
		t.Fatalf("GetFromKeyring failed: %v", err)
	}
	if got != testPass {
		t.Errorf("Expected '%s', got '%s'", testPass, got)
	}

	p := ConnectionProfile{
		ID:       testID,
		AuthType: AuthTypeKeyring,
	}
	resolved, err := p.ResolvePassword(context.Background())
	if err != nil {
		t.Fatalf("ResolvePassword from Keyring failed: %v", err)
	}
	if resolved != testPass {
		t.Errorf("Expected '%s', got '%s'", testPass, resolved)
	}
}
