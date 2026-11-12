# Day 06 — Interfaces & Composition (Go Backend)

**Sequential curriculum:** Day 05 was Structs, Methods & Pointers. Day 06 is Interfaces & Composition. Day 07 will be Error Handling Patterns.

## Learning outcomes
- Explain what interfaces solve and how Go implements them implicitly.
- Understand interface values, method sets, pointer receivers, type assertions/switches, and typed nil.
- Use small interfaces, composition, dependency inversion, and fakes for testing.
- Build a replaceable payment service with a runnable example.

## 1. Why interfaces?
Imagine an order service that must charge a customer. It should not be tightly coupled to one card gateway. Tomorrow the company may use a wallet, another provider, or a fake in tests. The service needs only the ability to **Charge**.

Analogy: a wall socket is a contract. Any appliance with a compatible plug can use it; the socket does not care about the brand.

```go
type PaymentProcessor interface {
    Charge(amountCents int) error
}
```

An interface defines behavior (method signatures), not data fields.

## 2. Implicit interface implementation
Any type whose method set contains the exact required methods satisfies the interface. Go has no `implements` keyword.

```go
type CardProcessor struct{}
func (CardProcessor) Charge(amountCents int) error { return nil }

var _ PaymentProcessor = CardProcessor{} // compile-time check
```

Parameter names do not matter; method name, parameter types, and return types must match. A value receiver means both `CardProcessor` and `*CardProcessor` have that method. A pointer receiver means only `*CardProcessor` satisfies it.

## 3. Interface values and typed nil
Conceptually an interface value holds a dynamic concrete type plus a dynamic value:
- nil interface: no dynamic type and no dynamic value;
- populated interface: concrete type and value;
- typed nil: concrete type exists, but its value is a nil pointer.

```go
var p PaymentProcessor = nil
var card *CardProcessor = nil
var q PaymentProcessor = card
fmt.Println(p == nil) // true
fmt.Println(q == nil) // false: dynamic type is *CardProcessor
```

A typed nil inside an interface is a classic bug. Validate dependencies and avoid returning typed nil pointers as interface values.

## 4. Type assertion and type switch
A type assertion checks/extracts a concrete value:
```go
card, ok := p.(*CardProcessor) // comma-ok avoids panic
```
A type switch handles multiple dynamic types:
```go
switch v := p.(type) {
case CardProcessor: ...
case *WalletProcessor: ...
default: ...
}
```
Use these at boundaries/diagnostics or for optional capabilities; do not repeatedly assert types instead of using the interface contract.

## 5. Composition and dependency inversion
Go favors composing small types rather than class inheritance. An `OrderService` can hold a `PaymentProcessor` and a `Notifier`. The service depends on the behavior contract; production injects a gateway, tests inject a fake. Keep interfaces small and usually define them near the consumer that needs them.

## 6. Complete runnable example
Path: `go-backend-daily/day-06/interfaces/main.go`

```go
package main

import (
    "errors"
    "fmt"
)

type PaymentProcessor interface { Charge(amountCents int) error }
type Notifier interface { Notify(message string) }

type CardProcessor struct { Provider string }
func (c CardProcessor) Charge(amountCents int) error {
    if amountCents <= 0 { return errors.New("amount must be positive") }
    fmt.Printf("[%s] charged %d cents\\n", c.Provider, amountCents)
    return nil
}

type WalletProcessor struct { WalletID string }
func (w *WalletProcessor) Charge(amountCents int) error {
    if amountCents <= 0 { return errors.New("amount must be positive") }
    fmt.Printf("[wallet %s] charged %d cents\\n", w.WalletID, amountCents)
    return nil
}

type ConsoleNotifier struct{}
func (ConsoleNotifier) Notify(message string) { fmt.Println("notification:", message) }

type OrderService struct { payment PaymentProcessor; notifier Notifier }
func NewOrderService(payment PaymentProcessor, notifier Notifier) (*OrderService, error) {
    if payment == nil { return nil, errors.New("payment processor is required") }
    if notifier == nil { return nil, errors.New("notifier is required") }
    return &OrderService{payment: payment, notifier: notifier}, nil
}
func (s *OrderService) PlaceOrder(totalCents int) error {
    if totalCents <= 0 { return errors.New("order total must be positive") }
    if err := s.payment.Charge(totalCents); err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }
    s.notifier.Notify("order paid")
    return nil
}

var _ PaymentProcessor = CardProcessor{}
var _ PaymentProcessor = (*WalletProcessor)(nil)
var _ Notifier = ConsoleNotifier{}

func describeProcessor(p PaymentProcessor) string {
    switch v := p.(type) {
    case CardProcessor: return "card provider: " + v.Provider
    case *WalletProcessor:
        if v == nil { return "typed-nil wallet" }
        return "wallet: " + v.WalletID
    default: return fmt.Sprintf("unknown processor (%T)", p)
    }
}

func main() {
    notifier := ConsoleNotifier{}
    card := CardProcessor{Provider: "DemoCard"}
    service, err := NewOrderService(card, notifier)
    if err != nil { panic(err) }
    if err := service.PlaceOrder(1250); err != nil { fmt.Println("error:", err) }
    fmt.Println(describeProcessor(card))

    wallet := &WalletProcessor{WalletID: "W-42"}
    walletService, err := NewOrderService(wallet, notifier)
    if err != nil { panic(err) }
    if err := walletService.PlaceOrder(700); err != nil { fmt.Println("error:", err) }
    fmt.Println(describeProcessor(wallet))

    var typedNil *WalletProcessor
    var processor PaymentProcessor = typedNil
    fmt.Println("typed nil interface is nil?", processor == nil) // false
    fmt.Println("plain nil interface is nil?", (PaymentProcessor)(nil) == nil) // true
}
```

### Run
From repo root: `cd go-backend-daily && go run ./day-06/interfaces`

Expected:
```text
[DemoCard] charged 1250 cents
notification: order paid
card provider: DemoCard
[wallet W-42] charged 700 cents
notification: order paid
wallet: W-42
typed nil interface is nil? false
plain nil interface is nil? true
```

## 7. Important lines explained
- `type PaymentProcessor interface`: defines the behavior contract.
- `Charge(amountCents int) error`: exact method signature required.
- `CardProcessor.Charge`: value receiver; CardProcessor's method set satisfies the interface.
- `WalletProcessor.Charge`: pointer receiver; *WalletProcessor satisfies it, not WalletProcessor value.
- `var _ PaymentProcessor = CardProcessor{}`: compiler checks implementation.
- `OrderService.payment PaymentProcessor`: service is decoupled from concrete gateway.
- `NewOrderService`: validates required dependencies.
- `PlaceOrder`: charge first, wrap error with %w, notify only on successful payment.
- `switch v := p.(type)`: inspect interface's dynamic concrete type.
- `var processor PaymentProcessor = typedNil`: dynamic type remains *WalletProcessor, so interface != nil.

## 8. Exercises
1. Add `RefundProcessor` with `Refund(amountCents int) error`; implement for card and wallet.
2. Create `FakePayment` recording last amount and failing on amount 13; use it without a real gateway.
3. Add `Receipt` with `CreateReceipt(orderID string) string` and a console implementation.
4. Write a type switch that returns card, wallet, or unknown.
5. Explain why a nil *WalletProcessor assigned to PaymentProcessor is not a nil interface.

## 9. Solutions
Path: `go-backend-daily/day-06/exercises/solutions.go`. Run:
`cd go-backend-daily/day-06/exercises && go run solutions.go`

Expected:
```text
remaining charge: 60
fake recorded: 250
processor: card
receipt for ORD-1
```
The solution implements RefundProcessor, FakePayment, Receipt, and a type switch. Card uses a pointer receiver because it mutates LastCharge.

## 10. Interview questions (short answers)
1. **Interface?** A set of method signatures describing behavior.
2. **How is it implemented?** Implicitly when the type's method set contains the required methods.
3. **Why no implements keyword?** Structural satisfaction decouples producers and consumers.
4. **Value vs pointer method sets?** Value method set contains value-receiver methods; pointer method set contains both value and pointer-receiver methods.
5. **Interface value?** Conceptually dynamic type plus dynamic value.
6. **Nil vs typed nil?** Nil interface has no dynamic type; typed nil has a dynamic type and nil concrete pointer.
7. **Type assertion?** Checks/extracts concrete type; use comma-ok form.
8. **Type switch?** Branches on dynamic concrete type.
9. **Why small interfaces?** Easier to satisfy, mock, compose, and maintain.
10. **Testing benefit?** Inject fakes for DB, gateway, notifier, clock.
11. **Interface for every struct?** No; add abstraction when substitution/testing boundary is useful.
12. **Interface vs concrete dependency?** Interface reduces coupling but should be purposeful, not overused.

## 11. Backend use cases
- Repository: PostgreSQL implementation vs in-memory fake.
- PaymentProcessor: real gateway vs mock.
- Notifier: email/SMS/push.
- Storage: local disk vs S3.
- Clock: system time vs deterministic fake.
- HTTP handler/service: depend on narrow interfaces, not vendor SDKs.

## 12. Common mistakes
- Huge interfaces with too many methods.
- Type assertions everywhere instead of calling the contract.
- Forgetting pointer-receiver method sets.
- Returning typed nil pointers as interface values.
- Assuming an interface stores a concrete struct directly.
- Premature interfaces without a real substitution need.

## Next: Day 07 — Error Handling Patterns
Sentinel errors, custom error types, wrapping/unwrapping (`errors.Is`/`errors.As`), validation errors, error context, and backend error boundaries.
