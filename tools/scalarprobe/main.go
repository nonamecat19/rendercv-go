// Command scalarprobe regenerates the scalar-resolution fixture from the
// vendored Python, so `resolve_test.go` compares against upstream's own reader
// rather than against expectations written on the Go side.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root := repoRoot()
	outFile := filepath.Join(root, "internal/schema/yamlreader/testdata/scalars/scalars.json")

	cmd := exec.Command(
		filepath.Join(root, "third_party/rendercv/.venv/bin/python"),
		filepath.Join(root, "tools/scalarprobe/probe.py"),
	)
	cmd.Dir = filepath.Join(root, "third_party/rendercv")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
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
