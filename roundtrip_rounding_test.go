package fatturapa_test

import (
	"encoding/xml"
	"testing"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundTripPreservesPayable checks that an inclusive-priced invoice's
// payable survives a GOBL -> FatturaPA -> GOBL round-trip.
func TestRoundTripPreservesPayable(t *testing.T) {
	// Inclusive prices whose net conversion doesn't divide evenly, so the
	// FatturaPA carries an Arrotondamento.
	const goblJSON = `{
		"$schema": "https://gobl.org/draft-0/bill/invoice",
		"$regime": "IT",
		"$addons": ["it-sdi-v1"],
		"uuid": "019ed4dc-f12a-790b-8fa1-2d13de01f9a6",
		"type": "standard",
		"code": "0001",
		"issue_date": "2026-06-17",
		"currency": "EUR",
		"tax": {
			"prices_include": "VAT",
			"rounding": "currency",
			"ext": {"it-sdi-document-type": "TD01", "it-sdi-format": "FPR12"}
		},
		"supplier": {
			"name": "Azienda Pippo Fatture S.r.l.",
			"tax_id": {"country": "IT", "code": "31122336964"},
			"addresses": [{"num": "42", "street": "Via Milano", "locality": "Milan", "region": "MI", "code": "20121", "country": "IT"}],
			"ext": {"it-sdi-fiscal-regime": "RF01"}
		},
		"customer": {
			"name": "Test Business S.r.l.",
			"tax_id": {"country": "IT", "code": "12345670017"},
			"addresses": [{"num": "1", "street": "Via Nazionale", "locality": "Rome", "code": "00184", "country": "IT"}]
		},
		"lines": [
			{
				"quantity": "3.00",
				"item": {"name": "Coffee", "price": "2.50"},
				"discounts": [{"reason": "Special Discount", "amount": "0.15"}],
				"charges": [{"reason": "Special Surcharge", "amount": "0.08"}],
				"taxes": [{"cat": "VAT", "key": "standard", "percent": "22.00%"}]
			}
		]
	}`

	out, err := gobl.Parse([]byte(goblJSON))
	require.NoError(t, err)
	env := gobl.NewEnvelope()
	require.NoError(t, env.Insert(out))

	sent, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.Equal(t, "7.43", sent.Totals.Payable.String())

	doc, err := test.ConvertFromGOBL(env, test.LoadOptions()...)
	require.NoError(t, err)
	xmlData, err := xml.Marshal(doc)
	require.NoError(t, err)

	back, err := test.ConvertToGOBL(xmlData)
	require.NoError(t, err)
	imported, ok := back.Extract().(*bill.Invoice)
	require.True(t, ok)

	assert.Equal(t, "7.43", imported.Totals.Payable.String())
}
