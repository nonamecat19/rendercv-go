// Tests of the missing-input guard. These run under a plain `go test ./...`,
// for the same reason `binaryname_test.go` does: a guard that decides whether
// other assertions run is only trustworthy if its own branches are pinned.
package conformance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// recorder stands in for `*testing.T` so a fatal can be observed instead of
// ending the test.
//
// **`Fatalf` and `Skipf` unwind.** On the real `*testing.T` both end the
// goroutine, so nothing after them runs; a recorder that merely stored the
// message and returned would let the guard walk on and overwrite it, and the
// first draft of this file did exactly that — the unparseable-opt-out case
// recorded the *absent file* message and looked like a defect in the guard
// rather than in its double.
type recorder struct {
	fatal   string
	skipped string
}

// unwound is the panic value that stands in for `runtime.Goexit`.
type unwound struct{}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(format string, args ...any) {
	r.fatal = fmt.Sprintf(format, args...)
	panic(unwound{})
}

func (r *recorder) Skipf(format string, args ...any) {
	r.skipped = fmt.Sprintf(format, args...)
	panic(unwound{})
}

// capture runs a guard call that is expected to end the test, and reports what
// the double recorded.
func capture(t *testing.T, path string) (fake recorder) {
	t.Helper()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Error("the guard returned; an absent input must end the test")
			return
		}
		if _, ok := recovered.(unwound); !ok {
			panic(recovered)
		}
	}()

	conformance.RequireInput(&fake, path, "the assertion", "run the probe")
	return fake
}

func TestRequireInputReturnsThePresentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := writeFile(path, "[]"); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	var fake recorder
	got := conformance.RequireInput(&fake, path, "the assertion", "run the probe")

	if string(got) != "[]" {
		t.Errorf("content = %q, want %q", got, "[]")
	}
	if fake.fatal != "" || fake.skipped != "" {
		t.Errorf("a readable file produced fatal=%q skipped=%q", fake.fatal, fake.skipped)
	}
}

// TestRequireInputIsFatalByDefault is the whole point of the guard: the absent
// input must stop the run rather than quietly remove an assertion from it.
func TestRequireInputIsFatalByDefault(t *testing.T) {
	t.Setenv(conformance.AllowMissingInputEnvVar, "")
	path := filepath.Join(t.TempDir(), "absent.json")

	fake := capture(t, path)

	if fake.skipped != "" {
		t.Errorf("an absent input skipped by default: %q", fake.skipped)
	}
	for _, want := range []string{path, "the assertion", "run the probe", conformance.AllowMissingInputEnvVar} {
		if !strings.Contains(fake.fatal, want) {
			t.Errorf("the failure message does not mention %q: %q", want, fake.fatal)
		}
	}
}

// TestRequireInputSkipsWhenTheOptOutIsSet is the escape hatch — one variable,
// set on purpose, and the skip message names it so a reader of the log knows
// why the assertion is gone.
func TestRequireInputSkipsWhenTheOptOutIsSet(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(conformance.AllowMissingInputEnvVar, value)
			path := filepath.Join(t.TempDir(), "absent.json")

			fake := capture(t, path)

			if fake.fatal != "" {
				t.Errorf("the opt-out did not prevent the failure: %q", fake.fatal)
			}
			if !strings.Contains(fake.skipped, conformance.AllowMissingInputEnvVar) {
				t.Errorf("the skip message does not name the variable: %q", fake.skipped)
			}
		})
	}
}

// TestRequireInputRejectsAnUnparseableOptOut is the detail `internal/clidiff`
// established and this inherits: a typo is fatal, because reading it as false
// would run a gate the developer thought they had disabled and reading it as
// true would disable a gate nobody disabled.
func TestRequireInputRejectsAnUnparseableOptOut(t *testing.T) {
	for _, value := range []string{"yes", "0.5", "tru"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(conformance.AllowMissingInputEnvVar, value)
			path := filepath.Join(t.TempDir(), "absent.json")

			fake := capture(t, path)

			if fake.skipped != "" {
				t.Errorf("a garbage opt-out skipped the assertion: %q", fake.skipped)
			}
			if !strings.Contains(fake.fatal, "is not a boolean") {
				t.Errorf("the failure does not name the parse problem: %q", fake.fatal)
			}
		})
	}
}

// TestOptOutIsIgnoredWhenTheInputIsPresent keeps the variable from becoming a
// mute button of its own: it may only excuse an absent input, never suppress an
// assertion that could have run.
func TestOptOutIsIgnoredWhenTheInputIsPresent(t *testing.T) {
	t.Setenv(conformance.AllowMissingInputEnvVar, "1")
	path := filepath.Join(t.TempDir(), "input.json")
	if err := writeFile(path, "[]"); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	var fake recorder
	if got := conformance.RequireInput(&fake, path, "the assertion", "run the probe"); string(got) != "[]" {
		t.Errorf("content = %q, want %q", got, "[]")
	}
	if fake.skipped != "" {
		t.Errorf("a readable file skipped because the opt-out was set: %q", fake.skipped)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
