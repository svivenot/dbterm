package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

const (
	// EncryptedPrefix marks an AES-256-GCM encrypted secret string
	EncryptedPrefix = "enc:v1:"
)

// IsEncrypted returns true if the string is in encrypted vault format
func IsEncrypted(val string) bool {
	return strings.HasPrefix(val, EncryptedPrefix)
}

// deriveMachineKey derives a stable 32-byte AES key specific to this machine and user
func deriveMachineKey() ([]byte, error) {
	home, _ := os.UserHomeDir()
	hostname, _ := os.Hostname()

	// Machine / OS identifier entropy
	var machineID string
	switch runtime.GOOS {
	case "darwin":
		machineID = "darwin-keychain-vault-" + home
	case "windows":
		machineID = "windows-vault-" + os.Getenv("COMPUTERNAME") + "-" + home
	default:
		// Linux: try reading /etc/machine-id or /var/lib/dbus/machine-id
		if data, err := os.ReadFile("/etc/machine-id"); err == nil {
			machineID = strings.TrimSpace(string(data))
		} else if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			machineID = strings.TrimSpace(string(data))
		} else {
			machineID = "linux-vault-" + hostname + "-" + home
		}
	}

	seed := fmt.Sprintf("dbterm-master-vault-seed:%s:%s:%s", hostname, home, machineID)
	hash := sha256.Sum256([]byte(seed))
	return hash[:], nil
}

// EncryptPassword encrypts a plaintext string with AES-256-GCM
func EncryptPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}

	key, err := deriveMachineKey()
	if err != nil {
		return "", fmt.Errorf("failed to derive machine key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPassword decrypts an AES-256-GCM encrypted string
func DecryptPassword(encryptedStr string) (string, error) {
	if encryptedStr == "" {
		return "", nil
	}
	if !IsEncrypted(encryptedStr) {
		return encryptedStr, nil
	}

	rawB64 := strings.TrimPrefix(encryptedStr, EncryptedPrefix)
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("invalid base64 encrypted payload: %w", err)
	}

	key, err := deriveMachineKey()
	if err != nil {
		return "", fmt.Errorf("failed to derive machine key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("encrypted payload too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (corrupted or wrong machine key): %w", err)
	}

	return string(plaintext), nil
}
