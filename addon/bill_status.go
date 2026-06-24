package sdi

import (
	"github.com/invopop/gobl/bill"
)

// normalizeStatus derives each status line's key and the overall status type
// from the SDI notification code stamped on the line (it-sdi-notification).
//
// MC (Mancata Consegna — failed delivery) is the only code whose status
// depends on the recipient: for a public administration (it-sdi-format FPA12)
// delivery may still complete later via an AT, so it is non-terminal
// (processing); for a business recipient (FPR12) it is terminal
// (acknowledged).
func normalizeStatus(st *bill.Status) {
	if st == nil {
		return
	}
	for _, line := range st.Lines {
		normalizeStatusLine(st, line)
	}
}

func normalizeStatusLine(st *bill.Status, line *bill.StatusLine) {
	if line == nil {
		return
	}
	switch line.Ext.Get(ExtKeyNotification) {
	case "RC", "AT":
		line.Key, st.Type = bill.StatusLineAcknowledged, bill.StatusTypeUpdate
	case "DT":
		line.Key, st.Type = bill.StatusLineAccepted, bill.StatusTypeUpdate
	case "NS":
		line.Key, st.Type = bill.StatusLineError, bill.StatusTypeUpdate
	case "MC":
		st.Type = bill.StatusTypeUpdate
		if line.Ext.Get(ExtKeyFormat) == "FPA12" {
			line.Key = bill.StatusLineProcessing
		} else {
			line.Key = bill.StatusLineAcknowledged
		}
	case "EC01":
		line.Key, st.Type = bill.StatusLineAccepted, bill.StatusTypeResponse
	case "EC02":
		line.Key, st.Type = bill.StatusLineRejected, bill.StatusTypeResponse
	}
}
