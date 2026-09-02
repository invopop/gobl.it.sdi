package fatturapa_test

import (
	"os"
	"path/filepath"
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

	t.Run("should leave the issuer unset when the supplier issues", func(t *testing.T) {
		inv := parseTestInvoice(t, "invoice-simple.xml")

		assert.Nil(t, inv.Ordering)
		assert.Empty(t, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType).String())
	})
}
