package auth

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		plaintext  string
		passphrase string
	}{
		{"simple", "hello world", "my-secret-key"},
		{"empty plaintext", "", "my-secret-key"},
		{"url value", "http://localhost:8081", "encryption-key-123"},
		{"api key", "a1b2c3d4e5f6", "super-secret"},
		{"unicode", "tëst-vàlüe-日本語", "key-with-spëcial-chars"},
		{"long value", strings.Repeat("x", 10000), "key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := EncryptString(tc.plaintext, tc.passphrase)
			if err != nil {
				t.Fatalf("EncryptString: %v", err)
			}

			if tc.plaintext != "" && !IsEncrypted(encrypted) {
				t.Fatalf("encrypted value missing prefix: %q", encrypted)
			}

			if tc.plaintext != "" && !strings.HasPrefix(encrypted, encryptedPrefix) {
				t.Fatalf("expected enc:: prefix, got %q", encrypted[:10])
			}

			decrypted, err := DecryptString(encrypted, tc.passphrase)
			if err != nil {
				t.Fatalf("DecryptString: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Fatalf("round-trip failed: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestDecryptPlaintextPassthrough(t *testing.T) {
	// Unencrypted values should pass through unchanged.
	val := "http://localhost:8081"
	got, err := DecryptString(val, "any-key")
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if got != val {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	encrypted, err := EncryptString("secret-value", "correct-key")
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	_, err = DecryptString(encrypted, "wrong-key")
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptEmptyKey(t *testing.T) {
	_, err := EncryptString("value", "")
	if err == nil {
		t.Fatal("expected error with empty key")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plaintext") {
		t.Fatal("plaintext should not be encrypted")
	}
	if !IsEncrypted("enc::abc123") {
		t.Fatal("enc:: prefix should be detected")
	}
}

func TestDifferentEncryptionsDiffer(t *testing.T) {
	// Same plaintext encrypted twice should produce different ciphertext (random nonce).
	e1, _ := EncryptString("same-value", "same-key")
	e2, _ := EncryptString("same-value", "same-key")
	if e1 == e2 {
		t.Fatal("two encryptions of same value should differ (random nonce)")
	}

	// Both should decrypt to the same value.
	d1, _ := DecryptString(e1, "same-key")
	d2, _ := DecryptString(e2, "same-key")
	if d1 != d2 {
		t.Fatalf("decrypted values differ: %q vs %q", d1, d2)
	}
}
