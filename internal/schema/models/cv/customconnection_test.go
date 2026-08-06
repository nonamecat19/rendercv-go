package cv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec §3.81 — field order and the extra-key policy.
func TestCustomConnectionFieldOrder(t *testing.T) {
	want := []string{"fontawesome_icon", "placeholder", "url"}
	got := cv.CustomConnectionFieldNames()
	if len(got) != len(want) {
		t.Fatalf("field names = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("field names = %v, want %v", got, want)
		}
	}
}

// Spec §3.81 — fontawesome_icon and placeholder are required text; url is
// required-but-nullable: the key must be present, but its value may be null.
func TestCustomConnectionRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErrs  int
		wantCodes []schemaerr.Code
	}{
		{
			name:     "all present, url a value",
			input:    "fontawesome_icon: fa-icon\nplaceholder: p\nurl: https://example.com\n",
			wantErrs: 0,
		},
		{
			name:     "url present but null is accepted",
			input:    "fontawesome_icon: fa-icon\nplaceholder: p\nurl: null\n",
			wantErrs: 0,
		},
		{
			name:      "fontawesome_icon missing",
			input:     "placeholder: p\nurl: null\n",
			wantErrs:  1,
			wantCodes: []schemaerr.Code{binder.CodeMissing},
		},
		{
			name:      "placeholder missing",
			input:     "fontawesome_icon: fa-icon\nurl: null\n",
			wantErrs:  1,
			wantCodes: []schemaerr.Code{binder.CodeMissing},
		},
		{
			name:      "url absent entirely is reported missing",
			input:     "fontawesome_icon: fa-icon\nplaceholder: p\n",
			wantErrs:  1,
			wantCodes: []schemaerr.Code{binder.CodeMissing},
		},
		{
			name:      "unknown key is rejected",
			input:     "fontawesome_icon: fa-icon\nplaceholder: p\nurl: null\nextra: nope\n",
			wantErrs:  1,
			wantCodes: []schemaerr.Code{binder.CodeExtraForbidden},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := cv.ValidateCustomConnection(
				parse(t, tc.input), []string{"cv", "custom_connections", "0"}, schemaerr.SourceMain,
			)
			if len(errs) != tc.wantErrs {
				t.Fatalf("errs = %+v, want %d error(s)", errs, tc.wantErrs)
			}
			for i, code := range tc.wantCodes {
				if errs[i].Code != code {
					t.Errorf("errs[%d].Code = %q, want %q", i, errs[i].Code, code)
				}
			}
		})
	}
}

func TestCustomConnectionUrlPresentButNull(t *testing.T) {
	model, errs := cv.ValidateCustomConnection(
		parse(t, "fontawesome_icon: fa\nplaceholder: p\nurl: null\n"),
		[]string{"cv", "custom_connections", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if model.Url == nil {
		t.Fatal("Url = nil, want a bound null node")
	}
}

// Spec §3.46 — `photo` tries the file-path interpretation before the URL
// interpretation: supplying a value valid as both must resolve to the path.
func TestResolvePhotoTriesPathBeforeURL(t *testing.T) {
	dir := t.TempDir()
	photoPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(photoPath, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	inputFile := filepath.Join(dir, "input.yaml")

	ctx := &models.ValidationContext{InputFilePath: inputFile}

	photo := cv.ResolvePhoto("photo.jpg", ctx)
	if photo.Kind != cv.PhotoKindPath {
		t.Fatalf("Kind = %v, want PhotoKindPath (an existing path is also a valid placeholder URL string)", photo.Kind)
	}
	if photo.Path.Value != photoPath {
		t.Errorf("Path.Value = %q, want %q", photo.Path.Value, photoPath)
	}
}

func TestResolvePhotoFallsBackToURL(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.yaml")
	ctx := &models.ValidationContext{InputFilePath: inputFile}

	photo := cv.ResolvePhoto("https://example.com/photo.jpg", ctx)
	if photo.Kind != cv.PhotoKindURL {
		t.Fatalf("Kind = %v, want PhotoKindURL", photo.Kind)
	}
	if photo.URL != "https://example.com/photo.jpg" {
		t.Errorf("URL = %q, want the raw value", photo.URL)
	}
}
