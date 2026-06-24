package fatturapa

import (
	sdi "github.com/invopop/gobl.fatturapa/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// goblBillInvoiceAddPaymentsData adds payment data from the FatturaPA document to the GOBL invoice
func goblBillInvoiceAddPaymentsData(inv *bill.Invoice, paymentsData []*PaymentData) {
	if inv == nil || len(paymentsData) == 0 {
		return
	}

	// Initialize payment if needed
	if inv.Payment == nil {
		inv.Payment = new(bill.PaymentDetails)
	}

	// Track payment conditions
	hasAdvance := false
	hasInstallments := false
	hasFull := false
	hasZeroDays := false

	// First pass: determine what types of payment conditions we have
	for _, paymentData := range paymentsData {
		switch paymentData.Conditions {
		case condizioniPagamentoAdvance:
			hasAdvance = true
		case condizioniPagamentoInstallments:
			hasInstallments = true
		case condizioniPagamentoFull:
			hasFull = true
			// Check if any payment detail with full condition has days = 0 for instant payment
			for _, paymentDetail := range paymentData.Payments {
				if paymentDetail.Days == 0 {
					hasZeroDays = true
					break
				}
			}
		}
	}

	// Second pass: process payment details based on conditions
	for _, paymentData := range paymentsData {
		// Process advances separately
		if paymentData.Conditions == condizioniPagamentoAdvance {
			for _, paymentDetail := range paymentData.Payments {
				goblBillPaymentAddAdvancePayment(inv.Payment, paymentDetail)
			}
			continue
		}

		// Process payment details
		for _, paymentDetail := range paymentData.Payments {
			// Add due dates
			if paymentDetail.DueDate != "" {
				goblBillPaymentAddDueDate(inv.Payment, paymentDetail)
			}

			// Add payment instructions (only once)
			// We only add the first payment instruction that appears in the document
			if inv.Payment.Instructions == nil && (paymentDetail.IBAN != "" || paymentDetail.BIC != "" || paymentDetail.Method != "") {
				goblBillPaymentAddPaymentInstructions(inv.Payment, paymentDetail)
			}
		}
	}

	// Set payment terms key based on conditions
	if inv.Payment.Terms == nil {
		inv.Payment.Terms = new(pay.Terms)
	}

	// If it has installments, it's always due date
	if hasInstallments {
		inv.Payment.Terms.Key = pay.TermKeyDueDate
	} else if hasFull {
		// If it has full but the days is 0, then we do not know so we set pending, otherwise it's still due date
		if hasZeroDays {
			inv.Payment.Terms.Key = pay.TermKeyPending
		} else {
			inv.Payment.Terms.Key = pay.TermKeyDueDate
		}
	} else if hasAdvance {
		// If it only has advances, it's advanced
		inv.Payment.Terms.Key = pay.TermKeyAdvanced
	}
}

// goblBillPaymentAddAdvancePayment adds an advance payment from the FatturaPA document to the GOBL invoice
func goblBillPaymentAddAdvancePayment(payment *bill.PaymentDetails, paymentDetail *PaymentDetailRow) {
	if payment == nil || paymentDetail == nil {
		return
	}

	// Parse amount
	amount, err := parseAmount(paymentDetail.Amount)
	if err != nil {
		return
	}

	// Create advance payment
	advance := &pay.Record{
		Amount:      amount,
		Description: "Advance payment",
	}

	// Add payment method if available
	if paymentDetail.Method != "" {
		setPaymentMethodAndKey(paymentDetail.Method, &advance.Ext, &advance.Key)
	}

	// Add date if available
	if paymentDetail.Date != "" {
		date, err := parseDate(paymentDetail.Date)
		if err != nil {
			return
		}
		advance.Date = &date
	}

	// Add advance to payment
	payment.Advances = append(payment.Advances, advance)
}

// goblBillPaymentAddDueDate adds a due date from the FatturaPA document to the GOBL invoice
func goblBillPaymentAddDueDate(payment *bill.PaymentDetails, paymentDetail *PaymentDetailRow) {
	if payment == nil || paymentDetail == nil || paymentDetail.DueDate == "" {
		return
	}

	// Initialize payment terms if needed
	if payment.Terms == nil {
		payment.Terms = new(pay.Terms)
	}

	// Parse amount and due date
	amount, err1 := parseAmount(paymentDetail.Amount)
	if err1 != nil {
		return
	}

	// Create due date
	dueDate := &pay.DueDate{
		Amount: amount,
	}

	// Parse due date
	date, err := parseDate(paymentDetail.DueDate)
	if err != nil {
		return
	}
	dueDate.Date = &date

	// Add due date to payment terms
	payment.Terms.DueDates = append(payment.Terms.DueDates, dueDate)
}

// goblBillPaymentAddPaymentInstructions adds payment instructions from the FatturaPA document to the GOBL invoice
func goblBillPaymentAddPaymentInstructions(payment *bill.PaymentDetails, paymentDetail *PaymentDetailRow) {
	if payment == nil || paymentDetail == nil {
		return
	}

	// Create payment instructions
	if payment.Instructions == nil {
		payment.Instructions = new(pay.Instructions)
	}

	// Add a Key
	payment.Instructions.Key = pay.MeansKeyAny

	// Add payment method if available
	if paymentDetail.Method != "" {
		setPaymentMethodAndKey(paymentDetail.Method, &payment.Instructions.Ext, &payment.Instructions.Key)
	}

	// Add credit transfer if IBAN or BIC is available
	if paymentDetail.IBAN != "" || paymentDetail.BIC != "" {
		creditTransfer := pay.CreditTransfer{
			IBAN: cbc.Code(paymentDetail.IBAN),
			BIC:  cbc.Code(paymentDetail.BIC),
		}
		if paymentDetail.FinancialInstitution != "" {
			creditTransfer.Name = paymentDetail.FinancialInstitution
		}
		payment.Instructions.CreditTransfer = append(payment.Instructions.CreditTransfer, &creditTransfer)
	}
}

// setPaymentMethodAndKey sets the payment method extension and key based on the method code
func setPaymentMethodAndKey(methodCode string, ext *tax.Extensions, key *cbc.Key) {
	*ext = ext.Set(sdi.ExtKeyPaymentMeans, cbc.Code(methodCode))

	// Find the key for the payment method code
	keyMap := sdi.PaymentMeansExtensions()
	for k, v := range keyMap.All() {
		if v == cbc.Code(methodCode) {
			if k == pay.MeansKeyOnline {
				// Double mapping for MP08 causes errors when parsing values
				// as it can either map to card or online, so we force it to card
				k = pay.MeansKeyCard
			}
			*key = k
			break
		}
	}
}
