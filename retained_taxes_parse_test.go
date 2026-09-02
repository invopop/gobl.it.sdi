package fatturapa_test

import (
	"os"
	"path/filepath"
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetainedTaxesInConversion(t *testing.T) {
	t.Run("should only apply retention to fund contributions with Ritenuta=SI", func(t *testing.T) {
		// Invoice has two fund contributions: one with Ritenuta=SI and one without.
		// The retained tax (20% of line 1600 + retained fund 64 = 332.80) should
		// only match against the retained fund contribution, not both.
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-fund-contribution-mixed-retention.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err, "parsing should succeed — retained tax matches line + retained fund contribution only")
		require.NotNil(t, env)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		// Should have 3 charges (one retained, one not, one zero-rate)
		require.Len(t, invoice.Charges, 3)

		// Find the fund contributions by type
		var retainedCharge, nonRetainedCharge, zeroRateCharge *bill.Charge
		for _, ch := range invoice.Charges {
			switch ch.Ext.Get(sdi.ExtKeyFundType) {
			case "TC22":
				retainedCharge = ch
			case "TC04":
				nonRetainedCharge = ch
			case "TC07":
				zeroRateCharge = ch
			}
		}
		require.NotNil(t, retainedCharge, "TC22 charge should exist")
		require.NotNil(t, nonRetainedCharge, "TC04 charge should exist")
		require.NotNil(t, zeroRateCharge, "TC07 charge should exist")

		// The retained fund contribution should have IRPEF tax applied
		var hasIRPEF bool
		for _, tx := range retainedCharge.Taxes {
			if tx.Category == it.TaxCategoryIRPEF {
				hasIRPEF = true
				assert.True(t, tx.Percent.Compare(num.MakePercentage(200, 3)) == 0, "rate should be 20%")
				assert.Equal(t, cbc.Code("A"), tx.Ext.Get(sdi.ExtKeyRetained))
			}
		}
		assert.True(t, hasIRPEF, "retained fund contribution should have IRPEF tax")

		// The non-retained fund contribution should NOT have IRPEF tax
		for _, tx := range nonRetainedCharge.Taxes {
			assert.NotEqual(t, it.TaxCategoryIRPEF, tx.Category, "non-retained fund contribution should not have IRPEF tax")
		}

		// Zero-rate fund contribution should not derive a base (would cause division by zero)
		assert.Nil(t, zeroRateCharge.Base, "zero-rate fund contribution should have no base")
	})

	t.Run("should convert retained taxes correctly", func(t *testing.T) {
		// Load the XML file with retained taxes
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-irpef.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check that the invoice has tax categories
		require.NotNil(t, invoice.Totals)
		require.NotNil(t, invoice.Totals.Taxes)
		require.NotEmpty(t, invoice.Totals.Taxes.Categories)

		// Find the retained tax category (IRPEF)
		var retainedCategory *tax.CategoryTotal
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Code == it.TaxCategoryIRPEF && cat.Retained {
				retainedCategory = cat
				break
			}
		}

		// Check the retained tax category
		require.NotNil(t, retainedCategory, "Retained tax category should exist")
		assert.Equal(t, it.TaxCategoryIRPEF, retainedCategory.Code)
		assert.True(t, retainedCategory.Retained, "Category should be marked as retained")

		// Check the retained tax amount
		assert.NotEmpty(t, retainedCategory.Amount)
		assert.True(t, retainedCategory.Amount.Compare(num.MakeAmount(37400, 2)) == 0, "Retained tax amount should be 374.00")

		// Check the retained tax rates
		require.NotEmpty(t, retainedCategory.Rates)
		require.Len(t, retainedCategory.Rates, 2)

		// Check first rate (20% IRPEF)
		rate1 := retainedCategory.Rates[0]
		require.NotNil(t, rate1.Percent)
		assert.True(t, rate1.Percent.Compare(num.MakePercentage(200, 3)) == 0, "Rate should be 20.0%")
		assert.True(t, rate1.Amount.Compare(num.MakeAmount(32400, 2)) == 0, "Amount should be 324.00")
		assert.Equal(t, cbc.Code("A"), rate1.Ext.Get(sdi.ExtKeyRetained))

		// Check second rate (50% IRPEF)
		rate2 := retainedCategory.Rates[1]
		require.NotNil(t, rate2.Percent)
		assert.True(t, rate2.Percent.Compare(num.MakePercentage(500, 3)) == 0, "Rate should be 50.0%")
		assert.True(t, rate2.Amount.Compare(num.MakeAmount(5000, 2)) == 0, "Amount should be 50.00")
		assert.Equal(t, cbc.Code("I"), rate2.Ext.Get(sdi.ExtKeyRetained))
	})

	t.Run("should not have retained taxes when not present", func(t *testing.T) {
		// Load the XML file without retained taxes
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check that the invoice has tax categories
		require.NotNil(t, invoice.Totals)
		require.NotNil(t, invoice.Totals.Taxes)
		require.NotEmpty(t, invoice.Totals.Taxes.Categories)

		// Check that there are no retained tax categories
		for _, cat := range invoice.Totals.Taxes.Categories {
			assert.False(t, cat.Retained, "No category should be marked as retained")
		}
	})
}
