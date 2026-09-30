package fatturapa

import (
	"bytes"
	"encoding/xml"
	"errors"

	"github.com/invopop/gobl"
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/xmldsig"
)

// Namespace used for the simplified FatturaPA format.
const namespaceFatturaPASimplified = "http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.0"

// SimplifiedInvoice is the FatturaElettronicaSemplificata document, used by
// the FSM10 format.
type SimplifiedInvoice struct {
	XMLName       xml.Name `xml:"p:FatturaElettronicaSemplificata"`
	FPANamespace  string   `xml:"xmlns:p,attr"`
	DSigNamespace string   `xml:"xmlns:ds,attr"`
	Versione      string   `xml:"versione,attr"`

	Header *SimplifiedHeader `xml:"FatturaElettronicaHeader"`
	Body   []*SimplifiedBody `xml:"FatturaElettronicaBody"`

	Signature *xmldsig.Signature `xml:"ds:Signature,omitempty"`
}

// SimplifiedHeader contains the parties involved in a simplified invoice.
type SimplifiedHeader struct {
	TransmissionData *TransmissionData   `xml:"DatiTrasmissione"`
	Supplier         *SimplifiedSupplier `xml:"CedentePrestatore"`
	Customer         *SimplifiedCustomer `xml:"CessionarioCommittente"`
}

// SimplifiedSupplier describes the seller/provider of a simplified invoice.
type SimplifiedSupplier struct {
	TaxID      *TaxID `xml:"IdFiscaleIVA"`
	FiscalCode string `xml:"CodiceFiscale,omitempty"`
	// Name of the business, or given name and surname of the person
	Name         string        `xml:"Denominazione,omitempty"`
	Given        string        `xml:"Nome,omitempty"`
	Surname      string        `xml:"Cognome,omitempty"`
	Address      *Address      `xml:"Sede"`
	Registration *Registration `xml:"IscrizioneREA,omitempty"`
	FiscalRegime string        `xml:"RegimeFiscale"`
}

// SimplifiedCustomer describes who a simplified invoice is addressed to.
type SimplifiedCustomer struct {
	FiscalIdentifiers *FiscalIdentifiers `xml:"IdentificativiFiscali"`
	OtherIdentifiers  *OtherIdentifiers  `xml:"AltriDatiIdentificativi,omitempty"`
}

// FiscalIdentifiers hold the customer's VAT number, fiscal code, or both.
type FiscalIdentifiers struct {
	TaxID      *TaxID `xml:"IdFiscaleIVA,omitempty"`
	FiscalCode string `xml:"CodiceFiscale,omitempty"`
}

// OtherIdentifiers hold the customer's name and address, which the simplified
// format only accepts together.
type OtherIdentifiers struct {
	// Name of the business, or given name and surname of the person
	Name    string   `xml:"Denominazione,omitempty"`
	Given   string   `xml:"Nome,omitempty"`
	Surname string   `xml:"Cognome,omitempty"`
	Address *Address `xml:"Sede"`
}

func newSimplifiedInvoice(env *gobl.Envelope, inv *bill.Invoice, config *config) (*SimplifiedInvoice, error) {
	supplier, err := newSimplifiedSupplier(inv.Supplier)
	if err != nil {
		return nil, err
	}

	body, err := newSimplifiedBody(inv)
	if err != nil {
		return nil, err
	}

	d := &SimplifiedInvoice{
		FPANamespace:  namespaceFatturaPASimplified,
		DSigNamespace: namespaceDSig,
		Versione:      formatoTrasmissioneFSM10,
		Header: &SimplifiedHeader{
			TransmissionData: newTransmissionData(inv, env, config.Transmitter),
			Supplier:         supplier,
			Customer:         newSimplifiedCustomer(inv.Customer),
		},
		Body: []*SimplifiedBody{body},
	}

	if config.Certificate != nil {
		if d.Signature, err = sign(env, d, config); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func newSimplifiedSupplier(s *org.Party) (*SimplifiedSupplier, error) {
	ns := &SimplifiedSupplier{
		Registration: newRegistration(s),
		FiscalRegime: "RF01",
	}

	if p := newProfile(s); p != nil {
		ns.Name, ns.Given, ns.Surname = p.Name, p.Given, p.Surname
	}
	if s.TaxID != nil {
		ns.TaxID = partyTaxID(s.TaxID)
	}
	if ns.TaxID == nil {
		return nil, errors.New("supplier tax ID is required")
	}
	if id := org.IdentityForKey(s.Identities, it.IdentityKeyFiscalCode); id != nil {
		ns.FiscalCode = id.Code.String()
	}
	if v := s.Ext.Get(sdi.ExtKeyFiscalRegime); v != "" {
		ns.FiscalRegime = v.String()
	}
	if len(s.Addresses) > 0 {
		ns.Address = newAddress(s.Addresses[0])
	}

	return ns, nil
}

func newSimplifiedCustomer(c *org.Party) *SimplifiedCustomer {
	if c == nil {
		return nil
	}

	fi := new(FiscalIdentifiers)
	if c.TaxID != nil {
		fi.TaxID = partyTaxID(c.TaxID)
	}
	if id := org.IdentityForKey(c.Identities, it.IdentityKeyFiscalCode); id != nil {
		fi.FiscalCode = id.Code.String()
	}

	nc := &SimplifiedCustomer{FiscalIdentifiers: fi}

	// A name without an address has no place in the format, and the fiscal
	// identifiers are enough to identify the customer.
	if p := newProfile(c); p != nil && len(c.Addresses) > 0 {
		nc.OtherIdentifiers = &OtherIdentifiers{
			Name:    p.Name,
			Given:   p.Given,
			Surname: p.Surname,
			Address: newAddress(c.Addresses[0]),
		}
	}

	return nc
}

// Buffer returns a byte buffer representation of the complete XML document.
func (d *SimplifiedInvoice) Buffer() (*bytes.Buffer, error) {
	return marshal(d, xml.Header)
}

// String converts a struct representation to its string representation
func (d *SimplifiedInvoice) String() (string, error) {
	buf, err := d.Buffer()
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Bytes returns the XML document bytes
func (d *SimplifiedInvoice) Bytes() ([]byte, error) {
	buf, err := d.Buffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
