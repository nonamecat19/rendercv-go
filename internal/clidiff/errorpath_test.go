//go:build conformance

package clidiff

import (
	"os"
	"path/filepath"
	"testing"
)

// validCV is the smallest document that renders.
const validCV = "cv:\n  name: John Doe\n"

// TestErrorHandlerPaths is behavior 31 of specs/013-parity-closeout/spec.md —
// the seven measured invocations that decide which of the two error paths a
// failure takes.
//
// Upstream has two panel printers and picks between them by **where** the
// failure was raised, not by its type: anything raised before
// `with ProgressPanel(...)` (`cli/render_command/render_command.py:231`)
// escapes to the `@handle_user_errors` decorator, whose `rich.print` ends the
// output with a newline (path A, last byte `0a`); anything raised inside
// `run_rendercv` reaches the `rich.live.Live`, whose final render ends with the
// panel's closing `╯` and nothing after it (path B, last byte `af`).
//
// The wantBytes column is the spec's own measurement. It is asserted only for
// the rows whose output contains no absolute path — the others are
// machine-dependent, and for those the differential against the vendored Python
// is the whole of the contract.
func TestErrorHandlerPaths(t *testing.T) {
	cases := []struct {
		name string
		inv  invocation
		// wantBytes is spec 013 §3.4's measured stdout size, 0 when the row's
		// output embeds a path and the number is not portable.
		wantBytes int
		// wantLastByte is the row's last-byte column.
		wantLastByte string
		// wantExit is the row's exit code.
		wantExit int
		// path is `A` or `B`, for the failure message only.
		path string
		// rawEqual asks for full byte equality of stdout, which is only
		// meaningful when the output carries neither a duration nor a path.
		rawEqual bool
		// extra is an assertion this row needs on top of compare's, for a row
		// whose contract is narrower than full byte equality. It never
		// replaces an assertion, only adds one.
		extra func(t *testing.T, upstream, port outcome)
	}{
		{
			name: "empty input file",
			inv: invocation{
				args:  []string{"render", "empty.yaml"},
				files: map[string]string{"empty.yaml": ""},
			},
			wantBytes:    553,
			wantLastByte: "0a",
			wantExit:     1,
			path:         "A",
			rawEqual:     true,
		},
		{
			// The odd override argument and the empty file are both path A;
			// the row exists because the two raise sites are different
			// (`render_command.py:228` versus `run_rendercv.py:114`) and the
			// measured size says which one won.
			name: "odd override over an empty file",
			inv: invocation{
				args:  []string{"render", "empty.yaml", "--cv.name=Jane"},
				files: map[string]string{"empty.yaml": ""},
			},
			wantBytes:    553,
			wantLastByte: "0a",
			wantExit:     1,
			path:         "A",
			rawEqual:     true,
		},
		{
			// `new` builds no ProgressPanel at all (behavior 33), so every one
			// of its errors is path A unconditionally.
			name:         "new with an unknown theme",
			inv:          invocation{args: []string{"new", "John Doe", "--theme", "nope"}},
			wantBytes:    638,
			wantLastByte: "0a",
			wantExit:     1,
			path:         "A",
			rawEqual:     true,
		},
		{
			name:         "new with an unknown locale",
			inv:          invocation{args: []string{"new", "John Doe", "--locale", "nope"}},
			wantBytes:    894,
			wantLastByte: "0a",
			wantExit:     1,
			path:         "A",
			rawEqual:     true,
		},
		{
			// A YAML *syntax* error is not a RenderCVUserValidationError, so
			// `collect_input_file_paths`' contextlib.suppress
			// (`run_rendercv.py:113`) does not swallow it and it surfaces
			// inside the Live as a validation table — path B, one byte shorter
			// at the end than its path A neighbours.
			name: "malformed YAML",
			inv: invocation{
				args:  []string{"render", "bad.yaml"},
				files: map[string]string{"bad.yaml": "cv: [\n"},
			},
			wantBytes:    1411,
			wantLastByte: "af",
			wantExit:     1,
			path:         "B",
			rawEqual:     true,
		},
		{
			// OSError is `run_rendercv`'s third clause, so a write that cannot
			// happen is path B too.
			name: "output folder is read-only",
			inv: invocation{
				args:  []string{"render", "cv.yaml", "-o", "out"},
				files: map[string]string{"cv.yaml": validCV},
				prepare: func(t *testing.T, dir string) {
					t.Helper()

					out := filepath.Join(dir, "out")
					if err := os.MkdirAll(out, 0o755); err != nil {
						t.Fatalf("creating the output folder: %v", err)
					}
					if err := os.Chmod(out, 0o555); err != nil {
						t.Fatalf("making the output folder read-only: %v", err)
					}
					// t.TempDir's cleanup cannot remove what it cannot write.
					t.Cleanup(func() { _ = os.Chmod(out, 0o755) })
				},
			},
			// 722 on both sides since `87cd21f` gave the port the `OS Error: `
			// prefix and the absolute path. The equality is a coincidence of
			// this errno's phrasing — upstream's body is 8 characters longer
			// than the port's before wrapping — so it is asserted as a
			// differential and not as a portable constant.
			wantLastByte: "af",
			wantExit:     1,
			path:         "B",
			// §10 P-3 covers the errno wording between the prefix and the
			// path, and osErrorRow narrows the downgrade to exactly that.
			extra: osErrorRow,
		},
		{
			// The success panel is path B as well: the Live's last render is
			// the completed panel, still without a trailing newline.
			name: "successful render",
			inv: invocation{
				args:  []string{"render", "cv.yaml"},
				files: map[string]string{"cv.yaml": validCV},
			},
			wantLastByte: "af",
			wantExit:     0,
			path:         "B",
		},
	}

	if os.Geteuid() == 0 {
		t.Log("running as root: the read-only output folder row cannot fail the way it must")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream, port := differential(t, tc.inv)

			t.Logf("path %s\nupstream: %s\nport:     %s", tc.path, upstream, port)

			compare(t, upstream, port)
			if tc.rawEqual {
				compareRaw(t, "stdout", upstream.stdout, port.stdout)
				compareRaw(t, "stderr", upstream.stderr, port.stderr)
			}
			if tc.extra != nil {
				tc.extra(t, upstream, port)
			}

			// The spec's own numbers, checked against the vendored Python
			// rather than against the port: a mismatch here means the
			// measurement moved, not that the port is wrong.
			if tc.wantBytes != 0 && len(upstream.stdout) != tc.wantBytes {
				t.Errorf("upstream stdout is %d bytes, spec §3.4 measured %d",
					len(upstream.stdout), tc.wantBytes)
			}
			if got := lastByte(upstream.stdout); got != tc.wantLastByte {
				t.Errorf("upstream stdout last byte is %s, spec §3.4 measured %s", got, tc.wantLastByte)
			}
			if upstream.exit != tc.wantExit {
				t.Errorf("upstream exit is %d, spec §3.4 measured %d", upstream.exit, tc.wantExit)
			}
		})
	}
}
