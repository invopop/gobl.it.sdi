package fatturapa

import (
	"strconv"

	"github.com/invopop/gobl/addons/it/sdi"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

const (
	naturaN21               = "N2.1"
	tipoDatoINVCONT         = "INVCONT"
	riferimentoTestoINVCONT = "Inversione contabile - art. 21 c.6 bis lett. a) DPR 633/72"
)

// GoodsServices contains all data related to the goods and services sold.
type GoodsServices struct {
	LineDetails []*LineDetail `xml:"DettaglioLinee"`
	TaxSummary  []*TaxSummary `xml:"DatiRiepilogo"`
}

// LineDetail contains line data such as description, quantity, price, etc.
type LineDetail struct {
	LineNumber       string             `xml:"NumeroLinea"`
	Description      string             `xml:"Descrizione"`
	Quantity         string             `xml:"Quantita"`
	Unit             string             `xml:"UnitaMisura,omitempty"`
	PeriodStart      string             `xml:"DataInizioPeriodo,omitempty"`
	PeriodEnd        string             `xml:"DataFinePeriodo,omitempty"`
	UnitPrice        string             `xml:"PrezzoUnitario"`
	PriceAdjustments []*PriceAdjustment `xml:"ScontoMaggiorazione,omitempty"`
	TotalPrice       string             `xml:"PrezzoTotale"`
	TaxRate          string             `xml:"AliquotaIVA"`
	Retained         string             `xml:"Ritenuta,omitempty"`
	TaxNature        string             `xml:"Natura,omitempty"`
	OtherData        []*OtherData       `xml:"AltriDatiGestionali,omitempty"`
}

// OtherData contains additional management data for a line item.
type OtherData struct {
	DataType      string `xml:"TipoDato"`
	TextReference string `xml:"RiferimentoTesto,omitempty"`
	NumReference  string `xml:"RiferimentoNumero,omitempty"`
	DateReference string `xml:"RiferimentoData,omitempty"`
}

// TaxSummary contains tax summary data such as tax rate, tax amount, etc.
type TaxSummary struct {
	TaxRate        string `xml:"AliquotaIVA"`
	TaxNature      string `xml:"Natura,omitempty"`
	TaxableAmount  string `xml:"ImponibileImporto"`
	TaxAmount      string `xml:"Imposta"`
	TaxLiability   string `xml:"EsigibilitaIVA,omitempty"`
	LegalReference string `xml:"RiferimentoNormativo,omitempty"`
}

func newGoodsServices(inv *bill.Invoice) *GoodsServices {
	return &GoodsServices{
		LineDetails: generateLineDetails(inv),
		TaxSummary:  generateTaxSummary(inv),
	}
}

func generateLineDetails(inv *bill.Invoice) []*LineDetail {
	var dl []*LineDetail

	for _, line := range inv.Lines {
		if line.Item == nil || line.Item.Price == nil {
			continue
		}

		// We need to invert the quantity if negative to comply with the
		// FatturaPA schema.
		q := line.Quantity
		lp := *line.Item.Price
		if q.IsNegative() {
			q = q.Negate()
			lp = lp.Negate()
		}
		d := &LineDetail{
			LineNumber:       strconv.Itoa(line.Index),
			Description:      line.Item.Name,
			Quantity:         formatAmount8(&q),
			Unit:             string(line.Item.Unit),
			UnitPrice:        formatAmount8(&lp),
			TotalPrice:       formatAmount8(line.Total),
			PriceAdjustments: extractLinePriceAdjustments(line),
		}

		if line.Period != nil {
			if !line.Period.Start.IsZero() {
				d.PeriodStart = line.Period.Start.String()
			}
			if !line.Period.End.IsZero() {
				d.PeriodEnd = line.Period.End.String()
			}
		}

		// Set VAT fields from the VAT combo on the line
		if vat := line.Taxes.Get(tax.CategoryVAT); vat != nil {
			d.TaxRate = formatPercentageWithZero(vat.Percent)
			d.TaxNature = exemptExtensionCode(vat.Ext)
			if d.TaxNature == naturaN21 && inv.HasTags(tax.TagReverseCharge) {
				d.OtherData = append(d.OtherData, &OtherData{
					DataType:      tipoDatoINVCONT,
					TextReference: riferimentoTestoINVCONT,
				})
			}
		}

		// Check for retained taxes
		for _, t := range line.Taxes {
			if t.Ext.Has(sdi.ExtKeyRetained) {
				d.Retained = flagSI
				break
			}
		}

		// Map item attributes (BG-32) to AltriDatiGestionali blocks.
		d.OtherData = append(d.OtherData, attributesToOtherData(line.Item.Attributes)...)

		dl = append(dl, d)
	}

	return dl
}

// attributesToOtherData maps an item's attributes (EN 16931 BG-32) to FatturaPA
// AltriDatiGestionali blocks. The attribute's type (or key as a fallback) becomes
// the TipoDato (BT-160), and its value (BT-161) is placed in the reference field
// that matches the GOBL value type: text and code values go to RiferimentoTesto,
// amounts to RiferimentoNumero, and dates to RiferimentoData.
func attributesToOtherData(attrs []*org.Attribute) []*OtherData {
	var out []*OtherData
	for _, a := range attrs {
		if a == nil {
			continue
		}
		name := a.Type.String()
		if name == "" {
			name = a.Key.String()
		}
		// The TipoDato is mandatory and limited to 10 characters (String10), and
		// INVCONT is reserved as the reverse-charge marker (it would be re-parsed
		// as such and the value lost), so skip anything that cannot be emitted as
		// a valid, unambiguous AltriDatiGestionali block.
		if name == "" || len(name) > 10 || name == tipoDatoINVCONT {
			continue
		}
		od := &OtherData{DataType: name}
		switch {
		case a.Text != "":
			od.TextReference = a.Text
		case a.Code != "":
			od.TextReference = a.Code.String()
		case a.Amount != nil:
			od.NumReference = formatAmount8(a.Amount)
		case a.Date != nil:
			od.DateReference = a.Date.String()
		default:
			// No BT-161 value to map; skip rather than emit a reference-less block.
			continue
		}
		out = append(out, od)
	}
	return out
}

func exemptExtensionCode(ext tax.Extensions) string {
	if v := ext.Get(sdi.ExtKeyExempt); v != "" {
		return v.String()
	}
	if v := ext.Get("it-sdi-nature"); v != "" { // old key
		return v.String()
	}
	return ""
}

func generateTaxSummary(inv *bill.Invoice) []*TaxSummary {
	var dr []*TaxSummary
	var vatRateTotals []*tax.RateTotal

	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code == tax.CategoryVAT {
			vatRateTotals = cat.Rates
			break
		}
	}

	for _, rateTotal := range vatRateTotals {
		// Get tax liability from extensions if present
		taxLiability := rateTotal.Ext.Get(sdi.ExtKeyVATLiability).String()

		dr = append(dr, &TaxSummary{
			TaxRate:        formatPercentageWithZero(rateTotal.Percent),
			TaxNature:      exemptExtensionCode(rateTotal.Ext),
			TaxableAmount:  formatAmount2(&rateTotal.Base),
			TaxAmount:      formatAmount2(&rateTotal.Amount),
			TaxLiability:   taxLiability,
			LegalReference: findRiferimentoNormativo(rateTotal),
		})
	}

	return dr
}

func extractLinePriceAdjustments(line *bill.Line) []*PriceAdjustment {
	list := make([]*PriceAdjustment, 0)

	for _, discount := range line.Discounts {
		// Unlike most formats, FatturaPA applies the discount to the unit price
		// instead of the line sum.
		// Quick ref: https://fex-app.com/FatturaElettronica/FatturaElettronicaBody/DatiBeniServizi/DettaglioLinee/PrezzoTotale
		a := discount.Amount
		if line.Quantity.Value() != 1 && !line.Quantity.IsZero() {
			a = a.RescaleUp(4).Divide(line.Quantity)
		}
		list = append(list, &PriceAdjustment{
			Type:    scontoMaggiorazioneTypeDiscount,
			Percent: formatPercentage(discount.Percent),
			Amount:  formatAmount8(&a),
		})
	}

	for _, charge := range line.Charges {
		a := charge.Amount
		if line.Quantity.Value() != 1 && !line.Quantity.IsZero() {
			a = a.RescaleUp(4).Divide(line.Quantity)
		}
		list = append(list, &PriceAdjustment{
			Type:    scontoMaggiorazioneTypeCharge,
			Percent: formatPercentage(charge.Percent),
			Amount:  formatAmount8(&a),
		})
	}

	return list
}

func findRiferimentoNormativo(rateTotal *tax.RateTotal) string {
	def := tax.ExtensionForKey(sdi.ExtKeyExempt)

	nature := exemptExtensionCode(rateTotal.Ext)
	for _, c := range def.Values {
		if c.Code.String() == nature {
			return c.Name[i18n.IT]
		}
	}

	return ""
}
