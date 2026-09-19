package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

// legacyFixture encrypts plaintext exactly as the pre-v3.0.0 scheme did.
//
// Test-only, and the mirror image of decryptLegacy: without it the tests
// would have to assert against hard-coded ciphertext blobs, which would
// pin the nonce as well as the scheme and say nothing about whether the
// tool handles freshly-written values.
func legacyFixture(t *testing.T, plaintext string) string {
	t.Helper()
	key, err := deriveLegacyKey(testPass)
	if err != nil {
		t.Fatalf("deriveLegacyKey: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("nonce: %v", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ct)
}
