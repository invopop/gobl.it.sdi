package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testInvoiceSplitPayment is 10 x 100.00 under split payment: 1,000.00 plus
// 220.00 VAT that the customer pays directly to the tax authority.
func testInvoiceSplitPayment(t *testing.T) *bill.Invoice {
	t.Helper()
	inv := testInvoiceStandard(t)
	inv.Tax.PricesInclude = ""
	inv.Lines[0].Discounts = nil
	inv.Lines[0].Taxes[0].Ext = splitPaymentExt()
	return inv
}

func splitPaymentExt() tax.Extensions {
	return tax.ExtensionsOf(cbc.CodeMap{
		sdi.ExtKeyVATLiability: sdi.ExtCodeVATLiabilitySplitPayment,
	})
}

func splitPaymentAdvances(inv *bill.Invoice) []*pay.Record {
	if inv.Payment == nil {
		return nil
	}
	var list []*pay.Record
	for _, a := range inv.Payment.Advances {
		if a.Waiver == sdi.WaiverVATSplitPayment {
			list = append(list, a)
		}
	}
	return list
}

func TestInvoiceSplitPayment(t *testing.T) {
	t.Run("adds the split payment VAT as a waiver advance", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		require.NoError(t, inv.Calculate())

		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "Scissione dei pagamenti", advs[0].Description)
		assert.Empty(t, advs[0].Key)
		assert.Equal(t, "220.00", advs[0].Amount.String())
		assert.Equal(t, "1220.00", inv.Totals.Payable.String())
		require.NotNil(t, inv.Totals.Due)
		assert.Equal(t, "1000.00", inv.Totals.Due.String())
	})

	t.Run("works out percentage due dates on the net amount", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Terms: &pay.Terms{
				DueDates: []*pay.DueDate{
					{Date: cal.NewDate(2022, 7, 13), Percent: num.NewPercentage(100, 2)},
				},
			},
			Instructions: &pay.Instructions{Key: pay.MeansKeyCreditTransfer},
		}
		require.NoError(t, inv.Calculate())
		require.NotNil(t, inv.Payment.Terms.DueDates[0].Amount)
		assert.Equal(t, "1000.00", inv.Payment.Terms.DueDates[0].Amount.String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("keeps a single advance when recalculated", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.Calculate())
		assert.Len(t, splitPaymentAdvances(inv), 1)
		assert.Equal(t, "1000.00", inv.Totals.Due.String())
	})

	t.Run("removes the advance when no VAT is under split payment any more", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		require.NoError(t, inv.Calculate())
		inv.Lines[0].Taxes[0].Ext = tax.Extensions{}
		require.NoError(t, inv.Calculate())
		assert.Nil(t, inv.Payment)
		assert.Nil(t, inv.Totals.Due)
	})

	t.Run("keeps the advances actually paid", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Advances: []*pay.Record{
				{Key: pay.MeansKeyCard, Description: "Deposit", Amount: num.MakeAmount(10000, 2)},
			},
		}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Payment.Advances, 2)
		assert.Equal(t, "Deposit", inv.Payment.Advances[0].Description)
		assert.Equal(t, "900.00", inv.Totals.Due.String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("only counts the VAT under split payment", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Lines = append(inv.Lines, &bill.Line{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "Other Item", Price: num.NewAmount(50000, 2)},
			Taxes:    tax.Set{{Category: tax.CategoryVAT, Rate: "general"}},
		})
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "220.00", advs[0].Amount.String())
		assert.Equal(t, "1830.00", inv.Totals.Payable.String())
		assert.Equal(t, "1610.00", inv.Totals.Due.String())
	})

	t.Run("skips split payment lines that charge no VAT", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Lines[0].Taxes[0] = &tax.Combo{
			Category: tax.CategoryVAT,
			Key:      tax.KeyOutsideScope,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyExempt:       "N2.2",
				sdi.ExtKeyVATLiability: sdi.ExtCodeVATLiabilitySplitPayment,
			}),
		}
		require.NoError(t, inv.Calculate())
		assert.Nil(t, inv.Payment)
		assert.Nil(t, inv.Totals.Due)
	})

	t.Run("keeps the payment terms when recalculated", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Terms: &pay.Terms{
				DueDates: []*pay.DueDate{
					{Date: cal.NewDate(2022, 7, 13), Percent: num.NewPercentage(100, 2)},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.Calculate())
		require.NotNil(t, inv.Payment)
		require.NotNil(t, inv.Payment.Terms)
		assert.Len(t, inv.Payment.Terms.DueDates, 1)
	})

	t.Run("keeps the payment instructions when recalculated", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{Key: pay.MeansKeyCreditTransfer},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.Calculate())
		require.NotNil(t, inv.Payment)
		assert.NotNil(t, inv.Payment.Instructions)
	})

	t.Run("keeps the payee when recalculated", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Payee: &org.Party{Name: "Factoring S.p.A."},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.Calculate())
		require.NotNil(t, inv.Payment)
		assert.NotNil(t, inv.Payment.Payee)
	})

	t.Run("replaces a waiver advance given by the user", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Payment = &bill.PaymentDetails{
			Advances: []*pay.Record{
				{Waiver: sdi.WaiverVATSplitPayment, Description: "Custom", Amount: num.MakeAmount(100, 2)},
			},
		}
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "Scissione dei pagamenti", advs[0].Description)
		assert.Equal(t, "220.00", advs[0].Amount.String())
	})

	t.Run("works out the VAT from prices that include it", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Tax.PricesInclude = tax.CategoryVAT
		inv.Lines[0].Item.Price = num.NewAmount(12200, 2)
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "220.00", advs[0].Amount.String())
		assert.Equal(t, "1220.00", inv.Totals.Payable.String())
		assert.Equal(t, "1000.00", inv.Totals.Due.String())
	})

	t.Run("adds up every VAT rate under split payment", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Lines = append(inv.Lines, &bill.Line{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "Book", Price: num.NewAmount(5000, 2)},
			Taxes:    tax.Set{{Category: tax.CategoryVAT, Rate: tax.RateIntermediate, Ext: splitPaymentExt()}},
		})
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "225.00", advs[0].Amount.String())
		assert.Equal(t, "1275.00", inv.Totals.Payable.String())
		assert.Equal(t, "1050.00", inv.Totals.Due.String())
	})

	t.Run("comes off what's left after a withholding", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Lines[0].Taxes = append(inv.Lines[0].Taxes, &tax.Combo{
			Category: it.TaxCategoryIRPEF,
			Percent:  num.NewPercentage(200, 3),
			Ext:      tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyRetained: "A"}),
		})
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "220.00", advs[0].Amount.String())
		assert.Equal(t, "1020.00", inv.Totals.Payable.String())
		assert.Equal(t, "800.00", inv.Totals.Due.String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("applies to credit notes too", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		inv.Type = bill.InvoiceTypeCreditNote
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "220.00", advs[0].Amount.String())
		assert.Equal(t, "1000.00", inv.Totals.Due.String())
	})

	t.Run("counts split payment VAT on a charge", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Tax.PricesInclude = ""
		inv.Lines[0].Discounts = nil
		inv.Charges = []*bill.Charge{
			{
				Reason: "Delivery",
				Amount: num.MakeAmount(10000, 2),
				Taxes:  tax.Set{{Category: tax.CategoryVAT, Rate: tax.RateGeneral, Ext: splitPaymentExt()}},
			},
		}
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "22.00", advs[0].Amount.String())
	})

	t.Run("leaves a bypassed invoice as it was calculated", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		require.NoError(t, inv.Calculate())
		inv.Tags = tax.WithTags(tax.TagBypass)
		require.NoError(t, inv.Calculate())
		advs := splitPaymentAdvances(inv)
		require.Len(t, advs, 1)
		assert.Equal(t, "220.00", advs[0].Amount.String())
	})

	t.Run("leaves invoices without split payment alone", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.Nil(t, inv.Payment)
		assert.Nil(t, inv.Totals.Due)
	})
}
