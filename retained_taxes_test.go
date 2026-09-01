package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl.it.sdi/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatiRitenuta(t *testing.T) {
	t.Run("when retained taxes are NOT present", func(t *testing.T) {
		t.Run("should be empty", func(t *testing.T) {
			env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			assert.Empty(t, dr)
		})
	})

	t.Run("when retained taxes are present", func(t *testing.T) {
		t.Run("should contain the correct retainted taxes", func(t *testing.T) {
			env := test.LoadTestFile("invoice-irpef.json", test.PathGOBLFatturaPA)
			doc, err := test.ConvertFromGOBL(env)
			require.NoError(t, err)

			dr := doc.Body[0].GeneralData.Document.RetainedTaxes

			require.Len(t, dr, 2)

			assert.Equal(t, "RT01", dr[0].Type)
			assert.Equal(t, "324.00", dr[0].Amount)
			assert.Equal(t, "20.00", dr[0].Rate)
			assert.Equal(t, "A", dr[0].Reason)

			assert.Equal(t, "RT01", dr[1].Type)
			assert.Equal(t, "50.00", dr[1].Amount)
			assert.Equal(t, "50.00", dr[1].Rate)
			assert.Equal(t, "I", dr[1].Reason)
		})
	})
}
