// Package rendercv is the public Go API of rendercv-go: a library surface for
// programs that want to validate and render a RenderCV document without
// shelling out to the binary.
//
// # What it mirrors
//
// Upstream RenderCV declares no public Python API. Every __init__.py in its
// tree is empty, there is no __all__ and no py.typed marker, and its docs
// generate a reference page for every module equally
// (src/rendercv/__init__.py:1-8, docs/api_reference/api_reference.py:13-29).
// Its de-facto API is whatever a caller imports from internal module paths,
// which is what its own CLI does.
//
// This package therefore mirrors the seven functions upstream's orchestrator
// calls (src/rendercv/cli/render_command/run_rendercv.py:127-198): ReadYAML and
// Build from the schema half, and the five generators. Every exported symbol
// here names the upstream construct it mirrors in its doc comment.
//
// It deliberately does not mirror run_rendercv itself, which takes a
// ProgressPanel — a Rich console object — and so cannot be called headlessly.
//
// # Obtaining a Model
//
// A [Model] comes from [Build] and from nowhere else. There is no exported
// constructor and the type has no exported fields, which is deliberate: a model
// carries resolved state that only validation can produce, including the input
// file path that PDF and PNG generation resolve against. Upstream keeps that
// state in a pydantic private attribute and then reads it from other modules
// anyway (rendercv_model.py:44-62, read at pdf_png.py:36 and :73), so a
// hand-built model is a hazard there. Here it is simply not expressible.
//
// # Not generated is not an error
//
// Each generator returns an empty path and a nil error when its corresponding
// settings.render_command.dont_generate_* flag is set. That is a successful
// outcome, mirroring upstream's None return, and it is distinct from both a
// written path and a failure. [GeneratePNG] returns a nil slice in that case.
//
// # Stability
//
// Frozen as of v1.0.0. This package follows semantic versioning: a breaking
// change to any exported symbol's signature or behavior requires a new major
// version. See CHANGELOG.md at the module root for the version history.
package rendercv
