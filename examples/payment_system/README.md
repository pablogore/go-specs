# Payment system example

A small **payment system** implemented and tested with the go-specs framework. The example shows BDD-style tests, automatic path exploration, invariant/property testing, shrinking of failing inputs, mocks and spies, and snapshot testing. The system includes an intentional bug so that exploration and shrinking can be demonstrated.

## System overview

- **Deposit(balance, amount)** — returns `balance + amount`.
- **Withdraw(balance, amount)** — returns the new balance after withdrawing `amount`. Contains a **bug** when `amount > balance` (returns a negative value).
- **PaymentService** — holds a **Ledger** and exposes **Transfer(from, to, amount)** to move `amount` between two balances and optionally record the operation via the ledger.
- **Ledger** — interface for recording transfers; implemented with mocks in tests.

## Project structure

```
examples/payment_system/
    ledger.go           # Ledger interface
    payment.go          # Deposit, Withdraw (with bug)
    payment_service.go  # PaymentService and Transfer
    payment_test.go     # BDD + path exploration + invariant
    payment_mock_test.go# Mock/spy verification
    snapshot_test.go    # Snapshot testing
    README.md
```

## Run the tests

```bash
go test ./examples/payment_system
```

Because `Withdraw` is buggy, the invariant test fails and the framework reports a minimal failing input, for example:

```
FAIL  withdraw invariants

minimal failing input:

balance = 0
amount = 1
```

(Exact numbers may vary; the shrinker finds a minimal case such as `balance=0, amount=1` or `balance=10, amount=11`.)

## BDD testing

Specs are structured with `Describe` and `It`:

```go
specs.Describe(t, "deposit", func(s *specs.Spec) {
    s.It("increases balance by amount", func(ctx *specs.Context) {
        ctx.Expect(Deposit(100, 50)).ToEqual(150)
    })
})
```

Nested `Describe` blocks and multiple `It` specs keep the suite readable and organized.

## Automatic exploration

The withdraw invariant is checked over a large input space without hand-written cases:

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

- **Paths** defines dimensions (`balance`, `amount`).
- **ExploreSmart(5000)** runs up to 5000 combinations (boundary, random, coverage-guided).
- **ctx.Path()** provides the current combination.

## Bug discovery

The property “withdraw never produces negative balance” is false when `amount > balance`. The framework:

1. Explores many inputs.
2. Stops at the first failing assertion.
3. Runs the **shrinker** to reduce the failing input.
4. Prints the **minimal failing input** in the failure message.

No need to guess the failing case; exploration finds it and shrinking minimizes it.

## Shrinking

When a path spec fails, go-specs shrinks the failing input:

- Reduces each dimension (e.g. binary search toward zero for integers).
- Re-runs the test for each candidate; keeps a smaller input if the test still fails.
- Stops when no smaller failing input exists.

The failure output shows this minimal input for easy debugging.

## Mock verification

The **Ledger** is mocked so we can assert that `RecordTransfer` is called correctly:

```go
m := mock.New()
ledger := &mockLedger{spy: m.Spy("RecordTransfer")}
service := &PaymentService{Ledger: ledger}

service.Transfer(100, 50, 20)

if !m.Spy("RecordTransfer").CalledWith(mock.Equal(100), mock.Equal(50), mock.Equal(20)) {
    t.Fatal("expected RecordTransfer(100, 50, 20)")
}
```

`mockLedger` implements `Ledger` by forwarding to `m.Spy("RecordTransfer")`. **CalledWith** and **CallCount** verify the interaction.

## Snapshot testing

Transfer results are captured and compared to stored snapshots:

```go
service := &PaymentService{Ledger: nil}
newFrom, newTo := service.Transfer(100, 50, 10)
result := map[string]any{"fromBalance": 100, "toBalance": 50, "amount": 10, "newFrom": newFrom, "newTo": newTo}
ctx.Snapshot("transfer_result", result)
```

Snapshots live in `__snapshots__/snapshot_test.snap.json`. To create or update them:

```bash
GO_SPECS_UPDATE_SNAPSHOTS=1 go test ./examples/payment_system -run TestTransferSnapshot
```

## Deterministic execution

Path exploration and shrinking use deterministic seeds where applicable so that repeated runs reproduce the same exploration and minimal failing input for the same code.

## Fixing the bug

To make all tests pass, change **Withdraw** in `payment.go` so that when `amount > balance` it does not return a negative value (e.g. return `balance` unchanged or signal an error). Then:

```bash
go test ./examples/payment_system
```

should pass.
