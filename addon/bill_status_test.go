package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeStatusFromNotification(t *testing.T) {
	cases := []struct {
		name   string
		code   cbc.Code // it-sdi-notification
		format cbc.Code // it-sdi-format, only consulted for MC
		want   cbc.Key  // expected line.Key
		typ    cbc.Key  // expected Status.Type
	}{
		{"RC", "RC", "", bill.StatusLineAcknowledged, bill.StatusTypeUpdate},
		{"AT", "AT", "", bill.StatusLineError, bill.StatusTypeUpdate},
		{"DT", "DT", "", bill.StatusLineAccepted, bill.StatusTypeUpdate},
		{"MC B2B", "MC", "FPR12", bill.StatusLineAcknowledged, bill.StatusTypeUpdate},
		{"MC PA", "MC", "FPA12", bill.StatusLineProcessing, bill.StatusTypeUpdate},
		{"EC01", "EC01", "", bill.StatusLineAccepted, bill.StatusTypeResponse},
		{"EC02", "EC02", "", bill.StatusLineRejected, bill.StatusTypeResponse},
		{"NS", "NS", "", bill.StatusLineError, bill.StatusTypeUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := cbc.CodeMap{sdi.ExtKeyNotification: tc.code}
			if tc.format != "" {
				ext[sdi.ExtKeyFormat] = tc.format
			}
			st := &bill.Status{
				Addons: tax.WithAddons(sdi.V1),
				Lines:  []*bill.StatusLine{{Ext: tax.ExtensionsOf(ext)}},
			}
			norm.Normalize(st, tax.AddonContext(sdi.V1))
			assert.Equal(t, tc.want, st.Lines[0].Key)
			assert.Equal(t, tc.typ, st.Type)
		})
	}
}

func TestNormalizeStatusMultipleLines(t *testing.T) {
	status := func(codes ...cbc.Code) *bill.Status {
		lines := make([]*bill.StatusLine, 0, len(codes))
		for _, c := range codes {
			lines = append(lines, &bill.StatusLine{
				Ext: tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyNotification: c}),
			})
		}
		return &bill.Status{
			Addons:   tax.WithAddons(sdi.V1),
			Supplier: &org.Party{Name: "Test Supplier"},
			Lines:    lines,
		}
	}
	normalized := func(t *testing.T, codes ...cbc.Code) *bill.Status {
		t.Helper()
		st := status(codes...)
		norm.Normalize(st, tax.AddonContext(sdi.V1))
		return st
	}

	t.Run("should keep each line's own key", func(t *testing.T) {
		st := normalized(t, "RC", "NS")
		assert.Equal(t, bill.StatusLineAcknowledged, st.Lines[0].Key)
		assert.Equal(t, bill.StatusLineError, st.Lines[1].Key)
	})

	t.Run("should not depend on line order", func(t *testing.T) {
		assert.Equal(t, bill.StatusTypeUpdate, normalized(t, "RC", "NS").Type)
		assert.Equal(t, bill.StatusTypeUpdate, normalized(t, "NS", "RC").Type)
		assert.Equal(t, bill.StatusTypeResponse, normalized(t, "EC01", "EC02").Type)
		assert.Equal(t, bill.StatusTypeResponse, normalized(t, "EC02", "EC01").Type)
	})

	t.Run("should reject a mixed set whichever way round", func(t *testing.T) {
		for _, codes := range [][]cbc.Code{{"RC", "EC01"}, {"EC01", "RC"}} {
			st := normalized(t, codes...)
			err := rules.Validate(st, tax.AddonContext(sdi.V1))
			assert.ErrorContains(t, err, "must not mix buyer responses with SDI updates")
		}
	})

	t.Run("should accept a same-direction set", func(t *testing.T) {
		st := normalized(t, "EC01", "EC02")
		assert.NoError(t, rules.Validate(st, tax.AddonContext(sdi.V1)))
	})
}

func TestNormalizeStatusFailedDeliveryFormat(t *testing.T) {
	line := func(ext cbc.CodeMap) *bill.StatusLine {
		return &bill.StatusLine{Ext: tax.ExtensionsOf(ext)}
	}
	status := func(l *bill.StatusLine) *bill.Status {
		st := &bill.Status{
			Addons:   tax.WithAddons(sdi.V1),
			Supplier: &org.Party{Name: "Test Supplier"},
			Lines:    []*bill.StatusLine{l},
		}
		norm.Normalize(st, tax.AddonContext(sdi.V1))
		return st
	}

	t.Run("should reject a failed delivery without a format", func(t *testing.T) {
		st := status(line(cbc.CodeMap{sdi.ExtKeyNotification: "MC"}))
		assert.Empty(t, st.Lines[0].Key, "key must not be guessed")
		err := rules.Validate(st, tax.AddonContext(sdi.V1))
		assert.ErrorContains(t, err, "failed delivery requires the 'it-sdi-format' extension")
	})

	t.Run("should accept a failed delivery with a format", func(t *testing.T) {
		for format, want := range map[cbc.Code]cbc.Key{
			"FPA12": bill.StatusLineProcessing,
			"FPR12": bill.StatusLineAcknowledged,
		} {
			st := status(line(cbc.CodeMap{
				sdi.ExtKeyNotification: "MC",
				sdi.ExtKeyFormat:       format,
			}))
			assert.Equal(t, want, st.Lines[0].Key, string(format))
			assert.NoError(t, rules.Validate(st, tax.AddonContext(sdi.V1)), string(format))
		}
	})

	t.Run("should not require a format on other codes", func(t *testing.T) {
		st := status(line(cbc.CodeMap{sdi.ExtKeyNotification: "RC"}))
		assert.NoError(t, rules.Validate(st, tax.AddonContext(sdi.V1)))
	})
}

func TestNormalizeStatusReasonKeys(t *testing.T) {
	status := func(code cbc.Code, reasons ...*bill.Reason) *bill.Status {
		st := &bill.Status{
			Addons:   tax.WithAddons(sdi.V1),
			Supplier: &org.Party{Name: "Test Supplier"},
			Lines: []*bill.StatusLine{{
				Ext:     tax.ExtensionsOf(cbc.CodeMap{sdi.ExtKeyNotification: code, sdi.ExtKeyFormat: "FPR12"}),
				Reasons: reasons,
			}},
		}
		norm.Normalize(st, tax.AddonContext(sdi.V1))
		return st
	}

	t.Run("should key each code's explanation", func(t *testing.T) {
		for code, want := range map[cbc.Code]cbc.Key{
			"NS":   bill.ReasonKeyLegal,
			"MC":   bill.ReasonKeyDelivery,
			"AT":   bill.ReasonKeyDelivery,
			"EC02": bill.ReasonKeyOther,
		} {
			st := status(code, &bill.Reason{Description: "why"})
			assert.Equal(t, want, st.Lines[0].Reasons[0].Key, string(code))
		}
	})

	t.Run("should key every explanation on the line", func(t *testing.T) {
		st := status("NS", &bill.Reason{Description: "first"}, &bill.Reason{Description: "second"})
		assert.Equal(t, bill.ReasonKeyLegal, st.Lines[0].Reasons[0].Key)
		assert.Equal(t, bill.ReasonKeyLegal, st.Lines[0].Reasons[1].Key)
	})

	t.Run("should leave a key the caller already chose", func(t *testing.T) {
		st := status("NS", &bill.Reason{Key: bill.ReasonKeyQuality, Description: "why"})
		assert.Equal(t, bill.ReasonKeyQuality, st.Lines[0].Reasons[0].Key)
	})

	t.Run("should leave codes that carry no explanation", func(t *testing.T) {
		st := status("RC", &bill.Reason{Description: "why"})
		assert.Equal(t, cbc.KeyEmpty, st.Lines[0].Reasons[0].Key)
	})
}
