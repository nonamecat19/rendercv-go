package cv_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/inputpath"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func parse(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return node
}

// Spec §3.44 — the ten fields, in declaration order.
func TestFieldOrder(t *testing.T) {
	want := []string{
		"name", "headline", "location", "email", "photo",
		"phone", "website", "social_networks", "custom_connections", "sections",
	}
	got := cv.FieldNames()
	if len(got) != len(want) {
		t.Fatalf("field names = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("field names = %v, want %v", got, want)
		}
	}
}

// Spec §3.44 — every field is accepted and every one is optional.
func TestEveryFieldAccepted(t *testing.T) {
	src := "" +
		"name: John Doe\n" +
		"headline: Engineer\n" +
		"location: Istanbul\n" +
		"email: john@example.com\n" +
		// A URL rather than a path: `photo` is a real union now, and a relative
		// path would have to exist on disk for this fixture to mean "accepted".
		// TestResolvePhotoReportsOnlyThePathFailure covers the path arm.
		"photo: https://example.com/photo.jpg\n" +
		"phone: \"+905419999999\"\n" +
		"website: https://example.com\n" +
		"social_networks:\n  - network: GitHub\n    username: johndoe\n" +
		"custom_connections:\n  - fontawesome_icon: fa\n    placeholder: p\n    url: null\n" +
		"sections:\n  education: []\n"

	model, errs := cv.Validate(parse(t, src), []string{"cv"}, schemaerr.SourceMain, testOptions())
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	for name, node := range map[string]*yamldoc.Node{
		"name":               model.Name,
		"headline":           model.Headline,
		"location":           model.Location,
		"email":              model.Email,
		"photo":              model.Photo,
		"phone":              model.Phone,
		"website":            model.Website,
		"social_networks":    model.SocialNetworks,
		"custom_connections": model.CustomConnections,
		"sections":           model.Sections,
	} {
		if node == nil {
			t.Errorf("%s = nil, want a bound node", name)
		}
	}
}

func TestEmptyMappingValidates(t *testing.T) {
	model, errs := cv.Validate(parse(t, "{}\n"), []string{"cv"}, schemaerr.SourceMain, testOptions())
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if len(model.KeyOrder()) != 0 {
		t.Errorf("key order = %v, want empty", model.KeyOrder())
	}
}

// Spec §3.45 — an eleventh key is rejected.
func TestUnknownKeyRejected(t *testing.T) {
	_, errs := cv.Validate(parse(t, "name: John\nnickname: Johnny\n"), []string{"cv"}, schemaerr.SourceMain, testOptions())
	if len(errs) != 1 || errs[0].Code != binder.CodeExtraForbidden {
		t.Fatalf("errs = %+v, want one extra_forbidden", errs)
	}
	want := []string{"cv", "nickname"}
	got := errs[0].SchemaLocation
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("schema location = %v, want %v", got, want)
	}
}

// Spec §3.50, §5.15 — recorded order keeps input order and drops null-valued keys.
func TestKeyOrder(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "nulls dropped",
			input: "name: John\nemail: null\nlocation: Istanbul\n",
			want:  []string{"name", "location"},
		},
		{
			name:  "input order, not declaration order",
			input: "location: Istanbul\nname: John\n",
			want:  []string{"location", "name"},
		},
		{
			name:  "absent keys never appear",
			input: "name: John\n",
			want:  []string{"name"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model, errs := cv.Validate(parse(t, tc.input), []string{"cv"}, schemaerr.SourceMain, testOptions())
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			got := model.KeyOrder()
			if len(got) != len(tc.want) {
				t.Fatalf("key order = %v, want %v", got, tc.want)
			}
			for i, key := range tc.want {
				if got[i] != key {
					t.Fatalf("key order = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// Spec §5.16 — a non-mapping input records an empty key order.
func TestNonMappingInputKeyOrder(t *testing.T) {
	model, errs := cv.Validate(
		&yamldoc.Node{Kind: yamldoc.KindSequence},
		[]string{"cv"}, schemaerr.SourceMain, testOptions(),
	)
	if len(model.KeyOrder()) != 0 {
		t.Errorf("key order = %v, want empty", model.KeyOrder())
	}
	if len(errs) == 0 {
		t.Error("errs = none, want the type failure")
	}
}

// Spec §3.51 — a recorded order cannot be disturbed by a caller.
func TestKeyOrderIsACopy(t *testing.T) {
	model, _ := cv.Validate(parse(t, "name: John\nlocation: Istanbul\n"), []string{"cv"}, schemaerr.SourceMain, testOptions())
	order := model.KeyOrder()
	order[0] = "tampered"
	if model.KeyOrder()[0] != "name" {
		t.Errorf("key order = %v, want the recorded order intact", model.KeyOrder())
	}
}

// testOptions supplies the fixture registry and an empty context, so the field
// validators wired into cv.Validate run in tests exactly as they do in the
// pipeline.
func testOptions() cv.Options {
	return cv.Options{Registry: fixtureRegistry()}
}

// The field validators are reachable through cv.Validate, not only standalone:
// a bad social network, a bad custom connection and a non-list section all
// surface from one call.
func TestValidateRunsFieldValidators(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "social network missing username",
			input: "social_networks:\n  - network: GitHub\n",
			want:  "Field required",
		},
		{
			name:  "unsupported network",
			input: "social_networks:\n  - network: Friendster\n    username: x\n",
			want: "Input should be 'LinkedIn', 'GitHub', 'GitLab', 'IMDB', 'Instagram'," +
				" 'ORCID', 'Mastodon', 'StackOverflow', 'ResearchGate', 'YouTube'," +
				" 'Google Scholar', 'Telegram', 'WhatsApp', 'Leetcode', 'X', 'Bluesky'" +
				" or 'Reddit'",
		},
		{
			name:  "custom connection missing url",
			input: "custom_connections:\n  - fontawesome_icon: fa\n    placeholder: p\n",
			want:  "Field required",
		},
		{
			name:  "section is not a list",
			input: "sections:\n  education:\n    institution: MIT\n",
			want:  "Each section should be a list of entries! This is not a list.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := cv.Validate(parse(t, tc.input), []string{"cv"}, schemaerr.SourceMain, testOptions())
			if len(errs) == 0 {
				t.Fatalf("errs = none, want %q", tc.want)
			}
			if errs[0].Message != tc.want {
				t.Errorf("message = %q, want %q", errs[0].Message, tc.want)
			}
		})
	}
}

// Spec §3.46 — the photo union resolves through cv.Validate, path first.
func TestValidateResolvesPhoto(t *testing.T) {
	model, errs := cv.Validate(
		parse(t, "photo: https://example.com/me.jpg\n"),
		[]string{"cv"}, schemaerr.SourceMain, testOptions(),
	)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if model.PhotoValue == nil {
		t.Fatal("photo = nil, want the union resolved")
	}
	if model.PhotoValue.Kind != cv.PhotoKindURL {
		t.Errorf("photo kind = %v, want the URL branch for a nonexistent path", model.PhotoValue.Kind)
	}
}

// Both arms of the photo union take a `str`, so a non-string fails on type
// before either arm runs: upstream reports pydantic's `path_type`, not a
// resolution message naming a file the user never wrote.
//
// Measured on 5, true, 1.5, a sequence, a mapping and a tagged scalar — all six
// 1411 bytes at exit 1, against the port's 1318 for the scalars and a bare
// `open : no such file or directory` for the two collections, whose empty Raw
// resolved to the empty path and reached the renderer.
func TestANonStringPhotoIsAPathTypeFailure(t *testing.T) {
	tests := []struct {
		src       string
		wantInput string
	}{
		{src: "photo: 5\n", wantInput: "5"},
		// `str(True)`, not the YAML token.
		{src: "photo: true\n", wantInput: "True"},
		{src: "photo: 1.5\n", wantInput: "1.5"},
		{src: "photo: []\n", wantInput: "..."},
		{src: "photo: {}\n", wantInput: "..."},
		{src: "photo: !!str x\n", wantInput: "x"},
	}

	for _, test := range tests {
		t.Run(strings.TrimSpace(test.src), func(t *testing.T) {
			_, errs := cv.Validate(
				parse(t, test.src), []string{"cv"}, schemaerr.SourceMain, testOptions(),
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != inputpath.CodePathType {
				t.Errorf("code = %q, want path_type", errs[0].Code)
			}
			if errs[0].Message != inputpath.MessagePathType {
				t.Errorf("message = %q", errs[0].Message)
			}
			if errs[0].Input != test.wantInput {
				t.Errorf("input = %q, want %q", errs[0].Input, test.wantInput)
			}
		})
	}
}

// The Input Value column of a photo resolution failure is the value the user
// wrote, not the path the message interpolates. They agree for an ordinary
// relative path and disagree for `photo: ""`, whose column is empty while the
// message names `.` (measured: both sides 1318 bytes).
func TestAnEmptyPhotoPathIsTheCurrentDirectory(t *testing.T) {
	_, errs := cv.Validate(
		parse(t, "photo: \"\"\n"), []string{"cv"}, schemaerr.SourceMain, testOptions(),
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Message != "The path `.` is not a file." {
		t.Errorf("message = %q", errs[0].Message)
	}
	if errs[0].Input != "" {
		t.Errorf("input = %q, want the empty string the user wrote", errs[0].Input)
	}
}

// Spec 004 §3.9 behavior 32 step 3 and §6 rule 2, on the measured seven-record
// input: every declared field first in declaration order, then the unknown keys
// in input order — including inside a nested model, which reports entirely
// before the outer model's unknown keys.
//
// The two unknown keys are written `zzz` then `aaa` so input order and
// alphabetical order disagree.
func TestDeclaredFieldErrorsPrecedeExtraKeys(t *testing.T) {
	src := "" +
		"name: John\n" +
		"email: bad\n" +
		"zzz_extra: 1\n" +
		"phone: nope\n" +
		"social_networks:\n" +
		"  - network: Nope\n" +
		"    username: \"\"\n" +
		"    extra_here: 1\n" +
		"aaa_extra: 2\n"

	_, errs := cv.Validate(parse(t, src), []string{"cv"}, schemaerr.SourceMain, testOptions())

	want := []string{
		"cv.email",
		"cv.phone",
		"cv.social_networks.0.network",
		"cv.social_networks.0.extra_here",
		"cv.zzz_extra",
		"cv.aaa_extra",
	}
	got := make([]string, 0, len(errs))
	for _, e := range errs {
		got = append(got, strings.Join(e.SchemaLocation, "."))
	}
	if len(got) != len(want) {
		t.Fatalf("locations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("locations = %v, want %v", got, want)
		}
	}
}

// The three plain-text fields are `str | None` upstream (cv.py:32, :36, :40),
// and pydantic's lax mode coerces nothing to a `str`. They were declared with
// no shape, so **`cv.name: 200` rendered a CV named `200` at exit 0** where
// upstream exits 1 — measured against the vendored Python, which reports
// `cv.name │ 200 │ Input should be a valid string.`
//
// Found while porting explicit YAML tags (spec 015): a tag made these nodes
// reach the field instead of being dropped as nulls, which is what had hidden
// the gap.
func TestPlainTextFieldsRejectANonString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		field string
	}{
		{name: "an integer name", input: "name: 200\n", field: "cv.name"},
		{name: "a float name", input: "name: 0.5\n", field: "cv.name"},
		{name: "a bool name", input: "name: true\n", field: "cv.name"},
		{name: "a tagged name", input: "name: !!str Bob\n", field: "cv.name"},
		{name: "an integer headline", input: "headline: 200\n", field: "cv.headline"},
		{name: "an integer location", input: "location: 200\n", field: "cv.location"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := cv.Validate(
				parse(t, tc.input), []string{"cv"}, schemaerr.SourceMain, testOptions())
			if len(errs) != 1 {
				t.Fatalf("errors = %+v, want exactly one", errs)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != tc.field {
				t.Errorf("location = %q, want %q", got, tc.field)
			}
			if errs[0].Message != "Input should be a valid string" {
				t.Errorf("message = %q, want the string-type message", errs[0].Message)
			}
		})
	}
}

// A string is still a string, tags aside — the shape check must not reject the
// documents every corpus case is built from.
func TestPlainTextFieldsAcceptAString(t *testing.T) {
	for _, input := range []string{
		"name: John Doe\n", "name: \"200\"\n", "name: '0.5'\n", "name: null\n",
		"headline: Engineer\n", "location: Istanbul\n",
	} {
		t.Run(input, func(t *testing.T) {
			_, errs := cv.Validate(
				parse(t, input), []string{"cv"}, schemaerr.SourceMain, testOptions())
			if len(errs) != 0 {
				t.Errorf("errors = %+v, want none", errs)
			}
		})
	}
}
