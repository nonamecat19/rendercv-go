package sample

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/version"
)

// matrix is testdata/matrix.json: the md5 of the starter CV upstream writes for
// each of the 198 theme/locale pairs, captured by tools/sampleprobe.
type matrix struct {
	Version   string            `json:"version"`
	Themes    []string          `json:"themes"`
	Locales   []string          `json:"locales"`
	Documents map[string]string `json:"documents"`
}

func loadMatrix(t *testing.T) matrix {
	t.Helper()
	content, err := os.ReadFile("testdata/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var m matrix
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestGenerateMatrix is spec 013 §8's first acceptance criterion, run against
// digests rather than a live Python process: every one of the 9 x 22 documents
// upstream can produce, byte for byte.
func TestGenerateMatrix(t *testing.T) {
	m := loadMatrix(t)
	if got, want := len(m.Documents), len(m.Themes)*len(m.Locales); got != want {
		t.Fatalf("fixture has %d documents, want %d", got, want)
	}
	for _, theme := range m.Themes {
		for _, locale := range m.Locales {
			t.Run(theme+"/"+locale, func(t *testing.T) {
				document, err := Generate("John Doe", theme, locale)
				if err != nil {
					t.Fatal(err)
				}
				digest := md5.Sum([]byte(document))
				if got, want := hex.EncodeToString(digest[:]), m.Documents[theme+"/"+locale]; got != want {
					t.Errorf("md5 = %s, upstream = %s", got, want)
				}
			})
		}
	}
}

// TestGenerateVersion pins the fixture to the constant: a submodule bump that
// moves upstream's `__version__` without moving ours would otherwise show up as
// 198 identical digest failures with no explanation.
func TestGenerateVersion(t *testing.T) {
	if got := loadMatrix(t).Version; got != version.RenderCV {
		t.Errorf("upstream is v%s, internal/version says v%s", got, version.RenderCV)
	}
}

// TestGenerateSchemaHint is the third of spec 013 §3.3 behavior 26's three
// version sites — the other two are `--version` and the `new` banner, both in
// internal/cli.
func TestGenerateSchemaHint(t *testing.T) {
	document, err := Generate("John Doe", "classic", "english")
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(document, "\n")
	want := "# yaml-language-server: $schema=https://raw.githubusercontent.com/" +
		"rendercv/rendercv/refs/tags/v" + version.RenderCV + "/schema.json"
	if first != want {
		t.Errorf("line 1 =\n%s\nwant\n%s", first, want)
	}
}

// TestGenerateBlockOrder is spec 013 §8's block-order and trailing-newline
// criterion, and §6.1's whitespace guarantee.
func TestGenerateBlockOrder(t *testing.T) {
	document, err := Generate("John Doe", "classic", "english")
	if err != nil {
		t.Fatal(err)
	}

	at := -1
	for _, key := range []string{"\ncv:\n", "\ndesign:\n", "\nlocale:\n", "\nsettings:\n"} {
		next := strings.Index(document, key)
		if next < 0 {
			t.Fatalf("no %q in the document", strings.TrimSpace(key))
		}
		if next <= at {
			t.Errorf("%q is out of order", strings.TrimSpace(key))
		}
		at = next
	}

	if strings.Contains(document, "\r") {
		t.Error("the document has a CR")
	}
	if !strings.HasSuffix(document, "\n") || strings.HasSuffix(document, "\n\n") {
		t.Errorf("the document does not end in exactly one newline: %q",
			document[len(document)-8:])
	}
}

// TestFileName is `new`'s output path (`cli/new_command/new_command.py:81`):
// spaces to underscores and no other sanitizing at all.
func TestFileName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"John Doe", "John_Doe_CV.yaml"},
		{"Matías", "Matías_CV.yaml"},
		{"", "_CV.yaml"},
		{"  pad  ", "__pad___CV.yaml"},
		{"a/b", "a/b_CV.yaml"},
		{".", "._CV.yaml"},
	}
	for _, c := range cases {
		if got := FileName(c.name); got != c.want {
			t.Errorf("FileName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestSplitNestedBullets is spec 013 §8's behavior-8 criterion. The indent is
// the literal twelve spaces upstream writes (`schema/sample_generator.py:154`),
// so a bullet at any other depth is re-indented to twelve — which is why the
// synthetic lines below are not at the sample content's own depth.
func TestSplitNestedBullets(t *testing.T) {
	const indent = "            " // twelve
	cases := []struct{ name, in, want string }{
		{
			name: "a deeper bullet is still re-indented to twelve",
			in:   "                - a - b",
			want: "                - a\n" + indent + "- b",
		},
		{
			name: "a shallower bullet is re-indented to twelve too",
			in:   "  - a - b - c",
			want: "  - a\n" + indent + "- b\n" + indent + "- c",
		},
		{
			name: "a preceding space suppresses the split",
			in:   "  - a  - b",
			want: "  - a  - b",
		},
		{
			name: "a following space suppresses it as well",
			in:   "  - a -  b",
			want: "  - a -  b",
		},
		{
			name: "a line that is not a list item is untouched",
			in:   "  summary: a - b",
			want: "  summary: a - b",
		},
		{
			name: "a dash without both spaces is not a separator",
			in:   "  - well-known -x",
			want: "  - well-known -x",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := splitNestedBullets(c.in); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

// TestCommentBlock is spec 013 §8's behavior-10 criterion over the six shapes
// of §5.3.6: the transform replaces the *first* two-space run with `# ` and
// then prefixes two spaces, whatever the line looks like.
func TestCommentBlock(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"a depth-1 key", "  page:\n", "  # page:"},
		{"a depth-2 key", "    size: us-letter\n", "  #   size: us-letter"},
		{"a depth-3 key", "      body: rgb(0, 0, 0)\n", "  #     body: rgb(0, 0, 0)"},
		{"a list item", "    - experience\n", "  #   - experience"},
		{"an empty line", "\n", "  "},
		{"a line with no two-space run", "x\n", "  x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := commentBlock(c.in); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

// TestGenerateUnknownAxis pins the internal error: `new` checks both flags
// before it calls here (`cli/new_command/new_command.py:65-77`), so this is
// unreachable from the CLI and must stay a plain failure rather than half of
// upstream's §4.3 wording.
func TestGenerateUnknownAxis(t *testing.T) {
	if _, err := Generate("John Doe", "nope", "english"); err == nil {
		t.Error("an unknown theme generated a document")
	}
	if _, err := Generate("John Doe", "classic", "nope"); err == nil {
		t.Error("an unknown locale generated a document")
	}
}
