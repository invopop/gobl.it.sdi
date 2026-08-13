package fatturapa

import (
	"testing"

	"github.com/invopop/gobl/cbc"
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
