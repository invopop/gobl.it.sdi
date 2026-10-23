package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTaxCombo(t *testing.T) {
	ad := tax.AddonForKey(sdi.V1)
	t.Run("nil", func(t *testing.T) {
		var tc *tax.Combo
		assert.NotPanics(t, func() {
			norm.Normalize(tc, tax.AddonContext(sdi.V1))
		})
	})

	t.Run("B2C EU export", func(t *testing.T) {
		tc := &tax.Combo{
			Country:  "ES",
			Category: tax.CategoryVAT,
			Key:      tax.KeyStandard,
			Percent:  num.NewPercentage(21, 3),
		}
		norm.Normalize(tc, tax.AddonContext(sdi.V1))
		assert.Equal(t, cbc.Code("N7"), tc.Ext.Get(sdi.ExtKeyExempt))
	})

	t.Run("B2C other export", func(t *testing.T) {
		tc := &tax.Combo{
			Country:  "GB",
			Category: tax.CategoryVAT,
			Key:      tax.KeyStandard,
			Percent:  num.NewPercentage(20, 3),
		}
		norm.Normalize(tc, tax.AddonContext(sdi.V1))
		assert.Equal(t, cbc.Code("N2.1"), tc.Ext.Get(sdi.ExtKeyExempt))
	})

	t.Run("check all extensions", func(t *testing.T) {
		ex := cbc.GetKeyDefinition(sdi.ExtKeyExempt, ad.Extensions)
		codes := cbc.DefinitionCodes(ex.Values)

		for _, code := range codes {
			t.Run("with code "+code.String(), func(t *testing.T) {
				tc := &tax.Combo{
					Category: tax.CategoryVAT,
					Ext:      tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyExempt: code}),
				}
				norm.Normalize(tc, tax.AddonContext(sdi.V1))
				assert.Equal(t, code, tc.Ext.Get(sdi.ExtKeyExempt), "extension should be set correctly")
				assert.NotEmpty(t, tc.Key, "key should be set based on extension")
				if tc.Key == tax.KeyZero {
					assert.NotNil(t, tc.Percent, "percent should be set for zero key")
					assert.Equal(t, int64(0), tc.Percent.Value())
				}
			})
		}
	})

	t.Run("check exempt handling", func(t *testing.T) {
		tc := &tax.Combo{
			Category: tax.CategoryVAT,
			Key:      tax.KeyExempt,
			Ext:      tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyExempt: "N3.2"}),
		}
		norm.Normalize(tc, tax.AddonContext(sdi.V1))
		assert.Equal(t, "N3.2", tc.Ext.Get(sdi.ExtKeyExempt).String())
		assert.Equal(t, tax.KeyIntraCommunity, tc.Key)
	})

}

// SDI rejects split payment on reverse charge VAT with check 00420.
func TestTaxComboSplitPaymentValidation(t *testing.T) {
	const msg = "split payment VAT cannot use a reverse charge (N6) nature (SDI check 00420)"

	t.Run("refuses split payment on reverse charge VAT", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes[0] = &tax.Combo{
			Category: tax.CategoryVAT,
			Key:      tax.KeyReverseCharge,
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				sdi.ExtKeyVATLiability: sdi.ExtCodeVATLiabilitySplitPayment,
			}),
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), msg)
	})

	t.Run("allows reverse charge VAT without split payment", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes[0] = &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyReverseCharge}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		if err != nil {
			assert.NotContains(t, err.Error(), msg)
		}
	})

	t.Run("allows split payment on standard VAT", func(t *testing.T) {
		inv := testInvoiceSplitPayment(t)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}
