package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type principalKey struct{}
type Principal struct { UserID string; Role Role }

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

func AuthMiddleware(tokens TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required"); return
		}
		claims, err := tokens.Verify(parts[1])
		if err != nil { writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required"); return }
		principal := Principal{UserID:claims.Subject, Role:claims.Role}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
		if principal.Role != role { writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions"); return }
		next.ServeHTTP(w, r)
	})
}

type API struct { Auth *AuthService; Tokens TokenManager; Store Store }
type credentialsRequest struct { Email string `json:"email"`; Password string `json:"password"` }
type refreshRequest struct { RefreshToken string `json:"refresh_token"` }
type errorResponse struct { Error string `json:"error"`; Message string `json:"message"` }

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("POST /auth/refresh", a.refresh)
	mux.Handle("GET /api/me", AuthMiddleware(a.Tokens, http.HandlerFunc(a.me)))
	admin := http.HandlerFunc(a.adminUsers)
	mux.Handle("GET /api/admin/users", AuthMiddleware(a.Tokens, RequireRole(RoleAdmin, admin)))
	return mux
}
func (a *API) health(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status":"ok"}) }

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "provide a valid JSON email and password"); return
	}
	user, err := a.Auth.Register(request.Email, request.Password)
	if err != nil {
		if errors.Is(err, ErrEmailExists) { writeError(w, http.StatusConflict, "email_exists", "an account with this email already exists"); return }
		writeError(w, http.StatusBadRequest, "invalid_request", "email or password does not meet requirements"); return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user":publicUser(user)})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "provide a valid JSON email and password"); return
	}
	pair, err := a.Auth.Login(request.Email, request.Password)
	if err != nil { writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect"); return }
	writeJSON(w, http.StatusOK, pair)
}
func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := decodeJSON(w, r, &request); err != nil || request.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required"); return
	}
	pair, err := a.Auth.Refresh(request.RefreshToken)
	if err != nil { writeError(w, http.StatusUnauthorized, "invalid_refresh_token", "refresh token is invalid, expired or already used"); return }
	writeJSON(w, http.StatusOK, pair)
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	user, err := a.Store.UserByID(principal.UserID)
	if err != nil { writeError(w, http.StatusUnauthorized, "unauthorized", "account is unavailable"); return }
	writeJSON(w, http.StatusOK, map[string]any{"user":publicUser(user)})
}
func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	// Demonstrates the authorization boundary. A real endpoint should return a paginated user list.
	writeJSON(w, http.StatusOK, map[string]string{"message":"admin access granted"})
}
func publicUser(user User) map[string]string {
	return map[string]string{"id":user.ID, "email":user.Email, "role":string(user.Role)}
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	w.Header().Set("Content-Type", "application/json")
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body); decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil { return err }
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF { return errors.New("request must contain one JSON value") }
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error:code, Message:message})
}

func main() {
	cfg, err := LoadConfig()
	if err != nil { slog.Error("configuration_error", "error", err); os.Exit(1) }
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level:slog.LevelInfo}))
	store := NewMemoryStore()
	tokens := TokenManager{Secret:cfg.JWTSecret, Issuer:cfg.JWTIssuer, Audience:cfg.JWTAudience, AccessTTL:cfg.AccessTokenTTL}
	auth, err := NewAuthService(store, PasswordHasher{Iterations:cfg.PasswordIterations}, tokens, cfg.RefreshTokenTTL)
	if err != nil { logger.Error("auth_service_init_failed", "error", err); os.Exit(1) }
	if err := auth.BootstrapAdmin(cfg.BootstrapAdminEmail, cfg.BootstrapAdminPass); err != nil {
		logger.Error("bootstrap_admin_failed", "error", err); os.Exit(1)
	}
	api := &API{Auth:auth, Tokens:tokens, Store:store}
	server := &http.Server{Addr:cfg.Address, Handler:api.Routes(), ReadHeaderTimeout:5*time.Second,
		ReadTimeout:10*time.Second, WriteTimeout:15*time.Second, IdleTimeout:60*time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http_server_starting", "address", cfg.Address, "environment", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { serverErr <- err }
		close(serverErr)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil { logger.Error("http_server_shutdown_failed", "error", err) }
	case err := <-serverErr:
		if err != nil { logger.Error("http_server_failed", "error", err); os.Exit(1) }
	}
	logger.Info("http_server_stopped")
}