package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Address string
	AppEnv string
	JWTSecret []byte
	JWTIssuer string
	JWTAudience string
	AccessTokenTTL time.Duration
	RefreshTokenTTL time.Duration
	PasswordIterations int
	BootstrapAdminEmail string
	BootstrapAdminPass string
}

func LoadConfig() (Config, error) {
	get := func(key, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" { return value }
		return fallback
	}
	appEnv := get("APP_ENV", "development")
	if appEnv != "development" && appEnv != "test" && appEnv != "staging" && appEnv != "production" {
		return Config{}, errors.New("APP_ENV must be development, test, staging or production")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if appEnv == "production" { return Config{}, errors.New("JWT_SECRET is required in production") }
		secret = "local-development-only-secret-change-before-deploy-2026"
	}
	if len([]byte(secret)) < 32 { return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes") }
	iterations, err := strconv.Atoi(get("PASSWORD_ITERATIONS", "600000"))
	if err != nil || iterations < 600000 { return Config{}, errors.New("PASSWORD_ITERATIONS must be an integer >= 600000") }
	accessTTL, err := time.ParseDuration(get("ACCESS_TOKEN_TTL", "15m"))
	if err != nil || accessTTL < time.Minute || accessTTL > time.Hour { return Config{}, errors.New("ACCESS_TOKEN_TTL must be between 1m and 1h") }
	refreshTTL, err := time.ParseDuration(get("REFRESH_TOKEN_TTL", "168h"))
	if err != nil || refreshTTL < time.Hour || refreshTTL > 30*24*time.Hour { return Config{}, errors.New("REFRESH_TOKEN_TTL must be between 1h and 720h") }
	return Config{
		Address:get("HTTP_ADDR", ":8080"), AppEnv:appEnv, JWTSecret:[]byte(secret),
		JWTIssuer:get("JWT_ISSUER", "go-backend-daily"), JWTAudience:get("JWT_AUDIENCE", "go-backend-api"),
		AccessTokenTTL:accessTTL, RefreshTokenTTL:refreshTTL, PasswordIterations:iterations,
		BootstrapAdminEmail:strings.ToLower(get("BOOTSTRAP_ADMIN_EMAIL", "")),
		BootstrapAdminPass:os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}, nil
}

// PBKDF2-HMAC-SHA256 keeps this teaching project standard-library-only.
// Prefer a vetted Argon2id implementation for new production systems.
type PasswordHasher struct { Iterations int }

func (h PasswordHasher) Hash(password string) (string, error) {
	if h.Iterations < 1000 { return "", errors.New("password hashing iterations below minimum") }
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil { return "", fmt.Errorf("generate password salt: %w", err) }
	derived := pbkdf2SHA256([]byte(password), salt, h.Iterations, 32)
	return strings.Join([]string{"pbkdf2_sha256", strconv.Itoa(h.Iterations),
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)}, "$"), nil
}

func (h PasswordHasher) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" { return false }
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1000 || iterations > 2000000 { return false }
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > 64 { return false }
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) != 32 { return false }
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	const hashLen = 32
	blocks := (keyLen + hashLen - 1) / hashLen
	result := make([]byte, 0, blocks*hashLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		_, _ = mac.Write(salt)
		_, _ = mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac.Reset()
			_, _ = mac.Write(u)
			u = mac.Sum(u[:0])
			for j := range t { t[j] ^= u[j] }
		}
		result = append(result, t...)
	}
	return result[:keyLen]
}

const jwtHeader = "{\"alg\":\"HS256\",\"typ\":\"JWT\"}"
const maxJWTLength = 8192

type Role string
const ( RoleUser Role = "user"; RoleAdmin Role = "admin" )

type Claims struct {
	Issuer string `json:"iss"`
	Subject string `json:"sub"`
	Audience string `json:"aud"`
	ExpiresAt int64 `json:"exp"`
	IssuedAt int64 `json:"iat"`
	NotBefore int64 `json:"nbf"`
	JWTID string `json:"jti"`
	Role Role `json:"role"`
}

type TokenManager struct {
	Secret []byte
	Issuer string
	Audience string
	AccessTTL time.Duration
	Now func() time.Time
}
func (m TokenManager) now() time.Time { if m.Now != nil { return m.Now() }; return time.Now() }

func (m TokenManager) Issue(user User) (string, Claims, error) {
	now := m.now().UTC()
	jti, err := randomToken(16)
	if err != nil { return "", Claims{}, err }
	claims := Claims{Issuer:m.Issuer, Subject:user.ID, Audience:m.Audience,
		IssuedAt:now.Unix(), NotBefore:now.Unix(), ExpiresAt:now.Add(m.AccessTTL).Unix(),
		JWTID:jti, Role:user.Role}
	payload, err := json.Marshal(claims)
	if err != nil { return "", Claims{}, err }
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(jwtHeader)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return token, claims, nil
}

func (m TokenManager) Verify(token string) (Claims, error) {
	var claims Claims
	if len(token) == 0 || len(token) > maxJWTLength { return claims, errors.New("invalid token") }
	parts := strings.Split(token, ".")
	if len(parts) != 3 { return claims, errors.New("invalid token") }
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || string(headerBytes) != jwtHeader { return claims, errors.New("unsupported token header") }
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil { return claims, errors.New("invalid token signature") }
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) { return claims, errors.New("invalid token signature") }
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil { return claims, errors.New("invalid token payload") }
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil { return Claims{}, fmt.Errorf("invalid token claims: %w", err) }
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF { return Claims{}, errors.New("invalid token claims") }
	now := m.now().Unix()
	if claims.Issuer != m.Issuer || claims.Audience != m.Audience || claims.Subject == "" || claims.JWTID == "" {
		return Claims{}, errors.New("invalid token claims")
	}
	if claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt <= now || claims.NotBefore > now || claims.IssuedAt > now+30 {
		return Claims{}, errors.New("token is expired or not yet valid")
	}
	if claims.Role != RoleUser && claims.Role != RoleAdmin { return Claims{}, errors.New("invalid token role") }
	return claims, nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil { return "", fmt.Errorf("generate random token: %w", err) }
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
func hashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func randomUUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil { return "", fmt.Errorf("generate user ID: %w", err) }
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buffer[0:4], buffer[4:6], buffer[6:8], buffer[8:10], buffer[10:16]), nil
}

var ErrNotFound = errors.New("not found")
var ErrEmailExists = errors.New("email already registered")
var ErrRefreshInvalid = errors.New("refresh token invalid or already used")

type User struct {
	ID string `json:"id"`
	Email string `json:"email"`
	PasswordHash string `json:"-"`
	Role Role `json:"role"`
}
type RefreshSession struct { UserID string; ExpiresAt time.Time }

type Store interface {
	CreateUser(User) error
	UserByEmail(string) (User, error)
	UserByID(string) (User, error)
	CreateRefresh(string, RefreshSession)
	RefreshUser(string, time.Time) (string, error)
	RotateRefresh(string, string, RefreshSession, time.Time) error
}

type MemoryStore struct {
	mu sync.RWMutex
	usersByEmail map[string]User
	usersByID map[string]User
	refreshSessions map[string]RefreshSession
}
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{usersByEmail:map[string]User{}, usersByID:map[string]User{}, refreshSessions:map[string]RefreshSession{}}
}
func (s *MemoryStore) CreateUser(user User) error {
	s.mu.Lock(); defer s.mu.Unlock()
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := s.usersByEmail[user.Email]; exists { return ErrEmailExists }
	s.usersByEmail[user.Email] = user; s.usersByID[user.ID] = user
	return nil
}
func (s *MemoryStore) UserByEmail(email string) (User, error) {
	s.mu.RLock(); defer s.mu.RUnlock()
	user, ok := s.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok { return User{}, ErrNotFound }; return user, nil
}
func (s *MemoryStore) UserByID(id string) (User, error) {
	s.mu.RLock(); defer s.mu.RUnlock()
	user, ok := s.usersByID[id]
	if !ok { return User{}, ErrNotFound }; return user, nil
}
func (s *MemoryStore) CreateRefresh(hash string, session RefreshSession) {
	s.mu.Lock(); defer s.mu.Unlock(); s.refreshSessions[hash] = session
}
func (s *MemoryStore) RefreshUser(hash string, now time.Time) (string, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	session, ok := s.refreshSessions[hash]
	if !ok { return "", ErrRefreshInvalid }
	if !session.ExpiresAt.After(now) { delete(s.refreshSessions, hash); return "", ErrRefreshInvalid }
	return session.UserID, nil
}
func (s *MemoryStore) RotateRefresh(oldHash, newHash string, session RefreshSession, now time.Time) error {
	s.mu.Lock(); defer s.mu.Unlock()
	old, ok := s.refreshSessions[oldHash]
	if !ok || !old.ExpiresAt.After(now) || old.UserID != session.UserID {
		delete(s.refreshSessions, oldHash); return ErrRefreshInvalid
	}
	delete(s.refreshSessions, oldHash)
	s.refreshSessions[newHash] = session
	return nil
}

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrInvalidInput = errors.New("invalid input")

type TokenPair struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
}
type AuthService struct {
	store Store
	hasher PasswordHasher
	tokens TokenManager
	refreshTTL time.Duration
	dummyHash string
}
func NewAuthService(store Store, hasher PasswordHasher, tokens TokenManager, refreshTTL time.Duration) (*AuthService, error) {
	dummyHash, err := hasher.Hash("dummy-password-never-used")
	if err != nil { return nil, err }
	return &AuthService{store:store, hasher:hasher, tokens:tokens, refreshTTL:refreshTTL, dummyHash:dummyHash}, nil
}
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 { return "", ErrInvalidInput }
	return email, nil
}
func (s *AuthService) Register(email, password string) (User, error) {
	email, err := normalizeEmail(email)
	if err != nil { return User{}, err }
	if len(password) < 12 || len(password) > 128 { return User{}, fmt.Errorf("%w: password must be 12-128 bytes", ErrInvalidInput) }
	hash, err := s.hasher.Hash(password); if err != nil { return User{}, err }
	id, err := randomUUID(); if err != nil { return User{}, err }
	user := User{ID:id, Email:email, PasswordHash:hash, Role:RoleUser}
	if err := s.store.CreateUser(user); err != nil { return User{}, err }
	return user, nil
}
func (s *AuthService) BootstrapAdmin(email, password string) error {
	if strings.TrimSpace(email) == "" && password == "" { return nil }
	if strings.TrimSpace(email) == "" || password == "" { return errors.New("both bootstrap admin email and password are required") }
	email, err := normalizeEmail(email); if err != nil { return err }
	if len(password) < 12 || len(password) > 128 { return errors.New("bootstrap admin password must be 12-128 bytes") }
	if _, err := s.store.UserByEmail(email); err == nil { return nil }
	hash, err := s.hasher.Hash(password); if err != nil { return err }
	id, err := randomUUID(); if err != nil { return err }
	return s.store.CreateUser(User{ID:id, Email:email, PasswordHash:hash, Role:RoleAdmin})
}
func (s *AuthService) Login(email, password string) (TokenPair, error) {
	if len(password) == 0 || len(password) > 128 { return TokenPair{}, ErrInvalidCredentials }
	normalized, err := normalizeEmail(email)
	if err != nil { normalized = "invalid@example.invalid" }
	user, findErr := s.store.UserByEmail(normalized)
	storedHash := s.dummyHash
	if findErr == nil { storedHash = user.PasswordHash }
	valid := s.hasher.Verify(password, storedHash)
	if findErr != nil || !valid { return TokenPair{}, ErrInvalidCredentials }
	return s.issuePair(user)
}
func (s *AuthService) Refresh(oldToken string) (TokenPair, error) {
	if len(oldToken) < 32 || len(oldToken) > 256 { return TokenPair{}, ErrRefreshInvalid }
	userID, err := s.store.RefreshUser(hashOpaqueToken(oldToken), time.Now().UTC())
	if err != nil { return TokenPair{}, ErrRefreshInvalid }
	user, err := s.store.UserByID(userID)
	if err != nil { return TokenPair{}, ErrRefreshInvalid }
	newRefresh, err := randomToken(32); if err != nil { return TokenPair{}, err }
	access, claims, err := s.tokens.Issue(user); if err != nil { return TokenPair{}, err }
	now := time.Now().UTC()
	if err := s.store.RotateRefresh(hashOpaqueToken(oldToken), hashOpaqueToken(newRefresh), RefreshSession{UserID:user.ID, ExpiresAt:now.Add(s.refreshTTL)}, now); err != nil {
		return TokenPair{}, ErrRefreshInvalid
	}
	return TokenPair{AccessToken:access, TokenType:"Bearer", ExpiresAt:time.Unix(claims.ExpiresAt,0).UTC(), RefreshToken:newRefresh}, nil
}
func (s *AuthService) issuePair(user User) (TokenPair, error) {
	access, claims, err := s.tokens.Issue(user); if err != nil { return TokenPair{}, err }
	refresh, err := randomToken(32); if err != nil { return TokenPair{}, err }
	s.store.CreateRefresh(hashOpaqueToken(refresh), RefreshSession{UserID:user.ID, ExpiresAt:time.Now().UTC().Add(s.refreshTTL)})
	return TokenPair{AccessToken:access, TokenType:"Bearer", ExpiresAt:time.Unix(claims.ExpiresAt,0).UTC(), RefreshToken:refresh}, nil
}