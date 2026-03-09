# Property Testing + Coverage-Guided Exploration + Spy

This example shows **go-specs** used together: BDD structure, **property testing** with path exploration, **coverage-guided exploration**, **automatic shrinking**, and **spy verification** of side effects.

## What it demonstrates

1. **BDD-readable tests** — `Describe` / `It` structure for Deposit, Withdraw, Transfer.
2. **Automatic bug discovery** — Property over `(balance, amount)` finds inputs that break the invariant.
3. **Shrinking to minimal failing case** — When a property fails, go-specs shrinks the input to a minimal repro (e.g. `balance = 0`, `amount = 1`).
4. **Coverage-guided exploration** — `ExploreCoverage(n)` prioritizes inputs that hit new code paths.
5. **Spy verification** — A mock `Ledger` records operations; specs assert that `RecordTransfer`, `RecordDeposit`, and `RecordWithdraw` are called with the expected arguments.

## System under test

- **PaymentService** — Manages user balances; `Deposit`, `Withdraw`, `Transfer`.
- **Ledger** — Interface for recording operations (injected; mocked in tests).
- **WithdrawBalance(balance, amount)** — Pure function used in the service. It has an **intentional bug**: it returns `balance - amount` even when that is negative, instead of rejecting over-withdrawal.

## Example failure (property finds the bug)

When you run:

```bash
go test ./examples/property_coverage_spy -v
```

the property and coverage specs **fail** because of the bug in `WithdrawBalance`. You’ll see output like:

```
=== RUN   TestPaymentService/payment_service_properties/balance_never_becomes_negative
    testing_backend.go:37: FAIL after 11 tests

        minimal failing input:

        balance = 0
        amount = 1
```

So the engine:

1. **Discovers** a failing input (e.g. after 11 tests for ExploreSmart, or after 2 for the small Cartesian run).
2. **Shrinks** it to the minimal case: `balance = 0`, `amount = 1` (withdrawing 1 from 0 produces -1, violating “balance never negative”).

The BDD examples (Deposit, Withdraw when sufficient) and the **spy** specs (records successful transfers, records deposit and withdraw) **pass**, showing that ledger recording is asserted correctly.

## Making the tests pass

To see all tests green, fix the bug in `payment_service.go`:

```go
func WithdrawBalance(balance, amount int) int {
	if amount > balance {
		return balance // or return 0; do not allow negative
	}
	return balance - amount
}
```

Re-run:

```bash
go test ./examples/property_coverage_spy -v
```

All specs, including the property and coverage ones, should pass.

## Files

| File | Purpose |
|------|--------|
| `payment_service.go` | `Ledger` interface, `PaymentService`, `WithdrawBalance` (with intentional bug). |
| `payment_service_test.go` | BDD examples, property tests (ExploreSmart + small-space It), ExploreCoverage, spy verification. |
| `README.md` | This file. |

## Specs in the example

- **Deposit** — Deterministic `It`: deposit increases balance.
- **Withdraw** — Deterministic `It`: withdraw reduces balance when sufficient.
- **balance never becomes negative** — `Paths(IntRange balance 0..1000, IntRange amount 0..1000).ExploreSmart(5000)`: explores the space and asserts `WithdrawBalance(balance, amount) >= 0`.
- **balance never negative (small space, finds bug)** — Same property over `0..5 × 0..5` with full Cartesian: guarantees the bug is hit and shrunk.
- **withdraw never creates negative balance** — `Paths(...).ExploreCoverage(2000)`: coverage-guided exploration for the same invariant.
- **records successful transfers** — Spy on `RecordTransfer`: after `Transfer("alice", "bob", 50)`, assert call count 1 and `CalledWith("alice", "bob", 50)`.
- **records deposit and withdraw on ledger** — Spies on `RecordDeposit` and `RecordWithdraw` to ensure the ledger is called with the right arguments.
