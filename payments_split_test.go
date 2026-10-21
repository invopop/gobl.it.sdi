package fatturapa_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Under split payment (scissione dei pagamenti) the customer pays the VAT
// straight to the tax authority, so the supplier is only ever owed the net.
// ImportoPagamento must reflect that while ImportoTotaleDocumento stays gross.
func TestPaymentsSplitPayment(t *testing.T) {
	t.Run("percent due date is net of split payment VAT", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Equal(t, "1388.40", doc.Body[0].GeneralData.Document.TotalAmount)

		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
		assert.Equal(t, "2023-04-30", dp.Payments[0].DueDate)
	})

	t.Run("explicit net due amount is a full payment, not an installment", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			net := num.MakeAmount(103200, 2)
			inv.Payment.Terms.DueDates[0].Percent = nil
			inv.Payment.Terms.DueDates[0].Amount = &net
		})
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
	})

	t.Run("percent installments are each their own rounded share of the net", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			// 100.03 + 22% = 122.04 with 22.01 of VAT, so each half of the
			// net 100.03 rounds on its own, as GOBL rounds percentage dues.
			inv.Lines = inv.Lines[:1]
			inv.Lines[0].Quantity = num.MakeAmount(1, 0)
			inv.Lines[0].Item.Price = num.NewAmount(10003, 2)
			inv.Lines[0].Discounts = nil
			inv.Discounts = nil
			inv.Charges = nil
			inv.Payment.Terms.DueDates = []*pay.DueDate{
				{Date: cal.NewDate(2023, 4, 30), Percent: num.NewPercentage(50, 2)},
				{Date: cal.NewDate(2023, 5, 31), Percent: num.NewPercentage(50, 2)},
			}
		})
		require.NoError(t, env.Calculate())
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Equal(t, "122.04", doc.Body[0].GeneralData.Document.TotalAmount)

		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP01", dp.Conditions)
		require.Len(t, dp.Payments, 2)
		assert.Equal(t, "50.02", dp.Payments[0].Amount)
		assert.Equal(t, "50.02", dp.Payments[1].Amount)
	})

	t.Run("an advance comes off the net before a percent due date", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Payment.Advances = []*pay.Record{{
				Description: "Deposit",
				Amount:      num.MakeAmount(10000, 2),
			}}
		})
		require.NoError(t, env.Calculate())
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		// 1388.40 gross, 356.40 split VAT, 100.00 already paid.
		dp := doc.Body[0].PaymentsData[1]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "932.00", dp.Payments[0].Amount)
	})

	t.Run("an advance comes off the net when there are no due dates", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Payment.Terms = nil
			inv.Payment.Advances = []*pay.Record{{
				Description: "Deposit",
				Amount:      num.MakeAmount(10000, 2),
			}}
		})
		require.NoError(t, env.Calculate())
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[1]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "932.00", dp.Payments[0].Amount)
	})

	t.Run("a waiver advance for the split payment VAT is not a payment", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Payment.Advances = []*pay.Record{splitPaymentWaiver()}
		})
		require.NoError(t, env.Calculate())
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		// GOBL's amount due is already net, so the VAT must not come off twice.
		require.Len(t, doc.Body[0].PaymentsData, 1)
		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
	})

	t.Run("only the advances actually paid are reported next to the waiver", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Payment.Advances = []*pay.Record{
				{Key: pay.MeansKeyCreditTransfer, Description: "Deposit", Amount: num.MakeAmount(10000, 2)},
				splitPaymentWaiver(),
			}
		})
		require.NoError(t, env.Calculate())
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		require.Len(t, doc.Body[0].PaymentsData, 2)
		adv := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP03", adv.Conditions)
		require.Len(t, adv.Payments, 1)
		assert.Equal(t, "MP05", adv.Payments[0].Method)
		assert.Equal(t, "100.00", adv.Payments[0].Amount)

		dp := doc.Body[0].PaymentsData[1]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "932.00", dp.Payments[0].Amount)
	})

	t.Run("a document calculated without the waiver advance comes to the same amount", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, withoutSplitPaymentWaiver)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		require.Len(t, doc.Body[0].PaymentsData, 1)
		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
	})

	t.Run("a document calculated without the waiver advance or due dates comes to the same amount", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			withoutSplitPaymentWaiver(inv)
			inv.Payment.Terms = nil
		})
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
	})

	t.Run("without due dates the single payment is net of split payment VAT", func(t *testing.T) {
		env := test.LoadTestFile("invoice-split-payment.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Payment.Terms = nil
		})
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]
		assert.Equal(t, "TP02", dp.Conditions)
		require.Len(t, dp.Payments, 1)
		assert.Equal(t, "1032.00", dp.Payments[0].Amount)
	})
}

// withoutSplitPaymentWaiver puts an invoice back the way GOBL calculated it
// before the split payment VAT was recorded as a waiver advance: no advance,
// no amount due, and due dates worked out on the gross total.
func withoutSplitPaymentWaiver(inv *bill.Invoice) {
	inv.Payment.Advances = nil
	inv.Totals.Advances = nil
	inv.Totals.Due = nil
	if inv.Payment.Terms == nil {
		return
	}
	for _, dd := range inv.Payment.Terms.DueDates {
		if dd.Percent != nil {
			gross := dd.Percent.Of(inv.Totals.Payable)
			dd.Amount = &gross
		}
	}
}

// splitPaymentWaiver is the advance that records the split payment VAT as an
// amount the supplier never collects, with GOBL working out its amount.
func splitPaymentWaiver() *pay.Record {
	return &pay.Record{
		Waiver:      "vat-split-payment",
		Description: "Scissione dei pagamenti",
		Taxes: []*tax.Filter{{
			Category: tax.CategoryVAT,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyVATLiability: sdi.ExtCodeVATLiabilitySplitPayment,
			}),
		}},
	}
}
