package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	// encryptedPrefix marks ciphertext produced by EncryptString.
	encryptedPrefix = "enc::"
	// hkdfInfo is the context string for HKDF key derivation.
	hkdfInfo = "openscanner-secrets-v1"
)

// deriveKey uses HKDF-SHA256 to derive a 32-byte AES-256 key from a passphrase.
func deriveKey(passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("encryption key is empty")
	}
	// Use a fixed salt — we derive a unique nonce per encryption, and
	// HKDF with info string provides sufficient domain separation.
	salt := sha256.Sum256([]byte("openscanner"))
	r := hkdf.New(sha256.New, []byte(passphrase), salt[:], []byte(hkdfInfo))
	key := make([]byte, 32)
	if _, err := r.Read(key); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return key, nil
}

// EncryptString encrypts plaintext with AES-256-GCM using a key derived from the passphrase.
// Returns a string prefixed with "enc::" followed by base64-encoded nonce+ciphertext.
func EncryptString(plaintext, passphrase string) (string, error) {
	key, err := deriveKey(passphrase)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a value produced by EncryptString.
// If the value is not encrypted (no "enc::" prefix), it is returned as-is.
func DecryptString(value, passphrase string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	key, err := deriveKey(passphrase)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// IsEncrypted reports whether value carries the "enc::" prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptedPrefix)
}
