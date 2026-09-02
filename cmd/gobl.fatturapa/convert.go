// Package main implements the CLI as well as mage commands (toplevel mage.go)
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/gobl"
	fatturapa "github.com/invopop/gobl.it.sdi"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
	"github.com/spf13/cobra"
)

const (
	// FormatTypeXML represents XML format (FatturaPA)
	FormatTypeXML string = ".xml"
	// FormatTypeJSON represents JSON format (GOBL)
	FormatTypeJSON string = ".json"
)

type convertOpts struct {
	*rootOpts
	cert          string
	password      string
	transmitter   string
	withTimestamp bool
	pretty        bool
}

func convert(o *rootOpts) *convertOpts {
	return &convertOpts{rootOpts: o}
}

func (c *convertOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [infile] [outfile]",
		Short: "Convert between GOBL and FatturaPA formats",
		Long:  `Auto-detect input format and convert between GOBL JSON and FatturaPA XML formats.`,
		RunE:  c.runE,
	}

	// Add flags for both conversion directions
	f := cmd.Flags()
	f.StringVarP(&c.cert, "cert", "c", "", "Certificate for signing in pkcs12 format (for XML output)")
	f.StringVarP(&c.password, "password", "x", "", "Password of the certificate (for XML output)")
	f.StringVarP(&c.transmitter, "transmitter", "T", "", "Tax ID of the transmitter. Must be prefixed by the country code (for XML output)")
	f.BoolVarP(&c.withTimestamp, "with-timestamp", "t", false, "Add timestamp to the output file (for XML output)")
	f.BoolVarP(&c.pretty, "pretty", "p", true, "Output pretty-printed result")

	return cmd
}

func (c *convertOpts) runE(cmd *cobra.Command, args []string) error {
	// Read input file
	input := inputFilename(args)

	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	opts, err := loadFatturaPAOptions(c)
	if err != nil {
		return err
	}

	// Detect format
	ext := filepath.Ext(input)

	// Process based on detected format
	var outputData []byte
	switch ext {
	case FormatTypeJSON:
		// Convert JSON to XML
		outputData, err = convertJSONToXML(data, c.pretty, opts...)
		if err != nil {
			return err
		}
	case FormatTypeXML:
		// Convert XML to JSON
		outputData, err = convertXMLToJSON(data, c.pretty)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unable to determine input format, please use .json or .xml")
	}

	// Write output
	outFile := outputFilename(args)
	if outFile == "" {
		// Write to stdout if no output file specified
		_, err = cmd.OutOrStdout().Write(outputData)
		return err
	}

	if err = os.WriteFile(outFile, outputData, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// convertJSONToXML converts GOBL JSON to FatturaPA XML
func convertJSONToXML(data []byte, pretty bool, opts ...fatturapa.Option) ([]byte, error) {
	// Parse GOBL data
	out, err := gobl.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing GOBL: %w", err)
	}

	var env *gobl.Envelope
	switch doc := out.(type) {
	case *gobl.Envelope:
		env = doc
	default:
		env = gobl.NewEnvelope()
		if err := env.Insert(doc); err != nil {
			return nil, fmt.Errorf("inserting document into envelope: %w", err)
		}
	}

	// Convert to FatturaPA
	doc, err := fatturapa.Convert(env, opts...)
	if err != nil {
		return nil, fmt.Errorf("converting to FatturaPA: %w", err)
	}

	// Marshal to XML
	var result []byte
	if pretty {
		result, err = xml.MarshalIndent(doc, "", "\t")
	} else {
		result, err = xml.Marshal(doc)
	}
	if err != nil {
		return nil, fmt.Errorf("marshaling to XML: %w", err)
	}

	return result, nil
}

// convertXMLToJSON converts FatturaPA XML to GOBL JSON
func convertXMLToJSON(data []byte, pretty bool) ([]byte, error) {
	// Parse FatturaPA data
	env, err := fatturapa.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("converting from FatturaPA to GOBL: %w", err)
	}

	// Marshal to JSON
	var result []byte
	if pretty {
		result, err = json.MarshalIndent(env, "", "  ")
	} else {
		result, err = json.Marshal(env)
	}
	if err != nil {
		return nil, fmt.Errorf("marshaling to JSON: %w", err)
	}

	return result, nil
}

// loadFatturaPAOptions builds the FatturaPA options from the conversion options
func loadFatturaPAOptions(opts *convertOpts) ([]fatturapa.Option, error) {
	var fatturapOpts []fatturapa.Option

	if opts.transmitter != "" {
		if len(opts.transmitter) < 3 {
			return nil, fmt.Errorf("tax ID must be prefixed by a valid country code")
		}

		countryCode := opts.transmitter[:2]
		taxID := opts.transmitter[2:]

		code := l10n.TaxCountryCode(countryCode)
		err := code.Validate()
		if err != nil {
			return nil, fmt.Errorf("tax ID must be prefixed by a valid country code")
		}

		transmitter := fatturapa.Transmitter{
			CountryCode: countryCode,
			TaxID:       taxID,
		}

		fatturapOpts = append(fatturapOpts, fatturapa.WithTransmitterData(&transmitter))
	}

	if opts.cert != "" {
		cert, err := loadCertificate(opts.cert, opts.password)
		if err != nil {
			return nil, err
		}

		fatturapOpts = append(fatturapOpts, fatturapa.WithCertificate(cert))
	}

	if opts.withTimestamp {
		fatturapOpts = append(fatturapOpts, fatturapa.WithTimestamp())
	}

	return fatturapOpts, nil
}

// loadCertificate loads a certificate from the given file with password
func loadCertificate(certFile, password string) (*xmldsig.Certificate, error) {
	cert, err := xmldsig.LoadCertificate(certFile, password)
	if err != nil {
		return nil, fmt.Errorf("loading certificate %s: %w", certFile, err)
	}

	return cert, nil
}
