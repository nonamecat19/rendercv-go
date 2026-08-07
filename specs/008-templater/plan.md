# Iteration 8 — plan

Go design for `spec.md`. Behavior lives there.

---

## 1. The decision this iteration exists to make

`AGENTS.md` §6.1 says the port uses **pongo2 plus a mechanical transform** of the template source.
`spec.md` §4F measured what the transform would have to cover, and the measurement changes the
answer for one of the two engines and not the other.

| | What the templates need | Verdict |
|---|---|---|
| **Template engine** | 7 tags, 5 filters, 4 slice forms, 32 `splitlines()` sites, 5 `{%- if` | **pongo2 stays**, with a transform (§2) |
| **Markdown parser** | five block processors **off**, line-by-line conversion, an XML-ish tree walk | **goldmark cannot be used**, and §3 says what replaces it |

Neither is a divergence — output is what the contract measures. The second refines a technology
choice `AGENTS.md` §2's table states, which is what a subsystem's plan is for; §3 says why that is
not a `divergences.md` matter and what would be.

---

## 2. The template transform

**Template source may diverge; template output may not.** The transform is a build-time step in
`tools/gentemplates`, not a runtime one, so what ships is Go-embedded pongo2 source a reviewer can
read beside upstream's.

Five substitutions, each mechanical and each verified by the byte diff rather than by inspection:

| Jinja | pongo2 | Why it cannot stay |
|---|---|---|
| `x.splitlines()` | `x_lines` | pongo2 evaluates no Python methods |
| `…[:n]` / `…[n:]` / `…[0]` / `…[1:]` | `slice` filter with the same bound | pongo2 has no slice syntax |
| `x.as_rgb()` | `x` — the model already stores the rendered string | spec 006 §4 made `Color.String()` the stored form |
| `{% set n = … %}` with a computed bound | unchanged | pongo2 has `set`; only the expression inside changes |
| `{%- if` | `{%- if` | pongo2 supports left-trim, but §4 has to prove it means the same thing |
| `|indent(4)` | `|indent:4` | **measured after T12**: pongo2 takes filter arguments Django-style, with `:` and no parentheses. Thirteen sites. |
| `|replace("a", "b")` | `|replace:"/a/b/"` | **measured after T12**: pongo2 has **no** `replace` — this table's first draft said it did. Its filters take exactly one parameter, so the port registers a `replace` whose parameter is `sed`-shaped. Eight sites. |

**Two corrections to this section, both found by building it rather than reading
the library:**

1. **pongo2 autoescapes HTML by default**, so `<`, `>`, `&` and `"` become
   entities at every `{{ }}`. A `.typ` contains all four — `escape_typst_characters`
   emits `\"` and `\<` on purpose — so leaving it on corrupts every Typst
   document. `registerFilters` turns it off, and `TestAutoescapingIsOff` says so
   under its own name because the symptom looks like a template bug.
2. **pongo2 has no `indent`**, which is the opposite of what the paragraph below
   assumed: there is nothing to override, so the port's filter is simply Jinja's.
   The hazard it describes is real anyway — an implementation that indented the
   first line would break the four cancelling `replace` sites — and the test
   still guards it.

**The `…Lines []string` fields are the model's, not the transform's.** Every `splitlines()` target
is one of four fields, so the processed entry model carries `MainColumnLines`,
`DateAndLocationColumnLines`, `DegreeColumnLines` and `SummaryLines` beside the strings. That is
`AGENTS.md` §6.1's own proposal and §4F behavior 55 is the part it does not solve: two of the four
slice forms have a **computed** bound, so a slice filter is needed as well as the pre-split field.

### `trim_blocks`, `lstrip_blocks` and `indent`

`spec.md` §1 behavior 3 and §4F behavior 57 are the two whitespace risks, and they are checked the
same way: **the byte diff against goldens is the only gate that matters.** A unit test of a filter
in isolation proves nothing about a 400-line `.typ`.

`indent` is the specific hazard: Jinja's does **not** indent the first line and skips blank lines,
and four `|replace("    ", "")` sites depend on that cancelling exactly. The port registers its own
rather than relying on the engine's — see the correction above for what the engine actually has —
and `TestIndentAndReplaceCancel` runs one of the four sites end to end.

---

## 3. Markdown: goldmark is out on the Typst path

`spec.md` §4C behavior 35 is decisive. Upstream deregisters `hashheader`, `setextheader`, `olist`,
`ulist` and `quote`, so `# H`, `1. x`, `- x` and `> q` are **ordinary text** on the Typst path.
goldmark parses all five and has no supported way to remove them from its block parser set that
leaves the rest intact.

Three options, and the third is the plan's:

| | Approach | Why not |
|---|---|---|
| A | goldmark with a custom parser set | Its block parsers are not independently removable in a way that survives a version bump; the port would pin a version and hope |
| B | goldmark, then undo what it parsed | Reconstructing source text from a parsed heading is lossy and the failure is silent |
| **C** | **A hand-written inline parser for the five constructs upstream keeps** | It is small: `strong`, `em`, `code`, `a`, and the admonition block. §4C behavior 37's table *is* the specification, and §4C behavior 36's line-by-line conversion means it never has to handle a multi-line construct except the admonition |

**goldmark stays for the HTML path**, which is `markdown_to_html` — plain
`markdown.markdown(...)` with no deregistration — and that is iteration 11's.

**This does not need `specs/divergences.md`.** That file is scoped to deviations *from upstream*,
and every entry carries a "what the user notices" field; here the answer is **nothing**, because the
gate is byte-identical output. Upstream uses python-markdown, which the port cannot use at all —
goldmark was never upstream's choice either, so choosing between two Go implementations of the same
required behavior is a plan decision and this is the plan.

`AGENTS.md` §2's table names goldmark under "Go / replacement tech" for "HTML / Markdown". That is a
plan statement about a subsystem whose plan did not exist yet, and it should be corrected to say
goldmark for HTML and a hand-written parser for the Typst path. Flagged for the human rather than
edited silently — it is the project manual, not a gated file.

**What would need the gate** is a Markdown difference a user could see: a construct upstream renders
and the port does not, or vice versa. If the hand-written parser turns out to have one, it stops
there.

---

## 4. Packages

```
internal/renderer/templater/
  environment.go     the pongo2 environment, the two loaders, the filters
  loader.go          the four-candidate lookup of spec §2
  filters.go         indent, length, lower, replace, string, clean_url, strip
  assemble.go        render_full_template's separators (spec §3)
  process/           the processors, one file per upstream module
    model.go         process_model, process_fields          (spec §4A)
    date.go          the three formatters and the time span  (spec §4B)
    markdown.go      markdown_to_typst and the escape        (spec §4, §4C)
    notes.go         footer and top note                     (spec §4D)
    connections.go   the connection list                     (spec §4E)
    entrytemplates.go the expansion and the removal passes   (spec §4G)
  templates/         the transformed template source, go:embed'd
tools/gentemplates/  the transform of §2
```

`process/` mirrors upstream file for file, which `AGENTS.md` §9 asks for and which matters more
here than usual: the modules interact in ways §4D behavior 43 showed are load-bearing, and a
reviewer diffing mentally needs the same boundaries.

---

## 5. Ordering, and what gates each part

The spine rule applies — this is one owner's work — but the *gates* differ, and that decides the
order:

1. **The processors first, unit-gated.** Every one of §4A–§4G is a pure function over the model,
   and each has measured examples in the spec. None needs a template to be testable.
2. **Then the environment and the transform, golden-gated.** The first `.typ` byte diff is the only
   thing that can prove `trim_blocks`, `lstrip_blocks` and `indent`, and it needs the processors to
   exist because the model reaches the template already processed.
3. **The corpus turns green one case at a time**, not all at once. `testdata/golden` has 15 artifact
   cases; the simplest CV with one section and one entry type is the first, and each additional
   entry type is one more.

---

## 6. Hazards

1. **`indent`'s first line.** §2.
2. **The two Typst-source placeholders in the footer** (`spec.md` §4D behavior 43) survive only
   because the escape function holds `#`-commands out. Test end to end.
3. **The connection order is the input file's** (§4E behavior 45). The Go model must preserve
   `cv`'s key order, which the yaml reader already does — but nothing currently *reads* it, so the
   first consumer is here.
4. **`process_fields` `str()`s a non-string** (§4A behavior 25), so a numeric extra field arrives
   at the template as a string. A Go port using a typed accessor would keep it numeric and format
   it differently.
5. **`substitute_placeholders` strips** (§4 behavior 15), and §4B behavior 34 shows the strip is
   the *only* one — a port that trimmed each substitution would differ by interior spaces.
6. **A literal uppercase word in a template is a missing placeholder** (§4G behavior 73). Nothing
   distinguishes them, and upstream does not try to.
