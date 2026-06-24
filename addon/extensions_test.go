package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationExtensionRegistered(t *testing.T) {
	def := tax.ExtensionForKey(sdi.ExtKeyNotification)
	require.NotNil(t, def, "it-sdi-notification must be registered")

	got := make([]cbc.Code, 0, len(def.Values))
	for _, v := range def.Values {
		got = append(got, v.Code)
	}
	assert.ElementsMatch(t,
		[]cbc.Code{"RC", "NS", "MC", "AT", "DT", "EC01", "EC02"}, got)
}
