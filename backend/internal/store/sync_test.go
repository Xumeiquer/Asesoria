package store_test

import (
	"context"
	"testing"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

func TestSyncFilingsRecalculatesActivityStart(t *testing.T) {
	st, ctx := testStore(t)
	taxpayer := testTaxpayer(t, st, ctx)

	q1 := domain.Filing{
		TaxpayerID: taxpayer.ID, ModelCode: "303", ModelName: "IVA", Entity: "AEAT",
		PeriodLabel: "1T 2026", PeriodStart: "2026-01-01", PeriodEnd: "2026-03-31",
		DueDate: "2026-04-20", Status: domain.StatusPending,
	}
	q2 := domain.Filing{
		TaxpayerID: taxpayer.ID, ModelCode: "303", ModelName: "IVA", Entity: "AEAT",
		PeriodLabel: "2T 2026", PeriodStart: "2026-04-01", PeriodEnd: "2026-06-30",
		DueDate: "2026-07-20", Status: domain.StatusPending,
	}
	q3 := domain.Filing{
		TaxpayerID: taxpayer.ID, ModelCode: "303", ModelName: "IVA", Entity: "AEAT",
		PeriodLabel: "3T 2026", PeriodStart: "2026-07-01", PeriodEnd: "2026-09-30",
		DueDate: "2026-10-20", Status: domain.StatusPending,
	}
	if _, err := st.UpsertFilings(ctx, []domain.Filing{q1, q2, q3}); err != nil {
		t.Fatal(err)
	}
	q1 = filingByPeriod(t, st, ctx, taxpayer.ID, "1T 2026")
	q2 = filingByPeriod(t, st, ctx, taxpayer.ID, "2T 2026")
	q3 = filingByPeriod(t, st, ctx, taxpayer.ID, "3T 2026")

	// Activity now starts in April: the first quarter must leave the queue.
	result, err := st.SyncFilings(ctx, taxpayer.ID, []domain.Filing{q2, q3}, []int{2026})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hidden != 1 {
		t.Fatalf("expected 1 hidden filing, got %+v", result)
	}
	q1 = filingByPeriod(t, st, ctx, taxpayer.ID, "1T 2026")
	if q1.Status != domain.StatusNotApplicable || !q1.AutoHidden {
		t.Fatalf("1T was not retired automatically: %+v", q1)
	}

	// The same period can also change its dates when the calendar is refreshed.
	q2Updated := q2
	q2Updated.ModelName = "IVA actualizado"
	q2Updated.DueDate = "2026-07-21"
	result, err = st.SyncFilings(ctx, taxpayer.ID, []domain.Filing{q2Updated, q3}, []int{2026})
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated < 1 {
		t.Fatalf("expected the changed filing to be updated, got %+v", result)
	}
	q2 = filingByPeriod(t, st, ctx, taxpayer.ID, "2T 2026")
	if q2.ModelName != "IVA actualizado" || q2.DueDate != "2026-07-21" {
		t.Fatalf("filing metadata was not refreshed: %+v", q2)
	}

	// If the activity start moves back, the automatically retired filing returns.
	result, err = st.SyncFilings(ctx, taxpayer.ID, []domain.Filing{q1, q2Updated, q3}, []int{2026})
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != 1 {
		t.Fatalf("expected 1 restored filing, got %+v", result)
	}
	q1 = filingByPeriod(t, st, ctx, taxpayer.ID, "1T 2026")
	if q1.Status != domain.StatusPending || q1.AutoHidden {
		t.Fatalf("1T was not restored: %+v", q1)
	}

	// Presented filings are historical evidence and must not be retired.
	if err := st.MarkPresented(ctx, q1.ID, ""); err != nil {
		t.Fatal(err)
	}
	result, err = st.SyncFilings(ctx, taxpayer.ID, []domain.Filing{q2Updated, q3}, []int{2026})
	if err != nil {
		t.Fatal(err)
	}
	if result.KeptPresented != 1 {
		t.Fatalf("expected the presented filing to be kept, got %+v", result)
	}
	q1 = filingByPeriod(t, st, ctx, taxpayer.ID, "1T 2026")
	if q1.Status != domain.StatusPresented || q1.AutoHidden {
		t.Fatalf("presented filing was modified: %+v", q1)
	}

	// A manual "no aplica" decision must survive later recalculations.
	if _, err := st.BulkSetFilingStatus(ctx, []string{q2.ID}, domain.StatusNotApplicable); err != nil {
		t.Fatal(err)
	}
	result, err = st.SyncFilings(ctx, taxpayer.ID, []domain.Filing{q2Updated, q3}, []int{2026})
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != 0 {
		t.Fatalf("a manual not_applicable filing was reopened: %+v", result)
	}
	q2 = filingByPeriod(t, st, ctx, taxpayer.ID, "2T 2026")
	if q2.Status != domain.StatusNotApplicable || q2.AutoHidden {
		t.Fatalf("manual not_applicable was not preserved: %+v", q2)
	}
}

func filingByPeriod(t *testing.T, st *store.Store, ctx context.Context, taxpayerID, period string) domain.Filing {
	t.Helper()
	list, err := st.ListFilings(ctx, store.FilingFilter{TaxpayerID: taxpayerID})
	if err != nil {
		t.Fatal(err)
	}
	for _, filing := range list {
		if filing.PeriodLabel == period {
			return filing
		}
	}
	t.Fatalf("filing %s not found", period)
	return domain.Filing{}
}
