package fatturapa_test

import (
	"os"
	"path/filepath"
	"strings"
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
		assert.Empty(t, invoice.Tax.Rounding, "rounding rule is only set for withheld invoices")
	})

	t.Run("should deduct the withholding from the payable amount", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-full-base.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		assert.Equal(t, tax.RoundingRuleCurrency, invoice.Tax.Rounding)
		for _, tc := range invoice.Lines[0].Taxes {
			assert.Empty(t, tc.Ext.Get(sdi.ExtKeyRetainedRate), "no rate is derived when the declared one matches")
		}
		assert.Equal(t, "1220.00", invoice.Totals.TotalWithTax.String())
		require.NotNil(t, invoice.Totals.RetainedTax)
		assert.Equal(t, "200.00", invoice.Totals.RetainedTax.String())
		assert.Equal(t, "1020.00", invoice.Totals.Payable.String())
		assert.Nil(t, invoice.Totals.Rounding)
	})

	t.Run("should keep precise rounding when document discounts round differently", func(t *testing.T) {
		// Three 0.40% discounts on 100.60 are 0.4024 each: summed first they
		// take 1.21 off the total, rounded first only 1.20.
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-full-base.xml"))
		require.NoError(t, err)
		xml := string(data)
		for _, r := range [][2]string{
			{"<PrezzoUnitario>1000.00</PrezzoUnitario>", "<PrezzoUnitario>100.60</PrezzoUnitario>"},
			{"<PrezzoTotale>1000.00</PrezzoTotale>", "<PrezzoTotale>100.60</PrezzoTotale>"},
			{"<ImponibileImporto>1000.00</ImponibileImporto>", "<ImponibileImporto>100.60</ImponibileImporto>"},
			{"<Imposta>220.00</Imposta>", "<Imposta>22.13</Imposta>"},
			{"<ImportoRitenuta>200.00</ImportoRitenuta>", "<ImportoRitenuta>20.12</ImportoRitenuta>"},
			{"<ImportoTotaleDocumento>1220.00</ImportoTotaleDocumento>",
				"<ScontoMaggiorazione><Tipo>SC</Tipo><Percentuale>0.40</Percentuale></ScontoMaggiorazione>" +
					"<ScontoMaggiorazione><Tipo>SC</Tipo><Percentuale>0.40</Percentuale></ScontoMaggiorazione>" +
					"<ScontoMaggiorazione><Tipo>SC</Tipo><Percentuale>0.40</Percentuale></ScontoMaggiorazione>" +
					"<ImportoTotaleDocumento>121.52</ImportoTotaleDocumento>"},
		} {
			require.Contains(t, xml, r[0])
			xml = strings.Replace(xml, r[0], r[1], 1)
		}

		env, err := test.ConvertToGOBL([]byte(xml))
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		assert.Empty(t, invoice.Tax.Rounding)
		require.NotNil(t, invoice.Totals.Discount)
		assert.Equal(t, "1.21", invoice.Totals.Discount.String())
		assert.Equal(t, "99.39", invoice.Totals.Total.String())
		assert.Equal(t, "121.52", invoice.Totals.TotalWithTax.String())
		assert.Equal(t, "101.40", invoice.Totals.Payable.String())
		assert.Nil(t, invoice.Totals.Rounding)
	})

	t.Run("should keep the declared rate when line totals have fractional cents", func(t *testing.T) {
		// Three lines of 10.005 sum to 30.015; rounding each line first would
		// give 30.03 and 20% of it no longer reproduces the 6.00 declared.
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-fractional-cents.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		assert.Empty(t, invoice.Tax.Rounding, "precise rounding keeps the sum at 30.02")
		assert.Equal(t, "30.02", invoice.Totals.Sum.String())
		assert.Equal(t, "6.60", invoice.Totals.Tax.String())

		var retained *tax.CategoryTotal
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Code == it.TaxCategoryIRPEF {
				retained = cat
			}
		}
		require.NotNil(t, retained)
		require.Len(t, retained.Rates, 1)
		assert.Equal(t, "20.00%", retained.Rates[0].Percent.String())
		assert.Equal(t, "6.00", retained.Amount.String())
		assert.Equal(t, "36.62", invoice.Totals.TotalWithTax.String())
		assert.Equal(t, "30.62", invoice.Totals.Payable.String())
		assert.Nil(t, invoice.Totals.Rounding)
	})
}

func TestRetainedTaxOnReducedBase(t *testing.T) {
	// Art. 25-bis on commissions: the statutory 23% withheld on half the
	// base, which only ImportoRitenuta reflects.
	t.Run("should import a withholding declared on a fraction of the base", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-reduced-base.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		var retained *tax.CategoryTotal
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Code == it.TaxCategoryIRES {
				retained = cat
			}
		}
		require.NotNil(t, retained, "IRES category should exist")
		assert.True(t, retained.Retained)
		assert.Equal(t, "263.93", retained.Amount.String())

		require.Len(t, retained.Rates, 1)
		rate := retained.Rates[0]
		require.NotNil(t, rate.Percent)
		assert.Equal(t, "11.50%", rate.Percent.String())
		assert.Equal(t, "2295.00", rate.Base.String())
		assert.Equal(t, cbc.Code("A"), rate.Ext.Get(sdi.ExtKeyRetained))
		assert.Equal(t, cbc.Code("23.00"), rate.Ext.Get(sdi.ExtKeyRetainedRate), "the declared rate is kept")
	})

	t.Run("should keep the declared rate with two decimals", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-reduced-base.xml"))
		require.NoError(t, err)
		xml := strings.Replace(string(data), "<AliquotaRitenuta>23.00</AliquotaRitenuta>", "<AliquotaRitenuta>23</AliquotaRitenuta>", 1)
		require.NotEqual(t, string(data), xml)

		env, err := test.ConvertToGOBL([]byte(xml))
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		assert.Equal(t, cbc.Code("23.00"), invoice.Lines[0].Taxes[1].Ext.Get(sdi.ExtKeyRetainedRate))
	})

	t.Run("should deduct the withholding from the payable amount", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-reduced-base.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		assert.Equal(t, "2799.90", invoice.Totals.TotalWithTax.String())
		require.NotNil(t, invoice.Totals.RetainedTax)
		assert.Equal(t, "263.93", invoice.Totals.RetainedTax.String())
		assert.Equal(t, "2535.97", invoice.Totals.Payable.String())
		assert.Nil(t, invoice.Totals.Rounding)
	})

	t.Run("should derive a rate with four decimals when two do not reproduce the amount", func(t *testing.T) {
		// 23% withheld on half of 6686.21: 11.50% gives 768.91, 11.5001% gives
		// the declared 768.92. The second block keeps its declared 8.50%.
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-retained-four-decimal-rate.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		rates := map[cbc.Code]*tax.RateTotal{}
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Retained {
				require.Len(t, cat.Rates, 1)
				rates[cat.Code] = cat.Rates[0]
			}
		}
		require.Contains(t, rates, it.TaxCategoryIRES)
		assert.Equal(t, "11.5001%", rates[it.TaxCategoryIRES].Percent.String())
		assert.Equal(t, "768.92", rates[it.TaxCategoryIRES].Amount.String())
		assert.Equal(t, cbc.Code("23.00"), rates[it.TaxCategoryIRES].Ext.Get(sdi.ExtKeyRetainedRate))
		require.Contains(t, rates, it.TaxCategoryENASARCO)
		assert.Equal(t, "8.50%", rates[it.TaxCategoryENASARCO].Percent.String())
		assert.Equal(t, "568.33", rates[it.TaxCategoryENASARCO].Amount.String())
		assert.Empty(t, rates[it.TaxCategoryENASARCO].Ext.Get(sdi.ExtKeyRetainedRate))

		assert.Equal(t, "8157.18", invoice.Totals.TotalWithTax.String())
		assert.Equal(t, "6819.93", invoice.Totals.Payable.String())
		assert.Nil(t, invoice.Totals.Rounding)
	})
}
