package fatturapa

import (
	"testing"
	"unicode/utf8"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestFindRiferimentoNormativo(t *testing.T) {
	// RiferimentoNormativo is a String100LatinType in the FatturaPA schema.
	t.Run("fits the schema limit for every exemption code", func(t *testing.T) {
		def := tax.ExtensionForKey(sdi.ExtKeyExempt)
		for _, c := range def.Values {
			ref := findRiferimentoNormativo(rateTotalWithNature(c.Code))
			assert.NotEmpty(t, ref, c.Code)
			assert.LessOrEqual(t, utf8.RuneCountInString(ref), 100, "%s: %s", c.Code, ref)
		}
	})

	t.Run("abbreviates N7", func(t *testing.T) {
		assert.Equal(t, "IVA assolta in altro stato UE ex art. 7-octies lett. a, b, art. 74-sexies DPR 633/72", findRiferimentoNormativo(rateTotalWithNature("N7")))
	})

	t.Run("keeps the official description when it fits", func(t *testing.T) {
		assert.Equal(t, "Non soggette - altri casi", findRiferimentoNormativo(rateTotalWithNature("N2.2")))
	})
}

func rateTotalWithNature(code cbc.Code) *tax.RateTotal {
	return &tax.RateTotal{Ext: tax.MakeExtensions().Set(sdi.ExtKeyExempt, code)}
}
