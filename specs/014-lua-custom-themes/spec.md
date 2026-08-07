# Iteration 14 — custom themes, scripted in Lua

Behavior of the custom-theme mechanism, extracted from the vendored Python, plus the shape D-002
requires the port to give it instead. No Go design here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary source: `src/rendercv/schema/models/design/design.py:26-145`.

---

## 0. What this iteration is

**The only iteration whose divergence is approved before it starts.** D-002 is `approved`: upstream
executes a custom theme's `__init__.py` at validation time, Go cannot, and the port scripts custom
themes in sandboxed Lua (`gopher-lua`) instead. So the question is not *whether* to diverge but
*what the Lua contract is*, and the answer has to be derived from what the Python one does.

## 1. What upstream does

1. `validate_design` looks for `<theme>/__init__.py` beside the input file
   (`design.py:90-91`) and, if present, **imports it** — arbitrary user code, executed during
   validation.
2. It expects the module to define a pydantic model of theme options. That model becomes the
   theme's option schema, so a custom theme can **add options** the built-ins do not have and
   **change the defaults** of ones they do.
3. **A syntax error and an import error are distinguishable, user-visible messages**
   (`:110`, `:117`) naming the theme.
4. **A theme folder with no `__init__.py` is valid** (`:137-142`): the theme falls back to a
   subclass of `ClassicTheme` with only its `theme` field set, so templates work and no option is
   added.
5. The generated `__init__.py` that `create-theme` writes imports from `rendercv.schema.models.*` —
   `Color`, `FontFamily`, `TypstDimension` — and redeclares the `Literal` unions. So the contract a
   user writes against is the design model of iteration 6, which this port already has.

## 2. What the port must decide, and what it must not

6. **The Lua script cannot be a transliteration of the Python.** The Python file declares a
   *pydantic model*; the equivalent is a declaration of fields, types and defaults, which in Lua is
   data — a returned table — not a class.
7. **Behavior 4 is the load-bearing one.** A theme with no script must behave exactly as the Python
   fallback does: `ClassicTheme` with the theme name set. That is `design.Effective` with the base
   tree and no overrides, which iteration 6 already produces — so the no-script path needs no new
   code and must be tested first, because it is the path every *built-in* theme takes.
8. **The sandbox is the point of D-002, not an implementation detail.** Upstream runs arbitrary
   Python with the full standard library available; a Lua state with the same reach would be the
   same hazard with a different syntax. What the state may touch — no `io`, no `os`, no `require`
   of arbitrary files — belongs in `plan.md` and is a security decision, not a parity one.
9. **The two error messages of behavior 3 have no port equivalent yet.** "Syntax error" maps to a
   Lua parse failure; "import error" does not map at all, because there are no imports. Whatever
   the port emits is *new user-visible text* and therefore extends D-002 rather than fitting inside
   it — which is a human gate.

## 3. Out of scope

**3.1 `create-theme`'s file writing** is iteration 12's, and it is already recorded as unreachable
by construction: it would have to write `init.lua` where upstream writes `__init__.py`, which is
this iteration's contract.

**3.2 The built-in themes** need none of this. All nine are YAML overrides resolved by
`design.Effective` (iteration 6), and no built-in theme has a script.

---

## 4. Acceptance criteria

- [x] Behavior 4 first: a theme folder with **no** script resolves to `ClassicTheme` with the theme
      name, and every built-in theme still resolves exactly as it does today — 24/24 documents
      unchanged. **Met with no new code**, as §2 behavior 7 predicted: `design.Effective` already
      produces it, and `design/customtheme_test.go` pins it so the prediction stays true rather
      than being asserted once and discovered wrong later.
- [x] A theme whose script **adds** an option: `luatheme.Options` carries it into the effective
      tree, `design.EffectiveWithScript` merges it, and `design.ValidateScript` checks the script's
      own shapes against the tree — a mis-typed option is dropped rather than printed into the
      artifact as a Go type name, which is what it did before a verifier caught it. **What a
      *document* puts in a script-declared option is still unchecked**: `luatheme.Validate` exists
      for it and nothing calls it. **The declared default is the type** — a Lua declaration carries no annotation
      but always carries a value, so a script cannot claim a type it does not demonstrate. That is
      a smaller contract than upstream's pydantic annotations and an honest one; it catches a group
      written where a value belongs and the reverse, which is the mistake that fails
      unreadably further down.
- [x] A theme whose script **changes a default**: the new default appears where no document
      overrides it, and a document override still wins. The layer sits between the theme's
      overrides and the document's block — above it and a user could not override their own theme,
      below it and a custom theme could change nothing.
- [x] The sandbox refuses filesystem and process access, asserted rather than assumed.
      `internal/schema/luatheme` closes `io`, `os`, `package`, `require`, `dofile`, `loadfile` and
      `debug`, and names each one in a table-driven test — so removing one from the list fails
      loudly instead of quietly widening what a downloaded theme can do. `string`, `table` and
      `math` remain, which is all a declaration needs.

## 5. Status

**All four acceptance criteria met**, and **not verified by a fresh context** — no
`rendercv-parity-verifier` pass has looked at this iteration, so the row in `STATE.md` reports what
the suite prints and nothing more.

**Wired.** `bridge.Resolve` looks for `<theme>/init.lua` beside the input file — upstream's
`validate_design` position, because the options must exist before anything reads the effective
tree — and three tests drive it end to end through real files: a script's default reaches the tree,
the document still beats it, and a theme with no script is bit-for-bit what it was.

What is deliberately *not* here: the two folder messages of §1 behavior 3, whose text is new and
human-gated (§2 behavior 9). **A script that fails is currently silent** — it falls back as though
absent, which is safe but tells the user nothing. That is the gap, and it is one message away from
closed once the text is approved. `create-theme` writing an `init.lua` is iteration 12's and
recorded as unreachable by construction.

**No corpus case exercises any of it** — the corpus has no custom theme — so every claim here rests
on unit tests rather than on a differential against upstream. Unblocked — D-002 is approved and no other gate applies. The honest ordering is
behavior 4 before anything else: it is the path all nine built-in themes and all 24 corpus
documents already take, so it is the one that can regress something that currently works.

Behavior 9's error text is the one piece that needs a human before it ships.
