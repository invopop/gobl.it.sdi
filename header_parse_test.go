package fatturapa_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseTestInvoice(t *testing.T, name string) *bill.Invoice {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), name))
	require.NoError(t, err)

	env, err := test.ConvertToGOBL(data)
	require.NoError(t, err)

	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.NotNil(t, inv)

	return inv
}

func TestHeaderIssuerInConversion(t *testing.T) {
	t.Run("should convert the third party issuer", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-third-party-issuer.xml")

		require.NotNil(t, inv.Ordering)
		issuer := inv.Ordering.Issuer
		require.NotNil(t, issuer)

		assert.Equal(t, "Fatturazione Terzi S.r.l.", issuer.Name)
		require.NotNil(t, issuer.TaxID)
		assert.Equal(t, l10n.TaxCountryCode("IT"), issuer.TaxID.Country)
		assert.Equal(t, cbc.Code("01234567897"), issuer.TaxID.Code)
	})

	t.Run("should convert the issuer type", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-third-party-issuer.xml")

		require.NotNil(t, inv.Tax)
		assert.Equal(t, sdi.ExtCodeIssuerTypeThirdParty, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
	})

	t.Run("should convert the customer issuer type", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-customer-issuer.xml")

		assert.Equal(t, sdi.ExtCodeIssuerTypeCustomer, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
		assert.Nil(t, inv.Ordering)
	})

	// The block exists only for a third party issuing on the supplier's behalf.
	t.Run("should treat a third-party block without an issuer type as TZ", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-intermediary.xml")

		assert.Equal(t, sdi.ExtCodeIssuerTypeThirdParty, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
	})

	t.Run("should keep the issuer alongside body order references", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-intermediary.xml")

		require.NotNil(t, inv.Ordering)
		assert.NotEmpty(t, inv.Ordering.Purchases)

		issuer := inv.Ordering.Issuer
		require.NotNil(t, issuer)
		assert.Nil(t, issuer.TaxID)
		require.Len(t, issuer.Identities, 1)
		assert.Equal(t, cbc.Code("RSSMRA80A01H501U"), issuer.Identities[0].Code)
		assert.Empty(t, issuer.Name)
		require.Len(t, issuer.People, 1)
		assert.Equal(t, "MARIO", issuer.People[0].Name.Given)
		assert.Equal(t, "ROSSI", issuer.People[0].Name.Surname)
	})

	t.Run("should convert a parsed person issuer back to Nome and Cognome", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-intermediary.xml"))
		require.NoError(t, err)
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		p := doc.Header.ThirdPartyIssuer.Identity.Profile
		assert.Empty(t, p.Name)
		assert.Equal(t, "MARIO", p.Given)
		assert.Equal(t, "ROSSI", p.Surname)
	})

	// Supplier and customer parsing drops the placeholder codes the converter
	// writes for them; the issuer never gets one, so its code is kept as given.
	t.Run("should keep an issuer tax code that looks like a placeholder", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-third-party-issuer.xml"))
		require.NoError(t, err)
		data = []byte(strings.Replace(string(data),
			"<IdPaese>IT</IdPaese>\n\t\t\t\t\t<IdCodice>01234567897</IdCodice>",
			"<IdPaese>FR</IdPaese>\n\t\t\t\t\t<IdCodice>0000000</IdCodice>", 1))
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv.Ordering.Issuer.TaxID)
		assert.Equal(t, cbc.Code("0000000"), inv.Ordering.Issuer.TaxID.Code)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		assert.Equal(t, "0000000", doc.Header.ThirdPartyIssuer.Identity.TaxID.Code)
	})

	t.Run("should keep a parsed person issuer's title", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-intermediary.xml"))
		require.NoError(t, err)
		data = []byte(strings.Replace(string(data),
			"<Cognome>ROSSI</Cognome>", "<Cognome>ROSSI</Cognome>\n\t\t\t\t\t<Titolo>Dott.</Titolo>", 1))
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.Len(t, inv.Ordering.Issuer.People, 1)
		assert.Equal(t, "Dott.", inv.Ordering.Issuer.People[0].Name.Prefix)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		assert.Equal(t, "Dott.", doc.Header.ThirdPartyIssuer.Identity.Profile.Title)
	})

	t.Run("should leave the issuer unset when the supplier issues", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-simple.xml")

		assert.Nil(t, inv.Ordering)
		assert.Empty(t, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType).String())
	})
}
