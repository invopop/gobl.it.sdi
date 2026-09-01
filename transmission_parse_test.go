package fatturapa_test

import (
	"os"
	"path/filepath"
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransmissionInConversion(t *testing.T) {
	t.Run("should convert transmission data correctly", func(t *testing.T) {
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

		// Check tax extensions for transmission format
		require.NotNil(t, invoice.Tax)
		require.False(t, invoice.Tax.Ext.IsZero())
		assert.Equal(t, cbc.Code("FPR12"), invoice.Tax.Ext.Get(sdi.ExtKeyFormat))

		// Check customer inboxes for recipient code
		require.NotNil(t, invoice.Customer)
		require.NotNil(t, invoice.Customer.Inboxes)

		// Find the inbox with the recipient code
		var recipientCodeInbox *org.Inbox
		for _, inbox := range invoice.Customer.Inboxes {
			if inbox.Key == sdi.KeyInboxCode {
				recipientCodeInbox = inbox
				break
			}
		}

		require.NotNil(t, recipientCodeInbox, "Recipient code inbox should exist")
		assert.Equal(t, cbc.Code("ABCDEF1"), recipientCodeInbox.Code)
	})

	t.Run("should convert transmission data with PEC correctly", func(t *testing.T) {
		// Load the XML file
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

		// Check tax extensions for transmission format
		require.NotNil(t, invoice.Tax)
		require.False(t, invoice.Tax.Ext.IsZero())
		assert.Equal(t, cbc.Code("FPR12"), invoice.Tax.Ext.Get(sdi.ExtKeyFormat))

		// Check customer inboxes for PEC
		require.NotNil(t, invoice.Customer)
		require.NotNil(t, invoice.Customer.Inboxes)

		// Find the inbox with the PEC
		var pecInbox *org.Inbox
		for _, inbox := range invoice.Customer.Inboxes {
			if inbox.Key == sdi.KeyInboxPEC {
				pecInbox = inbox
				break
			}
		}

		require.NotNil(t, pecInbox, "PEC inbox should exist")
		assert.Equal(t, "fooo@inbox.com", pecInbox.Email)
	})

	t.Run("should convert B2G transmission data correctly", func(t *testing.T) {
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

		// Check tax extensions for transmission format
		require.NotNil(t, invoice.Tax)
		require.False(t, invoice.Tax.Ext.IsZero())
		assert.Equal(t, cbc.Code("FPA12"), invoice.Tax.Ext.Get(sdi.ExtKeyFormat))

		// Check that the invoice is tagged as B2G
		assert.True(t, invoice.Tags.HasTags(tax.TagB2G), "Invoice should be tagged as B2G")
	})
}
