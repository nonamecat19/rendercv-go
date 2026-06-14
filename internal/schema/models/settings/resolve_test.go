package settings_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var reference = time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)

func resolve(t *testing.T, document string) settings.Resolved {
	t.Helper()
	if document == "" {
		return settings.Resolve(nil, reference)
	}
	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}
	return settings.Resolve(node, reference)
}

func TestResolveDefaults(t *testing.T) {
	got := resolve(t, "")

	if !got.CurrentDate.Equal(reference) {
		t.Errorf("current date = %v, want the reference", got.CurrentDate)
	}
	if got.PDFTitle != settings.DefaultPDFTitle {
		t.Errorf("pdf title = %q, want %q", got.PDFTitle, settings.DefaultPDFTitle)
	}
	if len(got.BoldKeywords) != 0 {
		t.Errorf("bold keywords = %v, want none", got.BoldKeywords)
	}
}

// The literal `today` resolves to the reference date rather than being kept as
// text, which is what `_resolved_current_date` does.
func TestTodayResolvesToTheReference(t *testing.T) {
	got := resolve(t, "current_date: today\n")
	if !got.CurrentDate.Equal(reference) {
		t.Errorf("current date = %v, want the reference", got.CurrentDate)
	}
}

func TestADeclaredDateWins(t *testing.T) {
	got := resolve(t, "current_date: 2020-07-04\n")
	want := time.Date(2020, 7, 4, 0, 0, 0, 0, time.UTC)
	if !got.CurrentDate.Equal(want) {
		t.Errorf("current date = %v, want %v", got.CurrentDate, want)
	}
}

// A midnight-exact Unix timestamp resolves the same way `datetime.
// fromtimestamp` reads it upstream (spec 006 delta §5.4, mechanism E2). Only a
// validated document reaches this shape, so the value is always exact here.
func TestEpochCurrentDateResolves(t *testing.T) {
	got := resolve(t, "current_date: 86400\n")
	want := time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	if !got.CurrentDate.Equal(want) {
		t.Errorf("current date = %v, want %v", got.CurrentDate, want)
	}
}

// Duplicates are removed. The order is this port's — see uniqueKeywords for why
// upstream has none to reproduce.
func TestBoldKeywordsAreDeduplicated(t *testing.T) {
	got := resolve(t, "bold_keywords:\n  - Go\n  - Python\n  - Go\n")
	want := []string{"Go", "Python"}
	if !reflect.DeepEqual(got.BoldKeywords, want) {
		t.Errorf("bold keywords = %v, want %v", got.BoldKeywords, want)
	}
}

// A block that writes a key with no value keeps the declared default, rather
// than resolving to the empty string.
func TestNullValuedKeysKeepTheDefault(t *testing.T) {
	got := resolve(t, "pdf_title:\nbold_keywords:\n")
	if got.PDFTitle != settings.DefaultPDFTitle {
		t.Errorf("pdf title = %q, want the default", got.PDFTitle)
	}
}
