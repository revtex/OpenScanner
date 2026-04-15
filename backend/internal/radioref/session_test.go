package radioref

import (
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore()

	token, err := store.Create("alice", "secret123")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(token) != 48 {
		t.Errorf("token length = %d, want 48", len(token))
	}

	sess := store.Get(token)
	if sess == nil {
		t.Fatal("Get returned nil for valid token")
	}
	if sess.Username != "alice" {
		t.Errorf("Username = %q, want %q", sess.Username, "alice")
	}
	if sess.Password != "secret123" {
		t.Errorf("Password = %q, want %q", sess.Password, "secret123")
	}
}

func TestSessionStore_GetMissing(t *testing.T) {
	store := NewSessionStore()
	if sess := store.Get("nonexistent"); sess != nil {
		t.Errorf("Get returned %+v for missing token, want nil", sess)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	token, _ := store.Create("bob", "pass")
	store.Delete(token)
	if sess := store.Get(token); sess != nil {
		t.Errorf("Get returned %+v after Delete, want nil", sess)
	}
}

func TestSessionStore_Expiry(t *testing.T) {
	store := NewSessionStore()
	token, _ := store.Create("carol", "pass")

	// Manually expire the session.
	store.mu.Lock()
	store.sessions[token].ExpiresAt = time.Now().Add(-time.Second)
	store.mu.Unlock()

	if sess := store.Get(token); sess != nil {
		t.Errorf("Get returned session after expiry, want nil")
	}
}

func TestSessionStore_CleanupOnCreate(t *testing.T) {
	store := NewSessionStore()
	tok1, _ := store.Create("old", "pass")

	// Expire tok1.
	store.mu.Lock()
	store.sessions[tok1].ExpiresAt = time.Now().Add(-time.Second)
	store.mu.Unlock()

	// Create a new session; cleanup should remove tok1.
	_, _ = store.Create("new", "pass")

	store.mu.Lock()
	_, found := store.sessions[tok1]
	store.mu.Unlock()
	if found {
		t.Error("expired session not cleaned up during Create")
	}
}

func TestSessionStore_MultipleTokensIndependent(t *testing.T) {
	store := NewSessionStore()
	tok1, _ := store.Create("user1", "pass1")
	tok2, _ := store.Create("user2", "pass2")

	if tok1 == tok2 {
		t.Fatal("two tokens should not be equal")
	}

	store.Delete(tok1)
	if sess := store.Get(tok2); sess == nil || sess.Username != "user2" {
		t.Error("deleting tok1 should not affect tok2")
	}
}
