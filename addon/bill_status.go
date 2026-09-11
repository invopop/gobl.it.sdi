package sdi

import (
	"fmt"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
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
		normalizeStatusLine(line)
	}
	// A document mixing the two directions is rejected by billStatusRules, so
	// any line that carries a direction gives the document's.
	for _, line := range st.Lines {
		if line == nil {
			continue
		}
		if t, ok := statusTypes[line.Ext.Get(ExtKeyNotification)]; ok {
			st.Type = t
			return
		}
	}
}

// statusTypes maps each notification code to the direction it reports: EC01
// and EC02 are the buyer answering, every other code is SDI reporting.
var statusTypes = map[cbc.Code]cbc.Key{
	"RC":   bill.StatusTypeUpdate,
	"NS":   bill.StatusTypeUpdate,
	"MC":   bill.StatusTypeUpdate,
	"AT":   bill.StatusTypeUpdate,
	"DT":   bill.StatusTypeUpdate,
	"EC01": bill.StatusTypeResponse,
	"EC02": bill.StatusTypeResponse,
}

// reasonKeys maps each notification code to the kind of explanation it
// carries: NS the validation faults SDI found, MC and AT why delivery could
// not be completed, EC02 the buyer's own grounds for refusing. The text comes
// from the notification itself, so only the key is set here.
var reasonKeys = map[cbc.Code]cbc.Key{
	"NS":   bill.ReasonKeyLegal,
	"MC":   bill.ReasonKeyDelivery,
	"AT":   bill.ReasonKeyDelivery,
	"EC02": bill.ReasonKeyOther,
}

func normalizeStatusLine(line *bill.StatusLine) {
	if line == nil {
		return
	}
	if key, ok := reasonKeys[line.Ext.Get(ExtKeyNotification)]; ok {
		for _, r := range line.Reasons {
			if r != nil && r.Key == cbc.KeyEmpty {
				r.Key = key
			}
		}
	}
	switch line.Ext.Get(ExtKeyNotification) {
	case "RC":
		line.Key = bill.StatusLineAcknowledged
	case "AT":
		// The recipient never received the invoice, so acknowledged
		// (buyer received a readable message) would misreport. The
		// sender must still deliver the invoice, with the attestation,
		// outside SDI.
		line.Key = bill.StatusLineError
	case "DT":
		line.Key = bill.StatusLineAccepted
	case "NS":
		line.Key = bill.StatusLineError
	case "MC":
		// Failed delivery means different things by recipient: a public
		// administration may still receive the invoice later via an AT, a
		// business will not. Without the format there is no way to tell, so
		// the key is left for the rules to reject.
		switch line.Ext.Get(ExtKeyFormat) {
		case "FPA12":
			line.Key = bill.StatusLineProcessing
		case "FPR12":
			line.Key = bill.StatusLineAcknowledged
		}
	case "EC01":
		line.Key = bill.StatusLineAccepted
	case "EC02":
		line.Key = bill.StatusLineRejected
	}
}

// billStatusRules rejects a status whose lines mix the buyer's answer with
// SDI's own reporting: one document reports one event, and a mixed set would
// leave the document's type depending on which line came first.
func billStatusRules() *rules.Set {
	return rules.For(new(bill.Status),
		rules.Assert("01", "notifications must not mix buyer responses with SDI updates",
			is.Func("has one notification direction", statusHasOneDirection),
		),
	)
}

// billStatusLineRules requires the transmission format on a failed delivery,
// whose meaning depends on whether the recipient is a public administration.
func billStatusLineRules() *rules.Set {
	return rules.For(new(bill.StatusLine),
		rules.When(is.Func("is a failed delivery", statusLineIsFailedDelivery),
			rules.Field("ext",
				rules.Assert("01",
					fmt.Sprintf("failed delivery requires the '%s' extension", ExtKeyFormat),
					tax.ExtensionsRequire(ExtKeyFormat),
				),
			),
		),
	)
}

func statusLineIsFailedDelivery(val any) bool {
	line, ok := val.(*bill.StatusLine)
	return ok && line != nil && line.Ext.Get(ExtKeyNotification) == "MC"
}

func statusHasOneDirection(val any) bool {
	st, ok := val.(*bill.Status)
	if !ok || st == nil {
		return true
	}
	var seen cbc.Key
	for _, line := range st.Lines {
		if line == nil {
			continue
		}
		t, ok := statusTypes[line.Ext.Get(ExtKeyNotification)]
		if !ok {
			continue
		}
		if seen != cbc.KeyEmpty && t != seen {
			return false
		}
		seen = t
	}
	return true
}
