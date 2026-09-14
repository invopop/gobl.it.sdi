package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl"
	fatturapa "github.com/invopop/gobl.it.sdi"
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func convertSimplified(t *testing.T, env *gobl.Envelope) *fatturapa.SimplifiedInvoice {
	t.Helper()
	inv, err := fatturapa.ConvertSimplifiedInvoice(env)
	require.NoError(t, err)
	return inv
}

func TestConvertFormats(t *testing.T) {
	t.Run("ordinary invoice", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := fatturapa.Convert(env)
		require.NoError(t, err)
		assert.IsType(t, &fatturapa.Invoice{}, doc)

		data, err := fatturapa.Bytes(doc)
		require.NoError(t, err)
		assert.Contains(t, string(data), "<p:FatturaElettronica ")

		_, err = fatturapa.ConvertSimplifiedInvoice(env)
		assert.ErrorContains(t, err, "expected a simplified invoice, got *fatturapa.Invoice")
	})

	t.Run("simplified invoice", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		doc, err := fatturapa.Convert(env)
		require.NoError(t, err)
		assert.IsType(t, &fatturapa.SimplifiedInvoice{}, doc)

		data, err := fatturapa.Bytes(doc)
		require.NoError(t, err)
		assert.Contains(t, string(data), "<p:FatturaElettronicaSemplificata ")

		_, err = fatturapa.ConvertInvoice(env)
		assert.ErrorContains(t, err, "expected an ordinary invoice, got *fatturapa.SimplifiedInvoice")
	})

	t.Run("bytes of an unsupported document", func(t *testing.T) {
		_, err := fatturapa.Bytes("not a document")
		assert.ErrorContains(t, err, "unsupported document type string")
	})
}

func TestSimplifiedConvert(t *testing.T) {
	t.Run("entries add up to the total with tax", func(t *testing.T) {
		for _, file := range []string{
			"invoice-simplified.json",
			"invoice-simplified-private.json",
			"invoice-simplified-exempt.json",
			"invoice-simplified-credit-note.json",
		} {
			env := test.LoadTestFile(file, test.PathGOBLFatturaPA)
			doc := convertSimplified(t, env)

			sum := num.MakeAmount(0, 2)
			for _, gs := range doc.Body[0].GoodsServices {
				a, err := num.AmountFromString(gs.Amount)
				require.NoError(t, err)
				sum = sum.Add(a)
			}
			inv := env.Extract().(*bill.Invoice)
			assert.Equal(t, inv.Totals.TotalWithTax.String(), sum.String(), file)
		}
	})

	t.Run("prices with VAT keep each line's total", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		gs := doc.Body[0].GoodsServices
		require.Len(t, gs, 2)
		assert.Equal(t, "37.00", gs[0].Amount)
		assert.Equal(t, "10.00", gs[0].VAT.Rate)
		assert.Equal(t, "12.00", gs[1].Amount)
		assert.Equal(t, "22.00", gs[1].VAT.Rate)
	})

	t.Run("net prices leave the rounding on a rate's last entry", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-private.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		gs := doc.Body[0].GoodsServices
		require.Len(t, gs, 4)
		assert.Equal(t, "0.12", gs[0].Amount)
		assert.Equal(t, "0.12", gs[1].Amount)
		assert.Equal(t, "0.13", gs[2].Amount)
		assert.Equal(t, "3.96", gs[3].Amount)
	})

	t.Run("discounts are negative entries", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-exempt.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		gs := doc.Body[0].GoodsServices
		require.Len(t, gs, 3)
		assert.Equal(t, "Sconto convenzione", gs[2].Description)
		assert.Equal(t, "-10.00", gs[2].Amount)
		assert.Equal(t, "N4", gs[2].TaxNature)
	})

	t.Run("stamp duty is an entry and sets the virtual stamp", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-exempt.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		assert.Equal(t, "SI", doc.Body[0].GeneralData.Document.VirtualStamp)
		gs := doc.Body[0].GoodsServices[1]
		assert.Equal(t, "Bollo", gs.Description)
		assert.Equal(t, "2.00", gs.Amount)
	})

	t.Run("credit note identifies the corrected invoice", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-credit-note.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		gd := doc.Body[0].GeneralData
		assert.Equal(t, "TD08", gd.Document.DocumentType)
		require.NotNil(t, gd.Corrected)
		assert.Equal(t, "SEMP-001", gd.Corrected.Number)
		assert.Equal(t, "2026-09-28", gd.Corrected.IssueDate)
		assert.Equal(t, "Storno vino della casa non servito", gd.Corrected.Details)
	})

	t.Run("standard invoice leaves out its preceding reference", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Preceding = []*org.DocumentRef{{Code: "SEMP-000", Reason: "Replaces a rejected invoice"}}
		})
		doc := convertSimplified(t, env)
		assert.Nil(t, doc.Body[0].GeneralData.Corrected)
	})

	t.Run("simplified format rejects ordinary document types", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyDocumentType, "TD01")
		})
		_, err := fatturapa.Convert(env)
		assert.ErrorContains(t, err, "document type TD01 is not valid in the FSM10 format")
	})

	t.Run("ordinary format rejects simplified document types", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyFormat, "FPR12")
		})
		_, err := fatturapa.Convert(env)
		assert.ErrorContains(t, err, "document type TD07 requires the FSM10 format")
	})

	t.Run("credit note requires a complete corrected invoice", func(t *testing.T) {
		for name, modify := range map[string]func(*bill.Invoice){
			"no preceding":  func(inv *bill.Invoice) { inv.Preceding = nil },
			"no issue date": func(inv *bill.Invoice) { inv.Preceding[0].IssueDate = nil },
			"no reason":     func(inv *bill.Invoice) { inv.Preceding[0].Reason = "" },
		} {
			env := test.LoadTestFile("invoice-simplified-credit-note.json", test.PathGOBLFatturaPA)
			test.ModifyInvoice(env, modify)
			_, err := fatturapa.Convert(env)
			assert.ErrorContains(t, err, "document type TD08 requires the corrected invoice with its code, issue date and reason", name)
		}
	})
}

func TestSimplifiedCustomer(t *testing.T) {
	t.Run("name and address", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		c := doc.Header.Customer
		assert.Equal(t, "13029381004", c.FiscalIdentifiers.TaxID.Code)
		require.NotNil(t, c.OtherIdentifiers)
		assert.Equal(t, "Studio Rossi S.r.l.", c.OtherIdentifiers.Name)
		assert.Equal(t, "Firenze", c.OtherIdentifiers.Address.Locality)
	})

	t.Run("name without an address is left out", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-credit-note.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		c := doc.Header.Customer
		assert.Equal(t, "13029381004", c.FiscalIdentifiers.TaxID.Code)
		assert.Nil(t, c.OtherIdentifiers)
	})

	t.Run("fiscal code only", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-private.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		c := doc.Header.Customer
		assert.Nil(t, c.FiscalIdentifiers.TaxID)
		assert.Equal(t, "RSSGNN60R30H501U", c.FiscalIdentifiers.FiscalCode)
		assert.Nil(t, c.OtherIdentifiers)
	})

	t.Run("person", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified-exempt.json", test.PathGOBLFatturaPA)
		doc := convertSimplified(t, env)
		c := doc.Header.Customer
		require.NotNil(t, c.OtherIdentifiers)
		assert.Equal(t, "Giovanni", c.OtherIdentifiers.Given)
		assert.Equal(t, "Rossi", c.OtherIdentifiers.Surname)
	})

	t.Run("foreign customer", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Country = "US"
			inv.Customer.TaxID.Code = ""
			inv.Customer.Addresses = []*org.Address{
				{Street: "Main Street", Locality: "Boston", Country: "US"},
			}
		})
		doc := convertSimplified(t, env)
		assert.Equal(t, "XXXXXXX", doc.Header.TransmissionData.RecipientCode)
		c := doc.Header.Customer
		assert.Equal(t, "US", c.FiscalIdentifiers.TaxID.Country)
		assert.Equal(t, "00000", c.OtherIdentifiers.Address.Code)
	})
}

func TestSimplifiedIssuerType(t *testing.T) {
	t.Run("is written to the header and parsed back", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Tax = inv.Tax.MergeExtensions(tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyIssuerType: sdi.ExtCodeIssuerTypeThirdParty,
			}))
		})

		doc, err := fatturapa.ConvertSimplifiedInvoice(env, test.LoadOptions()...)
		require.NoError(t, err)
		assert.Equal(t, "TZ", doc.Header.IssuerType)

		data, err := fatturapa.Bytes(doc)
		require.NoError(t, err)
		schema, err := test.LoadSimplifiedSchema()
		require.NoError(t, err)
		assert.Empty(t, test.ValidateXML(schema, data))

		parsed, err := fatturapa.Parse(data)
		require.NoError(t, err)
		inv, ok := parsed.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Equal(t, sdi.ExtCodeIssuerTypeThirdParty, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
	})

	t.Run("is omitted when the supplier issues", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simplified.json", test.PathGOBLFatturaPA)

		doc := convertSimplified(t, env)
		assert.Empty(t, doc.Header.IssuerType)
	})
}
