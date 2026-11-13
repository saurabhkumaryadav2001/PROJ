package main

import (
	"errors"
	"fmt"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrInsufficientBalance = errors.New("insufficient balance")

type InputError struct {
	Field string
}

func (e *InputError) Error() string {
	return "invalid input: " + e.Field
}

type Account struct {
	ID      int
	Balance int
}

func Withdraw(account *Account, amount int) error {
	if amount <= 0 {
		return &InputError{Field: "amount must be positive"}
	}
	if account == nil {
		return fmt.Errorf("withdraw: %w", ErrAccountNotFound)
	}
	if amount > account.Balance {
		return fmt.Errorf("withdraw %d from account %d: %w", amount, account.ID, ErrInsufficientBalance)
	}
	account.Balance -= amount
	return nil
}

func main() {
	account := &Account{ID: 7, Balance: 500}
	fmt.Println("withdraw 100:", Withdraw(account, 100))
	fmt.Println("balance:", account.Balance)

	err := Withdraw(account, 1000)
	fmt.Println("large withdrawal:", err)
	fmt.Println("is insufficient:", errors.Is(err, ErrInsufficientBalance))

	err = Withdraw(account, 0)
	var inputErr *InputError
	fmt.Println("bad input:", errors.As(err, &inputErr), inputErr)

	err = Withdraw(nil, 10)
	fmt.Println("missing account:", errors.Is(err, ErrAccountNotFound))
}
