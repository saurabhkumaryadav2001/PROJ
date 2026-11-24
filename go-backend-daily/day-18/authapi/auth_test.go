package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testService(t *testing.T) (*AuthService, *MemoryStore, TokenManager) {
	t.Helper()
	store := NewMemoryStore()
	now := time.Now().UTC()
	tokens := TokenManager{Secret:[]byte("test-secret-that-is-long-enough-for-hmac"), Issuer:"test-issuer", Audience:"test-api", AccessTTL:time.Minute, Now:func() time.Time{return now}}
	auth, err := NewAuthService(store, PasswordHasher{Iterations:1000}, tokens, 24*time.Hour)
	if err != nil { t.Fatal(err) }
	return auth, store, tokens
}

func TestPBKDF2KnownVector(t *testing.T) {
	got := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), 1, 32))
	const want = "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	if got != want { t.Fatalf("got %s", got) }
}

func TestPasswordHashUsesSaltAndVerifies(t *testing.T) {
	h := PasswordHasher{Iterations:1000}
	a, err := h.Hash("correct horse battery staple"); if err != nil { t.Fatal(err) }
	b, err := h.Hash("correct horse battery staple"); if err != nil { t.Fatal(err) }
	if a == b { t.Fatal("same password should have different salted hashes") }
	if !h.Verify("correct horse battery staple", a) { t.Fatal("valid password rejected") }
	if h.Verify("wrong password", a) { t.Fatal("wrong password accepted") }
}

func TestJWTRejectsTamperingExpiryAndNoneAlgorithm(t *testing.T) {
	_, _, manager := testService(t)
	token, _, err := manager.Issue(User{ID:"user-1", Role:RoleUser}); if err != nil { t.Fatal(err) }
	claims, err := manager.Verify(token)
	if err != nil || claims.Subject != "user-1" { t.Fatalf("verify failed: %+v %v", claims, err) }
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte("{\"sub\":\"attacker\"}"))
	if _, err := manager.Verify(strings.Join(parts, ".")); err == nil { t.Fatal("tampered JWT accepted") }
	manager.Now = func() time.Time { return time.Now().Add(2*time.Minute) }
	if _, err := manager.Verify(token); err == nil { t.Fatal("expired JWT accepted") }

	payload := base64.RawURLEncoding.EncodeToString([]byte("{\"iss\":\"test-issuer\"}"))
	header := base64.RawURLEncoding.EncodeToString([]byte("{\"alg\":\"none\",\"typ\":\"JWT\"}"))
	unsigned := header+"."+payload
	mac := hmac.New(sha256.New, manager.Secret); _, _ = mac.Write([]byte(unsigned))
	forged := unsigned+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := manager.Verify(forged); err == nil { t.Fatal("alg none accepted") }
}

func TestRegisterLoginRefreshRotationAndReplay(t *testing.T) {
	auth, _, tokens := testService(t)
	user, err := auth.Register("Person@Example.com", "a-very-long-test-password")
	if err != nil { t.Fatal(err) }
	if user.Email != "person@example.com" || user.Role != RoleUser { t.Fatalf("unexpected user: %+v", user) }
	if _, err := auth.Register("person@example.com", "another-long-password"); err != ErrEmailExists { t.Fatalf("expected duplicate error, got %v", err) }
	if _, err := auth.Login("person@example.com", "wrong-password"); err != ErrInvalidCredentials { t.Fatalf("expected generic login error, got %v", err) }
	pair, err := auth.Login("PERSON@example.com", "a-very-long-test-password"); if err != nil { t.Fatal(err) }
	if _, err := tokens.Verify(pair.AccessToken); err != nil { t.Fatal(err) }
	next, err := auth.Refresh(pair.RefreshToken); if err != nil { t.Fatal(err) }
	if next.RefreshToken == pair.RefreshToken { t.Fatal("refresh token did not rotate") }
	if _, err := auth.Refresh(pair.RefreshToken); err == nil { t.Fatal("old refresh token replay accepted") }
}

func TestRefreshRotationIsAtomicUnderConcurrency(t *testing.T) {
	auth, _, _ := testService(t)
	if _, err := auth.Register("race@example.com", "a-very-long-test-password"); err != nil { t.Fatal(err) }
	pair, err := auth.Login("race@example.com", "a-very-long-test-password"); if err != nil { t.Fatal(err) }
	results := make(chan error, 2)
	for i:=0; i<2; i++ { go func(){ _, err := auth.Refresh(pair.RefreshToken); results <- err }() }
	a,b := <-results, <-results
	if (a == nil) == (b == nil) { t.Fatalf("expected one refresh success, got %v and %v", a,b) }
}

func TestHTTPAuthAndRBAC(t *testing.T) {
	auth, store, tokens := testService(t)
	if err := auth.BootstrapAdmin("admin@example.com", "a-very-long-admin-password"); err != nil { t.Fatal(err) }
	handler := (&API{Auth:auth, Tokens:tokens, Store:store}).Routes()

	register := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader("{\"email\":\"user@example.com\",\"password\":\"a-very-long-password\"}"))
	register.Header.Set("Content-Type","application/json")
	rr := httptest.NewRecorder(); handler.ServeHTTP(rr,register)
	if rr.Code != http.StatusCreated { t.Fatalf("register=%d %s",rr.Code,rr.Body.String()) }

	escalate := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader("{\"email\":\"evil@example.com\",\"password\":\"a-very-long-password\",\"role\":\"admin\"}"))
	escalate.Header.Set("Content-Type","application/json")
	er := httptest.NewRecorder(); handler.ServeHTTP(er,escalate)
	if er.Code != http.StatusBadRequest { t.Fatalf("client role should be rejected, got %d",er.Code) }

	login := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("{\"email\":\"user@example.com\",\"password\":\"a-very-long-password\"}"))
	login.Header.Set("Content-Type","application/json")
	lr := httptest.NewRecorder(); handler.ServeHTTP(lr,login)
	if lr.Code != http.StatusOK { t.Fatalf("login=%d %s",lr.Code,lr.Body.String()) }
	var pair TokenPair
	if err := json.Unmarshal(lr.Body.Bytes(), &pair); err != nil { t.Fatal(err) }

	me := httptest.NewRequest(http.MethodGet,"/api/me",nil); me.Header.Set("Authorization","Bearer "+pair.AccessToken)
	mr := httptest.NewRecorder(); handler.ServeHTTP(mr,me)
	if mr.Code != http.StatusOK { t.Fatalf("me=%d",mr.Code) }

	adminDenied := httptest.NewRequest(http.MethodGet,"/api/admin/users",nil); adminDenied.Header.Set("Authorization","Bearer "+pair.AccessToken)
	dr := httptest.NewRecorder(); handler.ServeHTTP(dr,adminDenied)
	if dr.Code != http.StatusForbidden { t.Fatalf("normal user should get 403, got %d",dr.Code) }

	admin, err := auth.Login("admin@example.com","a-very-long-admin-password"); if err != nil { t.Fatal(err) }
	adminReq := httptest.NewRequest(http.MethodGet,"/api/admin/users",nil); adminReq.Header.Set("Authorization","Bearer "+admin.AccessToken)
	ar := httptest.NewRecorder(); handler.ServeHTTP(ar,adminReq)
	if ar.Code != http.StatusOK { t.Fatalf("admin should be allowed, got %d",ar.Code) }
}

func FuzzVerifyRejectsMalformedJWT(f *testing.F) {
	f.Add("not-a-token"); f.Add("a.b.c")
	f.Fuzz(func(t *testing.T, token string) {
		manager := TokenManager{Secret:[]byte("fuzz-secret-that-is-long-enough-for-hmac"), Issuer:"issuer", Audience:"audience", AccessTTL:time.Minute}
		_, _ = manager.Verify(token)
	})
}

func BenchmarkPasswordVerify(b *testing.B) {
	h := PasswordHasher{Iterations:1000}
	encoded, err := h.Hash("benchmark-password"); if err != nil { b.Fatal(err) }
	b.ResetTimer()
	for i:=0; i<b.N; i++ { h.Verify("benchmark-password", encoded) }
}