package fatturapa_test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.fatturapa/test"
	"github.com/invopop/gobl/addons/it/sdi"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestItemsInConversion tests the conversion of line items and tax summaries from FatturaPA XML to GOBL
func TestItemsInConversion(t *testing.T) {
	t.Run("should convert line items correctly", func(t *testing.T) {
		// Load the XML file with line items
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

		// Check the line items
		require.NotEmpty(t, invoice.Lines)
		require.GreaterOrEqual(t, len(invoice.Lines), 2, "Invoice should have at least 2 lines")

		// Check the first line
		line1 := invoice.Lines[0]
		assert.Equal(t, "Development services", line1.Item.Name)
		assert.True(t, line1.Quantity.Compare(num.MakeAmount(2000, 2)) == 0, "Quantity should be 20.00")
		assert.True(t, line1.Item.Price.Compare(num.MakeAmount(9000, 2)) == 0, "Unit price should be 90.00")
		assert.True(t, line1.Total.Compare(num.MakeAmount(162000, 2)) == 0, "Total should be 1620.00")

		// Check the tax rate on the first line
		require.NotEmpty(t, line1.Taxes)
		taxRate := num.MakePercentage(220, 3) // 22.0% with correct precision
		require.NotNil(t, line1.Taxes[0].Percent)
		assert.True(t, line1.Taxes[0].Percent.Compare(taxRate) == 0, "Tax rate should be 22.0%")

		// Check for price adjustments (discounts)
		require.NotEmpty(t, line1.Discounts)
		discountPercent := num.MakePercentage(100, 3) // 10.0% with correct precision
		require.NotNil(t, line1.Discounts[0].Percent)
		assert.True(t, line1.Discounts[0].Percent.Compare(discountPercent) == 0, "Discount percent should be 10.0%")
		assert.True(t, line1.Discounts[0].Amount.Compare(num.MakeAmount(18000, 2)) == 0, "Discount amount should be 180.00")

		// Check the second line
		line2 := invoice.Lines[1]
		assert.NotEmpty(t, line2.Item.Name)

		// Check for tax exemption on the second line
		require.NotEmpty(t, line2.Taxes)
		assert.Equal(t, tax.CategoryVAT, line2.Taxes[0].Category)
		assert.Equal(t, cbc.Code("N2.2"), line2.Taxes[0].Ext.Get(sdi.ExtKeyExempt))
	})

	t.Run("should default quantity to 1 when unit price is zero", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-zero-price.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		require.Len(t, invoice.Lines, 2)

		// Second line has zero unit price and no quantity in XML
		line2 := invoice.Lines[1]
		assert.Equal(t, "Free item", line2.Item.Name)
		assert.True(t, line2.Item.Price.IsZero(), "Unit price should be zero")
		assert.True(t, line2.Quantity.Compare(num.MakeAmount(1, 0)) == 0, "Quantity should default to 1")
		assert.True(t, line2.Total.IsZero(), "Total should be zero")
	})

	t.Run("should convert tax summaries correctly", func(t *testing.T) {
		// Load the XML file with tax summaries
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

		// Check the tax totals
		require.NotNil(t, invoice.Totals)
		require.NotNil(t, invoice.Totals.Taxes)

		// Check the VAT tax category
		var vatCategory *tax.CategoryTotal
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Code == "VAT" {
				vatCategory = cat
				break
			}
		}

		require.NotNil(t, vatCategory, "VAT tax category should exist")

		// Check the VAT tax amount
		assert.True(t, vatCategory.Amount.Compare(num.MakeAmount(35640, 2)) == 0, "VAT amount should be 356.40")

		// Check the VAT tax rates
		require.NotEmpty(t, vatCategory.Rates)

		// Find the 22% VAT rate
		var vat22Rate *tax.RateTotal
		for _, rt := range vatCategory.Rates {
			if rt.Percent != nil && rt.Percent.Compare(num.MakePercentage(220, 3)) == 0 {
				vat22Rate = rt
				break
			}
		}

		require.NotNil(t, vat22Rate, "22% VAT rate should exist")
		assert.True(t, vat22Rate.Base.Compare(num.MakeAmount(162000, 2)) == 0, "VAT base should be 1620.00")
		assert.True(t, vat22Rate.Amount.Compare(num.MakeAmount(35640, 2)) == 0, "VAT amount should be 356.40")

		// Find the exempt rate
		var exemptRate *tax.RateTotal
		for _, rt := range vatCategory.Rates {
			if rt.Percent == nil || rt.Percent.IsZero() {
				exemptRate = rt
				break
			}
		}

		require.NotNil(t, exemptRate, "Exempt rate should exist")
		assert.NotEmpty(t, exemptRate.Base)
		assert.True(t, exemptRate.Amount.IsZero(), "Exempt rate amount should be zero")
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

		// Check the retained taxes
		require.NotNil(t, invoice.Totals)
		require.NotNil(t, invoice.Totals.Taxes)

		// Find the retained tax category
		var retainedCategory *tax.CategoryTotal
		for _, cat := range invoice.Totals.Taxes.Categories {
			if cat.Retained {
				retainedCategory = cat
				break
			}
		}

		require.NotNil(t, retainedCategory, "Retained tax category should exist")

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
}

func TestINVCONTInConversion(t *testing.T) {
	t.Run("should set reverse-charge tag when INVCONT is present", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-reverse-charge.xml"))
		require.NoError(t, err)

		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.True(t, invoice.HasTags(tax.TagReverseCharge))
	})
}

func TestAltriDatiGestionaliAttributesInConversion(t *testing.T) {
	t.Run("should round-trip item attributes through AltriDatiGestionali", func(t *testing.T) {
		// Convert an invoice with an attribute of each value type to FatturaPA,
		// then parse it back and check the attributes survive.
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		date := cal.MakeDate(2024, 3, 15)
		amount := num.MakeAmount(1250, 2)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Type: "LOTTO", Text: "ABC-123"},
				{Type: "PESO", Amount: &amount},
				{Type: "SCADENZA", Date: &date},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		data, err := xml.MarshalIndent(doc, "", "\t")
		require.NoError(t, err)

		out, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		invoice, ok := out.Extract().(*bill.Invoice)
		require.True(t, ok)

		attrs := invoice.Lines[0].Item.Attributes
		require.Len(t, attrs, 3)

		// RiferimentoTesto -> Text
		assert.Equal(t, cbc.Code("LOTTO"), attrs[0].Type)
		assert.Equal(t, "ABC-123", attrs[0].Text)

		// RiferimentoNumero -> Amount
		assert.Equal(t, cbc.Code("PESO"), attrs[1].Type)
		require.NotNil(t, attrs[1].Amount)
		assert.Equal(t, "12.50", attrs[1].Amount.String())

		// RiferimentoData -> Date
		assert.Equal(t, cbc.Code("SCADENZA"), attrs[2].Type)
		require.NotNil(t, attrs[2].Date)
		assert.Equal(t, "2024-03-15", attrs[2].Date.String())
	})
}
