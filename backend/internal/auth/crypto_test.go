package auth

import (
	"encoding/hex"
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

// TestKeyDerivationInputsAreFrozen pins the HKDF inputs.
//
// These are key-derivation material, so changing them derives a different
// key and makes every stored "enc::" secret unreadable until it is
// re-encrypted. They changed exactly once, in v3.0.0, paired with the
// squelch-rekey tool; cmd/rekey pins the pre-v3.0.0 value the same way,
// because that tool still has to read secrets written under it.
//
// If this fails, an edit has invalidated every secret in every existing
// deployment. Revert it, or ship another rekey pass and a major version.
func TestKeyDerivationInputsAreFrozen(t *testing.T) {
	// Ciphertext produced by the shipped scheme, decryptable only if the
	// salt and info string still match what deployments encrypted with.
	const passphrase = "correct horse battery staple"
	const plaintext = "s3cret-value"

	ct, err := EncryptString(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := DecryptString(ct, passphrase)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip got %q, want %q", got, plaintext)
	}

	key, err := deriveKey(passphrase)
	if err != nil {
		t.Fatalf("deriveKey: %v", err)
	}
	// Golden key for the passphrase above under the v3.0.0 salt/info.
	const wantKey = "d6bf8dfaaa6b5450b8182c2e41d8803f497a04d9f6fc4703521c1ca0b32fa836"
	if hex.EncodeToString(key) != wantKey {
		t.Errorf("derived key changed: got %s, want %s\n"+
			"The HKDF salt or info string was modified. Every existing "+
			"enc:: secret is now unreadable. Revert that change, or pair "+
			"it with a squelch-rekey pass and a major version.",
			hex.EncodeToString(key), wantKey)
	}
}
