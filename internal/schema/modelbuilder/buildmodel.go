package modelbuilder

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// BuildModel mirrors build_rendercv_model_from_commented_map
// (rendercv_model_builder.py:175-189): validate the merged document, then turn
// whatever failed into user-facing records.
//
// **This is the only place `errorpipeline.Parse` is called**, and that is a
// property worth keeping rather than a coincidence. Parse is not idempotent —
// it applies the message dictionary and appends the trailing period — so a
// second caller would double-substitute some messages and add a second period
// to others. `TestParseHasOneCaller` enforces it.
//
// A validation failure is a `*schemaerr.UserValidationError` carrying every
// final record; an internal failure from the pipeline is returned as itself,
// because it means the port built a location the document cannot answer and is
// not something to show a user as a validation problem.
func BuildModel(
	result *BuildResult,
	ctx *valctx.ValidationContext,
) (*models.RenderCVModel, error) {
	model, raw := models.Validate(result.Document, ctx, schemaerr.SourceMain)
	if len(raw) == 0 {
		return model, nil
	}

	final, err := errorpipeline.Parse(raw, result.Document, result.OverlaySources)
	if err != nil {
		return nil, err
	}
	return nil, &schemaerr.UserValidationError{Errors: final}
}
