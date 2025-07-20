# Port ledger

Living state of the rendercv-go port. **Updated only by the merge owner, after
`rendercv-parity-verifier` reports.** Never edited by a porter as part of a feature commit.

Upstream target: `third_party/rendercv` @ `v2.8` (`2eba248`)

Legend: `—` not started · `spec` spec written · `red` tests written, failing ·
`wip` implementation in progress · `green` all its conformance cases pass

---

## Iterations

| # | Subsystem | Spec | Status | Conformance cases passing |
|---|---|---|---|---|
| 0 | Bootstrap (layout, AGENTS.md, submodule, agents, skills, CI) | — | green | n/a |
| 1 | Conformance harness (corpus, gengolden, helpers) | [001](001-conformance-harness/spec.md) | green | n/a (42 cases red by design) |
| 2 | YAML reader + core model (RenderCVModel, CV, Section) | [002](002-yaml-and-core-model/spec.md) | green (with cut scope, see below) | n/a (gated on unit tests, spec §7.2) |
| 3 | Entry types (9) | [003](003-entry-types/spec.md) | green (with cut scope, see below) | n/a (gated on unit tests, spec §7.1) |
| 4 | Validation-error parity | [004](004-validation-errors/spec.md) | green | n/a (gated on the 25-record differential, spec §7.3) |
| 5 | JSON Schema generator | [005](005-json-schema/spec.md) | green (Axis 3 now closed by 6) | n/a (gated on the 18 owned `$defs`, spec §7.1) |
| 6 | Design & themes (9) + the settings schema | [006](006-design-and-themes/spec.md) | green (with cut scope, see below) | n/a (gated on the 164 `$defs` differential and the override diff, spec §5) |
| 7 | Locale (English + 21 catalogs) | [007](007-locale/spec.md) | green | n/a (gated on the 45 `$defs` differential and the submodule catalog diff, spec §5) |
| 8 | Templater (pongo2 env, filters, markdown→typst, processors) | [008](008-templater/spec.md) | green (with cut scope, see below) | n/a (gated on the 52-fragment Jinja differential and 240 unit cases, spec §7) |
| 9 | Typst renderer (`.typ` emission) + iteration 6's T10 + iteration 8's Wave C | [009](009-typst-renderer/spec.md) | **green** — verified by a fresh context, which returned FAIL on four items; all four fixed and pinned | 24 / 24 |
| 10 | wazero + WASI typst → PDF, then PNG | — | — | 0 |
| 11 | Markdown + HTML renderers | [011](011-markdown-and-html/spec.md) | **green** — both documents byte-identical on all 24 cases | 24 / 24 md, 24 / 24 html |
| 12 | CLI (`new`, `render`, `create-theme`, overrides, watcher) | [012](012-cli/spec.md) | **started** — `render` and `new` are wired; `new`'s seven starter CVs are byte-identical against their goldens. `create-theme` and the six help panels are not written. Every parity number is blocked on one of the three gates below | 0 (see below) |
| 13 | Parity closeout (sample generator, version, error handler, packaging) | — | — | 0 |
| 14 | Lua-scripted custom themes (D-002) + the two folder messages | — | — | 0 |

## Parity axes

| Axis | Gate command | Status |
|---|---|---|
| 1 — artifacts byte-identical | `just test-parity` | **72 passing comparisons.** Every text artifact — 24/24 `.typ`, `.md` and `.html` — byte-identical against the vendored Python (`TestCorpusTypstIsByteIdentical`), over the 21 corpus inputs plus three the corpus cannot express. PDF and PNG are iteration 10's. The 15 CLI-driven artifact cases stay red until iteration 12: they shell `rendercv-go render`, which does not exist. |
| 2 — CLI surface | `just test-parity` | **1/21 passing** — `cli_version`, the first green case in the whole parity suite. It is the only CLI output carrying no `rendercv` token, which is why it is reachable before the binary-name question below is answered. |
| 3 — JSON Schema | `just schema-diff` | **green.** All 227 `$defs` byte-identical; the command exits 0. The oracle is `tools/genschema`, not the parity suite — `TestSchemaParity` shells `rendercv-go schema` and stays red until iteration 12. |
| 4 — validation errors | `just test-parity` | measurable, 0/7 corpus cases passing; the 25-record differential is green |

PDF content comparison (spec §1.2) is not yet measurable — it lands with iteration 10.

## Stretch goals (not gates)

- [ ] PNG pixel-level comparison (depends on the WASI typst font set — see D-006)
- [ ] Public `pkg/rendercv` API frozen and semver'd
- [ ] Cross-compiled release artifacts (linux/darwin/windows × amd64/arm64)

## Cut scope

Anything dropped from an iteration is recorded here with the reason, per `AGENTS.md` §10.2.

### Iteration 2

Verified by `rendercv-parity-verifier` in a fresh context. Everything below is carried into
iteration 3's spec as an open item; nothing here is a silent divergence.

1. **Coordinate columns diverge from ruamel in two shapes the T8 fixture cannot see.** A key
   with a null or empty value reports column 1 rather than the key's own indent
   (`internal/schema/yamlreader/build.go`), and a flow-sequence element reports the first value
   token rather than the `[`. Measured against upstream: 33/232 paths differ on
   `examples/John_Doe_ClassicTheme_CV.yaml`, 50/388 on
   `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`. **Lines match
   everywhere**, and spec §6.7 says only line numbers reach users, so nothing user-visible is
   affected yet — but `expected_errors.yaml` is iteration 4's import, so this must be fixed
   there, together with extending the fixture to cover both shapes.
2. **`models.Validate` does not call `cv.Validate`.** `models` owns `ValidationContext` and the
   path types, which `cv` imports, so the edge would cycle. Closing it needs those two types
   moved to a leaf package — a `plan.md` layout change, deferred to iteration 3 rather than
   made unreviewed at the tail of this one. Everything *inside* `cv` is wired (commit
   `fd33d82`).
3. **`phone` formatting (spec §3.49) is not implemented.** `+905419999999` does not serialize to
   `+90-541-999-99-99`; only the `tel:` strip is done. Spec §8 lists this as an acceptance
   criterion while spec §7 assigns phone formatting to iteration 4 — **the spec contradicts
   itself and iteration 4 must resolve which is right.**
4. **T7's no-op regression test over the submodule YAML corpus was never written.** The verifier
   ran it by hand — 64/64 files identical with and without `dealias` — so the transform is
   sound, but nothing in the suite guards it. `noalias_test.go` also asserts tokens where
   `tasks.md` required the parsed tree; the tree-level assertions now live in
   `yamlreader_test.go` (commit `f91da06`).
5. **T10's scalar corpus is hand-written, not generated.** `tools/yamlprobe` still emits only the
   five coordinate documents, so `resolve_test.go` states Go-side expectations rather than
   deriving them from upstream. Behavior was differentially verified as correct.
6. **§4.12's "mixed section" and "entry problems" criteria are tested through an injected
   validator**, because the concrete entry types are iteration 3's. They must be re-checked
   against real types when those land.

Two process failures in this iteration's history, recorded rather than rewritten:

- `1befa1e` bundles T9, T10 and T11 in one commit, against `AGENTS.md` §7.
- T8's coordinate test landed in `65aaa49`, *after* the T9 implementation it was supposed to
  precede, inverting the red-before-green rule of `AGENTS.md` §4.

### Iteration 3

Verified by `rendercv-parity-verifier` in a fresh context, which returned **FAIL** with three
blockers. Two were fixed inside the iteration (commit `9ddd896`); the rest is cut here. Nothing
below is a silent divergence, and none of it belongs in `divergences.md` — every item is an
ordinary bug reproducible in Go, not a place where parity is impossible.

**Fixed rather than cut** (recorded because the failure mode matters): three tests I wrote
asserted the *port's* error codes instead of upstream's, which is why the suite was green while
the codes were wrong. One test stated the correct upstream code in a comment and asserted a
different one on the next line. The diamond caught what self-report would not have.

1. **Entry error ordering does not interleave base and own fields.** The composed binder spec is
   `[base fields] ++ [own fields]`, the reverse of what `class X(BaseWithDates, BaseX)` produces,
   and the `date`/`start_date` validation errors are appended after all bind errors rather than
   emitted at their declared position. Measured against upstream:

   | input | upstream | Go |
   |---|---|---|
   | `ExperienceEntry{company, summary: {a: 1}}` | `position` missing, `summary` string_type | `summary` string_type, `position` missing |
   | `ExperienceEntry{position, highlights: "x"}` | `company` missing, `highlights` list_type | `highlights` list_type, `company` missing |
   | `ExperienceEntry{company, date: 2020-13-01, location: {a: 1}}` | `position`, `date`, `location` | `location`, `position`, `date` |
   | `PublicationEntry{title, authors, doi: bad, journal: {a: 1}}` | `doi` string_pattern_mismatch, `journal` string_type | `journal` string_type, `doi` string_pattern_mismatch |

   The descriptors already carry the right order, so part of the fix is feeding the binder the
   descriptor order. But the date errors must also be *interleaved* at their declared position
   rather than appended, which touches all three base binders plus `publication.go`. Spec 003 §6.3
   already carries a `TODO(iteration-4)` on entry error ordering, and iteration 4 owns error
   ordering, deduplication and message rewriting as its whole subject — so this lands there rather
   than as a rushed refactor at the tail of iteration 3. **Iteration 4 must not build its rewrite
   pipeline before fixing this**, because dedup by `schema_location` is order-sensitive.

2. **A non-scalar `date` or `start_date` produces no error at all.** Upstream produces two union-branch
   errors: `{date: {a: 1}}` on an `ExperienceEntry` gives `('date','int') int_type` and
   `('date','str') string_type`; `start_date` likewise. Go is silent. Reachable through real types
   for the first time in this iteration.

   Iteration 4 detail that makes this cheaper there than here: `unwanted_locations`
   (`pydantic_error_handling.py:24-32`) filters `"int"` and `"str"` out of the location, collapsing
   both rows to `('date',)`, and dedup by `schema_location` (`:167-176`) then merges them into
   **one** user-visible error. So the correct fix is one error at the field, and it needs iteration
   4's filtering and dedup to be expressed faithfully.

3. **Two `specs/003-entry-types/spec.md` §8 claims were inaccurate and are corrected in the record,
   not the code.** (a) §3.18 behavior 42 says iteration 2 "already emits both" custom error types —
   it emitted only `rendercv_other_error`; fixed in `9ddd896`. (b) §8's "every section test iteration 2
   wrote passes against the real registry with no edit to the test" is false: three tests changed
   (`sectionlist_test.go:20-24`, `:81-88`; `sectionvalidation_test.go`'s
   `TestFirstResolvableEntryWins`). The changes are correct — each fixture was one the
   accept-everything stub let pass and that upstream genuinely rejects, and each new expectation was
   measured against the vendored Python first — but the criterion as written cannot be checked, and
   the stale comment claiming otherwise has been corrected.

4. **`Registry.Discriminate` rebuilds the characteristic table on every call**
   (`entries/registry.go:70`). Not a parity issue; performance only. Left as is.

5. **The constructed-entry half of §8's discrimination criterion is not tested.** Upstream asserts
   the seven `(entry_type_name, section_model_name)` pairs twice — once from a raw dict and once
   from a constructed model instance (`tests/schema/models/cv/test_section.py:19-60`). Only the
   raw-mapping half is covered. The first attempt at the second half was a tautology: it validated
   a node and then re-resolved the same node, and both calls take the identical mapping branch, so
   it could not fail unless `Discriminate` were nondeterministic. It was removed rather than left
   as false coverage.

   Upstream's already-a-model branch (`section.py:173-176`) returns `entry.__class__.__name__` and
   has no Go equivalent — a non-mapping, non-string, non-null node falls through to the
   `messageNoType` branch under the `TODO(iteration-4)` in `sectionvalidation.go`. Reproducing it
   depends on iteration 4's §5.14 already-a-model decision, so it lands there.

6. **`messageModelType` is missing the entry type name.** Ours is `Input should be a valid
   dictionary`; upstream's is `Input should be a valid dictionary or instance of EducationEntry` —
   the concrete type is interpolated, and no `error_dictionary.yaml` row rewrites it, so it reaches
   the user raw. Measured on `[{institution, area}, null]`. Reachable from an ordinary document (a
   stray blank list item). Iteration 4 owns it, and it needs the entry type threaded into the
   binder's message rather than a table lookup.

Two process failures in this iteration, recorded rather than rewritten:

- `9ddd896` bundles the wrapper fix, the date-code split, the test corrections and two new
  criterion tests in one commit, against `AGENTS.md` §7. It should have been four.
- `8d131da` is labelled `test:` but ships 17 lines of production `default.go`. The production line
  was the deliberately-empty `Default()` that made the conformance test red, so the red-before-green
  intent held, but the type prefix is wrong.

Verifier items closed inside the iteration: the behavior-43 code table (it had substituted the date
code and `model_type` for `string_pattern_mismatch` and `url_too_long`, so two of the five were
never exercised), discrimination from an already-constructed entry, and the summary-only entry
matching no type end to end.

Not verifiable yet, and not claimed: PDF/PNG, artifact and CLI parity (iterations 10 and 12).

### Iteration 4

**No cut scope.** Every task in `tasks.md` landed, every `TODO(iteration-4)` in the tree is
cleared, and both items cut from iteration 3 plus four of the five carried from iteration 2 are
closed.

The gate is `TestWrongInputDifferential`: upstream's own `wrong_input.yaml` through the whole
port, compared against `expected_errors.yaml` **member by member, in order, with an equal-length
assertion** — all 25 records, all five members including coordinates. It is a port of
`tests/schema/test_pydantic_error_handling.py:19-54` and is the strongest mechanical Axis-4 gate
available.

The parity suite stays at its 42 red cases, which is iteration 1's baseline and not this
iteration's failure: no corpus case can pass until the renderer exists (spec §7.3). No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Bugs found while measuring, none in the task that found them.** Recorded because each was
invisible to the suite that existed at the time:

| Bug | Found by |
|---|---|
| Quoted mapping keys kept their quotes, so `"name": John` was rejected as an unknown key | the dictionary's one quoted scalar |
| `0b101` resolved to a string; ruamel reads it as 5 | the generated scalar corpus |
| Unknown keys were reported before every declared field | measuring the order upstream reports |
| An exact date's range and isoformat failures carried the wrong code (4 of 5 rows) | measuring §4.13's texts |
| `Invalid isoformat string` was missing entirely | the same measurement |
| `PublicationEntry.url` reported after every other field | registering its validator |
| A valueless key's span ended at column 1 whatever the indent | the coordinate differential |
| A flow-collection element started at its first value, not its bracket | the same |

**Two corrections to the spec, both recorded in place rather than silently edited:**

1. §3.2's table gave the reason for step 1 preceding the dictionary as "or `value is not a valid
   phone number` never matches". It does not hold — that message carries no prefix, and
   substitution matches by containment and replaces the whole message, so a prefix can only add a
   match. The ordering is still reproduced;
   `errorpipeline.TestPrefixStripDoesNotChangeWhichRowMatches` now asserts the unobservability, so
   a future row that makes the order matter fails rather than passing silently.
2. §3.13 behavior 46 said the URL length limit is 2083 **characters**. It is UTF-8 **bytes** — the
   check lives in pydantic-core, which is Rust. ASCII hides the difference, so nothing measured
   before was wrong, but a port counting runes would accept URLs upstream rejects.

**Carried forward, with owners:**

- **`emailaddr` is known-incomplete by decision** (spec §7.4). It reproduces 45 measured
  rejections; the library's catalogue is larger. The gap runs toward *accepting* what upstream
  rejects rather than misreporting it — the safer direction, since a plausible-but-wrong message
  passes review while a missing record shows up in the differential. `ErrUnclassified` exists and
  is unreachable, kept for a rule that can be detected but not named.
- **The YAML parser message is option B, scoped to the corpus** (spec §7.5, plan §6). Five
  goccy→ruamel phrasings are mapped and measured; an unmapped failure falls through to goccy's own
  line. **The coordinate half is not covered**: ruamel reports a context mark and a problem mark,
  goccy reports one token, so the corpus case's `line 1 to line 2` comes out as `line 1`. That is
  a decision needing `specs/divergences.md` and the human gate, and this iteration does not
  authorize writing one — see "Open for the human gate" below.
- **Two upstream crashes reassigned to iteration 12** (spec §7.8): a non-mapping entry and a
  list-valued `phone`. Neither is a validation-error-parity question — upstream produces no
  message, only an unhandled exception — so both wait on the CLI's unhandled-failure handling.
  Both carry `TODO(iteration-12)` markers.

**Open for the human gate:** the YAML-syntax coordinate span above. Nothing else.

### Iteration 5

**No cut scope.** Every task landed. The one deferral is written into the spec rather than cut
here: the `$defs` collision suffix (§7.2) is iteration 6's, because it is exercised only by the
per-theme variants and its numbering follows pydantic's emission order — a plausible
implementation would be a wrong answer that reads right, so `DefNameWithSuffix` panics naming the
iteration that owns it.

**Axis 3 is blocked, not failing, and the distinction is the iteration's main finding.**
`schema.json` is a projection of the model tree, so it is complete exactly when the tree is.
Measured over the 227 `$defs`:

| Owner | `$defs` | Share |
|---|---:|---:|
| Iteration 6 — design and themes | 161 | 74.8% |
| Iteration 7 — locale | 45 | 17.0% |
| Iteration 7 — settings | 3 | 3.3% |
| **Iterations 2–4 — what exists** | **18** | **4.8%** |

All 18 are byte-identical to upstream's. `just schema-diff` prints 8,621 lines of difference and
**zero of them are the port's** — every line it emits appears in upstream's file, which is asserted
by `TestPortSchemaInventsNothing`. A green `schema-diff` before iteration 7 would mean the port
invented something.

The alternative — reordering design and locale ahead of the generator — is recorded in spec §7.1
and rejected: the generator is what makes those two iterations mechanically checkable, and
reordering would put the two largest model iterations back to back with no gate between them.

**Four things the differential caught that transcription would not**, each invisible without
comparing real bytes:

| Finding |
|---|
| A required single-arm field **inlines** its arm: `authors` is `{items, type}`, not `{anyOf: [{items, type}]}` |
| `Cv`, `SocialNetwork` and `CustomConnection` have **no `description` key**; every entry type has an explicit `null` — the entry base's `json_schema_extra`, not a docstring difference |
| `CustomConnection.url` is required *and* nullable: in `required`, carrying the null arm, and with **no default** |
| `Cv`'s three scalar-or-list fields carry **no type key at all**, because pydantic cannot express that union |

One rule was derived rather than listed, and it will matter later: pydantic omits a field's title
when the schema is exactly one `$ref` plus `null`. That is why `date` and `start_date` have none
and `end_date` does — the shape, not the field. Iterations 6 and 7 add many more
optional-reference fields, and a list of the four known cases would have missed them silently.

**Two corrections to the parity contract**, both made in place in `specs/000-parity-contract`:

1. Axis 3 named a `rendercv-go schema` subcommand, which Axis 2.1's "no commands added" forbids —
   upstream's CLI has three commands and none is `schema`. The generation path is now named
   explicitly (`go run ./tools/genschema`), so the two axes cannot be read as contradicting.
2. Axis 3 required a trailing newline. Upstream's file has none; its last three bytes are `"\n}`.

**Carried forward:** the absent-set test states the remaining count as a number that must be
changed deliberately, so iterations 6 and 7 cannot close Axis 3 by accident or leave it closed on
paper. Iteration 7 moved it from 209 to 164; iteration 6 takes it to 0.

### Iteration 6

**Axis 3 is closed.** All 227 `$defs` are byte-identical and `just schema-diff` exits 0. The
oracle is `tools/genschema`; the parity suite's own Axis-3 case shells `rendercv-go schema` and
stays red until iteration 12, so "closed" means by the generation path spec 005 §4 named, not by
`just test-parity`.

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with twelve
findings, two of them blockers.** Everything below either was fixed inside the iteration or is cut
here with its reason. None of it belongs in `divergences.md`.

**The blocker worth reading, because the diamond is the only thing that could have caught it.**
The error pipeline reproduced upstream's step 2 — pydantic-core inserts a discriminated union's
resolved branch value as the location's second element, and `parse_plain_pydantic_error` deletes
it. The port never produces that element: `design.Validate` and `locale.Validate` resolve the union
themselves. Deleting anyway removed a **real** key. `design.colors.body` became `design.body`,
which failed to resolve against the document and reached the user as an internal error;
`design.nope` became `design`, which resolved and shipped a wrong location **silently**.

It was unreachable until this iteration emitted the first non-`theme` location under `design`, and
it survived my own tests because they stop at `models.Validate`. The fix deletes step 2 and adds
`TestDesignAndLocaleErrorsSurviveTheWholePipeline`, which runs five documents through `Parse` and
asserts what the user sees — including the colour message reaching dictionary row 13, the first
live producer for a row that has been in the table since iteration 4.

**Four shape checks were missing and are now measured** (`0dbd1e4`): a bool reported nothing at
all, `font_family` accepted a sequence, a non-string colour reported `string_type` instead of
`color_error`, and a non-mapping nested model reported nothing. Each of the four now carries the
code and message upstream gives, including the pair that distinguishes `bool_parsing` from
`bool_type` and the `model_type` text that **names the model**.

**Cut scope, with owners:**

1. **T10, the effective per-theme option tree, was cut to iteration 9** and is **now closed**
   there (`design.Effective`). Nothing validates a default, so the validation walk is the same for
   all nine themes — `TestDesignBlock` shows the same failure under `classic` and `sb2nov`. The
   cost was stated rather than hidden at the time: `WidenFontFamily` and `SnakeCaseSectionTitles`
   had no non-test callers, and spec §5 criterion 4's "must produce the same **model**" was not
   testable. Both are now called by `Effective`, which runs the two coercions where upstream's
   field validators do, and the criterion is met by a seven-document differential against
   upstream's own resolved model.
2. **Wave E — the D-002 Lua custom-theme path and spec §3 behavior 7's two folder messages — is
   iteration 14's**, a new row in the table above. `plan.md` §7 gives the reason: a sandbox bundled
   with 161 `$defs` makes both unreviewable. `design.go` carries the `TODO(iteration-14)` that
   `tasks.md` promised and did not have until the verifier asked for it.
3. **`settings` is still the thin slice of spec 004 §7.9.** The three `$defs` shipped here to close
   Axis 3; unknown-key rejection under `settings` and everything `RenderCommand` describes are
   iteration 12's. `specs/006-design-and-themes/settings.md` says so.

**A process failure the verifier found and I am recording rather than rewriting:**
`settings/schema.go` shipped before any spec covered it, against `AGENTS.md` §4. The retrofit is
`settings.md`, and it marks which of its criteria are open rather than implying the block is
finished.

**Two commit-discipline failures**, also recorded: `ff0c903` bundles T9, T11 and T12, and `58fc1f0`
bundles three `$defs` models with the Axis-3 status claim. Each should have been three commits.

**A design finding worth carrying forward:** a `design` block with **no `theme` key crashes
upstream** — `validate_design` runs in front of the union and does `str(design["theme"])` unguarded
(`design.py:57`), so the shape that gives `locale` a `union_tag_not_found` gives `design` a
`KeyError`. The port stays silent rather than reporting where upstream crashes, joining the two
crashes spec 004 §7.8 sent to iteration 12.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 8

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with three
blockers.** All three are fixed (`718c902`); the rest is cut here or recorded.

**The blocker worth reading is the one my own reasoning hid.** Spec §8 argued that the two
whitespace acceptance criteria could not be checked in this iteration, because only a corpus `.typ`
can exercise the transform and a corpus `.typ` needs iteration 9's model bridge. The second half is
true. **The first is not** — a *fragment* needs only a dictionary — and the verifier said so.

The differential I then wrote found, on its first run, that **Jinja strips one trailing newline
from every template** (`keep_trailing_newline=False`) and pongo2 does not. Every fragment gained a
`\n`, and `Assemble` joins entries with `"\n\n"`, so that is a blank line per entry and per section
on **every** artifact case. Reverting the one transform rule fails 43 of the 52 fragments.

I had written the argument that made it invisible. "Only the end-to-end gate can check this" is a
claim that needs testing rather than asserting, and spec §8 now records the reasoning rather than
the conclusion.

**The other two blockers**, both invisible to 235 passing unit tests:

1. **`escape_typst_characters` phase 1 rescanned the mutated text.** Upstream's `itertools.chain`
   binds both `finditer`s before the loop mutates `string`, so the command pattern never sees a
   math dummy. Rescanning matched `#emph[RENDERCVTYPSTCOMMANDORMATH0]` as one command and leaked
   the **literal dummy name** into the output.
2. **Python's `\b` is Unicode-aware and Go's is ASCII-only**, so any `bold_keywords` entry starting
   or ending with a non-ASCII character never matched — `["Café"]` left `Café au lait` untouched.
   `ats_diacritics` is a corpus case.

**Cut scope, with its owner:**

- **Wave C — the corpus's artifact cases — moves to iteration 9**, together with iteration 6's
  already-cut T10. Turning one green needs the schema-to-renderer model bridge, which is iteration
  9's by two earlier decisions. What moves with it is only what no fragment exercises:
  `Preamble.j2.typ` and `Header.j2.typ`, which read `cv._connections`, `cv._footer` and effective
  design values. All twenty-three of spec §7's criteria are met here.

**Two things the verifier found that are open, and are not blockers:**

- **`render_entry_templates` and `process_date` were not ported** — `tasks.md` T9 marked behaviors
  58–66 done and only the leaf helpers existed. Both are pure string functions needing nothing from
  the renderer, so they were **under-scoped, not blocked**. **Closed at the head of iteration 9**
  (`EntryDate`, `RenderEntryTemplates`), and the orchestrator is what made the other nine
  processors reachable at all: before it, `process.Run` called `RunFields` directly and no theme
  template was ever expanded.
- **Five `markdown_to_typst` divergences**, none declared: an image is dropped upstream and
  rendered here, raw HTML passes through unescaped upstream, an autolink becomes a link upstream, a
  link title is not stripped here, and a doubled backtick differs. All reachable from ordinary CV
  text. **This needs `specs/divergences.md` and the human gate** unless iteration 9 closes them —
  and unlike the gate I invented for the parser choice, these are user-visible.

**One more, measured and left as is:** an **empty** `bold_keywords` entry produces different
garbage on each side, because Go skips an empty regexp match adjacent to a non-empty one and Python
does not. Documented in its own test with the measurement; whether it is worth a divergence entry
is a human call.

**Process failures, recorded rather than rewritten:** `6f11003` bundles the environment, the loader
and seven filters where `AGENTS.md` §7's table asks for one commit per filter plus one for the
environment; `f785d95` bundles the generator, four filters, the embed, a `justfile` recipe and 26
generated templates across 31 files; and two generated fixtures landed in the same commit as the
code they gate, against §7's "fixtures land first, red".

Also: spec §4F behavior 56 says `-%}` appears **never**. It appears nine times. The transform is
correct anyway — pongo2 implements both trims — but the inventory the plan was sized from was
wrong.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 7

**No cut scope.** Every task landed: the ten-field catalog model with both length messages
(T1–T2), the submodule-diff gate (T3), all twenty-two catalogs (T4–T5), the `$defs` collision
numbering (T26), and the 45 locale `$defs` (T27).

T26 landed here rather than in iteration 6, where spec 005 §7.2's panic pointed. Locale's collision
is a flat list of twenty-two; design's is nine themes × their nested models. Same rule, and getting
it visibly right on the easy case means iteration 6 inherits it working. Its difficulty is
invisible in the output — `$defs` sorts its keys, so an alphabetically-assigned suffix produces a
file that *looks* correctly sorted while pairing every model with the wrong number.

**Axis 3 is now blocked on iteration 6 alone.** 63 of 227 `$defs` are the port's and byte-identical;
the 164 absent are the design tree and the three settings models. The absent-set count in
`internal/schema/jsonschema/golden_conformance_test.go` moved from 227−18 to 227−63, and
`just schema-diff` still emits exactly one `+` line — the diff header — so the port invents nothing.

**Two behaviors were measured only while implementing**, both written into spec §3.2 rather than
only here, and both cases where the port's natural answer differs from upstream:

1. **The twelve-element bound is `EnglishLocale`'s alone.** `{language: danish, month_names: [11
   items]}` validates and the same document with `english` does not — the `at.Len(12, 12)` lives in
   `Annotated` metadata that `create_simple_field_spec` strips when it rebuilds each variant's
   field. T2 had applied it to every member, which **rejects documents upstream accepts**, and no
   English fixture could see it. Fixed in `5ded987`; the schema half of the same fact (no
   `minItems`/`maxItems` on the twenty-one variants) had already been forced correct by the `$defs`
   differential, which is how the validation half was found.
2. **`{locale: {language: null}}` is a failure, not an absence** — `union_tag_invalid` with the tag
   reading `'None'`. Two sibling failures came with it, `union_tag_not_found` for a block with no
   `language` and `model_attributes_type` for a non-mapping `locale`.

**Three findings from the verifier, all closed inside the iteration:**

- **`ValidateCatalog` was unreachable.** `rendercvmodel.go` called only `ValidateLanguage`, so the
  extra-key and length rules T1 and T2 shipped existed and no document could reach them. The edge
  and its eight-row differential landed in `7cb5658`. **This is the failure mode to watch for in
  iteration 6**, whose `design` block is wired the same thin way today.
- **`Languages`' order was verified only transitively**, through the `Locale` `$defs` byte match.
  The order decides the `Phrases__N` numbering — the one thing about the locale `$defs` that cannot
  be read off the output, since the keys sort alphabetically whatever number they carry — so a
  submodule bump that reordered the union would have surfaced as forty-five byte failures naming no
  cause. `TestLanguagesAreInUnionOrder` derives the expectation from the glob.
- **The catalog drift check shares a parser with the tool that wrote the data.** `tools/localeprobe`
  and `catalogs_conformance_test.go` both read the YAML with `goccy/go-yaml` and the same struct
  tags, so a goccy defect would land in the data and the expectation alike. The verifier closed it
  out of band with ruamel — 0 differences, field by field, all 21 files — and the tool's head
  comment now states the gap. Nothing in the repo repeats that check.

**One process failure, recorded rather than rewritten:** `b7531dc` bundles T1, T2 and T5 in one
commit, against `AGENTS.md` §7. `tasks.md` lists all three as separate units.

The parity suite stays at its 42 red cases, which is iteration 1's baseline. No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Carried forward:** date formatting is **not** this iteration's despite the row title it used to
carry — spec §4.1 assigns `2020-09` → `Sept 2020` and §4.2 assigns `degree_with_area`'s
substitution to iteration 9, with the renderer.

## Iteration 9 — verification

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL** with two
blockers, one major and seven minor findings. Every one of the four it ranked as blocking or major
is fixed and pinned by a fixture that is red without the fix. The first verifier run for this
iteration was cut off by a session limit; the second completed.

| # | Verifier finding | Resolution |
|---|---|---|
| 1 | **blocker.** `design.templates.education_entry.degree_column: null` was ignored — the port emitted the declared `**DEGREE**` where upstream omits the column. The bridge dropped every null-valued key before the merge, so an explicit null could never override a default. Reachable from a twelve-line document. | `51a4121`. A null now survives into the merge, and `design.Effective` resolves it by the field's declared type: kept for the one `str \| None`-with-default field, restored to the default elsewhere (which is the case upstream rejects at validation). Pinned by `design_null_column`. |
| 2 | **blocker.** A document with `cv.photo` rendered **silently wrong** — 157 bytes of upstream's header `#grid` missing, exit 0, no warning. The photo was hardcoded falsy on the reasoning that the corpus has none. All 16 corpus files that mention `photo:` write it null. | `c3657bd`. A *local* photo needs no download and now renders; a *URL* one returns `typstdoc.ErrPhotoDownloadUnsupported` rather than a document. Pinned by `header_photo` and a unit test. |
| 3 | **major.** `splitLines` split on `\n` alone. Python's `str.splitlines()` breaks on eight more characters, so a `summary` with Windows line endings left a bare `\r` inside the rendered Typst and a ` ` produced one line where upstream produces two. | `e7a9299`. Full CPython boundary set, `\r\n` counted once, checked against a twenty-case fixture CPython itself wrote. |
| 4 | **major.** Seven of nine `locale.Resolve` override branches were reached by **no test in the repo**. The verifier measured them correct out of band; nothing pinned them. | `d1c1290`. One fixture case exercises all nine; eight of the nine now fail if their branch is deleted. `phrases` stays covered by its unit test only — the classic theme has no template that shows it. |
| 9 | minor. `spec.md` §6's boxes were unchecked while `tasks.md` said done. | `9733253`. |
| 10 | minor. `tools/typprobe`'s comment claimed only `cli_*`/`err_*` cases lack an input; three others do. | `9733253`, which now names them. |

**Findings 5, 6, 7 and 8 stand as recorded, unfixed, on purpose:**

- **6 and 7** are coverage observations, not defects: paths reached by a unit test but not the
  corpus (custom connections, HttpUrl normalization, `current_date` parsing — all differentially
  verified correct), and paths reachable by no document at all (`dumpValue`'s float, bool and
  mapping arms). The dead arms stay because dropping them would make `Dump` partial on input the
  *binder* admits even though upstream's own pipeline crashes on it — see finding 5.
- **8** is a commit-discipline slip already in history: `60f9daa` bundles a lint refactor of
  `tools/typprobe` with the colour fix. Recorded rather than rewritten, as `AGENTS.md` §7 breaches
  have been throughout.

## Iteration 11 — both documents, and a cut that should not have happened

The Markdown half needed **no new pipeline**: upstream's `render_full_template` takes `file_type`
as a parameter, so `internal/renderer/typstdoc` became `internal/renderer/document` and `Render`
gained a format. Three branches cover every difference — template directory, processor chain, and
whether a preamble exists.

**The HTML half was cut and then uncut in the same session, and the cut is the part worth
recording.** The measurement spec §6 demanded was run: goldmark matched python-markdown on 8 of 24
documents. From the first differing line of two cases this ledger concluded the misses were "loose
versus tight lists", called that a block-layer difference no post-pass could fix, and scoped a
python-markdown block-layer port as its own iteration.

All 16 misses had **one** cause: python-markdown nests a list item at `tab_length` 4 where
CommonMark nests at 2, and the entry templates emit nested highlights at 2. Normalizing that in the
*input* makes goldmark match 24 of 24. The difference between "its own iteration" and "one rule"
was reducing the diff instead of reading its first line.

That is the third time in this port that *only a bigger port can fix this* proved false. Spec 008
§8 said only a corpus `.typ` could check the template transform; a fragment differential found a
real bug in its first run. The pattern is worth naming: **an estimate of how hard something is
belongs in the same evidence class as a parity claim — measured, not asserted.**

Mutation-checked: without the list-indent rule 16 of 24 fail, with a tab length of 2 the same 16
fail, and keeping goldmark's trailing newline fails all 24.

**One upstream oddity reproduced rather than fixed**: `Full.html` interpolates a `title` that
`render_html` never binds, so every `.html` has an empty `<title>`. The port binds nothing there
either.

**Not verified by a fresh context.** Iteration 11 has had no `rendercv-parity-verifier` pass; the
row above reports what the suite prints, not an independent audit.

## The golden corpus expires daily — HUMAN GATE

**Found while wiring `render`, and it blocks 18 of the 42 parity cases regardless of how correct
the port is.**

`render_typst_only` now matches its golden on **exit code, stdout, stderr and file list**, and its
`.typ` differs on exactly one line:

```
golden:   day: 6,
got:      day: 7,
```

The corpus inputs write `settings.current_date: today`. Upstream resolved that to its *generation*
day — 2026-08-06 — and baked it into the preamble's `datetime(...)` and the top note. 18 golden
`.typ` files embed a date this way. **They can only pass on the day they were generated.**

This is a defect in `tools/gengolden`, not in the port: the corpus was captured without pinning the
date, exactly as `tools/docprobe` learned to do later (it passes
`--settings.current_date 2025-03-05`, which is why the 72 document comparisons are reproducible).

**The fix needs the human gate**, because it regenerates `testdata/golden/` and so changes the
contract (`AGENTS.md` §5):

1. `tools/gengolden` passes `--settings.current_date` with a fixed date, as `docprobe` does;
2. `just golden` regenerates;
3. the 18 affected cases become reproducible on any day.

Until then, `render`'s correctness is measurable only through the document differential of
iterations 9 and 11 — which is byte-exact and date-pinned, and which does pass.

## `new`'s eight cases collide with the sanctioned binary name — HUMAN GATE

`new` is implemented and its starter CVs are **byte-identical**: `tools/sampleprobe` captures them
from the vendored CLI and all seven variants `cmp` clean against their goldens. The two panels and
the greeting match too.

**One line cannot match, and it is the sanctioned divergence itself.** `new` prints

```
│   2. Run: rendercv render John_Doe_CV.yaml                                   │
```

The port must tell the user to run `rendercv-go` — that is `AGENTS.md` §1's one permitted
deviation, and printing `rendercv` would be an instruction that does not work. But the row is
inside a **fixed-width panel**, so the longer name changes the padding: substituting either
direction in the harness leaves the line three characters short or long.

So the eight `new_*` cases are byte-identical everywhere except a line that *must* differ. The
resolution is a human call between three options:

1. a `divergences.md` entry recording the line, leaving the cases red;
2. a harness rule that re-pads panel rows after substituting the binary name — narrow, and it
   weakens the width check on exactly the rows it touches;
3. accepting `rendercv` in output text as a brand rather than a command, which contradicts the
   instruction's purpose.

I did not choose. Option 2 was implemented, measured to be wrong by three characters, and reverted.

## `create-theme` cannot be byte-identical — HUMAN GATE

Measured while scoping iteration 12's remaining commands. `create-theme` writes fourteen files, and
**two kinds of them are things this port deliberately does not have**:

- `__init__.py` is Python that a custom theme executes at validation time. That is already D-002 —
  the port uses a sandboxed Lua `init.lua` instead — so the file written here must be `init.lua`,
  or D-002's feature does not work at all.
- The `.j2.typ` files are Jinja. The port ships their pongo2 transform, which is what its loader
  reads. Measured on `Header.j2.typ`: upstream's carries a newline after `{% macro image() %}`
  that Jinja's `trim_blocks` eats at parse time and the transform has already removed. **Writing
  upstream's bytes would hand the user a theme this binary renders differently from the one it just
  wrote.**

`AGENTS.md` §6.1 already sanctions template *source* diverging while output must not — but the
`create_theme` golden compares source, so the case is unreachable by construction rather than by
omission. It needs a `divergences.md` entry naming both files. Recorded, not written.

## Two measured behaviors awaiting the human gate

Neither is written into `specs/divergences.md`; that file is human-gated (`AGENTS.md` §5) and this
is the request.

1. **A non-string extra key on an entry.** `flag: true` or `ratio: 3.5` makes upstream die with an
   uncaught `TypeError: sequence item 3: expected str instance, bool found` (exit 2). The port
   exits 0 and renders, dropping the placeholder. Measured by the verifier.
2. **A `design` block that omits `theme`.** Upstream dies with an uncaught `KeyError: 'theme'`
   (ruamel `comments.py:854`, exit 2); the port defaults to `classic` and renders. Measured while
   building the locale fixture case.

Both are upstream *crashes*, not upstream behavior, and in both the port is friendlier. That is
still a divergence under axis 2 — same input, different exit code — and it is the human's call
whether to reproduce the crash, record the divergence, or leave it.

## Log

| Date | Event |
|---|---|
| 2026-08-06 | Repo bootstrapped; upstream pinned at v2.8; parity contract written. |
| 2026-08-06 | Iteration 1 green: 42-case corpus, gengolden, 351 golden files, red parity suite. |
| 2026-08-06 | Iteration 2 green with cut scope: reader, binder, overlay merge, cv, entry bases, sections. Conformance suite unchanged (42 red by design). Six items carried to iteration 3. |
| 2026-08-06 | Parity bug found and fixed: section-title capitalization used `unicode.ToTitle`, which is rune-to-rune and cannot express Python's `str.capitalize()` (`ßeta` → `Sseta`, `ﬁle` → `File`). The failing rows had been dropped from the test table rather than reported. |
| 2026-08-06 | Iteration 3 (entry types) started: spec investigation kicked off. |
| 2026-08-06 | Iteration 3 green with cut scope: nine entry types, registry in union order, dispatcher, real entry validator, `models.Validate` -> `cv.Validate`. Iteration 2 carried items 2 and 6 closed. Conformance unchanged at 42 red by design. |
| 2026-08-06 | Verifier returned FAIL on iteration 3 with three blockers: the entry-problems wrapper carried `rendercv_other_error` instead of `rendercv_entry_validation_error`, exact-date failures carried `value_error` instead of `rendercv_other_error`, and entry error ordering did not interleave base and own fields. The first two were fixed (`9ddd896`); the third is cut to iteration 4. Three tests had asserted the port's codes rather than upstream's, which is why the suite was green -- the reason the verifier is never the agent that wrote the code. |
| 2026-08-06 | Iteration 3 re-verified by a second fresh verifier: both code fixes confirmed correct and complete against the vendored Python and mutation-tested, no blockers. Seven findings cleared or cut (`c911d27`) — three tests were still asserting Go constants or bare non-emptiness where a literal upstream value was needed, one §8 criterion was a tautology and is now cut, and two stale claims in spec 003 are corrected in the spec text itself rather than only in this ledger. |
| 2026-08-06 | Iteration 4 (validation-error parity) started. Findings: the trailing period at `pydantic_error_handling.py:94-95` is unconditional, so every message iteration 3 emits is one character short; dictionary lookup is substring-with-break, not equality; `end_date` errors end in `!.` because upstream's literal ends in `!` and the period rule appends anyway, which also makes `error_dictionary.yaml`'s own `end_date` row dead code. Coordinate columns are never user-visible (`progress_panel.py:14-36` discards them in both code paths and no machine-readable error mode exists), which resolves iteration 2 carried item 1 as internal-only. Validation errors go to stdout with an empty stderr, which our Go side currently reverses -- an iteration-12 bug with a golden already pinning it. |
| 2026-08-07 | Iteration 7 green. The 45 locale `$defs` land byte-identical, taking the port from 18 of 227 to 63 and Axis 3 from "blocked on 6–7" to "blocked on 6". |
| 2026-08-07 | Two locale behaviors found by measuring rather than reading, both fixed inside the iteration: the twelve-element month bound is `EnglishLocale`'s alone, so applying it to every member rejected documents upstream accepts; and a null `language` is a tag failure rather than an absence. |
| 2026-08-07 | Verifier returned FAIL on iteration 7 with three findings, all closed: `ValidateCatalog` was unreachable from `rendercvmodel.go`, so the rules T1 and T2 shipped could not be reached by any document; `Languages`' order was pinned only transitively through the `$defs` bytes; and the catalog drift check shares a YAML parser with the tool that generated the data it checks. The `design` block is wired the same thin way today — iteration 6 must not repeat it. |
| 2026-08-07 | Iteration 6 green with cut scope: the 161 design `$defs` plus the three settings ones. **Axis 3 closed** — all 227 byte-identical, `just schema-diff` exits 0. |
| 2026-08-07 | Verifier returned FAIL on iteration 6 with twelve findings, two blockers, all closed or cut. The pipeline was deleting the second element of every `design` and `locale` location, which the port never produces: `design.colors.body` became `design.body` and reached the user as an internal error. Unreachable until this iteration emitted the first non-`theme` location, and invisible to tests that stop at `models.Validate`. |
| 2026-08-07 | Iteration 14 added to the table: the D-002 Lua custom-theme path, moved out of iteration 6 by its plan §7. |
| 2026-08-07 | Iteration 8's spec completed: all seven `templater/` modules and the 25 templates measured. Two findings that reach the plan — upstream deregisters five Markdown block processors on the Typst path, so no Go Markdown library works as-is; and the template vocabulary is seven tags, five filters and two Python methods over 32 `splitlines()` sites, so the pongo2 transform has to be mechanical. |
| 2026-08-07 | Iteration 8 Wave A: nine of ten processors landed, 195 measured subtests. The markdown→Typst parser replaces goldmark on that path — upstream deregisters five block processors, and `hr` and `indent` are **not** among them, so `---` renders as nothing and a four-space line is a code block. 101 differential rows are pinned in testdata. |
| 2026-08-07 | A human gate I invented was withdrawn. Iteration 8's T1 claimed the parser choice needed a `divergences.md` entry; that file is scoped to deviations from upstream and every entry names what the user notices, which here is nothing. Upstream uses python-markdown, so goldmark was never its choice either — picking between two Go libraries is a `plan.md` decision. The false gate blocked the iteration for two turns. |
| 2026-08-07 | Iteration 8 green with cut scope: ten processors, the pongo2 engine, the template transform, and a 52-fragment differential against Jinja. Wave C's corpus cases move to iteration 9 with iteration 6's T10. |
| 2026-08-07 | Verifier returned FAIL on iteration 8 with three blockers, all fixed. The one that matters: I had argued in spec §8 that the transform could only be checked by a corpus `.typ`, which is false for fragments — and that argument is what hid a trailing-newline bug adding a blank line to every entry and section of every artifact. |
| 2026-08-07 | Open for the human gate: five measured `markdown_to_typst` divergences — a dropped image, raw HTML, an autolink, a link title and a doubled backtick — all reachable from ordinary CV text. Unlike the parser-choice gate I invented and withdrew, these are user-visible. |
| 2026-08-07 | Iteration 9 opened by closing iteration 8's debt: `process_date` and `render_entry_templates`, both measured against upstream on a validated `EducationEntry`. The orchestrator is what made the other nine processors reachable — before it, nothing expanded a theme template. |
| 2026-08-07 | **The parity suite has its first green case: `cli_version`.** 41 red, down from the 42 that iteration 1 established as the baseline. `--version` prints upstream's version and no binary name, so it is the one CLI output the sanctioned divergence does not touch. |
| 2026-08-07 | **`new` wired.** `tools/sampleprobe` captures the starter CV per theme and locale from the vendored CLI; all seven variants are byte-identical against their goldens, as are both panels and the greeting. The eight cases still fail on one line — the `rendercv render …` instruction, which must name this binary and so changes a fixed-width panel row's padding. Recorded for the human gate. |
| 2026-08-07 | **Iteration 12 started.** `render` is wired end to end: overlays, dotted overrides, path placeholders, the five negative and five path flags, and Rich's result panel — whose geometry was recovered from the goldens, including a duration column the harness erases. `render_typst_only` matches on exit code, stdout, stderr and file list, and differs only on the baked generation date. |
| 2026-08-07 | **Corpus defect found: the goldens expire daily.** 18 `.typ` goldens embed the day they were generated because `gengolden` never pinned `settings.current_date`. Recorded for the human gate; it blocks those cases independently of the port. |
| 2026-08-07 | **Iteration 11 green** (unverified by a fresh context). Both text documents byte-identical on all 24 cases. The HTML was cut and uncut in the same session: the 16 goldmark misses were not "block-layer list structure" but one list-indent rule — python-markdown nests at 4 spaces, CommonMark at 2 — and normalizing the input makes goldmark match 24/24. |
| 2026-08-07 | **Iteration 9 green.** The fresh-context verifier returned FAIL with two blockers (a null `degree_column` ignored; a photo rendering silently wrong), one major (`splitLines` was not `str.splitlines()`) and one coverage hole (seven unpinned `locale.Resolve` branches). All four fixed, each behind a fixture that is red without its fix. 24/24 `.typ` byte-identical. Two upstream *crashes* the port does not reproduce are recorded for the human gate. |
| 2026-08-07 | **Axis 1's first passing cases.** The bridge (`internal/renderer/bridge`) and the orchestration (`internal/renderer/typstdoc`) landed, and all 21 corpus inputs that carry a `cv.yaml` render a `.typ` byte-identical to the vendored Python's, pinned to `settings.current_date: 2025-03-05` by `tools/typprobe`. All nine entry types are covered; the fixture is mutation-checked (19 of 21 fail on a one-newline change to `Assemble`). |
| 2026-08-07 | Iteration 6's T10 closed in iteration 9: `design.Effective` merges the base tree, the theme's overrides and the document's own block, deep at every layer, and runs the two coercions where upstream's validators do. Seven-document differential against upstream's resolved model. |
