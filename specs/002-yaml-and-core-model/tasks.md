# Tasks 002 — YAML reader and core model

25 commits. Each leaves `go build ./... && go test ./...` green (`AGENTS.md` §7). Fixture tasks
that must land red do so behind `//go:build conformance`, so the untagged suite stays green
while `go test -tags conformance ./...` shows the red (`AGENTS.md` §9).

**Marks.** `[parallel]` tasks within the same wave read none of each other's output and may be
fanned out to porters. `[sequential]` tasks are on the pipeline spine and stay with one owner
(`AGENTS.md` §5, the stop rule). Waves are strictly ordered.

Every task cites the spec section it implements. A task is done when its spec sections'
acceptance criteria (spec §8) pass.

---

## Wave A — leaves, no dependencies

### T1 — `schemaerr`: exception taxonomy · `[parallel]`
`internal/schema/schemaerr/{error.go,source.go}`.
`ValidationError` (with `Children`), `UserError`, `UserValidationError`, `InternalError`, the
`YamlSource` literals, `OverlayKey`, `OverlayToSource`. All three error types implement `error`.
Spec §2, §3.42, §3.43. Plan §6.
Tests: the four source literals are exactly spec §2's strings; the overlay map has exactly three
entries; `errors.As` distinguishes all three error types.

### T2 — `yamldoc`: node tree types · `[parallel]`
`internal/schema/yamldoc/{node.go,position.go}`. `Position`, `Span`, `Kind`, `ScalarStyle`,
`Node`, `Item`. No parsing. Plan §3.
Tests: constructors, `Kind` stringer, span ordering helpers.

### T3 — section title casing · `[parallel]`
`internal/schema/models/cv/section.go` — only `titleFromKey` and `snakeCaseTitle`.
Spec §3.62–§3.64, §3.66, §5.9, §5.10.
Tests: spec §5.9's seven-row table verbatim; all 28 stop words in first and non-first position;
the `ßeta`/`ﬁle`/`ǆab`/`çay` capitalization cases.

### T4 — entry type name in snake case · `[parallel]`
`internal/schema/models/cv/entries/bases/entry.go` — only the name transform.
Spec §3.68.
Tests: all nine names of spec §3.57 map to their snake-case forms.

### T5 — entry-type registry · `[parallel]`
`internal/schema/models/cv/entries/registry.go`. `Descriptor`, `Registry`, `Characteristic()`,
`Discriminate()`, `Names()`. Plus `registry_fixture_test.go` holding the eight descriptors with
upstream's real field sets. Spec §3.55–§3.58, §7.1. Plan §5.
Tests: `Characteristic()` on the fixture registry equals spec §3.56's table exactly; the common
set is exactly `{date, start_date, end_date, location, summary, highlights}`; `Names()` ends in
`TextEntry`; an entry carrying characteristic fields of two types discriminates to the earlier
one.

### T6 — validation context · `[parallel]`
`internal/schema/models/validationcontext.go`. `ValidationContext`, `InputPath()`, `Today()`.
Spec §3.22–§3.26. Plan §7.
Tests: absent context → no path, today; `"today"` → today; a real date → itself; `yesterday` →
today without error.

---

## Wave B — the reader spine

T7 has no dependencies and may be claimed with Wave A; it is listed here because everything
after it in this wave consumes it.

### T7 — goccy dependency + `dealias` token transform · `[parallel]`
Adds `github.com/goccy/go-yaml v1.19.2` to `go.mod`.
`internal/schema/yamlreader/noalias.go` — one exported-to-package function
`dealias(token.Tokens) token.Tokens`, per plan §1.3(c). Reads nothing from any other task; it
operates purely on goccy's token stream, which is why it is `[parallel]` despite being the
hardest fidelity requirement in the iteration.
Spec §3.10, §5.3. Plan §1.2, §1.3.

Table test, all ten probe cases, asserting the parsed tree not just the absence of an error:

```
key: *not_an_alias                       -> {key: "*not_an_alias"}
mixed: *a and more                       -> {mixed: "*a and more"}
multi:\n  - *one\n  - *two               -> {multi: ["*one", "*two"]}
nested:\n  inner: *deep_value            -> {nested: {inner: "*deep_value"}}
real_anchor: &anchor value\nuse: *anchor -> anchor node + {use: "*anchor"}
highlights:\n  - normal *star* here      -> {highlights: ["normal *star* here"]}
b: '*quoted'                             -> {b: "*quoted"}
block: |\n  a *literal* block            -> literal scalar preserved verbatim
date: 2020-09-24 / month: 2020-09        -> string nodes
year: 2020                               -> integer node
```

Plus a **no-op regression test**: `dealias` over all 46 YAML files in the submodule
(9 example CVs, the ATS corpus, test fixtures, 21 locale catalogs, 8 theme files,
`error_dictionary.yaml`) must produce trees identical to parsing without it. 46 identical,
0 differing, 0 failures.

The fifth probe case only reaches its final form once T9 unwraps anchor nodes; at T7 it asserts
the token-level outcome (an anchor node survives, the alias became a string).

If any case cannot be expressed, stop and switch to plan §1.3(d) — the swap is this function's
body and nothing else.

### T8 — coordinate fixture + differential test, red · `[sequential]`
`internal/schema/yamlreader/testdata/coords/` plus `tools/yamlprobe/` (a `just` target that
runs the vendored Python and dumps every node's `lc.data` for the fixture documents). Fixture
is **generated, never hand-written** (`AGENTS.md` §10.1). The test comparing Go's spans to the
fixture is `//go:build conformance` and lands red.
Spec §3.13, §6.7. Plan §3.
Fixture documents: nested mappings, sequences of mappings, sequences of scalars, an empty
mapping value, a multi-line flow mapping.

### T9 — AST → `yamldoc` builder · `[sequential]`
`internal/schema/yamlreader/build.go`: `lexer.Tokenize` → `dealias` → `parser.Parse` → walk
`*ast.File` → `yamldoc.Node`, with key order and spans. Turns T8's test green.
Decoding goes through the AST, never `goccy.Unmarshal`, so alias resolution gets no second
chance (plan §1.3.1).
**Unwraps `*ast.AnchorNode` to its value**, completing spec §5.3's fifth case:
`real_anchor: &anchor value` → `{real_anchor: "value"}`. This has no upstream test and is a
required acceptance criterion (spec §3.10a, §8).
Spec §3.10a, §3.12, §3.13, §5.3. Plan §1.3.1, §3.

### T10 — scalar resolution + differential corpus · `[sequential]`
`internal/schema/yamlreader/resolve.go`. Null/bool/int/float/string classification from `Raw`,
timestamps deliberately unresolved. Extends T8's generated fixture with the scalar corpus:
`yes`, `no`, `on`, `off`, `null`, `~`, ``, `0o17`, `0x1F`, `00123`, `+1_000`, `.inf`, `.NaN`,
`2020`, `2020-09`, `2020-09-24`, `2020-09-24T10:00:00Z`, quoted forms of each, literal and
folded block scalars with `|`, `|-`, `|+`, `>`, `>-`, `>+`, and every `*`-bearing case from T7.
Spec §3.11, §5.4. Plan §3, §8.2.

### T11 — `ReadFile` / `ReadString` · `[sequential]`
`internal/schema/yamlreader/yamlreader.go`. Existence check, extension check, UTF-8 read,
empty check, scalar-string-root check — in upstream's order.
Spec §3.1–§3.9, §4.1–§4.4, §5.1, §5.2.
Tests: every acceptance criterion under spec §8 "Reading" except those belonging to T7/T9/T10
and the two belonging to T13.

### T12 — path types · `[parallel]` (with T8–T11; reads only T6)
`internal/schema/models/path.go`. Both types, resolution base, existence and is-a-file checks,
the relative-to-cwd serializer with its absolute fallback.
Spec §3.35–§3.41, §4.5, §4.6, §5.25, §5.26.

---

## Wave C — merge and binder spine

### T13 — YAML syntax error → validation error · `[sequential]`
`internal/schema/modelbuilder/yamlerror.go`. Wrap a parser failure into one
`ValidationError`: no schema location, span from the parser's position, correct source, spec
§4.17's message, input echo `...`. First-line-plus-period normalization of the parser text.
Spec §3.82–§3.85, §5.12.
Add a `TODO(iteration-4)` naming spec §7.3 at the `{parser_message}` interpolation.

### T14 — overlay merge · `[sequential]`
`internal/schema/modelbuilder/merge.go`. `settings`/`settings.render_command` defaulting, the
three overlays in fixed order taking only their own top-level key and replacing wholesale,
retained overlay documents, the eleven render-command overrides with the truthiness rule, and
the position where dotted-key overrides will be applied (a no-op hook this iteration).
Spec §3.14–§3.21, §5.17, §5.18, §5.27, §5.28, §6.5.

### T15 — binder core · `[sequential]`
`internal/schema/binder/binder.go`. Field-set matching against `Node.Items`, the two extra-key
policies, absent-vs-null per plan §4, required-and-required-but-nullable, and ordered error
accumulation into `schemaerr.ValidationError` with schema locations and spans.
Spec §3.32–§3.34, §5.15, §5.16, §6.6.

---

## Wave D — the models

### T16 — top-level model · `[sequential]`
`internal/schema/models/rendercvmodel.go`. Four fields in order, four defaults, extra keys
forbidden, input path recorded out-of-band, the JSON-schema `required: []` marker carried as
metadata for iteration 5. `design`, `locale`, `settings` bind to opaque `yamldoc.Node`
placeholders this iteration.
Spec §3.27–§3.31.

### T17 — `Cv` fields and key order · `[sequential]`
`internal/schema/models/cv/cv.go`. The ten fields in declaration order, extra keys forbidden,
`_key_order` capture with null-valued keys dropped and the non-mapping-input case.
Spec §3.44, §3.45, §3.50, §3.51, §5.15, §5.16.
`photo`, `email`, `phone`, `website`, `sections` bind as raw nodes here; T18/T21/T25 replace
each.

### T18 — scalar-or-list routing + phone serialization · `[sequential]`
`internal/schema/models/cv/cv.go`. The shared rule for `email`, `phone`, `website`: null →
null, list → list-of-element validation, otherwise single-element validation; the missing-field-
name internal error; `tel:` stripping on serialization. Element validators are registered hooks
with iteration-2 pass-through implementations and `TODO(iteration-4)` markers.
Spec §3.47, §3.48, §3.49, §4.7.

### T19 — `BaseEntry` and `BaseEntryWithDate` · `[sequential]`
`internal/schema/models/cv/entries/bases/{entry.go,entrywithdate.go}`. Extra keys retained and
readable; the `date` field with arbitrary-date validation.
Spec §3.67, §3.69, §3.70, §4.13, §5.24.

### T20 — date-object conversion and exact dates · `[sequential]`
`internal/schema/models/cv/entries/bases/entrywithcomplexfields.go`, part 1. The six ordered
cases of spec §3.73, the no-reference-date internal error, and exact-date validation with its
two distinct failure messages.
Spec §3.71–§3.76, §4.14, §4.15, §4.18, §5.11.
The §4.13 table test carries a `TODO(iteration-4)` naming spec §7.3.

### T21 — complex-field entry: fields and date precedence · `[sequential]`
Same file, part 2. The five fields in declaration order plus the four-step precedence and the
ordering check.
Spec §3.77–§3.79, §4.16, §5.21, §5.22, §5.23.

### T22 — section validation · `[sequential]`
`internal/schema/models/cv/section.go`. Not-a-list, empty-list, per-entry inference, the
skip-and-continue loop, first-resolvable wins, all-entries validation against one type, and the
five section error messages with nested children preserved.
Spec §3.53, §3.54, §3.58–§3.61, §4.8–§4.12, §5.5–§5.8, §6.2, §6.3.
The `[1]` / `[[]]` case emits spec §4.9 with a `TODO(iteration-4)` naming spec §5.14.

### T23 — typed section list · `[sequential]`
Same file. `title`/`entry_type`/`entries` records in input order, the empty-list-forces-
`TextEntry` rule, first-entry inference for non-empty sections.
Spec §3.52, §3.65, §5.5.

---

## Wave E — shells

### T24 — social-network shell · `[parallel]`
`internal/schema/models/cv/socialnetwork.go`. Two required fields, extra keys forbidden, the
seventeen network names in order. No username patterns, no URL generation.
Spec §3.80, §7.
`TODO(iteration-4)` at the username field naming `social_network.py:59-184`.

### T25 — custom-connection shell + `photo` union · `[parallel]`
`internal/schema/models/cv/customconnection.go`: three fields, `url` required-but-nullable,
extra keys forbidden. Plus wiring `Cv.photo` to T12's existence-required path type with the
left-to-right union order.
Spec §3.46, §3.81.

---

## Fan-out summary

| Wave | Parallel tasks | Owner |
|---|---|---|
| A | T1, T2, T3, T4, T5, T6, T7 | seven porters |
| B | T12 alongside the T8–T11 spine | one porter for T8–T11, one for T12 |
| C | none | spine owner |
| D | none | spine owner |
| E | T24, T25 | two porters |

T8–T11 and T13–T23 are the spine and must not be split across agents. T7 is a leaf only
because `dealias` is a pure function over a token slice; it is still the highest-risk task in
the iteration and its 10+46 test table is the gate on plan §1.3(c).

## Exit

Iteration 2 is done when spec §8's checkboxes all pass under `go test ./...`, `just check` is
clean, and `rendercv-parity-verifier` confirms in a fresh context that
`go test -tags conformance ./...` is unchanged from its iteration-1 state — 42 cases red, no new
failures, no infrastructure errors (spec §7.2). `specs/STATE.md` moves iteration 2 to `green`
with `0` conformance cases, and the two deferred string decisions of spec §7.3 are carried into
iteration 3's spec as open items.
