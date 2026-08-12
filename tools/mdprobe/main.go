// Command mdprobe generates the two python-markdown differentials —
// internal/renderer/templater/process/testdata/html.json and
// .../markdown_to_typst.json — by running the vendored library over every shape
// each fixture already holds.
//
// **The fixture had no generator until this tool, and that is what it is for.**
// Their 761 and 428 rows are upstream's exact output and `AGENTS.md` §10.1
// forbids hand-writing them, but every row added since spec 011's T7 was
// produced by running the library at a prompt and pasting the answer. That is a
// claim, not a check: a mistyped `Out` is indistinguishable from a real
// upstream behavior, and a fixture that encodes one is worse than no fixture,
// because the suite then defends the mistake.
//
// **What is authored and what is generated.** The `In` column is authored — a
// shape someone chose to pin. The `Out` column is generated here and nowhere
// else. Adding a row means adding an input with `-add`, never writing an
// output.
//
// # The gate
//
// By default this tool **verifies and refuses to write**: it regenerates every
// existing row and reports any whose stored `Out` upstream does not reproduce.
// A row that does not reproduce is a finding — either the fixture is wrong or
// the submodule moved — and it goes to a human, not through a rewrite. Passing
// `-write` after a clean verify is what actually replaces the file.
//
// The serialization is Python's own `json.dumps`, because that is what the
// committed files are — and the two differ: `html.json` is `indent=2` with the
// keys `In`/`Out`, `markdown_to_typst.json` is `indent=1` with `in`/`want`.
// Both are `ensure_ascii=False` with a trailing newline. The bytes come out of
// the vendored runtime rather than being re-encoded on the Go side.
//
// Usage:
//
//	go run ./tools/mdprobe                          # verify both fixtures
//	go run ./tools/mdprobe -write                   # verify, then rewrite them
//	go run ./tools/mdprobe -fixture typst -add shapes.json -write
//
// `-add` takes a JSON array of input strings for the selected fixture; shapes
// already in it are ignored, and new ones are appended in the order given.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const upstreamDir = "third_party/rendercv"

// fixture is one differential: where it lives, what its columns are called, and
// which conversion fills it.
type fixture struct {
	// name is what `-fixture` selects it by.
	name string
	path string
	// inKey and outKey are the fixture's own column names, which the two files
	// spell differently.
	inKey, outKey string
	// indent is the `json.dumps` indent the committed file was written with.
	indent int
	// convert is the vendored callable, as a Python expression taking one
	// string.
	convert string
}

var fixtures = []fixture{{
	name:    "html",
	path:    "internal/renderer/templater/process/testdata/html.json",
	inKey:   "In",
	outKey:  "Out",
	indent:  2,
	convert: "markdown.markdown",
}, {
	name:   "typst",
	path:   "internal/renderer/templater/process/testdata/markdown_to_typst.json",
	inKey:  "in",
	outKey: "want",
	indent: 1,
	// **Not `markdown.markdown`.** The Typst path is upstream's own module-level
	// `Markdown` instance with four block processors deregistered and a Typst
	// output format, and it converts line by line
	// (`markdown_parser.py:147-190`). Calling the wrong one would produce a
	// fixture that looks plausible and pins nothing.
	convert: "markdown_to_typst",
}}

// convertScript reads the shapes from stdin and prints the fixture.
//
// `markdown.markdown` builds a fresh `Markdown` per call, which is what
// `markdown_to_html` does too (`markdown_parser.py:202`), so no row can be
// affected by the row before it.
const convertScript = `
import json
import sys

import markdown
from rendercv.renderer.templater.markdown_parser import markdown_to_typst

request = json.load(sys.stdin)
convert = {"markdown.markdown": markdown.markdown, "markdown_to_typst": markdown_to_typst}[
    request["convert"]
]
rows = [
    {request["in_key"]: shape, request["out_key"]: convert(shape)}
    for shape in request["shapes"]
]
print(json.dumps(rows, indent=request["indent"], ensure_ascii=False))
`

func main() {
	write := flag.Bool("write", false, "rewrite the fixtures once every existing row reproduces")
	add := flag.String("add", "", "a JSON array of new input shapes to append to -fixture")
	only := flag.String("fixture", "", "act on one fixture only: html or typst")
	flag.Parse()

	if err := run(*write, *add, *only); err != nil {
		fmt.Fprintf(os.Stderr, "mdprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(write bool, addPath, only string) error {
	if addPath != "" && only == "" {
		return fmt.Errorf("-add needs -fixture: the two differentials take different shapes")
	}

	matched := false
	for _, f := range fixtures {
		if only != "" && only != f.name {
			continue
		}
		matched = true
		shapes := addPath
		if only != f.name {
			shapes = ""
		}
		if err := f.regenerate(write, shapes); err != nil {
			return err
		}
	}
	if !matched {
		return fmt.Errorf("no fixture named %q; have html and typst", only)
	}
	return nil
}

func (f fixture) regenerate(write bool, addPath string) error {
	existing, err := f.read()
	if err != nil {
		return err
	}

	shapes := make([]string, 0, len(existing))
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		shapes = append(shapes, r.in)
		seen[r.in] = struct{}{}
	}

	added := 0
	if addPath != "" {
		newShapes, err := readShapes(addPath)
		if err != nil {
			return err
		}
		for _, shape := range newShapes {
			if _, ok := seen[shape]; ok {
				continue
			}
			seen[shape] = struct{}{}
			shapes = append(shapes, shape)
			added++
		}
	}

	generated, err := f.convertAll(shapes)
	if err != nil {
		return err
	}

	rows, err := f.decode(generated)
	if err != nil {
		return fmt.Errorf("upstream did not print the fixture: %w", err)
	}
	if len(rows) != len(shapes) {
		return fmt.Errorf("upstream returned %d rows for %d shapes", len(rows), len(shapes))
	}

	// The gate: every row that is already committed has to come back
	// unchanged.
	produced := make(map[string]string, len(rows))
	for _, r := range rows {
		produced[r.in] = r.out
	}
	mismatches := 0
	for _, r := range existing {
		if out := produced[r.in]; out != r.out {
			mismatches++
			fmt.Fprintf(os.Stderr, "%s does not reproduce: in %q\n  fixture   %q\n  upstream  %q\n",
				f.name, r.in, r.out, out)
		}
	}
	if mismatches > 0 {
		return fmt.Errorf("%s: %d of %d rows do not reproduce; nothing written —"+
			" a row that does not reproduce is a finding, not a rewrite",
			f.name, mismatches, len(existing))
	}

	fmt.Printf("%s: %d rows reproduce", f.name, len(existing))
	if added > 0 {
		fmt.Printf(", %d new shapes", added)
	}
	fmt.Println()

	if !write {
		fmt.Println("not written (pass -write)")
		return nil
	}
	if err := os.WriteFile(f.path, generated, 0o644); err != nil { //nolint:gosec // committed fixture
		return err
	}
	fmt.Printf("wrote %s (%d rows, %d bytes)\n", f.path, len(rows), len(generated))
	return nil
}

// row is one fixture entry, decoded through whichever pair of column names the
// file uses.
type row struct{ in, out string }

func (f fixture) read() ([]row, error) {
	raw, err := os.ReadFile(f.path) //nolint:gosec // a path this file names
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.path, err)
	}
	rows, err := f.decode(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", f.path, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s holds no rows", f.path)
	}
	return rows, nil
}

func (f fixture) decode(raw []byte) ([]row, error) {
	var records []map[string]string
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(records))
	for i, record := range records {
		in, ok := record[f.inKey]
		if !ok {
			return nil, fmt.Errorf("row %d has no %q column", i, f.inKey)
		}
		out, ok := record[f.outKey]
		if !ok {
			return nil, fmt.Errorf("row %d has no %q column", i, f.outKey)
		}
		rows = append(rows, row{in: in, out: out})
	}
	return rows, nil
}

func readShapes(path string) ([]string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a path the operator passed
	if err != nil {
		return nil, err
	}
	var shapes []string
	if err := json.Unmarshal(raw, &shapes); err != nil {
		return nil, fmt.Errorf("%s is not a JSON array of strings: %w", filepath.Base(path), err)
	}
	return shapes, nil
}

// convertAll runs the vendored library over the shapes and returns the fixture
// bytes it printed.
func (f fixture) convertAll(shapes []string) ([]byte, error) {
	input, err := json.Marshal(map[string]any{
		"shapes":  shapes,
		"convert": f.convert,
		"in_key":  f.inKey,
		"out_key": f.outKey,
		"indent":  f.indent,
	})
	if err != nil {
		return nil, err
	}

	command := exec.Command("uv", "run", "--frozen", "--all-extras", "python", "-c", convertScript)
	command.Dir = upstreamDir
	command.Stdin = bytes.NewReader(input)
	command.Stderr = os.Stderr

	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("running uv (is the submodule initialized? `just setup`): %w", err)
	}
	return out, nil
}
