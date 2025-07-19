// Command sampleprobe captures the starter CV that `rendercv new` writes, for
// each theme and locale the corpus exercises, into
// internal/cli/samples/<variant>.yaml.
//
// **The sample is data, not logic.** Upstream builds it from its own models, and
// a hand-written copy of 369 lines would be a golden by another name
// (AGENTS.md §10.1). This runs the vendored CLI and stores what it produced.
//
// **What this tool cannot check.** Only the variants listed below. `rendercv
// new` accepts any theme/locale pair, so a combination no golden uses is
// uncaptured — and a name other than "John Doe" is not captured at all, which
// is the open question `internal/cli/new.go` has to answer before the command
// can serve an arbitrary name.
//
// GENERATED, never hand-edited; regenerate with `just sampleprobe`.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

const (
	upstreamDir = "third_party/rendercv"
	outDir      = "internal/cli/samples"
)

// variants are the eight `new_*` corpus cases, keyed by the name the fixture
// takes. The empty argument list is the default sample.
var variants = map[string][]string{
	"default":                  {},
	"locale_arabic":            {"--locale", "arabic"},
	"locale_french":            {"--locale", "french"},
	"locale_japanese":          {"--locale", "japanese"},
	"locale_turkish":           {"--locale", "turkish"},
	"theme_engineeringresumes": {"--theme", "engineeringresumes"},
	"theme_moderncv":           {"--theme", "moderncv"},
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
	target := filepath.Join(root, outDir)
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	names := make([]string, 0, len(variants))
	for name := range variants {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		yaml, err := capture(root, variants[name])
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(target, name+".yaml"), yaml, 0o644); err != nil {
			return err
		}
		fmt.Printf("%-26s %d bytes\n", name, len(yaml))
	}
	return nil
}

func capture(root string, args []string) ([]byte, error) {
	scratch, err := os.MkdirTemp("", "sampleprobe-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	// `uv run` resolves the project from its working directory, so the project
	// is named explicitly and the working directory stays the scratch one —
	// which is where `new` writes.
	argv := []string{
		"run", "--frozen", "--all-extras",
		"--project", filepath.Join(root, upstreamDir),
		"rendercv", "new", "John Doe",
	}
	cmd := exec.Command("uv", append(argv, args...)...)
	cmd.Dir = scratch
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("upstream failed: %s", out)
	}

	matches, err := filepath.Glob(filepath.Join(scratch, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("produced %d yaml files", len(matches))
	}
	return os.ReadFile(matches[0])
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
