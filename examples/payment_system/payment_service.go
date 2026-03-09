package payment_system

// PaymentService performs transfers and records them via a Ledger.
type PaymentService struct {
	Ledger Ledger
}

// Transfer moves amount from fromBalance to toBalance.
// Returns (newFromBalance, newToBalance). If fromBalance < amount, no transfer occurs.
func (s *PaymentService) Transfer(from, to, amount int) (newFrom, newTo int) {
	if s == nil {
		return from, to
	}
	if from < amount {
		return from, to
	}
	newFrom = from - amount
	newTo = to + amount
	if s.Ledger != nil {
		s.Ledger.RecordTransfer(from, to, amount)
	}
	return newFrom, newTo
}
