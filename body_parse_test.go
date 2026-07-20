package fatturapa_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.fatturapa/test"
	"github.com/invopop/gobl/addons/it/sdi"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyInConversion(t *testing.T) {
	t.Run("should convert standard invoice body correctly", func(t *testing.T) {
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

		// Check invoice type and tags
		assert.Equal(t, bill.InvoiceTypeStandard, invoice.Type)
		assert.False(t, invoice.Tags.HasTags(tax.TagPartial), "Invoice should not be tagged as partial")
		assert.False(t, invoice.Tags.HasTags(tax.TagSimplified), "Invoice should not be tagged as simplified")
		assert.False(t, invoice.Tags.HasTags(tax.TagSelfBilled), "Invoice should not be tagged as self-billed")

		// Check invoice code
		assert.Equal(t, cbc.Code("SAMPLE-001"), invoice.Code)

		// Check invoice currency
		assert.Equal(t, currency.Code("EUR"), invoice.Currency)

		// Check invoice issue date
		require.NotNil(t, invoice.IssueDate)
		assert.Equal(t, "2023-03-02", invoice.IssueDate.String())
	})

	t.Run("should convert credit note body correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-credit-note.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check invoice type
		assert.Equal(t, bill.InvoiceTypeCreditNote, invoice.Type)

		// Check document references
		if len(invoice.Preceding) > 0 {
			// Find the corrective reference
			var correctiveRef *org.DocumentRef
			for _, ref := range invoice.Preceding {
				if ref.Type == "corrective" {
					correctiveRef = ref
					break
				}
			}

			if correctiveRef != nil {
				assert.Equal(t, "2021-01", correctiveRef.Series.String())
				assert.Equal(t, "123", correctiveRef.Code.String())
			}
		}
	})

	t.Run("should convert B2G invoice body correctly", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-b2g.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)
		require.NotNil(t, env)

		// Extract the invoice
		invoice, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, invoice)

		// Check that the invoice is tagged as B2G
		assert.True(t, invoice.Tags.HasTags(tax.TagB2G), "Invoice should be tagged as B2G")

		// Check tax extensions for transmission format
		require.NotNil(t, invoice.Tax)
		require.NotNil(t, invoice.Tax.Ext)
		assert.Equal(t, cbc.Code("FPA12"), invoice.Tax.Ext.Get(sdi.ExtKeyFormat))
	})

	t.Run("should convert invoice with stamp duty correctly", func(t *testing.T) {
		// Load the XML file
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

		// Check stamp duty
		require.NotNil(t, invoice.Tax)
		require.False(t, invoice.Tax.Ext.IsZero())

		if stampDuty := invoice.Tax.Ext.Get("it-sdi-stamp-duty"); stampDuty != "" {
			assert.Equal(t, cbc.Code("SI"), stampDuty)
		}
	})

	t.Run("should check totals are correct when no rounding is present", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv)

		require.Nil(t, inv.Totals.Rounding)
	})

	t.Run("should check totals are correct when rounding exists but not defined", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-simple-rounding.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv)
		require.NotNil(t, inv.Totals)
		require.NotNil(t, inv.Totals.Rounding)
		require.Equal(t, inv.Totals.Rounding.String(), "0.01")
		require.Equal(t, "1388.41", inv.Totals.Payable.String())

	})

	t.Run("should check totals are correct when rounding exists and defined", func(t *testing.T) {
		// Load the XML file
		data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathGOBLFatturaPA), "/out/invoice-simple-rounding.xml"))
		require.NoError(t, err)

		// Convert XML to GOBL
		env, err := test.ConvertToGOBL(data)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv)
		require.NotNil(t, inv.Totals)
		require.NotNil(t, inv.Totals.Rounding)
		require.Equal(t, inv.Totals.Rounding.String(), "0.01")
		require.Equal(t, "1388.41", inv.Totals.Payable.String())

	})
}
