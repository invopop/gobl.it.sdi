// Package sdi handles the extensions and validation rules in order to use
// GOBL with the Italian SDI and FatturaPA format.
package sdi

import (
	"errors"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	// Key identifies the SDI addon family. Individual versions append a
	// suffix; the family key is used as the fault-code namespace so that
	// rules that carry across versions keep stable codes.
	Key cbc.Key = "it-sdi"

	// V1 for SDI's FatturaPA versions 1.x
	V1 cbc.Key = Key + "-v1"

	// KeyFundContribution is the key for the Fund Contribution charge
	KeyFundContribution cbc.Key = "fund-contribution"
)

func init() {
	tax.RegisterAddonDef(newAddon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add("IT-SDI"),
		is.InContext(tax.AddonIn(V1)),
		billInvoiceRules(),
		billChargeRules(),
		billStatusRules(),
		billStatusLineRules(),
		orgAddressRules(),
		orgAttributeRules(),
		taxComboRules(),
		payInstructionsRules(),
		payAdvanceRules(),
		payDueDateRules(),
	)
	norm.RegisterWithGuard(
		is.InContext(tax.AddonIn(V1)),
		norm.For(normalizeInvoice),
		norm.For(normalizePayInstructions),
		norm.For(normalizePayRecord),
		norm.For(normalizeAddress),
		norm.For(normalizeTaxCombo),
		norm.For(normalizeStatus),
	)
}

func newAddon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V1,
		Name: i18n.String{
			i18n.EN: "Italy SDI FatturaPA v1.x",
		},
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Italy exchanges electronic invoices in the FatturaPA XML format through the tax
				authority's Sistema di Interscambio (SDI). This addon ensures a GOBL document has
				the fields and extensions needed to produce a valid FatturaPA file with
				[gobl.fatturapa](https://github.com/invopop/gobl.it.sdi).

				## Customer identification

				Every customer needs a ~tax_id~, and what you put in it depends on who they are. For
				an Italian business, use their partita IVA (VAT number). An Italian private
				individual has no VAT number, so give the country alone and put their codice
				fiscale in an identity with the key ~it-fiscal-code~. For a customer outside Italy,
				give the country, and their VAT number if they have one.

				In your GOBL document you can declare two kinds of inbox to tell SDI where the
				invoice should be delivered: ~it-sdi-code~ for a codice destinatario, and
				~it-sdi-pec~ for a PEC address, which goes in the inbox's ~email~ field. You can
				declare both. A FatturaPA document supports only one destination field, so the
				conversion prioritises the codice destinatario when it is available, and omits the
				PEC.

				FatturaPA always asks for a recipient code, and for a tax number when the party is
				a business. Some customers have neither, so the format defines a fixed value for
				each case and the conversion writes it for you.

				A codice destinatario names a channel accredited with SDI, so only recipients in
				Italy have one. An Italian customer who receives by PEC, or who has not given you
				a code, gets ~0000000~. A customer outside Italy has none, and SDI cannot deliver
				there anyway, so they get ~XXXXXXX~. The conversion takes it from the country, and
				any inbox on that customer makes no difference.

				The tax number follows the same idea. A party outside Italy with no VAT number
				gets ~0000000~. A business outside the EU has a tax number, but SDI can only check
				EU VAT numbers, so the conversion replaces it with ~OO99999999999~, on suppliers
				as much as customers. An Italian private individual needs no placeholder: their
				codice fiscale identifies them, and the conversion writes no tax number for them.
			`),
		},
		Extensions: extensions,
		Tags: []*tax.TagSet{
			invoiceTags,
		},
		Inboxes:   inboxes,
		Scenarios: scenarios,
	}
}

// validateLatin1String ensures that the item name only contains characters
// from Latin and Latin-1 range (ASCII 0-127 and extended Latin-1 128-255).
func validateLatin1String(val any) error {
	name, _ := val.(string)

	for _, r := range name {
		// Check if the character is outside Latin and Latin-1 range
		// Latin and Latin-1 includes ASCII (0-127) and extended Latin-1 (128-255)
		if r > 255 {
			return errors.New("contains characters outside of Latin and Latin-1 range")
		}
	}
	return nil
}
