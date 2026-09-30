package fatturapa

import (
	"fmt"
	"time"

	"github.com/invopop/xmldsig"
)

var xadesConfig = &xmldsig.XAdESConfig{
	Description: "Fattura PA",
}

func (d *OrdinaryInvoice) sign(config *config) error {
	data, err := d.canonical()
	if err != nil {
		return fmt.Errorf("converting to canonincal format: %w", err)
	}

	dsigOpts := []xmldsig.Option{
		xmldsig.WithDocID(d.env.Head.UUID.String()),
		xmldsig.WithXAdES(xadesConfig),
	}

	if config.Certificate != nil {
		dsigOpts = append(dsigOpts, xmldsig.WithCertificate(config.Certificate))
	}

	if config.WithTimestamp {
		dsigOpts = append(dsigOpts, xmldsig.WithTimestamp(xmldsig.TimestampFreeTSA))
	}

	if config.WithCurrentTime != (time.Time{}) {
		dsigOpts = append(dsigOpts, xmldsig.WithCurrentTime(func() time.Time {
			return config.WithCurrentTime
		}))
	}

	sig, err := xmldsig.Sign(data, dsigOpts...)
	if err != nil {
		return err
	}

	d.Signature = sig

	return nil
}

// Canonical converts a struct representation of fatturapa to its
// canonical representation as defined in https://www.w3.org/TR/2001/REC-xml-c14n-20010315
// (for a simpler explanation look at https://www.di-mgt.com.au/xmldsig-c14n.html)
// This is used when we need to create a hash for signing, timestamping, ...
func (d *OrdinaryInvoice) canonical() ([]byte, error) {
	buf, err := d.buffer("")
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
