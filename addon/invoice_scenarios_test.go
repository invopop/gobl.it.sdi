package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scenarioInvoice(t *testing.T) *bill.Invoice {
	t.Helper()
	return &bill.Invoice{
		Addons:    tax.WithAddons(sdi.V1),
		Series:    "TEST",
		Code:      "00123",
		IssueDate: cal.MakeDate(2022, 6, 13),
		Tax: &bill.Tax{
			PricesInclude: tax.CategoryVAT,
		},
		Supplier: &org.Party{
			Name:  "Test Supplier",
			TaxID: &tax.Identity{Country: "IT", Code: "12345678903"},
		},
		Customer: &org.Party{
			Name:  "Test Customer",
			TaxID: &tax.Identity{Country: "IT", Code: "13029381004"},
		},
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(10, 0),
				Item:     &org.Item{Name: "Test Item", Price: num.NewAmount(10000, 2)},
				Taxes:    tax.Set{{Category: "VAT", Rate: "general"}},
			},
		},
	}
}

func TestInvoiceScenarioExtensions(t *testing.T) {
	t.Run("document type defaults to TD01", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.Tax = nil
		require.NoError(t, inv.Calculate())
		assert.Equal(t, 2, inv.Tax.Ext.Len())
		assert.Equal(t, "TD01", inv.Tax.Ext.Get(sdi.ExtKeyDocumentType).String())
	})

	t.Run("B2G overwrites the format to FPA12", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.SetTags(tax.TagB2G)
		inv.Tax = &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFormat: "XXXX"}),
		}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, 2, inv.Tax.Ext.Len())
		assert.Equal(t, "FPA12", inv.Tax.Ext.Get(sdi.ExtKeyFormat).String())
	})

	t.Run("non-B2G overwrites the format to FPR12", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.Tax = &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFormat: "XXXX"}),
		}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, 2, inv.Tax.Ext.Len())
		assert.Equal(t, "FPR12", inv.Tax.Ext.Get(sdi.ExtKeyFormat).String())
	})
}

func TestInvoiceGetExtensions(t *testing.T) {
	inv := scenarioInvoice(t)
	require.NoError(t, inv.Calculate())
	ext := inv.GetExtensions()
	assert.Len(t, ext, 2)
	assert.Equal(t, "FPR12", ext[0].Get(sdi.ExtKeyFormat).String())
}
