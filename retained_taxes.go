package fatturapa

import (
	"fmt"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
)

// itRegime provides the category definitions that map retained taxes to their
// TipoRitenuta codes.
var itRegime = tax.RegimeDefFor(it.CountryCode)

// RetainedTax represents a retained tax.
type RetainedTax struct {
	Type   string `xml:"TipoRitenuta"`
	Amount string `xml:"ImportoRitenuta"`
	Rate   string `xml:"AliquotaRitenuta"`
	Reason string `xml:"CausalePagamento"`
}

func extractRetainedTaxes(inv *bill.Invoice) ([]*RetainedTax, error) {
	catTotals := findRetainedCategories(inv.Totals)
	var dr []*RetainedTax

	for _, catTotal := range catTotals {
		for _, rateTotal := range catTotal.Rates {
			drElem, err := newRetainedTax(catTotal.Code, rateTotal)
			if err != nil {
				return nil, err
			}
			dr = append(dr, drElem)
		}
	}

	return dr, nil
}

func findRetainedCategories(totals *bill.Totals) []*tax.CategoryTotal {
	var catTotals []*tax.CategoryTotal

	for _, catTotal := range totals.Taxes.Categories {
		if catTotal.Retained {
			catTotals = append(catTotals, catTotal)
		}
	}

	return catTotals
}

func newRetainedTax(cat cbc.Code, rateTotal *tax.RateTotal) (*RetainedTax, error) {
	rate := formatPercentage(rateTotal.Percent)
	amount := formatAmount2(&rateTotal.Amount)

	codeTR, err := findCodeTaxType(cat)
	if err != nil {
		return nil, err
	}

	return &RetainedTax{
		Type:   codeTR,
		Amount: amount,
		Rate:   rate,
		Reason: retainedExtensionCode(rateTotal.Ext),
	}, nil
}

func retainedExtensionCode(ext tax.Extensions) string {
	if v := ext.Get(sdi.ExtKeyRetained); v != "" {
		return v.String()
	}
	if v := ext.Get("it-sdi-retained-tax"); v != "" { // old key
		return v.String()
	}
	return ""
}

func findCodeTaxType(cat cbc.Code) (string, error) {
	if def := itRegime.CategoryDef(cat); def != nil {
		if code := def.Map[it.KeyFatturaPATipoRitenuta]; code != cbc.CodeEmpty {
			return code.String(), nil
		}
	}
	return "", fmt.Errorf("could not find TipoRitenuta code for tax category %s", cat)
}
