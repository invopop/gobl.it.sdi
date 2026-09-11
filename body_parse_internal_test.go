package fatturapa

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdjustTotalsWithWithholding(t *testing.T) {
	// A 1000.00 line with 22% VAT and a 20% withholding: total with tax
	// 1220.00, payable 1020.00.
	withheldInvoice := func() *bill.Invoice {
		return &bill.Invoice{
			Regime:    tax.WithRegime("IT"),
			Currency:  currency.EUR,
			IssueDate: cal.MakeDate(2026, 9, 6),
			Tax:       &bill.Tax{Rounding: tax.RoundingRuleCurrency},
			Lines: []*bill.Line{{
				Quantity: num.MakeAmount(1, 0),
				Item:     &org.Item{Name: "Consulting", Price: num.NewAmount(100000, 2)},
				Taxes: tax.Set{
					{Category: tax.CategoryVAT, Percent: num.NewPercentage(22, 2)},
					{Category: it.TaxCategoryIRPEF, Percent: num.NewPercentage(20, 2)},
				},
			}},
		}
	}

	tests := []struct {
		name           string
		total          string
		arrotondamento string
		rounding       string
		payable        string
	}{
		{"gross total", "1220.00", "", "", "1020.00"},
		{"net total", "1020.00", "", "", "1020.00"},
		{"gross total with a rounding adjustment", "1220.01", "", "0.01", "1020.01"},
		{"net total with a rounding adjustment", "1019.99", "", "-0.01", "1019.99"},
		{"gross total with a stated rounding adjustment", "1220.01", "0.01", "0.01", "1020.01"},
		// The stated adjustment exceeds half the withholding, so the smaller
		// difference alone would read the total as gross.
		{"net total with a large stated rounding adjustment", "1130.00", "110.00", "110.00", "1130.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := withheldInvoice()
			require.NoError(t, adjustTotals(inv, &GeneralDocumentData{TotalAmount: tt.total, Rounding: tt.arrotondamento}))
			require.NoError(t, inv.Calculate())

			assert.Equal(t, "1220.00", inv.Totals.TotalWithTax.String())
			if tt.rounding == "" {
				assert.Nil(t, inv.Totals.Rounding)
			} else {
				require.NotNil(t, inv.Totals.Rounding)
				assert.Equal(t, tt.rounding, inv.Totals.Rounding.String())
			}
			assert.Equal(t, tt.payable, inv.Totals.Payable.String())
		})
	}
}
