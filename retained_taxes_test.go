package fatturapa_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/regimes/it"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatiRitenuta(t *testing.T) {
	t.Run("when retained taxes are NOT present", func(t *testing.T) {
		t.Run("should be empty", func(t *testing.T) {
			env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			assert.Empty(t, dr)
		})
	})

	t.Run("when retained taxes are present", func(t *testing.T) {
		t.Run("should contain the correct retainted taxes", func(t *testing.T) {
			env := test.LoadTestFile("invoice-irpef.json", test.PathGOBLFatturaPA)
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			require.Len(t, dr, 2)

			assert.Equal(t, "RT01", dr[0].Type)
			assert.Equal(t, "324.00", dr[0].Amount)
			assert.Equal(t, "20.00", dr[0].Rate)
			assert.Equal(t, "A", dr[0].Reason)

			assert.Equal(t, "RT01", dr[1].Type)
			assert.Equal(t, "50.00", dr[1].Amount)
			assert.Equal(t, "50.00", dr[1].Rate)
			assert.Equal(t, "I", dr[1].Reason)
		})

		t.Run("should print the statutory rate of a reduced-base withholding", func(t *testing.T) {
			env := test.LoadTestFile("invoice-retained-reduced-base.json", test.PathGOBLFatturaPA)
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			require.Len(t, dr, 1)
			assert.Equal(t, "RT02", dr[0].Type)
			assert.Equal(t, "263.93", dr[0].Amount)
			assert.Equal(t, "23.00", dr[0].Rate)
			assert.Equal(t, "A", dr[0].Reason)
		})

		t.Run("should keep withholdings with different statutory rates apart", func(t *testing.T) {
			env := test.LoadTestFile("invoice-retained-reduced-base.json", test.PathGOBLFatturaPA)
			test.ModifyInvoice(env, func(inv *bill.Invoice) {
				for _, tc := range inv.Lines[1].Taxes {
					if tc.Category == it.TaxCategoryIRES {
						tc.Ext = tc.Ext.Set(sdi.ExtKeyRetainedRate, "46.00")
					}
				}
				require.NoError(t, inv.Calculate())
			})
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			require.Len(t, dr, 2)
			assert.Equal(t, "108.68", dr[0].Amount)
			assert.Equal(t, "23.00", dr[0].Rate)
			assert.Equal(t, "155.25", dr[1].Amount)
			assert.Equal(t, "46.00", dr[1].Rate)
		})
	})
}
