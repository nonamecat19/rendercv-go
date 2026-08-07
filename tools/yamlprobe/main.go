package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

func main() {
	root := repoRoot()
	script := filepath.Join(root, "tools/yamlprobe/probe.py")
	fixturesDir := filepath.Join(root, "internal/schema/yamlreader/testdata/coords")
	outFile := filepath.Join(fixturesDir, "coords.json")

	fixtures := []string{
		"nested_mapping.yaml",
		"sequence_of_mappings.yaml",
		"sequence_of_scalars.yaml",
		"empty_mapping_value.yaml",
		"flow_mapping.yaml",
		"indented_empty_value.yaml",
		"flow_sequence.yaml",
	}

	type coordEntry struct {
		File string         `json:"file"`
		Data map[string]any `json:"data"`
	}

	var entries []coordEntry
	for _, f := range fixtures {
		path := filepath.Join(fixturesDir, f)
		cmd := exec.Command(
			filepath.Join(root, "third_party/rendercv/.venv/bin/python"),
			script,
			path,
		)
		cmd.Dir = filepath.Join(root, "third_party/rendercv")
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe %s: %v\n", f, err)
			os.Exit(1)
		}
		var data map[string]any
		if err := json.Unmarshal(out, &data); err != nil {
			fmt.Fprintf(os.Stderr, "probe %s: parse output: %v\n%s\n", f, err, string(out))
			os.Exit(1)
		}
		entries = append(entries, coordEntry{File: f, Data: data})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })

	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outFile, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote", outFile)
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("no go.mod found")
		}
		dir = parent
	}
}
