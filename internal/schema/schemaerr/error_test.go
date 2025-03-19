package schemaerr_test

import (
	"errors"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

func TestYamlSourceLiterals(t *testing.T) {
	tests := []struct {
		name   string
		source schemaerr.YamlSource
		want   string
	}{
		{"main", schemaerr.SourceMain, "main_yaml_file"},
		{"design", schemaerr.SourceDesign, "design_yaml_file"},
		{"locale", schemaerr.SourceLocale, "locale_yaml_file"},
		{"settings", schemaerr.SourceSettings, "settings_yaml_file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.source) != tt.want {
				t.Errorf("YamlSource = %q, want %q", tt.source, tt.want)
			}
		})
	}
}

func TestOverlayToSource(t *testing.T) {
	if len(schemaerr.OverlayToSource) != 3 {
		t.Errorf("OverlayToSource has %d entries, want 3", len(schemaerr.OverlayToSource))
	}
	if schemaerr.OverlayToSource[schemaerr.OverlayDesign] != schemaerr.SourceDesign {
		t.Errorf("design -> %q, want %q", schemaerr.OverlayToSource[schemaerr.OverlayDesign], schemaerr.SourceDesign)
	}
	if schemaerr.OverlayToSource[schemaerr.OverlayLocale] != schemaerr.SourceLocale {
		t.Errorf("locale -> %q, want %q", schemaerr.OverlayToSource[schemaerr.OverlayLocale], schemaerr.SourceLocale)
	}
	if schemaerr.OverlayToSource[schemaerr.OverlaySettings] != schemaerr.SourceSettings {
		t.Errorf("settings -> %q, want %q", schemaerr.OverlayToSource[schemaerr.OverlaySettings], schemaerr.SourceSettings)
	}
}

func TestErrorTypes(t *testing.T) {
	ve := &schemaerr.ValidationError{Message: "validation error"}
	ue := &schemaerr.UserError{Message: "user error"}
	uve := &schemaerr.UserValidationError{Errors: []schemaerr.ValidationError{{Message: "user validation error"}}}
	ie := &schemaerr.InternalError{Message: "internal error"}

	if ve.Error() != "validation error" {
		t.Errorf("ValidationError.Error() = %q", ve.Error())
	}
	if ue.Error() != "user error" {
		t.Errorf("UserError.Error() = %q", ue.Error())
	}
	if uve.Error() != "user validation error" {
		t.Errorf("UserValidationError.Error() = %q", uve.Error())
	}
	if ie.Error() != "internal error" {
		t.Errorf("InternalError.Error() = %q", ie.Error())
	}
}

func TestErrorsAs(t *testing.T) {
	var ve *schemaerr.ValidationError
	var ue *schemaerr.UserError
	var uve *schemaerr.UserValidationError
	var ie *schemaerr.InternalError

	if !errors.As(&schemaerr.ValidationError{Message: "x"}, &ve) {
		t.Error("ValidationError not matched by errors.As")
	}
	if !errors.As(&schemaerr.UserError{Message: "x"}, &ue) {
		t.Error("UserError not matched by errors.As")
	}
	if !errors.As(&schemaerr.UserValidationError{}, &uve) {
		t.Error("UserValidationError not matched by errors.As")
	}
	if !errors.As(&schemaerr.InternalError{Message: "x"}, &ie) {
		t.Error("InternalError not matched by errors.As")
	}
}

func TestUserValidationErrorEmpty(t *testing.T) {
	uve := &schemaerr.UserValidationError{}
	if uve.Error() != "validation error" {
		t.Errorf("empty UserValidationError.Error() = %q, want 'validation error'", uve.Error())
	}
}
