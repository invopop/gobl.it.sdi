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

	t.Run("simplified overwrites the format to FSM10", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.SetTags(tax.TagSimplified)
		inv.Tax = &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFormat: "XXXX"}),
		}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "FSM10", inv.Tax.Ext.Get(sdi.ExtKeyFormat).String())
		assert.Equal(t, "TD07", inv.Tax.Ext.Get(sdi.ExtKeyDocumentType).String())
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

func TestInvoiceScenarioIssuerType(t *testing.T) {
	tests := []struct {
		tags    []cbc.Key
		docType cbc.Code
		want    cbc.Code
	}{
		{[]cbc.Key{tax.TagSelfBilled, tax.TagReverseCharge}, "TD16", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagImport}, "TD17", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagImport, sdi.TagGoodsEU}, "TD18", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagImport, sdi.TagGoods}, "TD19", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagRegularization}, "TD20", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagGoodsExtracted}, "TD22", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagGoodsWithTax}, "TD23", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagSanMarinoPaper}, "TD28", sdi.ExtCodeIssuerTypeCustomer},
		{[]cbc.Key{tax.TagSelfBilled, sdi.TagCeilingExceeded}, "TD21", ""},
		{[]cbc.Key{tax.TagSelfBilled}, "TD27", ""},
	}
	for _, ts := range tests {
		t.Run(ts.docType.String(), func(t *testing.T) {
			inv := scenarioInvoice(t)
			inv.SetTags(ts.tags...)
			require.NoError(t, inv.Calculate())
			assert.Equal(t, ts.docType, inv.Tax.Ext.Get(sdi.ExtKeyDocumentType))
			assert.Equal(t, ts.want, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
		})
	}

	t.Run("TD17 replaces an explicit TZ with CC", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.SetTags(tax.TagSelfBilled, sdi.TagImport)
		inv.Tax.Ext = tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyIssuerType: sdi.ExtCodeIssuerTypeThirdParty})
		require.NoError(t, inv.Calculate())
		assert.Equal(t, sdi.ExtCodeIssuerTypeCustomer, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
	})

	t.Run("third-party issuer on TD27 gets TZ", func(t *testing.T) {
		inv := scenarioInvoice(t)
		inv.SetTags(tax.TagSelfBilled)
		inv.Ordering = &bill.Ordering{
			Issuer: &org.Party{
				Name:  "Fatturazione Terzi S.r.l.",
				TaxID: &tax.Identity{Country: "IT", Code: "01234567897"},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, sdi.ExtCodeIssuerTypeThirdParty, inv.Tax.Ext.Get(sdi.ExtKeyIssuerType))
	})
}

func TestInvoiceGetExtensions(t *testing.T) {
	inv := scenarioInvoice(t)
	require.NoError(t, inv.Calculate())
	ext := inv.GetExtensions()
	assert.Len(t, ext, 2)
	assert.Equal(t, "FPR12", ext[0].Get(sdi.ExtKeyFormat).String())
}
