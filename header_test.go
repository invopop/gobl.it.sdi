package fatturapa_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderThirdPartyIssuer(t *testing.T) {
	t.Run("should contain the third party issuer details", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Ordering = &bill.Ordering{
				Issuer: &org.Party{
					Name: "Fatturazione Terzi S.r.l.",
					TaxID: &tax.Identity{
						Country: "IT",
						Code:    "01234567897",
					},
				},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		ti := doc.Header.ThirdPartyIssuer
		require.NotNil(t, ti)
		assert.Equal(t, "IT", ti.Identity.TaxID.Country)
		assert.Equal(t, "01234567897", ti.Identity.TaxID.Code)
		assert.Equal(t, "Fatturazione Terzi S.r.l.", ti.Identity.Profile.Name)
	})

	t.Run("should contain the third party issuer fiscal code", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Ordering = &bill.Ordering{
				Issuer: &org.Party{
					Name: "Mario Rossi",
					Identities: []*org.Identity{
						{
							Key:  it.IdentityKeyFiscalCode,
							Code: "RSSMRA80A01H501U",
						},
					},
				},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		ti := doc.Header.ThirdPartyIssuer
		require.NotNil(t, ti)
		assert.Equal(t, "RSSMRA80A01H501U", ti.Identity.FiscalCode)
		assert.Nil(t, ti.Identity.TaxID)
	})

	// IdFiscaleIVA is optional for the third party, so no placeholder code is
	// substituted the way it is for foreign customers.
	t.Run("should omit the tax ID for a foreign issuer without a code", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Ordering = &bill.Ordering{
				Issuer: &org.Party{
					Name: "Facturation Tiers SARL",
					TaxID: &tax.Identity{
						Country: "FR",
					},
					Identities: []*org.Identity{
						{
							Key:  it.IdentityKeyFiscalCode,
							Code: "RSSMRA80A01H501U",
						},
					},
				},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		ti := doc.Header.ThirdPartyIssuer
		require.NotNil(t, ti)
		assert.Nil(t, ti.Identity.TaxID)
		assert.Equal(t, "RSSMRA80A01H501U", ti.Identity.FiscalCode)
	})

	// RegimeFiscale is not part of DatiAnagraficiTerzoIntermediarioType.
	t.Run("should not set a fiscal regime for the third party issuer", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Ordering = &bill.Ordering{
				Issuer: &org.Party{
					Name: "Fatturazione Terzi S.r.l.",
					TaxID: &tax.Identity{
						Country: "IT",
						Code:    "01234567897",
					},
					Ext: tax.ExtensionsOf(cbc.CodeMap{
						sdi.ExtKeyFiscalRegime: "RF02",
					}),
				},
			}
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		require.NotNil(t, doc.Header.ThirdPartyIssuer)
		assert.Empty(t, doc.Header.ThirdPartyIssuer.Identity.FiscalRegime)
	})

	t.Run("should be omitted when the supplier issues the invoice", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Nil(t, doc.Header.ThirdPartyIssuer)
	})
}

func TestHeaderIssuerType(t *testing.T) {
	t.Run("should indicate an invoice compiled by a third party", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Tax = inv.Tax.MergeExtensions(tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyIssuerType: sdi.ExtCodeIssuerTypeThirdParty,
			}))
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Equal(t, "TZ", doc.Header.IssuerType)
	})

	t.Run("should indicate an invoice compiled by the customer", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Tax = inv.Tax.MergeExtensions(tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyIssuerType: sdi.ExtCodeIssuerTypeCustomer,
			}))
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Equal(t, "CC", doc.Header.IssuerType)
	})

	t.Run("should be omitted when the supplier issues the invoice", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Empty(t, doc.Header.IssuerType)
	})
}
