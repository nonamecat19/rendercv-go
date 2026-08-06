# Tasks 003 — Entry types

22 commits. Each leaves `go build ./... && go test ./...` green (`AGENTS.md` §7). The one task
that must land red does so behind `//go:build conformance`, so the untagged suite stays green while
`go test -tags conformance ./...` shows the red (as in tasks 002 T8).

**No golden refresh.** There is deliberately no `just golden` task and no human gate for one: all
nine entry types are already exercised by the existing 42-case corpus, so a regeneration would
change the contract for no coverage gain (spec §7.1, §9). Nothing here writes to
`testdata/golden/`. The combinations the corpus misses (spec §5.1) are covered by the differential
unit tests of spec §7.4, which are **not** hand-written goldens — a porter unsure of the
distinction should read spec §7.4 before writing the test.

**Marks.** `[parallel]` tasks within the same wave read none of each other's output and write no
shared file, so they may be fanned out to porters. `[sequential]` tasks are on the pipeline spine
and stay with one owner (`AGENTS.md` §5, the stop rule). Waves are strictly ordered.

Every task cites the spec section it implements. A task is done when its spec sections'
acceptance criteria (spec §8) pass.

---

## Wave A — the package extraction

Pure moves. No behavior change, no new tests, no edits beyond package clauses and imports. They
come first so a bisect separates the refactor from the feature (plan §7 hazard 3).

### T1 — move `ValidationContext` to `models/valctx` · `[sequential]`
`internal/schema/models/validationcontext.go` → `internal/schema/models/valctx/valctx.go`,
test with it. Update the three references: `models/path.go`, `models/rendercvmodel.go`,
`models/cv/cv.go` (`Options.Context`).
Spec §3.19 behavior 44. Plan §2.1, §2.2.
Commit type `refactor:`. `just check` clean; no test file gains or loses a case.

### T2 — move the path types to `models/inputpath` · `[sequential]`
`internal/schema/models/path.go` → `internal/schema/models/inputpath/inputpath.go`, test with it.
Update `models/cv/customconnection.go`'s two references. After this commit `models/cv` no longer
imports `models` — assert that with a `go list -deps` check or a lint rule, not by eye.
Spec §3.19 behavior 44. Plan §2.2.
Commit type `refactor:`.

---

## Wave B — shared prerequisites

### T3 — binder value types · `[sequential]`
`internal/schema/binder/binder.go`. Add `ValueType` with `ValueAny`, `ValueString`,
`ValueStringList`; add `Field.Value`; apply the eight-row check table of plan §4 after presence
resolution and before the missing-field pass. `ValueAny` is the zero value, so every existing
`Field` literal is unaffected.
Spec §3.13 behaviors 24–25, §4.4, §4.5, §5.7, §5.9, §5.10. Plan §4.
Tests: each row of plan §4's table; a required field written null gives §4.4 not §4.3; each of
`KindInt`, `KindFloat`, `KindBool`, `KindMapping`, `KindSequence` in a `ValueString` field gives
§4.4; a scalar in a `ValueStringList` field gives §4.5; a two-element bad list gives two §4.4
records located at `…0` and `…1` as decimal strings.
`TODO(iteration-4)` at the two message constants naming spec 002 §7.3.

### T4 — type the inherited complex fields · `[sequential]`
`internal/schema/models/cv/entries/bases/complexfieldsentry.go`. `location` and `summary` become
`ValueString`, `highlights` becomes `ValueStringList`. Closes the iteration-2 gap of spec §3.13
behavior 25; changes no field order and no date handling.
Spec §3.13 behaviors 24–26.
Tests: a mapping in `summary` gives §4.4; a scalar in `highlights` gives §4.5; a non-text
`highlights` element gives §4.4 at its index; `location: 2020` (a `KindInt` after iteration 2's
resolver) gives §4.4.

### T5 — generated field-order fixture + probe tool · `[parallel]`
`tools/entryprobe/` (a `just` target running the vendored Python) and
`internal/schema/models/cv/entries/testdata/field_orders.json`. The probe dumps, for each of the
eight models, `model_fields.keys()` as a list, plus the characteristic-field table from
`section.py:77`, plus `available_entry_type_names`. **Generated, never hand-written**
(`AGENTS.md` §10.1). Data only — no Go test in this commit.
Spec §3.2, §3.9, §3.10, §3.16 behavior 34, §6.1.

### T6 — registry skeleton + order test, red · `[sequential]`
`internal/schema/models/cv/entries/default.go`: `Default()` returning `NewRegistry()` — **empty**
— with a comment naming T17 as the task that fills it.
`internal/schema/models/cv/entries/default_conformance_test.go`, `//go:build conformance`: diff
`Default().Descriptors()` against T5's fixture — names in union order, and each descriptor's
`Fields` positionally — and diff `Default().Characteristic()` against the fixture's table. Lands
**red** and stays red until T17.
Spec §3.1 behaviors 2–3, §3.2, §3.16, §6.1, §6.2. Plan §3.3.
Nothing untagged calls `Default()` yet, so `go test ./...` stays green.

### T7 — the `doi` pattern matcher · `[parallel]`
`internal/schema/models/cv/entries/doipattern.go`: `matchDOIPattern`, hand-written per plan §5
with the Rust-`regex` `\w` class. **Must not use `regexp` with the literal `\b`.**
Spec §3.11 behaviors 17–20, §5.1. Plan §1, §5.
Tests: spec §3.11 behavior 19's sixteen rows verbatim as a table test. `ü10.5` and `ß10.5` reject;
`①10.5`, `-10.5`, `.10.5`, `prefix 10.5/x`, `\t10.5`, `10.5\n` accept; `10`, `notadoi`,
`abc10.5x`, `9910.5`, `_10.5` reject.
A second test asserts `regexp.MustCompile(`\b10\..*`)` disagrees on `ü10.5` — it documents *why*
the hand-written matcher exists and fails loudly if someone replaces it.

---

## Wave C — the nine entry types · all `[parallel]`

Each task adds **one new file plus its test**, appends nothing to a shared file, and reads only
Wave A/B output. That is what makes them genuinely parallel; the registry and dispatcher edits
that would collide are deliberately deferred to T17 and T18.

Each task's own tests are, at minimum:

- the descriptor's `Fields` in the exact order spec gives, asserted positionally;
- the type's conftest fixture (`tests/schema/models/cv/conftest.py`) validating with zero errors,
  using the fixture's exact bytes;
- `extra_attribute: "extra value"` retained and readable;
- each required own field missing → §4.3, in declaration order;
- each required own field written null → §4.4;
- none of `main_column`, `date_and_location_column`, `degree_column` declared.

### T8 — `BulletEntry` · `[parallel]`
`entries/bullet.go`. One required text field `bullet`; base `BaseEntry`; no date fields.
Spec §3.4, §5.19. Plan §3.1.

### T9 — `NumberedEntry` · `[parallel]`
`entries/numbered.go`. One required text field `number`; base `BaseEntry`.
Spec §3.5, §5.19.

### T10 — `ReversedNumberedEntry` · `[parallel]`
`entries/reversednumbered.go`. One required text field `reversed_number`; base `BaseEntry`. Its
description string (spec §4.7) is carried as metadata for iteration 5 and asserted verbatim.
Spec §3.6, §4.7, §5.19.

### T11 — `OneLineEntry` · `[parallel]`
`entries/oneline.go`. Two required text fields, order `label`, `details`; base `BaseEntry`.
Spec §3.3, §5.19.

### T12 — `NormalEntry` · `[parallel]`
`entries/normal.go`. Own field `name` (required text) then the six inherited; order
`name, date, start_date, end_date, location, summary, highlights`.
Spec §3.7, §5.21, §5.25. Plan §3.1.
Extra tests: spec 002 §5.23's date rejection and acceptance tables reached through this concrete
type (spec §5.21) — at least `start_date: aaa`, `start_date: 2023-01-01`/`end_date: 2021-01-01`,
`date: 2020-20-20`, and the four accepting forms. Plus a non-blank `summary` retained verbatim
(spec §5.25).

### T13 — `ExperienceEntry` · `[parallel]`
`entries/experience.go`. Own fields `company`, `position` then the six; order per spec §3.8.
Spec §3.8, §5.14, §5.23, §5.25.
Extra tests: `{company: …, location: …, date: "No."}` reports only the missing `position`
(spec §5.14). Plus the two combinations no golden covers: a real bare `date` alongside
`start_date`/`end_date` reaching spec 002 §3.77 step 1, and a non-blank `summary`
(spec §5.23, §5.25, §7.4).

### T14 — `EducationEntry` · `[parallel]`
`entries/education.go`. Own fields `institution`, `area`, `degree` (`degree` optional) then the
six; order per spec §3.9.
Spec §3.9, §4.8, §5.8, §5.18, §5.22, §5.23, §5.25.
Extra tests: `{}` reports `institution` then `area`, in that order (spec §5.8); a `degree`-less
entry is valid (spec §5.18) and reports `degree` **absent**, not empty text — the branch no golden
covers (spec §5.22, §7.4). Plus a real bare `date` alongside `start_date`/`end_date` reaching
spec 002 §3.77 step 1, and a non-blank `summary` (spec §5.23, §5.25).

### T15 — `PublicationEntry` · `[parallel]` (reads T7)
`entries/publication.go`. Six own fields in order `title, authors, summary, doi, url, journal`,
then `date` **last**; base `BaseEntryWithDate`; no `start_date`, `end_date`, `location`,
`highlights`. The three model-level rules of plan §6. `url` bound as `ValueAny` with a
`TODO(iteration-4)` naming spec §7.3.
Spec §3.10, §3.11, §3.12, §4.1, §4.2, §4.6, §4.9–§4.13, §5.2–§5.6, §5.24, §5.25. Plan §6.
**The riskiest task in the iteration for a reason unrelated to its size:** no golden case sets
`url` at all and none omits `doi`, so `ignore_url_if_doi_is_given` and `validate_doi_url` are
covered by this task's unit tests and by nothing else, ever (spec §5.24, plan §7 hazard 5). Spec
§8's four-state doi/url table is mandatory, using upstream's own two literals from
`tests/renderer/conftest.py:282-283`.
Extra tests: the `doi` rejection produces §4.1 with the pattern text verbatim; `doi_url` for
`10.1109/TASC.2023.3340648` and for absent (spec §5.3); `doi_url` byte-preserving for
`10. spaced ?`, `10.###`, `10.5\n`; `doi = "10." + 2100×"a"` → §4.2 with an **empty** schema
location; `{doi, url}` → `url` absent, no error; `authors: "scalar"` → §4.3 on `title` **then**
§4.5 on `authors`, in that order — declaration order, as spec §5.10 says and as T3 measured
against the vendored Python (an earlier draft of this line had the two reversed); `authors: [1, 2]` → two §4.4 at indices 0 and 1; `start_date: not-a-date` accepted
and retained with no error (spec §5.6); a non-blank `summary` retained verbatim (spec §5.25);
`doi_url` absent from the field order.

### T16 — `TextEntry` · `[parallel]`
No new model — spec §3.14 and plan §3.2 forbid one. This task adds the tests that pin the ninth
type's surface and the shared name transform:
`entries/textentry_test.go` (or the equivalent in `cv`) asserting a string node validates with
zero errors; the name is the literal `TextEntry`; the section model name is
`SectionWithTextEntries` (spec §3.15 behavior 31); and the nine snake-case names of spec §3.17
behavior 40 as a table test.
Spec §3.14, §3.15, §3.17 behavior 40, §4 (no strings of its own).
If this task finds itself adding a struct, it has misread the spec.

---

## Wave D — registry, dispatch and wiring · all `[sequential]`

One owner. Every task here reads the previous one's output.

### T17 — populate the registry in union order · `[sequential]`
`entries/default.go`: `Default()` returns the eight descriptors listed **literally** in the union
order of spec §3.1 behavior 2. Turns T6's conformance test green. Deletes iteration 2's fixture
registry (`entries/registry_test.go`'s eight hand-written descriptors) and repoints every
iteration-2 section test at `Default()` — **with no other edit to those tests** (spec §3.16
behavior 35).
Spec §3.1, §3.16, §6.2. Plan §3.3, §7.2 of the spec.
Tests: order asserted positionally, not as a set; the characteristic table equals spec §3.16
behavior 34 exactly and the common set is exactly the six names; `numbered_entry` and
`reversed_numbered_entry` discriminate to their own types (spec §8, the two rows upstream's table
omits); `{summary: …}` matches nothing → spec 002 §4.9 (spec §5.20); the seven pairs of
spec §5.15 hold from a raw mapping and from a constructed entry.
No `init()`-based registration (plan §8).

### T18 — the dispatcher · `[sequential]`
`entries/default.go`: `Validate(node, name, location, source, reference)`, a nine-arm switch plus a
default arm raising `schemaerr.InternalError`.
Spec §3.18 behavior 43, §3.19 behavior 45. Plan §3.3.
Tests: every name in `Default().Names()` has an arm (iterate the registry, not a literal list); an
unregistered name yields the internal error; each of the five entry-level codes of spec §3.18
behavior 43 is reachable and distinguishable without matching message text.

### T19 — wire the real entry validator · `[sequential]`
`internal/schema/models/cv/sectionvalidation.go`: `EntryValidator` gains `reference time.Time`,
`entryValidator` defaults to `entries.Validate`, `ValidateSection` gains the reference parameter,
and `cv.validateFields` threads `Options.Context.Today()`. The stub disappears; the
`SetEntryValidatorForTest` seam stays, for tests only.
The reference date itself is existing, already-ported behavior with its own precedence ladder
(spec §3.13 behavior 26a); this task only threads it and must not re-derive it.
Spec §3.13 behavior 26a, §3.19 behavior 45. Plan §3.4.
Tests: a section of invalid entries now produces spec 002 §4.12 with real children; no production
path reaches an accept-everything validator (assert by giving `cv.Validate` a bad entry and
requiring a non-empty error list).

### T20 — `models.Validate` calls `cv.Validate` · `[sequential]`
`internal/schema/models/rendercvmodel.go`. Validate a present `cv` node at location `["cv"]` with
`cv.Options{Registry: entries.Default(), Context: ctx}`; append its errors after the top-level
binder's. Add the typed member `CvModel *cv.Cv` beside the raw node. An absent `cv` is not an
error; a null `cv` reports through the binder's non-mapping branch. Closes carried item 2 of
`specs/STATE.md`.
Spec §3.19 behavior 44, §6.3, §6.4. Plan §2.3.
Tests: a document with a bad section reports the section and entry errors through
`models.Validate`, in document order; `{}` still validates; `cv: null` reports the model-type
error and nothing else.

### T21 — re-assert the section criteria against real types · `[sequential]`
No production code. Rewrites the two iteration-2 tests that used the injected stub so they run
against real types, closing carried item 6 of `specs/STATE.md`:
`[education_entry, experience_entry]` → spec 002 §4.12 naming `EducationEntry`, children reporting
`institution` and `area` missing at entry index `1` and **nothing** about `company`/`position`;
`[{x: 1}, <valid BulletEntry>]` → spec 002 §4.12 naming `BulletEntry` with a child error on entry
`0`.
Plus the nine-section CV of spec §5.12: one section per entry-type name, two conftest fixtures
each, zero errors, nine section records with the right entry types.
Spec §5.12, §5.13, §8 *Wiring*.

### T22 — differential fixture over upstream's wrong input · `[sequential]`
A test reading `tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml` **from the
submodule** and comparing, for the nine `welcome_to_rendercv_tests*` sections, the entry-level
schema locations and error codes against upstream's
`expected_errors.yaml:44-140`. Locations and codes only — the messages in that file are iteration
4's rewrites (spec §9).
Spec §5.10, §5.13, §5.14, §9.
`TODO(iteration-4)` naming spec §9 at the point where messages would be compared.

---

## Fan-out summary

| Wave | Parallel tasks | Owner |
|---|---|---|
| A | none | spine owner |
| B | T5, T7 alongside the T3–T4–T6 spine | one porter for T3/T4/T6, two for T5/T7 |
| C | T8, T9, T10, T11, T12, T13, T14, T15, T16 | nine porters |
| D | none | spine owner |

Wave C is the iteration's fan-out and the reason `AGENTS.md` §5's "delete fake edges" rule applies
here: the nine types never read each other. T15 is the largest and riskiest of the nine (plan §6)
and should go to the most capable porter; T7 must be green before it starts.

T3, T4, T6 and all of Wave D are the spine and must not be split across agents.

## Exit

Iteration 3 is done when:

1. Spec §8's checkboxes all pass under `go test ./...`.
2. `just check` is clean.
3. `rendercv-parity-verifier` confirms in a **fresh context** that
   `go test -tags conformance ./...` shows exactly the 42 cases red that iteration 1 established —
   no new failures, and T6's fixture test green (spec §7.1). A red T6 at this point means the
   registry is wrong, not that the corpus is missing.
4. `specs/STATE.md` moves iteration 3 to `green`, records carried items 2 and 6 of iteration 2 as
   closed, and re-states items 1, 3, 4 and 5 as iteration 4's inputs (spec §7).
5. The `PublicationEntry.url` normalization risk of spec §5.5 is carried into iteration 4's spec as
   an open item, **not** written into `specs/divergences.md` by this iteration.
