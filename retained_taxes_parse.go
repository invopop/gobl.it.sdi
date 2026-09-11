package fatturapa

import (
	"fmt"
	"strings"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
)

// processRetainedTaxes processes retained taxes and adds them to the appropriate line items
// and fund contribution charges
func processRetainedTaxes(inv *bill.Invoice, lineDetails []*LineDetail, retainedTaxes []*RetainedTax, fundContributions []*FundContribution) error {
	if len(retainedTaxes) == 0 || len(inv.Lines) == 0 {
		return nil
	}

	// Collect only fund contribution charges that were marked with Ritenuta=SI
	// in the original XML. We match by fund type code against the invoice charges.
	retainedCharges := retainedFundContributionCharges(inv.Charges, fundContributions)

	// Determine which lines are candidates for retained tax matching.
	// When lines are explicitly flagged with Ritenuta=SI, use only those.
	// When no lines are flagged, all lines are candidates — the flag is
	// optional in the FatturaPA spec and some invoicing software omits it.
	candidates := candidateLinesForRetention(inv.Lines, lineDetails)

	// Process each retained tax
	for _, rt := range retainedTaxes {
		// Parse the retained tax rate and amount
		rtRate, err1 := num.PercentageFromString(strings.TrimSpace(rt.Rate) + "%")
		rtAmount, err2 := parseAmount(rt.Amount)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid retained tax rate or amount: %s %s", rt.Rate, rt.Amount)
		}

		// Convert tax type to category code
		catCode, err := convertRetainedTaxType(rt.Type)
		if err != nil {
			return err
		}

		// Build the tax combo to apply
		taxCombo := &tax.Combo{
			Category: catCode,
			Percent:  &rtRate,
		}
		if rt.Reason != "" {
			taxCombo.Ext = taxCombo.Ext.Set(sdi.ExtKeyRetained, cbc.Code(rt.Reason))
		}

		// Try to match against a single line first (common case)
		matched := false
		for _, line := range candidates {
			if reproduces(rtRate, *line.Total, rtAmount) {
				line.Taxes = append(line.Taxes, taxCombo)
				matched = true
				break
			}
		}

		if matched {
			continue
		}

		// Try matching against the sum of all candidate lines + fund contribution charges
		totalBase := num.MakeAmount(0, 2)
		for _, line := range candidates {
			totalBase = totalBase.MatchPrecision(*line.Total).Add(*line.Total)
		}
		for _, charge := range retainedCharges {
			totalBase = totalBase.MatchPrecision(charge.Amount).Add(charge.Amount)
		}

		rate := &rtRate
		if !reproduces(rtRate, totalBase, rtAmount) {
			// The withholding applies to a fraction of the base (art. 25-bis:
			// 23% on half a commission) and FatturaPA has no element for it,
			// so derive the rate over the whole base.
			rate = effectiveRate(rtAmount, totalBase)
		}
		if rate == nil {
			return fmt.Errorf("could not match retained tax: %s %s%% %s on base %s", rt.Type, rt.Rate, rt.Amount, totalBase)
		}
		if rate != &rtRate {
			taxCombo.Ext = taxCombo.Ext.Set(sdi.ExtKeyRetainedRate, cbc.Code(formatPercentage(&rtRate)))
		}

		taxCombo.Percent = rate
		for _, line := range candidates {
			tc := *taxCombo
			tc.Ext = copyExtensions(taxCombo.Ext)
			line.Taxes = append(line.Taxes, &tc)
		}
		for _, charge := range retainedCharges {
			tc := *taxCombo
			tc.Ext = copyExtensions(taxCombo.Ext)
			charge.Taxes = append(charge.Taxes, &tc)
		}
	}

	return nil
}

// maxRetainedRateExp caps the decimal places tried when deriving a rate.
const maxRetainedRateExp = 6

// effectiveRate returns the percentage with the fewest decimals that yields
// amount from base, or nil when none exists within the bound.
func effectiveRate(amount, base num.Amount) *num.Percentage {
	if base.IsZero() {
		return nil
	}
	hundred := num.MakeAmount(100, 0)
	for exp := uint32(2); exp <= maxRetainedRateExp; exp++ {
		p := num.PercentageFromAmount(amount.Multiply(hundred).RescaleUp(exp).Divide(base))
		if reproduces(p, base, amount) {
			return &p
		}
	}
	return nil
}

// reproduces reports whether rate applied to base gives amount once rounded
// to the amount's precision.
func reproduces(rate num.Percentage, base, amount num.Amount) bool {
	return rate.Of(base).Rescale(amount.Exp()).Equals(amount)
}

// candidateLinesForRetention returns the lines that should be considered
// when matching retained taxes. If any line has Ritenuta=SI, only those
// lines are candidates. Otherwise all lines are candidates — the flag
// is optional in the FatturaPA spec and some invoicing software omits it.
func candidateLinesForRetention(lines []*bill.Line, lineDetails []*LineDetail) []*bill.Line {
	var flagged []*bill.Line
	for i, detail := range lineDetails {
		if i < len(lines) && detail.Retained == flagSI {
			flagged = append(flagged, lines[i])
		}
	}
	if len(flagged) > 0 {
		return flagged
	}
	return lines
}

// retainedFundContributionCharges returns the subset of invoice charges that
// correspond to fund contributions marked with Ritenuta=SI in the XML.
func retainedFundContributionCharges(charges []*bill.Charge, fcs []*FundContribution) []*bill.Charge {
	// Build a set of fund type codes that are marked as retained in the XML
	retainedTypes := make(map[cbc.Code]int)
	for _, fc := range fcs {
		if fc.Retained == flagSI {
			retainedTypes[cbc.Code(fc.Type)]++
		}
	}

	var out []*bill.Charge
	for _, charge := range charges {
		if !charge.Key.Has(sdi.KeyFundContribution) {
			continue
		}
		ft := charge.Ext.Get(sdi.ExtKeyFundType)
		if count, ok := retainedTypes[ft]; ok && count > 0 {
			out = append(out, charge)
			retainedTypes[ft]--
		}
	}
	return out
}

// copyExtensions returns a shallow copy of tax extensions.
func copyExtensions(ext tax.Extensions) tax.Extensions {
	return ext.Clone()
}

// convertRetainedTaxType converts a TipoRitenuta code to a tax category code
func convertRetainedTaxType(tipoRitenuta string) (cbc.Code, error) {
	if code := cbc.Code(tipoRitenuta); code != cbc.CodeEmpty {
		for _, def := range itRegime.Categories {
			if def.Map[it.KeyFatturaPATipoRitenuta] == code {
				return def.Code, nil
			}
		}
	}
	return "", fmt.Errorf("unknown TipoRitenuta code: %s", tipoRitenuta)
}
