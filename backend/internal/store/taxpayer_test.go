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
	}
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasIntracommunityOperations || !got.WithholdsProfessionalIncome ||
		!got.ExemptFromIRPFFractionalPayment || !got.UsesSII {
		t.Fatalf("profile flags were not persisted: %+v", got)
	}

	got.UsesSII = false
	got.WithholdsProfessionalIncome = false
	if err := st.UpdateTaxpayer(ctx, &got); err != nil {
		t.Fatal(err)
	}
	updated, err := st.GetTaxpayer(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.UsesSII || updated.WithholdsProfessionalIncome {
		t.Fatalf("profile flags were not updated: %+v", updated)
	}
}
