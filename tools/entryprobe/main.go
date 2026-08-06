// Command entryprobe regenerates the entry-model field-order fixture at
// internal/schema/models/cv/entries/testdata/field_orders.json by introspecting the
// vendored Python RenderCV (third_party/rendercv).
//
// The fixture pins three ordered facts that iteration 3's registry must reproduce
// (specs/003-entry-types/spec.md §3.2, §3.16, §6.1):
//
//   - each entry model's `model_fields` keys, in pydantic's declaration order;
//   - the characteristic-field table of section.py:77, one sorted set per type;
//   - `available_entry_type_names` (section.py:37-39), the eight model names in
//     `EntryModel` union order plus the literal "TextEntry" last.
//
// The fixture is GENERATED and is NEVER hand-written or hand-edited (AGENTS.md §10.1);
// this program is its only author. It is not part of testdata/golden, so regenerating
// it does not change the parity contract and needs no human gate.
//
// Usage:
//
//	go run ./tools/entryprobe    # or: just entryprobe
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
	scriptPath  = "tools/entryprobe/probe.py"
	outPath     = "internal/schema/models/cv/entries/testdata/field_orders.json"
)

// probe is the shape of probe.py's output, used only to sanity-check it before the
// bytes are written. Field order inside the file comes from Python, not from Go, so
// the raw stdout is what lands on disk.
type probe struct {
	TypeNames   []string `json:"available_entry_type_names"`
	FieldOrders []struct {
		Type   string   `json:"type"`
		Fields []string `json:"fields"`
	} `json:"field_orders"`
	Characteristic []struct {
		Type   string   `json:"type"`
		Fields []string `json:"fields"`
	} `json:"characteristic_entry_fields"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "entryprobe:", err)
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

// check rejects an output that upstream could not have produced, so a broken import
// or an empty run fails here rather than silently overwriting the fixture. It is not
// a parity assertion: the field orders themselves are pinned by the conformance test
// that reads the fixture, not by this program.
func check(p probe) error {
	const models = 8
	if len(p.TypeNames) != models+1 {
		return fmt.Errorf("got %d entry type names, want %d", len(p.TypeNames), models+1)
	}
	if last := p.TypeNames[models]; last != "TextEntry" {
		return fmt.Errorf("last entry type name is %q, want %q", last, "TextEntry")
	}
	if len(p.FieldOrders) != models {
		return fmt.Errorf("got %d field orders, want %d", len(p.FieldOrders), models)
	}
	if len(p.Characteristic) != models {
		return fmt.Errorf("got %d characteristic sets, want %d", len(p.Characteristic), models)
	}
	for i, fo := range p.FieldOrders {
		if fo.Type != p.TypeNames[i] {
			return fmt.Errorf("field order %d is %q, want %q (union order)", i, fo.Type, p.TypeNames[i])
		}
		if len(fo.Fields) == 0 {
			return fmt.Errorf("%s has no fields", fo.Type)
		}
		if p.Characteristic[i].Type != fo.Type {
			return fmt.Errorf(
				"characteristic set %d is %q, want %q (union order)",
				i, p.Characteristic[i].Type, fo.Type,
			)
		}
	}
	return nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
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
