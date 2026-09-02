package fatturapa_test

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

// This package imports GOBL's aggregate addon list, so a second copy of the
// addon anywhere in the dependency graph registers it-sdi-v1 twice and breaks
// tax.AllAddonDefs.
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
