package fatturapa

import (
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

func goblBillInvoiceAddHeader(inv *bill.Invoice, header *Header) {
	if inv == nil || header == nil {
		return
	}

	inv.Supplier = goblOrgPartyFromSupplier(header.Supplier)
	inv.Customer = goblOrgPartyFromCustomer(header.Customer)

	goblBillInvoiceAddIssuer(inv, header)

	// Need to do after customer is set
	goblBillInvoiceAddTransmission(inv, header.TransmissionData)
}

func goblBillInvoiceAddIssuer(inv *bill.Invoice, header *Header) {
	if header.IssuerType != "" {
		inv.Tax = inv.Tax.MergeExtensions(tax.ExtensionsOf(cbc.CodeMap{
			sdi.ExtKeyIssuerType: cbc.Code(header.IssuerType),
		}))
	}

	if header.ThirdPartyIssuer == nil {
		return
	}

	issuer := new(org.Party)
	goblOrgPartyAddIdentity(issuer, header.ThirdPartyIssuer.Identity)

	if inv.Ordering == nil {
		inv.Ordering = new(bill.Ordering)
	}
	inv.Ordering.Issuer = issuer
}
