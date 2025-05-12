package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.19 behavior 72: every model that can appear as a mapping value
// carries its own name. Measured for each.
func TestModelTypeSuffixPerModel(t *testing.T) {
	t.Run("Cv", func(t *testing.T) {
		_, errs := cv.Validate(parse(t, "k: 5\n").Items[0].Value, []string{"cv"}, schemaerr.SourceMain, testOptions())
		assertModelType(t, errs, "Cv")
	})

	t.Run("SocialNetwork", func(t *testing.T) {
		_, errs := cv.ValidateSocialNetwork(
			parse(t, "k: 5\n").Items[0].Value,
			[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
		)
		assertModelType(t, errs, "SocialNetwork")
	})

	t.Run("CustomConnection", func(t *testing.T) {
		_, errs := cv.ValidateCustomConnection(
			parse(t, "k: 5\n").Items[0].Value,
			[]string{"cv", "custom_connections", "0"}, schemaerr.SourceMain,
		)
		assertModelType(t, errs, "CustomConnection")
	})

	// Every one of the eight entry types, so a new one cannot arrive nameless.
	for _, descriptor := range entries.Default().Descriptors() {
		t.Run(string(descriptor.Name), func(t *testing.T) {
			errs, err := entries.Validate(
				parse(t, "k: 5\n").Items[0].Value, descriptor.Name, nil,
				schemaerr.SourceMain, sectionReference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			assertModelType(t, errs, string(descriptor.Name))
		})
	}
}

func assertModelType(t *testing.T, errs []schemaerr.ValidationError, model string) {
	t.Helper()
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	want := "Input should be a valid dictionary or instance of " + model
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
}
