package sdi

import (
	"fmt"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// partyHasTaxIDCode is reused by the invoice guards below.
var partyHasTaxIDCode = org.PartyHasTaxIDCode()

func normalizeInvoice(inv *bill.Invoice) {
	normalizeSupplier(inv.Supplier)
	normalizeIssuerType(inv)
}

// normalizeIssuerType sets TZ when the invoice names a third-party issuer and
// no issuer type is set.
func normalizeIssuerType(inv *bill.Invoice) {
	if inv.Tax.GetExt(ExtKeyIssuerType) != "" || inv.Ordering == nil || inv.Ordering.Issuer == nil {
		return
	}
	inv.Tax = inv.Tax.MergeExtensions(tax.ExtensionsOf(cbc.CodeMap{
		ExtKeyIssuerType: ExtCodeIssuerTypeThirdParty,
	}))
}

func normalizeSupplier(party *org.Party) {
	if party == nil {
		return
	}
	if party.Ext.Get(ExtKeyFiscalRegime) == "" {
		// Ordinary regime is default
		party.Ext = party.Ext.Set(ExtKeyFiscalRegime, "RF01")
	}

	// Normalize Italian supplier telephone numbers by stripping '+39' prefix
	if isItalianParty(party) && len(party.Telephones) > 0 {
		for _, tel := range party.Telephones {
			if tel != nil && len(tel.Number) >= 3 && tel.Number[:3] == "+39" {
				tel.Number = tel.Number[3:]
			}
		}
	}
}

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.Assert("22", "invoice must be in EUR or provide exchange rate for conversion", currency.CanConvertTo(currency.EUR)),
		rules.Field("tax",
			rules.Assert("01", "tax is required", is.Present),
			rules.Field("ext",
				rules.Assert("02",
					fmt.Sprintf("tax requires '%s' and '%s' extensions", ExtKeyDocumentType, ExtKeyFormat),
					tax.ExtensionsRequire(
						ExtKeyFormat,
						ExtKeyDocumentType,
					),
				),
			),
		),
		rules.Field("supplier",
			rules.Field("name",
				rules.Assert("03", "supplier name must use Latin-1 characters",
					is.FuncError("latin1", validateLatin1String),
				),
			),
			rules.Field("addresses",
				rules.Assert("04", "supplier addresses are required", is.Present),
			),
			rules.Field("ext",
				rules.Assert("05",
					fmt.Sprintf("supplier requires '%s' extension", ExtKeyFiscalRegime),
					tax.ExtensionsRequire(ExtKeyFiscalRegime),
				),
			),
			rules.Field("registration",
				rules.Field("entry",
					rules.Assert("06", "supplier registration entry is required when registration is present",
						is.Present,
					),
				),
				rules.Field("office",
					rules.Assert("07", "supplier registration office is required when registration is present",
						is.Present,
					),
				),
			),
		),
		rules.When(is.Func("supplier is Italian", invoiceSupplierIsItalian),
			rules.Field("supplier",
				rules.Field("telephones",
					rules.Each(
						rules.Field("num",
							rules.Assert("08", "Italian telephone number length must be between 5 and 12",
								is.Length(5, 12),
							),
						),
					),
				),
			),
		),
		rules.Field("customer",
			rules.Assert("09", "customer is required", is.Present),
			rules.Field("name",
				rules.Assert("10", "customer name must use Latin-1 characters",
					is.FuncError("latin1", validateLatin1String),
				),
			),
			rules.Field("tax_id",
				rules.Assert("11", "customer tax ID is required", is.Present),
			),
		),
		// A simplified invoice may identify the customer by tax code alone
		rules.When(is.Not(is.Func("simplified invoice", invoiceIsSimplified)),
			rules.Field("customer",
				rules.Field("addresses",
					rules.Assert("12", "customer addresses are required", is.Present),
				),
			),
			// Customer name required when tax_id code is present or people is nil
			rules.Assert("13", "customer name is required",
				is.Func("customer name check", invoiceCustomerHasNameOrPeople),
			),
			// Customer people required when name is empty
			rules.Assert("14", "customer people are required when name is empty",
				is.Func("customer people check", invoiceCustomerHasPeopleOrName),
			),
		),
		// Customer tax_id code required for Italian parties without fiscal code
		rules.When(is.Func("Italian customer without fiscal code", invoiceCustomerIsItalianWithoutFiscalCode),
			rules.Field("customer",
				rules.Field("tax_id",
					rules.Field("code",
						rules.Assert("15", "customer tax ID code is required for Italian parties without fiscal code",
							is.Present,
						),
					),
				),
			),
		),
		// Customer identity required for Italian parties without tax ID code
		rules.When(is.Func("Italian customer without tax ID code", invoiceCustomerIsItalianWithoutTaxIDCode),
			rules.Field("customer",
				rules.Field("identities",
					rules.Assert("16",
						fmt.Sprintf("customer requires identity with key '%s'", it.IdentityKeyFiscalCode),
						is.Func("has fiscal code", invoiceCustomerHasFiscalCodeIdentity),
					),
				),
			),
		),
		rules.Field("lines",
			rules.Each(
				rules.Assert("17", "line must have VAT tax category",
					bill.RequireLineTaxCategory(tax.CategoryVAT),
				),
				rules.Field("item",
					rules.Field("name",
						rules.Assert("18", "item name must use Latin-1 characters",
							is.FuncError("latin1", validateLatin1String),
						),
					),
				),
			),
		),
		// Ordering despatch validation
		rules.When(is.Func("has deferred tag", invoiceHasDeferredTag),
			rules.Field("ordering",
				rules.Field("despatch",
					rules.Each(
						rules.Field("issue_date",
							rules.Assert("19", "despatch issue date is required", is.Present),
						),
					),
				),
			),
		),
		rules.When(is.Func("no deferred tag", invoiceDoesNotHaveDeferredTag),
			rules.Field("ordering",
				rules.Field("despatch",
					rules.Assert("20", "despatch can only be set when invoice has deferred tag", is.Empty),
				),
			),
		),
		// Payment: instructions required when terms have due dates
		rules.Assert("21", "payment instructions are required when terms with due dates are present",
			is.Func("payment instructions check", invoicePaymentInstructionsPresent),
		),
		rules.Field("tax",
			rules.Field("ext",
				rules.Assert("23",
					fmt.Sprintf("'%s' FSM10 goes with document types TD07, TD08 and TD09 only", ExtKeyFormat),
					is.Func("simplified format matches document type", simplifiedFormatMatchesDocumentType),
				),
			),
		),
		simplifiedInvoiceRules(),
		// Issuer: a third party issuing the invoice must be identified and named
		rules.Assert("44", "issuer tax ID code or fiscal code is required",
			is.Func("issuer identification check", invoiceIssuerHasTaxIDCodeOrFiscalCode),
		),
		rules.Field("ordering",
			rules.Field("issuer",
				rules.Field("name",
					rules.Assert("45", "issuer name must use Latin-1 characters",
						is.FuncError("latin1", validateLatin1String),
					),
				),
				// The issuer's code is written as given, so it has to fit IdCodice
				rules.Field("tax_id",
					rules.Field("code",
						rules.Assert("64", "issuer tax ID code must be at most 28 characters",
							is.RuneLength(0, 28),
						),
					),
				),
				rules.Assert("53", "issuer person needs a given name and a surname when the issuer has no name",
					is.Func("person full name", partyPersonHasFullName),
				),
				rules.Assert("54", "issuer person name must use Latin-1 characters",
					is.Func("person latin1", partyPersonNameIsLatin1),
				),
				rules.Assert("55", "issuer person title must be 2 to 10 printable ASCII characters",
					is.Func("person title", partyPersonTitleFits),
				),
				rules.Assert("56", "issuer name must be at most 80 characters",
					is.Func("name length", partyNameFits),
				),
				rules.Assert("57", "issuer person given name and surname must be at most 60 characters each",
					is.Func("person name length", partyPersonNameFits),
				),
			),
		),
		rules.Assert("46", "issuer name or people is required",
			is.Func("issuer name check", invoiceIssuerHasNameOrPeople),
		),
		rules.Field("tax",
			rules.Field("ext",
				rules.Assert("47",
					fmt.Sprintf("tax extension '%s' must have a valid code", ExtKeyIssuerType),
					tax.ExtensionHasValidCode(ExtKeyIssuerType),
				),
			),
		),
		// FatturaPA keeps the third-party block for a third party acting for the
		// supplier; the simplified format has no such block.
		rules.When(is.Not(is.Func("simplified invoice", invoiceIsSimplified)),
			rules.Assert("48", "issuer is required when the issuer type is TZ",
				is.Func("third-party issuer named", invoiceThirdPartyIssuerIsNamed),
			),
		),
		rules.Assert("49", "ordering issuer must be removed: the customer issued this invoice (issuer type CC)",
			is.Func("customer issuer names no third party", invoiceCustomerIssuerNamesNoThirdParty),
		),
		rules.Assert("63", "ordering issuer must not be the customer: remove it and set the issuer type to CC",
			is.Func("ordering issuer is not the customer", invoiceIssuerIsNotCustomer),
		),
	)
}

func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.When(is.Func("is fund contribution", chargeIsFundContribution),
			rules.Field("percent",
				rules.Assert("01", "fund contribution charge requires a percentage", is.Present),
			),
			rules.Field("ext",
				rules.Assert("02",
					fmt.Sprintf("fund contribution charge requires '%s' extension", ExtKeyFundType),
					tax.ExtensionsRequire(ExtKeyFundType),
				),
			),
			rules.Field("taxes",
				rules.Assert("03", "fund contribution charge must have VAT tax category",
					tax.SetHasCategory(tax.CategoryVAT),
				),
			),
		),
	)
}

// --- Helper functions for rules ---

func invoiceSupplierIsItalian(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	return isItalianParty(inv.Supplier)
}

func invoiceCustomerHasNameOrPeople(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Customer == nil {
		return true
	}
	c := inv.Customer
	// Name required when tax_id code is present or people is nil
	if (c.TaxID != nil && c.TaxID.Code != cbc.CodeEmpty) || c.People == nil {
		return c.Name != ""
	}
	return true
}

func invoiceCustomerHasPeopleOrName(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Customer == nil {
		return true
	}
	c := inv.Customer
	if c.Name == "" {
		return len(c.People) > 0
	}
	return true
}

func invoiceCustomerIsItalianWithoutFiscalCode(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Customer == nil {
		return false
	}
	return isItalianParty(inv.Customer) && !hasFiscalCode(inv.Customer)
}

func invoiceCustomerIsItalianWithoutTaxIDCode(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Customer == nil {
		return false
	}
	return isItalianParty(inv.Customer) && !partyHasTaxIDCode.Check(inv.Customer)
}

func invoiceCustomerHasFiscalCodeIdentity(val any) bool {
	ids, ok := val.([]*org.Identity)
	if !ok {
		return false
	}
	return org.IdentityForKey(ids, it.IdentityKeyFiscalCode) != nil
}

func invoiceIssuerHasTaxIDCodeOrFiscalCode(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Ordering == nil || inv.Ordering.Issuer == nil {
		return true
	}
	return partyHasTaxIDCode.Check(inv.Ordering.Issuer) || hasFiscalCode(inv.Ordering.Issuer)
}

// titlePattern matches the FatturaPA TitoloType after the schema collapses
// whitespace: 2 to 10 printable ASCII characters, none of them a leading or
// trailing space.
var titlePattern = regexp.MustCompile(`^[\x21-\x7E][\x20-\x7E]{0,8}[\x21-\x7E]$`)

// anagraficaPersonName returns the name the conversion writes for a party with
// no name of its own: its first person's.
func anagraficaPersonName(val any) *org.Name {
	p, ok := val.(*org.Party)
	if !ok || p == nil || p.Name != "" || len(p.People) == 0 {
		return nil
	}
	return p.People[0].Name
}

func partyPersonHasFullName(val any) bool {
	n := anagraficaPersonName(val)
	return n == nil || (n.Given != "" && n.Surname != "")
}

func partyPersonNameIsLatin1(val any) bool {
	n := anagraficaPersonName(val)
	return n == nil || (validateLatin1String(n.Given) == nil && validateLatin1String(n.Surname) == nil)
}

func partyPersonTitleFits(val any) bool {
	n := anagraficaPersonName(val)
	return n == nil || n.Prefix == "" || titlePattern.MatchString(n.Prefix)
}

func partyPersonNameFits(val any) bool {
	n := anagraficaPersonName(val)
	return n == nil || (utf8.RuneCountInString(n.Given) <= 60 && utf8.RuneCountInString(n.Surname) <= 60)
}

// partyNameFits checks the name written as Denominazione, which takes up to 80
// characters.
func partyNameFits(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil || anagraficaPersonName(p) != nil {
		return true
	}
	return utf8.RuneCountInString(p.Name) <= 80
}

func invoiceIssuerHasNameOrPeople(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Ordering == nil || inv.Ordering.Issuer == nil {
		return true
	}
	return inv.Ordering.Issuer.Name != "" || len(inv.Ordering.Issuer.People) > 0
}

func invoiceThirdPartyIssuerIsNamed(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Tax.GetExt(ExtKeyIssuerType) != ExtCodeIssuerTypeThirdParty {
		return true
	}
	return inv.Ordering != nil && inv.Ordering.Issuer != nil
}

func invoiceCustomerIssuerNamesNoThirdParty(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Tax.GetExt(ExtKeyIssuerType) != ExtCodeIssuerTypeCustomer {
		return true
	}
	return inv.Ordering == nil || inv.Ordering.Issuer == nil
}

func invoiceIssuerIsNotCustomer(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Ordering == nil || inv.Ordering.Issuer == nil || inv.Customer == nil {
		return true
	}
	customerIDs := partyFiscalIDs(inv.Customer)
	for _, id := range partyFiscalIDs(inv.Ordering.Issuer) {
		if slices.Contains(customerIDs, id) {
			return false
		}
	}
	return true
}

func invoiceHasDeferredTag(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	return inv.HasTags(TagDeferred)
}

func invoiceDoesNotHaveDeferredTag(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	return !inv.HasTags(TagDeferred)
}

func invoicePaymentInstructionsPresent(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Payment == nil {
		return true
	}
	p := inv.Payment
	if p.Terms != nil && len(p.Terms.DueDates) > 0 {
		return p.Instructions != nil
	}
	return true
}

func chargeIsFundContribution(val any) bool {
	c, ok := val.(*bill.Charge)
	if !ok || c == nil {
		return false
	}
	return c.Key.Has(KeyFundContribution)
}

func hasFiscalCode(party *org.Party) bool {
	if party == nil {
		return false
	}
	return org.IdentityForKey(party.Identities, it.IdentityKeyFiscalCode) != nil

}

func isItalianParty(party *org.Party) bool {
	if party == nil || party.TaxID == nil {
		return false
	}
	return party.TaxID.Country.In("IT")
}
