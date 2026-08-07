// Command typprobe renders every corpus input with the vendored Python and
// stores the resulting `.typ` under
// internal/renderer/typstdoc/testdata/typ/<case>/, beside the input it came
// from.
//
// **This is the gate iteration 9 ends at** (spec 009 §6): the Go renderer's
// output has to be byte-identical to these. It is a differential rather than a
// read of `testdata/golden`, for one reason — the goldens were generated on a
// particular day and carry that date in the preamble and the top note, while
// this pins `settings.current_date` so both sides are reproducible.
//
// **What this tool cannot check.** Only the cases it renders. A corpus case
// whose input the CLI rejects, or that has no input file at all — the `cli_*`
// and `err_*` cases — is skipped, and nothing here covers the Markdown or HTML
// documents.
//
// The fixture is GENERATED and is NEVER hand-written or hand-edited
// (AGENTS.md §10.1); this program is its only author. It is not part of
// testdata/golden, so regenerating it does not change the parity contract and
// needs no human gate.
//
// Usage:
//
//	go run ./tools/typprobe    # or: just typprobe
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	upstreamDir = "third_party/rendercv"
	corpusDir   = "testdata/.work/run"
	outDir      = "internal/renderer/typstdoc/testdata/typ"

	// CurrentDate is what every case is rendered with, so the preamble's
	// `datetime(...)` and the top note's month are the same on both sides and on
	// every day. It is the reference date the port's own tests already use.
	CurrentDate = "2025-03-05"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "typprobe:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	cases, err := os.ReadDir(filepath.Join(root, corpusDir))
	if err != nil {
		return fmt.Errorf("reading the corpus (run `just golden` first?): %w", err)
	}

	target := filepath.Join(root, outDir)
	if err := os.RemoveAll(target); err != nil {
		return err
	}

	var rendered, skipped []string
	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		input := filepath.Join(root, corpusDir, name, "cv.yaml")
		if _, err := os.Stat(input); err != nil {
			skipped = append(skipped, name+" (no cv.yaml)")
			continue
		}

		typ, err := renderCase(root, input)
		if err != nil {
			skipped = append(skipped, name+" ("+err.Error()+")")
			continue
		}

		dir := filepath.Join(target, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		raw, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "cv.yaml"), raw, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "expected.typ"), typ, 0o644); err != nil {
			return err
		}
		rendered = append(rendered, name)
	}

	sort.Strings(rendered)
	sort.Strings(skipped)
	fmt.Printf("rendered %d cases into %s\n", len(rendered), outDir)
	for _, name := range skipped {
		fmt.Println("  skipped", name)
	}
	if len(rendered) == 0 {
		return fmt.Errorf("no case rendered; the fixture would be empty")
	}
	return nil
}

// renderCase runs the vendored CLI in a scratch directory, so the corpus's own
// working tree is never written to, and returns the one `.typ` it produced.
func renderCase(root, input string) ([]byte, error) {
	scratch, err := os.MkdirTemp("", "typprobe-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	raw, err := os.ReadFile(input)
	if err != nil {
		return nil, err
	}
	copied := filepath.Join(scratch, "cv.yaml")
	if err := os.WriteFile(copied, raw, 0o644); err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"uv", "run", "--frozen", "--all-extras",
		"rendercv", "render", copied,
		"-nopdf", "-nopng", "-nomd", "-nohtml",
		"--settings.current_date", CurrentDate,
	)
	cmd.Dir = filepath.Join(root, upstreamDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("upstream rejected it: %s", firstLine(out))
	}

	matches, err := filepath.Glob(filepath.Join(scratch, "rendercv_output", "*.typ"))
	if err != nil {
		return nil, err
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("produced %d .typ files", len(matches))
	}
	return os.ReadFile(matches[0])
}

func firstLine(out []byte) string {
	text := strings.TrimSpace(string(out))
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
