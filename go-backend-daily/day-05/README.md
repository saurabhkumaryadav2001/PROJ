# Go Backend Daily — Day 05: Structs, Methods & Pointers

**Prerequisites:** Days 01–04. **Next:** Day 06 — Interfaces & Composition.  
**Runnable:** `go run ./day-05/structs` and `go run ./day-05/exercises` (from this folder's parent/repository root when using a module).

## Learning outcomes
- Define structs and named/zero-value literals.
- Model nested data with composition.
- Distinguish value receivers from pointer receivers.
- Understand `&`, `*`, nil pointers, and struct-copy behavior.
- Build domain models and validation methods.

## 1. Structs: named fields in one value
A struct groups related values of different types. Think of a form with named boxes: an Employee has ID, Name, Role, Address and Salary.

```go
type Employee struct {
    ID      int
    Name    string
    Role     string
    Address  Address
    Salary   int
}
```

Use named literals (clear and robust to field order):
```go
employee := Employee{ID: 101, Name: "Saurabh", Role: "Software Engineer", Salary: 25000}
```
A zero-value struct is valid: `var employee Employee` gives numeric fields 0 and strings "". Nested structs express composition ("Employee has an Address"), not inheritance.

## 2. Methods and receivers
A method is a function with a receiver:
```go
func (e Employee) DisplayName() string { return e.Name + " (" + e.Role + ")" }
```
A **value receiver** receives a copy. It is good for read-only calculations such as annual salary.

A **pointer receiver** receives a pointer and can mutate the original:
```go
func (e *Employee) Promote(role string) { e.Role = role }
employee.Promote("Senior Engineer") // Go takes address for addressable values
```
Rule of thumb: use pointer receivers for mutation, to avoid copying large structs, or for consistent method sets. Don't use pointers everywhere by default.

## 3. Pointers: address and dereference
- `&x`: address of x.
- `*p`: value at pointer p (dereference).
- `p == nil`: pointer points to no value.
- Dereferencing nil panics; check nil if it is possible.
- Go permits `p.Field` as shorthand for `(*p).Field`.

## 4. Struct assignment and shallow copies
`copyValue := original` copies struct fields. Updating `copyValue.Name` does not change `original.Name`. However, slice/map/pointer fields are themselves copied as descriptors/pointers; their underlying data may remain shared (shallow copy).

Go has no special constructor keyword. A `NewEmployee(...)` function is a convention to centralize defaults or validation. Return an error when invariants can fail.

## 5. Runnable files
- `structs/main.go`: nested Address/Employee, named literals, value and pointer methods, struct copy, pointer mutation, zero value and NewEmployee.
- `exercises/solutions.go`: Product.InventoryValue, validated Restock and Rename.

Run:
```bash
go run ./day-05/structs
go run ./day-05/exercises
```

## Walk-through of the main file
| Code | Meaning |
|---|---|
| `type Address struct` | Defines address fields |
| `type Employee struct` | Employee domain model |
| `func (e Employee) DisplayName` | Value receiver, formatting only |
| `func (e *Employee) Promote` | Pointer receiver mutates role |
| `GiveRaise` | Validates positive amount then changes original salary |
| `AnnualSalary` | Read-only calculation on a copy |
| `Employee{...}` | Named struct literal |
| `Address: Address{...}` | Nested struct literal |
| `copyOfEmployee := employee` | Copies fields |
| `employeePointer := &employee` | Stores address |
| `employeePointer.Name` | Field access through pointer |
| `var newEmployee Employee` | Zero value |
| `NewEmployee(...)` | Constructor-style initialization helper |

## Exercises
1. Define Product (SKU, Name, Price, Stock) and an InventoryValue value method.
2. Add pointer receiver Restock(int) error; reject non-positive quantities.
3. Add pointer receiver Rename(string) error; trim and reject blank names.
4. Copy Product and explain why changing copy.Name leaves original unchanged.
5. Accept *Product, safely handle nil, and return SKU.

Solutions are in `exercises/solutions.go`.

## Interview questions
1. Struct? Composite type grouping named fields of different types.
2. Method? Function with a receiver.
3. Value vs pointer receiver? Copy vs address/original mutation.
4. `&x`? Address of x. `*p`? Dereference.
5. Nil pointer? Points to no value; dereference panics.
6. Go classes/inheritance? No traditional classes; structs, methods, interfaces and composition.
7. Does struct assignment deep-copy? No; fields copy, referenced data may remain shared.
8. Why named literals? Readability and resilience to field-order changes.
9. Constructors built into Go? No; `NewX` is a convention.
10. Can value call pointer method? Yes if addressable; Go takes its address for the call.
11. Why pointer receiver? Mutation, avoid large copies, consistent method set.
12. When not to use pointers? When simple value semantics are enough.

## Backend use cases
- DTOs: CreateShipmentRequest, ShipmentResponse.
- Domain methods: Shipment.MarkDelivered() can enforce state transitions.
- Configuration: nested DB/HTTP/logging settings.
- Validation: constructors reject invalid IDs/names.
- Workers: pass structs while consciously handling copies/shared fields.
- API response structs will become JSON in Day 09.

## Completion checklist
- [ ] Define and initialize structs.
- [ ] Use nested composition.
- [ ] Explain value and pointer receivers.
- [ ] Explain `&`, `*`, nil.
- [ ] Explain shallow copying.
- [ ] Run both programs and complete exercises.

**Next: Day 06 — Interfaces & Composition:** implicit implementation, interface values, nil-interface traps, type assertions/switches, small interfaces and dependency inversion.
