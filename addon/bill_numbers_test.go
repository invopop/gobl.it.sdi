package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The 20-character boundary of FatturaPA's String20Type.
const (
	twenty     = "SAMPLE-1234567890123"
	twentyOne  = "SAMPLE-12345678901234"
	codeOf20   = "12345678901234567890"
	codeOf21   = "123456789012345678901"
	seriesName = "SAMPLE"
)

func TestDocumentNumberLength(t *testing.T) {
	t.Run("numbers of 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Series = seriesName
		inv.Code = "1234567890123"
		inv.Preceding = []*org.DocumentRef{{Code: codeOf20, IssueDate: cal.NewDate(2022, 6, 1)}}
		inv.Ordering = &bill.Ordering{Purchases: []*org.DocumentRef{{Code: codeOf20}}}
		inv.Supplier.Registration = &org.Registration{Office: "RM", Entry: codeOf20}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, twenty, inv.Series.Join(inv.Code).String())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("invoice number over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Series = seriesName
		inv.Code = "12345678901234"
		require.NoError(t, inv.Calculate())
		require.Equal(t, twentyOne, inv.Series.Join(inv.Code).String())
		assert.ErrorContains(t, rules.Validate(inv), "invoice number (series and code) must be 20 ASCII characters or fewer")
	})

	t.Run("invoice code over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Code = codeOf21
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "invoice number (series and code) must be 20 ASCII characters or fewer")
	})

	t.Run("preceding number over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Preceding = []*org.DocumentRef{{Series: seriesName, Code: "12345678901234"}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "preceding document number and item reference must be 20 ASCII characters or fewer")
	})

	t.Run("preceding item reference over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Preceding = []*org.DocumentRef{{
			Code:       "FT-1",
			Identities: []*org.Identity{{Key: org.IdentityKeyItem, Code: codeOf21}},
		}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "preceding document number and item reference must be 20 ASCII characters or fewer")
	})

	t.Run("first of two item references over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Preceding = []*org.DocumentRef{{
			Code: "FT-1",
			Identities: []*org.Identity{
				{Key: org.IdentityKeyItem, Code: codeOf21},
				{Key: org.IdentityKeyItem, Code: "1"},
			},
		}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("second item reference over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Preceding = []*org.DocumentRef{{
			Code: "FT-1",
			Identities: []*org.Identity{
				{Key: org.IdentityKeyItem, Code: "1"},
				{Key: org.IdentityKeyItem, Code: codeOf21},
			},
		}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "preceding document number and item reference must be 20 ASCII characters or fewer")
	})

	t.Run("ordering number over 20 characters", func(t *testing.T) {
		for name, ordering := range map[string]*bill.Ordering{
			"purchase":  {Purchases: []*org.DocumentRef{{Code: codeOf21}}},
			"contract":  {Contracts: []*org.DocumentRef{{Code: codeOf21}}},
			"tender":    {Tender: []*org.DocumentRef{{Code: codeOf21}}},
			"receiving": {Receiving: []*org.DocumentRef{{Code: codeOf21}}},
		} {
			inv := testInvoiceStandard(t)
			inv.Ordering = ordering
			require.NoError(t, inv.Calculate())
			assert.ErrorContains(t, rules.Validate(inv), "ordering document numbers and item references must be 20 ASCII characters or fewer", name)
		}
	})

	t.Run("despatch item reference over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.SetTags(sdi.TagDeferred)
		inv.Ordering = &bill.Ordering{Despatch: []*org.DocumentRef{{
			Code:       "DDT-1",
			IssueDate:  cal.NewDate(2022, 6, 1),
			Identities: []*org.Identity{{Key: org.IdentityKeyItem, Code: codeOf21}},
		}}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("despatch number over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.SetTags(sdi.TagDeferred)
		inv.Ordering = &bill.Ordering{Despatch: []*org.DocumentRef{{Code: codeOf21, IssueDate: cal.NewDate(2022, 6, 1)}}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "ordering document numbers and item references must be 20 ASCII characters or fewer")
	})

	t.Run("registration entry over 20 characters", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Registration = &org.Registration{Office: "RM", Entry: codeOf21}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "supplier registration entry must be 20 ASCII characters or fewer")
	})

	t.Run("registration entry outside ASCII", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Registration = &org.Registration{Office: "RM", Entry: "N°123456"}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "supplier registration entry must be 20 ASCII characters or fewer")
	})

	t.Run("simplified invoice with long ordering and preceding numbers", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Preceding = []*org.DocumentRef{{Code: codeOf21}}
		inv.Ordering = &bill.Ordering{Purchases: []*org.DocumentRef{{Code: codeOf21}}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("simplified note with a long item reference", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].Identities = []*org.Identity{{Key: org.IdentityKeyItem, Code: codeOf21}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("simplified note with a preceding number over 20 characters", func(t *testing.T) {
		inv := testInvoiceSimplified(t)
		inv.Type = bill.InvoiceTypeCreditNote
		inv.Preceding = simplifiedPreceding()
		inv.Preceding[0].Code = codeOf21
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "simplified invoice preceding number must be 20 ASCII characters or fewer")
	})
}

func TestFundContributionCodeLength(t *testing.T) {
	charge := func(code cbc.Code) *bill.Charge {
		return &bill.Charge{
			Key:     sdi.KeyFundContribution,
			Code:    code,
			Percent: num.NewPercentage(4, 2),
			Taxes:   tax.Set{{Category: tax.CategoryVAT, Rate: "general", Percent: num.NewPercentage(22, 2)}},
			Ext:     tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyFundType: "TC04"}),
		}
	}

	t.Run("code of 20 characters", func(t *testing.T) {
		assert.NoError(t, rules.Validate(charge(codeOf20), withSDIContext()))
	})

	t.Run("code over 20 characters", func(t *testing.T) {
		assert.ErrorContains(t, rules.Validate(charge(codeOf21), withSDIContext()),
			"fund contribution charge code must be 20 ASCII characters or fewer")
	})
}
