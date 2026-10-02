package fatturapa

import (
	"errors"
	"fmt"

	"github.com/invopop/gobl"
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/xmlctx"
)

// parseSimplified builds a GOBL invoice from a FatturaElettronicaSemplificata
// document, with a line priced with VAT included for each entry.
func parseSimplified(doc []byte) (*gobl.Envelope, error) {
	d := new(SimplifiedInvoice)
	if err := xmlctx.Unmarshal(doc, d, xmlctx.WithNamespaces(map[string]string{
		"p":  namespaceFatturaPASimplified,
		"ds": namespaceDSig,
	})); err != nil {
		return nil, fmt.Errorf("unmarshal document: %w", err)
	}
	if d.Header == nil || len(d.Body) == 0 {
		return nil, errors.New("unmarshal document: missing header or body")
	}

	inv := new(bill.Invoice)
	inv.Addons = tax.WithAddons(sdi.V1)
	inv.Supplier = goblOrgPartyFromSimplifiedSupplier(d.Header.Supplier)
	inv.Customer = goblOrgPartyFromSimplifiedCustomer(d.Header.Customer)
	if d.Header.TransmissionData != nil {
		goblBillInvoiceAddTransmission(inv, d.Header.TransmissionData)
	}

	// TODO: add support for multiple bodies
	if err := goblBillInvoiceAddSimplifiedBody(inv, d.Body[0]); err != nil {
		return nil, err
	}

	return gobl.Envelop(inv)
}

func goblOrgPartyFromSimplifiedSupplier(supplier *SimplifiedSupplier) *org.Party {
	if supplier == nil {
		return nil
	}

	party := new(org.Party)
	goblOrgPartyAddIdentity(party, &Identity{
		TaxID:        supplier.TaxID,
		FiscalCode:   supplier.FiscalCode,
		Profile:      profileFromName(supplier.Name, supplier.Given, supplier.Surname),
		FiscalRegime: supplier.FiscalRegime,
	})
	goblOrgPartyAddRegistration(party, supplier.Registration)
	if supplier.Address != nil {
		party.Addresses = []*org.Address{goblOrgAddressFromAddress(supplier.Address)}
	}

	return party
}

func goblOrgPartyFromSimplifiedCustomer(customer *SimplifiedCustomer) *org.Party {
	if customer == nil {
		return nil
	}

	party := new(org.Party)
	identity := new(Identity)
	if fi := customer.FiscalIdentifiers; fi != nil {
		identity.TaxID = fi.TaxID
		identity.FiscalCode = fi.FiscalCode
	}
	if oi := customer.OtherIdentifiers; oi != nil {
		identity.Profile = profileFromName(oi.Name, oi.Given, oi.Surname)
		if oi.Address != nil {
			party.Addresses = []*org.Address{goblOrgAddressFromAddress(oi.Address)}
		}
	}
	goblOrgPartyAddIdentity(party, identity)

	return party
}

func profileFromName(name, given, surname string) *Profile {
	if name == "" && given == "" && surname == "" {
		return nil
	}
	return &Profile{Name: name, Given: given, Surname: surname}
}

func goblBillInvoiceAddSimplifiedBody(inv *bill.Invoice, body *SimplifiedBody) error {
	if gd := body.GeneralData; gd != nil && gd.Document != nil {
		doc := gd.Document
		goblBillInvoiceAddDocumentType(inv, doc.DocumentType)
		inv.Currency = currency.Code(doc.Currency)
		date, err := parseDate(doc.IssueDate)
		if err != nil {
			return fmt.Errorf("adding issue date: %w", err)
		}
		inv.IssueDate = date
		inv.Code = cbc.Code(doc.Number)

		if gd.Corrected != nil {
			ref, err := goblOrgDocumentRefFromCorrectedInvoice(gd.Corrected)
			if err != nil {
				return fmt.Errorf("adding corrected invoice: %w", err)
			}
			inv.Preceding = []*org.DocumentRef{ref}
		}
	}

	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	inv.Tax.PricesInclude = tax.CategoryVAT

	for _, gs := range body.GoodsServices {
		line, err := goblBillLineFromSimplifiedGoodsServices(gs)
		if err != nil {
			return fmt.Errorf("adding goods and services: %w", err)
		}
		inv.Lines = append(inv.Lines, line)
	}

	return nil
}

func goblOrgDocumentRefFromCorrectedInvoice(ci *CorrectedInvoice) (*org.DocumentRef, error) {
	ref := &org.DocumentRef{
		Code:   cbc.Code(ci.Number),
		Reason: ci.Details,
	}
	if ci.IssueDate != "" {
		date, err := parseDate(ci.IssueDate)
		if err != nil {
			return nil, err
		}
		ref.IssueDate = &date
	}
	return ref, nil
}

func goblBillLineFromSimplifiedGoodsServices(gs *SimplifiedGoodsServices) (*bill.Line, error) {
	amount, err := parseAmount(gs.Amount)
	if err != nil {
		return nil, fmt.Errorf("parsing amount: %w", err)
	}

	combo, err := goblSimplifiedVATCombo(gs, amount)
	if err != nil {
		return nil, err
	}

	return &bill.Line{
		Quantity: num.MakeAmount(1, 0),
		Item: &org.Item{
			Name:  gs.Description,
			Price: &amount,
		},
		Taxes: tax.Set{combo},
	}, nil
}

// goblSimplifiedVATCombo reads an entry's VAT, which the format states as a
// rate, as the tax included in the amount, or as an exemption.
func goblSimplifiedVATCombo(gs *SimplifiedGoodsServices, amount num.Amount) (*tax.Combo, error) {
	var rate string
	if gs.VAT != nil {
		rate = gs.VAT.Rate
		if rate == "" && gs.VAT.Amount != "" && gs.TaxNature == "" {
			vat, err := parseAmount(gs.VAT.Amount)
			if err != nil {
				return nil, fmt.Errorf("parsing VAT amount: %w", err)
			}
			if rate, err = rateFromIncludedVAT(amount, vat); err != nil {
				return nil, fmt.Errorf("entry %q: %w", gs.Description, err)
			}
		}
	}
	if rate == "" && gs.TaxNature == "" {
		return nil, fmt.Errorf("entry %q has no VAT rate, tax or exemption", gs.Description)
	}
	return goblVATCombo(rate, gs.TaxNature)
}

// rateFromIncludedVAT finds the Italian VAT rate that leaves the given VAT in
// the amount, or works the rate out from the two when none does.
func rateFromIncludedVAT(amount, vat num.Amount) (string, error) {
	if vat.IsZero() {
		return "", errors.New("zero VAT without a rate or exemption")
	}
	for _, r := range tax.RegimeDefFor(l10n.IT).CategoryDef(tax.CategoryVAT).Rates {
		for _, v := range r.Values {
			if v.Percent.From(amount).Rescale(2).Equals(vat.Rescale(2)) {
				return formatPercentage(&v.Percent), nil
			}
		}
	}
	net := amount.Subtract(vat)
	if net.IsZero() {
		return "", errors.New("VAT equals the amount")
	}
	rate := vat.RescaleUp(6).Divide(net).Multiply(num.MakeAmount(100, 0))
	return rate.Rescale(2).String(), nil
}
