package process

import "strings"

// The five emphasis patterns of `AsteriskProcessor.PATTERNS`
// (markdown/inlinepatterns.py:546-552), **in order**, and their underscore
// twins in `UnderscoreProcessor`.
//
// # Why these are hand-written matchers and not regexps
//
// Every one of them uses a backreference — `(\*)\1{2}` — and three use negative
// lookarounds. Go's RE2 has neither, by design. Expanding the backreferences is
// possible because the captured group is always the single delimiter character,
// but `(?!\1)` and `(?<!\w)` are not expressible at all.
//
// So each pattern is an explicit scan. That is more code than a transcribed
// regexp and it is the only faithful option; the alternative is a different
// pattern set, which is a different parse, which is different bytes.
//
// # The order is the behavior
//
// `parse_sub_patterns` (`:589-646`) only tries patterns **after** the index that
// matched the parent, which is what stops `***x***` from re-matching itself. The
// index is threaded through as `from`.
//
// # Matching and emitting are separate, spec 011 §7 / plan §3.2
//
// `match` reports **offsets** into `data`, not substrings, so a caller that
// needs source positions — the HTML backend's goldmark nodes, `emphasis_html.go`
// — can build a `text.Segment` directly instead of round-tripping through a
// copied string. `firstStart == firstEnd == -1` marks "no second group": three
// of the five patterns (`STRONG_RE`/`SMART_STRONG_RE`,
// `EMPHASIS_RE`/`SMART_EMPHASIS_RE`) only ever produce one. The Typst path
// (`build`, and `inline.go`'s `parseFrom`) slices `data[start:end]` right where
// it used to receive the substring directly — this is a zero-output-diff
// refactor, not a behavior change.
type emphasisPattern struct {
	// name is the upstream constant, for the doc trail only.
	name string
	// match reports the end offset and the two body groups as offsets into
	// data, or ok=false. secondStart < 0 means there is no second group.
	match func(data string, pos int) (end, firstStart, firstEnd, secondStart, secondEnd int, ok bool)
	// build turns the groups into Typst, recursing with the pattern index.
	build func(p *inlineParser, first, second string, index int, delim byte) string
}

// asteriskPatterns is `AsteriskProcessor.PATTERNS`.
//
// The delimiter is `*` throughout. The underscore set differs only in the
// delimiter and in the word-boundary guards, which `smart` carries.
// Built in an init rather than a composite literal: the builders call back into
// the parser, which reads this list, and Go rejects the cycle at package scope.
var (
	asteriskPatterns   []emphasisPattern
	underscorePatterns []emphasisPattern
)

func init() {
	asteriskPatterns = []emphasisPattern{
		{
			// EM_STRONG_RE `(\*)\1{2}(.+?)\1(.*?)\1{2}` → `strong,em`.
			// `***em*strong**`
			name: "EM_STRONG_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchTriple(data, pos, '*', 1, 2)
			},
			build: func(p *inlineParser, first, second string, index int, delim byte) string {
				return "#strong[" + "#emph[" + p.parseFrom(first, index, delim) + "]" +
					p.parseFrom(second, index, delim) + "]"
			},
		},
		{
			// STRONG_EM_RE `(\*)\1{2}(.+?)\1{2}(.*?)\1` → `em,strong`.
			// `***strong**em*`
			name: "STRONG_EM_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchTriple(data, pos, '*', 2, 1)
			},
			build: func(p *inlineParser, first, second string, index int, delim byte) string {
				return "#emph[" + "#strong[" + p.parseFrom(first, index, delim) + "]" +
					p.parseFrom(second, index, delim) + "]"
			},
		},
		{
			// STRONG_EM3_RE `(\*)\1(?!\1)([^*]+?)\1(?!\1)(.+?)\1{3}` → `strong,em`,
			// built the **other way round**: the first group is the strong's own
			// text and the second is the nested em. `**strong*em***`
			name: "STRONG_EM3_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchStrongEm3(data, pos, '*', false)
			},
			build: func(p *inlineParser, first, second string, index int, delim byte) string {
				return "#strong[" + p.parseFrom(first, index, delim) +
					"#emph[" + p.parseFrom(second, index, delim) + "]]"
			},
		},
		{
			// STRONG_RE `(\*{2})(.+?)\1`
			name: "STRONG_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				end, bodyStart, bodyEnd, ok := matchDelimited(data, pos, "**", "**", false)
				return end, bodyStart, bodyEnd, -1, -1, ok
			},
			build: func(p *inlineParser, first, _ string, index int, delim byte) string {
				return "#strong[" + p.parseFrom(first, index, delim) + "]"
			},
		},
		{
			// EMPHASIS_RE `(\*)([^\*]+)\1` — **the body admits no asterisk**, which
			// is the whole reason `*a **b** c*` parses as three separate emphases
			// rather than one containing a strong. Measured.
			name: "EMPHASIS_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				end, bodyStart, bodyEnd, ok := matchDelimited(data, pos, "*", "*", true)
				return end, bodyStart, bodyEnd, -1, -1, ok
			},
			build: func(p *inlineParser, first, _ string, index int, delim byte) string {
				return "#emph[" + p.parseFrom(first, index, delim) + "]"
			},
		},
	}

	// `UnderscoreProcessor.PATTERNS` (`:677-687`). Same five shapes with the
	// delimiter `_`, and the last three carry `(?<!\w)` / `(?!\w)` word guards —
	// which is what "smart" means and why `snake_case` keeps its underscores
	// while `_em_` does not.
	underscorePatterns = []emphasisPattern{
		{
			name: "EM_STRONG2_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchTriple(data, pos, '_', 1, 2)
			},
			build: asteriskPatterns[0].build,
		},
		{
			name: "STRONG_EM2_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchTriple(data, pos, '_', 2, 1)
			},
			build: asteriskPatterns[1].build,
		},
		{
			// SMART_STRONG_EM_RE `(?<!\w)(_)\1(?!\1)(.+?)(?<!\w)\1(?!\1)(.+?)\1{3}(?!\w)` —
			// unlike its asterisk twin, this one carries word-boundary guards
			// before the opening pair, before the middle delimiter, and after
			// the closing run. `__a_b___` fails all of them (the middle `_` sits
			// right after the word character `a`) and stays wholly literal;
			// `matchStrongEm3`'s asterisk call above never needs this because
			// `*` is never a `\w` byte to guard against.
			name: "SMART_STRONG_EM_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				return matchStrongEm3(data, pos, '_', true)
			},
			build: asteriskPatterns[2].build,
		},
		{
			// SMART_STRONG_RE `(?<!\w)(_{2})(?!_)(.+?)(?<!_)\1(?!\w)`
			name: "SMART_STRONG_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				end, bodyStart, bodyEnd, ok := matchSmart(data, pos, "__")
				return end, bodyStart, bodyEnd, -1, -1, ok
			},
			build: asteriskPatterns[3].build,
		},
		{
			// SMART_EMPHASIS_RE `(?<!\w)(_)(?!_)(.+?)(?<!_)\1(?!\w)`
			name: "SMART_EMPHASIS_RE",
			match: func(data string, pos int) (int, int, int, int, int, bool) {
				end, bodyStart, bodyEnd, ok := matchSmart(data, pos, "_")
				return end, bodyStart, bodyEnd, -1, -1, ok
			},
			build: asteriskPatterns[4].build,
		},
	}
}

// matchEmphasis tries patterns[cutoff+1:] in order at pos and returns the
// index and offsets of the first hit. It is `parse_sub_patterns`'s pattern
// selection (`:613-634`) alone, without the recursion or the Typst emission —
// the piece both the Typst path (`inline.go`'s `parseFrom`) and the HTML path
// (`emphasis_html.go`) share.
//
// `floor` is the calling processor's own registry priority — `prioEmStrong`
// for the asterisk set, `prioEmStrong2` for the underscore set, which is what
// makes the underscore patterns see asterisk spans already claimed.
//
// It matches against `maskAbove(data, floor)`, not `data` itself, and
// returned offsets are still valid on the original because masking never
// changes length. See `stash.go` for why every pattern registered above
// `em_strong` has to be resolved first.
func matchEmphasis(patterns []emphasisPattern, data string, pos, cutoff, floor int) (index, end, firstStart, firstEnd, secondStart, secondEnd int, ok bool) {
	masked := maskAbove(data, floor)
	for i, p := range patterns {
		if i <= cutoff {
			continue
		}
		if end, firstStart, firstEnd, secondStart, secondEnd, ok := p.match(masked, pos); ok {
			return i, end, firstStart, firstEnd, secondStart, secondEnd, true
		}
	}
	return 0, 0, 0, 0, 0, 0, false
}

// matchSmart is the underscore singles, whose four guards are the whole point:
// no word character before the run, no extra `_` after it, no `_` immediately
// before the closing run, and no word character after it.
//
// Together they are why `snake_case` and `a_b` keep their underscores — the
// character before the opening `_` is a word character — while `_em_` at a word
// boundary becomes emphasis.
func matchSmart(data string, pos int, run string) (end, bodyStart, bodyEnd int, ok bool) {
	if pos > 0 && isWordByte(data[pos-1]) {
		return 0, 0, 0, false
	}
	if !strings.HasPrefix(data[pos:], run) {
		return 0, 0, 0, false
	}
	rest := data[pos+len(run):]
	if len(rest) > 0 && rest[0] == '_' {
		return 0, 0, 0, false
	}

	for i := 1; i <= len(rest); i++ {
		if !strings.HasPrefix(rest[i:], run) {
			continue
		}
		if rest[i-1] == '_' {
			continue
		}
		after := i + len(run)
		if after < len(rest) && isWordByte(rest[after]) {
			continue
		}
		bodyStart = pos + len(run)
		bodyEnd = bodyStart + i
		return bodyEnd + len(run), bodyStart, bodyEnd, true
	}
	return 0, 0, 0, false
}

// isWordByte is Python's `\w` over the ASCII range, with every byte above it
// treated as a word character too — which is right for UTF-8 continuation bytes
// and for the Latin letters the corpus contains.
func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= 0x80
}

// matchTriple is the shape both EM_STRONG_RE and STRONG_EM_RE have: three
// delimiters, a lazy body, `innerRun` delimiters, a lazy tail, then `outerRun`.
//
// `(.+?)` requires at least one character; `(.*?)` allows none — which is the
// difference between the two bodies and why `first` is non-empty and `second`
// may be.
func matchTriple(data string, pos int, delim byte, innerRun, outerRun int) (end, firstStart, firstEnd, secondStart, secondEnd int, ok bool) {
	opening := strings.Repeat(string(delim), 3)
	if !strings.HasPrefix(data[pos:], opening) {
		return 0, 0, 0, 0, 0, false
	}
	rest := data[pos+3:]

	inner := strings.Repeat(string(delim), innerRun)
	outer := strings.Repeat(string(delim), outerRun)

	// Lazy: the first split that lets the remainder match wins.
	for i := 1; i <= len(rest); i++ {
		if !strings.HasPrefix(rest[i:], inner) {
			continue
		}
		tail := rest[i+len(inner):]
		for j := 0; j <= len(tail); j++ {
			if strings.HasPrefix(tail[j:], outer) {
				firstStart = pos + 3
				firstEnd = firstStart + i
				secondStart = firstEnd + len(inner)
				secondEnd = secondStart + j
				return secondEnd + len(outer), firstStart, firstEnd, secondStart, secondEnd, true
			}
		}
	}
	return 0, 0, 0, 0, 0, false
}

// matchStrongEm3 is `(\*)\1(?!\1)([^*]+?)\1(?!\1)(.+?)\1{3}`: two delimiters not
// followed by a third, a body with **no** delimiter in it, one delimiter not
// followed by another, a lazy body, then three delimiters.
//
// `smart` adds the underscore twin's extra word-boundary guards —
// `SMART_STRONG_EM_RE` is `(?<!\w)(_)\1(?!\1)(.+?)(?<!\w)\1(?!\1)(.+?)\1{3}(?!\w)`,
// three `\w`-adjacency checks `STRONG_EM3_RE` does not have, because `*` is
// never a word byte and the checks would be vacuous for it.
func matchStrongEm3(data string, pos int, delim byte, smart bool) (end, firstStart, firstEnd, secondStart, secondEnd int, ok bool) {
	if smart && pos > 0 && isWordByte(data[pos-1]) {
		return 0, 0, 0, 0, 0, false // `(?<!\w)` before the opening pair
	}

	two := strings.Repeat(string(delim), 2)
	if !strings.HasPrefix(data[pos:], two) {
		return 0, 0, 0, 0, 0, false
	}
	rest := data[pos+2:]
	if len(rest) > 0 && rest[0] == delim {
		return 0, 0, 0, 0, 0, false // the `(?!\1)` after the opening pair
	}

	for i := 1; i <= len(rest); i++ {
		if rest[i-1] == delim {
			return 0, 0, 0, 0, 0, false // `[^*]+?` — no delimiter inside the first body
		}
		if i >= len(rest) || rest[i] != delim {
			continue
		}
		if i+1 < len(rest) && rest[i+1] == delim {
			continue // the second `(?!\1)`
		}
		if smart && isWordByte(rest[i-1]) {
			continue // `(?<!\w)` before the middle delimiter
		}
		tail := rest[i+1:]
		three := strings.Repeat(string(delim), 3)
		for j := 1; j <= len(tail); j++ {
			if !strings.HasPrefix(tail[j:], three) {
				continue
			}
			closeEnd := pos + 2 + i + 1 + j + 3
			if smart && closeEnd < len(data) && isWordByte(data[closeEnd]) {
				continue // `(?!\w)` after the closing run
			}
			firstStart = pos + 2
			firstEnd = firstStart + i
			secondStart = firstEnd + 1
			secondEnd = secondStart + j
			return closeEnd, firstStart, firstEnd, secondStart, secondEnd, true
		}
	}
	return 0, 0, 0, 0, 0, false
}

// matchDelimited is the simple `open … close` pair. `plain` forbids the
// delimiter character inside the body, which is EMPHASIS_RE's `[^\*]+`.
func matchDelimited(data string, pos int, open, close string, plain bool) (end, bodyStart, bodyEnd int, ok bool) {
	if !strings.HasPrefix(data[pos:], open) {
		return 0, 0, 0, false
	}
	rest := data[pos+len(open):]

	for i := 1; i <= len(rest); i++ {
		if plain && rest[i-1] == open[0] {
			return 0, 0, 0, false
		}
		if strings.HasPrefix(rest[i:], close) {
			bodyStart = pos + len(open)
			bodyEnd = bodyStart + i
			return bodyEnd + len(close), bodyStart, bodyEnd, true
		}
	}
	return 0, 0, 0, false
}
