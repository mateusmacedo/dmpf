package postgres

import (
	"slices"
	"testing"
)

func TestEveryCapabilityTableDeclaresItsGrant(t *testing.T) {
	for capability, parts := range capabilities {
		granted := map[string]bool{}
		for _, g := range parts.grants {
			granted[g.Table] = true
		}
		for _, table := range parts.tables {
			if !granted[table] {
				t.Errorf("capability %s creates %s without declaring its grant", capability, table)
			}
		}
		for table := range granted {
			if !slices.Contains(parts.tables, table) {
				t.Errorf("capability %s grants %s, a table it does not create", capability, table)
			}
		}
	}
}
