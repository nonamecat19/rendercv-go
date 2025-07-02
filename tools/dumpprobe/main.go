// Command dumpprobe regenerates the entry-dump fixture at
// internal/schema/models/cv/entries/testdata/dumps.json by validating a set of
// entry mappings with the vendored Python RenderCV and recording what
// `model_dump(exclude_none=True)` returns for each.
//
// That dump is what `render_entry_templates` turns into placeholders
// (entry_templates_from_input.py:128-130), so it is the boundary iteration 9's
// bridge has to reproduce — and three of its behaviors are invisible in a
// node's text: an integer year stays an integer, a `pydantic.HttpUrl`
// normalizes, and an absent field is omitted while an empty string is kept.
//
// **What this tool cannot check.** The case list is hand-chosen, so a shape no
// case covers is unmeasured. It is not derived from the corpus; the corpus `.typ`
// goldens are the gate that covers what this misses.
//
// The fixture is GENERATED and is NEVER hand-written or hand-edited
// (AGENTS.md §10.1); this program is its only author. It is not part of
// testdata/golden, so regenerating it does not change the parity contract and
// needs no human gate.
//
// Usage:
//
//	go run ./tools/dumpprobe    # or: just dumpprobe
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	upstreamDir = "third_party/rendercv"
	scriptPath  = "tools/dumpprobe/probe.py"
	outPath     = "internal/schema/models/cv/entries/testdata/dumps.json"
)

// probe is the shape of probe.py's output, used only to sanity-check it before
// the bytes are written. The raw stdout is what lands on disk.
type probe struct {
	Dumps []struct {
		Type  string         `json:"type"`
		Case  string         `json:"case"`
		Input map[string]any `json:"input"`
		Dump  map[string]any `json:"dump"`
	} `json:"dumps"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dumpprobe:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	// Same invocation as `just upstream`: uv, frozen lockfile, all extras.
	cmd := exec.Command(
		"uv", "run", "--frozen", "--all-extras",
		"python", filepath.Join(root, scriptPath),
	)
	cmd.Dir = filepath.Join(root, upstreamDir)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run probe.py: %w", err)
	}

	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return fmt.Errorf("parse probe output: %w\n%s", err, out)
	}
	if err := check(p); err != nil {
		return err
	}

	target := filepath.Join(root, outPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create testdata dir: %w", err)
	}
	if err := os.WriteFile(target, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Println("wrote", outPath)
	return nil
}

// check rejects an output upstream could not have produced, so a broken import
// or an empty run fails here rather than silently overwriting the fixture. It is
// not a parity assertion: what the dumps contain is pinned by the conformance
// test that reads the fixture.
func check(p probe) error {
	if len(p.Dumps) == 0 {
		return fmt.Errorf("no dumps")
	}

	types := map[string]struct{}{}
	for _, dump := range p.Dumps {
		if len(dump.Dump) == 0 {
			return fmt.Errorf("%s/%s dumped nothing", dump.Type, dump.Case)
		}
		types[dump.Type] = struct{}{}
	}
	const models = 8
	if len(types) != models {
		return fmt.Errorf("got cases for %d types, want %d", len(types), models)
	}
	return nil
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
