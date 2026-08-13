package modelbuilder

import (
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// directiveFailure is ruamel's verdict on a directive block: the message it
// opens with and the mark it points at. Neither is in goccy's text.
type directiveFailure struct {
	message string
	at      yamldoc.Position
}

// directiveScan reconstructs ruamel's verdict for the failures goccy spells
// `unexpected directive value. document not started`. ok is false when the scan
// cannot name one, which is where goccy's own line is left to reach the user.
//
// **One goccy failure, four ruamel outcomes**, and goccy's text distinguishes
// none of them — it blames the *first* directive line at column 1 in every
// case. goccy accepts at most one directive per stream; ruamel processes them
// all in order (`ruamel/yaml/parser.py:288-330`) and so answers:
//
//   - `found duplicate YAML directive` at the second `%YAML`, column 1
//     (`parser.py:293-296`);
//   - `duplicate tag handle '<handle>'` at the second `%TAG` binding the same
//     handle, column 1 (`parser.py:307-310`), the handle Python-`repr`'d;
//   - `mapping values are not allowed here` at the first content line when no
//     `---` marker follows the directives — ruamel's *scanner*, not its parser,
//     because without the marker the directive block runs into the document;
//   - nothing at all: `%YAML 1.2\n%TAG !e! a:\n---\nk: v` is a valid document
//     upstream renders, and goccy cannot parse it. The scan declines there.
//
// Order matters and is ruamel's own: directives are processed as they are read,
// so a duplicate is reported before the missing marker, and the first duplicate
// in the stream wins over a later one of the other kind. Measured on all four
// outcomes plus `%YAML 1.2\n%YAML 1.1\n---`, `%YAML 1.2\n%TAG !e! a:\n%TAG !e!
// b:\n---` and `%TAG !! a:\n%TAG !! b:\n---`.
func directiveScan(content string) (directiveFailure, bool) {
	lines := strings.Split(content, "\n")

	seenYaml := false
	handles := map[string]bool{}
	body := 0 // the first line after the directive block, 1-based

	for i, text := range lines {
		if !strings.HasPrefix(text, "%") {
			body = i + 1
			break
		}
		at := yamldoc.Position{Line: i + 1, Column: 1}

		switch name, values := directiveFields(text); name {
		case "YAML":
			if seenYaml {
				return directiveFailure{message: "found duplicate YAML directive", at: at}, true
			}
			seenYaml = true
		case "TAG":
			if len(values) == 0 {
				continue
			}
			if handles[values[0]] {
				return directiveFailure{
					message: "duplicate tag handle " + schemaerr.PythonStringRepr(values[0]),
					at:      at,
				}, true
			}
			handles[values[0]] = true
		}
	}

	// A `---` marker after the directives means the document is well formed and
	// goccy simply cannot read it — nothing for this function to say.
	if body == 0 || strings.HasPrefix(lines[body-1], "---") {
		return directiveFailure{}, false
	}

	// **No marker: ruamel's scanner runs the directive block into the document
	// and stops at the first key indicator.** The column is the first colon on
	// that line, the rule `offendingKeyLine` already measured. A line carrying
	// none is a different ruamel failure — `expected '<document start>', but
	// found ('<scalar>',)`, whose found-token spelling is not reconstructible
	// here — so the scan declines it, as `blockScan` declines its own.
	colon := strings.IndexByte(lines[body-1], ':')
	if colon < 0 {
		return directiveFailure{}, false
	}
	return directiveFailure{
		message: "mapping values are not allowed here",
		at:      yamldoc.Position{Line: body, Column: colon + 1},
	}, true
}

// directiveFields splits a directive line into its name and its arguments:
// `%TAG !e! tag:x,1:` into `TAG` and `!e!`, `tag:x,1:`.
func directiveFields(text string) (name string, values []string) {
	fields := strings.Fields(strings.TrimPrefix(text, "%"))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
