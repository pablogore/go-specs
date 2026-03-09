package full_system_example

// Deposit returns balance + amount (no cap in this example).
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

// Transfer moves amount from fromBalance to toBalance and records via the ledger.
// Returns (newFromBalance, newToBalance). Fails if fromBalance < amount (no change).
func Transfer(ledger Ledger, fromBalance, toBalance, amount int) (int, int) {
	if fromBalance < amount {
		return fromBalance, toBalance
	}
	newFrom := fromBalance - amount
	newTo := toBalance + amount
	if ledger != nil {
		ledger.RecordTransfer(fromBalance, toBalance, amount)
	}
	return newFrom, newTo
}
