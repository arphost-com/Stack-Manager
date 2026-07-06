package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/storage"
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
	store *storage.Store
}

func NewStore(store *storage.Store, adminUsername, adminPassword string) (*Store, error) {
	s := &Store{store: store}
	ctx := context.Background()
	count, err := s.userCount(ctx)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		if strings.TrimSpace(adminPassword) == "" {
			return nil, fmt.Errorf("no users exist and no bootstrap admin password was provided")
		}
		if err := s.CreateUser(adminUsername, adminPassword, RoleAdmin); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Authenticate(username, password string) (PublicUser, bool) {
	username = normalizeUsername(username)
	var user User
	err := s.store.DB.QueryRowContext(context.Background(), `SELECT username, password_hash, role, created_at FROM users WHERE username=?`, username).
		Scan(&user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return PublicUser{}, false
	}
	if subtle.ConstantTimeCompare([]byte(user.Username), []byte(username)) != 1 {
		return PublicUser{}, false
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return PublicUser{}, false
	}
	return publicUser(user), true
}

func (s *Store) ListUsers() []PublicUser {
	ctx := context.Background()
	var cached []PublicUser
	if s.store.GetJSON(ctx, "users:list", &cached) {
		return cached
	}

	rows, err := s.store.DB.QueryContext(ctx, `SELECT username, role, created_at FROM users ORDER BY username ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	users := make([]PublicUser, 0)
	for rows.Next() {
		var user PublicUser
		if err := rows.Scan(&user.Username, &user.Role, &user.CreatedAt); err == nil {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	s.store.SetJSON(ctx, "users:list", users, s.store.CacheTTL)
	return users
}

func (s *Store) GetUser(username string) (PublicUser, bool) {
	username = normalizeUsername(username)
	ctx := context.Background()
	var user PublicUser
	err := s.store.DB.QueryRowContext(ctx, `SELECT username, role, created_at FROM users WHERE username=?`, username).
		Scan(&user.Username, &user.Role, &user.CreatedAt)
	return user, err == nil
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
	ctx := context.Background()
	_, err = s.store.DB.ExecContext(ctx, `INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		username, string(hash), role, time.Now().UTC())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return fmt.Errorf("user already exists: %s", username)
		}
		return err
	}
	s.store.DeleteCache(ctx, "users:list")
	return nil
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
	ctx := context.Background()
	res, err := s.store.DB.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE username=?`, string(hash), username)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("user not found: %s", username)
	}
	return nil
}

func (s *Store) DeleteUser(username string) error {
	username = normalizeUsername(username)
	ctx := context.Background()
	var role string
	err := s.store.DB.QueryRowContext(ctx, `SELECT role FROM users WHERE username=?`, username).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("user not found: %s", username)
	}
	if err != nil {
		return err
	}
	if role == RoleAdmin {
		admins, err := s.adminCount(ctx)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return fmt.Errorf("cannot delete the last admin user")
		}
	}
	if _, err := s.store.DB.ExecContext(ctx, `DELETE FROM users WHERE username=?`, username); err != nil {
		return err
	}
	s.store.DeleteCache(ctx, "users:list")
	return nil
}

func (s *Store) userCount(ctx context.Context) (int, error) {
	var count int
	err := s.store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) adminCount(ctx context.Context) (int, error) {
	var count int
	err := s.store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role=?`, RoleAdmin).Scan(&count)
	return count, err
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
	Token     string     `json:"token"`
	User      PublicUser `json:"user"`
	ExpiresAt time.Time  `json:"expires_at"`
}

type SessionManager struct {
	store *storage.Store
	ttl   time.Duration
}

func NewSessionManager(store *storage.Store, ttl time.Duration) *SessionManager {
	return &SessionManager{store: store, ttl: ttl}
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
	raw, err := json.Marshal(session)
	if err != nil {
		return Session{}, err
	}
	if err := m.store.Redis.Set(context.Background(), "session:"+token, raw, m.ttl).Err(); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (m *SessionManager) Get(token string) (Session, bool) {
	raw, err := m.store.Redis.Get(context.Background(), "session:"+token).Bytes()
	if err != nil {
		return Session{}, false
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return Session{}, false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		m.Delete(token)
		return Session{}, false
	}
	return session, true
}

func (m *SessionManager) Delete(token string) {
	_ = m.store.Redis.Del(context.Background(), "session:"+token).Err()
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
