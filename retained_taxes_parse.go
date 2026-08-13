package fatturapa

import (
	"fmt"
	"strings"

	"github.com/invopop/gobl/addons/it/sdi"
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
			expectedAmount := rtRate.Of(*line.Total)

			if expectedAmount.Equals(rtAmount) {
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
			totalBase = totalBase.Add(*line.Total)
		}
		for _, charge := range retainedCharges {
			totalBase = totalBase.Add(charge.Amount)
		}

		expectedTotal := rtRate.Of(totalBase)
		if expectedTotal.Equals(rtAmount) {
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
			matched = true
		}

		if !matched {
			return fmt.Errorf("could not match retained tax: %s %s%% %s", rt.Type, rt.Rate, rt.Amount)
		}
	}

	return nil
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
