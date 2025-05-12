package binder_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §4.32 and §3.19 behavior 72. The `model_type` message names the
// target model, so it is not one constant: `cv: 5` says "or instance of Cv".
//
// Iteration 2 shipped the text without its suffix. Measured against the vendored
// Python for `cv: null` and `cv: 5`, which give the same message — the value's
// kind does not change it.
func TestModelTypeNamesTheModel(t *testing.T) {
	spec := binder.Spec{
		Fields: []binder.Field{{Name: "name"}},
		Policy: binder.ForbidExtra,
		Model:  "Cv",
	}

	for _, src := range []string{"5\n", "null\n", "just text\n", "[a]\n"} {
		t.Run(src, func(t *testing.T) {
			_, errs := binder.Bind(parse(t, "k: "+src).Items[0].Value, spec, nil, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			const want = "Input should be a valid dictionary or instance of Cv"
			if errs[0].Message != want {
				t.Errorf("message = %q, want %q", errs[0].Message, want)
			}
			if errs[0].Code != "model_type" {
				t.Errorf("code = %q, want model_type", errs[0].Code)
			}
		})
	}
}

// A Spec with no model name omits the suffix. No upstream model does that — it
// is the zero value, and this pins what it produces rather than leaving it to
// whoever first forgets to set the field.
func TestModelTypeWithoutAModelName(t *testing.T) {
	spec := binder.Spec{Fields: []binder.Field{{Name: "name"}}, Policy: binder.ForbidExtra}

	_, errs := binder.Bind(parse(t, "k: 5\n").Items[0].Value, spec, nil, schemaerr.SourceMain)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Message != "Input should be a valid dictionary" {
		t.Errorf("message = %q", errs[0].Message)
	}
}
