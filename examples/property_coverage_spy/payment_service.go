package property_coverage_spy

// Ledger records payment operations (injected for testing; use a mock in specs).
type Ledger interface {
	RecordDeposit(userID string, amount int)
	RecordWithdraw(userID string, amount int)
	RecordTransfer(from, to string, amount int)
}

// PaymentService manages user balances and records operations via a Ledger.
type PaymentService struct {
	ledger   Ledger
	balances map[string]int
}

// NewPaymentService returns a payment service that uses the given ledger.
func NewPaymentService(ledger Ledger) *PaymentService {
	return &PaymentService{
		ledger:   ledger,
		balances: make(map[string]int),
	}
}

// Deposit adds amount to the user's balance and records it on the ledger.
func (s *PaymentService) Deposit(userID string, amount int) {
	if s.balances == nil {
		s.balances = make(map[string]int)
	}
	s.balances[userID] += amount
	if s.ledger != nil {
		s.ledger.RecordDeposit(userID, amount)
	}
}

// Withdraw deducts amount from the user's balance and records it.
// Uses WithdrawBalance for the math; never allows negative balance.
func (s *PaymentService) Withdraw(userID string, amount int) {
	balance := s.balances[userID]
	newBalance := WithdrawBalance(balance, amount)
	s.balances[userID] = newBalance
	if s.ledger != nil {
		s.ledger.RecordWithdraw(userID, amount)
	}
}

// Transfer moves amount from fromUser to toUser and records it on the ledger.
func (s *PaymentService) Transfer(fromUser, toUser string, amount int) {
	if s.balances == nil {
		return
	}
	fromBal := s.balances[fromUser]
	if fromBal < amount {
		return
	}
	s.balances[fromUser] = fromBal - amount
	s.balances[toUser] += amount
	if s.ledger != nil {
		s.ledger.RecordTransfer(fromUser, toUser, amount)
	}
}

// Balance returns the current balance for the user.
func (s *PaymentService) Balance(userID string) int {
	return s.balances[userID]
}

// WithdrawBalance returns the new balance after withdrawing amount.
// Never returns negative; if amount > balance, returns 0.
func WithdrawBalance(balance, amount int) int {
	if amount > balance {
		return 0
	}
	return balance - amount
}
