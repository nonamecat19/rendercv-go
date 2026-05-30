//go:build conformance

package clidiff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// base is a document that renders, in LF. Every CRLF and CR variant below is
// derived from it so the two halves of a row differ in nothing but line
// endings.
const base = "cv:\n  name: John Doe\n  sections:\n    test_section:\n    - this is a text entry.\n"

// syntaxError is a document with a block-mapping fault on its last line.
// Upstream reports it as a span — `line 2 to line 6` — which is what makes it
// the probe for the knock-on effect: a translation that changed the line count
// would move both ends.
const syntaxError = "cv:\n  name: John Doe\n  sections:\n    test_section:\n    - a\n   b: c\n"

// crlf and cr rewrite a document's line endings.
func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
func cr(s string) string   { return strings.ReplaceAll(s, "\n", "\r") }

// TestNewlineTranslationOnRejectedDocuments is the reported half of spec delta
// 002-P §6: upstream reads every input through
// `pathlib.Path.read_text(encoding="utf-8")` (`run_rendercv.py:115,140`,
// `render_command.py:212-215`), whose default `newline=None` translates `\r\n`
// and `\r` to `\n` before ruamel sees the text.
//
// Each row here is a document ruamel rejects, so nothing is generated and both
// streams are comparable byte for byte — including the **span** in the Location
// column, which is the part translation moves. Measured before the fix, the
// port answered `cv:\n  name: \rA\n` with `while parsing a block mapping.` at
// `line 1 to line 3` where upstream answers `while scanning a simple key.` at
// `line 3 to line 4`.
func TestNewlineTranslationOnRejectedDocuments(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		// The probe from §6, and its already-agreeing LF twin: the two
		// documents are the same after translation, so they must produce the
		// same panel as each other as well as the same panel as upstream.
		{"a lone CR mid-value", "cv:\n  name: \rA\n"},
		{"the LF twin of that document", "cv:\n  name: \nA\n"},

		// The knock-on: a reported span in a document with no LF at all. The
		// numbers only line up if the port counts lines in the translated
		// text.
		{"a span in an LF document", syntaxError},
		{"a span in a CRLF document", crlf(syntaxError)},
		{"a span in a CR-only document", cr(syntaxError)},

		// A CR the YAML layer would otherwise have kept.
		{
			"a CR inside a literal block scalar",
			"cv:\n  name: John Doe\n  sections:\n    test_section:\n    - |\n      first\rsecond\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream, port := differential(t, invocation{
				args:  append([]string{"render", "CV.yaml"}, noGeneration...),
				files: map[string]string{"CV.yaml": tc.yaml},
			})

			t.Logf("upstream: %s\nport:     %s", upstream, port)

			compare(t, upstream, port)
			compareRaw(t, "stdout", upstream.stdout, port.stdout)
			compareRaw(t, "stderr", upstream.stderr, port.stderr)
		})
	}
}

// TestNewlineTranslationOnOverlayFiles holds the same rule on the three overlay
// flags, which upstream reads with the same call
// (`render_command.py:212,213,215`). Measured before the fix, a lone `\r` in a
// `--settings` file moved the port's span to `line 1 to line 3` where upstream
// reports `line 3 to line 4`, exactly as in the main file.
func TestNewlineTranslationOnOverlayFiles(t *testing.T) {
	cases := []struct {
		name    string
		flag    string
		overlay string
		content string
	}{
		{
			name: "a lone CR in a settings overlay", flag: "-s", overlay: "s.yaml",
			content: "settings:\n  render_command:\n    dont_generate_png: \rtrue\n",
		},
		{
			name: "a lone CR in a design overlay", flag: "-d", overlay: "d.yaml",
			content: "design:\n  theme: \rclassic\n",
		},
		{
			name: "a lone CR in a locale overlay", flag: "-lc", overlay: "l.yaml",
			content: "locale:\n  month: \rmonth\n",
		},
		{
			name: "a CRLF design overlay", flag: "-d", overlay: "d.yaml",
			content: crlf("design:\n  theme: engineeringresumes\n  foo: [1,\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream, port := differential(t, invocation{
				args: append([]string{"render", "CV.yaml", tc.flag, tc.overlay}, noGeneration...),
				files: map[string]string{
					"CV.yaml":  base,
					tc.overlay: tc.content,
				},
			})

			t.Logf("upstream: %s\nport:     %s", upstream, port)

			compare(t, upstream, port)
			compareRaw(t, "stdout", upstream.stdout, port.stdout)
			compareRaw(t, "stderr", upstream.stderr, port.stderr)
		})
	}
}

// TestNewlineTranslationOnRenderedDocuments is the accepted half, and it is as
// load-bearing as the rejected one: a translation applied too eagerly, or one
// that dropped a break instead of folding it, would still agree on every
// rejected row while silently changing an artifact.
//
// These rows generate, so the panel carries durations and cannot be compared
// byte for byte. What is compared instead is stronger than the panel text: the
// panel's geometry, the set of files created — a name is part of it, and
// `name: "A<CR>B"` is the row where the port wrote `A\nB_CV.typ` and upstream
// `A_B_CV.typ` — and the **bytes of every generated artifact**.
func TestNewlineTranslationOnRenderedDocuments(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"an LF document", base},
		{"a CRLF document", crlf(base)},
		{"a CR-only document", cr(base)},
		{
			"mixed CR, LF and CRLF",
			"cv:\r\n  name: John Doe\n  sections:\r    test_section:\r\n    - this is a text entry.\n",
		},
		{"a CR at end of file", base + "\r"},
		{"a CRLF at end of file", base + "\r\n"},

		// The scalar rows. Upstream folds each of these to the name `A B`,
		// because after translation the CR is a line break inside a
		// multi-line flow scalar.
		{
			"a CR inside a double-quoted scalar",
			"cv:\n  name: \"A\rB\"\n  sections:\n    test_section:\n    - this is a text entry.\n",
		},
		{
			"a CRLF inside a double-quoted scalar",
			"cv:\n  name: \"A\r\nB\"\n  sections:\n    test_section:\n    - this is a text entry.\n",
		},
		{
			"a CR inside a single-quoted scalar",
			"cv:\n  name: 'A\rB'\n  sections:\n    test_section:\n    - this is a text entry.\n",
		},
		{
			"a CRLF inside a literal block scalar",
			"cv:\n  name: John Doe\n  sections:\n    test_section:\n    - |\n      first\r\n      second\n",
		},

		// An escaped `\r` is produced by the scanner, after the read boundary,
		// and upstream carries the carriage return into the Markdown. The
		// artifact comparison below is what pins that it still does.
		{
			"an escaped CR in a double-quoted scalar",
			"cv:\n  name: John Doe\n  sections:\n    test_section:\n    - \"first\\rsecond\"\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream, port := differential(t, invocation{
				args: []string{
					"render", "CV.yaml", "--settings.current_date", "2025-03-05",
					"--dont-generate-pdf", "--dont-generate-png",
				},
				files: map[string]string{"CV.yaml": tc.yaml},
			})

			t.Logf("upstream: %s\nport:     %s", upstream, port)

			if upstream.exit != port.exit {
				t.Errorf("exit code: upstream %d, port %d", upstream.exit, port.exit)
			}
			compareFrames(t, upstream.stdout, port.stdout)
			compareRaw(t, "stderr", upstream.stderr, port.stderr)
			compareTrees(t, upstream, port)
			compareArtifacts(t, upstream, port)
		})
	}
}

// compareTrees asserts both sides created the same files. It is `compare`'s
// tree check on its own, for the rows whose stdout carries a duration.
func compareTrees(t *testing.T, upstream, port outcome) {
	t.Helper()

	u, p := strings.Join(upstream.tree, "\n"), strings.Join(port.tree, "\n")
	if u != p {
		t.Errorf("created file tree:\nupstream:\n%s\nport:\n%s", u, p)
	}
}

// compareArtifacts asserts every file both sides created is byte-identical.
// Only the files upstream created are read, because compareTrees has already
// failed the row if the two sets differ.
func compareArtifacts(t *testing.T, upstream, port outcome) {
	t.Helper()

	for _, rel := range upstream.tree {
		want, err := os.ReadFile(filepath.Join(upstream.dir, rel))
		if err != nil {
			t.Fatalf("reading upstream %s: %v", rel, err)
		}
		got, err := os.ReadFile(filepath.Join(port.dir, rel))
		if err != nil {
			t.Errorf("reading port %s: %v", rel, err)
			continue
		}
		if string(want) != string(got) {
			t.Errorf("%s differs byte for byte:\nupstream:\n%q\nport:\n%q", rel, want, got)
		}
	}
}
