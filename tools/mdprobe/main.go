// Command mdprobe generates the python-markdown differential,
// internal/renderer/templater/process/testdata/html.json, by running the
// vendored library over every shape the fixture already holds.
//
// **The fixture had no generator until this tool, and that is what it is for.**
// Its 761 rows are upstream's exact output and `AGENTS.md` §10.1 forbids
// hand-writing them, but every row added since spec 011's T7 was produced by
// running `markdown.markdown` at a prompt and pasting the answer. That is a
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
// The serialization is Python's own `json.dumps(rows, indent=2,
// ensure_ascii=False)` with a trailing newline, because that is what the
// committed file is; the bytes come out of the vendored runtime rather than
// being re-encoded on the Go side.
//
// Usage:
//
//	go run ./tools/mdprobe                      # verify all rows reproduce
//	go run ./tools/mdprobe -write               # verify, then rewrite the file
//	go run ./tools/mdprobe -add shapes.json -write
//
// `-add` takes a JSON array of input strings; shapes already in the fixture are
// ignored, and new ones are appended in the order given.
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

const (
	upstreamDir = "third_party/rendercv"
	outPath     = "internal/renderer/templater/process/testdata/html.json"
)

// convertScript reads the shapes from stdin and prints the fixture.
//
// `markdown.markdown` builds a fresh `Markdown` per call, which is what
// `markdown_to_html` does too (`markdown_parser.py:202`), so no row can be
// affected by the row before it.
const convertScript = `
import json
import sys

import markdown

shapes = json.load(sys.stdin)
rows = [{"In": shape, "Out": markdown.markdown(shape)} for shape in shapes]
print(json.dumps(rows, indent=2, ensure_ascii=False))
`

// row is one fixture entry. The field names are the fixture's.
type row struct {
	In  string `json:"In"`
	Out string `json:"Out"`
}

func main() {
	write := flag.Bool("write", false, "rewrite the fixture once every existing row reproduces")
	add := flag.String("add", "", "a JSON array of new input shapes to append")
	flag.Parse()

	if err := run(*write, *add); err != nil {
		fmt.Fprintf(os.Stderr, "mdprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(write bool, addPath string) error {
	existing, err := readFixture()
	if err != nil {
		return err
	}

	shapes := make([]string, 0, len(existing))
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		shapes = append(shapes, r.In)
		seen[r.In] = struct{}{}
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

	generated, err := convert(shapes)
	if err != nil {
		return err
	}

	var rows []row
	if err := json.Unmarshal(generated, &rows); err != nil {
		return fmt.Errorf("upstream did not print the fixture: %w", err)
	}
	if len(rows) != len(shapes) {
		return fmt.Errorf("upstream returned %d rows for %d shapes", len(rows), len(shapes))
	}

	// The gate: every row that is already committed has to come back
	// unchanged.
	produced := make(map[string]string, len(rows))
	for _, r := range rows {
		produced[r.In] = r.Out
	}
	mismatches := 0
	for _, r := range existing {
		if out := produced[r.In]; out != r.Out {
			mismatches++
			fmt.Fprintf(os.Stderr, "does not reproduce: In %q\n  fixture   %q\n  upstream  %q\n",
				r.In, r.Out, out)
		}
	}
	if mismatches > 0 {
		return fmt.Errorf("%d of %d rows do not reproduce; nothing written —"+
			" a row that does not reproduce is a finding, not a rewrite", mismatches, len(existing))
	}

	fmt.Printf("%d rows reproduce", len(existing))
	if added > 0 {
		fmt.Printf(", %d new shapes", added)
	}
	fmt.Println()

	if !write {
		fmt.Println("not written (pass -write)")
		return nil
	}
	if err := os.WriteFile(outPath, generated, 0o644); err != nil { //nolint:gosec // committed fixture
		return err
	}
	fmt.Printf("wrote %s (%d rows, %d bytes)\n", outPath, len(rows), len(generated))
	return nil
}

func readFixture() ([]row, error) {
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("reading the fixture: %w", err)
	}
	var rows []row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parsing the fixture: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s holds no rows", outPath)
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

// convert runs the vendored python-markdown over the shapes and returns the
// fixture bytes it printed.
func convert(shapes []string) ([]byte, error) {
	input, err := json.Marshal(shapes)
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
