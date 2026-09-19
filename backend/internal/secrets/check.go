// Package secrets verifies that the secrets stored in the database can
// still be read with the configured encryption key.
//
// This exists because of the v3.0.0 key-derivation change. It does not
// migrate anything — squelch-rekey (backend/cmd/rekey) does that, on
// purpose and with a backup. All this does is refuse to start when the
// migration has not been run, so the operator gets one clear sentence
// instead of a scattering of decrypt failures at call time: a broken
// downstream push, a Trunk Recorder instance that will not connect, an
// admin page showing "enc::..." where a secret should be.
package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/revtex/squelch/internal/auth"
	"github.com/revtex/squelch/internal/db"
)

// ErrUnreadable is returned when a stored secret cannot be decrypted.
var ErrUnreadable = errors.New("stored secrets cannot be decrypted")

// Queries is the subset of *db.Queries this check needs.
type Queries interface {
	ListSettings(ctx context.Context) ([]db.Setting, error)
	ListDownstreams(ctx context.Context) ([]db.Downstream, error)
	ListTRInstances(ctx context.Context) ([]db.TrInstance, error)
}

// CheckReadable reports an error if any stored "enc::" value fails to
// decrypt under encryptionKey.
//
// With no encryption key configured there is nothing to check: secrets
// are stored in plaintext, which the server already warns about
// elsewhere.
func CheckReadable(ctx context.Context, q Queries, encryptionKey string) error {
	if encryptionKey == "" {
		return nil
	}

	var unreadable []string

	settings, err := q.ListSettings(ctx)
	if err != nil {
		return fmt.Errorf("list settings: %w", err)
	}
	for _, s := range settings {
		if !readable(s.Value, encryptionKey) {
			unreadable = append(unreadable, fmt.Sprintf("settings.%s", s.Key))
		}
	}

	downstreams, err := q.ListDownstreams(ctx)
	if err != nil {
		return fmt.Errorf("list downstreams: %w", err)
	}
	for _, d := range downstreams {
		if !readable(d.ApiKey, encryptionKey) {
			unreadable = append(unreadable, fmt.Sprintf("downstream %d api key", d.ID))
		}
	}

	instances, err := q.ListTRInstances(ctx)
	if err != nil {
		return fmt.Errorf("list tr instances: %w", err)
	}
	for _, i := range instances {
		if i.PasswordEnc.Valid && !readable(i.PasswordEnc.String, encryptionKey) {
			unreadable = append(unreadable, fmt.Sprintf("trunk recorder instance %d password", i.ID))
		}
	}

	if len(unreadable) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %v\n\n"+
		"The secrets encryption scheme changed in v3.0.0. Secrets written by an\n"+
		"earlier version have to be re-encrypted once, with the server stopped:\n\n"+
		"    squelch-rekey -db <path to squelch.db>            # report only\n"+
		"    squelch-rekey -db <path to squelch.db> -apply     # re-encrypt\n\n"+
		"It takes a backup before writing. If you are not upgrading, then\n"+
		"SQUELCH_ENCRYPTION_KEY does not match the one these secrets were\n"+
		"written with — check that before running anything.", ErrUnreadable, unreadable)
}

// readable reports whether value is either not encrypted at all or
// decrypts cleanly under encryptionKey.
func readable(value, encryptionKey string) bool {
	if !auth.IsEncrypted(value) {
		return true
	}
	_, err := auth.DecryptString(value, encryptionKey)
	return err == nil
}
