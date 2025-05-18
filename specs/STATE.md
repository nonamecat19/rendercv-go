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
| 5 | JSON Schema generator | — | — | 0 / 1 |
| 6 | Design & themes (9) + Lua-scripted custom themes (D-002) | — | — | 0 / 9 |
| 7 | Locale (English + 21 catalogs) + date formatting | — | — | 0 / 22 |
| 8 | Templater (pongo2 env, filters, markdown→typst, processors) | — | — | 0 |
| 9 | Typst renderer (`.typ` emission) | — | — | 0 / 18 |
| 10 | wazero + WASI typst → PDF, then PNG | — | — | 0 |
| 11 | Markdown + HTML renderers | — | — | 0 / 4 |
| 12 | CLI (`new`, `render`, `create-theme`, overrides, watcher) | — | — | 0 |
| 13 | Parity closeout (sample generator, version, error handler, packaging) | — | — | 0 |

## Parity axes

| Axis | Gate command | Status |
|---|---|---|
| 1 — artifacts byte-identical | `just test-parity` | measurable, 0/15 cases passing |
| 2 — CLI surface | `just test-parity` | measurable, 0/20 cases passing |
| 3 — JSON Schema | `just test-parity`, `just schema-diff` | measurable, failing |
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
