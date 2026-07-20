package fatturapa

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/invopop/gobl/addons/it/sdi"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// goblBillInvoiceAddGoodsServices adds goods and services from the FatturaPA document to the GOBL invoice
func goblBillInvoiceAddGoodsServices(inv *bill.Invoice, goodsServices *GoodsServices, retainedTaxes []*RetainedTax, fundContributions []*FundContribution) error {
	if inv == nil || goodsServices == nil {
		return nil
	}

	// Add line details, passing the retained taxes and tax summaries
	if err := goblBillInvoiceAddLineDetails(inv, goodsServices.LineDetails, retainedTaxes, fundContributions, goodsServices.TaxSummary); err != nil {
		return fmt.Errorf("adding line details: %w", err)
	}

	return nil
}

// goblBillInvoiceAddLineDetails adds line details from the FatturaPA document to the GOBL invoice
func goblBillInvoiceAddLineDetails(inv *bill.Invoice, lineDetails []*LineDetail, retainedTaxes []*RetainedTax, fundContributions []*FundContribution, taxSummaries []*TaxSummary) error {
	if inv == nil || len(lineDetails) == 0 {
		return nil
	}

	// Initialize lines if needed
	if inv.Lines == nil {
		inv.Lines = make([]*bill.Line, 0)
	}

	// Process each line detail
	for _, detail := range lineDetails {
		// Parse line number, quantity, unit price, and total price
		index, err := strconv.Atoi(detail.LineNumber)
		if err != nil {
			return fmt.Errorf("parsing line number: %w", err)
		}

		// Parse total price and unit price first
		unitPrice, err := parseAmount(detail.UnitPrice)
		if err != nil {
			return fmt.Errorf("parsing unit price: %w", err)
		}
		totalPrice, err := parseAmount(detail.TotalPrice)
		if err != nil {
			return fmt.Errorf("parsing total price: %w", err)
		}

		// Handle quantity parsing/calculation
		var quantity num.Amount
		if detail.Quantity != "" {
			quantity, err = parseAmount(detail.Quantity)
			if err != nil {
				return fmt.Errorf("parsing quantity: %w", err)
			}
		} else if unitPrice.IsZero() {
			quantity = num.MakeAmount(1, 0)
		} else {
			quantity = totalPrice.Divide(unitPrice)
		}

		// Create a new line
		line := &bill.Line{
			Index:    index,
			Quantity: quantity,
			Total:    &totalPrice,
			Item: &org.Item{
				Name:  detail.Description,
				Price: &unitPrice,
			},
		}

		// Add period dates
		if detail.PeriodStart != "" || detail.PeriodEnd != "" {
			p := &cal.Period{}
			if detail.PeriodStart != "" {
				p.Start, err = parseDate(detail.PeriodStart)
				if err != nil {
					return fmt.Errorf("parsing period start: %w", err)
				}
			}
			if detail.PeriodEnd != "" {
				p.End, err = parseDate(detail.PeriodEnd)
				if err != nil {
					return fmt.Errorf("parsing period end: %w", err)
				}
			}
			line.Period = p
		}

		// Add unit. Add to description if unit is invalid
		if detail.Unit != "" {
			unit := org.Unit(detail.Unit)
			if err := unit.Validate(); err != nil {
				line.Item.Description = fmt.Sprintf("(Unit: %s)", detail.Unit)
			} else {
				line.Item.Unit = unit
			}
		}

		// Add price adjustments
		goblBillLineAddPriceAdjustments(line, detail.PriceAdjustments)

		// Add tax information
		vatCombo, err := goblVATCombo(detail.TaxRate, detail.TaxNature)
		if err != nil {
			return fmt.Errorf("parsing line %s tax: %w", detail.LineNumber, err)
		}
		if vatCombo != nil {
			line.Taxes = append(line.Taxes, vatCombo)
		}

		// Map AltriDatiGestionali blocks: the INVCONT marker sets the
		// reverse-charge tag, everything else becomes an item attribute (BG-32).
		for _, od := range detail.OtherData {
			if od == nil {
				continue
			}
			if od.DataType == tipoDatoINVCONT {
				inv.SetTags(tax.TagReverseCharge)
				continue
			}
			if attr := otherDataToAttribute(od); attr != nil {
				line.Item.Attributes = append(line.Item.Attributes, attr)
			}
		}

		// Add line to invoice
		inv.Lines = append(inv.Lines, line)
	}

	// Process retained taxes
	if err := processRetainedTaxes(inv, lineDetails, retainedTaxes, fundContributions); err != nil {
		return fmt.Errorf("processing retained taxes: %w", err)
	}

	// Match tax summary liability information with line items
	if len(inv.Lines) > 0 && len(taxSummaries) > 0 {
		goblBillLinesAddTaxSummary(inv.Lines, lineDetails, taxSummaries)
	}

	return nil
}

// otherDataToAttribute converts a FatturaPA AltriDatiGestionali block into a GOBL
// item attribute (EN 16931 BG-32). The TipoDato (BT-160) becomes the attribute
// type, and the populated reference field (BT-161) becomes the matching value:
// RiferimentoTesto to text, RiferimentoNumero to amount, RiferimentoData to date.
func otherDataToAttribute(od *OtherData) *org.Attribute {
	if od == nil || od.DataType == "" {
		return nil
	}
	a := &org.Attribute{Type: cbc.Code(od.DataType)}
	switch {
	case od.TextReference != "":
		a.Text = od.TextReference
	case od.NumReference != "":
		amount, err := parseAmount(od.NumReference)
		if err != nil {
			// Preserve the raw value rather than dropping the attribute.
			a.Text = od.NumReference
		} else {
			a.Amount = &amount
		}
	case od.DateReference != "":
		date, err := parseDate(od.DateReference)
		if err != nil {
			a.Text = od.DateReference
		} else {
			a.Date = &date
		}
	default:
		// TipoDato with no reference value has nothing to map to a GOBL value.
		return nil
	}
	return a
}

// goblBillLinesAddTaxSummary matches tax summary liability information with line items
func goblBillLinesAddTaxSummary(lines []*bill.Line, lineDetails []*LineDetail, taxSummaries []*TaxSummary) {
	// Process each line
	for i, line := range lines {
		// Skip if line has no taxes
		if len(line.Taxes) == 0 {
			continue
		}

		// Get the VAT tax for this line
		var vatTax *tax.Combo
		for _, t := range line.Taxes {
			if t.Category == tax.CategoryVAT {
				vatTax = t
				break
			}
		}

		// Skip if no VAT tax found
		if vatTax == nil {
			continue
		}

		// Get the line detail for this line
		detail := lineDetails[i]

		// Find a matching tax summary
		for _, summary := range taxSummaries {
			// Match by tax rate or nature
			rateMatches := detail.TaxRate == summary.TaxRate
			natureMatches := detail.TaxNature != "" && detail.TaxNature == summary.TaxNature

			if (rateMatches || natureMatches) && summary.TaxLiability != "" {
				// Add the tax liability to the VAT tax extensions
				vatTax.Ext = vatTax.Ext.Set(sdi.ExtKeyVATLiability, cbc.Code(summary.TaxLiability))
				break
			}
		}
	}
}

// goblBillLineAddPriceAdjustments adds price adjustments to a line
func goblBillLineAddPriceAdjustments(line *bill.Line, adjustments []*PriceAdjustment) {
	if line == nil || len(adjustments) == 0 {
		return
	}

	for _, adj := range adjustments {
		amount, err1 := parseAmount(adj.Amount)
		// FatturaPA stores the percentage as a string without the % symbol so we add it so that the conversion works
		percent, err2 := num.PercentageFromString(strings.TrimSpace(adj.Percent) + "%")

		if err1 != nil && err2 != nil {
			// Skip if both amount and percent are invalid
			continue
		}

		// Check if percentage is zero, done so that if percentage is zero, it is not added to the invoice
		var percentPtr *num.Percentage
		if err2 == nil && percent != num.PercentageZero {
			percentPtr = &percent
		}

		// If the quantity is not 1, we need to multiply the amount by the quantity
		// because FatturaPA stores the discount/charge per unit
		if line.Quantity.Value() != 1 {
			amount = amount.Multiply(line.Quantity)
		}

		switch adj.Type {
		case scontoMaggiorazioneTypeDiscount:
			if line.Discounts == nil {
				line.Discounts = make([]*bill.LineDiscount, 0)
			}
			line.Discounts = append(line.Discounts, &bill.LineDiscount{
				Amount:  amount,
				Percent: percentPtr,
			})
		case scontoMaggiorazioneTypeCharge:
			if line.Charges == nil {
				line.Charges = make([]*bill.LineCharge, 0)
			}
			line.Charges = append(line.Charges, &bill.LineCharge{
				Amount:  amount,
				Percent: percentPtr,
			})
		}
	}
}
