package payment_system

// Ledger is an external dependency that records transfer operations.
// Implementations are typically mocked in tests.
type Ledger interface {
	RecordTransfer(from, to, amount int)
}
