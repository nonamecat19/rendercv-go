package conformance

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// PDFFacts is what parity axis 1 compares a PDF on.
//
// Raw bytes are out by construction: upstream's PDFs carry a creation timestamp
// and a document ID, and a byte diff would fail on every run for reasons that
// have nothing to do with the port. `specs/000-parity-contract/spec.md` §1.2
// names the three things that must match instead, and this is that triple.
type PDFFacts struct {
	// Text is the document's text in reading order, as `pdftotext -layout`
	// extracts it. Layout mode is deliberate: it preserves column structure, so
	// a table that collapses into the wrong order is caught.
	Text string
	// Pages is the page count.
	Pages int
	// Geometry is the page size of every page, `WIDTHxHEIGHT` in points,
	// one entry per page.
	Geometry []string
	// Fonts is every embedded font's base family name, in `pdffonts`' own
	// order, with the six-letter subset tag (`XXXXXX+`) stripped: the tag is
	// typst's own subsetting id and is not a property of the document upstream
	// and the port share.
	Fonts []string
}

// AssertPDF checks the two PDFs on the three facts axis 1 names.
//
// The oracle is poppler (`pdftotext`, `pdfinfo`), the same pair the iteration 10
// measurement pass used. It is an external tool like `uv` is for the goldens —
// the suite reads it, the port does not depend on it.
func AssertPDF(t *testing.T, name string, want, got []byte) {
	t.Helper()

	wantFacts, err := ReadPDF(want)
	if err != nil {
		t.Fatalf("%s: reading the golden PDF: %v", name, err)
	}
	gotFacts, err := ReadPDF(got)
	if err != nil {
		t.Errorf("%s: reading the produced PDF: %v", name, err)
		return
	}

	if wantFacts.Pages != gotFacts.Pages {
		t.Errorf("%s: page count: golden %d, got %d", name, wantFacts.Pages, gotFacts.Pages)
	}

	// spec §1.2's fourth criterion: embedded font names, as a set. Order does
	// not carry meaning — poppler lists fonts by PDF object id, which typst
	// assigns independently on each side — so the comparison is set equality,
	// not sequence equality.
	if !sameStringSet(wantFacts.Fonts, gotFacts.Fonts) {
		t.Errorf("%s: embedded fonts: golden %v, got %v", name, wantFacts.Fonts, gotFacts.Fonts)
	}

	for i, w := range wantFacts.Geometry {
		if i >= len(gotFacts.Geometry) {
			break
		}
		if g := gotFacts.Geometry[i]; w != g {
			t.Errorf("%s: page %d geometry: golden %s, got %s", name, i+1, w, g)
		}
	}

	// Text last: it is the assertion that produces a diff worth reading, and a
	// page-count mismatch above already explains most of what it would print.
	AssertText(t, name+" (extracted text)", wantFacts.Text, gotFacts.Text)
}

// ReadPDF extracts the three comparable facts from PDF bytes.
func ReadPDF(raw []byte) (PDFFacts, error) {
	text, err := pdftotext(raw)
	if err != nil {
		return PDFFacts{}, err
	}
	pages, geometry, err := pdfinfo(raw)
	if err != nil {
		return PDFFacts{}, err
	}
	fonts, err := pdffonts(raw)
	if err != nil {
		return PDFFacts{}, err
	}
	return PDFFacts{Text: text, Pages: pages, Geometry: geometry, Fonts: fonts}, nil
}

func pdftotext(raw []byte) (string, error) {
	// `-layout` keeps the physical layout; `-enc UTF-8` pins the encoding so a
	// locale difference on the machine running the suite cannot change the
	// comparison. `-` reads stdin and `-` writes stdout.
	out, err := run("pdftotext", raw, "-layout", "-enc", "UTF-8", "-", "-")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func pdfinfo(raw []byte) (int, []string, error) {
	// `-f 1 -l -1` asks for every page, which makes pdfinfo print a per-page
	// size line — a document whose pages differ in size is a real failure mode
	// and a single "Page size" line would hide it.
	out, err := run("pdfinfo", raw, "-f", "1", "-l", "-1", "-")
	if err != nil {
		return 0, nil, err
	}

	var pages int
	var geometry []string
	for line := range strings.Lines(string(out)) {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch {
		case key == "Pages":
			pages, _ = strconv.Atoi(value)
		case strings.HasPrefix(key, "Page ") && strings.HasSuffix(key, "size"):
			geometry = append(geometry, normalizeSize(value))
		}
	}
	if pages == 0 {
		return 0, nil, fmt.Errorf("pdfinfo reported no pages")
	}
	// A single-page-size document prints one line for every page anyway with
	// `-f/-l`, so a short list means the parse missed something.
	if len(geometry) != pages {
		return 0, nil, fmt.Errorf("pdfinfo reported %d pages but %d size lines", pages, len(geometry))
	}
	return pages, geometry, nil
}

// pdffonts lists every font `pdffonts` reports, with typst's own six-letter
// subset tag (`XXXXXX+`, e.g. `GRREMH+SourceSans3-Italic`) stripped from the
// front. The tag is assigned per compile and is not stable even between two
// runs of the same document on the same binary, so leaving it in would make
// the comparison fail on font subsetting rather than on a font difference.
func pdffonts(raw []byte) ([]string, error) {
	out, err := run("pdffonts", raw, "-")
	if err != nil {
		return nil, err
	}

	var fonts []string
	lines := strings.Split(string(out), "\n")
	// The first two lines are the header and its `---` underline.
	for _, line := range lines[min(2, len(lines)):] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		fonts = append(fonts, stripSubsetTag(fields[0]))
	}
	return fonts, nil
}

// stripSubsetTag removes a leading `XXXXXX+` subset tag: exactly six
// uppercase ASCII letters followed by `+`, the shape every subsetter in this
// pipeline (both typst builds) uses.
func stripSubsetTag(name string) string {
	plus := strings.IndexByte(name, '+')
	if plus != 6 {
		return name
	}
	for _, r := range name[:6] {
		if r < 'A' || r > 'Z' {
			return name
		}
	}
	return name[7:]
}

// sameStringSet compares two slices as sets — spec §1.2 names embedded fonts
// "as a set", so neither order nor duplicate count carries meaning.
func sameStringSet(a, b []string) bool {
	toSet := func(items []string) map[string]bool {
		set := make(map[string]bool, len(items))
		for _, s := range items {
			set[s] = true
		}
		return set
	}
	wantSet, gotSet := toSet(a), toSet(b)
	if len(wantSet) != len(gotSet) {
		return false
	}
	for s := range wantSet {
		if !gotSet[s] {
			return false
		}
	}
	return true
}

// normalizeSize turns pdfinfo's `595.28 x 841.89 pts (A4)` into `595.28x841.89`.
// The paper-name suffix is dropped: it is poppler's own guess, not a property of
// the document.
func normalizeSize(value string) string {
	fields := strings.Fields(value)
	if len(fields) < 3 || fields[1] != "x" {
		return value
	}
	return fields[0] + "x" + fields[2]
}

func run(tool string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(tool, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", tool, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
