package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func languageNode(t *testing.T, value string) *yamldoc.Node {
	t.Helper()
	doc, err := yamlreader.ReadString("language: " + value + "\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc.Items[0].Value
}

// Spec 004 §3.17 behavior 67 and §4.30, asserted as the whole literal — the
// enumeration order is pydantic's and nothing weaker would catch a reordering.
func TestUnknownLanguage(t *testing.T) {
	errs := locale.ValidateLanguage(
		languageNode(t, "klingon"), []string{"locale"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	const want = "Input tag 'klingon' found using 'language' does not match any of" +
		" the expected tags: 'english', 'arabic', 'danish', 'dutch', 'french'," +
		" 'german', 'hebrew', 'hindi', 'hungarian', 'indonesian', 'italian'," +
		" 'japanese', 'korean', 'mandarin_chinese', 'norwegian_bokmål'," +
		" 'norwegian_nynorsk', 'persian', 'portuguese', 'russian', 'spanish'," +
		" 'turkish', 'vietnamese'"
	if errs[0].Message != want {
		t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, want)
	}

	// The failure is at the locale block, not at `language`: pydantic raises it
	// while resolving which union member to use.
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "locale" {
		t.Errorf("location = %q, want locale", got)
	}

	// No dictionary row matches, so the pipeline only appends a period.
	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if final[0].Message != want+"." {
		t.Errorf("final message = %q, want the raw text plus a period", final[0].Message)
	}
}

// All twenty-two are accepted, including the non-ASCII one.
func TestEveryLanguageAccepted(t *testing.T) {
	if len(locale.Languages) != 22 {
		t.Fatalf("there are %d languages, want 22", len(locale.Languages))
	}
	if locale.Languages[0] != "english" {
		t.Errorf("the first language is %q, want english — it is not sorted first",
			locale.Languages[0])
	}

	for _, language := range locale.Languages {
		errs := locale.ValidateLanguage(
			languageNode(t, `"`+language+`"`), []string{"locale"}, schemaerr.SourceMain,
		)
		if len(errs) != 0 {
			t.Errorf("%s: errs = %+v, want none", language, errs)
		}
	}
}
