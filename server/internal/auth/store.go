package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
)

type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type PublicUser struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	path string
	mu   sync.RWMutex
	data userData
}

type userData struct {
	Users []User `json:"users"`
}

func NewStore(path, adminUsername, adminPassword string) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if strings.TrimSpace(adminPassword) == "" {
			return nil, fmt.Errorf("no users exist and no bootstrap admin password was provided")
		}
		if err := s.CreateUser(adminUsername, adminPassword, RoleAdmin); err != nil {
			return nil, err
		}
	} else if len(s.data.Users) == 0 {
		if strings.TrimSpace(adminPassword) == "" {
			return nil, fmt.Errorf("users file is empty and no bootstrap admin password was provided")
		}
		if err := s.CreateUser(adminUsername, adminPassword, RoleAdmin); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Authenticate(username, password string) (PublicUser, bool) {
	username = normalizeUsername(username)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.data.Users {
		if subtle.ConstantTimeCompare([]byte(user.Username), []byte(username)) != 1 {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			return PublicUser{}, false
		}
		return publicUser(user), true
	}
	return PublicUser{}, false
}

func (s *Store) ListUsers() []PublicUser {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]PublicUser, 0, len(s.data.Users))
	for _, user := range s.data.Users {
		users = append(users, publicUser(user))
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	return users
}

func (s *Store) GetUser(username string) (PublicUser, bool) {
	username = normalizeUsername(username)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.data.Users {
		if user.Username == username {
			return publicUser(user), true
		}
	}
	return PublicUser{}, false
}

func (s *Store) CreateUser(username, password, role string) error {
	username = normalizeUsername(username)
	role = normalizeRole(role)
	if err := validateUserInput(username, password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.data.Users {
		if user.Username == username {
			return fmt.Errorf("user already exists: %s", username)
		}
	}

	s.data.Users = append(s.data.Users, User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	})
	return s.saveLocked()
}

func (s *Store) SetPassword(username, password string) error {
	username = normalizeUsername(username)
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Users {
		if s.data.Users[i].Username == username {
			s.data.Users[i].PasswordHash = string(hash)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("user not found: %s", username)
}

func (s *Store) DeleteUser(username string) error {
	username = normalizeUsername(username)

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, user := range s.data.Users {
		if user.Username == username {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("user not found: %s", username)
	}
	if s.data.Users[idx].Role == RoleAdmin && s.adminCountLocked() <= 1 {
		return fmt.Errorf("cannot delete the last admin user")
	}

	s.data.Users = append(s.data.Users[:idx], s.data.Users[idx+1:]...)
	return s.saveLocked()
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		return err
	}
	return nil
}

func (s *Store) saveLocked() error {
	tmp := s.path + ".tmp"
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) adminCountLocked() int {
	count := 0
	for _, user := range s.data.Users {
		if user.Role == RoleAdmin {
			count++
		}
	}
	return count
}

func publicUser(user User) PublicUser {
	return PublicUser{
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case RoleAdmin:
		return RoleAdmin
	default:
		return RoleOperator
	}
}

func validateUserInput(username, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must be 3 to 64 characters")
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("username may contain only lowercase letters, numbers, dots, underscores, or hyphens")
	}
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	return nil
}

type Session struct {
	Token     string
	User      PublicUser
	ExpiresAt time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Session),
		ttl:      ttl,
	}
}

func (m *SessionManager) Create(user PublicUser) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	session := Session{
		Token:     token,
		User:      user,
		ExpiresAt: time.Now().UTC().Add(m.ttl),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now().UTC())
	m.sessions[token] = session
	return session, nil
}

func (m *SessionManager) Get(token string) (Session, bool) {
	m.mu.RLock()
	session, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		m.Delete(token)
		return Session{}, false
	}
	return session, true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *SessionManager) cleanupLocked(now time.Time) {
	for token, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
}

func BearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
