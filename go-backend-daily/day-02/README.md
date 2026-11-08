# Day 02 — Functions, parameters, returns, scope & errors

**Sequence:** Day 01 (Go foundations) is complete. Day 02 is functions and errors. **Next: Day 03 — control flow, loops and `switch`.**

## Outcomes
Write/call functions; distinguish parameters from arguments; return one or multiple values; understand local/package scope; validate inputs; return/wrap/inspect errors; compose small functions into a backend operation.

## Run
From `go-backend-daily/`:
```bash
go run ./day-02/functions
go run ./day-02/errors-demo
go run ./day-02/exercises
```
Expected highlights:
```
Hello, Saurabh! Welcome to Go backend learning.
Order total after discount: ₹1980.00
Expected failure: calculate subtotal: order must contain at least one item
Caller recognized the empty-order condition.
```
**Money caution:** `float64` is used for learning only. Production money should use integer paise/cents or a decimal representation; binary floating-point can be imprecise.

## 1. Function = reusable machine
Analogy: a factory machine takes input, performs one defined job, and produces output. Functions prevent duplicate logic and make business rules testable.

```go
func add(a, b int) int {
    return a + b
}
sum := add(10, 20)
```
- `func` declares a function; `add` is its name.
- `(a, b int)` are typed parameters (Go allows shortening `a int, b int` to `a, b int`).
- final `int` is return type; `{}` contains the body; `return` sends the result back and ends that call.
- Declaration does not execute; `add(10,20)` is the call.

## 2. Parameter vs argument
**Parameter** is the name in declaration (`name` in `func greet(name string)`). **Argument** is the concrete value passed (`"Saurabh"` in `greet("Saurabh")`). Go passes values by value: a function gets a copy. Reassigning a copied `int` won't mutate the caller. Slices/maps have shared underlying data through copied descriptors; that's a later lesson nuance.

## 3. Return values & Go's `(value, error)`
A function can return zero, one, or several values. A common backend signature is `(result, error)`:

```go
func divide(a, b float64) (float64, error) {
    if b == 0 { return 0, errors.New("division by zero") }
    return a / b, nil
}
result, err := divide(10, 2)
if err != nil {
    // handle failure first; don't use result
    return
}
fmt.Println(result)
```
Success returns result + `nil` error; failure returns a zero/placeholder + non-nil error. `nil` means no error. Go errors are values—not exceptions—so callers decide whether to retry, log, return HTTP, or stop. **Always check `err` before using the result.**

## 4. Scope and naming
- **Local:** declared in a function/block; visible there and nested blocks.
- **Package-level:** declared outside functions; available to files in the same package (subject to rules).
- **Exported:** package-level identifier begins uppercase (`CalculateTotal`), accessible from other packages.
- **Unexported:** begins lowercase (`calculateTotal`), package-only.
Prefer local variables. Mutable global state causes hidden coupling and can create server race conditions. Stable package-level sentinel errors are reasonable.

## 5. Errors are values
- `errors.New("message")`: simple error.
- `fmt.Errorf("operation: %w", err)`: add context and preserve error chain.
- `errors.Is(err, target)`: detect sentinel/value through wrapping.
- `errors.As(err, &target)`: extract matching typed error.
- `%v` formats error text but does not wrap; `%w` wraps.
Never silently ignore errors; handle intentionally.

**Error flow in backend:** validate at boundary → return early → wrap lower-level error with operation context → handle at the layer with enough knowledge → HTTP boundary maps known domain errors to 400/404/409/500. Never expose raw DB/internal errors to clients.

## 6. Main runnable example: order total
File: `functions/main.go`.
1. `package main` makes executable program.
2. Imports: `errors` (construct/match errors), `fmt` (format/print), `strings` (trim).
3. `ErrEmptyOrder` is a sentinel condition.
4. `Item` groups name, price and quantity fields (structs are covered Day 05).
5. `main` prints greeting, builds input, captures `(total, err)`, handles failure before output, then demonstrates an expected error and `errors.Is`.
6. `greet(name string) string` accepts one string and returns a string.
7. `calculateSubtotal(items []Item) (float64,error)` rejects empty orders; initializes local subtotal; loops items; validates trimmed name, positive price and quantity; converts quantity explicitly with `float64`; accumulates line totals; returns `subtotal,nil`.
8. `applyDiscount` validates 0–100 then calculates.
9. `calculateTotal` composes two small functions. On failure it wraps with `%w` and returns early; on success it returns final total + nil.
10. `fmt.Printf("%.2f", total)` prints two decimal places.

## 7. Typed error example
`errors-demo/main.go` defines:
- `ErrUserNotFound`: sentinel condition for `errors.Is`.
- `ValidationError{Field,Message}`: structured error implementing `Error() string`.
- `findUser`: invalid ID returns typed validation error; ID 42 succeeds; other IDs wrap not-found.
- `main`: uses `errors.As` to extract validation details and `errors.Is` to recognize not-found. A real API could map invalid ID to HTTP 400 and not-found to 404.

## 8. Exercises (attempt before solutions)
1. Implement `max(a,b int) int`.
2. Implement `safeDivide(a,b float64) (float64,error)`; reject divisor zero.
3. Implement `priceAfterDiscount(price,percent float64) (float64,error)`; reject negative price and percentage outside 0–100.
4. Implement `calculateShipping(weightKg float64) (float64,error)`: reject <=0; ₹50 up to 1kg and ₹30 for each additional kg (simplify rounding).
5. Make `calculateTotal` reject 101% discount. Explain why `%w` is more useful than `%v`.
Solutions 1–3 are in `exercises/solutions.go`; run it as above. Solve 4–5 independently.

## 9. Interview questions (short answers)
1. **What is a function?** Reusable operation with a clear input/output contract.
2. **Parameter vs argument?** Declared name vs supplied value.
3. **Multiple return values?** Yes; `(value,error)` is common.
4. **Why explicit errors?** Caller controls failure; no hidden exception flow.
5. **Meaning of nil error?** Success/no error.
6. **Scope?** Region where a name is accessible.
7. **Why small functions?** Single responsibility, reuse, readability, testability.
8. **`errors.New` vs `fmt.Errorf`?** Simple error vs formatted/contextual error; `%w` can wrap.
9. **`%w` vs `%v`?** `%w` preserves chain; `%v` only formats text.
10. **`errors.Is` vs `errors.As`?** Match a target vs extract a matching type.
11. **Should a service print errors?** Usually return them; appropriate layer logs/maps/retries.
12. **How map repository error to HTTP?** Wrap context, map known domain errors, log unexpected errors and return safe 500.

## 10. Backend use cases
`CreateUser(input) (User,error)` • `GetOrder(id) (Order,error)` • `CalculateInvoice(lines) (Money,error)` • `SaveShipment(dto) (Shipment,error)` • `SendEmail(to string) error` • HTTP handler calls service, checks error and maps it to response.

## Day 02 checklist
- [ ] Write/call a function.
- [ ] Explain parameter vs argument.
- [ ] Return `(value,error)` and check `err != nil`.
- [ ] Explain local/package scope.
- [ ] Wrap with `%w`; use `errors.Is` / `errors.As`.
- [ ] Solve exercises 1–3 without copying.

**Next:** Day 03 — `if`, `for`, `switch`, `break`, `continue`, and request-processing flow.
