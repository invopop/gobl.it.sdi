package fatturapa_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/pay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentsInConversion(t *testing.T) {
	t.Run("should convert simple payment correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check payment data
		require.NotNil(t, invoice.Payment)

		// Check payment terms
		require.NotNil(t, invoice.Payment.Terms)
		assert.Equal(t, pay.TermKeyPending, invoice.Payment.Terms.Key)

		// Check payment instructions
		require.NotNil(t, invoice.Payment.Instructions)
		assert.Equal(t, pay.MeansKeyCard, invoice.Payment.Instructions.Key)
	})

	t.Run("should convert payment with IBAN correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple-iban.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check payment data
		require.NotNil(t, invoice.Payment)

		// Check payment terms
		require.NotNil(t, invoice.Payment.Terms)
		assert.Equal(t, pay.TermKeyPending, invoice.Payment.Terms.Key)

		// Check payment instructions
		require.NotNil(t, invoice.Payment.Instructions)
		assert.Equal(t, pay.MeansKeyCreditTransfer, invoice.Payment.Instructions.Key)

		// Check credit transfer
		require.NotEmpty(t, invoice.Payment.Instructions.CreditTransfer)
		creditTransfer := invoice.Payment.Instructions.CreditTransfer[0]
		assert.Equal(t, cbc.Code("IT60X0542811101000000123456"), creditTransfer.IBAN)
		assert.Equal(t, cbc.Code("BCITITMM"), creditTransfer.BIC)
	})

	t.Run("should convert multiple due dates correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-irpef.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check payment data
		require.NotNil(t, invoice.Payment)

		// Check payment terms
		require.NotNil(t, invoice.Payment.Terms)
		assert.Equal(t, pay.TermKeyDueDate, invoice.Payment.Terms.Key)

		// Check due dates
		require.NotEmpty(t, invoice.Payment.Terms.DueDates)
		require.Len(t, invoice.Payment.Terms.DueDates, 2)

		// Check first due date
		dueDate1 := invoice.Payment.Terms.DueDates[0]
		require.NotNil(t, dueDate1.Date)
		assert.Equal(t, "2023-03-02", dueDate1.Date.String())
		assert.True(t, dueDate1.Amount.Compare(num.MakeAmount(50000, 2)) == 0, "Amount should be 500.00")

		// Check second due date
		dueDate2 := invoice.Payment.Terms.DueDates[1]
		require.NotNil(t, dueDate2.Date)
		assert.Equal(t, "2023-04-02", dueDate2.Date.String())
		assert.True(t, dueDate2.Amount.Compare(num.MakeAmount(54440, 2)) == 0, "Amount should be 544.40")

		// Check payment instructions
		require.NotNil(t, invoice.Payment.Instructions)
		assert.Equal(t, pay.MeansKeyCreditTransfer, invoice.Payment.Instructions.Key)
	})

	t.Run("should convert advance payment correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-hotel-private.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check payment data
		require.NotNil(t, invoice.Payment)

		// Check payment terms
		require.NotNil(t, invoice.Payment.Terms)
		assert.Equal(t, pay.TermKeyDueDate, invoice.Payment.Terms.Key)

		// Check advances
		require.NotEmpty(t, invoice.Payment.Advances)
		require.Len(t, invoice.Payment.Advances, 1)

		// Check advance
		advance := invoice.Payment.Advances[0]
		assert.True(t, advance.Amount.Compare(num.MakeAmount(2900, 2)) == 0, "Amount should be 29.00")
		assert.Equal(t, pay.MeansKeyCard, advance.Key)
	})
}
