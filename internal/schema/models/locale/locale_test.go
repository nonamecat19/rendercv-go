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

// localeBlock parses a whole `locale:` mapping and returns it together with its
// `language` value. Both halves are needed: the discriminator failure quotes the
// tag but reports against the block.
func localeBlock(t *testing.T, value string) (block, language *yamldoc.Node) {
	t.Helper()
	doc, err := yamlreader.ReadString("language: " + value + "\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc, doc.Items[0].Value
}

// Spec 004 §3.17 behavior 67 and §4.30, asserted as the whole literal — the
// enumeration order is pydantic's and nothing weaker would catch a reordering.
func TestUnknownLanguage(t *testing.T) {
	block, language := localeBlock(t, "klingon")
	errs := locale.ValidateLanguage(block, language, []string{"locale"}, schemaerr.SourceMain)
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

	// The Input Value column is the **block**, so it renders as a mapping does.
	// Upstream's `input` for a discriminator failure is the object it was
	// resolving a member for, never the tag the message already quotes.
	if errs[0].Input != "..." {
		t.Errorf("input = %q, want the locale mapping's rendering", errs[0].Input)
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

// A TaggedScalar is no tag of the discriminator, however its text reads: the
// union is over `str` literals and a TaggedScalar is not one.
//
// Measured on `locale: {language: !!str english}`, which upstream rejects with
// the whole enumeration (1970 bytes) where the port let the tag through and
// reported `string_type` from the catalog binder (1318 bytes).
func TestATaggedLanguageIsNoTag(t *testing.T) {
	for _, src := range []string{"!!str english", "!u turkish"} {
		t.Run(src, func(t *testing.T) {
			block, language := localeBlock(t, src)
			errs := locale.ValidateLanguage(block, language, []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != locale.CodeUnionTag {
				t.Errorf("code = %q, want union_tag_invalid", errs[0].Code)
			}
			if !strings.Contains(errs[0].Message, "found using 'language'") {
				t.Errorf("message = %q, want the discriminator failure", errs[0].Message)
			}
		})
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
		block, tag := localeBlock(t, `"`+language+`"`)
		errs := locale.ValidateLanguage(block, tag, []string{"locale"}, schemaerr.SourceMain)
		if len(errs) != 0 {
			t.Errorf("%s: errs = %+v, want none", language, errs)
		}
	}
}
