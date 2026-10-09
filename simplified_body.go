package fatturapa

import (
	"fmt"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

const (
	descriptionCharge   = "Maggiorazione"
	descriptionDiscount = "Sconto"
)

// SimplifiedBody contains the invoice data of a simplified invoice apart from
// the parties involved, which are contained in SimplifiedHeader.
type SimplifiedBody struct {
	GeneralData   *SimplifiedGeneralData     `xml:"DatiGenerali"`
	GoodsServices []*SimplifiedGoodsServices `xml:"DatiBeniServizi"`
}

// SimplifiedGeneralData contains the document data and, for notes, the
// invoice being corrected.
type SimplifiedGeneralData struct {
	Document  *SimplifiedDocumentData `xml:"DatiGeneraliDocumento"`
	Corrected *CorrectedInvoice       `xml:"DatiFatturaRettificata,omitempty"`
}

// SimplifiedDocumentData contains the document type, currency, date and number.
type SimplifiedDocumentData struct {
	DocumentType string `xml:"TipoDocumento"`
	Currency     string `xml:"Divisa"`
	IssueDate    string `xml:"Data"`
	Number       string `xml:"Numero"`
	VirtualStamp string `xml:"BolloVirtuale,omitempty"`
}

// CorrectedInvoice identifies the invoice a simplified credit or debit note
// corrects, and what it corrects.
type CorrectedInvoice struct {
	Number    string `xml:"NumeroFR"`
	IssueDate string `xml:"DataFR"`
	Details   string `xml:"ElementiRettificati"`
}

// SimplifiedGoodsServices describes a good or service and its amount,
// including VAT.
type SimplifiedGoodsServices struct {
	Description    string         `xml:"Descrizione"`
	Amount         string         `xml:"Importo"`
	VAT            *SimplifiedVAT `xml:"DatiIVA"`
	TaxNature      string         `xml:"Natura,omitempty"`
	LegalReference string         `xml:"RiferimentoNormativo,omitempty"`
}

// SimplifiedVAT states the VAT included in an amount, as the tax itself or as
// its rate.
type SimplifiedVAT struct {
	Amount string `xml:"Imposta,omitempty"`
	Rate   string `xml:"Aliquota,omitempty"`
}

// simplifiedEntry keeps an entry's VAT combo and amount until the amounts are
// rounded.
type simplifiedEntry struct {
	goodsServices *SimplifiedGoodsServices
	vat           *tax.Combo
	amount        num.Amount
}

func newSimplifiedBody(inv *bill.Invoice) (*SimplifiedBody, error) {
	documentType, err := findCodeDocumentType(inv)
	if err != nil {
		return nil, err
	}
	switch documentType {
	case "TD07", "TD08", "TD09":
	default:
		return nil, fmt.Errorf("document type %s is not valid in the %s format", documentType, formatoTrasmissioneFSM10)
	}

	doc := &SimplifiedDocumentData{
		DocumentType: documentType,
		Currency:     string(inv.Currency),
		IssueDate:    inv.IssueDate.String(),
		Number:       invoiceNumber(inv),
	}
	for _, charge := range inv.Charges {
		if charge.Key.Has(bill.ChargeKeyStampDuty) {
			doc.VirtualStamp = flagSI
		}
	}

	// Only credit and debit notes identify the invoice they correct, and the
	// format requires its number, date and reason.
	gd := &SimplifiedGeneralData{Document: doc}
	if documentType != "TD07" {
		if len(inv.Preceding) == 0 || !correctedInvoiceComplete(inv.Preceding[0]) {
			return nil, fmt.Errorf("document type %s requires the corrected invoice with its code, issue date and reason", documentType)
		}
		gd.Corrected = newCorrectedInvoice(inv.Preceding[0])
	}

	return &SimplifiedBody{
		GeneralData:   gd,
		GoodsServices: newSimplifiedGoodsServices(inv),
	}, nil
}

func correctedInvoiceComplete(ref *org.DocumentRef) bool {
	return ref != nil && ref.Code != "" && ref.IssueDate != nil && ref.Reason != ""
}

func newCorrectedInvoice(ref *org.DocumentRef) *CorrectedInvoice {
	ci := &CorrectedInvoice{
		Number:  ref.Series.Join(ref.Code).String(),
		Details: ref.Reason,
	}
	if ref.IssueDate != nil {
		ci.IssueDate = ref.IssueDate.String()
	}
	return ci
}

// newSimplifiedGoodsServices lists one entry per line, charge and discount.
// The format has no totals, so the entries must add up to the invoice's total
// with tax: the entries of each VAT rate are matched to the rate's base plus
// tax, and the last entry takes the cents the separate roundings leave over.
func newSimplifiedGoodsServices(inv *bill.Invoice) []*SimplifiedGoodsServices {
	var entries []*simplifiedEntry
	for _, line := range inv.Lines {
		if line.Item == nil || line.Item.Price == nil {
			continue
		}
		entries = append(entries, newSimplifiedEntry(inv, line.Item.Name, *line.Total, line.Taxes))
	}
	for _, charge := range inv.Charges {
		entries = append(entries, newSimplifiedEntry(inv, chargeDescription(charge), charge.Amount, charge.Taxes))
	}
	for _, discount := range inv.Discounts {
		entries = append(entries, newSimplifiedEntry(inv, discountDescription(discount), discount.Amount.Negate(), discount.Taxes))
	}
	if len(entries) == 0 {
		return nil
	}

	for _, e := range entries {
		e.amount = e.amount.Rescale(2)
	}
	if inv.Totals.Taxes != nil {
		if vat := inv.Totals.Taxes.Category(tax.CategoryVAT); vat != nil {
			for _, rt := range vat.Rates {
				matchRateTotal(entries, rt)
			}
		}
	}
	rest := inv.Totals.TotalWithTax
	for _, e := range entries {
		rest = rest.Subtract(e.amount)
	}
	last := entries[len(entries)-1]
	last.amount = last.amount.Add(rest)

	list := make([]*SimplifiedGoodsServices, len(entries))
	for i, e := range entries {
		e.goodsServices.Amount = formatAmount2(&e.amount)
		list[i] = e.goodsServices
	}
	return list
}

// newSimplifiedEntry prepares an entry from an amount that includes VAT when
// the invoice's prices do, and excludes it otherwise.
func newSimplifiedEntry(inv *bill.Invoice, description string, amount num.Amount, taxes tax.Set) *simplifiedEntry {
	e := &simplifiedEntry{
		goodsServices: &SimplifiedGoodsServices{
			Description: description,
			VAT:         new(SimplifiedVAT),
		},
		vat:    taxes.Get(tax.CategoryVAT),
		amount: amount,
	}
	if e.vat == nil {
		return e
	}

	e.goodsServices.VAT.Rate = formatPercentageWithZero(e.vat.Percent)
	e.goodsServices.TaxNature = exemptExtensionCode(e.vat.Ext)
	e.goodsServices.LegalReference = findRiferimentoNormativo(&tax.RateTotal{Ext: e.vat.Ext})

	pricesIncludeVAT := inv.Tax != nil && inv.Tax.PricesInclude == tax.CategoryVAT
	if e.vat.Percent != nil && !pricesIncludeVAT {
		a := amount.RescaleUp(4)
		e.amount = a.Add(e.vat.Percent.Of(a))
	}
	return e
}

// matchRateTotal adjusts the last of the rate's entries so they add up to the
// rate's base plus tax.
func matchRateTotal(entries []*simplifiedEntry, rt *tax.RateTotal) {
	rest := rt.Base.Add(rt.Amount)
	var last *simplifiedEntry
	for _, e := range entries {
		if e.vat != nil && rt.Matches(rateTotalOf(e.vat)) {
			rest = rest.Subtract(e.amount)
			last = e
		}
	}
	if last != nil {
		last.amount = last.amount.Add(rest)
	}
}

func rateTotalOf(c *tax.Combo) *tax.RateTotal {
	rt := &tax.RateTotal{
		Country: c.Country,
		Ext:     c.Ext,
		Percent: c.Percent,
	}
	if c.Surcharge != nil {
		rt.Surcharge = &tax.RateTotalSurcharge{Percent: *c.Surcharge}
	}
	return rt
}

func chargeDescription(charge *bill.Charge) string {
	switch {
	case charge.Reason != "":
		return charge.Reason
	case charge.Key.Has(bill.ChargeKeyStampDuty):
		return causaleBollo
	}
	return descriptionCharge
}

func discountDescription(discount *bill.Discount) string {
	if discount.Reason != "" {
		return discount.Reason
	}
	return descriptionDiscount
}
