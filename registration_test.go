package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

// The addon must be registered exactly once. This package pulls in GOBL's
// aggregate addon list, so while GOBL still bundled its own copy of the it-sdi
// addon the key was registered twice: the second registration replaced the map
// entry but appended to the key list, leaving tax.AllAddonDefs indexing past
// the end of its slice.
func TestAddonRegisteredOnce(t *testing.T) {
	count := 0
	assert.NotPanics(t, func() {
		for _, ad := range tax.AllAddonDefs() {
			if ad != nil && ad.Key == cbc.Key("it-sdi-v1") {
				count++
			}
		}
	})
	assert.Equal(t, 1, count)
}
