package config

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	// KeyringServiceName is the identifier used in the OS keychain / credential vault
	KeyringServiceName = "dbterm"
)

// SaveToKeyring stores a password securely in the OS native keychain / keyring
func SaveToKeyring(profileID, password string) error {
	if profileID == "" {
		return fmt.Errorf("profile ID cannot be empty")
	}
	return keyring.Set(KeyringServiceName, profileID, password)
}

// GetFromKeyring retrieves a password from the OS native keychain / keyring
func GetFromKeyring(profileID string) (string, error) {
	if profileID == "" {
		return "", fmt.Errorf("profile ID cannot be empty")
	}
	return keyring.Get(KeyringServiceName, profileID)
}

// DeleteFromKeyring removes a password from the OS native keychain / keyring
func DeleteFromKeyring(profileID string) error {
	if profileID == "" {
		return fmt.Errorf("profile ID cannot be empty")
	}
	err := keyring.Delete(KeyringServiceName, profileID)
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}

// IsKeyringAvailable checks if the OS native keyring service is responsive
func IsKeyringAvailable() bool {
	testKey := "__dbterm_probe__"
	err := keyring.Set(KeyringServiceName, testKey, "probe")
	if err != nil {
		return false
	}
	_ = keyring.Delete(KeyringServiceName, testKey)
	return true
}
