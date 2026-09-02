package fatturapa

import (
	"github.com/invopop/gobl"
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

const (
	formatoTrasmissioneFPA12 = "FPA12" // B2G
	formatoTrasmissioneFPR12 = "FPR12" // B2B or B2C
)

// Invoices sent to Italian individuals or businesses can use 0000000 as the
// codice destinatario when it is not indicated explicitly.
// When the recipient is foreign, XXXXXXX is used.
const (
	defaultCodiceDestinatarioItalianBusiness = "0000000"
	defaultCodiceDestinatarioForeignBusiness = "XXXXXXX"
)

// TransmissionData contains data related to the transmission of the invoice
type TransmissionData struct {
	TransmitterID      *TaxID `xml:"IdTrasmittente,omitempty"` // nolint:revive
	ProgressiveNumber  string `xml:"ProgressivoInvio,omitempty"`
	TransmissionFormat string `xml:"FormatoTrasmissione,omitempty"`
	RecipientCode      string `xml:"CodiceDestinatario"`
	RecipientPEC       string `xml:"PECDestinatario,omitempty"`
}

func newTransmissionData(inv *bill.Invoice, env *gobl.Envelope, transmitter *Transmitter) *TransmissionData {
	dt := &TransmissionData{
		RecipientCode: codiceDestinatario(inv.Customer),
		RecipientPEC:  pecDestinatario(inv.Customer),
	}

	// Do we need to add the transmitter info?
	if transmitter != nil {
		dt.TransmitterID = &TaxID{
			Country: transmitter.CountryCode,
			Code:    transmitter.TaxID,
		}
		dt.ProgressiveNumber = env.Head.UUID.String()[:8]
		dt.TransmissionFormat = formatoTransmissione(inv)
	}

	return dt
}

func formatoTransmissione(inv *bill.Invoice) string {
	if inv.Tax != nil && inv.Tax.Ext.Has(sdi.ExtKeyFormat) {
		return inv.Tax.Ext.Get(sdi.ExtKeyFormat).String()
	}
	// Default is always FPR12 for regular non-government invoices
	return formatoTrasmissioneFPR12
}

func codiceDestinatario(cus *org.Party) string {
	if cus != nil {
		if cus.TaxID != nil && cus.TaxID.Country.Code() != l10n.IT {
			return defaultCodiceDestinatarioForeignBusiness
		}
		for _, inbox := range cus.Inboxes {
			if inbox.Key == sdi.KeyInboxCode {
				return inbox.Code.String()
			}
		}
	}

	// When this is returned, we'll assume there is a PEC.
	// This is also valid for individuals.
	return defaultCodiceDestinatarioItalianBusiness
}

func pecDestinatario(cus *org.Party) string {
	if cus != nil {
		for _, inbox := range cus.Inboxes {
			// PECDestinatario is only allowed alongside the "0000000" code,
			// so a codice destinatario inbox takes precedence over the PEC.
			if inbox.Key == sdi.KeyInboxCode {
				return ""
			}
		}
		for _, inbox := range cus.Inboxes {
			if inbox.Key == sdi.KeyInboxPEC {
				if inbox.Email != "" {
					return inbox.Email
				}
				return inbox.Code.String()
			}
		}
	}
	return ""
}
