# Tasks 004 — Validation-error parity

53 commits. Each leaves `go build ./... && go test ./...` green (`AGENTS.md` §7). The two tasks
that must land red do so behind `//go:build conformance`, so the untagged suite stays green while
`go test -tags conformance ./...` shows the red (as in tasks 002 T8 and tasks 003 T6).

**No golden refresh.** There is deliberately no `just golden` task and no human gate for one
(spec §7.3, §9). Nothing here writes to `testdata/golden/`. The two submodule fixtures of spec §9
are read where they live.

**One human gate.** T52 is a stop point: if the YAML-parser-message decision of spec §7.5 lands on
option C, it needs `specs/divergences.md` and human approval (`AGENTS.md` §5). No other task in
this iteration may write to that file.

**Marks.** `[parallel]` tasks within the same wave read none of each other's output and write no
shared file, so they may be fanned out to porters. `[sequential]` tasks are on the pipeline spine
and stay with one owner (`AGENTS.md` §5, the stop rule). Waves are strictly ordered.

Every task cites the spec section it implements. A task is done when its spec sections' acceptance
criteria (spec §8) pass.

---

## Wave A — prerequisites

The dictionary data and the coordinate columns. Both are read by everything downstream, and the
column fixes must precede T50 or the widened differential lands red for the wrong reason
(plan §7 hazard 7).

### T1 — dictionary submodule-diff test, red · `[sequential]`
`internal/schema/errorpipeline/dictionary_conformance_test.go`, `//go:build conformance`. Reads
`third_party/rendercv/src/rendercv/schema/error_dictionary.yaml` with the project's own reader and
asserts the Go `dictionary` slice equals it **key for key, value for value, in file order**, with
equal length. Declares an empty slice in `dictionary.go` so the package compiles. Lands **red** and
stays red until T2.
Spec §3.4 behaviors 11–12, §4.8–§4.11, §4.14, §9. Plan §3.
The test must exercise both traps of plan §3: rows 3 and 4's doubled backslashes, and row 13's
quoted scalars.

### T2 — the thirteen dictionary rows · `[sequential]`
`internal/schema/errorpipeline/dictionary.go`: the rows as an **ordered array**, never a map
(plan §7 hazard 6). Turns T1 green. Add the reachability comment naming the five dead rows.
Spec §3.4 behaviors 12–14. Plan §3.
Tests: order asserted positionally; a substring-containment lookup returning row 5 for
`Input should be a valid URL, relative URL without a base` and row 13 for
`value is not a valid color: string not recognised as a valid color`.

### T3 — coordinate columns for a key with a null or empty value · `[sequential]`
`internal/schema/yamlreader/build.go`. Closes half of `specs/STATE.md` cut-scope item 1: such a key
currently reports column 1 instead of its own indent.
Spec §7.2. Measured baseline in `specs/STATE.md`: 33/232 differing paths on
`examples/John_Doe_ClassicTheme_CV.yaml`, 50/388 on `expected_errors.yaml`.
Tests: the differing paths for this shape now match upstream; the count drops.

### T4 — coordinate columns for a flow-sequence element · `[sequential]`
Same file, the other half: an element currently reports the first value token instead of the `[`.
After this commit **0/232 and 0/388** paths differ.
Spec §7.2.

### T5 — `tools/yamlprobe` generates the coordinate fixture · `[parallel]`
Extend the probe to dump every `(path, coordinates)` pair for the two documents of T3/T4, and
repoint `internal/schema/yamlreader/resolve_test.go` at the generated data. Closes
`specs/STATE.md` cut-scope item 5: the test currently states Go-side expectations.
Spec §7.2. Generated, never hand-written (`AGENTS.md` §10.1).

### T6 — `dealias` no-op regression test over the submodule corpus · `[parallel]`
`internal/schema/yamlreader/noalias_test.go`: for every `*.yaml` in `third_party/rendercv`, assert
the parsed tree is identical with and without the transform. Closes `specs/STATE.md` cut-scope
item 4; the verifier ran this by hand at 64/64 and nothing in the suite guards it.
Spec §7 (carried item 4). No production change.

---

## Wave B — the pipeline spine · all `[sequential]`

One owner. Every task reads the previous one's output. The order of the eleven steps is the
contract (spec §3.2), so they land in that order and each commit adds one step with its test.

### T7 — mark records raw, add `LocationIsFinal` · `[sequential]`
`internal/schema/schemaerr/error.go`: add the boolean of plan §4 step 3 and document that a record
produced by `models/**` is **raw** — its `Message` is pre-dictionary, its `SchemaLocation`
pre-filter — and that `errorpipeline.Parse` is the only thing that finalizes it.
Spec §3.2, §3.17 behavior 65. Plan §2.1, §4 step 3.
No behavior change; existing tests unchanged.

### T8 — `errorpipeline.Parse` and `parseOne`: strip and period · `[sequential]`
`internal/schema/errorpipeline/errorpipeline.go`. `Parse` iterating raw records; `parseOne` with
step 1 (both prefixes, `ReplaceAll`, in upstream's order) and step 8 (the period), and nothing
between them yet. The doc comment on `parseOne` names the three artifact endings `!.`, `.".`,
`)".` with their §4 references, and states that the period statement is last.
Spec §3.2 behavior 4, §3.6, §6.6, §6.7. Plan §4.
Tests: a second occurrence of `Value error, ` in one message is also removed; §3.6's four-row
table for the cases reachable without the other steps.

### T9 — step 2: the `design` / `locale` discriminator skip · `[sequential]`
`internal/schema/errorpipeline/location.go`. Slice on a **copy**, guarded for a short location.
Spec §3.3 behavior 9, behavior 10. Plan §4 step 2.
Tests: §3.3 behavior 9's four-row table including the one-element case; `settings` is untouched; an
empty location does not panic and does not take the branch.

### T10 — step 4: the location filter · `[sequential]`
Same file. The seven substrings as an array in upstream's order, with the comment of plan §4
warning that the filter is **not** dead code in the port.
Spec §3.3 behaviors 5–8. Plan §2.2 consequence 1, §7 hazard 1.
Tests: one row per substring over the five measured synthetic elements of §3.3 behavior 6; §3.3
behavior 7's five-row section-key table asserting the four collapsed locations are equal; a decimal
index survives.

### T11 — steps 5 and 6: the `end_date` and `current_date` overrides · `[sequential]`
Same file or `errorpipeline.go`. Containment, not equality. The `current_date` `date`-suffix strip
is implemented with a comment saying it is inert in the port (plan §4).
Spec §3.5 behaviors 15–19, §4.12, §4.13.
Tests: `end_date: invalid_date` yields §4.12 **before** the dictionary runs; a location ending
`my_end_date` also matches; the strip fires only when both `location[-1] == "date"` and
`location[-2] == "current_date"`; §4.13 comes out with one period and §4.12 with `!.`.

### T12 — step 7: dictionary substitution · `[sequential]`
`errorpipeline.go`. `for` over T2's array, `strings.Contains`, `break`.
Spec §3.4 behaviors 11, 14, §6.5.
Tests: first match wins with two rows that would both match a synthetic message; rows 1, 2, 3, 4
are proven unreachable through the pipeline — an invalid `doi` emerges as
`String should match pattern '\b10\..*'.`, a bad `end_date` as §4.12, `Input should be a valid
integer` as `Input should be a valid integer.`

### T13 — step 3: honour `LocationIsFinal` · `[sequential]`
`errorpipeline.go`. When set, skip steps 2 and 4 for that record.
Spec §3.2 step 3, §3.17 behavior 65. Plan §4 step 3.
Tests: a record pinned at `("design","theme")` keeps it even though `loc[0] == "design"` would
otherwise drop an element; a context-supplied input wins over the raw one.

### T14 — steps 9 and 12: source and coordinate-document selection · `[sequential]`
`errorpipeline.go`. Default `main_yaml_file` and the main document; replaced when overlays were
supplied, the location is non-empty, and `location[0]` is an overlay key.
Spec §3.12 behavior 40. Reuses `schemaerr.OverlayToSource`, already present.
Tests: the three cases of spec §5.14, as unit tests on hand-built records.

### T15 — step 10: coordinate resolution · `[sequential]`
`internal/schema/errorpipeline/coordinates.go`. Wraps iteration 2's resolver. **`location[:-1]`
when the code is exactly `binder.CodeMissing`, the full location otherwise** — compared against the
literal code, not a category.
Spec §3.10 behaviors 34–37, §6.9.
Tests: two missing fields of one entry report identical coordinates; a non-`missing` code with the
same location resolves one level deeper.

### T16 — step 11: rendering the input value · `[sequential]`
`errorpipeline.go`. `...` for `KindMapping` and `KindSequence`; `None` for null; digits for an int.
Spec §3.11 behavior 39, §4.15.
Tests: all four kinds, plus the context-override interaction from T13.

### T17 — the entry-wrapper splice · `[sequential]`
`errorpipeline.go`. Per child of a `CodeEntryValidation` record: drop the first location element,
prepend the wrapper's **raw** location, run `parseOne`, append immediately after the wrapper. One
level only, with a comment forbidding recursion. An empty `Children` slice is the internal failure
of §4.16.
Spec §3.7 behaviors 21–26, §4.16, §6.3. Plan §4.1.
Tests: `("entries", 1, "institution")` under the measured wrapper becomes
`["cv","sections","welcome_to_rendercv_tests_2","1","institution"]`; a child with an empty location
lands at the wrapper's location; a wrapper with no children yields §4.16; a child that is itself a
wrapper is **not** recursed into.

### T18 — deduplication · `[sequential]`
`errorpipeline.go`. Ordered set keyed by the location joined with `\x00`, first occurrence wins.
Spec §3.8 behaviors 27–29, §6.4. Plan §4.1.
Tests: spec §3.8 behavior 28's three-row table; three records at two locations preserve order and
keep the first; a section key containing `.` does not collide with a nested path.

### T19 — the two coordinate-walk internal failures · `[sequential]`
`coordinates.go`: §4.17 for an out-of-range sequence index, §4.18 for an unknown key, both as
`schemaerr.InternalError` and both surfaced through `Parse`'s `error` return.
Spec §3.10 behavior 38, §4.17, §4.18, §5.16.
Tests: upstream's own two substrings, `Index 10 is out of range` and `Key 'nonexistent' not found`
(`tests/schema/test_pydantic_error_handling.py:233-246`).

### T20 — the survivor rules at the three multi-branch sites · `[sequential]`
No new package. Makes the port emit the record upstream's dedup keeps (plan §2.2 consequence 2):
`cv.photo` emits the **path** failure only and no URL failure; the two date sites emit one record
each. Closes the `TODO(iteration-4)` at `internal/schema/models/cv/customconnection.go:96`.
Spec §3.8 behaviors 28–29, §5.3. Plan §2.2, §7 hazard 2.
Tests: the three-row table; `photo: photo_doesnt_exist.jpg` yields exactly one record, with §4.25's
message and not §4.9's.

---

## Wave C — the leaves · all `[parallel]` unless noted

Three library wrappers, eight social-network rules, and the placeholder strings. None reads
another's output; the ones that share a file are marked.

### C1 — the three borrowed-library wrappers

### T21 — `internal/schema/phonenum` · `[parallel]`
New package plus `github.com/nyaruka/phonenumbers` v1.8.1 in `go.mod`. `Validate` returning the
RFC 3966 form with the `tel:` prefix; `Serialize` stripping it.
Spec §3.14 behaviors 47–50, §4.8. Plan §1.1, §5.1.
Tests: spec §3.14 behavior 48's five-row table plus the two extra measured numbers
`+819012345678` → `tel:+81-90-1234-5678` and `+61412345678` → `tel:+61-412-345-678`, so a metadata
bump fails loudly (plan §1.1); `not_a_valid_phone_number` fails; the failure message is the
**dictionary key** `value is not a valid phone number`, not §4.8's replacement.

### T22 — `internal/schema/httpurl` · `[parallel]`
New package plus `github.com/nlnwa/whatwg-url` v0.6.2 in `go.mod`. `Validate` in the four-step
order of plan §5.2, returning one of three codes. `CodeURLTooLong` moves here from
`internal/schema/models/cv/entries/publication.go:30`.
Spec §3.13 behaviors 41–46, §4.9, §4.19, §4.20. Plan §1.2, §5.2.
Tests: spec §3.13 behavior 42's eleven-row normalization table; every measured `url_parsing` reason
producing the dictionary key `Input should be a valid URL` with the library's text discarded;
`ftp://example.com` producing §4.19; the length semantics of behavior 46 including the
420-character input that serializes to 2420 and the malformed-but-long input that reports
`url_too_long`.

### T23 — `internal/schema/emailaddr` · `[parallel]`
New package, no dependency. `Validate` returning the normalized address, with the ordered checks of
plan §5.3 and a distinguishable "unclassified" failure for anything outside the measured set.
Spec §3.15 behaviors 52–57, §4.21, §4.22. Plan §1.3, §5.3, §7 hazard 9.
Tests: spec §3.15 behavior 55's twelve-row message table; behavior 56's accepted inputs;
behavior 54's normalization (domain lowercased, local part not, non-ASCII domain kept);
behavior 53's three wrapper behaviors; and a test asserting **no** input in the tables reaches the
unclassified branch.

### C2 — registration of the wrappers

`internal/schema/models/cv/scalarorlist.go` is shared by T24–T26, so those three are `[sequential]`
among themselves. The rest are parallel.

### T24 — register the phone validator · `[sequential]` (reads T21)
Replace `elementValidators["phone"]`'s pass-through. `cv.SerializePhone` becomes a one-line forward
to `phonenum.Serialize`. Closes the `TODO(iteration-4)` at
`internal/schema/models/cv/scalarorlist.go:94`.
Spec §3.14. Plan §5.1.
Tests: a list-valued `phone` produces one record per bad element at the element's index.

### T25 — register the email validator · `[sequential]` (reads T23)
Replace `elementValidators["email"]`'s pass-through.
Spec §3.15. Partially closes the `TODO(iteration-4)` at `.../scalarorlist.go:14`.

### T26 — register the website validator · `[sequential]` (reads T22)
Replace `elementValidators["website"]`'s pass-through. Closes `.../scalarorlist.go:14`.
Spec §3.13 behavior 41.

### T27 — register `PublicationEntry.url` · `[parallel]` (reads T22)
`internal/schema/models/cv/entries/publication.go`: register one `httpURLValidators` entry and
import `CodeURLTooLong` from `httpurl`. Closes both `TODO(iteration-4)` markers in that file
(`:44` and `:53`). Note spec 003 §5.24: no golden case sets `url`, so this task's unit tests are
the only gate this decision will ever have.
Spec §3.13 behaviors 41, 45, §4.9, §4.19, §4.20, and spec 003 §7.3.

### T28 — register `CustomConnection.url` · `[parallel]` (reads T22)
`internal/schema/models/cv/customconnection.go`: closes the `TODO(iteration-4)` at `:39`.
Spec §3.13 behavior 41.

### T29 — the generated social-network URL is validated, not normalized · `[parallel]` (reads T22)
`internal/schema/models/cv/socialnetwork.go`: build the URL from the seventeen-entry prefix table
(`models/cv/social_network.py:33-50`), validate it, and **discard** the normalized form.
Spec §3.13 behavior 44, §5.8. Plan §5.2.
Tests: a LinkedIn username of `not a valid %%^&*()` produces **no** record and the raw
concatenation survives; the Mastodon URL is built by splitting the handle
(`social_network.py:178-180`).

### C3 — the eight per-network username rules · all `[parallel]`

Each adds one rule and its test to `internal/schema/models/cv/socialnetwork.go`. They touch one
shared file, so if fanned out they must be merged by the spine owner in this order; if run by one
porter they are still eight commits (`AGENTS.md` §7).

Each task asserts: the message verbatim from §4, the code `rendercv_other_error`, the location
`username`, and that the rule is a **full match**, not a search.

### T30 — Mastodon · `[parallel]` — spec §3.16 behavior 59 row 1, §4.1
### T31 — StackOverflow · `[parallel]` — spec §3.16 row 2, §4.2
### T32 — YouTube · `[parallel]` — spec §3.16 row 3, §4.3, §3.16 behavior 61
The emitted form ends `username.".` The stray `"` is upstream's and must not be removed.
### T33 — ORCID · `[parallel]` — spec §3.16 row 4, §4.4
### T34 — IMDB · `[parallel]` — spec §3.16 row 5, §4.5; `nm12345678` must fail (full match)
### T35 — Bluesky · `[parallel]` — spec §3.16 row 6, §4.6, behavior 60's redundant anchors
### T36 — WhatsApp · `[parallel]` (reads T21) — spec §3.16 row 7, §4.7
### T37 — Reddit · `[parallel]` — spec §3.16 row 8, §4.24

### T38 — the username check runs only for a valid network · `[sequential]` (reads T30–T37)
`socialnetwork.go`: the gate of spec §3.16 behavior 58, and the assertion that the nine ruleless
networks accept any username. Closes the `TODO(iteration-4)` at `.../socialnetwork.go:69`.
Spec §3.16 behavior 58.

### T39 — the unknown-network message · `[parallel]`
`socialnetwork.go:123`'s placeholder becomes §4.23, the full seventeen-name enumeration with `or`
before the last. Closes the `TODO(iteration-4)` at `.../socialnetwork.go:86`.
Spec §3.16 behavior 62, §4.23, §3.19 behavior 71.

### C4 — the placeholder strings iterations 2 and 3 left · all `[parallel]`

### T40 — `model_type`'s per-model suffix · `[parallel]`
`internal/schema/binder/binder.go`: `messageModelType` gains the model name. `binder.Spec` carries
it. Closes part of the `TODO(iteration-4)` at `.../binder.go:32`.
Spec §3.19 behaviors 71–72, §4.32.
Tests: `cv: null` and `cv: 5` both give `Input should be a valid dictionary or instance of Cv.`;
the suffix is asserted for `Cv`, `SocialNetwork`, `CustomConnection` and one entry class.

### T41 — confirm the four borrowed binder texts · `[sequential]` (same file as T40)
`binder.go`: remove the `TODO(iteration-4)` at `:32` and `:48` by asserting each of
`Field required`, `Extra inputs are not permitted`, `Input should be a valid string`,
`Input should be a valid list` against the measured raw text, and that the pipeline turns them into
§4.14, §4.10, `Input should be a valid string.` and
`This field should contain a list of items but it doesn't.`
Spec §3.19, §4.10, §4.14, §5.10, §5.12.

### T42 — declared-field errors precede extra-key errors · `[sequential]` (same file)
`binder.go`: reorder so all declared-field failures come first in declaration order, then extra-key
failures in input order. Closes the `TODO(iteration-4)` at `.../binder.go:177`.
Spec §3.9 behavior 32 step 3, §6.2.
Tests: the measured seven-record sequence — `email`, `phone`, `social_networks.0.network`,
`social_networks.0.username`, `social_networks.0.extra_here`, `cv.zzz_extra`, `cv.aaa_extra`.

### T43 — decide the CPython date texts · `[parallel]`
`internal/schema/models/cv/entries/bases/entrywithdate.go` and `entrywithcomplexfields.go`: the
decision spec 002 §7.3 deferred. The three range messages are reproducible verbatim in Go and are
already implemented; this task removes the two `TODO(iteration-4)` markers (`entrywithdate.go:61`,
`entrywithcomplexfields.go:100`) and adds §4.33 for a year outside four digits, replacing the
not-a-valid-date fallback.
Spec §3.19 behavior 71, §4.33, §4.34, and spec 002 §4.13, §5.11.
Tests: `year 0 is out of range`, `month must be in 1..12`, `day is out of range for month` and
`Invalid isoformat string: '10000-01-01'` all reachable and verbatim; the first three then map
through the dictionary to their §4 replacements and the fourth does not.

### T44 — confirm the two path messages · `[parallel]`
`internal/schema/models/inputpath/inputpath.go`: assert §4.25 and §4.26 verbatim and that both
interpolate the path **relative to the resolution base**, not the absolute path.
Spec §3.17 behavior 63, §4.25, §4.26.

### T45 — the minimal `design.theme` slice · `[parallel]`
A new `internal/schema/models/design` shell holding **only** the built-in theme-name set and the
`^[a-z0-9]+$` check, emitting §4.27 with `LocationIsFinal` set and the input re-pinned to the theme
name. Nothing else from `design` (spec §7.9).
Spec §3.17 behaviors 64–65, §4.27, §7.9. Reads T7.
Tests: `theme: not_a_valid_theme` yields one record at `("design","theme")` with §4.27 and input
`not_a_valid_theme`; `theme: classic` yields none; `{theme: classic, nope: 1}` yields
`("design","nope")` with §4.10 (the discriminator element dropped by T9).

### T46 — the minimal `settings.current_date` slice · `[parallel]`
A new `internal/schema/models/settings` shell holding **only** a `current_date` shape check that
fails for a value which is neither `YYYY-MM-DD` nor the literal `today`. Nothing else from
`settings` (spec §7.9).
Spec §3.5 behaviors 18–19, §4.13, §7.9.
Tests: `current_date: todady` yields one record at `("settings","current_date")` which the pipeline
rewrites to §4.13; `current_date: today` and `current_date: 2025-01-01` yield none.

### T47 — the locale discriminator message · `[parallel]`
`internal/schema/models/locale`: **only** the twenty-two language names and the union-tag check
emitting §4.30. The catalogs stay iteration 7's.
Spec §3.17 behavior 67, §4.30.
Tests: `{language: klingon}` yields one record at `("locale",)` with §4.30 verbatim, including
`norwegian_bokmål`; `{language: english, month: 123}` yields `("locale","month")` with
`Input should be a valid string.` after T9 drops the discriminator element.

### T48 — the YAML-syntax producer's shape · `[parallel]`
`internal/schema/modelbuilder/yamlerror.go`: assert everything about the record **except** the
interpolated text, which is T52's — no schema location, 1-indexed coordinates from the parser's
marks, the source of the failing document, §4.15 as input, first-line-stripped-period-appended
interpolation.
Spec §3.18 behaviors 68–70, §4.15, §4.31.
Tests: coordinates absent when the parser supplies no marks; the first-line and period rules.

---

## Wave D — wiring, the differential, and the gate · all `[sequential]`

One owner. Every task reads the previous one's output.

### T49 — the single call site · `[sequential]`
`internal/schema/modelbuilder`: `BuildModel` calling `models.Validate` and then
`errorpipeline.Parse`, wrapping the result in `schemaerr.UserValidationError`.
Spec §2, §3.1 behavior 2. Plan §2.4.
Tests: a syntax failure short-circuits validation and produces one record; a schema failure
produces the parsed list; a test asserts `errorpipeline.Parse` has exactly one non-test caller.

### T50 — widen the differential to all five members · `[sequential]`
`internal/schema/models/wronginput_conformance_test.go`: replace the locations-and-codes comparison
with a **member-by-member, in-order, equal-length** comparison of all 25 records against
`tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`. Removes the
`TODO(iteration-4)` at `:82`. This is upstream's `tests/schema/test_pydantic_error_handling.py:19-54`.
Spec §5.13, §7.2, §8 **[diff]**, §9.
The test lands **green** if Waves A–C are complete; a red here names the failing member and record
index and is a real defect, not a missing corpus case.

### T51 — the overlay differential · `[sequential]`
Same file: the same fixture run with a `design` overlay, asserting `design_yaml_file` for
`design`-rooted records and `main_yaml_file` for the rest, plus the two simpler cases.
Spec §3.12 behavior 40, §5.14, §8 **[diff]**.

### T52 — the YAML-parser-message decision · `[sequential]` · **HUMAN GATE if option C**
`internal/schema/modelbuilder/yamlerror.go:53`. Implement option B of plan §6: map
`goccy/go-yaml`'s error taxonomy onto ruamel's phrasing for the syntax failures the corpus
contains — today exactly one, `while parsing a flow sequence` in
`testdata/golden/err_not_yaml/`. Removes the `TODO(iteration-4)` at `:53`.
Spec §4.31, §7.5. Plan §6.
**Stop here** if the mapping cannot cover the corpus case: that is option C, which needs a
`specs/divergences.md` entry and human approval (`AGENTS.md` §5). This spec does not authorize
writing one.

### T53 — reassign the two upstream crashes · `[sequential]`
No production change. Rewrite the `TODO(iteration-4)` at
`internal/schema/models/cv/sectionvalidation.go:110` and add one at the phone serializer, both
naming iteration 12 and spec §7.8, so neither is orphaned.
Spec §3.14 behavior 51, §5.20, §7.8.

---

## Marker ledger

Every `TODO(iteration-4)` in the tree at the start of this iteration, and the task that clears it.
None is orphaned.

| Marker | Task |
|---|---|
| `models/cv/socialnetwork.go:69` — per-network rules and generated URL | T29–T38 |
| `models/cv/socialnetwork.go:86` — the literal message | T39 |
| `models/cv/customconnection.go:39` — `CustomConnection.url` | T28 |
| `models/cv/customconnection.go:96` — the photo URL branch | T20 |
| `models/cv/entries/publication.go:44` — the two pydantic messages | T27 |
| `models/cv/entries/publication.go:53` — the HTTP-URL seam | T27 |
| `models/wronginput_conformance_test.go:82` — compare messages | T50 |
| `modelbuilder/yamlerror.go:53` — the parser message | T52 |
| `binder/binder.go:32` — missing / extra_forbidden / model_type text | T40, T41 |
| `binder/binder.go:48` — string_type / list_type text | T41 |
| `binder/binder.go:177` — field vs extra-key order | T42 |
| `models/cv/scalarorlist.go:14` — the three real validators | T24, T25, T26 |
| `models/cv/scalarorlist.go:94` — phone normalization | T21, T24 |
| `models/cv/sectionvalidation.go:110` — the non-mapping entry crash | T53 (reassigned to iteration 12) |
| `models/cv/entries/bases/entrywithcomplexfields.go:100` — the isoformat text | T43 |
| `models/cv/entries/bases/entrywithdate.go:61` — the CPython date texts | T43 |

---

## Fan-out summary

| Wave | Parallel tasks | Owner |
|---|---|---|
| A | T5, T6 alongside the T1–T2–T3–T4 spine | one porter for the spine, two for T5/T6 |
| B | none — the eleven steps are the spine | spine owner |
| C | T21, T22, T23 · T27, T28, T29 · T30–T37 · T39, T43, T44, T45, T46, T47, T48 | up to eight porters |
| D | none | spine owner |

Wave C is the iteration's fan-out and the reason `AGENTS.md` §5's "delete fake edges" rule applies:
the eight social networks never read each other, and the three library wrappers are independent
leaves. T22 is the largest and most parity-critical of the three wrappers and should go to the most
capable porter. T24–T26 and T38, T41, T42 share a file with a sibling and are sequential within
their groups.

All of Waves B and D are the spine and must not be split across agents.

## Exit

Iteration 4 is done when:

1. Spec §8's checkboxes all pass under `go test ./...`, including the two **[diff]** criteria.
2. `just check` is clean.
3. `rendercv-parity-verifier` confirms in a **fresh context** that
   `go test -tags conformance ./...` shows exactly the 42 cases red that iteration 1 established —
   no new failures — and that T1's dictionary fixture test is green (spec §7.3). A red T1 means the
   dictionary drifted, not that the corpus is missing.
4. `specs/STATE.md` moves iteration 4 to `green`, records cut-scope items 1, 3, 4 and 5 of
   iteration 2 as closed, records iteration 3's `PublicationEntry.url` open item as closed by
   T22/T27, and carries forward: the email message tail (spec §7.4, iteration 13), the
   `err_unknown_theme` absolute-path golden (spec §7.7, iteration 12), the stdout/stderr inversion
   (spec §7.6, iteration 12), and the two upstream crashes (spec §7.8, iteration 12).
5. If T52 landed on option C, `specs/divergences.md` carries the entry **and** the human approval
   is recorded. If it landed on option B, no divergence exists and none is written.
