// Package property_coverage_spy_test demonstrates: (1) BDD examples, (2) property tests
// (Paths + ExploreSmart / ExploreCoverage / Cartesian), (3) spy verification with the mock.
// WithdrawBalance is correct; the comments explain how shrinking would surface a minimal
// counterexample if the implementation were buggy.
package property_coverage_spy_test

import (
	"testing"

	"github.com/pablogore/go-specs/mock"
	"github.com/pablogore/go-specs/specs"
	"github.com/pablogore/go-specs/examples/property_coverage_spy"
)

// mockLedger implements property_coverage_spy.Ledger and records calls to the mock's spies.
type mockLedger struct {
	m *mock.Mock
}

func newMockLedger(m *mock.Mock) *mockLedger {
	if m == nil {
		return nil
	}
	return &mockLedger{m: m}
}

func (l *mockLedger) RecordDeposit(userID string, amount int) {
	l.m.Spy("RecordDeposit").Call(userID, amount)
}

func (l *mockLedger) RecordWithdraw(userID string, amount int) {
	l.m.Spy("RecordWithdraw").Call(userID, amount)
}

func (l *mockLedger) RecordTransfer(from, to string, amount int) {
	l.m.Spy("RecordTransfer").Call(from, to, amount)
}

func TestPaymentService(t *testing.T) {
	specs.Describe(t, "payment service properties", func(s *specs.Spec) {

		// -------------------------------------------------------
		// 1. BDD examples (deterministic)
		// -------------------------------------------------------

		s.Describe("Deposit", func(s2 *specs.Spec) {
			s2.It("increases balance by amount", func(ctx *specs.Context) {
				svc := property_coverage_spy.NewPaymentService(nil)
				svc.Deposit("alice", 100)
				ctx.Expect(svc.Balance("alice")).ToEqual(100)
				svc.Deposit("alice", 50)
				ctx.Expect(svc.Balance("alice")).ToEqual(150)
			})
		})

		s.Describe("Withdraw", func(s2 *specs.Spec) {
			s2.It("reduces balance by amount when sufficient", func(ctx *specs.Context) {
				svc := property_coverage_spy.NewPaymentService(nil)
				svc.Deposit("alice", 100)
				svc.Withdraw("alice", 30)
				ctx.Expect(svc.Balance("alice")).ToEqual(70)
			})
		})

		// -------------------------------------------------------
		// 2. Property test: balance never negative (ExploreSmart)
		// -------------------------------------------------------

		s.Paths(func(p *specs.PathBuilder) {
			p.IntRange("balance", 0, 1000)
			p.IntRange("amount", 0, 1000)
		}).ExploreSmart(5000).It("balance never becomes negative", func(ctx *specs.Context) {
			balance := ctx.Path().Int("balance")
			amount := ctx.Path().Int("amount")
			result := property_coverage_spy.WithdrawBalance(balance, amount)
			ctx.Expect(result >= 0).To(specs.BeTrue())
		})

		// Same property over a small Cartesian space (0..5). The implementation is correct,
		// so the property holds for all combinations.
		//
		// If WithdrawBalance were implemented incorrectly:
		//
		//     return balance - amount
		//
		// this property test would fail and the framework would shrink
		// the failing case to a minimal counterexample such as:
		//
		//     balance = 0
		//     amount  = 1
		s.Paths(func(p *specs.PathBuilder) {
			p.IntRange("balance", 0, 5)
			p.IntRange("amount", 0, 5)
		}).It("balance never negative (small space)", func(ctx *specs.Context) {
			balance := ctx.Path().Int("balance")
			amount := ctx.Path().Int("amount")
			result := property_coverage_spy.WithdrawBalance(balance, amount)
			ctx.Expect(result >= 0).To(specs.BeTrue())
		})

		// -------------------------------------------------------
		// 3. Coverage-guided exploration
		// -------------------------------------------------------

		s.Paths(func(p *specs.PathBuilder) {
			p.IntRange("balance", 0, 200)
			p.IntRange("amount", 0, 200)
		}).ExploreCoverage(2000).It("withdraw never creates negative balance", func(ctx *specs.Context) {
			balance := ctx.Path().Int("balance")
			amount := ctx.Path().Int("amount")
			result := property_coverage_spy.WithdrawBalance(balance, amount)
			ctx.Expect(result >= 0).To(specs.BeTrue())
		})

		// -------------------------------------------------------
		// 4. Spy verification: ledger is called correctly
		// -------------------------------------------------------

		s.It("records successful transfers", func(ctx *specs.Context) {
			m := mock.New()
			ledger := newMockLedger(m)
			svc := property_coverage_spy.NewPaymentService(ledger)
			svc.Deposit("alice", 100)
			svc.Deposit("bob", 0)
			svc.Transfer("alice", "bob", 50)

			recordSpy := m.Spy("RecordTransfer")
			ctx.Expect(recordSpy.CallCount()).ToEqual(1)
			ctx.Expect(recordSpy.CalledWith(
				mock.Equal("alice"),
				mock.Equal("bob"),
				mock.Equal(50),
			)).To(specs.BeTrue())
		})

		s.It("records deposit and withdraw on ledger", func(ctx *specs.Context) {
			m := mock.New()
			ledger := newMockLedger(m)
			svc := property_coverage_spy.NewPaymentService(ledger)
			svc.Deposit("alice", 80)
			svc.Withdraw("alice", 20)

			ctx.Expect(m.Spy("RecordDeposit").CallCount()).ToEqual(1)
			ctx.Expect(m.Spy("RecordDeposit").CalledWith(mock.Equal("alice"), mock.Equal(80))).To(specs.BeTrue())
			ctx.Expect(m.Spy("RecordWithdraw").CallCount()).ToEqual(1)
			ctx.Expect(m.Spy("RecordWithdraw").CalledWith(mock.Equal("alice"), mock.Equal(20))).To(specs.BeTrue())
		})
	})
}
