package fatturapa_test

import (
	"testing"

	fatturapa "github.com/invopop/gobl.it.sdi"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simplifiedXML wraps entries in a minimal FSM10 document.
func simplifiedXML(entries string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:FatturaElettronicaSemplificata xmlns:p="http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.0" versione="FSM10">
	<FatturaElettronicaHeader>
		<DatiTrasmissione>
			<IdTrasmittente><IdPaese>IT</IdPaese><IdCodice>01234567890</IdCodice></IdTrasmittente>
			<ProgressivoInvio>1</ProgressivoInvio>
			<FormatoTrasmissione>FSM10</FormatoTrasmissione>
			<CodiceDestinatario>0000000</CodiceDestinatario>
		</DatiTrasmissione>
		<CedentePrestatore>
			<IdFiscaleIVA><IdPaese>IT</IdPaese><IdCodice>12345678903</IdCodice></IdFiscaleIVA>
			<Denominazione>Test Supplier</Denominazione>
			<Sede><Indirizzo>Via Roma</Indirizzo><CAP>00100</CAP><Comune>Roma</Comune><Nazione>IT</Nazione></Sede>
			<RegimeFiscale>RF01</RegimeFiscale>
		</CedentePrestatore>
		<CessionarioCommittente>
			<IdentificativiFiscali><CodiceFiscale>RSSGNN60R30H501U</CodiceFiscale></IdentificativiFiscali>
		</CessionarioCommittente>
	</FatturaElettronicaHeader>
	<FatturaElettronicaBody>
		<DatiGenerali>
			<DatiGeneraliDocumento>
				<TipoDocumento>TD07</TipoDocumento>
				<Divisa>EUR</Divisa>
				<Data>2026-09-15</Data>
				<Numero>1</Numero>
			</DatiGeneraliDocumento>
		</DatiGenerali>
		` + entries + `
	</FatturaElettronicaBody>
</p:FatturaElettronicaSemplificata>`)
}

func parseSimplifiedInvoice(t *testing.T, data []byte) *bill.Invoice {
	t.Helper()
	env, err := fatturapa.Parse(data)
	require.NoError(t, err)
	require.NoError(t, env.Calculate())
	return env.Extract().(*bill.Invoice)
}

func TestSimplifiedParse(t *testing.T) {
	t.Run("simplified invoice", func(t *testing.T) {
		inv := parseSimplifiedInvoice(t, simplifiedXML(`
		<DatiBeniServizi>
			<Descrizione>Pranzo</Descrizione>
			<Importo>22.00</Importo>
			<DatiIVA><Aliquota>10.00</Aliquota></DatiIVA>
		</DatiBeniServizi>`))
		assert.True(t, inv.HasTags(tax.TagSimplified))
		assert.Equal(t, "FSM10", inv.Tax.Ext.Get("it-sdi-format").String())
		assert.Equal(t, "TD07", inv.Tax.Ext.Get("it-sdi-document-type").String())
		assert.Equal(t, tax.CategoryVAT, inv.Tax.PricesInclude)
		require.Len(t, inv.Lines, 1)
		assert.Equal(t, "22.00", inv.Lines[0].Item.Price.String())
		assert.Equal(t, "10.00%", inv.Lines[0].Taxes[0].Percent.String())
		assert.Equal(t, "22.00", inv.Totals.TotalWithTax.String())
	})

	t.Run("rate wins over VAT amount", func(t *testing.T) {
		inv := parseSimplifiedInvoice(t, simplifiedXML(`
		<DatiBeniServizi>
			<Descrizione>Pranzo</Descrizione>
			<Importo>22.00</Importo>
			<DatiIVA><Imposta>2.00</Imposta><Aliquota>22.00</Aliquota></DatiIVA>
		</DatiBeniServizi>`))
		assert.Equal(t, "22.00%", inv.Lines[0].Taxes[0].Percent.String())
	})

	t.Run("exemption", func(t *testing.T) {
		inv := parseSimplifiedInvoice(t, simplifiedXML(`
		<DatiBeniServizi>
			<Descrizione>Visita</Descrizione>
			<Importo>80.00</Importo>
			<DatiIVA><Aliquota>0.00</Aliquota></DatiIVA>
			<Natura>N4</Natura>
		</DatiBeniServizi>`))
		assert.Equal(t, "N4", inv.Lines[0].Taxes[0].Ext.Get("it-sdi-exempt").String())
		assert.Nil(t, inv.Lines[0].Taxes[0].Percent)
	})

	t.Run("entry without VAT", func(t *testing.T) {
		_, err := fatturapa.Parse(simplifiedXML(`
		<DatiBeniServizi>
			<Descrizione>Pranzo</Descrizione>
			<Importo>22.00</Importo>
			<DatiIVA></DatiIVA>
		</DatiBeniServizi>`))
		assert.ErrorContains(t, err, `entry "Pranzo" has no VAT rate, tax or exemption`)
	})

	t.Run("zero VAT amount without an exemption", func(t *testing.T) {
		_, err := fatturapa.Parse(simplifiedXML(`
		<DatiBeniServizi>
			<Descrizione>Pranzo</Descrizione>
			<Importo>22.00</Importo>
			<DatiIVA><Imposta>0.00</Imposta></DatiIVA>
		</DatiBeniServizi>`))
		assert.ErrorContains(t, err, "zero VAT without a rate or exemption")
	})

	t.Run("converted invoices keep their total with tax", func(t *testing.T) {
		for _, file := range []string{
			"invoice-simplified.json",
			"invoice-simplified-private.json",
			"invoice-simplified-exempt.json",
			"invoice-simplified-credit-note.json",
		} {
			env := test.LoadTestFile(file, test.PathGOBLFatturaPA)
			want := env.Extract().(*bill.Invoice).Totals.TotalWithTax

			doc, err := fatturapa.ConvertSimplifiedInvoice(env)
			require.NoError(t, err)
			data, err := doc.Bytes()
			require.NoError(t, err)

			inv := parseSimplifiedInvoice(t, data)
			assert.Equal(t, want.String(), inv.Totals.TotalWithTax.String(), file)
		}
	})
}
