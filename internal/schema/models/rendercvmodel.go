package models

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Defaults of spec §3.28 (rendercv_model.py:19, :24, :31, :38): an empty cv,
// the classic theme, the English locale, and default settings. Only the theme
// and language are namable this iteration; the design, locale and settings
// models themselves belong to iterations 6 and 7.
const (
	DefaultTheme    = "classic"
	DefaultLanguage = "en"
)

// JSONSchemaRequired is the marker of spec §3.30: `cv` is deliberately omitted
// from the JSON-schema `required` list even though it is semantically required,
// so the same schema validates standalone design, locale and settings files
// (rendercv_model.py:15-18). Emitting it is iteration 5's job; the model only
// carries it.
var JSONSchemaRequired = []string{}

// fieldOrder is the declaration order of spec §3.27
// (rendercv_model.py:19-42). It is contractual: JSON-schema property order and
// error order both follow it.
var fieldOrder = []binder.Field{
	{Name: "cv"},
	{Name: "design"},
	{Name: "locale"},
	{Name: "settings"},
}

// RenderCVModel mirrors RenderCVModel (schema/models/rendercv_model.py:14-62).
//
// All four fields hold raw document nodes this iteration. A nil field means the
// key was absent and the default of spec §3.28 applies. `cv` gains its typed
// model in iteration 2's later tasks; `design`, `locale` and `settings` gain
// theirs in iterations 6 and 7.
type RenderCVModel struct {
	Cv       *yamldoc.Node
	Design   *yamldoc.Node
	Locale   *yamldoc.Node
	Settings *yamldoc.Node

	// inputFilePath is recorded out-of-band after validation, not as a document
	// field, for later path resolution (spec §3.31, rendercv_model.py:44-62).
	inputFilePath string
}

// FieldNames returns the four field names in declaration order (spec §3.27).
func FieldNames() []string {
	names := make([]string, 0, len(fieldOrder))
	for _, field := range fieldOrder {
		names = append(names, field.Name)
	}
	return names
}

// InputFilePath reports the path recorded after validation, and whether one was
// recorded at all (spec §3.31).
func (m *RenderCVModel) InputFilePath() (string, bool) {
	if m == nil || m.inputFilePath == "" {
		return "", false
	}
	return m.inputFilePath, true
}

// Validate binds a merged document to the top-level model. Unknown keys are
// rejected (spec §3.29); every field has a default, so an empty mapping
// validates (spec §3.28).
func Validate(
	node *yamldoc.Node,
	ctx *ValidationContext,
	source schemaerr.YamlSource,
) (*RenderCVModel, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fieldOrder, Policy: binder.ForbidExtra},
		nil,
		source,
	)

	model := &RenderCVModel{}
	model.Cv, _ = result.Value("cv")
	model.Design, _ = result.Value("design")
	model.Locale, _ = result.Value("locale")
	model.Settings, _ = result.Value("settings")

	// Spec §3.31: the input file path comes from the context, after validation,
	// and is held out-of-band.
	if path, ok := ctx.InputPath(); ok {
		model.inputFilePath = path
	}

	return model, errs
}
