# Day 01 — Go Foundations

## Learn
Go is a compiled, statically typed language useful for APIs, cloud services, networking and backend systems. A module is a project root; packages organize code. An executable uses `package main` and starts in `func main()`.

## hello/main.go
```go
package main
import "fmt"
func main() { fmt.Println("Hello, Go backend developer!") }
```
Run `go run ./hello`; build with `go build -o hello ./hello`.

## Variables and zero values
```go
var name string = "Saurabh"
var count int // zero value is 0
active := true // inferred bool; short declaration works inside functions
score := 9.49 // inferred float64
const maxPageSize = 100
```
Zero values: numeric 0, bool false, string empty, pointers/maps/slices nil. Go rejects unused imports and local variables. `gofmt` formats code.

## Formatting
`fmt.Printf("%s %d %.2f %t\\n", name, count, score, active)` uses string, integer, float and bool format verbs. `%v` default value; `%T` type.

## Exercise
1. Print name, role and date on separate lines.
2. Declare a product name, price and stock flag; format all three.
3. Print an unassigned int and explain zero value.
4. Build the config program.
5. Create an unused variable; inspect compiler error.

Interview check: What does compiled mean? Why package main? `var` vs `:=`? What is a zero value? Why unused variables fail?
