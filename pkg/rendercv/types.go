package rendercv

import (
	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// This file is the only place an internal package appears in a declaration.
// Everything below is an alias, so a caller writes rendercv.UserError and never
// names an internal path, while errors.As keeps matching errors raised deep
// inside the pipeline — which a parallel set of types would silently break.
// api_test.go enforces that no other exported signature mentions internal.

// Document is the parsed YAML tree, mirroring the ruamel CommentedMap upstream
// passes around (schema/rendercv_model_builder.py:65-84).
type Document = yamldoc.Node

// YamlSource names which of the four input files a value or an error came from,
// mirroring upstream's YamlSource literal (exception.py:5-10).
type YamlSource = schemaerr.YamlSource

// The four sources upstream's YamlSource literal admits (exception.py:5-10).
const (
	SourceMain     = schemaerr.SourceMain
	SourceDesign   = schemaerr.SourceDesign
	SourceLocale   = schemaerr.SourceLocale
	SourceSettings = schemaerr.SourceSettings
)

// UserError is a user-facing failure with no document location, mirroring
// RenderCVUserError (exception.py:22-24).
type UserError = schemaerr.UserError

// UserValidationError carries every validation failure of one run, in the order
// they were produced, mirroring RenderCVUserValidationError
// (exception.py:27-29).
type UserValidationError = schemaerr.UserValidationError

// InternalError is a failure that should never reach a user, mirroring
// RenderCVInternalError (exception.py:32-34).
//
// Upstream derives it from RuntimeError rather than ValueError, and the
// distinction is the point: a UserError means the input was wrong, an
// InternalError means this program is broken.
type InternalError = schemaerr.InternalError

// ValidationError is one validation record, mirroring the RenderCVValidationError
// dataclass (exception.py:20-25). It is a payload rather than an exception
// upstream too, carried inside a UserValidationError.
type ValidationError = schemaerr.ValidationError

// Model is a validated, resolved document, ready to render.
//
// It mirrors RenderCVModel (schema/models/rendercv_model.py:14-61), but it is
// not that type: upstream keeps the render paths and the input file path inside
// the model, while this port resolves a model into a render-ready document and
// keeps the path context beside it. Model holds both.
//
// Obtain one from [Build]. The type has no exported fields and no exported
// constructor — see the package doc for why.
type Model struct {
	doc     bridge.Document
	options generateOptions
}

// Name is the CV's name as the document spelled it, or "" when the document
// has none — `cv.name` (schema/models/cv/cv.py:31). It is exposed because a caller that renders several documents needs
// something to tell them apart, and every other field is machinery.
func (m *Model) Name() string {
	if m == nil || m.doc.Model == nil || m.doc.Model.CvModel == nil {
		return ""
	}
	name := m.doc.Model.CvModel.Name
	if name == nil || name.Kind != yamldoc.KindString {
		return ""
	}
	return name.Raw
}

// built reports whether this Model came from [Build].
//
// **The zero value is reachable** — `var m rendercv.Model` compiles outside this
// package even though every field is unexported — and it carries no resolved
// document, so rendering it dereferences nothing and panics. Plan 016 §1.3
// claimed unexported fields made a hand-built model impossible; they make it
// unusable, which is not the same thing, so the generators check rather than
// assume.
func (m *Model) built() bool {
	return m != nil && m.doc.Model != nil
}
