package entries_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var publicationReference = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// publicationMinimal is the two required fields, so a test can add exactly the
// one key it is about.
const publicationMinimal = "title: T\nauthors:\n  - J. Doe\n"

func validatePublication(t *testing.T, src string) (*entries.PublicationEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidatePublicationEntry(
		node, []string{"cv", "sections", "x", "0"}, schemaerr.SourceMain, publicationReference,
	)
}

// Spec §3.10 behavior 15 — six own fields then `date`, asserted positionally
// because `date` being *last* is the counter-intuitive half: it comes from the
// first-listed base, and pydantic emits the last-listed base's fields first.
// Runtime order verified against the vendored Python:
// `['title', 'authors', 'summary', 'doi', 'url', 'journal', 'date']`.
func TestPublicationDescriptorFields(t *testing.T) {
	got := entries.PublicationDescriptor()
	if got.Name != "PublicationEntry" {
		t.Errorf("name = %q, want %q", got.Name, "PublicationEntry")
	}

	want := []string{"title", "authors", "summary", "doi", "url", "journal", "date"}
	if len(got.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", got.Fields, want)
	}
	for i, name := range want {
		if got.Fields[i] != name {
			t.Fatalf("fields = %v, want %v", got.Fields, want)
		}
	}
}

// Spec §3.10 behavior 16 — the base is BaseEntryWithDate, so the four
// complex fields are absent. `doi_url` is absent too: it is a derived method,
// not a field, and a declared field would leak into iteration 5's JSON Schema
// (spec §3.12 behavior 22, plan §7 hazard 1).
func TestPublicationDescriptorDeclaresNoOtherFields(t *testing.T) {
	declared := make(map[string]bool)
	for _, name := range entries.PublicationDescriptor().Fields {
		declared[name] = true
	}
	for _, name := range []string{
		"start_date", "end_date", "location", "highlights",
		"doi_url", "DOIURL",
		"main_column", "date_and_location_column", "degree_column",
	} {
		if declared[name] {
			t.Errorf("field %q is declared, want it absent", name)
		}
	}
}

// Spec §5.17 — the conftest fixture, with its exact values, including the
// triple-asterisk author `***H. Tom***`
// (tests/schema/models/cv/conftest.py:43-52).
func TestPublicationFixtureValidates(t *testing.T) {
	const title = "Magneto-Thermal Thin Shell Approximation for 3D Finite Element" +
		" Analysis of No-Insulation Coils"
	entry, errs := validatePublication(t, ""+
		"title: "+title+"\n"+
		"authors:\n"+
		"  - J. Doe\n"+
		// Quoted because a leading `*` is a YAML alias; the value is the
		// fixture's own bytes.
		"  - \"***H. Tom***\"\n"+
		"  - S. Doe\n"+
		"  - A. Andsurname\n"+
		"date: \"2021-12-08\"\n"+
		"journal: IEEE Transactions on Applied Superconductivity\n"+
		"doi: 10.1109/TASC.2023.3340648\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Title == nil || entry.Title.Raw != title {
		t.Errorf("title = %+v, want %q", entry.Title, title)
	}
	if entry.Authors == nil || len(entry.Authors.Elems) != 4 {
		t.Fatalf("authors = %+v, want four elements", entry.Authors)
	}
	if got := entry.Authors.Elems[1].Raw; got != "***H. Tom***" {
		t.Errorf("authors[1] = %q, want %q", got, "***H. Tom***")
	}
	if entry.Date == nil || entry.Date.Raw != "2021-12-08" {
		t.Errorf("date = %+v, want %q", entry.Date, "2021-12-08")
	}
	if entry.DOI == nil || entry.DOI.Raw != "10.1109/TASC.2023.3340648" {
		t.Errorf("doi = %+v, want %q", entry.DOI, "10.1109/TASC.2023.3340648")
	}
	if entry.Journal == nil || entry.Journal.Raw != "IEEE Transactions on Applied Superconductivity" {
		t.Errorf("journal = %+v, want the fixture's journal", entry.Journal)
	}
}

// Spec §5.11 — upstream parametrizes `extra_attribute` over every entry model and
// asserts it is readable (tests/schema/models/cv/test_section.py:63-83).
func TestPublicationExtraAttributeRetained(t *testing.T) {
	entry, errs := validatePublication(t, publicationMinimal+"extra_attribute: extra value\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("extra_attribute")
	if !ok {
		t.Fatalf("extra_attribute absent, want it retained")
	}
	if value.Raw != "extra value" {
		t.Errorf("extra_attribute = %q, want %q", value.Raw, "extra value")
	}
}

// Spec §4.3, §5.8 — required fields absent report `missing`, in declaration
// order, so `title` precedes `authors` when both are gone.
func TestPublicationMissingFields(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "both absent report title then authors",
			src:  "journal: Nature\n",
			want: []string{"cv.sections.x.0.title", "cv.sections.x.0.authors"},
		},
		{
			name: "title absent",
			src:  "authors:\n  - J. Doe\n",
			want: []string{"cv.sections.x.0.title"},
		},
		{
			name: "authors absent",
			src:  "title: T\n",
			want: []string{"cv.sections.x.0.authors"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validatePublication(t, test.src)
			if len(errs) != len(test.want) {
				t.Fatalf("errs = %+v, want %d", errs, len(test.want))
			}
			for i, want := range test.want {
				if got := strings.Join(errs[i].SchemaLocation, "."); got != want {
					t.Errorf("errs[%d] location = %q, want %q", i, got, want)
				}
				if errs[i].Code != binder.CodeMissing {
					t.Errorf("errs[%d] code = %q, want %q", i, errs[i].Code, binder.CodeMissing)
				}
			}
		})
	}
}

// Spec §5.7 — a required field written null is a type error, not a missing
// field. Verified against the vendored Python: `{title: None, authors: ["a"]}`
// reports `string_type` at `title`.
func TestPublicationNullRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		src  string
		path string
		code schemaerr.Code
	}{
		{
			name: "title null is string_type",
			src:  "title: null\nauthors:\n  - a\n",
			path: "cv.sections.x.0.title",
			code: binder.CodeStringType,
		},
		{
			name: "authors null is list_type",
			src:  "title: T\nauthors: null\n",
			path: "cv.sections.x.0.authors",
			code: binder.CodeListType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validatePublication(t, test.src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != test.code {
				t.Errorf("code = %q, want %q", errs[0].Code, test.code)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != test.path {
				t.Errorf("location = %q, want %q", got, test.path)
			}
		})
	}
}

// Spec §4.5, §5.10 — `authors: "scalar"` reports the missing `title` *first* and
// then the list failure, because pydantic walks fields in declaration order and
// interleaves absences with shape failures. Verified against the vendored
// Python: `[{loc: ["title"], type: "missing"}, {loc: ["authors"], type:
// "list_type"}]`.
func TestPublicationScalarAuthorsReportsTitleFirst(t *testing.T) {
	_, errs := validatePublication(t, "authors: scalar\n")
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want two", errs)
	}
	if errs[0].Code != binder.CodeMissing ||
		strings.Join(errs[0].SchemaLocation, ".") != "cv.sections.x.0.title" {
		t.Errorf("errs[0] = %+v, want missing at title", errs[0])
	}
	if errs[1].Code != binder.CodeListType ||
		strings.Join(errs[1].SchemaLocation, ".") != "cv.sections.x.0.authors" {
		t.Errorf("errs[1] = %+v, want list_type at authors", errs[1])
	}
}

// Spec §4.4, §5.9 — a non-text list element is reported at its own index.
// Verified against the vendored Python: `authors: [1, 2]` reports `string_type`
// at `authors.0` and `authors.1`.
func TestPublicationNonTextAuthorElements(t *testing.T) {
	_, errs := validatePublication(t, "title: T\nauthors:\n  - 1\n  - 2\n")
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want two", errs)
	}
	for i, want := range []string{"cv.sections.x.0.authors.0", "cv.sections.x.0.authors.1"} {
		if errs[i].Code != binder.CodeStringType {
			t.Errorf("errs[%d] code = %q, want %q", i, errs[i].Code, binder.CodeStringType)
		}
		if got := strings.Join(errs[i].SchemaLocation, "."); got != want {
			t.Errorf("errs[%d] location = %q, want %q", i, got, want)
		}
	}
}

// Spec §3.11, §4.1 — a `doi` that does not match is rejected with pydantic's own
// message, carrying the pattern exactly as written at publication.py:34.
// Verified against the vendored Python for `notadoi`.
func TestPublicationDOIPatternMismatch(t *testing.T) {
	_, errs := validatePublication(t, publicationMinimal+"doi: notadoi\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != entries.CodeStringPatternMismatch {
		t.Errorf("code = %q, want %q", errs[0].Code, entries.CodeStringPatternMismatch)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.sections.x.0.doi" {
		t.Errorf("location = %q, want %q", got, "cv.sections.x.0.doi")
	}
	const want = `String should match pattern '\b10\..*'`
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
}

// Spec §3.11 behavior 18 — the pattern is a search, not an anchored match.
func TestPublicationDOIPatternIsASearch(t *testing.T) {
	_, errs := validatePublication(t, publicationMinimal+"doi: prefix 10.5/x\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
}

// Spec §5.3 — the pair upstream's own test pins
// (tests/schema/models/cv/entries/test_publication.py:7-17): the DOI it uses and
// the absent case.
func TestPublicationDOIURL(t *testing.T) {
	entry, errs := validatePublication(t, publicationMinimal+"doi: 10.1109/TASC.2023.3340648\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	const want = "https://doi.org/10.1109/TASC.2023.3340648"
	if got := entry.DOIURL(); got != want {
		t.Errorf("DOIURL() = %q, want %q", got, want)
	}

	absent, errs := validatePublication(t, publicationMinimal)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if got := absent.DOIURL(); got != "" {
		t.Errorf("DOIURL() = %q, want the empty string for an absent doi", got)
	}
	if absent.DOI != nil {
		t.Errorf("doi = %+v, want absent", absent.DOI)
	}
}

// Spec §3.12 behavior 22, §5.2 — the `doi` bytes are concatenated verbatim: no
// trimming, no escaping, no percent-encoding. Expectations obtained by running
// the vendored Python on the same three inputs (spec §7.4), which produced
// `https://doi.org/10. spaced ?`, `https://doi.org/10.###` and
// `https://doi.org/10.5\n`, all with zero errors.
func TestPublicationDOIURLPreservesBytes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "spaces and a question mark", src: `doi: "10. spaced ?"` + "\n", want: "https://doi.org/10. spaced ?"},
		{name: "hashes", src: `doi: "10.###"` + "\n", want: "https://doi.org/10.###"},
		{name: "trailing newline", src: `doi: "10.5\n"` + "\n", want: "https://doi.org/10.5\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, errs := validatePublication(t, publicationMinimal+test.src)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if got := entry.DOIURL(); got != test.want {
				t.Errorf("DOIURL() = %q, want %q", got, test.want)
			}
		})
	}
}

// Spec §3.12 behavior 23, §4.2 — the only reachable failure of the generated DOI
// URL is its length, and it names the **entry**, not a field within it.
//
// **This test used to assert an empty location, citing a measurement of
// `[{loc: []}]`.** That measurement was taken by validating a bare
// `PublicationEntry`, which no upstream code path does. `section.py:229`
// validates the wrapper shape `{"entries": [...]}`, and re-measured at that
// level the same input gives `[{loc: ["entries", 0], type: "url_too_long"}]` —
// two entries give `["entries", 0]` and `["entries", 1]`. An empty location made
// the spliced record collide with its own wrapper's, and dedup then deleted the
// row (`errorpipeline/splice_test.go`), so the whole error never reached a user.
//
// It is the entry's own location, exactly like the start-after-end rule beside
// it in `bases/complexfieldsentry.go`.
func TestPublicationDOIURLTooLong(t *testing.T) {
	_, errs := validatePublication(t, publicationMinimal+"doi: 10."+strings.Repeat("a", 2100)+"\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	// Upstream's literal, not the Go constant, for the reason spelled out in
	// TestNormalDateRejections.
	if errs[0].Code != "url_too_long" {
		t.Errorf("code = %q, want %q", errs[0].Code, "url_too_long")
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.sections.x.0" {
		t.Errorf("location = %q, want the entry's own %q", got, "cv.sections.x.0")
	}
	const want = "URL should have at most 2083 characters"
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
}

// Spec §3.12 behaviors 21-23, §5.4, §5.24 — the four doi/url states.
//
// This is the only gate these two rules will ever have: no golden case sets
// `url` at all and every one sets `doi` (spec §5.24, plan §7 hazard 5), so the
// conformance suite is permanently silent here. The two literals are upstream's
// own (tests/renderer/conftest.py:282-283) and every expectation below was
// obtained by running the vendored Python on the same four inputs (spec §7.4).
//
// One deliberate difference is recorded rather than asserted: upstream's URL type
// normalizes, so its "url only" row holds `https://example.com/` with a trailing
// slash, while this port keeps the bytes as written. That normalization is
// iteration 4's decision for `url`, `email`, `phone` and `website` together
// (spec §5.5, §7.3) and is not a divergence this iteration takes.
func TestPublicationDOIBeatsURL(t *testing.T) {
	const doi = "10.1007/978-3-319-69626-3_101-1"
	const url = "https://example.com"

	tests := []struct {
		name    string
		src     string
		wantDOI string
		wantURL string
		wantURI string
	}{
		{name: "neither"},
		{
			name:    "doi only",
			src:     "doi: " + doi + "\n",
			wantDOI: doi,
			wantURI: "https://doi.org/" + doi,
		},
		{
			name:    "url only",
			src:     "url: " + url + "\n",
			wantURL: url,
		},
		{
			name:    "both keep doi and clear url",
			src:     "doi: " + doi + "\nurl: " + url + "\n",
			wantDOI: doi,
			wantURI: "https://doi.org/" + doi,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, errs := validatePublication(t, publicationMinimal+test.src)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if got := nodeText(entry.DOI); got != test.wantDOI {
				t.Errorf("doi = %q, want %q", got, test.wantDOI)
			}
			if got := nodeText(entry.URL); got != test.wantURL {
				t.Errorf("url = %q, want %q", got, test.wantURL)
			}
			if got := entry.DOIURL(); got != test.wantURI {
				t.Errorf("DOIURL() = %q, want %q", got, test.wantURI)
			}
		})
	}
}

// Spec §3.10 behavior 16, §5.6 — a publication entry has no `start_date`, so an
// invalid one is retained as an unknown key and gets no date validation at all.
// Verified against the vendored Python: `{title, authors, start_date:
// "not-a-date"}` validates and lands in `model_extra`.
func TestPublicationStartDateIsAnUnvalidatedUnknownKey(t *testing.T) {
	entry, errs := validatePublication(t, publicationMinimal+"start_date: not-a-date\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("start_date")
	if !ok {
		t.Fatalf("start_date absent from extras, want it retained")
	}
	if value.Kind != yamldoc.KindString || value.Raw != "not-a-date" {
		t.Errorf("start_date = %+v, want the string %q", value, "not-a-date")
	}
}

// Spec §5.25, §7.4 — a non-blank `summary` on a publication entry appears in no
// YAML file in the submodule, so no golden case covers it. The expectation was
// obtained by running the vendored Python on this input, which retained
// `'This paper presents a new method for computer vision.'` verbatim with zero
// errors.
func TestPublicationSummaryRetainedVerbatim(t *testing.T) {
	const summary = "This paper presents a new method for computer vision."
	entry, errs := validatePublication(t, publicationMinimal+"summary: "+summary+"\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Summary == nil || entry.Summary.Raw != summary {
		t.Errorf("summary = %+v, want %q", entry.Summary, summary)
	}
}

// Spec §4.9-§4.13 — metadata strings iteration 5 emits verbatim into the JSON
// Schema, pinned here against publication.py:20, :30-32, :38, :42 and :94.
func TestPublicationMetadataStrings(t *testing.T) {
	tests := []struct{ got, want string }{
		{entries.AuthorsDescription, "You can bold your name with **double asterisks**."},
		{
			entries.DOIDescription,
			"The DOI (Digital Object Identifier). If provided, it will be used as the link instead of the URL.",
		},
		{entries.URLDescription, "A URL link to the publication. Ignored if DOI is provided."},
		{entries.JournalDescription, "The journal, conference, or venue where it was published."},
		{entries.DOIURLPrefix, "https://doi.org/"},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("metadata = %q, want %q", test.got, test.want)
		}
	}
}

func nodeText(node *yamldoc.Node) string {
	if node == nil {
		return ""
	}
	return node.Raw
}

// Spec 004 §3.13 behavior 41: `PublicationEntry.url` is declared
// `pydantic.HttpUrl`, and upstream parses it during **field** validation — so
// its failure lands at `url`'s declared position, between `doi` and `journal`.
//
// No golden case sets this field (spec 003 §5.24), so these are the only gate
// this decision will ever have.
func TestPublicationURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		code string
		want string
	}{
		{
			name: "a value that does not parse",
			url:  "not a url",
			code: "url_parsing",
			// The dictionary key; the pipeline replaces it with §4.9.
			want: "Input should be a valid URL",
		},
		{
			name: "the wrong scheme",
			url:  "ftp://example.com",
			code: "url_scheme",
			want: "URL scheme should be 'http' or 'https'",
		},
		{
			name: "too long",
			url:  "https://example.com/" + strings.Repeat("a", 2100),
			code: "url_too_long",
			want: "URL should have at most 2083 characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validatePublication(t, publicationMinimal+"url: "+test.url+"\n")
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if string(errs[0].Code) != test.code {
				t.Errorf("code = %q, want %q", errs[0].Code, test.code)
			}
			if errs[0].Message != test.want {
				t.Errorf("message = %q, want %q", errs[0].Message, test.want)
			}
			if last := errs[0].SchemaLocation[len(errs[0].SchemaLocation)-1]; last != "url" {
				t.Errorf("location ends %q, want url", last)
			}
		})
	}

	t.Run("a valid URL is accepted", func(t *testing.T) {
		_, errs := validatePublication(t, publicationMinimal+"url: https://example.com/paper\n")
		if len(errs) != 0 {
			t.Errorf("errs = %+v, want none", errs)
		}
	})
}

// The three URL failure kinds reach the record through one binder hook, so the
// hook must take the code from the error rather than from a single registered
// constant. `doi` is the counter-example: one failure kind, one ScalarCode.
func TestPublicationURLCodesAreDistinct(t *testing.T) {
	seen := map[schemaerr.Code]bool{}
	for _, url := range []string{
		"not a url", "ftp://example.com",
		"https://example.com/" + strings.Repeat("a", 2100),
	} {
		_, errs := validatePublication(t, publicationMinimal+"url: "+url+"\n")
		if len(errs) == 1 {
			seen[errs[0].Code] = true
		}
	}
	if len(seen) != 3 {
		t.Errorf("reached %d distinct codes, want 3 — a single ScalarCode would"+
			" collapse them", len(seen))
	}
}
