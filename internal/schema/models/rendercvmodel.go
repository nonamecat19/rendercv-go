package models

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
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

	// CvModel is the bound `cv`, and `cv.Default()` when the key is absent —
	// spec §3.28's "an empty `cv`" (rendercv_model.py:19-23). A validated model
	// therefore always has one, matching upstream, where `rendercv_model.cv` is
	// never `None`. The raw node stays beside it because later iterations read
	// spans and unnormalized text from it (spec 003 §6.5), and *that* node is
	// still nil when the key is absent — it is the record of what the document
	// said, not of what the model defaulted to.
	CvModel *cv.Cv

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
	ctx *valctx.ValidationContext,
	source schemaerr.YamlSource,
) (*RenderCVModel, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fieldOrder, Policy: binder.ForbidExtra, Model: "RenderCVModel"},
		nil,
		source,
	)

	model := &RenderCVModel{}
	model.Cv, _ = result.Value("cv")
	model.Design, _ = result.Value("design")
	model.Locale, _ = result.Value("locale")
	model.Settings, _ = result.Value("settings")

	// Spec 003 §3.19 behavior 44: `cv` is validated here, which iteration 2 could
	// not do — models/cv imported models for the context and path types, so the
	// call would have closed an import cycle. Those types moved to models/valctx
	// and models/inputpath (tasks 003 T1, T2), and the edge is now guarded by a
	// test in models/cv.
	//
	// An absent `cv` is not an error: every field has a default (spec §3.28), and
	// the JSON-schema `required` list deliberately omits `cv` (spec §3.30) so that
	// a standalone design, locale or settings file validates. A present-but-null
	// `cv` is a non-mapping, and the binder's own model-type branch reports it.
	//
	// **An absent `cv` binds the default model, not nil** — spec §3.28's "an
	// empty `cv`", which is `default_factory=Cv` (rendercv_model.py:19-23). The
	// model used to be left nil here on the theory that a caller wants to tell
	// "not supplied" from "supplied and empty"; no caller ever wanted that, and
	// upstream cannot express it, because `rendercv_model.cv` is never `None`.
	// What the nil actually bought was a panic: `locale: {language: english}`
	// with no `cv:` key dereferenced nil in the renderer's bridge and exited 2,
	// where upstream writes `NAME_IN_SNAKE_CASE_CV.typ` and exits 0.
	if model.Cv == nil {
		model.CvModel = cv.Default()
	} else {
		cvModel, cvErrs := cv.Validate(
			model.Cv,
			[]string{"cv"},
			source,
			cv.Options{Registry: entries.Default(), Context: ctx},
		)
		model.CvModel = cvModel
		errs = append(errs, cvErrs...)
	}

	// `design` and `locale` in full; `settings` is still the thin slice spec 004
	// §7.9 pulled forward, and the rest of it is iteration 12's.
	errs = append(errs, design.Validate(model.Design, []string{"design"}, source, ctx)...)
	// Iteration 7 landed the catalog model; this is the edge that makes it
	// reachable. Without it the extra-key and month-length rules exist and no
	// document can reach them.
	errs = append(errs, locale.Validate(model.Locale, []string{"locale"}, source)...)
	if model.Settings != nil {
		if current, ok := mappingValue(model.Settings, "current_date"); ok {
			errs = append(errs, settings.ValidateCurrentDate(current, []string{"settings"}, source)...)
		}
		// `bold_keywords` is `list[str]`, and a wrong-typed value for it used to
		// render at exit 0 where upstream refuses the document. It sits after
		// `current_date` because pydantic reports in declaration order
		// (settings.py:11-31).
		if keywords, ok := mappingValue(model.Settings, "bold_keywords"); ok {
			errs = append(errs,
				settings.ValidateBoldKeywords(keywords, []string{"settings"}, source)...)
		}
		// **Unknown keys under `settings` were accepted by nobody's decision**:
		// two specs each recorded the settings model as the other's work. An
		// audit measured `settings: {bogus: 1}` rendering at exit 0 where
		// upstream reports an unknown key.
		errs = append(errs,
			settings.ValidateUnknownKeys(model.Settings, []string{"settings"}, source)...)
	}

	// Spec §3.31: the input file path comes from the context, after validation,
	// and is held out-of-band.
	if path, ok := ctx.InputPath(); ok {
		model.inputFilePath = path
	}

	// Unknown keys last, after `cv` and everything under it has reported.
	return model, append(errs, result.ExtraErrors...)
}

// mappingValue reads one key of a mapping node, reporting whether it was there.
func mappingValue(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil, false
	}
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}
