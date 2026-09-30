# GOBL - Italy SDI

Italian electronic invoicing for GOBL: the `it-sdi-v1` addon and conversion to
and from the FatturaPA format.

Copyright [Invopop Ltd.](https://invopop.com) 2023. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

[![Lint](https://github.com/invopop/gobl.it.sdi/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.it.sdi/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.it.sdi/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.it.sdi/actions/workflows/test.yaml)
[![codecov](https://codecov.io/gh/invopop/gobl.it.sdi/graph/badge.svg)](https://codecov.io/gh/invopop/gobl.it.sdi)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.it.sdi)](https://goreportcard.com/report/github.com/invopop/gobl.it.sdi)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.it.sdi?status.svg)](https://godoc.org/github.com/invopop/gobl.it.sdi)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.it.sdi)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/invopop/gobl.it.sdi)

## Introduction

FatturaPA defines two versions of invoices:

- Ordinary invoices, `FatturaElettronica` types `FPA12` and `FPR12` defined in the v1.2 schema, usable for all sales.
- Simplified invoices, `FatturaElettronicaSemplificata` type `FSM10` defined in the v1.0 schema, with a reduced set of requirements. They are limited to €400, unless the supplier is in the flat rate (RF19) or cross-border franchise (RF20) regime or they are credit or debit notes correcting a preceding invoice.

A GOBL invoice with the `simplified` tag converts to a simplified invoice, and any other to an ordinary one. Unlike other tax regimes, Italy requires simplified invoices to include the customer's tax ID. For "cash register" style receipts locally called "Scontrinos", another format and API is used.

## The it-sdi addon

The `addon` package implements the `it-sdi-v1` GOBL addon: the extensions,
scenarios, normalizers, and validation rules a GOBL document needs to be
converted to a valid FatturaPA file, plus the mapping of SDI notification
codes to document statuses.

To calculate or validate documents that declare `it-sdi-v1`, import the
package:

```go
import _ "github.com/invopop/gobl.it.sdi/addon"
```

The addon also has to be on GOBL's approved external addon list for `it-sdi-v1`
to be accepted as an `$addons` value; that entry arrives with the GOBL release
that removes the in-core copy.

## Sources

You can find copies of the Italian FatturaPA schema in the [schemas folder](./schemas).

Key websites:

- [FatturaPA & SDI service documentation page on on Italy's tax authority's website](https://www.agenziaentrate.gov.it/portale/web/guest/fatturazione-elettronica-e-dati-fatture-transfrontaliere-new/)
- [FatturaPA documentation page on FatturaPA's dedicated website](https://www.fatturapa.gov.it/en/norme-e-regole/documentazione-fattura-elettronica/formato-fatturapa/)

Useful files:

- [Ordinary invoice table view (EN)](https://www.agenziaentrate.gov.it/portale/documents/d/guest/table-view-b2b-ordinary-invoice-1-9-1) - by far the most comprehensible spec doc, for schema v1.2.3
- [Simplified invoice table view (EN)](https://www.agenziaentrate.gov.it/portale/documents/d/guest/table-view-b2b-simplified-invoice-1-9-1) - the same for schema v1.0.2
- [Technical specifications v1.9.1 (IT)](https://www.agenziaentrate.gov.it/portale/documents/d/guest/allegato-a-specifiche-tecniche-vers-1-9-1) - SDI's checks and error codes for both formats
- [XSD V1.2.3](https://www.agenziaentrate.gov.it/portale/documents/d/guest/schema_vfpr12_v1-2-3)
- [XSD V1.0.2 (FSM10) - simplified invoices](https://www.agenziaentrate.gov.it/portale/documents/d/guest/schema_vfsm10v_1-0-2)
- [CIUS-IT (Italian Core Invoice Usage Specification) - EN16931 mappings](https://www.agid.gov.it/sites/default/files/repository_files/documentazione/eigor_cius_it_rel_1_0_0_accessibile_0.pdf)

## Limitations

### To FatturaPA

The FatturaPA XML schema is quite large and complex. This library is not complete and only supports a subset of the schema. The current implementation is focused on the most common use cases.

- FatturaPA allows multiple invoices within the document, but this library only supports a single invoice per transmission.
- Only a subset of payment methods (ModalitaPagamento) are supported. See `payments.go` for the list of supported codes.
- Stamp duty is declared paid (`BolloVirtuale`) only from a stamp duty charge. A duty the supplier pays without charging it to the customer can't be declared; the Agenzia delle Entrate lists such invoices for the supplier to confirm when it computes the quarter's duty.

Some of the optional elements currently not supported include:

- `Allegati` (attachments)

### Simplified invoices

A simplified invoice has one entry per line, charge and discount, each with its amount including VAT; discounts are negative entries. The entries add up to the invoice's total with tax. The addon rejects what the format cannot carry, such as retained taxes, fund contributions, a rounding amount, a charge or discount without VAT, a sale to a habitual exporter (N3.5, which needs the declaration of intent) or a stamp duty exemption (NB1, NB2, NB3). These are left out of the XML:

- Payment terms, instructions and advances.
- Ordering references and notes.
- Preceding references on a standard simplified invoice (TD07); only credit and debit notes identify the invoice they correct.
- Supplier contacts.
- Line quantities, unit prices, periods and item attributes.
- The customer's name when there is no address, since the format only accepts the two together. The tax ID or fiscal code still identifies the customer.

Parsing a simplified invoice gives one line per entry, priced with VAT included. `BolloVirtuale` on its own creates no stamp duty charge: a duty charged to the customer is already one of the entries, and the flag doesn't say which.

### From FatturaPA

Converting from FatturaPA to GOBL has some limitations:

- Currently, only one invoice per XML file is supported. FatturaPA allows multiple invoices in a single transmission, but this library only processes the first one.
- Digital signature validation is not fully implemented. While the library can parse signed documents, it does not currently validate all aspects of the signature.

## Usage

### Go

#### To FatturaPA

If you already have a GOBL Envelope available in Go, you could convert and output to a data file like this:

```golang
doc, err := fatturapa.Convert(env)
if err != nil {
    panic(err)
}

data, err := doc.Bytes()
if err != nil {
    panic(err)
}

if err = os.WriteFile("./test.xml", data, 0644); err != nil {
    panic(err)
}
```

`Convert` returns a `Document`: an `*OrdinaryInvoice` for the FPA12 and FPR12 formats, or a `*SimplifiedInvoice` for FSM10, following the invoice's `it-sdi-format` extension.

See the following example for signing the XML with a certificate:

```golang
// import from github.com/invopop/xmldsig
cert, err := xmldsig.LoadCertificate(filename, password)
if err != nil {
    panic(err)
}

doc, err := fatturapa.Convert(env,
    fatturapa.WithCertificate(cert),
    fatturapa.WithTimestamp(), // if you want to include a timestamp in the digital signature
)
if err != nil {
    panic(err)
}
```

If you want to include the fiscal data of the entity integrating with the SDI (Italy's e-invoice system) and `ProgressivoInvio` (transmission number) in the XML, you can use the `WithTransmitterData` option. This option must be used if you are integrating diredctly with the SDI, but if you are working with a third party service to send the XML, it would be on their side to include this data.

```golang
transmitter := fatturapa.Transmitter{
    CountryCode: countryCode, // ISO 3166-1 alpha-2
    TaxID:       taxID,       // Valid tax ID of transmitter
}

doc, err := fatturapa.Convert(env,
    fatturapa.WithTransmitterData(&transmitter),
    // other options
)
```

#### From FatturaPA

Converting from FatturaPA XML to GOBL is also straightforward. `Parse` transforms an ordinary or simplified FatturaPA XML document into a GOBL Envelope:

```golang
// Import the XML data from a file or other source
xmlData, err := os.ReadFile("./invoice.xml")
if err != nil {
    panic(err)
}

// Convert the XML to a GOBL Envelope
env, err := fatturapa.Parse(xmlData)
if err != nil {
    panic(err)
}

// The envelope now contains a GOBL invoice
invoice, ok := env.Extract().(*bill.Invoice)
if !ok {
    panic("expected an invoice")
}

// You can now work with the GOBL invoice
// For example, validate it
if err = env.Validate(); err != nil {
    panic(err)
}

// Or convert it to JSON
jsonData, err := json.MarshalIndent(env, "", "  ")
if err != nil {
    panic(err)
}

if err = os.WriteFile("./invoice.json", jsonData, 0644); err != nil {
    panic(err)
}
```

Note that when converting from FatturaPA to GOBL:

1. The XML document must contain a valid digital signature. The library will check for the presence of a signature but does not currently perform full signature validation.
2. Only the first invoice in the XML file will be processed if the document contains multiple invoices.
3. The resulting GOBL invoice will include the Italian SDI addon (`sdi.V1`) to maintain compatibility with FatturaPA-specific fields.

### CLI

The command line interface can be useful for situations when you're using a language other than Golang in your application. Download one of the [pre-compiled `gobl.fatturapa` releases](https://github.com/invopop/gobl.it.sdi/releases) or install with:

```bash
go install github.com/invopop/gobl.it.sdi/cmd/gobl.fatturapa
```

#### Converting GOBL to FatturaPA

To convert from GOBL JSON to FatturaPA XML:

```bash
gobl.fatturapa convert input.json output.xml
```

If you have a digital certificate, run with:

```bash
gobl.fatturapa convert -c cert.p12 -p password input.json output.xml
```

To include the transmitter information, add the `-T` flag and provide the _country code_ and the _tax ID_:

```bash
gobl.fatturapa convert -T ES12345678 input.json output.xml
```

#### Converting FatturaPA to GOBL

To convert from FatturaPA XML to GOBL JSON:

```bash
gobl.fatturapa convert input.xml output.json
```

By default, the JSON output is pretty-printed. To disable this, use the `--pretty=false` flag:

```bash
gobl.fatturapa convert --pretty=false input.xml output.json
```
