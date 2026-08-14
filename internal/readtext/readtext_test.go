package readtext

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUniversal is the translation `pathlib.Path.read_text` applies by default
// (`_pyio.py:1925-1929`), row by row.
//
// Every document here is one of the shapes measured against the vendored Python
// for spec delta 002-P §6; the differential in `internal/clidiff` runs the same
// set end to end. This table pins the transform itself, so a row that stops
// translating fails here with the two strings rather than as a box-drawing diff
// four layers up.
func TestUniversal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no carriage return is returned unchanged",
			in:   "cv:\n  name: John Doe\n",
			want: "cv:\n  name: John Doe\n",
		},
		{
			name: "the empty document",
			in:   "",
			want: "",
		},
		{
			// The defect's own probe. The translated document has one more
			// line than the raw one, which is the whole reason upstream's
			// reported span was `line 3 to line 4` and the port's `line 1 to
			// line 3`.
			name: "a lone CR mid-value becomes a line break",
			in:   "cv:\n  name: \rA\n",
			want: "cv:\n  name: \nA\n",
		},
		{
			name: "CRLF throughout",
			in:   "cv:\r\n  name: John Doe\r\n",
			want: "cv:\n  name: John Doe\n",
		},
		{
			name: "a document whose only newline is CR",
			in:   "cv:\r  name: John Doe\r",
			want: "cv:\n  name: John Doe\n",
		},
		{
			name: "mixed CR, LF and CRLF",
			in:   "cv:\r\n  name: John Doe\n  sections:\r    a:\r\n    - text\n",
			want: "cv:\n  name: John Doe\n  sections:\n    a:\n    - text\n",
		},
		{
			name: "CR at end of file",
			in:   "cv:\n  name: John Doe\n\r",
			want: "cv:\n  name: John Doe\n\n",
		},
		{
			name: "CRLF at end of file",
			in:   "cv:\n  name: John Doe\n\r\n",
			want: "cv:\n  name: John Doe\n\n",
		},
		{
			// The order of the two replacements, and the only row that can
			// tell them apart: CRLF first leaves one break, LF first would
			// leave two.
			name: "CRLF is one break, not two",
			in:   "a\r\nb",
			want: "a\nb",
		},
		{
			name: "CR CRLF is two breaks",
			in:   "a\r\r\nb",
			want: "a\n\nb",
		},
		{
			name: "LF CR is two breaks",
			in:   "a\n\rb",
			want: "a\n\nb",
		},
		{
			// Context-free: the decoder sits below the YAML scanner, so a
			// quoted scalar gets the same treatment as the indentation does.
			// Upstream renders this document as the name `A B` — the CR became
			// a line break inside a multi-line double-quoted scalar and folded
			// to a space.
			name: "inside a double-quoted scalar",
			in:   "cv:\n  name: \"A\rB\"\n",
			want: "cv:\n  name: \"A\nB\"\n",
		},
		{
			name: "inside a single-quoted scalar",
			in:   "cv:\n  name: 'A\rB'\n",
			want: "cv:\n  name: 'A\nB'\n",
		},
		{
			name: "inside a literal block scalar",
			in:   "cv:\n  summary: |\n    first\rsecond\n",
			want: "cv:\n  summary: |\n    first\nsecond\n",
		},
		{
			// A `\r` *written* as an escape is two ordinary characters here:
			// the scanner turns it into a carriage return long after the read
			// boundary, and upstream carries it into the artifacts.
			name: "an escaped CR in the source is not a CR yet",
			in:   "cv:\n  name: \"first\\rsecond\"\n",
			want: "cv:\n  name: \"first\\rsecond\"\n",
		},
		{
			// Python's universal-newline mode translates exactly three
			// sequences. NEL and LINE SEPARATOR are line breaks to YAML 1.1
			// and to Unicode, and are not touched here.
			name: "NEL and LINE SEPARATOR are untouched",
			in:   "a\u0085b\u2028c",
			want: "a\u0085b\u2028c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Universal(tc.in); got != tc.want {
				t.Errorf("Universal(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestUniversalIsIdempotent holds the property the four call sites rely on: a
// document already translated once passes through unchanged, so a second read
// of the same file cannot drift from the first.
func TestUniversalIsIdempotent(t *testing.T) {
	for _, in := range []string{
		"cv:\r\n  name: \rA\r",
		"a\r\r\nb\n\r",
		"plain\n",
	} {
		once := Universal(in)
		if twice := Universal(once); twice != once {
			t.Errorf("Universal is not idempotent on %q: %q then %q", in, once, twice)
		}
	}
}

// TestFile asserts the translation happens on the way out of the filesystem,
// which is where upstream's does.
func TestFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CV.yaml")
	if err := os.WriteFile(path, []byte("cv:\r\n  name: \rA\r"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	got, err := File(path)
	if err != nil {
		t.Fatalf("File(%s): %v", path, err)
	}
	if want := "cv:\n  name: \nA\n"; string(got) != want {
		t.Errorf("File\n got %q\nwant %q", got, want)
	}
}

// TestFileReportsAMissingFile keeps the error path intact: the three CLI call
// sites turn it into upstream's `The file ... does not exist!`.
func TestFileReportsAMissingFile(t *testing.T) {
	if _, err := File(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("File on a missing path returned no error")
	}
}
