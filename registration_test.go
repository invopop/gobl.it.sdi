package fatturapa_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

// TestAddonRegisteredOnce pins that it-sdi-v1 is registered exactly once; a
// second registration anywhere in the dependency graph makes tax.AllAddonDefs
// index past the end of its key list.
func TestAddonRegisteredOnce(t *testing.T) {
	count := 0
	assert.NotPanics(t, func() {
		for _, ad := range tax.AllAddonDefs() {
			if ad != nil && ad.Key == sdi.V1 {
				count++
			}
		}
	})
	assert.Equal(t, 1, count)
}
