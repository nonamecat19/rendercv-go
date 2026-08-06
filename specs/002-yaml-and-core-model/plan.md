# Plan 002 — YAML reader and core model

Go design for [`spec.md`](spec.md). Behavior claims live there; this file decides code.

---

## 1. Dependency decision — the YAML library

### 1.1 Requirements

| # | Requirement | spec.md |
|---|---|---|
| R1 | Per-node line and column | §3.13, §6.7 |
| R2 | Mapping key order preserved | §3.12, §3.50, §6.1 |
| R3 | Timestamp-looking scalars stay strings | §3.11 |
| R4 | `*` at token start is a plain scalar, not an alias | §3.10 |
| R5 | Implicit scalar resolution (null/bool/int/float/str) matching ruamel's | §3.69, §3.73 |
| R6 | Scalar text preserved exactly, no normalization | contract §1.1 |

### 1.2 Candidates

Both were probed empirically against v1.19.2 / v3.0.1 before this decision. Nothing below is
inferred from documentation.

**`gopkg.in/yaml.v3` — rejected on two independent counts.**

1. **R3 fails.** `2020-09-24` decodes to `time.Time`. Upstream disables the timestamp
   constructor outright (`schema/yaml_reader.py:83-86`) and keeps the string. Recovering this
   means post-processing every scalar in the tree against `Node.Tag`, i.e. reimplementing
   resolution anyway.
2. **R4 is not reachable.** Alias handling lives inside the vendored libyaml parser
   (`yaml_parser_parse_node`), unknown anchors are an error, and there is no exported seam
   between scanning and parsing. Achieving R4 means forking the whole package.

R1 is also weaker: `yaml.Node` carries a start position only. The project is maintenance-only
upstream. Rejected.

**`github.com/goccy/go-yaml` v1.19.2 — selected.** Verified properties:

- **R3.** `2020-09-24` and `2020-09` are `*ast.StringNode`; `2020` is `*ast.IntegerNode`. This
  matches upstream, whose arbitrary-date type is integer-or-text (spec §3.69) and whose
  date-object rule has a distinct integer branch (spec §3.73 case 1).
- **R1.** Both halves of a pair carry positions: `MappingValueNode.Key.GetToken().Position` and
  `.Value.GetToken().Position`, 1-indexed. This is exactly the key-position/value-position pair
  ruamel's `lc.data[key]` holds (plan §3).
- **R2.** `MappingNode.Values` is an ordered slice.
- **R6.** Tokens carry `Value` and `Origin` separately.
- **R4** via §1.3 below.
- **R5** is not delegated to goccy — see plan §3.

`AGENTS.md` §2 already names goccy, so this confirms rather than changes the architecture.

**No supported option disables alias resolution.** Checked before considering a fork:
`option.go` has no decode option touching anchors or aliases — `MarshalAnchor` and
`WithSmartAnchor` are encode-side only. `scanner.Scanner`'s alias path (`scanAlias`,
`scanner/scanner.go:1272-1283`) is unexported, and the package pulls in goccy's `internal/…`
tree, so a fork is a multi-file copy rather than a one-line override. The token-stream seam of
§1.3 is the only route through the public API.

### 1.3 The `*` problem

Four approaches were considered. Three are ruled out by evidence, not by argument.

**(a) Reinterpret `*ast.AliasNode` after parsing. — RULED OUT.** `parser.Parse` itself fails on
`mixed: *a and more` with `value is not allowed in this context. map key-value is pre-defined`.
The failure is at parse time, so no AST post-processing can reach it. Re-serializing a rewritten
AST does not help either: `File.String()` re-emits alias syntax.

**(b) Pre-parse transform of the source text. — RULED OUT.** Rewrite `*` to a sentinel before
parsing, restore after. Deciding *where* `*` opens an alias requires knowing whether the offset
is inside a single- or double-quoted scalar, a block scalar, a comment, or an anchor name — i.e.
it requires a YAML scanner, which is the thing being avoided. The failure mode is silent
corruption of user text, and `highlights: ["**bold**"]` is ordinary input in this project
(markdown in CV fields is a first-class feature). Cheap and wrong. Rejected.

**(c) Token-stream transform between `lexer.Tokenize` and `parser.Parse`. — SELECTED.**

```go
tokens := dealias(lexer.Tokenize(src))
file, err := parser.Parse(tokens, 0)
```

Both calls are public API, and the seam sits at the same structural position where upstream
patches ruamel (`schema/yaml_reader.py:70-80`) — between scanning and parsing. `dealias` walks
the stream and rewrites every `token.AliasType` token into a plain `token.StringType` token.

The one subtlety: the lexer emits `*` as its own token and splits the rest of the scalar, so
`mixed: *a and more` arrives as `Alias("*")`, `String("a")`, `String("and more")` where the
plain scalar `a and more` would have been one token. The transform therefore absorbs the whole
run of `String` tokens on the alias token's own line, concatenates their `Origin`s, sets `Value`
to that origin with leading whitespace trimmed, and keeps the alias token's `Position` (which is
the `*`'s own column — the correct start for the merged scalar).

Verified independently against v1.19.2: all seven reference cases of spec §5.3 produce upstream's
values, plus block scalars, comments and the timestamp cases. Comments, quoted scalars and block
scalars need no special handling because the lexer has already resolved them by the time the
transform runs — precisely the safety (b) lacked. Regression-checked as a **no-op** across all
46 real YAML files in the submodule (9 example CVs, the ATS corpus, test fixtures, 21 locale
catalogs, 8 theme files, `error_dictionary.yaml`): 46 identical, 0 differing, 0 failures.

*Residual risk:* the transform depends on `lexer.Tokenize` / `parser.Parse` staying separable
and on `token.Token` staying constructible. Both are public API, but neither is covered by a
compatibility promise. Mitigated by isolating the whole thing behind one function (task T7) and
by task T10's differential corpus.

**(d) Fork goccy's lexer so it never emits alias tokens. — FALLBACK.** The direct analogue of
upstream's monkeypatch, and behaviorally exact. Cost: a multi-file vendored copy (§1.2) to
re-diff on every goccy upgrade. Kept available because (c) is isolated behind `dealias`, so the
swap is one function body. *Trigger:* goccy changes the token API, or task T10's differential
corpus finds a case (c) cannot express. Adopting (d) changes no observable behavior, so it is a
dependency decision, not a divergence, and needs no entry in `specs/divergences.md`.

### 1.3.1 Two consequences for the decoder

1. **Decode through the AST, never `goccy.Unmarshal`.** Unmarshal runs its own decode path,
   which would give alias resolution a second chance to fire. `yamlreader/build.go` walks
   `*ast.File` and builds `yamldoc.Node` directly (plan §3).
2. **Anchor nodes must be unwrapped.** `real_anchor: &anchor value` parses to an
   `*ast.AnchorNode` wrapping a `*ast.StringNode`, whereas upstream yields the plain value
   `value` (spec §5.3, fifth case). The builder unwraps `*ast.AnchorNode` to its `Value` and
   discards the name. This is *not* optional and *not* covered by any upstream test — it is the
   anchor half of spec §3.10a, and suppressing anchors instead of unwrapping them would pass the
   other six reference cases and fail this one.

### 1.4 Other dependencies

None. Everything else in this iteration is stdlib. Email, HTTP-URL and phone validation are
iteration 4's and are represented here as registered hooks (§6), not as imported libraries, so
this iteration does not pre-commit their library choices.

---

## 2. Package layout

Mirrors upstream paths per `AGENTS.md` §9. Upstream's `schema/models/cv/entries/__init__.py`
is empty and the entry union lives in `section.py`; this port moves the union's *registry* into
`entries` (see §5) so `entries` does not import `section`.

```
internal/schema/
  schemaerr/                       ← src/rendercv/exception.py
    error.go                       ValidationError tree, UserError, InternalError
    source.go                      YamlSource / OverlayKey named types + the fixed map

  yamldoc/                         ← (new) the parsed-document representation
    node.go                        Node, Item, Kind, ScalarStyle
    position.go                    Position, Span
    walk.go                        lookup by dotted path (used by iteration 4)

  yamlreader/                      ← src/rendercv/schema/yaml_reader.py
    yamlreader.go                  ReadFile / ReadString, extension + empty + string-root checks
    noalias.go                     the token-stream rewrite of §1.3(c)
    build.go                       goccy AST → yamldoc.Node
    resolve.go                     implicit scalar resolution (null/bool/int/float/str)

  modelbuilder/                    ← src/rendercv/schema/rendercv_model_builder.py (overlay half)
    merge.go                       overlay merge, settings defaulting, render-command overrides
    yamlerror.go                   parser failure → schemaerr.ValidationError

  binder/                          ← (new) the pydantic-shaped part of validation
    binder.go                      key policy, absent-vs-null, error accumulation

  models/
    base.go                        ← models/base.py       extra-key policy constants
    validationcontext.go           ← models/validation_context.py
    path.go                        ← models/path.py
    rendercvmodel.go               ← models/rendercv_model.py
    cv/
      cv.go                        ← models/cv/cv.py
      section.go                   ← models/cv/section.py
      socialnetwork.go             ← models/cv/social_network.py   (shell)
      customconnection.go          ← models/cv/custom_connection.py (shell)
      entries/
        registry.go                ← (new) §5; iteration 3 fills it
        bases/
          entry.go                 ← entries/bases/entry.py
          entrywithdate.go         ← entries/bases/entry_with_date.py
          entrywithcomplexfields.go← entries/bases/entry_with_complex_fields.py
```

Nothing is exported from `pkg/rendercv` in this iteration. The public API surface is frozen in
iteration 13; exporting a model shape now would freeze decisions the renderer has not made yet.

---

## 3. The document tree

`yamldoc` is the port's stand-in for ruamel's `CommentedMap`. It is deliberately not a
`map[string]any`: order (spec §6.1), positions (spec §3.13), and absent-vs-null (§4) are all
observable, and none survive a Go map.

```go
type Position struct{ Line, Column int } // both 1-indexed, as goccy reports them
type Span struct{ Start, End Position }

type Kind uint8
const (
    KindNull Kind = iota
    KindBool
    KindInt
    KindFloat
    KindString
    KindMapping
    KindSequence
)

type Node struct {
    Kind  Kind
    Span  Span
    Raw   string      // scalar text exactly as written, unquoted
    Style ScalarStyle // Plain, SingleQuoted, DoubleQuoted, Literal, Folded
    Items []Item      // KindMapping, in input order
    Elems []*Node     // KindSequence, in input order
}

type Item struct {
    Key     string
    KeySpan Span
    Value   *Node // never nil; an explicit null is a KindNull node
}
```

**Span conventions.** ruamel's `lc.data[key]` is `[key_line, key_col, value_line, value_col]`
(0-indexed), which is why `pydantic_error_handling.py:216-217` reads four numbers for a mapping
key and two for a list index. `Item.KeySpan` therefore stores `Start` = the key token's
position and `End` = the value token's position. A sequence element stores `Start == End` = the
element's own token position. Positions are kept 1-indexed as goccy reports them; the ±1
arithmetic of spec §6.7 is iteration 4's and is applied on read, not on store. The conversion
back to ruamel's frame is `ruamel = goccy - 1` on both axes, and task T8's differential fixture
pins it.

**Scalar resolution** (`resolve.go`) reproduces ruamel's YAML 1.2 core-schema resolution *minus*
the timestamp tag: `null`/`~`/empty → `KindNull`; `true`/`false` → `KindBool`; integer and float
patterns → `KindInt`/`KindFloat`; everything else → `KindString`. `Raw` always keeps the
original text, so a validator that wants "the value as the user wrote it" (spec §4.16) has it.
A *quoted* scalar is always `KindString` regardless of content. This is the highest-risk
correctness area after `*`, because goccy's own token typing (`Integer`, `Bool`, …) is not
guaranteed to agree with ruamel's resolver on edge cases (`yes`, `0o17`, `.inf`, `0x1F`,
`00123`, `+1_000`). We resolve from `Raw` with our own table rather than trusting goccy's token
type, and task 10's differential test covers it.

---

## 4. Absent vs. present-and-null

Upstream distinguishes the two, but only in three places, and the port handles each where
upstream does — on the raw document, not in the model.

| Case | Upstream | Port |
|---|---|---|
| `_key_order` drops null-valued keys (spec §3.50) | reads the raw dict before validation (`cv.py:166`, `:173`) | `binder` walks `Node.Items` and skips `KindNull` values |
| unknown key rejected even when null (spec §5.15) | `extra="forbid"` (`base.py:5`) | `binder` compares `Item.Key` against the field set; the value is never consulted |
| `custom_connections[].url` is required-but-nullable (spec §3.81) | no default on the field | `binder` reports "missing" when no `Item` has that key; a `KindNull` value binds to a nil pointer |

**Everywhere else, absent and null are indistinguishable**, because every other optional field
in this iteration declares `= None` and pydantic maps an explicit null onto the same value as
the default. So:

> Model fields use `*T` for optional values and plain `T` for required ones. There is no
> `Optional[T]` wrapper type, no `Set bool` flag, and no sentinel. Presence is a property of the
> `yamldoc.Node`, and only the binder asks about it.

This keeps the model structs readable, keeps `gofumpt`/`golangci-lint` quiet about a wrapper
generic threaded through every field, and puts the distinction in the one component that has the
document in hand. The cost is that a model value alone cannot answer "was this key written?" —
acceptable, because nothing downstream of validation asks (the renderer consumes `_key_order`,
which the binder computes).

---

## 5. The entry-type registry

Per spec §7.1. Upstream computes the characteristic-field table at import time from the concrete
classes (`section.py:36-42`, `:77`); iteration 2 has no concrete classes, so the dependency is
inverted.

```go
package entries

type TypeName string

// Descriptor is what a concrete entry type publishes about itself.
// It mirrors what upstream reads off a pydantic class: its name and its
// model_fields key set (inherited fields included).
type Descriptor struct {
    Name   TypeName
    Fields []string
}

// Registry holds descriptors in discrimination-priority order
// (spec §3.57). Order is load-bearing; it is not a map.
type Registry struct{ descriptors []Descriptor }

func NewRegistry(in ...Descriptor) *Registry
func (r *Registry) Descriptors() []Descriptor
func (r *Registry) Names() []TypeName            // the eight, plus TextEntry appended
func (r *Registry) Characteristic() map[TypeName]map[string]struct{}
func (r *Registry) Discriminate(keys []string) (TypeName, bool)
```

Rules:

- `NewRegistry` takes the descriptors **as an explicit ordered argument list**, not via `init()`
  side effects in the concrete entry packages. Registration order is the priority order of spec
  §3.57, and `init()` ordering across packages is not something to stake a byte-parity contract
  on.
- `Characteristic()` implements spec §3.55 and is computed once per registry.
- `Discriminate` implements spec §3.58's mapping branch: first descriptor whose characteristic
  set intersects `keys`.
- `section.go` imports `entries`; `entries` imports nothing from `models/cv`. Iteration 3 adds
  `entries/education.go` etc., each exposing a `Descriptor`, and one call site assembles them in
  order.

**Iteration 2's tests use a fixture registry** whose eight descriptors carry upstream's real
field sets (spec §3.56 lists the resulting characteristic sets; the full field sets come from
the same source). Iteration 3 swaps in the real descriptors, and every test in
`section_test.go` must pass unchanged. That equality is the whole point of the inversion.

---

## 6. Validation error representation

Iteration 4 renders errors; it must not have to reshape anything. Per `AGENTS.md` §9,
validation errors are a typed tree.

```go
package schemaerr

type YamlSource string
const (
    SourceMain     YamlSource = "main_yaml_file"
    SourceDesign   YamlSource = "design_yaml_file"
    SourceLocale   YamlSource = "locale_yaml_file"
    SourceSettings YamlSource = "settings_yaml_file"
)

type OverlayKey string // "design" | "locale" | "settings"
var OverlayToSource = map[OverlayKey]YamlSource{...} // exception.py:13-17

// Code discriminates the error so iteration 4 can look up rewrite rules in
// error_dictionary.yaml without parsing the message.
type Code string

type ValidationError struct {
    Code           Code
    SchemaLocation []string   // nil == absent (exception.py:22)
    YamlLocation   *yamldoc.Span
    YamlSource     YamlSource
    Message        string     // already interpolated
    Input          string
    Children       []ValidationError // spec §3.61's nested failures
}

type UserError struct{ Message string }              // exception.py:29-31
type UserValidationError struct{ Errors []ValidationError } // exception.py:34-36
type InternalError struct{ Message string }          // exception.py:39-41
```

All three error types implement `error`; callers match with `errors.As`, never on string
content. `Children` exists solely for spec §4.12, which carries a sub-list of failures; it is
never flattened to text inside this iteration.

`Code` carries upstream's discriminators (`rendercv_other_error`,
`rendercv_entry_validation_error`) and the pydantic-native ones this iteration can raise
(`missing`, `extra_forbidden`, `value_error`, `string_type`, `list_type`). Iteration 4 owns the
full catalog; iteration 2 defines only the codes it emits and leaves `Code` an open string type
so adding one is not a breaking change.

`YamlLocation` is a `*yamldoc.Span` rather than a flattened tuple so the ±1 arithmetic of spec
§6.7 stays in one place (iteration 4's renderer) instead of being baked in at raise sites.

---

## 7. Threading the validation context

```go
package models

type ValidationContext struct {
    InputFilePath string    // "" == absent
    CurrentDate   any       // time.Time, the string "today", or anything else
}

func (c *ValidationContext) InputPath() (string, bool) // validation_context.py:29-33
func (c *ValidationContext) Today() time.Time          // validation_context.py:53-58
```

Upstream threads this through pydantic's opaque `info.context` under a nested `"context"` key
and reads it defensively at three call sites. Go has no such ambient channel, so the context is
an explicit first parameter on every bind function. The nested `"context"` key and the
`isinstance` guards (`validation_context.py:29`, `:53`) have no analogue and are dropped — they
are Python-typing artifacts with no observable behavior, not a divergence. What **is** preserved
verbatim is the fallback ladder: `Today()` returns the context date if it is a real date, today
if the value is exactly `"today"`, and today for anything else including garbage
(`validation_context.py:53-58`). `CurrentDate` stays `any` for exactly that reason — it must be
able to hold the invalid value the settings model will later complain about. This is the one
place `AGENTS.md` §9's no-naked-`any` rule is relaxed, and it is in `internal/`, not
`pkg/rendercv`.

---

## 8. Known hazards (`AGENTS.md` §6)

Hazards 1–4 (Jinja/pongo2 semantics, `trim_blocks`, custom filters, loader order) are the
templater's and land in iteration 8. Hazard 5 (custom themes → Lua, D-002) is iteration 6's.
Hazard 6 (fonts) is iteration 10's. None of them constrain this iteration's code.

One of them constrains this iteration's **data**: hazard 2 says whitespace is observable in
rendered output. Every scalar this iteration parses eventually reaches a template, so
`yamldoc.Node.Raw` must be the scalar's text with block-scalar folding/chomping applied exactly
as ruamel applies it, and with no trimming of our own. `resolve.go` never rewrites `Raw`; it
only classifies. Task 10's differential test includes literal and folded block scalars with
every chomping indicator.

Iteration-local hazards, in descending severity:

1. **`*` fidelity** (§1.3). Mitigated by approach (c) plus a differential test, with (d) as a
   committed fallback.
2. **Implicit scalar resolution** (§3). goccy's token typing is not ruamel's resolver. We
   resolve ourselves and test differentially.
3. **Position frames.** goccy is 1-indexed; ruamel is 0-indexed; upstream's coordinate
   arithmetic is asymmetric between mapping keys and list indices (spec §6.7). Pinned by a
   fixture generated from the vendored Python, never hand-written.
4. **Two non-reproducible strings** (spec §7.3): CPython date-range messages and ruamel parser
   messages. This iteration pins the observed values in table tests so that iteration 4's
   decision is a visible test change rather than a silent drift.
5. **Upstream crashes on `[1]` as a section** (spec §5.14). The port must decide reproduce-or-
   improve. Deferred to iteration 4's gate; until then the Go code returns spec §4.9's error and
   the test asserting that is marked with a `TODO(iteration-4)` comment naming this spec section.

---

## 9. Tradeoffs considered and rejected

- **Decoding into `map[string]any` and keeping a parallel position index.** Simpler tree, but
  loses key order and forces two structures to stay in sync across the overlay merge, which
  mutates the document. Rejected.
- **A generic `Optional[T]` wrapper on every model field.** Uniform, but pays for a distinction
  that matters in three places (§4) across roughly sixty fields, and makes every template-facing
  accessor a method call. Rejected.
- **Reflection-driven binding with struct tags, mirroring pydantic.** Would shrink the binder,
  but the error text, error *order*, and error *count* are all contractual (contract §4), and
  reflection makes those emergent rather than specified. Hand-written binders per model keep
  every raise site visible and citable. Accepted cost: more code.
- **Putting the entry union in `section.go` as upstream does.** Would force `section` to import
  the concrete entry packages and iteration 3 to edit iteration 2's file. The registry inversion
  (§5) costs one small package and buys a clean iteration boundary.
- **Deferring the whole `*`-handling problem to iteration 3.** It is the single hardest fidelity
  requirement here (spec §3.10) and it is cheapest to solve while the reader is the only
  consumer of the token stream. Kept.
