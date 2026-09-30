// Package fatturapa implements the conversion from GOBL to FatturaPA XML.
package fatturapa

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"encoding/xml"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"

	"github.com/invopop/gobl"
	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/xmlctx"
	"github.com/invopop/xmldsig"
)

// <p:FatturaElettronica xmlns:ds="http://www.w3.org/2000/09/xmldsig#"
//   xmlns:p="http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.2"
//   xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" versione="FPA12" xsi:schemaLocation="http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.2 http://www.fatturapa.gov.it/export/fatturazione/sdi/fatturapa/v1.2/Schema_del_file_xml_FatturaPA_versione_1.2.xsd">

// Namespace used for FatturaPA. DSig stuff is handled in the signatures.
const (
	namespaceFatturaPA = "http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.2"
	namespaceDSig      = "http://www.w3.org/2000/09/xmldsig#"
	namespaceXSI       = "http://www.w3.org/2001/XMLSchema-instance"
	schemaLocation     = "http://ivaservizi.agenziaentrate.gov.it/docs/xsd/fatture/v1.2 https://www.fatturapa.gov.it/export/documenti/fatturapa/v1.2.2/Schema_del_file_xml_FatturaPA_v1.2.2.xsd"
)

// Document is a FatturaPA document ready to be sent to SDI.
type Document interface {
	Buffer() (*bytes.Buffer, error)
	String() (string, error)
	Bytes() ([]byte, error)
}

// OrdinaryInvoice is the FatturaElettronica document, used by the FPA12 and
// FPR12 formats.
type OrdinaryInvoice struct {
	env *gobl.Envelope `xml:"-"` // Envelope to convert.

	XMLName        xml.Name `xml:"p:FatturaElettronica"`
	FPANamespace   string   `xml:"xmlns:p,attr"`
	DSigNamespace  string   `xml:"xmlns:ds,attr"`
	XSINamespace   string   `xml:"xmlns:xsi,attr"`
	Versione       string   `xml:"versione,attr"`
	SchemaLocation string   `xml:"xsi:schemaLocation,attr"`

	Header *Header `xml:"FatturaElettronicaHeader"`
	Body   []*Body `xml:"FatturaElettronicaBody"`

	Signature *xmldsig.Signature `xml:"ds:Signature,omitempty"`
}

// Convert expects the base envelope and provides a new Document
// containing the XML version.
func Convert(env *gobl.Envelope, opts ...Option) (Document, error) {
	invoice, ok := env.Extract().(*bill.Invoice)
	if !ok || invoice == nil {
		return nil, errors.New("expected an invoice")
	}
	config := parseOptions(opts...)
	if formatoTransmissione(invoice) == formatoTrasmissioneFSM10 {
		d, err := newSimplifiedInvoice(env, invoice, config)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	d, err := newOrdinaryInvoice(env, invoice, config)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func newOrdinaryInvoice(env *gobl.Envelope, invoice *bill.Invoice, config *config) (*OrdinaryInvoice, error) {
	// Make sure we're dealing with raw data
	if err := invoice.RemoveIncludedTaxes(); err != nil {
		return nil, err
	}

	TransmissionData := newTransmissionData(invoice, env, config.Transmitter)

	header, err := newHeader(invoice, TransmissionData)
	if err != nil {
		return nil, err
	}

	body, err := newBody(invoice)
	if err != nil {
		return nil, err
	}

	// Basic document headers
	d := &OrdinaryInvoice{
		env:            env,
		FPANamespace:   namespaceFatturaPA,
		DSigNamespace:  namespaceDSig,
		XSINamespace:   namespaceXSI,
		Versione:       formatoTransmissione(invoice),
		SchemaLocation: schemaLocation,
		Header:         header,
		Body:           []*Body{body},
	}

	if config.Certificate != nil {
		if d.Signature, err = sign(env, d, config); err != nil {
			return nil, err
		}
	}

	return d, nil
}

// Parse expects the XML document bytes and provides a new GOBL
// envelope containing the invoice.
func Parse(doc []byte) (*gobl.Envelope, error) {
	// Convert document to UTF-8 if needed
	convertedDoc, err := convertToUTF8(doc)
	if err != nil {
		return nil, fmt.Errorf("convert encoding: %w", err)
	}

	ns, err := rootNamespace(convertedDoc)
	if err != nil {
		return nil, fmt.Errorf("unmarshal document: %w", err)
	}
	if ns == namespaceFatturaPASimplified {
		return parseSimplified(convertedDoc)
	}

	d := &OrdinaryInvoice{}
	if err := xmlctx.Unmarshal(convertedDoc, d, xmlctx.WithNamespaces(map[string]string{
		"p":   namespaceFatturaPA,
		"ds":  namespaceDSig,
		"xsi": namespaceXSI,
	})); err != nil {
		return nil, fmt.Errorf("unmarshal document: %w", err)
	}

	// Verify signature. Standin for now.
	// Skip signature verification for now
	/*
		if d.Signature == nil {
			return nil, errors.New("signature is missing")
		}
	*/

	// Create a new invoice with empty fields so that converter can fill it
	inv := new(bill.Invoice)

	inv.Addons = tax.WithAddons(sdi.V1)

	// Retrieves information from the header and adds it to the invoice
	goblBillInvoiceAddHeader(inv, d.Header)

	// Retrieves information from the body and adds it to the invoice
	// TODO: add support for multiple bodies
	if err := goblBillInvoiceAddBody(inv, d.Body[0]); err != nil {
		return nil, err
	}

	// Final totals check
	if err := adjustTotals(inv, d.Body[0].GeneralData.Document); err != nil {
		return nil, err
	}

	// Generate envelope
	env, err := gobl.Envelop(inv)
	if err != nil {
		return nil, err
	}

	return env, nil
}

// Buffer returns a byte buffer representation of the complete XML document.
func (d *OrdinaryInvoice) Buffer() (*bytes.Buffer, error) {
	return marshal(d, xml.Header)
}

// String converts a struct representation to its string representation
func (d *OrdinaryInvoice) String() (string, error) {
	buf, err := d.Buffer()
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Bytes returns the XML document bytes
func (d *OrdinaryInvoice) Bytes() ([]byte, error) {
	buf, err := d.Buffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshal(doc any, base string) (*bytes.Buffer, error) {
	buf := bytes.NewBufferString(base)
	data, err := xml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal document: %w", err)
	}
	if _, err := buf.Write(data); err != nil {
		return nil, fmt.Errorf("writing to buffer: %w", err)
	}
	return buf, nil
}

// rootNamespace returns the namespace of the document's root element, which
// tells the ordinary and simplified formats apart.
func rootNamespace(doc []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(doc))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Space, nil
		}
	}
}

// convertToUTF8 detects the encoding from the XML declaration and converts
// the document to UTF-8 if necessary
func convertToUTF8(doc []byte) ([]byte, error) {
	// Extract encoding from XML declaration
	encoding := detectEncoding(doc)
	if encoding == "" || strings.EqualFold(encoding, "utf-8") {
		return doc, nil
	}

	// Handle windows-1252 encoding
	if strings.EqualFold(encoding, "windows-1252") {
		decoder := charmap.Windows1252.NewDecoder()
		reader := transform.NewReader(bytes.NewReader(doc), decoder)
		converted, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to convert from windows-1252: %w", err)
		}
		// Replace the encoding declaration with UTF-8
		converted = replaceEncodingDeclaration(converted, "UTF-8")
		return converted, nil
	}

	// Add more encodings as needed
	return nil, fmt.Errorf("unsupported encoding: %s", encoding)
}

// detectEncoding extracts the encoding attribute from the XML declaration
func detectEncoding(doc []byte) string {
	// Match XML declaration and extract encoding
	re := regexp.MustCompile(`<\?xml[^>]+encoding=["']([^"']+)["']`)
	matches := re.FindSubmatch(doc)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return ""
}

// replaceEncodingDeclaration replaces the encoding in the XML declaration
func replaceEncodingDeclaration(doc []byte, newEncoding string) []byte {
	re := regexp.MustCompile(`(<\?xml[^>]+encoding=["'])([^"']+)(["'])`)
	return re.ReplaceAll(doc, []byte("${1}"+newEncoding+"${3}"))
}
