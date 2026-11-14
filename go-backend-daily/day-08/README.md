# Day 08 — Packages, Modules & Dependency Management

**Sequence:** Day 07 covered error handling. Day 08 covers how Go code is organized and built across packages and modules. Day 09 will cover JSON and file I/O.

## Learning outcomes
- Distinguish a package from a module.
- Understand `go.mod`, module paths, imports, and `go.sum`.
- Understand exported (`Uppercase`) and unexported (`lowercase`) identifiers.
- Create a multi-package backend with a clean dependency direction.
- Understand `internal` package boundaries.
- Use Go commands to inspect, format, test, build, and manage dependencies.
- Avoid circular imports and package-level design mistakes.

## 1. Package vs module

A **package** is a group of Go source files compiled together that share one package name and directory. It is a code organization and visibility boundary.

A **module** is a versioned collection of packages, rooted at a directory containing `go.mod`. It is the unit Go uses for dependency/version management.

Analogy:
- Package = one department/team inside a company.
- Module = the whole company, with its identity and dependency contract.
- `go.mod` = company registration + dependency manifest.
- Import path = the address used to reach a department/package.

## 2. Package declaration and directory rule

Every `.go` file begins with a package declaration (except special generated/build-tag cases). Ordinary files in the same directory should use the same package name and are compiled together. A directory normally represents one package; don't mix unrelated packages in one directory.

```go
package catalog
```

The package name is usually the short name used inside source. It does not have to equal the final segment of the import path, though matching them is conventional.

## 3. Module identity and go.mod

The module path is declared at the module root:

```go
module github.com/saurabhkumaryadav2001/go-backend-daily

go 1.23
```

An import such as:

```go
import "github.com/saurabhkumaryadav2001/go-backend-daily/day-08/shop/internal/catalog"
```

is resolved relative to that module root. Local package imports use the full module path + directory path, not filesystem-relative `./` imports.

Commands:
- `go mod init <module-path>` — create module identity (once).
- `go mod tidy` — add required modules/remove unused requirements and maintain sums.
- `go list ./...` — list packages in the module.
- `go test ./...` — test all packages.
- `go build ./...` — compile all packages.
- `go run ./path/to/package` — run a package with `main`.

## 4. Exported vs unexported names

Go uses capitalization as its visibility rule:
- `Product`, `NewProduct`, `SKU` — exported (accessible from another package).
- `normalizeSKU`, `validateSKU` — unexported (only accessible inside `catalog`).

This is not public/private on a class; it applies to identifiers such as types, functions, methods, constants, and fields.

If a struct field is unexported, other packages cannot set it directly. A constructor can validate input and establish invariants.

## 5. Import paths and package aliases

Import path identifies a package. The package's declared name is the identifier used after import.

```go
import "net/http"
http.ListenAndServe(...)
```

An alias is useful for conflicts or clarity:

```go
import catalogpkg "example.com/app/internal/catalog"
```

Use aliases sparingly. Prefer meaningful package names, not aliases that repeat the package name.

## 6. internal packages

A directory named `internal` creates a Go-enforced import restriction. Packages beneath it can only be imported by code inside the parent tree allowed by Go's internal rule. It prevents consumers outside your application tree from depending on implementation details.

In this lesson, `shop/internal/catalog` and `shop/internal/httpapi` are application internals. The `cmd/api` executable can import them because it is inside the `shop` tree. A different module cannot import them.

## 7. Dependency direction

A healthy small backend structure:

```text
cmd/api                 composition root: wire dependencies and start server
   |
   v
internal/httpapi         transport: HTTP request/response
   |
   v
internal/catalog         domain/application behavior
```

Keep lower-level/domain packages independent of HTTP. Avoid circular imports: if package A imports B, B cannot import A. If two packages need each other, rethink ownership or introduce a small interface at the consumer boundary.

## 8. Complete runnable project

Paths:
- `day-08/shop/cmd/api/main.go`
- `day-08/shop/internal/catalog/catalog.go`
- `day-08/shop/internal/httpapi/handler.go`

Run from the module root:

```bash
go run ./day-08/shop/cmd/api
```

Then:

```bash
curl http://localhost:8080/products
curl http://localhost:8080/health
```

The server uses only the standard library; no external dependency is needed, so this lesson does not generate a `go.sum`.

## 9. Code walkthrough

### internal/catalog/catalog.go
- `package catalog` names this package.
- `Product` is exported; its fields are exported so the HTTP package can read them.
- `NewProduct` is an exported constructor and validates a SKU/name.
- `normalizeSKU` and `validateSKU` are private implementation details.
- `List` returns a copy of the product slice so callers cannot replace the repository's slice elements directly.

### internal/httpapi/handler.go
- `package httpapi` is a separate transport package.
- Imports `encoding/json`, `net/http`, and the catalog package by module path.
- `Handler` stores the catalog dependency.
- `NewHandler` is the public constructor.
- `ServeHTTP` implements `http.Handler`.
- Routes are selected by path; JSON encoding is performed at the HTTP boundary.
- Domain package does not import `net/http`, so business code can be reused by workers/CLI.

### cmd/api/main.go
- `package main` creates an executable.
- Imports the HTTP package and standard library.
- Wires the catalog into the handler (composition root).
- Starts the server and reports startup errors.

## 10. Exercise

Create a `internal/users` package:
1. Define exported `User` with `ID` and `Name`.
2. Add `NewUser(id int, name string) (User, error)` that rejects ID <= 0 and blank names.
3. Keep `normalizeName` unexported.
4. Import it from a separate `cmd/demo/main.go` using the full module import path.
5. Run `go run ./day-08/exercises/demo`.

## 11. Exercise solution

Path: `day-08/exercises/demo/main.go` (plus `day-08/exercises/internal/users/user.go`).

Run:

```bash
go run ./day-08/exercises/demo
```

Expected:

```text
user: {ID:42 Name:Saurabh}
invalid user: user id must be positive
```

## 12. Interview questions

1. **Package vs module?** Package is a compilation/namespace unit; module is a versioned collection of packages with `go.mod`.
2. **What is a module path?** Canonical import prefix identifying the module.
3. **Why full import paths instead of relative imports?** Stable package identity and module-aware dependency resolution.
4. **How does Go export an identifier?** Its name begins with an uppercase Unicode letter.
5. **What does internal do?** Restricts imports to the permitted parent tree.
6. **What is go.sum?** Checksums used to verify downloaded module content; it is not a lockfile listing every runtime dependency.
7. **What does go mod tidy do?** Reconciles module requirements and checksums with packages/tests in the module.
8. **Can two packages import each other?** No; import cycles are rejected.
9. **Where should main live?** In a `package main` directory, commonly under `cmd/<app>`.
10. **Should domain packages import net/http?** Usually no; keep transport concerns at the edge.
11. **What is a composition root?** The place (often `main`) that constructs and connects dependencies.
12. **Why use constructors?** Validate input and establish valid state instead of exposing unsafe initialization.

## 13. Backend use cases

- Split HTTP handlers, domain logic, persistence, config, and workers into packages.
- Reuse domain/service packages from API and background worker binaries.
- Keep DB implementation behind a package boundary.
- Keep internal implementation private with `internal`.
- Pin external library versions in `go.mod`; review upgrades.
- Make builds repeatable with module checksums and CI commands.

## 14. Production checklist

- One coherent package per directory.
- Short, clear package names; avoid `util`, `common`, `misc` dumping grounds.
- Export only what another package needs.
- Use constructors where invariants matter.
- Keep domain packages independent of HTTP and database driver specifics where practical.
- Avoid import cycles; dependency arrows should point one way.
- Run `gofmt`, `go test ./...`, `go vet ./...`, `go build ./...`.
- Do not add dependencies for functionality already in the standard library.
- Review licenses, maintenance, vulnerabilities, and version changes before adding dependencies.

## 15. Next lesson — Day 09

JSON and File I/O: structs and JSON tags, encoding/json, marshal/unmarshal, validation after decoding, reading/writing files, streaming JSON, and safe file operations.
