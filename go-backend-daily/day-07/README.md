# Day 07 — Error Handling Patterns (Go Backend)

**Sequence:** Day 06 covered interfaces and composition. Day 07 covers robust error handling; Day 08 will cover packages, modules, and dependency management.

## Outcomes
- Return errors instead of hiding failures.
- Distinguish ordinary errors from panics.
- Create sentinel errors with `errors.New`.
- Create custom error types that carry structured details.
- Add context with `fmt.Errorf` and `%w` without losing error identity.
- Inspect wrapped errors using `errors.Is` and `errors.As`.
- Map domain errors to HTTP status codes at the transport boundary.
- Avoid leaking internal error details to API clients.

## 1. The mental model

A backend request passes through handler → service → repository. Each layer should return failure information upward. The layer with enough context decides what to do. A repository should not decide HTTP status codes; a service should not print an error and continue; a handler should translate domain failures into transport responses.

**Analogy:** an error is a sealed incident report passed up the chain. Each layer adds context (what it was doing) without destroying the original cause. The API boundary decides how to communicate a safe, understandable response to the caller.

## 2. Errors versus panics

- Return `error` for expected/operational failures: invalid input, missing row, conflict, timeout, unavailable dependency.
- Use `panic` for programmer mistakes or broken invariants that should not be treated as normal control flow.
- Do not use panic for user input or ordinary DB/API failures.
- `recover` is only useful in a deferred function and is generally placed at a process boundary (for example, HTTP middleware) to prevent one request from crashing the server. It does not replace normal error handling.

## 3. Sentinel errors

A sentinel is a package-level error value used to identify a category:

```go
var ErrProductNotFound = errors.New("product not found")

return fmt.Errorf("get product %d: %w", id, ErrProductNotFound)

if errors.Is(err, ErrProductNotFound) {
    // map to 404 or another domain action
}
```

Use `errors.Is` even when an error is wrapped. Do not compare `err.Error()` strings. Text is for humans; error identity is for program logic.

## 4. Custom / typed errors

Use a custom error type when callers need structured information (field, code, retryability, limit, etc.). Implement `Error() string` to satisfy the built-in error interface.

```go
type ValidationError struct {
    Field, Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

var validationErr *ValidationError
if errors.As(err, &validationErr) {
    fmt.Println(validationErr.Field)
}
```

`errors.As` searches the unwrap chain for a matching type and assigns it to the target.

## 5. Wrapping errors (`%w`)

```go
return fmt.Errorf("reserve inventory: %w", err)
```

`%w` adds useful operation context and preserves the original cause in the chain. Use it when callers should still inspect the cause. Use `%v` if you intentionally want only text and do not want to expose the cause programmatically. Wrap at meaningful boundaries; avoid repeated vague messages.

## 6. `errors.Is` vs `errors.As`

- `errors.Is(err, target)`: asks whether the chain contains/represents a specific error value (sentinel/category).
- `errors.As(err, &target)`: asks whether the chain contains a value assignable to a particular error type (structured/custom error).

## 7. Complete runnable example

Path: `go-backend-daily/day-07/errors/main.go`. It models products, inventory reservation and order creation. It demonstrates validation errors, missing products, stock conflicts, wrapping, `errors.Is`, `errors.As`, and HTTP mapping.

### Run
From the module root:

```bash
cd go-backend-daily
go run ./day-07/errors
```

### Expected output

```text
valid order: success
invalid quantity: quantity: must be positive (HTTP mapping 400)
  errors.As: field=quantity message=must be positive
missing product: order validation: get product 99: product not found (HTTP mapping 404)
  errors.Is: product-not-found identified
out of stock: reserve inventory: product 2 has 0 units, requested 1: insufficient stock (HTTP mapping 409)
  errors.Is: insufficient-stock identified
too many units: reserve inventory: product 1 has 3 units, requested 99: insufficient stock (HTTP mapping 409)
  errors.Is: insufficient-stock identified
```

## 8. Main file — line-by-line guide

- Imports `errors` for sentinel creation and chain inspection; `fmt` for formatting and wrapping.
- `ErrProductNotFound` / `ErrInsufficientStock` are stable categories.
- `ValidationError` carries field and message; its `Error()` method makes it an `error`.
- `Product` and `Order` are domain data.
- `ProductRepository` stores sample products in a map (in-memory demonstration).
- `NewProductRepository` initializes map entries.
- `GetByID` returns zero Product plus wrapped sentinel if missing.
- `ReduceStock` propagates lookup failure; if stock is too low it wraps `ErrInsufficientStock`; otherwise it updates map.
- `OrderService` composes repository; constructor injects it.
- `CreateOrder` validates ID/quantity, checks product, reserves stock, and wraps errors with operation context.
- `httpStatusForError` demonstrates boundary mapping: validation → 400, not found → 404, stock conflict → 409, unknown → 500.
- `report` prints errors for this learning demo, then demonstrates `errors.Is` and `errors.As`.
- `main` runs success and failure scenarios so you can see the error chain.

## 9. Exercise

Create a `Withdraw(account *Account, amount int) error` function:

1. Amount must be positive; return a typed `InputError`.
2. Nil account should wrap `ErrAccountNotFound`.
3. Amount above balance should wrap `ErrInsufficientBalance`.
4. Valid withdrawal updates balance.
5. Demonstrate `errors.Is` and `errors.As`.

## 10. Solution

Path: `go-backend-daily/day-07/exercises/solutions.go`.

Run:

```bash
cd go-backend-daily/day-07/exercises
go run solutions.go
```

Expected:

```text
withdraw 100: <nil>
balance: 400
large withdrawal: withdraw 1000 from account 7: insufficient balance
is insufficient: true
bad input: true invalid input: amount must be positive
missing account: true
```

The solution is a complete executable file with sentinel errors, a custom InputError, validation, balance mutation, and safe error inspection.

## 11. Interview questions (short answers)

1. **Why return errors instead of panic?** Errors are expected, explicit, composable failures; panic is exceptional and unwinds the stack.
2. **What is a sentinel error?** A stable package-level error value used as a category.
3. **Why `errors.Is` instead of string compare?** Wrapping preserves identity; text can change and is not a contract.
4. **`errors.Is` vs `errors.As`?** Category/value identity vs matching a concrete error type.
5. **What does `%w` do?** Wraps and preserves the underlying error for inspection.
6. **When use a custom error?** When callers need structured information (field, code, retryability, etc.).
7. **Should repository return HTTP 404?** No; repository returns domain/technical errors, handler maps to HTTP.
8. **Should raw DB errors be sent to clients?** No; log safely and return a stable public message.
9. **Where use recover?** At a boundary (e.g. request middleware) to contain unexpected panics; not for normal failures.
10. **Can you wrap a nil error?** Avoid it; `fmt.Errorf("... %w", nil)` is not a success result.

## 12. Production practices

- Add context: `fmt.Errorf("load customer %d: %w", id, err)`.
- Return errors promptly; don't silently ignore them.
- Keep error messages lowercase/no punctuation convention (idiomatic Go style), and add context at call sites.
- Map internal/domain errors to HTTP status at handler boundary.
- Log internal details with request/trace IDs, but do not expose SQL, credentials, stack traces, or vendor messages to clients.
- Use stable public error codes/messages in API responses.
- Distinguish retryable errors (timeouts, temporary unavailable) from permanent errors (validation, not found).
- Avoid double-logging at every layer; usually log once where the request/job is handled.
- Avoid wrapping nil errors.

## 13. Next

**Day 08 — Packages, Modules & Dependency Management:** package boundaries, `go.mod`, module paths, imports, internal packages, versioning, and reproducible builds.
