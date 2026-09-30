package fatturapa

import (
	"fmt"
	"time"

	"github.com/invopop/gobl"
	"github.com/invopop/xmldsig"
)

var xadesConfig = &xmldsig.XAdESConfig{
	Description: "Fattura PA",
}

// sign produces the XAdES signature of a document that has none yet, over its
// canonical representation as defined in https://www.w3.org/TR/2001/REC-xml-c14n-20010315
// (for a simpler explanation look at https://www.di-mgt.com.au/xmldsig-c14n.html).
func sign(env *gobl.Envelope, doc any, config *config) (*xmldsig.Signature, error) {
	buf, err := marshal(doc, "")
	if err != nil {
		return nil, fmt.Errorf("converting to canonincal format: %w", err)
	}

	dsigOpts := []xmldsig.Option{
		xmldsig.WithDocID(env.Head.UUID.String()),
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

	return xmldsig.Sign(buf.Bytes(), dsigOpts...)
}
