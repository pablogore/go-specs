package payment_system

// Deposit returns balance + amount.
func Deposit(balance, amount int) int {
	return balance + amount
}

// Withdraw returns the new balance after withdrawing amount.
// Never returns negative; if amount > balance, returns 0.
func Withdraw(balance, amount int) int {
	if amount > balance {
		return 0
	}
	return balance - amount
}
