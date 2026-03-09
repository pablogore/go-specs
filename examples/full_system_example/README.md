# Full system example: payment service

This example demonstrates **all major features** of the go-specs testing framework using a small payment system with a deliberate bug. It is suitable for documentation and as a reference for BDD, path exploration, shrinking, mocks, and snapshots.

## System overview

The payment service provides:

- **Deposit(balance, amount)** — returns `balance + amount`
- **Withdraw(balance, amount)** — returns the new balance; **contains a bug** when `amount > balance`
- **Transfer(ledger, fromBalance, toBalance, amount)** — moves `amount` from source to destination and optionally records via a **Ledger** (external dependency)

The **Ledger** interface is mocked in tests so we can verify that `RecordTransfer(from, to, amount)` is called with the expected arguments.

## Run the example

```bash
go test ./examples/full_system_example
```

Because `Withdraw` has a bug, the test run will:

1. **Explore** the input space (balance and amount from 0 to 1000) using **ExploreSmart** (boundary values, random sampling, coverage-guided mutation).
2. **Discover** a failing case (e.g. `amount > balance` producing a negative balance).
3. **Shrink** the failing input to a minimal case and report it.

Example output:

```
FAIL after 11 tests

minimal failing input:

balance = 0
amount = 1
```

## Path exploration

Path specs define dimensions and let the framework generate combinations:

```go
s.Paths(func(p *specs.PathBuilder) {
    p.IntRange("balance", 0, 1000)
    p.IntRange("amount", 0, 1000)
}).ExploreSmart(5000).It("never produces negative balance", func(ctx *specs.Context) {
    balance := ctx.Path().Int("balance")
    amount := ctx.Path().Int("amount")
    newBalance := Withdraw(balance, amount)
    ctx.Expect(newBalance >= 0).To(specs.BeTrue())
})
```

- **IntRange** defines an integer dimension (min, max).
- **ExploreSmart(5000)** runs up to 5000 inputs using smart exploration (boundaries, random, coverage, corpus).
- **ctx.Path()** gives the current combination (e.g. `balance`, `amount`).

Other options: **.It(...)** for full Cartesian enumeration, **.Sample(n)** for random sampling, **.ExploreCoverage(n)** for coverage-guided exploration.

## Automatic bug discovery

The property “withdraw never produces negative balance” is violated when `amount > balance`. The framework:

1. Tries many inputs (ExploreSmart).
2. Stops when an assertion fails.
3. Runs the **shrinker** to reduce the failing input to a minimal one (e.g. `balance=0`, `amount=1`).
4. Reports that minimal case in the failure message.

No need to hand-pick the failing case; exploration and shrinking do it.

## Shrinking

When a path spec fails, go-specs runs a **shrinker** that:

- Takes the failing `PathValues`.
- Shrinks each dimension in turn (e.g. binary search toward zero for integers).
- Re-runs the test for each candidate; if it still fails, keeps the smaller input.
- Stops when no smaller failing input is found.

The failure message shows this **minimal failing input** so you can fix the bug with a small, reproducible example.

## Mock verification

The **Ledger** dependency is replaced by a mock that records calls:

```go
m := mock.New()
ledger := NewMockLedger(m)
svc := NewTransferService(ledger)

svc.Transfer(100, 50, 20)

recordSpy := m.Spy("RecordTransfer")
ctx.Expect(recordSpy.CallCount()).ToEqual(1)
if !recordSpy.CalledWith(mock.Equal(100), mock.Equal(50), mock.Equal(20)) {
    t.Fatal("expected RecordTransfer(100, 50, 20)")
}
```

- **NewMockLedger(m)** returns a `Ledger` that forwards `RecordTransfer` to **m.Spy("RecordTransfer")**.
- **Spy.CallCount()** and **Spy.CalledWith(matchers...)** verify that the right call happened.

## Snapshots

Structured results can be captured and compared to stored snapshots:

```go
result := map[string]any{"fromBalance": from, "toBalance": to, "amount": 25}
ctx.Snapshot("transfer_result", result)
```

Snapshots are stored under `__snapshots__/<test_file>.snap.json`. To create or update them:

```bash
GO_SPECS_UPDATE_SNAPSHOTS=1 go test ./examples/full_system_example
```

## Fixing the bug

To make the example pass, fix **Withdraw** in `payment.go` so that when `amount > balance` the function does not return a negative value (e.g. return `balance` unchanged or return an error). After that, `go test ./examples/full_system_example` should pass.

## Files

| File               | Purpose                                                |
|--------------------|--------------------------------------------------------|
| `payment.go`       | Deposit, Withdraw (with bug), Transfer + Ledger call   |
| `payment_mocks.go`| Ledger interface, TransferService, NewMockLedger      |
| `payment_test.go` | BDD specs: Deposit, Transfer (mocks), snapshot, Withdraw (ExploreSmart) |
| `README.md`       | This overview                                          |
