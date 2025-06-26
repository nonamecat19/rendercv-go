# Iteration 8 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

`[parallel]` marks a leaf that reads no other task's output. **Wave A is the only genuine fan-out
this iteration has**: six processors that never read each other, over six upstream modules. The
environment and the transform are the spine and stay with one owner (`AGENTS.md` §5).

---

## Wave 0 — withdrawn

### T1 — the divergence entry · ~~`[human gate]`~~ — **withdrawn, not needed**

This task claimed the Markdown-parser decision of `plan.md` §3 needed a `divergences.md` entry and
the human gate. **It does not, and saying so blocked the iteration for no reason.**

`divergences.md` is scoped by its own header — "every deviation from `third_party/rendercv`" — and
every entry carries a **"what the user notices"** field. D-001: they type a different command name.
D-002: a custom theme needs translating. For a Markdown parser that produces byte-identical output
the answer is **nothing**, and an entry whose user-visible effect is nothing is not a divergence.

The deeper error: upstream uses python-markdown, which the port cannot use **at all**. goldmark was
never upstream's choice either — it is one of two Go implementations of the same required behavior,
and picking between them is a plan decision, which is what `plan.md` is for. `AGENTS.md` §2's table
names goldmark under "Go / replacement tech"; that is a plan statement about a subsystem whose plan
had not been written yet, and `plan.md` §3 is where it gets refined.

**What would need the gate**: a Markdown difference a user could see — a construct upstream renders
and the port does not, or vice versa. If the hand-written parser turns out to have one, that is a
divergence and it stops here.

`AGENTS.md` §2's table should be corrected to match; it is not itself a gated file, but it is the
project manual and the correction is flagged for the human rather than made silently.

---

## Wave A — the processors — **done**

Six leaves. Each is a pure function over the model with measured examples already in `spec.md`, and
none needs a template to be testable.

### T2 — the two string processors · `[parallel]` — **done**
`process/strings.go`: `substitute_placeholders` and `clean_url`.
Spec §4 behaviors 15 and 5, §1 behavior 5.
Tests: longest-first ordering; the trailing `.strip()`; `clean_url` removing `https://` **anywhere**
and exactly one trailing slash.

### T3 — the Typst escape · `[parallel]` — **done**
`process/markdown.go`, first half: `escape_typst_characters`.
Spec §4 behavior 16.
Tests: the three phases in order; `$$x$$` collapsing to `$x$`; the two longer replacements running
after the thirteen single characters; a lone `"\n"` returning immediately.

### T4 — markdown → Typst · `[parallel]` — **done**
`process/markdown.go`, second half: the hand-written parser of `plan.md` §3, the five-tag walk, and
line-by-line conversion.
Spec §4C behaviors 35–39.
Tests: `# Heading` surviving as literal text; an unmatched `*` on adjacent lines not pairing; the
five mapped tags; a dropped `admonition-title`; tail text surviving.

### T5 — dates · `[parallel]` — **done**
`process/date.go`: the eight placeholders and the three formatters.
Spec §4B behaviors 27–31.
Tests: a bare year never running through `single_date`; only `format_single_date` falling back to
the raw string; `YEAR_IN_TWO_DIGITS` as a slice.

### T6 — time spans · `[parallel]` — **done**
`process/date.go`: `compute_time_span_string`.
Spec §4B behaviors 32–34.
**Its own unit, not T5's**, because the arithmetic is where the wrong answer is reasonable: `< 2
years` reported as `1`, the unconditional `+ 1` month, the overflow fold, and the empty-string
collapse that makes a template disappear rather than print `0 years`.

### T7 — connections · `[parallel]` — **done**
`process/connections.go`: the six keys, the twenty-one icons, and the two format branches.
Spec §4E behaviors 45–51.
Tests: the input file's key order driving the list; the three icons that are not the lowercased
name; `Google Scholar`'s literal body; the four Typst shapes from two independent flags.

### T8 — the footer and the top note · `[parallel]` — **done**
`process/notes.go`.
Spec §4D behaviors 40–44.
Tests: the two placeholder maps being different; `NAME` as `""` when absent; the exact
`context { [ … ] }` spacing.

### T9 — entry template expansion · `[parallel]` — **done**
`process/entrytemplates.go`, first half: behaviors 58–66 and the six processors of 67–72.
Tests: an empty-string field counting as not provided; phrase expansion leaving sub-placeholders;
`SUMMARY` wrapped only when standalone; `DOI` overwriting `URL`.

### T10 — the removal passes · `[parallel]` — **done**
`process/entrytemplates.go`, second half: behaviors 73–76.
**Its own unit** for the same reason as T6 — it has the most surface and the least obvious rules.
Tests: a missing field taking its `**`, its comma and its connector word while `*in*` survives;
`clean_trailing_parts` dropping a line that became empty; a literal uppercase word treated as a
missing placeholder.

### T11 — `process_model` · `[sequential]` — **done**
`process/model.go`. The orchestrator, so it reads every task above.
Spec §4A behaviors 17–26.
Tests: the deep copy, proven by rendering Typst then Markdown from one model; bolding before the
Typst conversion; `_plain_name` reaching `pdf_title` while the processed name reaches the header;
the four skipped fields.

---

## Wave B — the engine — **done**

**T14 landed first**, out of the wave's written order. It is the only unit here that needs neither
pongo2 nor a transformed template, and landing it before them means the first golden failure points
at a template rather than at the separators around it.

### T12 — the environment · `[sequential]` — **done**
`environment.go`, `loader.go`, `filters.go`: the two loaders, the four-candidate Typst lookup, and
the seven filters.
Spec §1, §2. Plan §2.
Tests: a user override of one entry type for one theme taking effect; `indent` not indenting the
first line, proven by one `|replace("    ", "")` site cancelling.

### T13 — `tools/gentemplates` · `[sequential]` — **done**
The five substitutions of `plan.md` §2, emitting the embedded pongo2 source. `just gentemplates`
reruns it. Its head states what the transform does **not** guarantee, as `localeprobe`'s does.

### T14 — the assembly · `[sequential]` — **done**
`assemble.go`: `render_full_template`'s separators.
Spec §3 behaviors 9–13. **Testable before any template renders**, so it lands before the corpus
work rather than inside it.

---

## Wave C — the corpus, one case at a time

### T15 — the first golden green · `[sequential]`
The simplest corpus CV: one section, one entry type, the classic theme. This is the commit that
first proves `trim_blocks`, `lstrip_blocks` and `indent` together, and no earlier task can.

### T16–T23 — the remaining eight entry types · `[parallel]`
One commit each, per `AGENTS.md` §7's table. They fan out because each is one template and one
golden and they never read each other.

### T24 — the remaining themes · `[parallel]`
Eight commits, one per theme, once every entry type is green.

### T25 — close the ledger · `[sequential]`
`specs/STATE.md`, with the artifact-parity count Axis 1 actually reaches.

---

## Out of scope

The HTML wrapper and `markdown_to_html` are iteration 11's (`spec.md` §5.1); compiling the `.typ`
is iteration 10's (§5.2). This iteration ends at a `.typ` and a `.md` string.
