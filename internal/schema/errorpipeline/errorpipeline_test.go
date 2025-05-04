package errorpipeline

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Spec 004 §3.6's four-row table, for the rows reachable with only steps 1 and 8
// in place. Three of the four look like bugs and are not — the period is
// appended after whatever punctuation the message already ends with.
func TestTrailingPeriod(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			// The one row with no dictionary entry, measured at
			// expected_errors.yaml:112.
			name:    "an unmatched message just gains the period",
			message: "Input should be a valid string",
			want:    "Input should be a valid string.",
		},
		{
			// §4.12's end_date text ends in `!`, so the final message ends `!.`.
			name: "a message ending in an exclamation keeps it",
			message: "This is not a valid `end_date`. Please use either YYYY-MM-DD," +
				" YYYY-MM, or YYYY format or 'present'!",
			want: "This is not a valid `end_date`. Please use either YYYY-MM-DD," +
				" YYYY-MM, or YYYY format or 'present'!.",
		},
		{
			// §4.7's YouTube text ends `username."`, so the final ends `.".`.
			name:    "a message ending in a quoted sentence gains a second period",
			message: `The username should be a valid YouTube username."`,
			want:    `The username should be a valid YouTube username.".`,
		},
		{
			// §4.11's color text ends `50%)"`, so the final ends `)".`.
			name:    "a message ending in a quoted parenthesis",
			message: `some examples of valid colors: "hsl(0, 100%, 50%)"`,
			want:    `some examples of valid colors: "hsl(0, 100%, 50%)".`,
		},
		{
			name:    "a message already ending in a period is untouched",
			message: "This field is required.",
			want:    "This field is required.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := appendPeriod(test.message); got != test.want {
				t.Errorf("appendPeriod(%q) =\n  %q\nwant\n  %q", test.message, got, test.want)
			}
		})
	}
}

// Step 1 is replacement, not a prefix test, so **every** occurrence goes
// (spec 004 §6 rule 6). The doubled cases are synthetic: no upstream message
// carries either prefix twice. They exist because the rule is contractual and
// the obvious implementation — `strings.TrimPrefix` — satisfies every
// single-occurrence row while getting these wrong.
func TestStripPrefixes(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			// Measured on `email: bad`.
			name:    "the email prefix, once",
			message: "value is not a valid email address: An email address must have an @-sign.",
			want:    "An email address must have an @-sign.",
		},
		{
			// Measured on `date: 2020-13-01`.
			name:    "the value-error prefix, once",
			message: "Value error, month must be in 1..12",
			want:    "month must be in 1..12",
		},
		{
			name:    "the email prefix twice, both removed",
			message: "value is not a valid email address: a value is not a valid email address: b",
			want:    "a b",
		},
		{
			name:    "the value-error prefix twice, both removed",
			message: "Value error, a Value error, b",
			want:    "a b",
		},
		{
			name:    "not at the start, still removed",
			message: "wrapped: Value error, inner",
			want:    "wrapped: inner",
		},
		{
			name:    "a message with neither is unchanged",
			message: "This field is required.",
			want:    "This field is required.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := stripPrefixes(test.message); got != test.want {
				t.Errorf("stripPrefixes(%q) =\n  %q\nwant\n  %q", test.message, got, test.want)
			}
		})
	}
}

// The steps compose, and the first row goes through both the strip and the
// dictionary: `Value error, month must be in 1..12` comes out as row 8's
// replacement.
//
// It does **not** prove step 1 precedes step 7 — see
// TestPrefixStripDoesNotChangeWhichRowMatches for why nothing can.
func TestParseAppliesStripThenPeriod(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{
		{Code: "value_error", Message: "Value error, month must be in 1..12"},
		{Code: "string_type", Message: "Input should be a valid string"},
	}, nil, nil)

	want := []string{"The month must be between 1 and 12.", "Input should be a valid string."}
	if len(got) != len(want) {
		t.Fatalf("Parse returned %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Message != want[i] {
			t.Errorf("record %d message = %q, want %q", i, got[i].Message, want[i])
		}
	}
}

// Record order is the raw order, unsorted (spec 004 §6 rule 1). Asserted on
// messages that would sort differently, so an accidental sort fails here.
func TestParseKeepsRawOrder(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{
		{Message: "zebra"}, {Message: "alpha"}, {Message: "middle"},
	}, nil, nil)

	want := []string{"zebra.", "alpha.", "middle."}
	for i := range want {
		if got[i].Message != want[i] {
			t.Fatalf("messages = %v, want %v", messagesOf(got), want)
		}
	}
}

// Parse does not mutate what it was given. The raw list is iterated more than
// once downstream — for the record and again for its children — so aliasing it
// would corrupt the second pass.
func TestParseLeavesTheRawRecordsAlone(t *testing.T) {
	raw := []schemaerr.ValidationError{{Message: "Value error, boom"}}
	Parse(raw, nil, nil)

	if raw[0].Message != "Value error, boom" {
		t.Errorf("raw record was mutated: %q", raw[0].Message)
	}
}

func messagesOf(records []schemaerr.ValidationError) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.Message)
	}
	return out
}

// Step 3, in the shape the port gives it. The one producer that needs it is the
// theme-name failure of spec 004 §4.27: its raw location is `("design",)` and
// its final one is `("design", "theme")`, pinned at
// `expected_errors.yaml:141-145`.
//
// `design` is a discriminated root, so without the flag step 2 would drop
// `theme` as a branch value — the unpinned half below is what makes the pinned
// half mean something.
func TestParseSkipsTheDiscriminatorForAPinnedLocation(t *testing.T) {
	pinned := schemaerr.ValidationError{
		Message:         "nope",
		SchemaLocation:  []string{"design", "theme"},
		LocationIsFinal: true,
	}
	if got := Parse([]schemaerr.ValidationError{pinned}, nil, nil)[0]; len(got.SchemaLocation) != 2 ||
		got.SchemaLocation[1] != "theme" {
		t.Errorf("location = %v, want it left alone", got.SchemaLocation)
	}

	// The same location without the flag loses its second element, which is what
	// makes the assertion above mean something.
	unpinned := pinned
	unpinned.LocationIsFinal = false
	if got := Parse([]schemaerr.ValidationError{unpinned}, nil, nil)[0]; len(got.SchemaLocation) != 1 {
		t.Errorf("location = %v, want the branch element dropped", got.SchemaLocation)
	}
}

// The other half of step 3: an overriding input. Upstream reads it from `ctx`;
// the port has no context map, so the validator writes Input directly and the
// pipeline carries it through untouched.
//
// §4.27 is the measured case — the record shows the theme *name*, not the whole
// `design` mapping, which is what an unoverridden input would render as `...`
// (`expected_errors.yaml:143`).
func TestParseCarriesAnOverriddenInput(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{{
		Code:            "rendercv_other_error",
		SchemaLocation:  []string{"design", "theme"},
		LocationIsFinal: true,
		Message:         "The theme `nope` is not available",
		Input:           "nope",
	}}, nil, nil)[0]

	if got.Input != "nope" {
		t.Errorf("input = %q, want the validator's own value", got.Input)
	}
}

// Steps 9 and 12. All three conditions must hold to leave the main document:
// overlays supplied, a non-empty location, and a first element that names one.
func TestSelectSource(t *testing.T) {
	main := &yamldoc.Node{Kind: yamldoc.KindMapping}
	design := &yamldoc.Node{Kind: yamldoc.KindMapping}
	locale := &yamldoc.Node{Kind: yamldoc.KindMapping}
	overlays := map[schemaerr.OverlayKey]*yamldoc.Node{
		schemaerr.OverlayDesign: design,
		schemaerr.OverlayLocale: locale,
	}

	tests := []struct {
		name       string
		location   []string
		overlays   map[schemaerr.OverlayKey]*yamldoc.Node
		wantSource schemaerr.YamlSource
		wantDoc    *yamldoc.Node
	}{
		{
			name:       "no overlays, so the main file whatever the root",
			location:   []string{"design", "theme"},
			overlays:   nil,
			wantSource: schemaerr.SourceMain,
			wantDoc:    main,
		},
		{
			name:       "an overlay root with that overlay supplied",
			location:   []string{"design", "theme"},
			overlays:   overlays,
			wantSource: schemaerr.SourceDesign,
			wantDoc:    design,
		},
		{
			name:       "the other overlay",
			location:   []string{"locale", "month"},
			overlays:   overlays,
			wantSource: schemaerr.SourceLocale,
			wantDoc:    locale,
		},
		{
			name:       "a cv-rooted record keeps the main document",
			location:   []string{"cv", "name"},
			overlays:   overlays,
			wantSource: schemaerr.SourceMain,
			wantDoc:    main,
		},
		{
			// `settings` is an overlay key, but no settings overlay was supplied
			// here — the map lookup, not the key list, is what decides.
			name:       "an overlay key with no document supplied",
			location:   []string{"settings", "current_date"},
			overlays:   overlays,
			wantSource: schemaerr.SourceMain,
			wantDoc:    main,
		},
		{
			// The filter can empty a location entirely.
			name:       "an empty location",
			location:   nil,
			overlays:   overlays,
			wantSource: schemaerr.SourceMain,
			wantDoc:    main,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, doc := selectSource(test.location, main, test.overlays)
			if source != test.wantSource {
				t.Errorf("source = %q, want %q", source, test.wantSource)
			}
			if doc != test.wantDoc {
				t.Errorf("document = %p, want %p", doc, test.wantDoc)
			}
		})
	}
}

// The source lands on the record, and it is chosen after step 4 — a `design`
// record whose branch element step 2 dropped still roots at `design`.
func TestParseSetsTheOverlaySource(t *testing.T) {
	design := &yamldoc.Node{Kind: yamldoc.KindMapping}
	got := Parse(
		[]schemaerr.ValidationError{{
			SchemaLocation: []string{"design", "classic", "page", "top_margin"},
			Message:        "nope",
		}},
		&yamldoc.Node{Kind: yamldoc.KindMapping},
		map[schemaerr.OverlayKey]*yamldoc.Node{schemaerr.OverlayDesign: design},
	)[0]

	if got.YamlSource != schemaerr.SourceDesign {
		t.Errorf("source = %q, want %q", got.YamlSource, schemaerr.SourceDesign)
	}
}
