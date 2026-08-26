# GOBL - FatturaPA Tools

Convert GOBL documents to and from Italy's FatturaPA format.

Copyright [Invopop Ltd.](https://invopop.com) 2023. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

[![Lint](https://github.com/invopop/gobl.fatturapa/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.fatturapa/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.fatturapa/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.fatturapa/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.fatturapa)](https://goreportcard.com/report/github.com/invopop/gobl.fatturapa)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.fatturapa?status.svg)](https://godoc.org/github.com/invopop/gobl.fatturapa)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.fatturapa)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/invopop/gobl.fatturapa)

## Introduction

FatturaPA defines two versions of invoices:

- Ordinary invoices, `FatturaElettronica` types `FPA12` and `FPR12` defined in the v1.2 schema, usable for all sales.
- Simplified invoices, `FatturaElettronicaSemplificata` type `FSM10` defined in the v1.0 schema, with a reduced set of requirements but can only be used for sales of less then €400, as of writing.

Unlike other tax regimes, Italy requires simplified invoices to include the customer's tax ID. For "cash register" style receipts locally called "Scontrinos", another format and API is used.

## Sources

You can find copies of the Italian FatturaPA schema in the [schemas folder](./schemas).

Key websites:

- [FatturaPA & SDI service documentation page on on Italy's tax authority's website](https://www.agenziaentrate.gov.it/portale/web/guest/fatturazione-elettronica-e-dati-fatture-transfrontaliere-new/)
- [FatturaPA documentation page on FatturaPA's dedicated website](https://www.fatturapa.gov.it/en/norme-e-regole/documentazione-fattura-elettronica/formato-fatturapa/)

Useful files:

- [Ordinary Schema V1.2.1 Spec Table View (EN)](https://www.fatturapa.gov.it/export/documenti/fatturapa/v1.2.1/Table-view-B2B-Ordinary-invoice.pdf) - by far the most comprehensible spec doc. Since the difference between 1.2.2 and 1.2.1 is minimal, this is perfectly usable.
- [Ordinary Schema V1.2.2 PDF (IT)](https://www.fatturapa.gov.it/export/documenti/Specifiche_tecniche_del_formato_FatturaPA_v1.3.1.pdf) - most up-to-date but difficult
- [XSD V1.2.2](https://www.fatturapa.gov.it/export/documenti/fatturapa/v1.2.2/Schema_del_file_xml_FatturaPA_v1.2.2.xsd)
- [XSD V1 (FSM10) - simplified invoices](https://www.agenziaentrate.gov.it/portale/documents/20143/288192/ST+Fatturazione+elettronica+-+Schema+VFSM10_Schema_VFSM10.xsd/010f1b41-6683-1b31-ba36-c8bced659c06)
- [CIUS-IT (Italian Core Invoice Usage Specification) - EN16931 mappings](https://www.agid.gov.it/sites/default/files/repository_files/documentazione/eigor_cius_it_rel_1_0_0_accessibile_0.pdf)

## Limitations

### To FatturaPA

The FatturaPA XML schema is quite large and complex. This library is not complete and only supports a subset of the schema. The current implementation is focused on the most common use cases.

- Simplified invoices are not currently supported (please get in touch if you need this).
- FatturaPA allows multiple invoices within the document, but this library only supports a single invoice per transmission.
- Only a subset of payment methods (ModalitaPagamento) are supported. See `payments.go` for the list of supported codes.
- Party identifiers are substituted where FatturaPA expects a placeholder instead of a real value, so a GOBL party should carry only the identifiers it actually has. A non-EU business's own tax number is not transmitted, since only EU VAT numbers are meaningful to SDI, and a customer outside Italy is always addressed as undeliverable whatever inbox it carries. See `parties.go` and `transmission.go` for the values.

Some of the optional elements currently not supported include:

- `Allegati` (attachments)

### From FatturaPA

Converting from FatturaPA to GOBL has some limitations:

- Currently, only one invoice per XML file is supported. FatturaPA allows multiple invoices in a single transmission, but this library only processes the first one.
- Digital signature validation is not fully implemented. While the library can parse signed documents, it does not currently validate all aspects of the signature.

## Usage

### Go

#### To FatturaPA

There are a couple of entry points to build a new Fatturapa document. If you already have a GOBL Envelope available in Go, you could convert and output to a data file like this:

```golang
converter := fatturapa.NewConverter()

doc, err := converter.ConvertFromGOBL(env)
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

See the following example for signing the XML with a certificate:

```golang
// import from github.com/invopop/xmldsig
cert, err := xmldsig.LoadCertificate(filename, password)
if err != nil {
    panic(err)
}

converter := fatturapa.NewConverter(
    fatturapa.WithCertificate(cert),
    fatturapa.WithTimestamp(), // if you want to include a timestamp in the digital signature
)

doc, err := converter.ConvertFromGOBL(env)
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

converter := fatturapa.NewConverter(
    fatturapa.WithTransmitterData(transmitter),
    // other options
)
```

#### From FatturaPA

Converting from FatturaPA XML to GOBL is also straightforward. You can use the `ConvertToGOBL` method to transform a FatturaPA XML document into a GOBL Envelope:

```golang
// Import the XML data from a file or other source
xmlData, err := os.ReadFile("./invoice.xml")
if err != nil {
    panic(err)
}

// Create a converter
converter := fatturapa.NewConverter()

// Convert the XML to a GOBL Envelope
env, err := converter.ConvertToGOBL(xmlData)
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

The command line interface can be useful for situations when you're using a language other than Golang in your application. Download one of the [pre-compiled `gobl.fatturapa` releases](https://github.com/invopop/gobl.fatturapa/releases) or install with:

```bash
go install github.com/invopop/gobl.fatturapa/cmd/gobl.fatturapa
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
