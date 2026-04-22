package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNonReproducibleStreamsAreTheTwoD011Cases pins the exemption list. It may
// only shrink, and only with the divergence: every entry is a golden `-verify`
// stops byte-comparing, so an accidental addition would silently retire a real
// check.
func TestNonReproducibleStreamsAreTheTwoD011Cases(t *testing.T) {
	want := []string{
		filepath.Join("err_bad_override_key", "stderr.txt"),
		filepath.Join("err_missing_file", "stderr.txt"),
	}
	got := map[string]bool{}
	for _, s := range nonReproducibleStreams {
		if s.Divergence != "D-011" {
			t.Errorf("%s is exempt under %s; only D-011 exempts a stream", s.Path, s.Divergence)
		}
		if len(s.Must) == 0 {
			t.Errorf("%s is exempt with no replacement check", s.Path)
		}
		got[s.Path] = true
	}
	if len(got) != len(want) {
		t.Fatalf("exemption list has %d entries, want %d", len(got), len(want))
	}
	for _, path := range want {
		if !got[path] {
			t.Errorf("%s is no longer exempt", path)
		}
	}
}

// TestCheckStreamContent covers the check that replaces the byte comparison:
// it must accept a traceback rendered anywhere, and still reject an empty or
// unrecognizable one.
func TestCheckStreamContent(t *testing.T) {
	stream := nonReproducibleStream{
		Path:       filepath.Join("err_missing_file", "stderr.txt"),
		Divergence: "D-011",
		Must: []string{
			"Traceback (most recent call last)",
			"FileNotFoundError: [Errno 2] No such file or directory:",
		},
	}
	// The same traceback as another machine renders it: a different checkout
	// path and a different CPython prefix, which is exactly what D-011 exempts.
	foreign := "╭──── Traceback (most recent call last) ────╮\n" +
		"│ /home/runner/work/rendercv-go/rendercv-go/third_party/rendercv/src │\n" +
		"│ /home/runner/.local/share/uv/python/cpython-3.12.9-linux-x86_64-gnu │\n" +
		"╰────────────────────────────────────────────────────────────────────╯\n" +
		"FileNotFoundError: [Errno 2] No such file or directory:\n'does_not_exist.yaml'\n"

	tests := []struct {
		name     string
		content  string
		problems int
		mentions string
	}{
		{name: "a traceback from another machine", content: foreign, problems: 0},
		{name: "empty", content: "", problems: 1, mentions: "is empty"},
		{name: "whitespace only", content: "\n\n  \n", problems: 1, mentions: "is empty"},
		{
			name:     "a clean panel instead of a traceback",
			content:  "╭─ Error ─╮\n│ Oops.   │\n╰─────────╯\n",
			problems: 2,
			mentions: "no longer contains",
		},
		{
			name:     "the traceback of a different exception",
			content:  strings.Replace(foreign, "FileNotFoundError", "PermissionError", 1),
			problems: 1,
			mentions: "FileNotFoundError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := checkStreamContent(stream, "the committed golden", tt.content)
			if len(problems) != tt.problems {
				t.Fatalf("got %d problem(s) %q, want %d", len(problems), problems, tt.problems)
			}
			if tt.mentions != "" && !strings.Contains(strings.Join(problems, "\n"), tt.mentions) {
				t.Errorf("problems %q do not mention %q", problems, tt.mentions)
			}
		})
	}
}

// TestCheckNonReproducibleReadsBothCopies pins that the check runs against the
// committed golden *and* the freshly generated one: a stream that vanished from
// either side is a failure, not an exemption.
func TestCheckNonReproducibleReadsBothCopies(t *testing.T) {
	stream := nonReproducibleStreams[0]
	good := "╭─ Traceback (most recent call last) ─╮\n╰─────────────────────────────────────╯\n" +
		strings.Join(stream.Must[1:], "\n") + "\n"

	committed, scratch := t.TempDir(), t.TempDir()
	write := func(dir, content string) {
		t.Helper()
		path := filepath.Join(dir, stream.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(committed, good)
	write(scratch, good)
	if problems := checkNonReproducible(committed, scratch, stream.Path); len(problems) != 0 {
		t.Fatalf("two good copies reported %q", problems)
	}

	if err := os.Remove(filepath.Join(scratch, stream.Path)); err != nil {
		t.Fatal(err)
	}
	problems := checkNonReproducible(committed, scratch, stream.Path)
	if len(problems) != 1 || !strings.Contains(problems[0], "the generated stream") {
		t.Fatalf("a missing generated stream reported %q, want one problem naming it", problems)
	}
}
