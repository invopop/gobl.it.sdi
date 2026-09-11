package sdi_test

import (
	"regexp"
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

func TestRetainedRateExtensionPattern(t *testing.T) {
	def := tax.ExtensionForKey(sdi.ExtKeyRetainedRate)
	require.NotNil(t, def, "it-sdi-retained-rate must be registered")
	re := regexp.MustCompile(def.Pattern)

	for _, v := range []string{"23.00", "4.60", "100.00"} {
		assert.True(t, re.MatchString(v), v)
	}
	for _, v := range []string{"23", "23.0", "23.000", "101.00", "23,00", "-1.00"} {
		assert.False(t, re.MatchString(v), v)
	}
}
