package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/norm"
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
		{"AT", "AT", "", bill.StatusLineAcknowledged, bill.StatusTypeUpdate},
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
