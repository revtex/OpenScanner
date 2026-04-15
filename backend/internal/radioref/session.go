package radioref

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const sessionTTL = 30 * time.Minute
const maxSessions = 50

// Session holds temporary RadioReference credentials.
type Session struct {
	Username  string
	Password  string
	ExpiresAt time.Time
}

// SessionStore manages ephemeral RR credential sessions keyed by a random token.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// Create stores credentials and returns a session token.
func (s *SessionStore) Create(username, password string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	if len(s.sessions) >= maxSessions {
		return "", errors.New("too many active sessions")
	}
	s.sessions[token] = &Session{
		Username:  username,
		Password:  password,
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	return token, nil
}

// Get returns credentials for a session token, or nil if expired/missing.
func (s *SessionStore) Get(token string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, token)
		return nil
	}
	return sess
}

// Delete removes a session.
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *SessionStore) cleanup() {
	now := time.Now()
	for k, v := range s.sessions {
		if now.After(v.ExpiresAt) {
			delete(s.sessions, k)
		}
	}
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
