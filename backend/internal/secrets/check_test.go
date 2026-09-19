package secrets

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/revtex/squelch/internal/auth"
	"github.com/revtex/squelch/internal/db"
)

const key = "test-encryption-key"

type fakeQueries struct {
	settings    []db.Setting
	downstreams []db.Downstream
	instances   []db.TrInstance
}

func (f fakeQueries) ListSettings(context.Context) ([]db.Setting, error) {
	return f.settings, nil
}
func (f fakeQueries) ListDownstreams(context.Context) ([]db.Downstream, error) {
	return f.downstreams, nil
}
func (f fakeQueries) ListTRInstances(context.Context) ([]db.TrInstance, error) {
	return f.instances, nil
}

func enc(t *testing.T, v string) string {
	t.Helper()
	out, err := auth.EncryptString(v, key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReadableSecretsPass(t *testing.T) {
	q := fakeQueries{
		settings:    []db.Setting{{Key: "jwtSecret", Value: enc(t, "s")}, {Key: "branding", Value: "Squelch"}},
		downstreams: []db.Downstream{{ID: 1, ApiKey: enc(t, "k")}},
		instances:   []db.TrInstance{{ID: 1, PasswordEnc: sql.NullString{String: enc(t, "p"), Valid: true}}},
	}
	if err := CheckReadable(context.Background(), q, key); err != nil {
		t.Fatalf("expected readable secrets to pass, got %v", err)
	}
}

func TestUnreadableSecretIsReported(t *testing.T) {
	// A value encrypted under a different key stands in for one written
	// by the pre-v3.0.0 derivation: same shape, undecryptable here.
	other, err := auth.EncryptString("s", "a-different-key")
	if err != nil {
		t.Fatal(err)
	}
	q := fakeQueries{settings: []db.Setting{{Key: "jwtSecret", Value: other}}}

	err = CheckReadable(context.Background(), q, key)
	if err == nil {
		t.Fatal("expected an error for an unreadable secret")
	}
	if !errors.Is(err, ErrUnreadable) {
		t.Errorf("error should wrap ErrUnreadable, got %v", err)
	}
	if !contains(err.Error(), "squelch-rekey") {
		t.Errorf("error must name the tool that fixes it, got: %v", err)
	}
	if !contains(err.Error(), "settings.jwtSecret") {
		t.Errorf("error must name which secret failed, got: %v", err)
	}
}

func TestNoEncryptionKeySkipsCheck(t *testing.T) {
	// Plaintext at rest is a different problem, warned about elsewhere.
	q := fakeQueries{settings: []db.Setting{{Key: "jwtSecret", Value: "enc::not-really"}}}
	if err := CheckReadable(context.Background(), q, ""); err != nil {
		t.Fatalf("expected no check without an encryption key, got %v", err)
	}
}

func TestPlaintextValuesAreNotFlagged(t *testing.T) {
	q := fakeQueries{
		settings:    []db.Setting{{Key: "branding", Value: "Squelch"}},
		downstreams: []db.Downstream{{ID: 1, ApiKey: "plain-key"}},
		instances:   []db.TrInstance{{ID: 1, PasswordEnc: sql.NullString{Valid: false}}},
	}
	if err := CheckReadable(context.Background(), q, key); err != nil {
		t.Fatalf("plaintext values must not be flagged, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
