package fatturapa

import (
	"testing"

	"github.com/invopop/gobl/num"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateFromIncludedVAT(t *testing.T) {
	rate := func(t *testing.T, amount, vat string) string {
		t.Helper()
		a, err := num.AmountFromString(amount)
		require.NoError(t, err)
		v, err := num.AmountFromString(vat)
		require.NoError(t, err)
		r, err := rateFromIncludedVAT(a, v)
		require.NoError(t, err)
		return r
	}

	t.Run("Italian rates", func(t *testing.T) {
		assert.Equal(t, "22.00", rate(t, "122.00", "22.00"))
		assert.Equal(t, "10.00", rate(t, "11.00", "1.00"))
		assert.Equal(t, "5.00", rate(t, "10.50", "0.50"))
		assert.Equal(t, "4.00", rate(t, "10.40", "0.40"))
	})

	t.Run("small amounts", func(t *testing.T) {
		assert.Equal(t, "22.00", rate(t, "0.13", "0.02"))
	})

	t.Run("negative amounts", func(t *testing.T) {
		assert.Equal(t, "22.00", rate(t, "-24.40", "-4.40"))
	})

	t.Run("rate outside the Italian ones", func(t *testing.T) {
		assert.Equal(t, "7.00", rate(t, "107.00", "7.00"))
	})

	t.Run("zero VAT", func(t *testing.T) {
		_, err := rateFromIncludedVAT(num.MakeAmount(1000, 2), num.MakeAmount(0, 2))
		assert.ErrorContains(t, err, "zero VAT without a rate or exemption")
	})
}
