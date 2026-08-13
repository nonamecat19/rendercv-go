// Command phoneprobe regenerates the phone-formatting fixture at
// internal/renderer/bridge/testdata/phones.json from the vendored Python's own
// dependencies.
//
// The header's phone body is `phonenumbers.format_number` over the **stored**
// RFC 3966 string (connections.py:96-110), and the format is a design option
// with three values. Neither the storage regrouping nor the per-region national
// format is derivable by hand — both come from Google's libphonenumber
// metadata — so this fixture is what says the Go port of that data agrees with
// the Python one.
//
// **What this tool cannot check.** The number list is hand-chosen. Two ports of
// the same metadata can still disagree on a number no case covers, and only the
// corpus goldens cover what the corpus actually contains.
//
// The fixture is GENERATED and is NEVER hand-written or hand-edited
// (AGENTS.md §10.1); this program is its only author. It is not part of
// testdata/golden, so regenerating it does not change the parity contract and
// needs no special approval.
//
// Usage:
//
//	go run ./tools/phoneprobe    # or: just phoneprobe
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
	scriptPath  = "tools/phoneprobe/probe.py"
	outPath     = "internal/renderer/bridge/testdata/phones.json"
)

type probe struct {
	Numbers []struct {
		Input     string            `json:"input"`
		Valid     bool              `json:"valid"`
		Stored    string            `json:"stored"`
		Formatted map[string]string `json:"formatted"`
	} `json:"numbers"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "phoneprobe:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

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

func check(p probe) error {
	if len(p.Numbers) == 0 {
		return fmt.Errorf("no numbers")
	}
	valid := 0
	for _, number := range p.Numbers {
		if !number.Valid {
			continue
		}
		valid++
		if len(number.Formatted) != 3 {
			return fmt.Errorf("%s has %d formats, want 3", number.Input, len(number.Formatted))
		}
	}
	if valid == 0 {
		return fmt.Errorf("every number was rejected; the probe is not measuring anything")
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
