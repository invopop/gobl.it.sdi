package sdi

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// string20Max is the length of FatturaPA's String20Type, which holds document
// numbers and references as 1 to 20 Basic Latin characters.
const string20Max = 20

// documentNumberRules keep the numbers FatturaPA stores as String20Type within
// that type, as SDI rejects longer ones with error 00200.
func documentNumberRules() rules.Def {
	return rules.Object(
		rules.Assert("44", "invoice number (series and code) must be 20 ASCII characters or fewer",
			is.Func("invoice number fits", invoiceNumberFits),
		),
		rules.Field("preceding",
			rules.Each(
				rules.Assert("45", "preceding document number and item reference must be 20 ASCII characters or fewer",
					is.Func("document reference fits", documentRefFits),
				),
			),
		),
		rules.Field("ordering",
			rules.Assert("46", "ordering document numbers and item references must be 20 ASCII characters or fewer",
				is.Func("ordering references fit", orderingRefsFit),
			),
		),
		rules.Field("supplier",
			rules.Field("registration",
				rules.Field("entry",
					rules.Assert("47", "supplier registration entry must be 20 ASCII characters or fewer",
						is.Func("registration entry fits", fitsString20),
					),
				),
			),
		),
	)
}

func invoiceNumberFits(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	return fitsString20(inv.Series.Join(inv.Code))
}

func documentRefFits(val any) bool {
	ref, ok := val.(*org.DocumentRef)
	if !ok || ref == nil {
		return true
	}
	if !fitsString20(ref.Series.Join(ref.Code)) {
		return false
	}
	// The converter writes the last item identity, so all of them must fit.
	for _, id := range ref.Identities {
		if id != nil && id.Key == org.IdentityKeyItem && !fitsString20(id.Code) {
			return false
		}
	}
	return true
}

func orderingRefsFit(val any) bool {
	o, ok := val.(*bill.Ordering)
	if !ok || o == nil {
		return true
	}
	for _, refs := range [][]*org.DocumentRef{o.Purchases, o.Contracts, o.Tender, o.Receiving} {
		for _, ref := range refs {
			if !documentRefFits(ref) {
				return false
			}
		}
	}
	// A despatch reference (DatiDDT) carries only its number, not an item reference.
	for _, ref := range o.Despatch {
		if ref != nil && !fitsString20(ref.Series.Join(ref.Code)) {
			return false
		}
	}
	return true
}

// fitsString20 passes empty values, which presence rules report.
func fitsString20(val any) bool {
	var s string
	switch v := val.(type) {
	case string:
		s = v
	case cbc.Code:
		s = v.String()
	default:
		return true
	}
	if len(s) > string20Max {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}
