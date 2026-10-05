package sdi

import (
	"slices"
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// simplifiedDocumentTypes are the document types of the FSM10 format.
var simplifiedDocumentTypes = []cbc.Code{"TD07", "TD08", "TD09"}

// simplifiedExemptCodes are the Natura codes of the FSM10 schema, except N3.2:
// art. 21-bis c.2 DPR 633/72 excludes intra-EU supplies from simplified invoices.
var simplifiedExemptCodes = []cbc.Code{
	"N1", "N2.1", "N2.2", "N3.1", "N3.3", "N3.4", "N3.5", "N3.6", "N4", "N5",
}

// simplifiedStampDutyExemptions are the AltriDatiGestionali codes that exempt a
// document from stamp duty.
var simplifiedStampDutyExemptions = []string{"NB1", "NB2", "NB3"}

// simplifiedExcludedTags pick a document type of the ordinary format.
var simplifiedExcludedTags = []cbc.Key{
	TagFreelance,
	tax.TagPartial,
	tax.TagSelfBilled,
	tax.TagReverseCharge,
	TagCeilingExceeded,
	TagSanMarinoPaper,
	TagImport,
	TagGoods,
	TagGoodsEU,
	TagGoodsWithTax,
	TagGoodsExtracted,
	TagRegularization,
	TagDeferred,
	TagThirdPeriod,
	TagDepreciableAssets,
}

// simplifiedLimit is the most a simplified invoice may total, in EUR.
var simplifiedLimit = num.MakeAmount(40000, 2)

// simplifiedInvoiceRules keep a simplified invoice within what the FSM10
// format can carry and what art. 21-bis DPR 633/72 allows.
func simplifiedInvoiceRules() rules.Def {
	return rules.When(is.Func("simplified invoice", invoiceIsSimplified),
		rules.Assert("24", "simplified invoice cannot be issued to a public administration",
			is.Func("not b2g", invoiceIsNotB2G),
		),
		rules.Assert("25", "simplified invoice cannot use another document type tag",
			is.Func("no ordinary document type tag", invoiceHasNoOrdinaryDocumentTypeTag),
		),
		rules.Assert("43", "simplified invoice document type must match its type: TD07 for a standard invoice, TD08 for a credit note, TD09 for a debit note",
			is.Func("simplified document type matches invoice type", invoiceSimplifiedTypeMatches),
		),
		rules.Assert("26", "simplified invoice total cannot exceed 400.00 EUR, unless the supplier is in regime RF19 or RF20 or it is a credit or debit note correcting a preceding invoice",
			is.Func("within the simplified limit", invoiceWithinSimplifiedLimit),
		),
		rules.Assert("27", "simplified invoice VAT exemption must be N1, N2.1, N2.2, N3.1, N3.3, N3.4, N3.5, N3.6, N4 or N5",
			is.Func("simplified exemption codes", invoiceHasSimplifiedExemptions),
		),
		rules.Assert("28", "simplified invoice cannot cover operations taxed on a business in another EU country (N2.1)",
			is.Func("no N2.1 to an EU business", invoiceHasNoOutsideScopeToEUBusiness),
		),
		rules.Assert("41", "simplified invoice cannot declare a stamp duty exemption (NB1, NB2 or NB3), which only the ordinary format carries",
			is.Func("no stamp duty exemption", invoiceHasNoStampDutyExemption),
		),
		rules.Assert("42", "simplified invoice cannot cover a sale to a habitual exporter (N3.5), whose declaration of intent only the ordinary format carries",
			is.Func("no sale to a habitual exporter", invoiceHasNoHabitualExporterSale),
		),
		rules.Assert("29", "simplified invoice cannot carry retained taxes",
			is.Func("no retained taxes", invoiceHasNoRetainedTaxes),
		),
		rules.Field("charges",
			rules.Each(
				rules.Assert("30", "simplified invoice cannot carry fund contributions",
					is.Not(is.Func("is fund contribution", chargeIsFundContribution)),
				),
				rules.Field("taxes",
					rules.Assert("31", "simplified invoice charge requires a VAT tax combo",
						tax.SetHasCategory(tax.CategoryVAT),
					),
				),
			),
		),
		rules.Field("discounts",
			rules.Each(
				rules.Field("taxes",
					rules.Assert("32", "simplified invoice discount requires a VAT tax combo",
						tax.SetHasCategory(tax.CategoryVAT),
					),
				),
			),
		),
		rules.Field("totals",
			rules.Field("rounding",
				rules.Assert("33", "simplified invoice cannot carry a rounding amount",
					is.Func("no rounding", amountIsZero),
				),
			),
		),
		// Only credit and debit notes carry the corrected invoice (DatiFatturaRettificata),
		// which art. 21-bis c.1 h DPR 633/72 requires them to identify.
		rules.When(is.Func("simplified note", invoiceIsNote),
			rules.Field("preceding",
				rules.Assert("40", "simplified credit or debit note requires the preceding invoice it corrects",
					is.Present,
				),
				rules.Assert("34", "simplified invoice can refer to one preceding invoice only",
					is.Length(0, 1),
				),
				rules.Each(
					rules.Field("issue_date",
						rules.Assert("35", "simplified invoice preceding issue date is required", is.Present),
					),
					rules.Field("reason",
						rules.Assert("36", "simplified invoice preceding reason is required", is.Present),
					),
				),
			),
			rules.Assert("37", "simplified invoice cannot be issued before the preceding invoice",
				is.Func("issued after the preceding invoice", invoiceIssuedAfterPreceding),
			),
		),
		rules.Assert("38", "simplified invoice supplier and customer must be different parties",
			is.Func("supplier is not the customer", invoiceSupplierIsNotCustomer),
		),
		rules.Assert("39", "simplified invoice supplier and customer cannot both be outside Italy",
			is.Func("a party in Italy", invoiceHasPartyInItaly),
		),
	)
}

func invoiceIsSimplified(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	if inv.HasTags(tax.TagSimplified) {
		return true
	}
	return inv.Tax != nil && inv.Tax.Ext.Get(ExtKeyFormat) == "FSM10"
}

// simplifiedFormatMatchesDocumentType passes when FSM10 and its document types
// appear together or not at all.
func simplifiedFormatMatchesDocumentType(val any) bool {
	ext, ok := tax.ExtensionsFromValue(val)
	if !ok || !ext.Has(ExtKeyFormat, ExtKeyDocumentType) {
		return true
	}
	simplifiedFormat := ext.Get(ExtKeyFormat) == "FSM10"
	simplifiedType := ext.Get(ExtKeyDocumentType).In(simplifiedDocumentTypes...)
	return simplifiedFormat == simplifiedType
}

// simplifiedInvoiceTypes are the invoice types of the FSM10 document types.
var simplifiedInvoiceTypes = map[cbc.Code]cbc.Key{
	"TD07": bill.InvoiceTypeStandard,
	"TD08": bill.InvoiceTypeCreditNote,
	"TD09": bill.InvoiceTypeDebitNote,
}

func invoiceSimplifiedTypeMatches(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Tax == nil {
		return true
	}
	typ, ok := simplifiedInvoiceTypes[inv.Tax.Ext.Get(ExtKeyDocumentType)]
	return !ok || inv.Type == typ
}

func invoiceIsNote(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	return inv.Type.In(bill.InvoiceTypeCreditNote, bill.InvoiceTypeDebitNote)
}

func invoiceIsNotB2G(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	return !inv.HasTags(tax.TagB2G)
}

func invoiceHasNoOrdinaryDocumentTypeTag(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, tag := range simplifiedExcludedTags {
		if inv.HasTags(tag) {
			return false
		}
	}
	return true
}

// invoiceWithinSimplifiedLimit applies SDI check 00460: the total may not
// exceed 400.00 EUR, except for suppliers in the flat rate (RF19) or
// cross-border franchise (RF20) regimes, and for credit and debit notes that
// correct a preceding invoice.
func invoiceWithinSimplifiedLimit(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Totals == nil {
		return true
	}
	if invoiceIsNote(inv) && len(inv.Preceding) > 0 {
		return true
	}
	if inv.Supplier != nil && inv.Supplier.Ext.Get(ExtKeyFiscalRegime).In("RF19", "RF20") {
		return true
	}
	total := currency.Convert(inv.ExchangeRates, inv.Currency, currency.EUR, inv.Totals.TotalWithTax)
	if total == nil {
		return true // rule 22 reports the missing exchange rate
	}
	return total.Rescale(2).Compare(simplifiedLimit) <= 0
}

func invoiceHasSimplifiedExemptions(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, c := range invoiceTaxCombos(inv) {
		if c.Category != tax.CategoryVAT {
			continue
		}
		if code := c.Ext.Get(ExtKeyExempt); code != "" && !code.In(simplifiedExemptCodes...) {
			return false
		}
	}
	return true
}

// invoiceHasNoOutsideScopeToEUBusiness applies art. 21-bis c.2 DPR 633/72,
// which excludes the operations of art. 21 c.6-bis(a): those taxed on a
// business customer in another EU country.
func invoiceHasNoOutsideScopeToEUBusiness(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Customer == nil || inv.Customer.TaxID == nil {
		return true
	}
	id := inv.Customer.TaxID
	if id.Code == cbc.CodeEmpty || id.Country.In("IT") || !l10n.Union(l10n.EU).HasMember(id.Country.Code()) {
		return true
	}
	for _, c := range invoiceTaxCombos(inv) {
		if c.Category == tax.CategoryVAT && c.Ext.Get(ExtKeyExempt) == "N2.1" {
			return false
		}
	}
	return true
}

// invoiceHasNoStampDutyExemption rejects the AltriDatiGestionali codes NB1, NB2
// and NB3, which the stamp duty guide requires to be sent in an ordinary invoice.
func invoiceHasNoStampDutyExemption(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, line := range inv.Lines {
		if line == nil || line.Item == nil {
			continue
		}
		for _, a := range line.Item.Attributes {
			if a == nil {
				continue
			}
			for _, code := range simplifiedStampDutyExemptions {
				if strings.EqualFold(a.Type.String(), code) || strings.EqualFold(a.Key.String(), code) {
					return false
				}
			}
		}
	}
	return true
}

// invoiceHasNoHabitualExporterSale rejects N3.5: the invoice must state the
// protocol of the declaration of intent (art. 1 c.1 lett. c DL 746/1983),
// which SDI takes only in an AltriDatiGestionali block.
func invoiceHasNoHabitualExporterSale(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, c := range invoiceTaxCombos(inv) {
		if c.Category == tax.CategoryVAT && c.Ext.Get(ExtKeyExempt) == "N3.5" {
			return false
		}
	}
	return true
}

func invoiceHasNoRetainedTaxes(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, c := range invoiceTaxCombos(inv) {
		if taxComboIsRetained(c) {
			return false
		}
	}
	return true
}

func amountIsZero(val any) bool {
	switch a := val.(type) {
	case *num.Amount:
		return a == nil || a.IsZero()
	case num.Amount:
		return a.IsZero()
	}
	return true
}

func invoiceIssuedAfterPreceding(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	for _, ref := range inv.Preceding {
		if ref != nil && ref.IssueDate != nil && inv.IssueDate.Before(ref.IssueDate.Date) {
			return false
		}
	}
	return true
}

// invoiceSupplierIsNotCustomer applies SDI check 00471, which forbids the
// same party on both sides of a TD07.
func invoiceSupplierIsNotCustomer(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Tax == nil || inv.Tax.Ext.Get(ExtKeyDocumentType) != "TD07" {
		return true
	}
	if inv.Supplier == nil || inv.Customer == nil {
		return true
	}
	supplierIDs := partyFiscalIDs(inv.Supplier)
	for _, id := range partyFiscalIDs(inv.Customer) {
		if slices.Contains(supplierIDs, id) {
			return false
		}
	}
	return true
}

// partyFiscalIDs lists the VAT ID and fiscal codes that identify a party in the
// FSM10 header. An Italian VAT number also counts as a fiscal code, because a
// company's codice fiscale is its partita IVA.
func partyFiscalIDs(p *org.Party) []string {
	var ids []string
	if p.TaxID != nil && p.TaxID.Code != cbc.CodeEmpty {
		ids = append(ids, "vat:"+p.TaxID.Country.String()+p.TaxID.Code.String())
		if p.TaxID.Country.In("IT") {
			ids = append(ids, "cf:"+p.TaxID.Code.String())
		}
	}
	if id := org.IdentityForKey(p.Identities, it.IdentityKeyFiscalCode); id != nil && id.Code != cbc.CodeEmpty {
		ids = append(ids, "cf:"+id.Code.String())
	}
	return ids
}

// invoiceHasPartyInItaly applies SDI check 00476.
func invoiceHasPartyInItaly(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Supplier == nil || inv.Customer == nil {
		return true
	}
	if inv.Supplier.TaxID == nil || inv.Customer.TaxID == nil {
		return true
	}
	return inv.Supplier.TaxID.Country.In("IT") || inv.Customer.TaxID.Country.In("IT")
}

// invoiceTaxCombos lists the tax combos of the lines, charges and discounts.
func invoiceTaxCombos(inv *bill.Invoice) []*tax.Combo {
	var combos []*tax.Combo
	for _, line := range inv.Lines {
		if line != nil {
			combos = append(combos, line.Taxes...)
		}
	}
	for _, charge := range inv.Charges {
		if charge != nil {
			combos = append(combos, charge.Taxes...)
		}
	}
	for _, discount := range inv.Discounts {
		if discount != nil {
			combos = append(combos, discount.Taxes...)
		}
	}
	return combos
}
