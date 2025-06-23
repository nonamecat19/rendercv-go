package templater

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/flosch/pongo2/v6"
)

// filtersOnce guards the global filter registry pongo2 uses. Upstream caches one
// environment per input path (`templater.py:17`); the port has no per-run state
// to cache, so what needs guarding is only the one-time registration.
var (
	filtersOnce sync.Once
	filtersErr  error
)

// Environment renders template fragments, mirroring `get_jinja2_environment`
// plus `render_single_template` (templater.py:17-47, :157-190).
//
// **`trim_blocks` and `lstrip_blocks` are not options here.** Upstream sets both
// (spec 008 §1 behavior 3) and pongo2 has no equivalent, so the transform of
// `tools/gentemplates` bakes their effect into the template source. That is
// `plan.md` §2's tradeoff, and the byte diff against goldens is what checks it —
// nothing in this file can.
type Environment struct {
	Loader Loader
	// Theme selects the first candidate path for a Typst fragment.
	Theme string
}

// NewEnvironment registers the filters once and returns an environment.
func NewEnvironment(inputDir string, builtin fs.FS, theme string) (*Environment, error) {
	filtersOnce.Do(func() { filtersErr = registerFilters() })
	if filtersErr != nil {
		return nil, fmt.Errorf("registering filters: %w", filtersErr)
	}
	return &Environment{
		Loader: Loader{InputDir: inputDir, Builtin: builtin},
		Theme:  theme,
	}, nil
}

// Render is `render_single_template` (`:157-190`).
//
// The four context names are always present — `cv`, `design`, `locale`,
// `settings` — and `extra` is the per-call keyword arguments the callers add:
// `section_title`, `snake_case_section_title`, `entry_type`, `entry`,
// `html_body`.
func (e *Environment) Render(
	format Format,
	name string,
	context pongo2.Context,
	extra pongo2.Context,
) (string, error) {
	source, err := e.Loader.Load(format, e.Theme, name)
	if err != nil {
		return "", err
	}

	template, err := pongo2.FromString(source)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", name, err)
	}

	merged := make(pongo2.Context, len(context)+len(extra))
	for key, value := range context {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}

	out, err := template.Execute(merged)
	if err != nil {
		return "", fmt.Errorf("rendering %s: %w", name, err)
	}
	return out, nil
}
