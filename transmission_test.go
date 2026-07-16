package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl.fatturapa/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransmissionData(t *testing.T) {
	t.Run("should contain transmitting subject info", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env, test.LoadOptions()...)
		require.NoError(t, err)

		dt := doc.Header.TransmissionData

		assert.Equal(t, "IT", dt.TransmitterID.Country)
		assert.Equal(t, "01234567890", dt.TransmitterID.Code)
		assert.Equal(t, "679a2f25", dt.ProgressiveNumber)
		assert.Equal(t, "FPR12", dt.TransmissionFormat)
		assert.Equal(t, "ABCDEF1", dt.RecipientCode)
	})

	t.Run("should skip transmitter info and only include codice destinatario if transmitter is not present", func(t *testing.T) {

		opts := test.LoadOptionsWithoutTransmitter()

		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env, opts...)
		require.NoError(t, err)

		dt := doc.Header.TransmissionData

		assert.Equal(t, "ABCDEF1", dt.RecipientCode)
		assert.Nil(t, dt.TransmitterID)
		assert.Equal(t, "", dt.ProgressiveNumber)
		assert.Equal(t, "", dt.TransmissionFormat)
	})

	t.Run("should set codice destinatario to 0000000 if customer is Italian with PEC", func(t *testing.T) {

		env := test.LoadTestFile("invoice-simple-with-pec.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env, test.LoadOptions()...)
		require.NoError(t, err)

		dt := doc.Header.TransmissionData

		assert.Equal(t, "0000000", dt.RecipientCode)
		assert.Equal(t, "fooo@inbox.com", dt.RecipientPEC)
	})

	t.Run("should ignore the PEC when the customer also has a codice destinatario", func(t *testing.T) {

		env := test.LoadTestFile("invoice-simple-with-code-and-pec.json", test.PathGOBLFatturaPA)
		doc, err := test.ConvertFromGOBL(env, test.LoadOptions()...)
		require.NoError(t, err)

		dt := doc.Header.TransmissionData

		assert.Equal(t, "ABC1234", dt.RecipientCode)
		assert.Equal(t, "", dt.RecipientPEC)
	})
}
