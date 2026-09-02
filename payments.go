package fatturapa

import (
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
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

func newPaymentData(inv *bill.Invoice) []*PaymentData {
	if inv.Payment == nil {
		return nil
	}

	paymentData := []*PaymentData{}

	if inv.Payment.Advances != nil {
		paymentData = append(paymentData, &PaymentData{
			Conditions: condizioniPagamentoAdvance,
			Payments:   prepareAdvancePaymentDetails(inv),
		})
	}

	if inv.Payment.Instructions == nil {
		return paymentData
	}

	// Determine payment condition based on number of due dates
	condition := condizioniPagamentoFull
	if checkInstallments(inv) {
		condition = condizioniPagamentoInstallments
	}

	paymentDetails := preparePaymentDetails(inv)
	if len(paymentDetails) == 0 {
		return paymentData
	}

	paymentData = append(paymentData, &PaymentData{
		Conditions: condition,
		Payments:   paymentDetails,
	})

	return paymentData
}

func prepareAdvancePaymentDetails(inv *bill.Invoice) []*PaymentDetailRow {
	var dp []*PaymentDetailRow
	payment := inv.Payment

	for _, advance := range payment.Advances {
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

func preparePaymentDetails(inv *bill.Invoice) []*PaymentDetailRow {
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
		for _, dueDate := range payment.Terms.DueDates {
			r := br // copy
			r.DueDate = dueDate.Date.String()
			r.Amount = formatAmount2(&dueDate.Amount)
			dp = append(dp, &r)
		}
	}

	// If there are no due dates, then a single DettaglioPagamento is created
	// with the total payable amount.
	if len(dp) == 0 {
		br.Amount = formatAmount2(&inv.Totals.Payable)
		dp = append(dp, &br)
	}

	return dp
}

// checkInstallments checks if the payment method should be by installments
func checkInstallments(inv *bill.Invoice) bool {
	if inv.Payment != nil && inv.Payment.Terms != nil &&
		// check that if there is more than one due date, then the method should be by installments
		(len(inv.Payment.Terms.DueDates) > 1 ||
			// check that if there is only one due date but the ammount is less than the total payable, then the method should be by installments
			(len(inv.Payment.Terms.DueDates) == 1 && inv.Payment.Terms.DueDates[0].Amount.Compare(inv.Totals.Payable) == -1)) {
		return true
	}
	return false
}
