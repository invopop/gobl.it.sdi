package sdi

import (
	"slices"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// WaiverVATSplitPayment identifies the payment advance for the VAT that the
// customer pays directly to the tax authority under split payment (scissione
// dei pagamenti, art. 17-ter DPR 633/72), which the supplier never collects.
const WaiverVATSplitPayment cbc.Key = "vat-split-payment"

// normalizeSplitPayment adds the split payment VAT as a waiver advance, so the
// amount due and the due dates are net of it while the VAT stays declared. Any
// previous one is removed first, so recalculating never duplicates it, and an
// invoice that no longer has split payment loses it. A bypassed invoice isn't
// calculated, so it keeps the advance it was calculated with.
func normalizeSplitPayment(inv *bill.Invoice) {
	if inv.HasTags(tax.TagBypass) {
		return
	}
	if p := inv.Payment; p != nil {
		n := len(p.Advances)
		p.Advances = slices.DeleteFunc(p.Advances, func(a *pay.Record) bool {
			return a != nil && a.Waiver == WaiverVATSplitPayment
		})
		if len(p.Advances) < n && len(p.Advances) == 0 && p.Terms == nil && p.Instructions == nil && p.Payee == nil && p.Payer == nil {
			inv.Payment = nil
		}
	}
	if !invoiceHasSplitPayment(inv) {
		return
	}
	if inv.Payment == nil {
		inv.Payment = new(bill.PaymentDetails)
	}
	// The amount is calculated by GOBL from the rounded VAT totals matching the filter.
	inv.Payment.Advances = append(inv.Payment.Advances, &pay.Record{
		Waiver:      WaiverVATSplitPayment,
		Description: "Scissione dei pagamenti",
		Taxes:       splitPaymentTaxes(),
	})
}

// splitPaymentTaxes selects the VAT the customer pays directly to the tax
// authority.
func splitPaymentTaxes() []*tax.Filter {
	return []*tax.Filter{
		{
			Category: tax.CategoryVAT,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				ExtKeyVATLiability: ExtCodeVATLiabilitySplitPayment,
			}),
		},
	}
}

// invoiceHasSplitPayment reports whether any line or charge charges VAT under
// split payment. Lines that charge none, such as exempt ones, may still carry
// the split payment liability but have nothing to pay to the tax authority.
func invoiceHasSplitPayment(inv *bill.Invoice) bool {
	for _, l := range inv.Lines {
		if chargesSplitPaymentVAT(l.Taxes) {
			return true
		}
	}
	for _, c := range inv.Charges {
		if chargesSplitPaymentVAT(c.Taxes) {
			return true
		}
	}
	return false
}

func chargesSplitPaymentVAT(taxes tax.Set) bool {
	for _, c := range taxes {
		if c.Category == tax.CategoryVAT && c.Key == tax.KeyStandard &&
			c.Ext.Get(ExtKeyVATLiability) == ExtCodeVATLiabilitySplitPayment {
			return true
		}
	}
	return false
}
