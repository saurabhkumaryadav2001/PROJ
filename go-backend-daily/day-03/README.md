# Day 03 — Control Flow: if, for, range, switch, break & continue

**Sequence:** Day 01 Go foundations → Day 02 functions/errors → **Day 03 control flow**. **Next:** Day 04 — arrays, slices and maps.

## Outcomes
- make decisions with `if`, `else if`, `else`;
- use Go's only loop keyword: `for`;
- choose condition, classic and `range` loops;
- use `switch` for status/role routing;
- use `break` to exit and `continue` to skip an iteration;
- use guard clauses and early returns in backend code.

## Run
From `go-backend-daily/`:
```bash
go run ./day-03/control-flow
go run ./day-03/exercises
```

## 1. Control flow = traffic signals
A program normally runs top-to-bottom. Control flow lets it choose a route: `if` decides based on a condition; `for` repeats work; `switch` chooses one branch; `break` exits; `continue` skips to the next loop iteration; `return` leaves the current function.

Why it matters in backend: validate requests, process batches, route shipment statuses, retry work, stop when a record is found, and enforce business rules.

## 2. if / else
```go
if age >= 18 {
    fmt.Println("adult")
} else {
    fmt.Println("minor")
}
```
The condition must be a `bool`; Go does not treat 1, 0, or strings as truthy/falsy. Braces are required. `else if` checks another condition only if previous ones failed. Conditions can include `&&` (AND), `||` (OR), and `!` (NOT).

### Short statement
```go
if err := validate(input); err != nil {
    return err
}
```
The variable declared after `if` is scoped to that if/else. This is idiomatic error checking.

## 3. Guard clauses
A guard clause rejects an invalid/exceptional case immediately:
```go
if strings.TrimSpace(destination) == "" {
    return errors.New("destination is required")
}
// normal business logic continues
```
This keeps the main path less nested and easier to read. Avoid deeply nested `if` blocks when early return/continue makes intent clearer.

## 4. Go's only loop keyword: for
Go has no separate `while` or `do-while`. Use `for`.

### A) Condition-only (while-like)
```go
count := 0
for count < 3 {
    fmt.Println(count)
    count++
}
```
### B) Classic three-part loop
```go
for i := 0; i < 3; i++ {
    fmt.Println(i)
}
```
`i := 0` initialization; `i < 3` condition; `i++` post-step. Each part can be omitted, but semicolons remain when using classic form.
### C) range loop
```go
for index, value := range names {
    fmt.Println(index, value)
}
```
Use `_` to ignore a value: `for _, shipment := range shipments`. `range` over slices/arrays gives index and value; over maps gives key/value; strings gives byte index and rune; channels gives received values (later lessons).

## 5. break vs continue vs return
- `break`: exits the nearest loop or switch immediately.
- `continue`: skips remaining body and starts next iteration.
- `return`: exits the entire function, not just a loop.
```go
for attempt := 1; attempt <= 3; attempt++ {
    if attempt == 2 { continue }
    if attempt == 3 { break }
}
```
Use `continue` for invalid batch items when the batch should keep going. Use `break` when a stop condition is reached. Use `return` when current function's work is finished or failed.

## 6. switch
```go
switch status {
case "pending":
    dispatch()
case "in_transit":
    monitor()
case "delivered", "cancelled":
    archive()
default:
    return errors.New("unsupported status")
}
```
Go switch cases do not fall through by default. Multiple values can share a case. `default` handles unmatched values. You can switch without an expression:
```go
switch {
case score >= 90: grade = "A"
case score >= 75: grade = "B"
default: grade = "C"
}
```
Prefer switch when matching one value against many known alternatives; prefer if/else for ranges or a small number of boolean conditions.

## 7. Runnable backend example
`control-flow/main.go` processes shipment records:
1. `main` builds sample shipments and calls `processShipments`.
2. `processShipments` uses `range` to visit each shipment.
3. A guard clause rejects blank destinations and `continue`s.
4. Status is normalized using `strings.TrimSpace` and `strings.ToLower`.
5. `switch` routes pending, in-transit, delivered/cancelled and unknown statuses.
6. Nested `if` checks pending shipment weight.
7. It returns processed/rejected counts.
8. `main` demonstrates a classic loop with `continue` and `break`.
9. `findShipment` uses index loop and early `return` when found; otherwise returns zero struct + false.

## 8. Exercises (attempt before solutions)
1. `classifyTemperature(celsius float64) string`: below 0 freezing; below 20 cold; 20–30 comfortable inclusive; above 30 hot.
2. `countPositive(numbers []int) int`: count values > 0 using `range` and `continue`.
3. `findFirstEven(numbers []int) (int,bool)`: return first even; false if none.
4. `normalizeRole(role string) string`: admin/administrator → ADMIN; driver → DRIVER; maintenance/mechanic → MAINTENANCE; otherwise UNKNOWN. Ignore case and surrounding spaces.
5. Add a maximum of 5 attempts to a retry loop and stop as soon as success is simulated.
Solutions 1–4 are in `exercises/solutions.go`. Run: `go run ./day-03/exercises`.

## 9. Interview questions
1. **Does Go have while?** No; condition-only `for` is used.
2. **What is range?** Iterates supported collections and returns index/key and value (depending on type).
3. **break vs continue?** Exit loop vs skip current iteration.
4. **break vs return?** Exit nearest loop/switch vs exit current function.
5. **Does Go switch fall through?** No, not by default.
6. **When use switch over if/else?** Many discrete values (status, role, command); if/else for ranges/boolean logic.
7. **What is a guard clause?** Early return/continue for invalid or exceptional cases.
8. **Can if initialize a variable?** Yes: `if err := fn(); err != nil { ... }`; variable scope is that if statement.
9. **What does `_` do in range?** Discards a value you don't need.
10. **How process a batch with one invalid record?** Validate, count/report it, `continue` if remaining records should proceed.

## 10. Backend use cases
- API validation: `if request.ID <= 0 { return badRequest }`.
- Shipment state machine: `switch status`.
- Batch import: `for _, row := range rows`; invalid row → `continue`.
- Retry loop: bounded `for attempt := 1; attempt <= max; attempt++`.
- Search: iterate and `return` immediately on match.
- Role authorization: switch/lookup role and reject unsupported role.
- Pagination: `for page := 1; page <= totalPages; page++`.

## Day 03 checklist
- [ ] I can write `if/else if/else`.
- [ ] I know Go only uses `for` for loops.
- [ ] I can use classic and `range` loops.
- [ ] I can explain `break`, `continue`, `return`.
- [ ] I can route status using `switch`.
- [ ] I completed exercises 1–4.

**Next:** Day 04 — arrays, slices, maps, capacity, append, copy, and common backend data structures.
