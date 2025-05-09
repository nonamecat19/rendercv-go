package entries_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// unchecked lists, per entry type, the declared fields the port does not yet
// validate — so they cannot appear in the error list no matter what value they
// hold.
//
// **It is empty, and every entry type now fails on every declared field.**
// `PublicationEntry.url` was the last one; it landed with the `httpurl` package.
// The map stays because the invariant below needs somewhere to record a future
// gap, and because an empty map is a stronger statement than no map: it says the
// gap was closed rather than never considered.
//
// Entries are written out rather than inferred. Inferring would make the test
// agree with whatever the port happens to do, which is how a gap becomes
// permanent.
var unchecked = map[entries.TypeName][]string{}

// Spec 004 §3.9a behaviors 33a and 33c, as one invariant over all eight entry
// types: the order the binder reports failures in **is** the order the
// descriptor advertises.
//
// The two are now built from one value per type — `bases.ComplexSpec`,
// `bases.DateSpec`, or the own-field slice — so they cannot drift while both
// sites keep reading it. What this test catches is a site that stops:
// re-composing the field list at a `Validate*` call, or at a descriptor, makes
// it fail. It is an invariant over all eight rather than a row per type because
// the iteration-3 defect (base fields composed ahead of own fields) is
// invisible to any test that makes only one field fail.
//
// It deliberately does **not** pin the order against upstream — reordering
// `ComplexSpec` moves the descriptor and the binder together and this test stays
// green. That half is TestDefaultRegistryMatchesUpstreamFieldOrders, which diffs
// the descriptors against a fixture generated from live upstream introspection.
// Neither test is sufficient alone.
//
// Asserted as **equality**, not as a subsequence. The earlier subsequence form
// compared the last element of each location, which silently skipped every
// union-branch record — `date.int` ends in `int`, not in a declared field — and
// so scored the branch pairs as absent rather than as ordered. Comparing the
// first element counts them.
func TestFieldErrorsAreInDescriptorOrder(t *testing.T) {
	// A mapping is wrong for every shape the port checks — text, list and the
	// three date unions alike — so it makes as many fields fail at once as
	// possible. Fields whose failures collapse to one record and fields that
	// emit a branch pair are both reduced to their first location element here,
	// which is the field name in each case.
	const badValue = "\n  wrong: 1\n"

	for _, descriptor := range entries.Default().Descriptors() {
		t.Run(string(descriptor.Name), func(t *testing.T) {
			var document strings.Builder
			for _, field := range descriptor.Fields {
				document.WriteString(field)
				document.WriteString(":")
				document.WriteString(badValue)
			}

			errs, err := entries.Validate(
				parseNode(t, document.String()), descriptor.Name, nil,
				schemaerr.SourceMain, entryReference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}

			seen := map[string]bool{}
			var got []string
			for _, e := range errs {
				if len(e.SchemaLocation) == 0 {
					continue
				}
				name := e.SchemaLocation[0]
				if !seen[name] {
					seen[name] = true
					got = append(got, name)
				}
			}

			skip := map[string]bool{}
			for _, field := range unchecked[descriptor.Name] {
				skip[field] = true
			}
			want := make([]string, 0, len(descriptor.Fields))
			for _, field := range descriptor.Fields {
				if !skip[field] {
					want = append(want, field)
				}
			}

			assertRows(t, got, want)
		})
	}
}

// The `unchecked` map is a record of a gap, so it must name a real field of a
// real type. A stale entry would silently excuse a field from the invariant
// above.
func TestUncheckedNamesRealFields(t *testing.T) {
	registry := entries.Default()
	for typeName, fields := range unchecked {
		var declared []string
		for _, descriptor := range registry.Descriptors() {
			if descriptor.Name == typeName {
				declared = descriptor.Fields
			}
		}
		if declared == nil {
			t.Errorf("unchecked names %q, which is not an entry type", typeName)
			continue
		}
		for _, field := range fields {
			if !contains(declared, field) {
				t.Errorf("unchecked names %s.%s, which %s does not declare",
					typeName, field, typeName)
			}
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
