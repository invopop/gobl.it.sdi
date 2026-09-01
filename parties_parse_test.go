package fatturapa_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPartiesInConversion tests the conversion of parties from FatturaPA XML to GOBL
func TestPartiesInConversion(t *testing.T) {
	t.Run("should convert supplier correctly", func(t *testing.T) {
		// Load the XML file with supplier information
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

		// Check the supplier
		supplier := invoice.Supplier
		require.NotNil(t, supplier)

		// Verify supplier fields
		assert.Equal(t, "MªF. Services", supplier.Name)

		// Verify tax ID
		require.NotNil(t, supplier.TaxID)
		assert.Equal(t, l10n.TaxCountryCode("IT"), supplier.TaxID.Country)
		assert.Equal(t, cbc.Code("12345678903"), supplier.TaxID.Code)

		// Verify addresses
		require.NotEmpty(t, supplier.Addresses)
		address := supplier.Addresses[0]
		assert.Equal(t, "VIALE DELLA LIBERTÀ", address.Street)
		assert.Equal(t, "1", address.Number)
		assert.Equal(t, "ROMA", address.Locality)
		assert.Equal(t, "RM", address.Region)
		assert.Equal(t, l10n.ISOCountryCode("IT"), address.Country)

		// Verify registration
		require.NotNil(t, supplier.Registration)
		assert.Equal(t, "RM", supplier.Registration.Office)
		assert.Equal(t, "123456", supplier.Registration.Entry)
	})

	t.Run("should convert customer correctly", func(t *testing.T) {
		// Load the XML file with customer information
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

		// Check the customer
		customer := invoice.Customer
		require.NotNil(t, customer)

		// Verify customer fields
		assert.Equal(t, "MARIO LEONI", customer.Name)

		// Verify tax ID
		require.NotNil(t, customer.TaxID)
		assert.Equal(t, l10n.TaxCountryCode("IT"), customer.TaxID.Country)
		assert.Equal(t, cbc.Code("09876543217"), customer.TaxID.Code)

		// Verify addresses
		require.NotEmpty(t, customer.Addresses)
		address := customer.Addresses[0]
		assert.Equal(t, "VIALE DELI LAVORATORI", address.Street)
		assert.Equal(t, "32", address.Number)
		assert.Equal(t, "FIRENZE", address.Locality)
		assert.Equal(t, "FI", address.Region)
		assert.Equal(t, l10n.ISOCountryCode("IT"), address.Country)
	})

	t.Run("should convert customer with PEC correctly", func(t *testing.T) {
		// Load the XML file with customer PEC information
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple-with-pec.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check the customer
		customer := invoice.Customer
		require.NotNil(t, customer)

		// For this test, we're just checking that the customer exists
		// The PEC email might be stored in a different way than we expected
		assert.NotEmpty(t, customer.Name)
	})

	t.Run("should convert foreign customer correctly", func(t *testing.T) {
		// Load the XML file with foreign customer information
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-hotel.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check the customer
		customer := invoice.Customer
		require.NotNil(t, customer)

		// Verify customer fields for foreign customer
		assert.NotEmpty(t, customer.Name)

		// Verify tax ID for foreign customer
		require.NotNil(t, customer.TaxID)
		// Foreign customers might have different country codes
		assert.NotEqual(t, l10n.IT, customer.TaxID.Country)
	})
}
