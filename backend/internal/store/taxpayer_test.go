package store_test

import (
	"testing"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

func TestTaxpayerProfileFlagsRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	taxpayer := domain.Taxpayer{
		NIF: "12345678Z", Name: "Flags", LegalForm: domain.LegalFormAutonomo,
		VATRegime: "general", IRPFMethod: "directa", EconomicActivity: true,
		HasIntracommunityOperations:     true,
		WithholdsProfessionalIncome:     true,
		ExemptFromIRPFFractionalPayment: true,
		UsesSII:                         true,
		HasGeneralVATActivity:           true,
		MonthlyVAT:                      true,
		MonthlyWithholdings:             true,
		IntracommunityOver50K:           true,
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasIntracommunityOperations || !got.WithholdsProfessionalIncome ||
		!got.ExemptFromIRPFFractionalPayment || !got.UsesSII ||
		!got.HasGeneralVATActivity || !got.MonthlyVAT ||
		!got.MonthlyWithholdings || !got.IntracommunityOver50K {
		t.Fatalf("profile flags were not persisted: %+v", got)
	}

	got.UsesSII = false
	got.WithholdsProfessionalIncome = false
	got.HasGeneralVATActivity = false
	got.MonthlyVAT = false
	got.MonthlyWithholdings = false
	got.IntracommunityOver50K = false
	if err := st.UpdateTaxpayer(ctx, &got); err != nil {
		t.Fatal(err)
	}
	updated, err := st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.UsesSII || updated.WithholdsProfessionalIncome ||
		updated.HasGeneralVATActivity || updated.MonthlyVAT ||
		updated.MonthlyWithholdings || updated.IntracommunityOver50K {
		t.Fatalf("profile flags were not updated: %+v", updated)
	}
}
