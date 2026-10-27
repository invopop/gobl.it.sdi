package sdi_test

import (
	"testing"

	sdi "github.com/invopop/gobl.it.sdi/addon"
	"github.com/invopop/gobl/rules"
	"github.com/stretchr/testify/assert"
)

// Fault codes are a public contract, so each one must name a single rule.
// Two branches adding rules at the same time can easily pick the same next
// number without a merge conflict.
func TestRuleCodesAreUnique(t *testing.T) {
	seen := make(map[rules.Code]string)
	var walk func(s *rules.Set)
	walk = func(s *rules.Set) {
		for _, a := range s.Assert {
			if prev, ok := seen[a.ID]; ok {
				t.Errorf("%s is used by both %q and %q", a.ID, prev, a.Desc)
			}
			seen[a.ID] = a.Desc
		}
		for _, sub := range s.Subsets {
			walk(sub)
		}
	}
	for _, s := range rules.Registry() {
		if s.Package == sdi.Key.String() {
			walk(s)
		}
	}
	assert.NotEmpty(t, seen, "no rules registered for the addon")
}
