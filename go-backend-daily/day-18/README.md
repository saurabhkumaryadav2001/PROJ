# Day 18 — Authentication & Authorization in Go

Previous: [Day 17 — Observability](../day-17/README.md)  
Next: Day 19 — PostgreSQL-backed authentication and persistent refresh sessions

## What you build

A runnable Go HTTP API with registration, password hashing, HS256 JWT access tokens, Bearer authentication middleware, user/admin RBAC, opaque refresh tokens with rotation, bootstrap-admin setup, graceful shutdown and tests.

**Analogy:** Authentication is the hotel front desk checking who you are and issuing a key. Authorization is checking which doors that key may open. Being logged in does not mean you can enter the manager's office.

## Authentication vs authorization

| Concept | Question | Example |
|---|---|---|
| Authentication | Who are you? | Verify password or signed access token |
| Authorization | What may you do? | Only admin may call /api/admin/users |

A 401 means authentication is missing or invalid. A 403 means the caller is authenticated but lacks permission.

## JWT anatomy

A JWT has three Base64URL sections: header, payload and signature.

- Header: this sample accepts only HS256.
- Payload: claims such as iss, sub, aud, exp, iat, nbf, jti and role.
- Signature: HMAC-SHA256 over the encoded header and payload.

**Base64URL is encoding, not encryption.** Never put passwords, API keys or private data in JWT claims. The verifier fixes the accepted algorithm, validates the signature in constant time, checks issuer/audience/time claims, rejects oversized tokens and rejects malformed payloads.

HS256 uses one shared secret to sign and verify. Keep it outside Git and use at least 32 bytes of high-entropy secret material. For multi-service production systems, consider asymmetric signing or an OpenID Connect identity provider.

## Password hashing

The teaching implementation uses PBKDF2-HMAC-SHA256 with a random 16-byte salt, 32-byte derived key and 600,000 iterations by default. Stored format:

    pbkdf2_sha256$iterations$salt$derivedKey

A salt makes identical passwords produce different hashes. ConstantTimeCompare avoids an early-exit comparison. Unit tests use 1,000 iterations only to run quickly; application configuration refuses fewer than 600,000.

For new production systems, prefer a vetted Argon2id library (or bcrypt where required by policy). This repository's cryptography is for learning, not a replacement for security review.

## Access vs refresh tokens

| Access token | Refresh token |
|---|---|
| Short-lived JWT (15 minutes) | Opaque random 256-bit value |
| Sent as Authorization: Bearer | Sent only to /auth/refresh |
| Verified by signature and claims | Stored server-side as SHA-256 hash |
| Expires quickly | Rotated after successful use |

Refresh rotation consumes the old hash and stores a new hash atomically. Reuse is rejected. A database-backed implementation must perform this in one transaction or conditional update.

## RBAC

- user can call GET /api/me.
- admin can call GET /api/admin/users.
- Public registration always creates user.
- The JSON decoder rejects unknown fields, so a client cannot submit role=admin.
- An administrator can be bootstrapped only by operator-supplied environment variables.

RBAC does not replace resource ownership checks. A user may read their own order, not another user's order.

## Project files

- authapi/core.go — validated configuration, PBKDF2 password hashing, JWT signing/verification, user/session store interface, refresh rotation and auth service.
- authapi/api.go — Bearer middleware, role middleware, HTTP handlers, strict JSON decoding, server startup and graceful shutdown.
- authapi/auth_test.go — password known-answer test, JWT tamper/expiry/algorithm tests, refresh replay/concurrency tests and HTTP RBAC tests.
- migrations/001_auth_tables.sql — PostgreSQL reference schema for Day 19.
- .env.example, Dockerfile, compose.yaml — local runtime setup.

The existing root go-backend-daily/go.mod is reused. Do not create a nested module.

## Run locally

From the go-backend-daily module root:

~~~bash
gofmt -w day-18/authapi/*.go
go test ./day-18/authapi
go test -race ./day-18/authapi
go vet ./day-18/authapi
go test -cover ./day-18/authapi
go run ./day-18/authapi
~~~

The local-only development secret is built into the development default. Production refuses to start without JWT_SECRET. Never use the example secret in production.

Optional admin bootstrap (set both values):

~~~bash
BOOTSTRAP_ADMIN_EMAIL=admin@example.com \
BOOTSTRAP_ADMIN_PASSWORD='a-long-random-local-password' \
go run ./day-18/authapi
~~~

PowerShell:

~~~powershell
$env:BOOTSTRAP_ADMIN_EMAIL = "admin@example.com"
$env:BOOTSTRAP_ADMIN_PASSWORD = "a-long-random-local-password"
go run ./day-18/authapi
~~~

## API endpoints

| Method | Path | Access |
|---|---|---|
| GET | /healthz | Public |
| POST | /auth/register | Public |
| POST | /auth/login | Public |
| POST | /auth/refresh | Refresh token |
| GET | /api/me | Bearer access token |
| GET | /api/admin/users | Admin access token |

Register:

~~~bash
curl -i -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"a-long-password-123"}'
~~~

Login:

~~~bash
curl -sS -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"a-long-password-123"}'
~~~

Use the returned access token:

~~~bash
curl -i http://localhost:8080/api/me \
  -H 'Authorization: Bearer <access-token>'
~~~

Refresh and rotate:

~~~bash
curl -i -X POST http://localhost:8080/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh-token>"}'
~~~

## Line-by-line walkthrough

### core.go

1. LoadConfig reads environment values, validates environment name, requires a 32-byte secret, enforces a 600,000 password iteration default and bounds token lifetimes.
2. PasswordHasher.Hash generates a cryptographically random salt, derives a key and serializes the algorithm/work factor/salt/hash.
3. PasswordHasher.Verify parses bounded parameters, recomputes the derived key and compares it in constant time.
4. pbkdf2SHA256 implements RFC 8018: compute U1, repeatedly compute Ui = HMAC(password, Ui-1), XOR each Ui into the block result and concatenate blocks.
5. TokenManager.Issue creates standard claims, JSON encodes them, Base64URL encodes header/payload and signs the result with HMAC-SHA256.
6. TokenManager.Verify enforces the exact HS256 header, checks HMAC before trusting claims, then validates issuer, audience, subject, JTI, role, expiry and not-before/issued-at times.
7. randomToken uses crypto/rand; hashOpaqueToken stores only a SHA-256 hash of refresh credentials.
8. Store separates business logic from persistence. MemoryStore uses RWMutex for concurrent requests.
9. RotateRefresh locks once, validates and consumes the old refresh hash, then inserts the new hash. Exactly one concurrent request can win.
10. AuthService.Register always assigns RoleUser; BootstrapAdmin is an operator-only path.
11. Login returns one generic credential error and performs a dummy password verification for unknown accounts.
12. Refresh validates the opaque token, issues a new access token and atomically rotates the refresh credential.

### api.go

1. AuthMiddleware parses Authorization: Bearer, verifies the JWT and puts a small principal in request context.
2. RequireRole checks the principal role and returns 403 when permission is insufficient.
3. Routes registers public and protected method-aware routes.
4. decodeJSON limits request bodies to 16 KiB, rejects unknown fields and rejects trailing JSON values. This blocks client-supplied role fields.
5. publicUser explicitly omits password hashes from responses.
6. main loads validated configuration, constructs dependencies, optionally bootstraps an admin, applies HTTP timeouts and shuts down gracefully on SIGINT/SIGTERM.

## Security checklist before production

- Replace the in-memory store with PostgreSQL; the current store loses users and sessions on restart and is not shared between replicas.
- Use a maintained JWT library or OIDC provider; consider asymmetric signing and key rotation.
- Use vetted Argon2id/bcrypt password hashing and benchmark its cost.
- Add login/register/refresh rate limits, MFA, email verification, password reset and account disable/revocation flows.
- Use HTTPS, secure secrets management and audit events; never log passwords or raw tokens.
- For browser SPAs, keep access tokens short-lived and preferably in memory. Store refresh tokens in HttpOnly; Secure; SameSite cookies with CSRF/origin protections.
- Role claims can become stale until token expiry. Use short lifetimes or server-side permission/version checks for role changes.
- Enforce record ownership and tenant boundaries, not only broad roles.
- In production, rotate/revoke refresh-token families on reuse detection and persist refresh hashes transactionally.

## PostgreSQL reference schema

migrations/001_auth_tables.sql defines app_users and refresh_sessions. It stores password hashes and SHA-256 refresh-token hashes, not plaintext credentials. Day 19 will wire this schema into a PostgreSQL repository.

## Exercises and solutions

1. Authentication vs authorization? Authentication proves identity; authorization checks permission.
2. Is a JWT payload encrypted? No. It is Base64URL-encoded and readable.
3. Why validate iss, aud and exp? To ensure the trusted issuer created it, it targets this API and has not expired.
4. Why reject alg=none? The server must enforce its own allowed algorithm rather than trust attacker-controlled token metadata.
5. Why use a password salt? Identical passwords get different hashes and precomputed tables cannot be reused directly.
6. Why not plain SHA-256 for passwords? It is fast; password hashing should deliberately slow guessing.
7. Why store refresh-token hashes? A database read leak should not immediately reveal active bearer credentials.
8. Why rotate refresh tokens? A previously used/stolen token cannot be replayed indefinitely.
9. Why atomic rotation? Concurrent requests must not both consume the same old token.
10. Why assign user on registration? The client is untrusted and must not grant itself admin privileges.
11. 401 vs 403? 401 is missing/invalid authentication; 403 is authenticated but not permitted.
12. Why is memory storage insufficient for multiple replicas? Each process has separate state and restart loses it; use shared persistent storage.

## Interview questions

- What are the three JWT sections?
- Is a signed JWT confidential?
- What is HS256 and how does it differ from RS256/EdDSA?
- What do iss, sub, aud, exp, iat, nbf and jti mean?
- What is the difference between access and refresh tokens?
- What is refresh-token replay and how does rotation reduce it?
- Why use Argon2id/bcrypt/PBKDF2 rather than a fast hash?
- What is a salt and why is it not secret?
- What is RBAC? How is it different from resource ownership authorization?
- Why should a client never choose its own role?
- What is the principle of least privilege?
- How would you revoke a JWT after logout or a role change?

## Backend use cases

- HRMS: employees see their own profile; HR admins manage employee records.
- Transport system: drivers, dispatchers and finance users have separate permissions.
- Vendor portal: vendors see only their own invoices; finance users approve payments.
- E-commerce: customers see their own orders; support/admin roles have bounded access.
- Internal services: each service identity receives only the scopes it needs.

## Verification

~~~bash
go test ./day-18/authapi
go test -race ./day-18/authapi
go vet ./day-18/authapi
go test -cover ./day-18/authapi
go test -fuzz=FuzzVerifyRejectsMalformedJWT -fuzztime=10s ./day-18/authapi
go test -bench=BenchmarkPasswordVerify -benchmem ./day-18/authapi
~~~

## Day 19

PostgreSQL-backed authentication: implement a PostgreSQL Store, migrations, transactional refresh-token rotation, unique-email conflict handling, repository integration tests and sessions shared across API replicas.
