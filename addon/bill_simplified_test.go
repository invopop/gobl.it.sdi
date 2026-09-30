package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testInvoiceSimplified is a 180.00 EUR simplified invoice (2 × 100.00 with
// VAT included, less 10%), leaving the scenarios to pick FSM10 and TD07.
func testInvoiceSimplified(t *testing.T) *bill.Invoice {
	t.Helper()
	inv := testInvoiceStandard(t)
	inv.SetTags(tax.TagSimplified)
	inv.Tax.Ext = tax.Extensions{}
	inv.Lines[0].Quantity = num.MakeAmount(2, 0)
	return inv
}

func simplifiedPreceding() []*org.DocumentRef {
	return []*org.DocumentRef{
		{
			Code:      "122TEST",
			IssueDate: cal.NewDate(2022, 6, 1),
			Reason:    "Returned item",
		},
	}
}

func withLineVAT(inv *bill.Invoice, combo *tax.Combo) {
	combo.Category = tax.CategoryVAT
	inv.Lines[0].Taxes = tax.Set{combo}
}

func exemptVAT(key cbc.Key, code cbc.Code) *tax.Combo {
	return &tax.Combo{
		Key: key,
		Ext: tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyExempt: code}),
	}
}

func TestSimplifiedInvoiceValidation(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "FSM10", inv.Tax.Ext.Get(sdi.ExtKeyFormat).String())
		assert.Equal(t, "TD07", inv.Tax.Ext.Get(sdi.ExtKeyDocumentType).String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("credit note", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "TD08", inv.Tax.Ext.Get(sdi.ExtKeyDocumentType).String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("debit note", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeDebitNote
		inv.Preceding = simplifiedPreceding()
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "TD09", inv.Tax.Ext.Get(sdi.ExtKeyDocumentType).String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("standard invoice with an incomplete preceding reference", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].IssueDate = nil
		inv.Preceding[0].Reason = ""
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("customer identified by fiscal code only", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Customer = &org.Party{
			TaxID: &tax.Identity{Country: "IT"},
			Identities: []*org.Identity{
				{Key: it.IdentityKeyFiscalCode, Code: "RSSGNN60R30H501U"},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("customer without an address", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Customer.Addresses = nil
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("charge and discount with VAT", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		vat := tax.Set{{Category: tax.CategoryVAT, Rate: "general"}}
		inv.Charges = []*bill.Charge{
			{Key: bill.ChargeKeyStampDuty, Amount: num.MakeAmount(200, 2), Taxes: vat},
		}
		inv.Discounts = []*bill.Discount{
			{Reason: "Loyalty", Amount: num.MakeAmount(1000, 2), Taxes: vat},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("services outside the scope to a customer outside the EU", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Customer.TaxID = &tax.Identity{Country: "US"}
		withLineVAT(inv, exemptVAT(tax.KeyOutsideScope, "N2.1"))
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("public administration", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.SetTags(tax.TagSimplified, tax.TagB2G)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot be issued to a public administration")
	})

	t.Run("another document type tag", func(t *testing.T) {
		for _, tag := range []cbc.Key{tax.TagSelfBilled, tax.TagPartial, sdi.TagFreelance, sdi.TagDeferred} {
			inv := testInvoiceSimplified(t)
			inv.SetTags(tax.TagSimplified, tag)
			require.NoError(t, inv.Calculate())
			assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot use another document type tag", tag.String())
		}
	})

	t.Run("reverse charge exemption", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		withLineVAT(inv, &tax.Combo{Key: tax.KeyReverseCharge})
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice VAT exemption must be N1, N2.1, N2.2, N3.1, N3.3, N3.4, N3.5, N3.6, N4 or N5")
	})

	t.Run("VAT due in another EU country", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		withLineVAT(inv, &tax.Combo{Country: "FR", Percent: num.NewPercentage(20, 2)})
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice VAT exemption must be N1, N2.1, N2.2, N3.1, N3.3, N3.4, N3.5, N3.6, N4 or N5")
	})

	t.Run("intra-community supply", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		withLineVAT(inv, exemptVAT(tax.KeyIntraCommunity, "N3.2"))
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice VAT exemption must be N1, N2.1, N2.2, N3.1, N3.3, N3.4, N3.5, N3.6, N4 or N5")
	})

	t.Run("services outside the scope to a business in another EU country", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "282741168"}
		withLineVAT(inv, exemptVAT(tax.KeyOutsideScope, "N2.1"))
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot cover operations taxed on a business in another EU country")
	})

	t.Run("exemption on a charge", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Delivery", Amount: num.MakeAmount(500, 2), Taxes: tax.Set{
				{Category: tax.CategoryVAT, Key: tax.KeyReverseCharge},
			}},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice VAT exemption must be N1, N2.1, N2.2, N3.1, N3.3, N3.4, N3.5, N3.6, N4 or N5")
	})

	t.Run("stamp duty exemption code", func(t *testing.T) {
		for _, attr := range []*org.Attribute{
			{Type: "NB1", Text: "Documento assicurativo"},
			{Type: "NB2", Text: "Terzo settore"},
			{Key: "nb3", Text: "Estratto conto"},
		} {
			inv := testInvoiceSimplified(t)
			inv.Lines[0].Item.Attributes = []*org.Attribute{attr}
			require.NoError(t, inv.Calculate())
			assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot declare a stamp duty exemption")
		}
	})

	t.Run("other management data codes", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Lines[0].Item.Attributes = []*org.Attribute{{Type: "TARGA", Text: "AB123CD"}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("sale to a habitual exporter", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		withLineVAT(inv, exemptVAT(tax.KeyExport, "N3.5"))
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot cover a sale to a habitual exporter (N3.5)")
	})

	t.Run("retained tax", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Lines[0].Taxes = append(inv.Lines[0].Taxes, &tax.Combo{
			Category: "IRPEF",
			Ext:      tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyRetained: "A"}),
			Percent:  num.NewPercentage(20, 2),
		})
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot carry retained taxes")
	})

	t.Run("fund contribution", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Charges = []*bill.Charge{
			{
				Key:     sdi.KeyFundContribution,
				Percent: num.NewPercentage(4, 2),
				Taxes:   tax.Set{{Category: tax.CategoryVAT, Rate: "general"}},
				Ext:     tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFundType: "TC04"}),
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot carry fund contributions")
	})

	t.Run("charge without VAT", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Delivery", Amount: num.MakeAmount(500, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice charge requires a VAT tax combo")
	})

	t.Run("stamp duty without VAT", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Charges = []*bill.Charge{
			{Key: bill.ChargeKeyStampDuty, Amount: num.MakeAmount(200, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice charge requires a VAT tax combo")
	})

	t.Run("discount without VAT", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Discounts = []*bill.Discount{
			{Reason: "Loyalty", Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice discount requires a VAT tax combo")
	})

	t.Run("rounding", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		require.NoError(t, inv.Calculate())
		inv.Totals.Rounding = num.NewAmount(1, 2)
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot carry a rounding amount")
	})

	t.Run("credit note without a preceding invoice", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified credit or debit note requires the preceding invoice it corrects")
	})

	t.Run("debit note without a preceding invoice", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeDebitNote
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified credit or debit note requires the preceding invoice it corrects")
	})

	t.Run("two preceding invoices", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = append(simplifiedPreceding(), simplifiedPreceding()...)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice can refer to one preceding invoice only")
	})

	t.Run("preceding without an issue date", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].IssueDate = nil
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice preceding issue date is required")
	})

	t.Run("preceding without a reason", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].Reason = ""
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice preceding reason is required")
	})

	t.Run("preceding issued after the invoice", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].IssueDate = cal.NewDate(2022, 6, 14)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice cannot be issued before the preceding invoice")
	})

	t.Run("supplier is also the customer", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Customer.TaxID = &tax.Identity{Country: "IT", Code: "12345678903"}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice supplier and customer must be different parties")
	})

	t.Run("supplier and customer both outside Italy", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Supplier.TaxID = &tax.Identity{Country: "ES", Code: "B85905495"}
		inv.Customer.TaxID = &tax.Identity{Country: "US"}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice supplier and customer cannot both be outside Italy")
	})

	t.Run("document type that doesn't match the invoice type", func(t *testing.T) {
		for _, typ := range []cbc.Key{bill.InvoiceTypeCreditNote, bill.InvoiceTypeDebitNote} {
			inv := testInvoiceSimplified(t)
			inv.Type = typ
			inv.Preceding = simplifiedPreceding()
			require.NoError(t, inv.Calculate())
			inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyDocumentType, "TD07")
			assert.ErrorContains(t, rules.Validate(inv), "simplified invoice document type must match its type", typ.String())
		}
	})

	t.Run("simplified format with an ordinary document type", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		require.NoError(t, inv.Calculate())
		inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyDocumentType, "TD01")
		assert.ErrorContains(t, rules.Validate(inv), "'it-sdi-format' FSM10 goes with document types TD07, TD08 and TD09 only")
	})

	t.Run("simplified document type in the ordinary format", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		require.NoError(t, inv.Calculate())
		inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyFormat, "FPR12")
		assert.ErrorContains(t, rules.Validate(inv), "'it-sdi-format' FSM10 goes with document types TD07, TD08 and TD09 only")
	})
}

func TestSimplifiedInvoiceLimit(t *testing.T) {
	// 5 × 100.00 with VAT included, less 10%: 450.00 EUR.
	overLimit := func(t *testing.T) *bill.Invoice {
		t.Helper()
		inv := testInvoiceSimplified(t)
		inv.Lines[0].Quantity = num.MakeAmount(5, 0)
		return inv
	}

	t.Run("exactly 400.00", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Lines[0].Quantity = num.MakeAmount(4, 0)
		inv.Lines[0].Discounts = nil
		require.NoError(t, inv.Calculate())
		require.Equal(t, "400.00", inv.Totals.TotalWithTax.String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("flat rate regime", func(t *testing.T) {
		inv := overLimit(t)
		inv.Supplier.Ext = tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFiscalRegime: "RF19"})
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("cross-border VAT franchise regime", func(t *testing.T) {
		inv := overLimit(t)
		inv.Supplier.Ext = tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFiscalRegime: "RF20"})
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("correcting a preceding invoice", func(t *testing.T) {
		inv := overLimit(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("under the limit once converted to EUR", func(t *testing.T) {
		inv := overLimit(t)
		inv.Currency = "USD"
		inv.ExchangeRates = []*currency.ExchangeRate{
			{From: "USD", To: "EUR", Amount: num.MakeAmount(80, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("over the limit", func(t *testing.T) {
		inv := overLimit(t)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice total cannot exceed 400.00 EUR")
	})

	t.Run("standard invoice referring to a preceding one", func(t *testing.T) {
		inv := overLimit(t)
		inv.Preceding = simplifiedPreceding()
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice total cannot exceed 400.00 EUR")
	})

	t.Run("over the limit once converted to EUR", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Lines[0].Quantity = num.MakeAmount(4, 0)
		inv.Currency = "USD"
		inv.ExchangeRates = []*currency.ExchangeRate{
			{From: "USD", To: "EUR", Amount: num.MakeAmount(120, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice total cannot exceed 400.00 EUR")
	})
}
