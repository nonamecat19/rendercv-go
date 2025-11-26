package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// TestOSErrorMessage is spec 013 §4.7 behavior 35: `run_rendercv` catches
// `OSError` and prints `f"OS Error: {e}"` (`run_rendercv.py:195-196`), and the
// message upstream embeds carries an **absolute** path — Python's
// `[Errno 13] Permission denied: '/abs/out/John_Doe_CV.typ'`.
//
// The port emitted neither the prefix nor an absolute path: measured
// `open out/John_Doe_CV.typ: permission denied` against upstream's 637 bytes.
//
// The errno body itself stays Go's (`<op> <path>: <strerror>`), which is §10's
// P-3 and is not this test's business; the prefix and the absolute path are.
func TestOSErrorMessage(t *testing.T) {
	absolute, err := filepath.Abs(filepath.Join("out", "John_Doe_CV.typ"))
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "John_Doe_CV.typ")

	for _, row := range []struct {
		name string
		err  error
		want string
		ok   bool
	}{
		{
			name: "relative path is made absolute",
			err:  &fs.PathError{Op: "open", Path: filepath.Join("out", "John_Doe_CV.typ"), Err: syscall.EACCES},
			want: "OS Error: open " + absolute + ": permission denied",
			ok:   true,
		},
		{
			name: "an absolute path is left alone",
			err:  &fs.PathError{Op: "open", Path: elsewhere, Err: syscall.EACCES},
			want: "OS Error: open " + elsewhere + ": permission denied",
			ok:   true,
		},
		{
			name: "mkdir failures are OS errors too",
			err:  &fs.PathError{Op: "mkdir", Path: elsewhere, Err: syscall.EACCES},
			want: "OS Error: mkdir " + elsewhere + ": permission denied",
			ok:   true,
		},
		{
			// Upstream prints `str(e)` of the OSError, not of whatever
			// re-raised it, so the wrapper's own text is dropped.
			name: "a wrapped path error still reports",
			err:  fmt.Errorf("generate typst: %w", &fs.PathError{Op: "open", Path: elsewhere, Err: syscall.EACCES}),
			want: "OS Error: open " + elsewhere + ": permission denied",
			ok:   true,
		},
		{
			name: "a bare errno has no path to absolutize",
			err:  syscall.ENOSPC,
			want: "OS Error: no space left on device",
			ok:   true,
		},
		{
			// `errMissingFile` is the port's own wording, not an OSError, and
			// prefixing it would corrupt a message the goldens already pin.
			name: "the missing-file message is not an OS error",
			err:  errMissingFile("cv.yaml"),
			ok:   false,
		},
		{
			name: "an ordinary error is not an OS error",
			err:  errors.New("there is a problem with the theme"),
			ok:   false,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, ok := osErrorMessage(row.err)
			if ok != row.ok {
				t.Fatalf("osErrorMessage ok = %v, want %v (message %q)", ok, row.ok, got)
			}
			if ok && got != row.want {
				t.Errorf("osErrorMessage = %q, want %q", got, row.want)
			}
		})
	}
}

// TestFailPanelPrefixesOSErrors pins the routing: the prefix belongs to the
// panel `print_user_error` draws for path B, and nothing else about failPanel
// changes — a validation failure still gets its table.
func TestFailPanelPrefixesOSErrors(t *testing.T) {
	t.Setenv("COLUMNS", "200") // wide enough that nothing wraps

	absolute := filepath.Join(t.TempDir(), "John_Doe_CV.typ")

	var panel bytes.Buffer
	failPanel(&panel, &fs.PathError{Op: "open", Path: absolute, Err: syscall.EACCES})
	if got := panel.String(); !strings.Contains(got, "OS Error: open "+absolute+": permission denied") {
		t.Errorf("panel = %q, want an OS Error line naming %q", got, absolute)
	}

	var table bytes.Buffer
	failPanel(&table, &schemaerr.UserValidationError{Errors: []schemaerr.ValidationError{{
		SchemaLocation: []string{"cv", "name"},
		Message:        "Input should be a valid string",
	}}})
	got := table.String()
	if !strings.Contains(got, "There are validation errors!") {
		t.Errorf("panel = %q, want the validation table", got)
	}
	if strings.Contains(got, "OS Error:") {
		t.Errorf("panel = %q, want no OS Error prefix on a validation failure", got)
	}
}

// TestRenderReportsOSErrorForUnwritableOutput is the measured invocation:
// `render cv.yaml -o <read-only dir>` — upstream 637 bytes ending `af`, exit 1,
// with `OS Error: [Errno 13] Permission denied: '<absolute>'`. The port wrote
// 552 bytes with no prefix and a relative path.
func TestRenderReportsOSErrorForUnwritableOutput(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the write permission this test removes")
	}
	t.Setenv("COLUMNS", "200")

	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o755) })

	var stdout, stderr bytes.Buffer
	code := Render(RenderOptions{
		InputPath:    input,
		OutputFolder: out,
		NoPDF:        true,
		NoPNG:        true,
	}, &stdout, &stderr)

	if code != exitValidationError {
		t.Errorf("exit = %d, want %d", code, exitValidationError)
	}
	got := stdout.String()
	if !strings.Contains(got, "OS Error: ") {
		t.Errorf("stdout = %q, want an OS Error panel", got)
	}
	if !strings.Contains(got, filepath.Join(out, "John_Doe_CV.typ")) {
		t.Errorf("stdout = %q, want the absolute path %q",
			got, filepath.Join(out, "John_Doe_CV.typ"))
	}
}
