package fatturapa_test

import (
	"testing"

	sdi "github.com/invopop/gobl.fatturapa/addon"
	"github.com/invopop/gobl.fatturapa/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartiesSupplier(t *testing.T) {
	t.Run("should contain the supplier info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "IT", s.Identity.TaxID.Country)
		assert.Equal(t, "12345678903", s.Identity.TaxID.Code)
		assert.Equal(t, "MªF. Services", s.Identity.Profile.Name)
		assert.Equal(t, "RF02", s.Identity.FiscalRegime)
		assert.Equal(t, "VIALE DELLA LIBERTÀ", s.Address.Street)
		assert.Equal(t, "1", s.Address.Number)
		assert.Equal(t, "00100", s.Address.Code)
		assert.Equal(t, "ROMA", s.Address.Locality)
		assert.Equal(t, "RM", s.Address.Region)
		assert.Equal(t, "IT", s.Address.Country)
		assert.Equal(t, "RM", s.Registration.Office)
		assert.Equal(t, "123456", s.Registration.Entry)
		assert.Equal(t, "50000.00", s.Registration.Capital)
		assert.Equal(t, "LN", s.Registration.LiquidationState)
	})

	t.Run("should set the supplier fiscal regime", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		inv := env.Extract().(*bill.Invoice)
		inv.Supplier.Ext = tax.Extensions{}
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "RF01", s.Identity.FiscalRegime)
	})

	t.Run("should keep supplier info for EU company with Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = "81237984062783472"
			inv.Supplier.TaxID.Country = l10n.AT.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "AT", s.Identity.TaxID.Country)
		assert.Equal(t, "81237984062783472", s.Identity.TaxID.Code)
	})

	t.Run("should default supplier info for EU individual with no Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = ""
			inv.Supplier.TaxID.Country = l10n.SE.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "SE", s.Identity.TaxID.Country)
		assert.Equal(t, "0000000", s.Identity.TaxID.Code)
	})

	t.Run("should keep supplier Tax ID for Greek company (EL tax country code)", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = "925667500"
			inv.Supplier.TaxID.Country = l10n.EL.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "EL", s.Identity.TaxID.Country)
		assert.Equal(t, "925667500", s.Identity.TaxID.Code)
	})

	t.Run("should replace supplier ID info for non-EU company with Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = "09823876432"
			inv.Supplier.TaxID.Country = l10n.GB.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "GB", s.Identity.TaxID.Country)
		assert.Equal(t, "OO99999999999", s.Identity.TaxID.Code)
	})

	t.Run("should default supplier info for non-EU individual with no Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = ""
			inv.Supplier.TaxID.Country = l10n.JP.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		s := doc.Header.Supplier

		assert.Equal(t, "JP", s.Identity.TaxID.Country)
		assert.Equal(t, "0000000", s.Identity.TaxID.Code)
	})

	t.Run("should return an error if supplier has no Tax ID", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID = nil
		})

		_, err := test.ConvertFromGOBL(env)
		require.ErrorContains(t, err, "supplier tax ID is required")
	})

	t.Run("should return an error for IT supplier with no Tax ID code", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Supplier.TaxID.Code = ""
		})

		_, err := test.ConvertFromGOBL(env)
		require.ErrorContains(t, err, "supplier tax ID is required")
	})

	t.Run("should emit IscrizioneREA liquidation state and sole shareholder", func(t *testing.T) {
		cases := []struct {
			name            string
			liquidation     string
			soleShareholder string
		}{
			{"not in liquidation, multiple shareholders", "LN", "SM"},
			{"not in liquidation, sole shareholder", "LN", "SU"},
			{"in liquidation, multiple shareholders", "LS", "SM"},
			{"in liquidation, sole shareholder", "LS", "SU"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
				test.ModifyInvoice(env, func(inv *bill.Invoice) {
					inv.Supplier.Registration.Ext = inv.Supplier.Registration.Ext.
						Set(sdi.ExtKeyLiquidationState, cbc.Code(c.liquidation)).
						Set(sdi.ExtKeyShareholderState, cbc.Code(c.soleShareholder))
				})
				doc, err := test.ConvertFromGOBL(env)
				require.NoError(t, err)

				s := doc.Header.Supplier
				assert.Equal(t, c.liquidation, s.Registration.LiquidationState)
				assert.Equal(t, c.soleShareholder, s.Registration.SoleShareholder)
			})
		}
	})

	t.Run("should omit SocioUnico when sole shareholder unset", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		assert.Empty(t, doc.Header.Supplier.Registration.SoleShareholder)
	})
}

func TestPartiesCustomer(t *testing.T) {
	t.Run("should contain the customer info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-irpef.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Nil(t, c.Identity.TaxID)
		assert.Equal(t, "MRALNE80E05H501C", c.Identity.FiscalCode)
		assert.Equal(t, "", c.Identity.Profile.Name)
		assert.Equal(t, "MARIO", c.Identity.Profile.Given)
		assert.Equal(t, "LEONI", c.Identity.Profile.Surname)
		assert.Equal(t, "Dott.", c.Identity.Profile.Title)
		assert.Equal(t, "VIALE DELI LAVORATORI", c.Address.Street)
		assert.Equal(t, "32", c.Address.Number)
		assert.Equal(t, "50100", c.Address.Code)
		assert.Equal(t, "FIRENZE", c.Address.Locality)
		assert.Equal(t, "FI", c.Address.Region)
		assert.Equal(t, "IT", c.Address.Country)
	})

	t.Run("should contain customer info with codice fiscale", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Code = ""
			inv.Customer.Identities = org.AddIdentity(inv.Customer.Identities,
				&org.Identity{
					Key:  it.IdentityKeyFiscalCode,
					Code: "RSSGNC73A02F205X",
				},
			)
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Nil(t, c.Identity.TaxID)
		assert.Equal(t, "RSSGNC73A02F205X", c.Identity.FiscalCode)
	})

	t.Run("should contain customer info for EU citizen with Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Code = "81237984062783472"
			inv.Customer.TaxID.Country = l10n.AT.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Equal(t, "AT", c.Identity.TaxID.Country)
		assert.Equal(t, "81237984062783472", c.Identity.TaxID.Code)
	})

	t.Run("should contain customer info for EU citizen with no Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Code = ""
			inv.Customer.TaxID.Country = l10n.SE.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Equal(t, "SE", c.Identity.TaxID.Country)
		assert.Equal(t, "0000000", c.Identity.TaxID.Code)
	})

	t.Run("should replace customer ID info for non-EU citizen with Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Code = "09823876432"
			inv.Customer.TaxID.Country = l10n.GB.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Equal(t, "GB", c.Identity.TaxID.Country)
		assert.Equal(t, "OO99999999999", c.Identity.TaxID.Code)
	})

	t.Run("should contain customer info for non-EU citizen with no Tax ID given", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Code = ""
			inv.Customer.TaxID.Country = l10n.JP.Tax()
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Equal(t, "JP", c.Identity.TaxID.Country)
		assert.Equal(t, "0000000", c.Identity.TaxID.Code)
	})

	t.Run("should not fail if missing key data", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.TaxID.Country = ""
		})

		_, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
	})

	t.Run("should take the company name when provided even if no tax ID", func(t *testing.T) {
		env := test.LoadTestFile("invoice-irpef.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Customer.Name = "ACME Corp"
		})

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		c := doc.Header.Customer

		assert.Equal(t, "ACME Corp", c.Identity.Profile.Name)
		assert.Empty(t, c.Identity.Profile.Given)
	})
}
