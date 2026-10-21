package fatturapa

import (
	"fmt"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// PaymentData contains all data related to the payment of the document.
type PaymentData struct {
	Conditions string              `xml:"CondizioniPagamento"`
	Payments   []*PaymentDetailRow `xml:"DettaglioPagamento,omitempty"`
}

// PaymentDetailRow contains data related to a single payment.
type PaymentDetailRow struct {
	Beneficiary          string `xml:"Beneficiario,omitempty"`
	Method               string `xml:"ModalitaPagamento"`
	Date                 string `xml:"DataRiferimentoTerminiPagamento,omitempty"`
	Days                 int64  `xml:"GiorniTerminiPagamento,omitempty"`
	DueDate              string `xml:"DataScadenzaPagamento,omitempty"`
	Amount               string `xml:"ImportoPagamento"`
	FinancialInstitution string `xml:"IstitutoFinanziario,omitempty"`
	IBAN                 string `xml:"IBAN,omitempty"`
	ABI                  string `xml:"ABI,omitempty"`
	CAB                  string `xml:"CAB,omitempty"`
	BIC                  string `xml:"BIC,omitempty"`
	Code                 string `xml:"CodicePagamento,omitempty"`
}

func newPaymentData(inv *bill.Invoice) ([]*PaymentData, error) {
	if inv.Payment == nil {
		return nil, nil
	}

	paymentData := []*PaymentData{}

	if advances := paidAdvances(inv); len(advances) > 0 {
		paymentData = append(paymentData, &PaymentData{
			Conditions: condizioniPagamentoAdvance,
			Payments:   prepareAdvancePaymentDetails(advances),
		})
	}

	if inv.Payment.Instructions == nil {
		return paymentData, nil
	}

	// Determine payment condition based on number of due dates
	condition := condizioniPagamentoFull
	if checkInstallments(inv) {
		condition = condizioniPagamentoInstallments
	}

	paymentDetails, err := preparePaymentDetails(inv)
	if err != nil {
		return nil, err
	}
	if len(paymentDetails) == 0 {
		return paymentData, nil
	}

	paymentData = append(paymentData, &PaymentData{
		Conditions: condition,
		Payments:   paymentDetails,
	})

	return paymentData, nil
}

// paidAdvances returns the advances the customer has actually paid. Waiver
// advances, such as the split payment VAT, record amounts the supplier never
// collects, so they are not payments.
func paidAdvances(inv *bill.Invoice) []*pay.Record {
	var list []*pay.Record
	for _, a := range inv.Payment.Advances {
		if a != nil && a.Waiver == "" {
			list = append(list, a)
		}
	}
	return list
}

func prepareAdvancePaymentDetails(advances []*pay.Record) []*PaymentDetailRow {
	var dp []*PaymentDetailRow

	for _, advance := range advances {
		row := &PaymentDetailRow{
			Method: advance.Ext.Get(sdi.ExtKeyPaymentMeans).String(),
			Amount: formatAmount2(&advance.Amount),
		}

		if advance.Date != nil {
			row.Date = advance.Date.String()
		}
		if advance.CreditTransfer != nil {
			row.IBAN = advance.CreditTransfer.IBAN.String()
			row.BIC = advance.CreditTransfer.BIC.String()
		}
		dp = append(dp, row)
	}

	return dp
}

func preparePaymentDetails(inv *bill.Invoice) ([]*PaymentDetailRow, error) {
	var dp []*PaymentDetailRow
	payment := inv.Payment

	br := PaymentDetailRow{
		Method: payment.Instructions.Ext.Get(sdi.ExtKeyPaymentMeans).String(),
	}
	if len(payment.Instructions.CreditTransfer) > 0 {
		ct1 := payment.Instructions.CreditTransfer[0]
		br.IBAN = ct1.IBAN.String()
		br.BIC = ct1.BIC.String()
		br.FinancialInstitution = ct1.Name
	}

	// First check if there are multiple due dates, and if so, create a
	// DettaglioPagamento for each one.
	if terms := payment.Terms; terms != nil {
		amounts := dueDateAmounts(inv)
		for i, dueDate := range terms.DueDates {
			// ImportoPagamento is mandatory, so there is nothing to emit without it.
			if amounts[i] == nil {
				return nil, fmt.Errorf("due date %d has no amount", i)
			}
			r := br // copy
			r.DueDate = dueDate.Date.String()
			r.Amount = formatAmount2(amounts[i])
			dp = append(dp, &r)
		}
	}

	// If there are no due dates, then a single DettaglioPagamento is created
	// with everything the supplier is owed.
	if len(dp) == 0 {
		due := amountDue(inv)
		br.Amount = formatAmount2(&due)
		dp = append(dp, &br)
	}

	return dp, nil
}

// dueDateAmounts returns the amount to print for each due date. GOBL works
// out percentage due dates over the payable total, which under split payment
// still includes the VAT the customer pays to the tax authority, so those are
// recalculated over what the supplier is actually owed, each rounded on its
// own as GOBL rounds them. Explicit amounts are the issuer's call and left
// untouched.
func dueDateAmounts(inv *bill.Invoice) []*num.Amount {
	dds := inv.Payment.Terms.DueDates
	amounts := make([]*num.Amount, len(dds))
	split := splitPaymentVAT(inv)
	net := amountDue(inv)
	for i, dd := range dds {
		if split.IsZero() || dd.Percent == nil {
			amounts[i] = dd.Amount
			continue
		}
		a := dd.Percent.Of(net).Rescale(net.Exp())
		amounts[i] = &a
	}
	return amounts
}

// amountDue is what the customer still owes the supplier: the payable total
// less the advances actually paid, and less any VAT the customer pays directly
// to the tax authority under split payment. GOBL's own amount due isn't used:
// it already leaves out the split payment waiver advance, so taking the VAT
// off it again would count it twice.
func amountDue(inv *bill.Invoice) num.Amount {
	due := inv.Totals.Payable
	for _, a := range paidAdvances(inv) {
		due = due.Subtract(a.Amount)
	}
	return due.Subtract(splitPaymentVAT(inv))
}

// splitPaymentVAT adds up the VAT marked with split payment liability.
func splitPaymentVAT(inv *bill.Invoice) num.Amount {
	sum := num.MakeAmount(0, inv.Totals.Payable.Exp())
	cat := inv.Totals.Taxes.Category(tax.CategoryVAT)
	if cat == nil {
		return sum
	}
	for _, rt := range cat.Rates {
		if rt.Ext.Get(sdi.ExtKeyVATLiability) == sdi.ExtCodeVATLiabilitySplitPayment {
			sum = sum.Add(rt.Amount)
		}
	}
	return sum
}

// checkInstallments checks if the payment method should be by installments
func checkInstallments(inv *bill.Invoice) bool {
	if inv.Payment == nil || inv.Payment.Terms == nil {
		return false
	}
	dds := inv.Payment.Terms.DueDates
	// More than one due date is always paid by installments.
	if len(dds) > 1 {
		return true
	}
	// A single due date for less than what is owed is a partial payment.
	if len(dds) == 1 && dds[0].Amount != nil {
		return dueDateAmounts(inv)[0].Compare(amountDue(inv)) == -1
	}
	return false
}
