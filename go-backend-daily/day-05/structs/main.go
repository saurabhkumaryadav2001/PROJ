package main

import "fmt"

type Address struct {
	City    string
	Pincode string
}

type Employee struct {
	ID      int
	Name    string
	Role    string
	Address Address
	Salary  int
}

// Value receiver: receives a copy of Employee.
func (e Employee) DisplayName() string {
	return fmt.Sprintf("%s (%s)", e.Name, e.Role)
}

// Pointer receiver: modifies the original Employee.
func (e *Employee) Promote(newRole string) {
	e.Role = newRole
}

// Pointer receiver: safely changes salary on the original value.
func (e *Employee) GiveRaise(amount int) {
	if amount > 0 {
		e.Salary += amount
	}
}

// Value receiver: calculates using a copy; does not mutate Employee.
func (e Employee) AnnualSalary() int {
	return e.Salary * 12
}

func main() {
	fmt.Println("=== 1. Struct literals ===")

	// Named fields are clear and resilient to field-order changes.
	employee := Employee{
		ID:   101,
		Name: "Saurabh",
		Role: "Software Engineer",
		Address: Address{
			City:    "Mumbai",
			Pincode: "400101",
		},
		Salary: 25000,
	}

	fmt.Printf("employee=%+v\n", employee)
	fmt.Println("display:", employee.DisplayName())
	fmt.Println("annual salary:", employee.AnnualSalary())

	fmt.Println("\n=== 2. Pointer receiver mutates original ===")
	employee.Promote("Senior Software Engineer")
	employee.GiveRaise(5000)
	fmt.Printf("after changes=%+v\n", employee)

	fmt.Println("\n=== 3. Struct assignment copies fields ===")
	copyOfEmployee := employee
	copyOfEmployee.Name = "Rahul"
	fmt.Println("original name:", employee.Name)
	fmt.Println("copy name:", copyOfEmployee.Name)

	fmt.Println("\n=== 4. Pointer and address ===")
	employeePointer := &employee
	fmt.Println("pointer:", employeePointer)
	fmt.Println("name through pointer:", employeePointer.Name)
	employeePointer.Name = "Saurabh Kumar"
	fmt.Println("original name after pointer update:", employee.Name)

	fmt.Println("\n=== 5. Zero-value struct ===")
	var newEmployee Employee
	fmt.Printf("zero value=%+v\n", newEmployee)
	fmt.Println("zero-value name:", newEmployee.Name)

	fmt.Println("\n=== 6. Constructor-style function ===")
	created := NewEmployee(102, "Asha", "Developer", "Pune", "411001", 30000)
	fmt.Printf("created=%+v\n", created)
}

// NewEmployee centralizes valid Employee initialization.
func NewEmployee(id int, name, role, city, pincode string, salary int) Employee {
	return Employee{
		ID:      id,
		Name:    name,
		Role:    role,
		Address: Address{City: city, Pincode: pincode},
		Salary:  salary,
	}
}
