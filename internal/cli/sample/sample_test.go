package sample

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/version"
)

// axes lists the themes and the locales the embedded blocks cover — behavior
// 54's 9 and 22.
func axes(t *testing.T) (themes, locales []string) {
	t.Helper()
	for _, axis := range []struct {
		dir  string
		into *[]string
		want int
	}{
		{"blocks/design", &themes, 9},
		{"blocks/locale", &locales, 22},
	} {
		entries, err := fs.ReadDir(blocks, axis.dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			*axis.into = append(*axis.into, strings.TrimSuffix(entry.Name(), ".yaml"))
		}
		if got := len(*axis.into); got != axis.want {
			t.Fatalf("%s holds %d blocks, want %d", axis.dir, got, axis.want)
		}
	}
	return themes, locales
}

// TestGenerateEveryPair is the part of spec 013 §8's 198-case criterion that
// needs no Python: every pair generates, and no two pairs generate the same
// document. What each one must *say* is upstream_conformance_test.go's
// differential — this only catches a block that went missing or got duplicated
// on its way into the embedded set.
func TestGenerateEveryPair(t *testing.T) {
	themes, locales := axes(t)
	seen := make(map[string]string, len(themes)*len(locales))
	for _, theme := range themes {
		for _, locale := range locales {
			pair := theme + "/" + locale
			document, err := Generate("John Doe", theme, locale)
			if err != nil {
				t.Errorf("%s: %v", pair, err)
				continue
			}
			if other, ok := seen[document]; ok {
				t.Errorf("%s and %s generate the same document", other, pair)
			}
			seen[document] = pair
		}
	}
	if got, want := len(seen), len(themes)*len(locales); got != want {
		t.Errorf("generated %d distinct documents, want %d", got, want)
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
