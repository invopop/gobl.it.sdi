package fatturapa

import "github.com/invopop/gobl/bill"

// Header contains all data related to the parties involved in the document.
type Header struct {
	TransmissionData *TransmissionData `xml:"DatiTrasmissione,omitempty"`
	Supplier         *Supplier         `xml:"CedentePrestatore,omitempty"`
	Customer         *Customer         `xml:"CessionarioCommittente,omitempty"`
}

func newHeader(inv *bill.Invoice, TransmissionData *TransmissionData) (*Header, error) {
	supplier, err := newSupplier(inv.Supplier)
	if err != nil {
		return nil, err
	}
	return &Header{
		TransmissionData: TransmissionData,
		Supplier:         supplier,
		Customer:         newCustomer(inv.Customer),
	}, nil
}
