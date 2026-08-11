package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli/sample"
	"github.com/nonamecat19/rendercv-go/internal/conformance"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc/assets"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

// The packaging inventory of spec 013 §3.8 — the shape of what the binary
// ships, as opposed to what it does with it. Every number below was measured
// against the vendored tree at `third_party/rendercv/src/rendercv/` and agrees
// with the spec's behaviors 54 and 56.
//
// These are the assertions nothing else in the suite makes: a tenth theme, a
// dropped locale, a fourth command or a Typst package version that no longer
// matches the one the preamble imports are all silent until something counts.

// TestRegisteredCommandsAreExactlyThree is behavior 56.
//
// Upstream registers by auto-import — `app.py:142-152` globs `*_command.py` —
// and its own test asserts the registered count equals the file count
// (`tests/cli/test_app.py:24-32`). The port names its three explicitly, so the
// equivalent check is over the source: every `AddCommand` call in the package,
// resolved back to the `cobra.Command` literal's `Use` field. Axis 2 forbids a
// fourth, and a fourth added without a spec would otherwise pass every test.
func TestRegisteredCommandsAreExactlyThree(t *testing.T) {
	want := []string{"render", "new", "create-theme"}

	got := registeredCommandNames(t)
	if !slices.Equal(got, want) {
		t.Errorf("registered commands = %q, want %q", got, want)
	}
}

// registeredCommandNames reads the package's own source and returns the `Use`
// verb of every command passed to `AddCommand`, in registration order.
func registeredCommandNames(t *testing.T) []string {
	t.Helper()

	directory, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the cli package directory: %v", err)
	}

	// One walk over every non-test source file, in name order so the result is
	// deterministic: it records both `x := &cobra.Command{Use: "…"}` and the
	// identifiers `AddCommand` was handed, and resolves the second against the
	// first afterwards, because a declaration need not precede its use.
	fileSet := token.NewFileSet()
	uses := map[string]string{}
	var added []string
	for _, entry := range directory {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if assign, ok := node.(*ast.AssignStmt); ok {
				collectCommandLiterals(assign, uses)
			}
			if argument, ok := addCommandArgument(node); ok {
				added = append(added, argument)
			}
			return true
		})
	}

	names := make([]string, 0, len(added))
	for _, ident := range added {
		use, ok := uses[ident]
		if !ok {
			t.Fatalf("AddCommand(%s): no cobra.Command literal for that identifier", ident)
		}
		// `Use` is `render [input]`; the command's name is its first word.
		names = append(names, strings.Fields(use)[0])
	}
	return names
}

// collectCommandLiterals records the `Use` string of every `cobra.Command`
// composite literal bound to a plain identifier by assign.
func collectCommandLiterals(assign *ast.AssignStmt, into map[string]string) {
	for i, rhs := range assign.Rhs {
		if i >= len(assign.Lhs) {
			return
		}
		target, ok := assign.Lhs[i].(*ast.Ident)
		if !ok {
			continue
		}
		if unary, ok := rhs.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			rhs = unary.X
		}
		literal, ok := rhs.(*ast.CompositeLit)
		if !ok || !isCobraCommand(literal.Type) {
			continue
		}
		if use, ok := literalString(literal, "Use"); ok {
			into[target.Name] = use
		}
	}
}

func isCobraCommand(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Command" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "cobra"
}

// literalString returns the value of one string-valued field of a composite
// literal.
func literalString(literal *ast.CompositeLit, field string) (string, bool) {
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		value, ok := pair.Value.(*ast.BasicLit)
		if !ok || value.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(value.Value)
		if err != nil {
			return "", false
		}
		return unquoted, true
	}
	return "", false
}

// addCommandArgument returns the identifier a `…AddCommand(x)` call was given.
func addCommandArgument(node ast.Node) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "AddCommand" {
		return "", false
	}
	argument, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return "", false
	}
	return argument.Name, true
}

// TestEmbeddedTemplateInventory is behavior 54's template rows: 13 Typst
// templates (four top-level fragments plus the nine entry templates), 12
// Markdown (three plus nine — Markdown has no `Preamble`) and one HTML.
//
// The port's tree is the pongo2 transform of upstream's (D-005), so the bytes
// differ by design and the file *set* does not.
func TestEmbeddedTemplateInventory(t *testing.T) {
	entries := []string{
		"BulletEntry", "EducationEntry", "ExperienceEntry", "NormalEntry",
		"NumberedEntry", "OneLineEntry", "PublicationEntry",
		"ReversedNumberedEntry", "TextEntry",
	}

	cases := []struct {
		kind  string
		count int
		files []string
	}{
		{
			kind:  "typst",
			count: 13,
			files: append([]string{
				"Header.j2.typ", "Preamble.j2.typ",
				"SectionBeginning.j2.typ", "SectionEnding.j2.typ",
			}, suffixed(entries, "entries/", ".j2.typ")...),
		},
		{
			kind:  "markdown",
			count: 12,
			files: append([]string{
				"Header.j2.md", "SectionBeginning.j2.md", "SectionEnding.j2.md",
			}, suffixed(entries, "entries/", ".j2.md")...),
		},
		{kind: "html", count: 1, files: []string{"Full.html"}},
	}

	root := templater.BuiltinTemplates()
	for _, testCase := range cases {
		t.Run(testCase.kind, func(t *testing.T) {
			got := templateFiles(t, root, testCase.kind)
			if len(got) != testCase.count {
				t.Errorf("%s templates = %d, want %d", testCase.kind, len(got), testCase.count)
			}
			want := slices.Clone(testCase.files)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s templates =\n%q\nwant\n%q", testCase.kind, got, want)
			}
		})
	}
}

// templateFiles lists one template set, sorted, relative to its kind folder.
func templateFiles(t *testing.T, root fs.FS, kind string) []string {
	t.Helper()

	var found []string
	err := fs.WalkDir(root, kind, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(kind, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the %s templates: %v", kind, err)
	}
	slices.Sort(found)
	return found
}

func suffixed(names []string, prefix, suffix string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = prefix + name + suffix
	}
	return out
}

// TestLocaleInventory is behavior 54's `other_locales/` row: 21 files, one per
// language, plus English, whose values are Python defaults rather than a YAML
// file and which therefore has no file upstream and no separate count here.
func TestLocaleInventory(t *testing.T) {
	// The stems of `schema/models/locale/other_locales/`, measured.
	want := []string{
		"arabic", "danish", "dutch", "french", "german", "hebrew", "hindi",
		"hungarian", "indonesian", "italian", "japanese", "korean",
		"mandarin_chinese", "norwegian_bokmål", "norwegian_nynorsk", "persian",
		"portuguese", "russian", "spanish", "turkish", "vietnamese",
	}

	available := locale.AvailableLocales()
	if available[0] != "english" {
		t.Errorf("AvailableLocales()[0] = %q, want %q", available[0], "english")
	}
	got := slices.Clone(available[1:])
	slices.Sort(got)
	if len(got) != 21 {
		t.Errorf("non-English locales = %d, want 21", len(got))
	}
	if !slices.Equal(got, want) {
		t.Errorf("non-English locales =\n%q\nwant\n%q", got, want)
	}
}

// TestThemeInventory is behavior 54's `other_themes/` row: 8 override files,
// plus `classic`, which is a hand-declared model upstream and so has no file.
func TestThemeInventory(t *testing.T) {
	// The stems of `schema/models/design/other_themes/`, measured.
	want := []string{
		"ember", "engineeringclassic", "engineeringresumes", "harvard", "ink",
		"moderncv", "opal", "sb2nov",
	}

	themes := design.Themes()
	if themes[0] != design.BaseTheme {
		t.Errorf("Themes()[0] = %q, want %q", themes[0], design.BaseTheme)
	}
	got := slices.Clone(themes[1:])
	slices.Sort(got)
	if len(got) != 8 {
		t.Errorf("non-classic themes = %d, want 8", len(got))
	}
	if !slices.Equal(got, want) {
		t.Errorf("non-classic themes =\n%q\nwant\n%q", got, want)
	}
}

// TestSampleContentIsShipped is behavior 54's `schema/sample_content.yaml` row.
// The port's equivalent is the block tree embedded under `cli/sample/blocks`,
// which is only observable through the generator, so the assertion is that the
// generator produces a document for the default theme and locale rather than an
// empty string.
//
// The other of the two `schema/` data files, `error_dictionary.yaml`, is
// asserted in its own package — see
// `internal/schema/errorpipeline/inventory_test.go`.
func TestSampleContentIsShipped(t *testing.T) {
	content, err := sample.Generate("John Doe", defaultTheme, defaultLocale)
	if err != nil {
		t.Fatalf("sample.Generate: %v", err)
	}
	for _, block := range []string{"cv:", "design:", "locale:", "settings:"} {
		if !strings.Contains(content, block) {
			t.Errorf("the generated sample is missing the %q block", block)
		}
	}
}

// typstPackageReference is the `@preview/<name>:<version>` the preamble imports.
var typstPackageReference = regexp.MustCompile(`@preview/([a-zA-Z0-9_-]+):([0-9]+\.[0-9]+\.[0-9]+)`)

// TestTypstPackageMetadata is behavior 55: the name and version the emitter
// writes must be the vendored `renderer/rendercv_typst/typst.toml`'s. It is
// packaging metadata that is directly observable in an artifact — a bumped
// package with an unbumped preamble compiles against the wrong library or not
// at all.
func TestTypstPackageMetadata(t *testing.T) {
	name, version := vendoredTypstPackage(t)

	preamble, err := fs.ReadFile(templater.BuiltinTemplates(), "typst/Preamble.j2.typ")
	if err != nil {
		t.Fatalf("reading the embedded preamble: %v", err)
	}
	match := typstPackageReference.FindStringSubmatch(string(preamble))
	if match == nil {
		t.Fatalf("the preamble imports no @preview package")
	}
	if match[1] != name || match[2] != version {
		t.Errorf("the preamble imports @preview/%s:%s, want @preview/%s:%s",
			match[1], match[2], name, version)
	}

	// The package the compiler resolves that import against is vendored under
	// D-007, in the `preview/<name>/<version>/` layout `get_package_path`
	// builds (`pdf_png.py:114-146`). A version bump that misses it is a
	// compile failure, not a byte diff.
	shipped := filepath.ToSlash(filepath.Join("preview", name, version, "typst.toml"))
	if _, err := fs.Stat(assets.Packages(), shipped); err != nil {
		t.Errorf("the vendored Typst package cache has no %s: %v", shipped, err)
	}
}

// vendoredTypstPackage reads `name` and `version` out of the submodule's
// `typst.toml`, which is the specification for both.
//
// **An absent submodule fails this test rather than skipping it.** The first
// draft skipped, and a skip here is behavior 55 quietly leaving the suite while
// the run still prints `ok` — and this test is untagged, so it is in the default
// `go test ./...` path where nobody is looking for a missing gate.
// `conformance.RequireInput` is the shared opt-out
// (`internal/conformance/requireinput.go`), the same convention
// `internal/clidiff` established for the differential.
func vendoredTypstPackage(t *testing.T) (name, version string) {
	t.Helper()

	const path = "../../third_party/rendercv/src/rendercv/renderer/rendercv_typst/typst.toml"
	content := conformance.RequireInput(t, path,
		"spec 013 behavior 55's Typst package assertion",
		"run `just submodule` to check out third_party/rendercv")
	for _, line := range strings.Split(string(content), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.Trim(strings.TrimSpace(value), `"`)
		case "version":
			version = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	if name == "" || version == "" {
		t.Fatalf("%s declares no name/version", path)
	}
	return name, version
}

// TestCopiedTemplatesAreUserWritable is behavior 22: `copy_templates` adds user
// write permission to every file and directory it copies
// (`cli/copy_templates.py:29-53`). The chmod pass exists for immutable
// distributions — a wheel installed read-only would otherwise hand the user a
// template folder they cannot edit, which defeats the flag's whole purpose.
//
// The port writes 0o644 files and 0o755 directories rather than chmod-ing after
// the fact; the observable contract is the same and is what this asserts.
func TestCopiedTemplatesAreUserWritable(t *testing.T) {
	cases := []struct {
		name    string
		options NewOptions
		folder  string
	}{
		{
			name:    "typst",
			options: NewOptions{Name: "John Doe", CreateTypstTemplates: true},
			folder:  defaultTheme,
		},
		{
			name:    "markdown",
			options: NewOptions{Name: "John Doe", CreateMarkdownTemplates: true},
			folder:  "markdown",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			if code := New(testCase.options, io.Discard, io.Discard); code != 0 {
				t.Fatalf("new exited %d, want 0", code)
			}

			seen := 0
			err := filepath.WalkDir(testCase.folder, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				seen++
				if info.Mode().Perm()&0o200 == 0 {
					t.Errorf("%s has mode %v, which is not user-writable", path, info.Mode().Perm())
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walking %s: %v", testCase.folder, err)
			}
			// The folder itself, the `entries` folder and their contents.
			if seen < 2 {
				t.Errorf("walked %d entries under %s, want the copied tree", seen, testCase.folder)
			}
		})
	}
}
