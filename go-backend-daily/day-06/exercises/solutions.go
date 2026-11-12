package main

import (
	"errors"
	"fmt"
)

type PaymentProcessor interface { Charge(amountCents int) error }
type RefundProcessor interface { Refund(amountCents int) error }
type Receipt interface { CreateReceipt(orderID string) string }

type CardProcessor struct { LastCharge int }
func (c *CardProcessor) Charge(amount int) error {
	if amount <= 0 { return errors.New("amount must be positive") }
	c.LastCharge = amount
	return nil
}
func (c *CardProcessor) Refund(amount int) error {
	if amount <= 0 || amount > c.LastCharge { return errors.New("invalid refund") }
	c.LastCharge -= amount
	return nil
}

type WalletProcessor struct{}
func (WalletProcessor) Charge(amount int) error {
	if amount <= 0 { return errors.New("amount must be positive") }
	return nil
}
func (WalletProcessor) Refund(amount int) error {
	if amount <= 0 { return errors.New("amount must be positive") }
	return nil
}

type FakePayment struct { LastAmount int; Fail bool }
func (f *FakePayment) Charge(amount int) error {
	f.LastAmount = amount
	if f.Fail || amount == 13 { return errors.New("fake gateway declined") }
	return nil
}

type ConsoleReceipt struct{}
func (ConsoleReceipt) CreateReceipt(orderID string) string { return "receipt for " + orderID }

func processorName(p PaymentProcessor) string {
	switch p.(type) {
	case *CardProcessor: return "card"
	case WalletProcessor: return "wallet"
	default: return "unknown"
	}
}

func main() {
	card := &CardProcessor{}
	var pay PaymentProcessor = card
	_ = pay.Charge(100)
	var refund RefundProcessor = card
	_ = refund.Refund(40)
	fmt.Println("remaining charge:", card.LastCharge)

	fake := &FakePayment{}
	_ = fake.Charge(250)
	fmt.Println("fake recorded:", fake.LastAmount)
	fmt.Println("processor:", processorName(pay))
	fmt.Println(ConsoleReceipt{}.CreateReceipt("ORD-1"))
}
