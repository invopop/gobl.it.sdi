package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl.fatturapa/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDettaglioLinee(t *testing.T) {
	t.Run("should contain the line info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dl := doc.Body[0].GoodsServices.LineDetails[0]

		assert.Equal(t, "1", dl.LineNumber)
		assert.Equal(t, "Development services", dl.Description)
		assert.Equal(t, "20.00", dl.Quantity)
		assert.Equal(t, "90.00", dl.UnitPrice)
		assert.Equal(t, "1620.00", dl.TotalPrice)
		assert.Equal(t, "22.00", dl.TaxRate)
		assert.Equal(t, "", dl.TaxNature)

		sm := dl.PriceAdjustments[0]

		assert.Equal(t, "SC", sm.Type)
		assert.Equal(t, "10.00", sm.Percent)
		assert.Equal(t, "9.0000", sm.Amount)

		dl = doc.Body[0].GoodsServices.LineDetails[1]

		assert.Equal(t, "2", dl.LineNumber)
		assert.Equal(t, "N2.2", dl.TaxNature)
	})
}

func TestDettaglioLineePeriod(t *testing.T) {
	t.Run("should map period dates from GOBL to FatturaPA", func(t *testing.T) {
		env := test.LoadTestFile("invoice-services-period.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dl := doc.Body[0].GoodsServices.LineDetails[0]
		assert.Equal(t, "2024-01-01", dl.PeriodStart)
		assert.Equal(t, "2024-01-31", dl.PeriodEnd)

		// Line without period should have empty dates
		dl2 := doc.Body[0].GoodsServices.LineDetails[1]
		assert.Empty(t, dl2.PeriodStart)
		assert.Empty(t, dl2.PeriodEnd)
	})

	t.Run("should omit missing period date", func(t *testing.T) {
		env := test.LoadTestFile("invoice-services-period.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Period.End = cal.Date{}
		})
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dl := doc.Body[0].GoodsServices.LineDetails[0]
		assert.Equal(t, "2024-01-01", dl.PeriodStart)
		assert.Empty(t, dl.PeriodEnd)
	})
}

func TestAltriDatiGestionaliINVCONT(t *testing.T) {
	t.Run("should add INVCONT for N2.1 with reverse-charge tag", func(t *testing.T) {
		env := test.LoadTestFile("invoice-reverse-charge.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		for _, dl := range doc.Body[0].GoodsServices.LineDetails {
			require.Len(t, dl.OtherData, 1)
			assert.Equal(t, "INVCONT", dl.OtherData[0].DataType)
			assert.Equal(t, "Inversione contabile - art. 21 c.6 bis lett. a) DPR 633/72", dl.OtherData[0].TextReference)
		}
	})

	t.Run("should not add INVCONT for N2.1 without reverse-charge tag", func(t *testing.T) {
		env := test.LoadTestFile("invoice-hotel.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		// First line has N2.1 but no reverse-charge tag on the invoice
		dl := doc.Body[0].GoodsServices.LineDetails[0]
		assert.Equal(t, "N2.1", dl.TaxNature)
		assert.Empty(t, dl.OtherData)
	})
}

func TestAltriDatiGestionaliAttributes(t *testing.T) {
	t.Run("should map item attributes by value type", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		date := cal.MakeDate(2024, 3, 15)
		amount := num.MakeAmount(1250, 2)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Type: "LOTTO", Text: "ABC-123"},
				{Type: "COLORE", Code: "RAL5010"},
				{Type: "PESO", Amount: &amount, Unit: org.UnitKilogram},
				{Type: "SCADENZA", Date: &date},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		od := doc.Body[0].GoodsServices.LineDetails[0].OtherData
		require.Len(t, od, 4)

		// Text -> RiferimentoTesto
		assert.Equal(t, "LOTTO", od[0].DataType)
		assert.Equal(t, "ABC-123", od[0].TextReference)

		// Code -> RiferimentoTesto
		assert.Equal(t, "COLORE", od[1].DataType)
		assert.Equal(t, "RAL5010", od[1].TextReference)

		// Amount -> RiferimentoNumero. The unit is dropped, there is no field for it.
		assert.Equal(t, "PESO", od[2].DataType)
		assert.Equal(t, "12.50", od[2].NumReference)
		assert.Empty(t, od[2].TextReference)

		// Date -> RiferimentoData
		assert.Equal(t, "SCADENZA", od[3].DataType)
		assert.Equal(t, "2024-03-15", od[3].DateReference)
	})

	t.Run("should map every value of an attribute into a single block", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		date := cal.MakeDate(2025, 3, 10)
		amount := num.MakeAmount(8000, 2)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Type: "INTENTO", Text: "08060120341234567-000001", Date: &date},
				{Type: "CASSA-PREV", Text: "ENASARCO TC07", Amount: &amount},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		od := doc.Body[0].GoodsServices.LineDetails[0].OtherData
		require.Len(t, od, 2)

		// Declaration of intent: protocol number and the AdE receipt date.
		assert.Equal(t, "INTENTO", od[0].DataType)
		assert.Equal(t, "08060120341234567-000001", od[0].TextReference)
		assert.Equal(t, "2025-03-10", od[0].DateReference)
		assert.Empty(t, od[0].NumReference)

		// Pension fund: which fund and how much was contributed.
		assert.Equal(t, "CASSA-PREV", od[1].DataType)
		assert.Equal(t, "ENASARCO TC07", od[1].TextReference)
		assert.Equal(t, "80.00", od[1].NumReference)
		assert.Empty(t, od[1].DateReference)
	})

	t.Run("should prefer the text over the code, as both share RiferimentoTesto", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Type: "COLORE", Text: "rosso", Code: "RAL5010"},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		od := doc.Body[0].GoodsServices.LineDetails[0].OtherData
		require.Len(t, od, 1)
		assert.Equal(t, "rosso", od[0].TextReference)
	})

	t.Run("should fall back to the attribute key when no type is set", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Key: org.AttributeKeyColor, Text: "red"},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		od := doc.Body[0].GoodsServices.LineDetails[0].OtherData
		require.Len(t, od, 1)
		assert.Equal(t, "color", od[0].DataType)
		assert.Equal(t, "red", od[0].TextReference)
	})

	t.Run("should skip valueless, oversized, and reserved attributes", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Attributes = []*org.Attribute{
				{Type: "OK", Text: "keep"},
				{Type: "NOVALUE"},                       // no value
				{Type: "INVCONT", Text: "reserved"},     // reverse-charge marker
				{Type: "TOOLONGTIPO", Text: "over ten"}, // over ten characters
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		od := doc.Body[0].GoodsServices.LineDetails[0].OtherData
		require.Len(t, od, 1)
		assert.Equal(t, "OK", od[0].DataType)
		assert.Equal(t, "keep", od[0].TextReference)
	})
}

func TestDatiRiepilogo(t *testing.T) {
	t.Run("should contain the tax summary info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dr := doc.Body[0].GoodsServices.TaxSummary[0]

		assert.Equal(t, "22.00", dr.TaxRate)
		assert.Equal(t, "1620.00", dr.TaxableAmount)
		assert.Equal(t, "356.40", dr.TaxAmount)
		assert.Equal(t, "", dr.TaxNature)
		assert.Equal(t, "", dr.LegalReference)

		dr = doc.Body[0].GoodsServices.TaxSummary[1]

		assert.Equal(t, "N2.2", dr.TaxNature)
		assert.Equal(t, "S", dr.TaxLiability)
		assert.Equal(t, "Non soggette - altri casi", dr.LegalReference)
	})
}

func TestNegativeQuantityConversion(t *testing.T) {
	t.Run("should convert negative quantities to negative prices", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)

		// Modify the invoice to have a negative quantity
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Quantity = num.MakeAmount(-2000, 2) // -20.00
			require.NoError(t, inv.Calculate())
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dl := doc.Body[0].GoodsServices.LineDetails[0]

		// Quantity should be positive
		assert.Equal(t, "20.00", dl.Quantity)

		// Unit price should be negative
		assert.Equal(t, "-90.00", dl.UnitPrice)

		// Total price should still be negative
		assert.Equal(t, "-1620.00", dl.TotalPrice)
	})

	t.Run("should handle discounts correctly with negative quantities", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)

		// Modify the invoice to have a negative quantity
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Quantity = num.MakeAmount(-2000, 2) // -20.00
			require.NoError(t, inv.Calculate())
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dl := doc.Body[0].GoodsServices.LineDetails[0]

		// Price adjustments (discounts/charges) should have positive amounts
		// regardless of quantity sign
		require.NotEmpty(t, dl.PriceAdjustments)

		// The discount amount should be positive (9.0000), not negative
		// Original line has 10% discount = 180 total / 20 units = 9 per unit
		assert.Equal(t, "SC", dl.PriceAdjustments[0].Type)
		assert.Equal(t, "10.00", dl.PriceAdjustments[0].Percent)
		assert.Equal(t, "9.0000", dl.PriceAdjustments[0].Amount)
	})
}
