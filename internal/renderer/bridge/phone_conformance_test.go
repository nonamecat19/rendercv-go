//go:build conformance

package bridge_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
)

// phones is testdata/phones.json, generated from the vendored Python's own
// libphonenumber by tools/phoneprobe (tasks 009 T4).
type phones struct {
	Numbers []struct {
		Input     string            `json:"input"`
		Valid     bool              `json:"valid"`
		Stored    string            `json:"stored"`
		Formatted map[string]string `json:"formatted"`
	} `json:"numbers"`
}

func loadPhones(t *testing.T) phones {
	t.Helper()
	path := filepath.Join("testdata", "phones.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — regenerate it with `just phoneprobe`", path, err)
	}
	var out phones
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(out.Numbers) == 0 {
		t.Fatalf("%s carries no numbers; the fixture is empty", path)
	}
	return out
}

// Spec 009 plan §3. Two ports of Google's libphonenumber metadata have to agree
// on both halves: what `PhoneNumber` stores, and what `format_number` makes of
// it. A disagreement here is the divergence the plan says to record — a phone
// number in a header is as user-visible as text gets.
func TestPhoneFormattingMatchesUpstream(t *testing.T) {
	fixture := loadPhones(t)

	for _, number := range fixture.Numbers {
		t.Run(number.Input, func(t *testing.T) {
			stored, err := phonenum.Validate(number.Input)
			if !number.Valid {
				if err == nil {
					t.Fatalf("Validate(%q) accepted a number upstream rejects", number.Input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(%q) = %v, want it accepted", number.Input, err)
			}
			if stored != number.Stored {
				t.Fatalf("stored = %q, want %q", stored, number.Stored)
			}

			for format, want := range number.Formatted {
				got := bridge.FormatPhone(stored, format)
				if got != want {
					t.Errorf("FormatPhone(%q, %q) = %q, want %q", stored, format, got, want)
				}
			}
		})
	}
}
