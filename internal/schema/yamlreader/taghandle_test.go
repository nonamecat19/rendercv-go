package yamlreader_test

import (
	"errors"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// A named tag handle with no `%TAG` directive binding it is ruamel's
// `ParserError: found undefined tag handle` (`ruamel/yaml/parser.py:404-411`),
// raised before the node is ever constructed. goccy has no handle table of its
// own and hands `!e!x` back as an ordinary local tag, so the port used to
// resolve it, build a `TaggedScalar` out of it and report whatever field it
// eventually failed — the wrong message at the wrong location.
//
// Every row is measured against the vendored ruamel: the handle is the
// Python-`repr`'d text of the error, and the two positions are its context and
// problem marks, which `get_yaml_error_location` reads in that order.
func TestAnUndefinedTagHandleIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		handle string
		at     yamldoc.Position
		to     yamldoc.Position
	}{
		{
			name:   "flow sequence value",
			yaml:   "k: [!e!x v]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 5},
		},
		{
			name:   "plain value",
			yaml:   "k: !e!x v\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 4},
		},
		{
			name:   "block sequence entry",
			yaml:   "k:\n- !e!x v\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 2, Column: 3},
		},
		{
			name:   "block mapping key",
			yaml:   "k:\n  !e!x j: v\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 2, Column: 3},
		},
		{
			name:   "flow mapping key",
			yaml:   "k: {!e!x j: v}\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 5},
		},
		{
			name:   "nested in a sequence of mappings",
			yaml:   "k:\n- j:\n    m: !e!x v\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 3, Column: 8},
		},
		{
			name:   "top level",
			yaml:   "!e!x\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 1},
		},
		{
			name:   "the second of two",
			yaml:   "k: [v, !e!x w]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 8},
		},
		{
			name:   "a handle other than the bound one",
			yaml:   "%TAG !e! tag:example.com,2000:\n---\nk: [!f!y v]\n",
			handle: "!f!",
			at:     yamldoc.Position{Line: 3, Column: 5},
		},
		{
			// The scanner reads the handle *after* the short handle it already
			// consumed, so `!!e!x` is the handle `!e!` and the suffix `x` —
			// not the `!!` handle over the suffix `e!x`
			// (`ruamel/yaml/scanner.py:1074-1086`). Measured: it is the same
			// `found undefined tag handle '!e!'`.
			name:   "written after the secondary handle",
			yaml:   "k: [!!e!x v]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 5},
		},
		{
			// A handle's name may carry digits, `-` and `_`
			// (`ruamel/yaml/scanner.py:1634`).
			name:   "a name with digits and a dash",
			yaml:   "k: [!a-1!x v]\n",
			handle: "!a-1!",
			at:     yamldoc.Position{Line: 1, Column: 5},
		},
		{
			// The suffix may itself contain a `!`; only the first one closes
			// the handle. Measured: `!e!x!y` is still the handle `!e!`.
			name:   "a suffix carrying another bang",
			yaml:   "k: [!e!x!y v]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 5},
		},
		{
			// **An anchor moves the marks off the tag.** `parse_node` sets
			// `start_mark` from whichever token it read first, so an anchor
			// before the tag becomes the context mark and the tag stays the
			// problem mark (`ruamel/yaml/parser.py:375-388`). Measured: context
			// column 5, problem column 8.
			name:   "anchored before the tag",
			yaml:   "k: [&a !e!x v]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 5},
			to:     yamldoc.Position{Line: 1, Column: 8},
		},
		{
			// An anchor *after* the tag overwrites **both** marks
			// (`ruamel/yaml/parser.py:399-403`: `start_mark = tag_mark =
			// token.start_mark`), so neither points at the tag. Measured: both
			// column 10, the `&`.
			name:   "anchored after the tag",
			yaml:   "k: [!e!x &a v]\n",
			handle: "!e!",
			at:     yamldoc.Position{Line: 1, Column: 10},
			to:     yamldoc.Position{Line: 1, Column: 10},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)

			var undefined *yamlreader.UndefinedTagHandleError
			if !errors.As(err, &undefined) {
				t.Fatalf("ReadString err = %v, want UndefinedTagHandleError", err)
			}
			if undefined.Handle != test.handle {
				t.Errorf("handle = %q, want %q", undefined.Handle, test.handle)
			}
			if undefined.Start != test.at {
				t.Errorf("start = %+v, want %+v", undefined.Start, test.at)
			}
			// Only an anchored node has two different marks; everywhere else
			// ruamel reports the tag twice, so the row leaves `to` unset.
			end := test.to
			if end == (yamldoc.Position{}) {
				end = test.at
			}
			if undefined.End != end {
				t.Errorf("end = %+v, want %+v", undefined.End, end)
			}
		})
	}
}

// The refusal is the handle table's, so everything the table binds still
// resolves, and every tag that names no handle at all is untouched. Each row is
// measured against the vendored ruamel: the first four load, and the last two
// are failures of a different kind that this check must not claim.
func TestADefinedTagHandleIsUntouched(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "bound named handle", yaml: "%TAG !e! tag:example.com,2000:\n---\nk: [!e!x v]\n"},
		{
			name: "bound named handle after the secondary one",
			yaml: "%TAG !e! tag:example.com,2000:\n---\nk: [!!e!x v]\n",
		},
		{name: "local tag", yaml: "k: [!foo v]\n"},
		{name: "secondary handle", yaml: "k: [!!str v]\n"},
		{name: "the non-specific tag", yaml: "k: [! v]\n"},
		{name: "a bare secondary handle", yaml: "k: [!! v]\n"},
		{name: "a verbatim tag", yaml: "k: !<tag:example.com,2000:x> v\n"},
		{name: "a bang inside a quoted scalar", yaml: "k: ['!e!x v']\n"},
		{name: "a bang inside a plain scalar", yaml: "k: a!e!x\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)

			var undefined *yamlreader.UndefinedTagHandleError
			if errors.As(err, &undefined) {
				t.Fatalf("ReadString err = %v, want no UndefinedTagHandleError", err)
			}
		})
	}
}

// A `%TAG` binding reaches a tag written after the secondary handle, because
// the handle the scanner reads out of `!!e!x` is `!e!` and not `!!`. Measured:
// upstream reprs it as `Tag('tag:example.com,2000:x')`, where the port read the
// handle as `!!` and produced `tag:yaml.org,2002:e!x`.
func TestASecondaryHandlePrefixDoesNotSwallowTheNamedHandle(t *testing.T) {
	doc, err := yamlreader.ReadString(
		"%TAG !e! tag:example.com,2000:\n---\nk: [!!e!x v]\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	tagged := doc.Items[0].Value.Elems[0]
	if got, want := tagged.Tag, "tag:example.com,2000:x"; got != want {
		t.Errorf("tag = %q, want %q", got, want)
	}
}
