package entries_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var dispatchReference = time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC)

func parseNode(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return node
}

// parseValue yields a non-mapping node. The reader rejects a document whose root
// is not a mapping, so a bare scalar has to be reached through one — which is
// also how a real entry arrives, as a section list element.
func parseValue(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	return parseNode(t, "value: "+src+"\n").Items[0].Value
}

// Spec §3.18 behavior 43. Every name the registry advertises must dispatch —
// derived from Names() rather than a literal list, so adding a type to the
// registry without an arm fails here instead of at runtime.
func TestValidateHasAnArmForEveryRegisteredName(t *testing.T) {
	// A minimal valid document per type. TextEntry's is a bare string; the rest
	// are mappings carrying their required fields.
	valid := map[entries.TypeName]string{
		"OneLineEntry":          "label: Languages\ndetails: English\n",
		"NormalEntry":           "name: Some Project\n",
		"ExperienceEntry":       "company: Google\nposition: Engineer\n",
		"EducationEntry":        "institution: MIT\narea: Computer Science\n",
		"PublicationEntry":      "title: A Paper\nauthors:\n  - J. Doe\n",
		"BulletEntry":           "bullet: Did a thing\n",
		"NumberedEntry":         "number: First\n",
		"ReversedNumberedEntry": "reversed_number: Latest\n",
		"TextEntry":             "just text",
	}

	names := entries.Default().Names()
	if len(names) != 9 {
		t.Fatalf("registry advertises %d names, want 9", len(names))
	}

	for _, name := range names {
		t.Run(string(name), func(t *testing.T) {
			src, ok := valid[name]
			if !ok {
				t.Fatalf("no sample document for %q; the test table is out of date", name)
			}

			var node *yamldoc.Node
			if name == "TextEntry" {
				node = parseValue(t, src)
			} else {
				node = parseNode(t, src)
			}

			errs, err := entries.Validate(
				node, name, []string{"cv", "sections", "x", "0"},
				schemaerr.SourceMain, dispatchReference,
			)
			if err != nil {
				t.Fatalf("Validate(%q) returned an internal error: %v", name, err)
			}
			if len(errs) != 0 {
				t.Errorf("Validate(%q) errs = %+v, want none", name, errs)
			}
		})
	}
}

// An unregistered name is a caller bug, not user input, so it is an internal
// error rather than a validation error.
func TestValidateRejectsAnUnregisteredName(t *testing.T) {
	errs, err := entries.Validate(
		parseNode(t, "bullet: x\n"), "NoSuchEntry", nil, schemaerr.SourceMain, dispatchReference,
	)
	if errs != nil {
		t.Errorf("errs = %+v, want none", errs)
	}

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
}

// Spec §3.18 behavior 43 — the entry-level codes are reachable through the
// dispatcher and distinguishable from each other. Matched on code, never on
// message text, because the texts are iteration 4's (spec §7.3).
func TestValidateReachesEachEntryLevelCode(t *testing.T) {
	tests := []struct {
		name     string
		typeName entries.TypeName
		src      string
		scalar   bool
		wantCode schemaerr.Code
	}{
		{
			name:     "missing required field",
			typeName: "BulletEntry",
			src:      "date: 2020\n",
			wantCode: "missing",
		},
		{
			name:     "a text field is not text",
			typeName: "BulletEntry",
			src:      "bullet:\n  a: 1\n",
			wantCode: "string_type",
		},
		{
			name:     "a list field is not a list",
			typeName: "PublicationEntry",
			src:      "title: T\nauthors: notalist\n",
			wantCode: "list_type",
		},
		{
			name:     "an exact date is unparseable",
			typeName: "EducationEntry",
			src:      "institution: MIT\narea: CS\nstart_date: aaa\n",
			wantCode: "value_error",
		},
		{
			name:     "the entry is not a mapping",
			typeName: "BulletEntry",
			src:      "just text",
			scalar:   true,
			wantCode: "model_type",
		},
	}

	seen := map[schemaerr.Code]bool{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var node *yamldoc.Node
			if test.scalar {
				node = parseValue(t, test.src)
			} else {
				node = parseNode(t, test.src)
			}

			errs, err := entries.Validate(
				node, test.typeName, []string{"cv"},
				schemaerr.SourceMain, dispatchReference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			if len(errs) == 0 {
				t.Fatalf("errs is empty, want a %q", test.wantCode)
			}

			var found bool
			for _, e := range errs {
				if e.Code == test.wantCode {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("errs = %+v, want one with code %q", errs, test.wantCode)
			}
			seen[test.wantCode] = true
		})
	}

	if len(seen) != len(tests) {
		t.Errorf("reached %d distinct codes, want %d — two rows share a code", len(seen), len(tests))
	}
}
