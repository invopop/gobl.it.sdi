package fatturapa

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindCodeTaxType(t *testing.T) {
	tests := []struct {
		cat  cbc.Code
		want string
	}{
		{it.TaxCategoryIRPEF, "RT01"},
		{it.TaxCategoryIRES, "RT02"},
		{it.TaxCategoryINPS, "RT03"},
		{it.TaxCategoryENASARCO, "RT04"},
		{it.TaxCategoryENPAM, "RT05"},
		{it.TaxCategoryCP, "RT06"},
	}
	for _, test := range tests {
		t.Run(test.cat.String(), func(t *testing.T) {
			code, err := findCodeTaxType(test.cat)
			require.NoError(t, err)
			assert.Equal(t, test.want, code)
		})
	}

	t.Run("category without a mapping", func(t *testing.T) {
		_, err := findCodeTaxType(tax.CategoryVAT)
		assert.ErrorContains(t, err, "could not find TipoRitenuta code for tax category VAT")
	})

	t.Run("unknown category", func(t *testing.T) {
		_, err := findCodeTaxType("FOO")
		assert.ErrorContains(t, err, "could not find TipoRitenuta code for tax category FOO")
	})
}

func TestConvertRetainedTaxType(t *testing.T) {
	tests := []struct {
		code string
		want cbc.Code
	}{
		{"RT01", it.TaxCategoryIRPEF},
		{"RT02", it.TaxCategoryIRES},
		{"RT03", it.TaxCategoryINPS},
		{"RT04", it.TaxCategoryENASARCO},
		{"RT05", it.TaxCategoryENPAM},
		{"RT06", it.TaxCategoryCP},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			cat, err := convertRetainedTaxType(test.code)
			require.NoError(t, err)
			assert.Equal(t, test.want, cat)
		})
	}

	t.Run("unknown code", func(t *testing.T) {
		_, err := convertRetainedTaxType("RT99")
		assert.ErrorContains(t, err, "unknown TipoRitenuta code: RT99")
	})

	t.Run("empty code", func(t *testing.T) {
		_, err := convertRetainedTaxType("")
		assert.ErrorContains(t, err, "unknown TipoRitenuta code")
	})
}

// Every retained category must map to a TipoRitenuta code, and each code back
// to the category it came from.
func TestRetainedTaxTypesRoundTrip(t *testing.T) {
	for _, def := range itRegime.Categories {
		if !def.Retained {
			continue
		}
		t.Run(def.Code.String(), func(t *testing.T) {
			code, err := findCodeTaxType(def.Code)
			require.NoError(t, err)

			cat, err := convertRetainedTaxType(code)
			require.NoError(t, err)
			assert.Equal(t, def.Code, cat, "%s is mapped by more than one category", code)
		})
	}
}

func TestEffectiveRate(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		base   string
		want   string
	}{
		{"exact at two decimals", "263.93", "2295.00", "11.50%"},
		// 12.01 / 120.00 = 10.0083..%: the quotient rounds up to the
		// candidate that reproduces the amount.
		{"rounds the quotient to the nearest candidate", "12.01", "120.00", "10.01%"},
		{"exact at three decimals", "123.45", "1000.00", "12.345%"},
		{"exact at four decimals", "768.92", "6686.21", "11.5001%"},
		{"declared rate reproduces", "200.00", "1000.00", "20.00%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, err := num.AmountFromString(tt.amount)
			require.NoError(t, err)
			base, err := num.AmountFromString(tt.base)
			require.NoError(t, err)

			p := effectiveRate(amount, base)
			require.NotNil(t, p)
			assert.Equal(t, tt.want, p.String())
			assert.True(t, p.Of(base).Equals(amount))
		})
	}

	t.Run("returns nil for a zero base", func(t *testing.T) {
		assert.Nil(t, effectiveRate(num.MakeAmount(100, 2), num.MakeAmount(0, 2)))
	})

	t.Run("returns nil beyond the precision bound", func(t *testing.T) {
		// 0.01 on 10,000,000.00 needs a seven-decimal rate.
		assert.Nil(t, effectiveRate(num.MakeAmount(1, 2), num.MakeAmount(1000000000, 2)))
	})
}
