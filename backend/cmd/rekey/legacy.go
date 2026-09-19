package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// The pre-v3.0.0 key-derivation inputs.
//
// These lived in internal/auth until v3.0.0 and are reproduced here — and
// only here — because this tool is the one thing that still has to read
// secrets written under them. They are inputs to HKDF, not labels: the
// exact byte strings below are what every deployment before v3.0.0
// encrypted with, so they must never be "tidied".
//
// Nothing in the server imports this file. When no supported deployment
// still holds pre-v3.0.0 ciphertext, delete the tool rather than edit it.
const (
	legacyHKDFInfo  = "openscanner-secrets-v1"
	legacyHKDFSalt  = "openscanner"
	encryptedPrefix = "enc::"
)

// deriveLegacyKey reproduces the pre-v3.0.0 HKDF-SHA256 derivation.
func deriveLegacyKey(passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("encryption key is empty")
	}
	salt := sha256.Sum256([]byte(legacyHKDFSalt))
	r := hkdf.New(sha256.New, []byte(passphrase), salt[:], []byte(legacyHKDFInfo))
	key := make([]byte, 32)
	if _, err := r.Read(key); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return key, nil
}

// decryptLegacy decrypts a value written by the pre-v3.0.0 scheme. It
// mirrors the old auth.DecryptString exactly: AES-256-GCM, nonce
// prepended, base64 over the whole thing, behind an "enc::" prefix.
func decryptLegacy(value, passphrase string) (string, error) {
	if !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	key, err := deriveLegacyKey(passphrase)
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
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
