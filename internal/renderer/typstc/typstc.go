// Package typstc compiles Typst documents to PDF and PNG.
//
// It replaces upstream's `typst` Python bindings
// (`third_party/rendercv/src/rendercv/renderer/pdf_png.py`) with the same
// compiler built for `wasm32-wasip1` and executed on wazero — no CGO, no
// external binary, no network. `specs/divergences.md` D-006 and D-007.
//
// The compiler sees these things, mirroring `get_typst_compiler`
// (`pdf_png.py:154-186`):
//
//	/work    the input document's own directory, read-write
//	/out     the output's directory, read-write, mounted only when it is not
//	         already under /work — upstream has the whole filesystem and
//	         writes the PDF/PNG wherever settings.render_command points
//	/fonts   the vendored font tree, plus any font folder the caller adds
//	/pkg     the Typst package cache, laid out preview/<name>/<version>/
package typstc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"

	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc/assets"
)

// DefaultPPI is the resolution upstream gets for PNGs by passing nothing to
// `typst.compile`, which is `typst-py`'s own default (`typst/__init__.pyi:119`).
const DefaultPPI = 144

// Format selects the compiler's output.
type Format string

const (
	// FormatPDF writes a single PDF at the output path.
	FormatPDF Format = "pdf"
	// FormatPNG writes one file per page, named `<output>_<n>.png`, one-based
	// (`pdf_png.py:86-89`). The output path is a prefix, not a file name.
	FormatPNG Format = "png"
)

// Request is one compilation.
type Request struct {
	// InputPath is the .typ file, on the host. Its directory becomes the
	// compiler's root, which is what makes a photo referenced by base name
	// resolve (`pdf_png.py:93-111`).
	InputPath string

	// OutputPath is the PDF file to write, or for FormatPNG the prefix each
	// page name is built from. Its directory must already exist.
	OutputPath string

	// Format defaults to FormatPDF.
	Format Format

	// PPI is the PNG resolution. Zero means DefaultPPI. Ignored for PDF.
	PPI int

	// FontDirs are extra host font folders, searched *after* the vendored set,
	// which is the order upstream passes them in (`pdf_png.py:174-185`). A
	// folder that does not exist is skipped, as upstream's does.
	FontDirs []string

	// Today pins `World::today`. The zero value means the system date, which
	// is what upstream uses; the conformance suite passes a fixed date.
	Today time.Time
}

// Result reports what the compiler produced.
type Result struct {
	// Pages is the page count the document laid out.
	Pages int
}

// Error is a compilation failure carrying the compiler's own diagnostics.
//
// Typst's messages are user-facing in upstream too — `pdf_png.py` lets them
// propagate into the CLI's error panel — so they are kept verbatim rather than
// wrapped into Go phrasing.
type Error struct {
	// Diagnostics is what the compiler wrote to stderr, trimmed.
	Diagnostics string
	// ExitCode is the shim's exit status.
	ExitCode int
}

func (e *Error) Error() string {
	if e.Diagnostics == "" {
		return fmt.Sprintf("typst exited %d", e.ExitCode)
	}
	return e.Diagnostics
}

// Compile runs the embedded Typst compiler over one document.
func Compile(ctx context.Context, req Request) (Result, error) {
	if req.Format == "" {
		req.Format = FormatPDF
	}
	if req.PPI == 0 {
		req.PPI = DefaultPPI
	}

	input, err := filepath.Abs(req.InputPath)
	if err != nil {
		return Result{}, fmt.Errorf("typstc: resolving the input path: %w", err)
	}
	output, err := filepath.Abs(req.OutputPath)
	if err != nil {
		return Result{}, fmt.Errorf("typstc: resolving the output path: %w", err)
	}

	// Upstream has the whole filesystem; this port mounts only the two
	// directories the request actually names. The input's directory is always
	// `/work`, matching a photo referenced by base name (`pdf_png.py:93-111`).
	// The output may sit elsewhere — `settings.render_command.pdf_path` and the
	// CLI's `-pdf`/`-png` flags both allow a path outside the input's
	// directory, and upstream renders it there — so a second directory is
	// mounted at `/out` only when the output does not already live under
	// `/work`. The caller (`generate.PDF`/`generate.PNG`) has already created
	// that directory, mirroring `path_resolver.py:109`'s `mkdir`.
	root := filepath.Dir(input)
	guestIn, err := underRoot(root, workMount, input)
	if err != nil {
		return Result{}, err
	}
	outRoot := root
	outMount := workMount
	guestOut, err := underRoot(root, workMount, output)
	if err != nil {
		outRoot = filepath.Dir(output)
		outMount = outAltMount
		guestOut, err = underRoot(outRoot, outMount, output)
		if err != nil {
			return Result{}, err
		}
	}

	argv := []string{
		"typst",
		"--root", workMount,
		"--pkg", pkgMount,
		"--in", guestIn,
		"--out", guestOut,
		"--format", string(req.Format),
		"--ppi", strconv.Itoa(req.PPI),
	}
	// **The vendored set goes first**, then the caller's folders — the order
	// upstream passes to `typst.Compiler`, which lists
	// `rendercv_fonts.paths_to_font_folders` ahead of the input file's own
	// `fonts/` directory (`pdf_png.py:174-185`). The order decides which face
	// wins a family-name tie, so reversing it is a silent metrics divergence.
	argv = append(argv, "--font-dir", fontsMount)
	fontMounts := make(map[string]fs.FS)
	for i, dir := range req.FontDirs {
		absolute, err := filepath.Abs(dir)
		if err != nil {
			return Result{}, fmt.Errorf("typstc: resolving a font folder: %w", err)
		}
		// A missing `fonts/` folder is normal: upstream passes the path
		// whether or not it exists, and typst ignores what it cannot read.
		if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
			continue
		}
		mount := fmt.Sprintf("/fonts-%d", i)
		fontMounts[mount] = os.DirFS(absolute)
		argv = append(argv, "--font-dir", mount)
	}
	if !req.Today.IsZero() {
		argv = append(argv, "--today", req.Today.Format("2006-01-02"))
	}

	dirMounts := map[string]string{workMount: root}
	if outMount != workMount {
		dirMounts[outMount] = outRoot
	}

	var stdout, stderr strings.Builder
	code, err := run(ctx, argv, dirMounts, fontMounts, &stdout, &stderr)
	if err != nil {
		return Result{}, err
	}
	if code != 0 {
		return Result{}, &Error{
			Diagnostics: strings.TrimRight(stderr.String(), "\n"),
			ExitCode:    int(code),
		}
	}

	return Result{Pages: parsePages(stdout.String())}, nil
}

const (
	workMount   = "/work"
	outAltMount = "/out"
	pkgMount    = "/pkg"
	fontsMount  = "/fonts"
)

// run instantiates the compiler and executes it once.
func run(
	ctx context.Context,
	argv []string,
	dirMounts map[string]string,
	fontMounts map[string]fs.FS,
	stdout, stderr io.Writer,
) (uint32, error) {
	engine, err := shared()
	if err != nil {
		return 0, err
	}

	// Read-write, because the compiler writes the PDF and the PNGs itself.
	fsConfig := wazero.NewFSConfig().
		WithFSMount(assets.Packages(), pkgMount).
		WithFSMount(assets.Fonts(), fontsMount)
	for mount, dir := range dirMounts {
		fsConfig = fsConfig.WithDirMount(dir, mount)
	}
	for mount, tree := range fontMounts {
		fsConfig = fsConfig.WithFSMount(tree, mount)
	}

	config := wazero.NewModuleConfig().
		WithArgs(argv...).
		WithStdout(stdout).
		WithStderr(stderr).
		WithFSConfig(fsConfig).
		// The module is instantiated once per call and its name would collide.
		WithName("")

	module, err := engine.runtime.InstantiateModule(ctx, engine.module, config)
	if module != nil {
		// Closing frees the instance's memory. Its error cannot inform the
		// caller — the compile has already produced its exit code and its
		// stderr by this point — but it must be acknowledged rather than
		// dropped silently.
		defer func() { _ = module.Close(ctx) }()
	}
	if err == nil {
		return 0, nil
	}

	var exit *sys.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, fmt.Errorf("typstc: running the compiler: %w", err)
}

// engine is the compiled module and the runtime that owns it.
type engine struct {
	runtime wazero.Runtime
	module  wazero.CompiledModule
}

// shared compiles the 29 MB module once per process.
//
// Compiling it is the expensive part — seconds — and a render that writes both
// a PDF and its PNGs would otherwise pay it twice.
var shared = sync.OnceValues(func() (engine, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return engine{}, fmt.Errorf("typstc: instantiating WASI: %w", err)
	}
	module, err := runtime.CompileModule(ctx, assets.Typst)
	if err != nil {
		return engine{}, fmt.Errorf("typstc: compiling the embedded typst module: %w", err)
	}
	return engine{runtime: runtime, module: module}, nil
})

// underRoot maps a host path to its guest path under the given mount, or
// fails if path does not live under root.
func underRoot(root, mount, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("typstc: %q is outside %q", path, root)
	}
	return mount + "/" + filepath.ToSlash(rel), nil
}

// parsePages reads the shim's `pages <n>` line. A malformed line reports zero
// rather than failing the compilation: the artifact is already written, and the
// count is only used for reporting.
func parsePages(stdout string) int {
	for line := range strings.Lines(stdout) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "pages ")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
			return n
		}
	}
	return 0
}
