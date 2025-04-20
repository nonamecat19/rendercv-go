# Plan 004 — Validation-error parity

Go design for [`spec.md`](spec.md). Behavior claims live there; this file decides code.

---

## 1. Dependency decisions

Three are added. Each was **measured** against the vendored Python before being chosen; the
measurements are in `spec.md` §7.1 and are not repeated here.

### 1.1 `github.com/nyaruka/phonenumbers` v1.8.1 — accepted

A generated Go port of Google's libphonenumber, built from the same metadata the Python
`phonenumbers` package uses. `spec.md` §3.14 needs RFC 3966 formatting, which is not a string
transform: the grouping is derived from per-region metadata (`+34-612-345-678` regroups to
`+34-612-34-56-78`).

Alternatives rejected:

- **A restricted implementation.** Would need the region metadata anyway — that is the whole
  content of the library. There is no small correct subset.
- **`tel:` strip only** (what iteration 2 shipped). Wrong, and Axis-1 gated in two golden cases
  (`spec.md` §3.14 behavior 49), so it fails `just test-parity` the moment the renderer lands.
- **Deferral.** The two carried items it blocks (`cv.phone` formatting, the WhatsApp username
  rule of `spec.md` §4.7) are both this iteration's, and deferring leaves iteration 9 unable to
  render two corpus cases.

**Residual risk:** metadata version skew. The Python side pins `phonenumbers` 9.0.24 and the Go
side pins its own generated metadata. The two golden numbers are ES and US, whose formats have
been stable for a decade, and all seven measured numbers agree today. Mitigation: a table test
holding the seven measured pairs, so a metadata bump that changes any of them fails loudly rather
than silently shifting a `.typ` byte.

### 1.2 `github.com/nlnwa/whatwg-url` v0.6.2 — accepted

`spec.md` §3.13 behavior 42 is the WHATWG URL Standard's serialization, which is what
pydantic-core implements through the Rust `url` crate. Measured: this library reproduces every row,
including `https://ünicode.de/ünï` → `https://xn--nicode-2ya.de/%C3%BCn%C3%AF`, and rejects every
input pydantic rejects.

Alternatives rejected:

- **`net/url` plus `golang.org/x/net/idna`.** Gets the easy rows (trailing slash, scheme and host
  lowercasing, default-port drop) and misses the hard ones: `net/url` does not percent-encode a
  non-ASCII path on output, does not apply IDNA, and its `String()` does not reconstruct an empty
  query as a bare `?`. Every one of those is in behavior 42's measured table. Estimated as more
  code than the wrapper below, with a permanently uncertain tail.
- **A hand-written restricted parser.** Same objection, worse: the WHATWG host parser is a
  specification, not a heuristic.
- **Deferral.** Axis-1 gated: `https://alicechen.dev` → `https://alicechen.dev/` appears in two
  golden `.typ` files.

**Residual risks, both mitigated by a wrapper rather than by using the library raw:**

1. The library accepts `ftp://`; pydantic's `HttpUrl` restricts the scheme. The wrapper applies
   `spec.md` §4.19 after a successful parse.
2. The library reports rich parse-error text. `spec.md` §3.13 behavior 45 shows the reason clause
   is **unobservable** — the dictionary flattens every `url_parsing` message to `spec.md` §4.9 —
   so the wrapper discards the text and emits one code. This is the single largest simplification
   in the iteration and it must be stated in the wrapper's doc comment, or a later reader will try
   to reproduce ruamel-style reason strings.
3. The 2083-character limit is checked on the **input** string **before** parsing
   (`spec.md` §3.13 behavior 46), not on the serialized form. Order matters and is tested.

### 1.3 No library for email — a hand-written syntax checker

No Go port of `email-validator` exists, and the library's syntax module is 822 lines. `spec.md`
§7.4 bounds the scope to a measured message set. The implementation is therefore a hand-written
ordered check sequence in `internal/schema/emailaddr`, whose gate is `spec.md` §3.15 behavior 55's
twelve-row table plus behavior 56's accepted inputs.

`golang.org/x/net/idna` is **not** added: `spec.md` §3.15 behavior 54 measured that the domain is
left in Unicode form, lowercased but not punycoded, so `strings.ToLower` on the domain is the whole
normalization. (This differs from the URL case, where IDNA *is* applied — the two must not be
conflated.)

**Residual risk, explicitly accepted and owned:** an input producing a message outside the twelve
is an open parity risk assigned to iteration 13 (`spec.md` §7.4). No divergence is written.

### 1.4 Nothing else

`golang.org/x/text` is already a dependency. The dictionary is compiled in as Go data (§3), so no
YAML read happens at runtime and `goccy/go-yaml` gains no new caller.

---

## 2. The architectural change: raw records, then one transform

### 2.1 Why the port needs the same two-stage shape

Iterations 2 and 3 have validators write `Message` directly into
`schemaerr.ValidationError`. That cannot produce `spec.md`'s output, because four of its rules are
**global**: the location filter (§3.3), the `end_date` override (§3.5), deduplication by location
(§3.8), and the trailing-period rule (§3.6) which must run after every other message step. A
validator cannot know that its sibling produced the same location.

So the port adopts upstream's split. Validators emit **raw** records; one function turns the raw
list into the final list.

```
cv.Validate / models.Validate / entries.Validate
        │  emits []schemaerr.ValidationError with Code + raw Message + raw SchemaLocation
        ▼
errorpipeline.Parse(raw, document, overlays) -> []schemaerr.ValidationError  (final)
```

`schemaerr.ValidationError` is reused for both stages rather than a second type being introduced:
the members are identical and a `RawValidationError` twin would double every construction site for
no type safety, since nothing can statically distinguish "before Parse" from "after". The
distinction is documented on `Parse` and enforced by the single call site (§2.4).

### 2.2 What the Go raw records look like, and where they differ from pydantic's

Go has **no pydantic-core**, so it emits **no synthetic branch elements**. That has three
consequences, and getting them wrong is the most likely way this iteration silently breaks parity:

1. **The location filter still runs, and it is not dead code.** Its only surviving effect is on
   *real* user keys — `interests`, `my_list`, `strengths` (`spec.md` §3.3 behavior 7). A porter who
   reasons "we have no synthetic tags, so we can skip the filter" reproduces none of that table.
2. **Where upstream emits two records that dedup to one, the port must emit the one that
   survives.** Upstream's dedup is the mechanism; the port's job is the *result*. Three sites,
   from `spec.md` §3.8 behavior 28:

   | Site | Upstream emits | Port emits |
   |---|---|---|
   | `end_date: invalid_date` | exact-date branch, then literal branch | one record at `("end_date",)`, message irrelevant — §3.5 overrides it |
   | `settings.current_date` | date branch, then literal branch | one record at `("settings","current_date")`, message irrelevant — §3.5 overrides it |
   | `cv.photo` | path branch, then URL branch | **the path branch only** — the URL record must not be emitted, or dedup keeps the wrong message |

   The `photo` row is the only one where the surviving *message* differs, and it is pinned by
   `expected_errors.yaml:14-18`. `cv.ResolvePhoto` already has the left-to-right shape
   (`internal/schema/models/cv/customconnection.go:88-108`); this iteration makes its URL branch
   emit no record when the path branch already failed.
3. **Dedup is still implemented.** It is reachable independently of branch tags — §3.3 behavior 7's
   collapsed section keys prove it — and leaving it out would report four records where upstream
   reports one.

### 2.3 Package layout

```
internal/schema/
  errorpipeline/            ← schema/pydantic_error_handling.py
    errorpipeline.go           Parse, parseOne — the eleven steps of spec §3.2
    location.go                the filter, the discriminator skip, the entries splice
    dictionary.go              the thirteen rows as an ordered slice
    dictionary_test.go         diffs the slice against the submodule YAML
    coordinates.go             the document walk of spec §3.10, wrapping iteration 2's resolver
    errorpipeline_test.go
  emailaddr/                ← the email-validator surface pydantic reaches
    emailaddr.go               Validate(string) (normalized string, error)
    emailaddr_test.go
  httpurl/                  ← pydantic.HttpUrl
    httpurl.go                 Validate(string) (normalized string, error) + the three codes
    httpurl_test.go
  phonenum/                 ← pydantic_extra_types.phone_numbers.PhoneNumber
    phonenum.go                Validate(string) (rfc3966 string, error), Serialize(string) string
    phonenum_test.go
```

`errorpipeline` imports `schemaerr`, `yamldoc`, `yamlreader`'s resolver and stdlib. It imports
**nothing** from `models`, so `models` → `errorpipeline` is one-directional and the pipeline can be
unit-tested on hand-built raw records with no model involved.

`emailaddr`, `httpurl` and `phonenum` are leaves importing only stdlib and their one external
dependency. They sit beside `models/` rather than inside it because three different model packages
need them (`cv`, `cv/entries`, and `cv`'s social-network validator).

**Naming.** `errorpipeline` rather than `pydanticerrors`: the port has no pydantic, and a package
named after a Python library would be a lie at every call site. The doc comment names
`schema/pydantic_error_handling.py`, which satisfies `AGENTS.md` §9's mirror rule.

### 2.4 The single call site

`internal/schema/modelbuilder` gains the assembly upstream does at
`rendercv_model_builder.py:175-189`:

```go
// BuildModel mirrors build_rendercv_model_from_commented_map
// (rendercv_model_builder.py:160-189).
func BuildModel(
    doc *yamldoc.Node,
    inputPath string,
    overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) (*models.RenderCVModel, error)
```

It calls `models.Validate`, and when that returns a non-empty raw list it runs
`errorpipeline.Parse` and wraps the result in `schemaerr.UserValidationError`. `Parse` is called
from **nowhere else**; a test asserts that by grepping the tree, because a second call site would
double-apply the period rule and the dictionary.

`models.Validate`'s signature does not change. What changes is that its records are documented as
raw, and the three places that currently pre-format a message stop doing so (§4).

---

## 3. The dictionary

Thirteen rows, order-significant (`spec.md` §3.4 behavior 11, §6.5). A Go `map` cannot hold them:
iteration order is randomized, so first-match-wins would be nondeterministic. It is a slice.

```go
// dictionaryRow is one row of schema/error_dictionary.yaml. Old is matched by
// substring containment against the message; the first match wins.
type dictionaryRow struct{ Old, New string }

// dictionary is error_dictionary.yaml:2-14 in file order. Five of the thirteen
// rows are unreachable in the pinned tree (spec §3.4 behavior 12) and are kept
// anyway, byte for byte, because reachability is not the port's to decide.
var dictionary = [...]dictionaryRow{ ... }
```

**Why compiled-in data and not `go:embed`.** The file lives in the submodule, which is not present
at runtime; embedding would require copying it into the Go tree, which is a transcription risk with
no test. Instead the data is Go source and `dictionary_test.go` reads
`third_party/rendercv/src/rendercv/schema/error_dictionary.yaml`, parses it with the project's own
reader, and asserts key-for-key and value-for-value equality **in order**. That test is the only
thing standing between the port and a silently drifted message, so it is a task of its own and it
lands before the data.

Two traps the test must cover, both from `spec.md` §3.4 behavior 13:

- Rows 3 and 4's keys contain **doubled** backslashes. The Go literals must be raw strings or
  double-escaped accordingly. A porter who writes `\b10\..*` produces a *live* row and breaks
  parity in the opposite direction from the obvious mistake.
- Row 13's key and value are the only quoted scalars in the YAML file, so the reader must handle
  them without changing the bytes.

---

## 4. The eleven steps, as Go

`parseOne` is a straight-line function with the eleven steps of `spec.md` §3.2 in order, one
labelled block each, each block citing its upstream line range. No step is factored out into a
helper unless it is more than ten lines, because the *order* is the contract and a reader must be
able to see it at a glance.

```go
func parseOne(
    raw schemaerr.ValidationError,
    doc *yamldoc.Node,
    overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) schemaerr.ValidationError
```

Notes per step where the Go shape is not obvious:

**Step 1 (strip).** `strings.ReplaceAll`, twice, in the order of `pydantic_error_handling.py:23`.
Not `TrimPrefix` — `spec.md` §3.2 behavior 4 and §6.6 require every occurrence.

**Step 2 (discriminator skip).** Guard the index: `len(loc) > 0 && (loc[0] == "design" || loc[0] ==
"locale")`, then `append(loc[:1:1], loc[2:]...)` on a **copy**. Aliasing the caller's slice here
would corrupt the raw list, which `Parse` iterates twice (once for the record, once for children).
`spec.md` §3.3 behavior 10 is the empty case.

**Step 3 (context override).** The Go raw record has no `ctx` map. The two things upstream reads
from it — an overriding `input` and an overriding `loc` — are instead **carried on the raw record
itself**: a validator that needs to re-pin its location writes the final location directly and
sets a flag. Two producers need it, both measured: `spec.md` §4.27's theme-name failure (which
re-pins to `("design","theme")`) and nothing else. So rather than a general context map,
`schemaerr.ValidationError` gains one boolean:

```go
// LocationIsFinal marks a record whose SchemaLocation was pinned by the
// validator itself and must not be re-derived. It is how design.py:67's
// ctx["loc"] override reaches the pipeline (spec §3.2 step 3, §3.17 behavior 65).
LocationIsFinal bool
```

When set, steps 2 and 4 are skipped for that record. Rejected alternative: a
`Context map[string]any` mirroring pydantic's — it would put a naked `any` on the one type
`AGENTS.md` §9 most wants typed, for exactly one caller.

**Step 4 (build location).** `spec.md` §3.3. The filter:

```go
// unwantedLocations is pydantic_error_handling.py:24-32. Every element whose
// string CONTAINS one of these is dropped. The port emits no synthetic branch
// tags, so the only elements this ever removes are real user keys — see
// spec §3.3 behavior 7, and plan §2.2 consequence 1 before deleting it.
var unwantedLocations = [...]string{
    "tagged-union", "list", "literal", "int", "str", "constrained-str", "function-",
}
```

Kept as an array in upstream's order even though the result is order-independent, so a diff against
`:24-32` is mechanical.

**Steps 5 and 6 (the two overrides).** `strings.Contains(loc[len(loc)-1], "end_date")` and the
same for `current_date`, after the `date` suffix strip. The `current_date` strip is **inert in the
port** — Go emits no `date` branch element — and is implemented anyway, with a comment saying so,
because iteration 7 owns `settings.current_date` and may introduce a location shape that reaches
it.

**Step 7 (dictionary).** A `for` over the slice with `strings.Contains` and `break`.

**Step 8 (period).** `if !strings.HasSuffix(msg, ".") { msg += "." }`. It is the last statement
that touches the message and a comment says so. The three artifact endings (`!.`, `.".`, `)".`) are
in the function's doc comment with their §4 references, because every one of them looks like a bug.

**Steps 9–11.** Source selection reads `location[0]` against the overlay map; coordinate
resolution calls iteration 2's resolver with `location[:len(location)-1]` when
`raw.Code == binder.CodeMissing` and the full location otherwise — compared against the literal
code, not a category (`spec.md` §3.10 behavior 36); `input` rendering switches on
`yamldoc.Kind`, emitting `...` for `KindMapping` and `KindSequence`.

### 4.1 `Parse`: splice, then dedup

```go
// Parse mirrors parse_validation_errors (pydantic_error_handling.py:130-176).
// Input records are RAW; output records are final. Calling it twice on the same
// list double-applies the dictionary and the period rule.
func Parse(
    raw []schemaerr.ValidationError,
    doc *yamldoc.Node,
    overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) ([]schemaerr.ValidationError, error)
```

The wrapper-unpacking of `spec.md` §3.7 reads `Children`, which
`schemaerr.ValidationError` already has and which iteration 2's section validator already fills for
`CodeEntryValidation`. Per child: drop the first location element, prepend the **raw** wrapper
location, then run `parseOne`. One level only — a comment forbids recursion
(`spec.md` §3.7 behavior 26).

A `CodeEntryValidation` record with an empty `Children` slice returns
`&schemaerr.InternalError{Message: …}` with `spec.md` §4.16's text. Go cannot distinguish
"missing ctx" from "missing caused_by", and upstream's message covers both, so one branch suffices.

Dedup: a `map[string]struct{}` keyed by the location joined with `\x00` — not `.`, which a section
key can contain — plus an output slice, preserving first occurrence.

`error` in the signature carries the internal failures of `spec.md` §4.16–§4.18 only. A validation
problem is never an `error` here; it is a record.

---

## 5. The three borrowed-library wrappers

Each exposes the same two-function shape so the seams iterations 2 and 3 registered can be filled
with one line each.

### 5.1 `phonenum`

```go
// Validate mirrors pydantic_extra_types.phone_numbers.PhoneNumber
// (models/cv/cv.py:23-25): parse, require a valid number, and return the
// RFC 3966 form, `tel:` prefix included. Failure is spec §4.8's dictionary key.
func Validate(value string) (string, error)

// Serialize mirrors serialize_phone (models/cv/cv.py:231-250).
func Serialize(stored string) string
```

`Serialize` stays `strings.ReplaceAll(stored, "tel:", "")` — upstream replaces rather than trims,
and a phone number cannot contain a second `tel:`, so the two agree; the shape is kept for
diffability. Iteration 2's `cv.SerializePhone` becomes a one-line forward.

The registration at `internal/schema/models/cv/scalarorlist.go:28-32` swaps
`"phone": passThroughValidator` for a validator calling `phonenum.Validate` and emitting the raw
record whose message is the dictionary key `value is not a valid phone number` — **not** §4.8's
replacement. The pipeline does the replacing; a validator that pre-substitutes would then get a
second period appended only if the text did not end in one, which happens to be harmless here and
is a landmine everywhere else.

### 5.2 `httpurl`

```go
// Validate mirrors pydantic.HttpUrl. It returns the WHATWG-serialized form and,
// on failure, one of three codes (spec §3.13 behavior 45).
func Validate(value string) (string, error)
```

Order inside `Validate`, and it is the order that is tested (`spec.md` §3.13 behavior 46):

1. `len(value) > 2083` → `CodeURLTooLong`, message §4.20. **Before** parsing, and on the input.
2. parse → any failure is `CodeURLParsing` with the message `Input should be a valid URL`, which is
   the dictionary key and therefore all the pipeline needs. The library's reason text is
   **discarded** (`spec.md` §3.13 behavior 45).
3. scheme not `http` or `https` → `CodeURLScheme`, message §4.19.
4. otherwise the serialized form.

`CodeURLTooLong` already exists at `internal/schema/models/cv/entries/publication.go:30`; it moves
to `httpurl` and `publication.go` imports it, so the generated-DOI-URL check and `cv.website` share
one constant.

Registration is one line at each of the four sites of `spec.md` §3.13 behavior 41:
`httpURLValidators` in `publication.go:53`, `elementValidators["website"]` in `scalarorlist.go`,
`CustomConnection.Url`, and the generated social URL. The social site is different and the
difference is load-bearing: it **validates and discards** the normalized form
(`spec.md` §3.13 behavior 44), so it calls `Validate` for its error only.

### 5.3 `emailaddr`

```go
// Validate mirrors pydantic's validate_email wrapper (pydantic/networks.py) over
// the email-validator library. It returns the normalized address. The message set
// it can produce is spec §3.15 behavior 55; anything outside that set is the open
// risk of spec §7.4.
func Validate(value string) (string, error)
```

Ordered checks, matching the measured outcomes: length pre-check (§4.21), pretty-form unwrap,
trim, then the syntax checks in the order that reproduces behavior 55's table, then domain
lowercasing. The table **is** the specification of the order: `a@` must report "something after"
and not "no period", `.a@b.com` must report "cannot start with a period" and not "invalid
characters".

Registration replaces `elementValidators["email"]`.

---

## 6. The one possible divergence, and how the tasks handle it

`spec.md` §7.5. §4.31's message interpolates ruamel's parser text; `goccy/go-yaml`'s differs.
Measured example of the target: `This is not a valid YAML file. while parsing a flow sequence.`

Three options, with the recommendation last:

| Option | Cost | Parity |
|---|---|---|
| A. Emit goccy's first line | zero | fails Axis 4 for every syntax error; `err_not_yaml`'s golden diffs |
| B. Map goccy's error taxonomy onto ruamel's phrasing | bounded by the corpus, unbounded in general | exact for mapped cases, option A for the rest |
| C. Declare a divergence and normalize the sentence | one `specs/divergences.md` entry, human gate | honest, permanently visible |

**Recommendation: B, scoped to the syntax failures the corpus actually contains, with C as the
fallback for the remainder.** The corpus has exactly one syntax case (`err_not_yaml`, one message),
so B's mapped set starts at size one and grows only when a case is added. That makes B cheap
today, honest about its limits, and it keeps the golden green. Whether the unmapped remainder needs
a divergence entry is a **decision, not an implementation**, and `tasks.md` makes it a stop point
before the human gate — this plan does not authorize writing the entry.

Note the asymmetry with §7.1: phone, URL and email were *measured* reproducible, so they got
libraries and no gate. This one was measured *not* reproducible, so it gets a gate.

---

## 7. Known hazards (`AGENTS.md` §6)

Hazards 1–4 (Jinja semantics, whitespace control, custom filters, loader order) are the
templater's and land in iteration 8. Hazard 5 (Lua themes) is iteration 6's; note that
`spec.md` §4.28 and §4.29 are pinned here but only *reachable* there. Hazard 6 (fonts) is
iteration 10's.

Iteration-local hazards, in descending severity:

1. **Deleting the location filter as dead code.** Plan §2.2 consequence 1. The port emits no
   synthetic tags, so the filter looks vestigial, and removing it silently loses `spec.md` §3.3
   behavior 7's whole table — including `interests`, which real users write. Mitigated by making
   that table a required test and by the comment on `unwantedLocations`.
2. **Emitting the wrong survivor at a multi-branch site.** Plan §2.2 consequence 2. The `photo`
   row is the only one where the message differs, it is pinned by `expected_errors.yaml:14-18`,
   and the failure mode is a single wrong sentence in a 25-record diff. Mitigated by making the
   three-row table a test and by the differential.
3. **Pre-formatting a message in a validator.** Any validator that emits §4.8 instead of the
   dictionary key, or that appends its own period, produces text that is *nearly* right. Mitigated
   by: raw-record documentation on `Parse`, a lint-style test asserting no `schemaerr` message in
   `models/**` equals a dictionary *value*, and by §4's rule that the period statement is the last
   one in `parseOne`.
4. **Backslash escaping in dictionary rows 3 and 4.** Plan §3. The natural Go literal is *wrong*,
   and wrong in the direction that makes a dead row live. Mitigated by the submodule-diff test
   landing first.
5. **Ordering.** `spec.md` §3.9 behavior 32's six parts, of which step 3 (declared fields before
   extra keys) is newly measured and contradicts nothing but is easy to get backwards. Mitigated
   by the seven-record table test and by the differential's equal-length assertion.
6. **Go map iteration in the dictionary.** A `map[string]string` would pass most tests and fail
   nondeterministically. Mitigated by the type being an array (§3) and by a test asserting the
   thirteen rows' order.
7. **Coordinate columns.** `spec.md` §7.2 makes the differential compare all five members, so the
   two column shapes of iteration 2's cut-scope item 1 must be fixed *before* the differential
   widens, or the widened test lands red for a reason unrelated to messages. `tasks.md` orders
   them accordingly.
8. **Metadata skew in `phonenumbers`.** §1.1. Bounded, tested, and the two golden numbers are the
   stable ones.
9. **The email tail.** §1.3, `spec.md` §7.4. Known-incomplete by decision, with an owner. The
   hazard is not the incompleteness but a porter treating the twelve-row table as exhaustive and
   writing a catch-all message for everything else — which would produce *wrong* text where
   upstream produces *specific* text. The wrapper returns a distinguishable "unclassified" error
   and a test asserts no measured input reaches it.

---

## 8. Tradeoffs considered and rejected

- **Keeping messages at the validators and post-processing only the location and dedup.** Would
  avoid the raw/final split. Rejected: §3.6's period rule must run after §3.5's overrides and after
  the dictionary, both of which are global; and the dictionary's substring matching only works on
  raw text.
- **A second `RawValidationError` type.** Rejected in §2.1: identical members, doubled
  construction sites, no static distinction available anyway.
- **A pydantic-shaped `Context map[string]any` on the record.** Rejected in §4 step 3: a naked
  `any` on the contractual error type, for one caller. Replaced by one boolean.
- **Reproducing pydantic's synthetic branch tags so the filter has something to filter.** Rejected:
  it would mean inventing strings like `function-after[validate_exact_date(), union[str,int]]` in
  Go, which are unobservable by construction (the filter removes them) and would need updating
  whenever pydantic-core changed. The port emits real locations and relies on the survivor rules
  of §2.2 instead.
- **A general regex-driven message rewriter.** Rejected: upstream's rule is substring containment
  with first-match-wins over thirteen fixed rows. A regex engine here would be strictly more
  powerful and strictly harder to diff against `error_dictionary.yaml`.
- **`net/url` for the URL work.** Rejected in §1.2 on measured grounds.
- **Porting `email-validator` in full (822 lines).** Rejected as scope: it would be most of the
  iteration, and `spec.md` §7.4's bounded set covers everything the contract pins plus everything
  measured. Assigned to iteration 13 if it proves necessary.
- **Fixing the `!.` of §4.12, the stray `"` of §4.3, or the dead dictionary rows.** Rejected on
  principle: they are upstream's output and the contract is byte-level. Each is called out in a
  doc comment so nobody "fixes" them later.
- **Deferring the coordinate columns.** Rejected in `spec.md` §7.2 with reasoning; the alternative
  permanently weakens the only mechanical Axis-4 gate.
