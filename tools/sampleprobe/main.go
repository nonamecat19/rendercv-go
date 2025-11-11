// Command sampleprobe regenerates the starter CV's raw blocks and the two
// fixtures that gate the generator, by running tools/sampleprobe/probe.py
// against the vendored Python RenderCV.
//
// **What is data and what is logic.** Upstream's starter CV is a pydantic dump
// of `RenderCVModel` (`schema/sample_generator.py:217-221`) put through four
// string transforms (`:151-195`). The transforms are logic and
// internal/cli/sample implements them; the dump is data and this captures it.
// The capture is decomposed along the two axes spec 013 §3.1 behavior 14
// measures — the `cv` and `settings` blocks carry neither, the `design` block is
// a function of the theme alone and the `locale` block of the locale alone — so
// 33 blocks stand in for 198 documents. probe.py refuses to emit anything unless
// that decomposition rejoins the whole dump byte for byte, for every pair.
//
// **What this tool cannot check.** It captures upstream's *output*; it does not
// derive it. A field pydantic would add, drop or reorder in a future release
// shows up here only after a submodule bump and a rerun. The two fixtures it
// writes alongside — the 198 document digests and the `cv.name` battery — are
// what turn that data into a checkable claim.
//
// GENERATED, never hand-edited (AGENTS.md §10.1); this program is their only
// author. None of it is under testdata/golden, so a rerun changes no golden and
// needs no human gate.
//
// Usage:
//
//	go run ./tools/sampleprobe    # or: just sampleprobe
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	upstreamDir = "third_party/rendercv"
	scriptPath  = "tools/sampleprobe/probe.py"
	blocksDir   = "internal/cli/sample/blocks"
	fixtureDir  = "internal/cli/sample/testdata"

	themeCount  = 9
	localeCount = 22
)

// probe is probe.py's output.
type probe struct {
	Version   string            `json:"version"`
	Themes    []string          `json:"themes"`
	Locales   []string          `json:"locales"`
	Cv        string            `json:"cv"`
	Settings  string            `json:"settings"`
	Design    map[string]string `json:"design"`
	Locale    map[string]string `json:"locale"`
	Documents map[string]string `json:"documents"`
	Names     []struct {
		Name   string `json:"name"`
		Region string `json:"region"`
	} `json:"names"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sampleprobe:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	// The submodule's own interpreter, by path. `uv run … rendercv` from
	// outside the submodule resolves to whatever `rendercv` is on PATH.
	cmd := exec.Command(
		filepath.Join(root, upstreamDir, ".venv/bin/python"),
		filepath.Join(root, scriptPath),
	)
	cmd.Dir = filepath.Join(root, upstreamDir)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run probe.py: %w", err)
	}

	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return fmt.Errorf("parse probe output: %w", err)
	}
	if err := check(p); err != nil {
		return err
	}

	blocks := filepath.Join(root, blocksDir)
	if err := os.RemoveAll(blocks); err != nil {
		return err
	}
	files := map[string]string{
		"cv.yaml":       p.Cv,
		"settings.yaml": p.Settings,
	}
	for theme, block := range p.Design {
		files["design/"+theme+".yaml"] = block
	}
	for locale, block := range p.Locale {
		files["locale/"+locale+".yaml"] = block
	}
	for name, content := range files {
		target := filepath.Join(blocks, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d blocks to %s\n", len(files), blocksDir)

	if err := writeFixture(root, "matrix.json", map[string]any{
		"version":   p.Version,
		"themes":    p.Themes,
		"locales":   p.Locales,
		"documents": p.Documents,
	}); err != nil {
		return err
	}
	return writeFixture(root, "names.json", p.Names)
}

// writeFixture writes one test fixture, newline-terminated so the file is a
// well-formed text file rather than a bare JSON blob.
func writeFixture(root, name string, value any) error {
	encoded, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		return err
	}
	target := filepath.Join(root, fixtureDir, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote", filepath.Join(fixtureDir, name))
	return nil
}

// check rejects an output upstream could not have produced, so a broken import
// or a half-finished run fails here rather than overwriting good data with
// nothing. It is not a parity assertion: probe.py asserts the decomposition,
// and internal/cli/sample's tests assert the generator.
func check(p probe) error {
	switch {
	case p.Version == "":
		return fmt.Errorf("no version")
	case len(p.Themes) != themeCount:
		return fmt.Errorf("got %d themes, want %d", len(p.Themes), themeCount)
	case len(p.Locales) != localeCount:
		return fmt.Errorf("got %d locales, want %d", len(p.Locales), localeCount)
	case len(p.Design) != themeCount:
		return fmt.Errorf("got %d design blocks, want %d", len(p.Design), themeCount)
	case len(p.Locale) != localeCount:
		return fmt.Errorf("got %d locale blocks, want %d", len(p.Locale), localeCount)
	case len(p.Documents) != themeCount*localeCount:
		return fmt.Errorf("got %d digests, want %d", len(p.Documents), themeCount*localeCount)
	case len(p.Names) == 0:
		return fmt.Errorf("no name battery")
	}

	// The four blocks are spliced by their first line: the generator splits the
	// dump on `design:\n  theme: …` and `locale:\n  language: …`
	// (`schema/sample_generator.py:168-190`), so a block that does not start
	// with its own discriminator would break the surgery silently.
	if !strings.HasPrefix(p.Cv, "cv:\n  name: John Doe\n") {
		return fmt.Errorf("the cv block does not start with cv:/name:")
	}
	if !strings.HasPrefix(p.Settings, "settings:\n") {
		return fmt.Errorf("the settings block does not start with settings:")
	}
	for theme, block := range p.Design {
		if !strings.HasPrefix(block, "design:\n  theme: "+theme+"\n") {
			return fmt.Errorf("the %s design block does not start with its theme", theme)
		}
	}
	for locale, block := range p.Locale {
		if !strings.HasPrefix(block, "locale:\n  language: "+locale+"\n") {
			return fmt.Errorf("the %s locale block does not start with its language", locale)
		}
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
