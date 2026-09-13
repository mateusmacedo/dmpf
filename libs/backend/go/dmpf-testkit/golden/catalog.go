package golden

import (
	"errors"
	"fmt"
	"sort"
)

// ErrAmbiguousFixture: two fixtures for one contract major would let each
// stack pass against a different oracle (FIX-11).
var ErrAmbiguousFixture = errors.New("golden: more than one fixture for the same contract major")

// Catalog holds one fixture per identity — qualified contract name plus
// contract major — and refuses a second, naming both paths.
type Catalog struct {
	entries map[string]entry
}

type entry struct {
	path    string
	fixture Fixture
}

func identityKey(f Fixture) string {
	return f.Identity.Contract.Package + "." + f.Identity.Contract.Message + "@" + f.Covers.ContractMajor
}

func (c *Catalog) Add(path string, f Fixture) error {
	if c.entries == nil {
		c.entries = map[string]entry{}
	}
	key := identityKey(f)
	if prev, dup := c.entries[key]; dup {
		return fmt.Errorf("%w: %s declared by %s and %s", ErrAmbiguousFixture, key, prev.path, path)
	}
	c.entries[key] = entry{path: path, fixture: f}
	return nil
}

// Lookup finds the canonical fixture of a contract major and the path it came
// from.
func (c *Catalog) Lookup(pkg, message, contractMajor string) (Fixture, string, bool) {
	e, ok := c.entries[pkg+"."+message+"@"+contractMajor]
	return e.fixture, e.path, ok
}

func (c *Catalog) Len() int { return len(c.entries) }

// Keys lists the identities in a stable order.
func (c *Catalog) Keys() []string {
	out := make([]string, 0, len(c.entries))
	for k := range c.entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
