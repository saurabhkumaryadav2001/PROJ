package main

import (
	"errors"
	"fmt"
)

type PaymentProcessor interface {
	Charge(amountCents int) error
}

type Notifier interface {
	Notify(message string)
}

type CardProcessor struct{ Provider string }

func (c CardProcessor) Charge(amountCents int) error {
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	fmt.Printf("[%s] charged %d cents\n", c.Provider, amountCents)
	return nil
}

type WalletProcessor struct{ WalletID string }

func (w *WalletProcessor) Charge(amountCents int) error {
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	fmt.Printf("[wallet %s] charged %d cents\n", w.WalletID, amountCents)
	return nil
}

type ConsoleNotifier struct{}

func (ConsoleNotifier) Notify(message string) { fmt.Println("notification:", message) }

type OrderService struct {
	payment  PaymentProcessor
	notifier Notifier
}

func NewOrderService(payment PaymentProcessor, notifier Notifier) (*OrderService, error) {
	if payment == nil {
		return nil, errors.New("payment processor is required")
	}
	if notifier == nil {
		return nil, errors.New("notifier is required")
	}
	return &OrderService{payment: payment, notifier: notifier}, nil
}

func (s *OrderService) PlaceOrder(totalCents int) error {
	if totalCents <= 0 {
		return errors.New("order total must be positive")
	}
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
	case CardProcessor:
		return "card provider: " + v.Provider
	case *WalletProcessor:
		if v == nil {
			return "typed-nil wallet"
		}
		return "wallet: " + v.WalletID
	default:
		return fmt.Sprintf("unknown processor (%T)", p)
	}
}

func main() {
	notifier := ConsoleNotifier{}
	card := CardProcessor{Provider: "DemoCard"}
	service, err := NewOrderService(card, notifier)
	if err != nil {
		panic(err)
	}
	if err := service.PlaceOrder(1250); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(describeProcessor(card))

	wallet := &WalletProcessor{WalletID: "W-42"}
	walletService, err := NewOrderService(wallet, notifier)
	if err != nil {
		panic(err)
	}
	if err := walletService.PlaceOrder(700); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(describeProcessor(wallet))

	var typedNil *WalletProcessor
	var processor PaymentProcessor = typedNil
	fmt.Println("typed nil interface is nil?", processor == nil) // false
	fmt.Println("plain nil interface is nil?", (PaymentProcessor)(nil) == nil) // true
}
