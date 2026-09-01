package fatturapa

import (
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

func goblBillInvoiceAddTransmission(inv *bill.Invoice, dt *TransmissionData) {
	if inv.Tax == nil {
		inv.Tax = &bill.Tax{}
	}

	inv.Tax.Ext = inv.Tax.Ext.Set(sdi.ExtKeyFormat, cbc.Code(dt.TransmissionFormat))

	if dt.TransmissionFormat == "FPA12" {
		inv.Tags.SetTags(tax.TagB2G)
	}

	if inv.Customer == nil {
		inv.Customer = &org.Party{}
	}

	if inv.Customer.Inboxes == nil {
		inv.Customer.Inboxes = []*org.Inbox{}
	}

	if dt.RecipientCode != "" && dt.RecipientCode != "XXXXXXX" && dt.RecipientCode != "0000000" {
		inv.Customer.Inboxes = append(inv.Customer.Inboxes,
			&org.Inbox{
				Key:  sdi.KeyInboxCode,
				Code: cbc.Code(dt.RecipientCode),
			},
		)
	}

	if dt.RecipientPEC != "" {
		inv.Customer.Inboxes = append(inv.Customer.Inboxes,
			&org.Inbox{
				Key:   sdi.KeyInboxPEC,
				Email: dt.RecipientPEC,
			},
		)
	}

}
