package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl.it.sdi/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentsSimple(t *testing.T) {
	t.Run("should contain the supplier party info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]

		require.NotNil(t, dp)
		assert.Equal(t, "TP02", dp.Conditions)
		assert.Len(t, dp.Payments, 1)
		assert.Equal(t, "MP08", dp.Payments[0].Method)
		assert.Equal(t, "1388.40", dp.Payments[0].Amount)
	})
}

func TestPaymentsSimpleIBAN(t *testing.T) {
	t.Run("should contain the supplier party info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple-iban.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]

		require.NotNil(t, dp)
		assert.Equal(t, "TP02", dp.Conditions)
		assert.Len(t, dp.Payments, 1)
		assert.Equal(t, "MP05", dp.Payments[0].Method)
		assert.Equal(t, "1388.40", dp.Payments[0].Amount)
		assert.Equal(t, "IT60X0542811101000000123456", dp.Payments[0].IBAN)
		assert.Equal(t, "BCITITMM", dp.Payments[0].BIC)
	})
}

func TestPayments(t *testing.T) {
	t.Run("multiple due dates", func(t *testing.T) {
		env := test.LoadTestFile("invoice-irpef.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]

		require.NotNil(t, dp)
		assert.Equal(t, "TP01", dp.Conditions)
		assert.Len(t, dp.Payments, 2)
		assert.Equal(t, "MP05", dp.Payments[0].Method)
		assert.Equal(t, "2023-03-02", dp.Payments[0].DueDate)
		assert.Equal(t, "500.00", dp.Payments[0].Amount)
		assert.Equal(t, "MP05", dp.Payments[1].Method)
		assert.Equal(t, "2023-04-02", dp.Payments[1].DueDate)
		assert.Equal(t, "544.40", dp.Payments[1].Amount)
	})

	t.Run("advance payment", func(t *testing.T) {
		env := test.LoadTestFile("invoice-hotel-private.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]

		require.NotNil(t, dp)
		assert.Equal(t, "TP03", dp.Conditions)
		assert.Len(t, dp.Payments, 1)
		assert.Equal(t, "MP08", dp.Payments[0].Method)
		assert.Equal(t, "29.00", dp.Payments[0].Amount)
	})

	t.Run("prepaid", func(t *testing.T) {
		env := test.LoadTestFile("invoice-hotel.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)

		dp := doc.Body[0].PaymentsData[0]

		require.NotNil(t, dp)
		assert.Equal(t, "TP03", dp.Conditions)
		assert.Len(t, dp.Payments, 1)
		assert.Equal(t, "MP08", dp.Payments[0].Method)
		assert.Equal(t, "241.00", dp.Payments[0].Amount)
	})
}
