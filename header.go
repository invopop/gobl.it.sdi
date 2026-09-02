package fatturapa

import (
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
)

// Header contains all data related to the parties involved in the document.
type Header struct {
	TransmissionData *TransmissionData `xml:"DatiTrasmissione,omitempty"`
	Supplier         *Supplier         `xml:"CedentePrestatore,omitempty"`
	Customer         *Customer         `xml:"CessionarioCommittente,omitempty"`
	ThirdPartyIssuer *ThirdPartyIssuer `xml:"TerzoIntermediarioOSoggettoEmittente,omitempty"`
	IssuerType       string            `xml:"SoggettoEmittente,omitempty"`
}

func newHeader(inv *bill.Invoice, TransmissionData *TransmissionData) (*Header, error) {
	supplier, err := newSupplier(inv.Supplier)
	if err != nil {
		return nil, err
	}
	h := &Header{
		TransmissionData: TransmissionData,
		Supplier:         supplier,
		Customer:         newCustomer(inv.Customer),
		IssuerType:       inv.Tax.GetExt(sdi.ExtKeyIssuerType).String(),
	}
	if inv.Ordering != nil {
		h.ThirdPartyIssuer = newThirdPartyIssuer(inv.Ordering.Issuer)
	}

	return h, nil
}
