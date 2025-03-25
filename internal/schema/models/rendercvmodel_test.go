package models_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
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

// Spec §3.27 — the four fields, in declaration order.
func TestFieldOrder(t *testing.T) {
	want := []string{"cv", "design", "locale", "settings"}
	got := models.FieldNames()
	if len(got) != len(want) {
		t.Fatalf("field names = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("field names = %v, want %v", got, want)
		}
	}
}

// Spec §3.28 — an empty document validates and every field falls back to its default.
func TestEmptyDocumentValidates(t *testing.T) {
	model, errs := models.Validate(parse(t, "{}\n"), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if model.Cv != nil || model.Design != nil || model.Locale != nil || model.Settings != nil {
		t.Errorf("model = %+v, want every field absent", model)
	}
	if models.DefaultTheme != "classic" {
		t.Errorf("default theme = %q, want %q", models.DefaultTheme, "classic")
	}
	if models.DefaultLanguage != "en" {
		t.Errorf("default language = %q, want %q", models.DefaultLanguage, "en")
	}
}

// Spec §3.29, §5.15 — unknown top-level keys are rejected, null-valued ones too.
func TestUnknownTopLevelKeyRejected(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "with a value", input: "cv:\n  name: John\nunknown: 1\n"},
		{name: "null-valued", input: "cv:\n  name: John\nunknown: null\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := models.Validate(parse(t, tc.input), nil, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != binder.CodeExtraForbidden {
				t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeExtraForbidden)
			}
			if len(errs[0].SchemaLocation) != 1 || errs[0].SchemaLocation[0] != "unknown" {
				t.Errorf("schema location = %v, want [unknown]", errs[0].SchemaLocation)
			}
		})
	}
}

// Spec §3.30 — `cv` is deliberately absent from the JSON-schema required list.
func TestJSONSchemaRequiredMarker(t *testing.T) {
	if len(models.JSONSchemaRequired) != 0 {
		t.Errorf("JSONSchemaRequired = %v, want empty", models.JSONSchemaRequired)
	}
}

// Spec §3.31 — the input file path is recorded out-of-band when the context supplies one.
func TestInputFilePathRecording(t *testing.T) {
	tests := []struct {
		name string
		ctx  *models.ValidationContext
		want string
	}{
		{name: "no context", ctx: nil},
		{name: "empty context", ctx: &models.ValidationContext{}},
		{name: "path supplied", ctx: &models.ValidationContext{InputFilePath: "/tmp/CV.yaml"}, want: "/tmp/CV.yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model, _ := models.Validate(parse(t, "{}\n"), tc.ctx, schemaerr.SourceMain)
			got, ok := model.InputFilePath()
			if tc.want == "" {
				if ok {
					t.Errorf("input file path = %q, want absent", got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Errorf("input file path = %q (present %v), want %q", got, ok, tc.want)
			}
		})
	}
}

// The four known keys bind to their document nodes.
func TestKnownKeysBind(t *testing.T) {
	src := "cv:\n  name: John\ndesign:\n  theme: sb2nov\nlocale:\n  language: en\nsettings:\n  bold_keywords: []\n"
	model, errs := models.Validate(parse(t, src), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	for name, node := range map[string]*yamldoc.Node{
		"cv":       model.Cv,
		"design":   model.Design,
		"locale":   model.Locale,
		"settings": model.Settings,
	} {
		if node == nil {
			t.Errorf("%s = nil, want a bound node", name)
		}
	}
}
