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
3. **The custom-theme fork raises six distinct user-visible messages, not two.** In the order
   `validate_design` reaches them (`design.py:59-132`):

   | # | Trigger | Message | Type |
   |---|---|---|---|
   | a | theme name fails `^[a-z0-9]+$` | "The custom theme name should only contain lowercase letters and digits. The provided value is `\`{theme_name}\``." (`:63-64`) | `PydanticCustomError`, `loc=(design, theme)` |
   | b | folder does not exist | "The custom theme folder `\`{folder}\`` does not exist. It should be in the same directory as the input file." (`:77-78`) | `PydanticCustomError` |
   | c | no `*.j2.typ` anywhere under the folder | "The custom theme folder `\`{folder}\`` does not contain any *.j2.typ files. It should contain at least one *.j2.typ file." (`:85-86`) | `PydanticCustomError` |
   | d | `__init__.py` syntax error | "The custom theme {theme_name}'s \_\_init\_\_.py file has a syntax error. Please fix it." (`:110-111`) | `PydanticCustomError` |
   | e | `__init__.py` import error | "The custom theme {theme_name}'s \_\_init\_\_.py file has an import error! Check the import statements." (`:117-118`) | `PydanticCustomError` |
   | f | module has no `<Theme>Theme` class | "The custom theme {theme_name} does not have a {model_name} class." (`:129-130`) | plain `ValueError`, **not** pydantic-wrapped — no `loc` |

   Two more paths exist in the source (`spec_from_file_location` returning `None`, `:97-99`; a
   `None` loader, `:103-105`) but both raise `RenderCVInternalError` and are reachable only by
   mocking `importlib` in upstream's own test suite (`tests/schema/models/design/test_design.py:146-197`)
   — not from any real folder a user could construct. They are recorded for completeness, not
   ported. Checks run in this order: (a) → (b) → (c) → (d)/(e) → (f), so a theme failing two ways
   reports the first.
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
9. **Behavior 3's six messages split by whether Lua has an equivalent shape to check.** Messages
   (a), (b), (c) are the name-pattern and two folder checks — they run before any script is
   touched, need no Lua-specific reasoning, and are ported verbatim (`design.ValidateTheme`,
   `design.ValidateCustomThemeFolder`; `internal/schema/models/design/customtheme.go`). Messages
   (d), (e), (f) — syntax error, import error, missing model class — describe *Python's* module
   system and have no faithful Lua equivalent: "import error" does not map at all, because Lua
   `init.lua` has no imports, and "missing class" does not map because a Lua declaration is a
   table, not a class to look up by name. Whatever the port emitted for these would be *new
   user-visible text*, extending D-002 rather than fitting inside it.
   **What the port actually does today is not the resolution described in an earlier draft of
   this section.** `luatheme.Run` does produce gopher-lua's own error text — a parse failure
   names the line, a non-table return says "the script returned %s, want a table of theme
   options" — but `themeScript` (`internal/renderer/bridge/model.go`) discards that error and
   returns a nil options map, and `EffectiveWithScript` then falls back to the theme's base
   defaults exactly as it would for a missing script. The artifact renders at exit 0 with no
   message at all; the broken script is invisible. Surfacing gopher-lua's message (or inventing
   upstream-shaped wording) is still an open, human-gated choice — recorded as such in
   `divergences.md` D-002 — not a closed one. A fresh-context verifier caught this contradiction
   (iteration 14's fifth re-verification): the code's own comment at `themeScript` already said
   the gap was open; this section had drifted from it.

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
      *document* puts against a script-declared option is now checked too**: `design.EffectiveWithScript`
      calls `luatheme.Validate(script, document)` and drops only the conflicting document key,
      leaving the script's (or the tree's) value underneath — the closure Finding 5 named as dead
      code. **The declared default is the type** — a Lua declaration carries no annotation
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

**Verified six times by a fresh context, FAIL every time through the 5th** (`specs/STATE.md`,
"Iteration 14 [re-]verified..." sections, 2026-08-10). Each pass found real defects the previous
one's own tests could not see — three blockers on the first pass, regressions in the fix itself on
the second, third and sixth passes, a fourth door into the same leak class on the fourth, and two
blockers reaching *built-in* themes (not just scripted custom ones) on the fifth. Every finding so
far has been fixed and pinned by a test; the four §4 boxes above reflect what the fixes cover today,
not a claim that the criteria are closed for good. One item is deliberately still open rather than
fixed: upstream's
forbid-extra rejection of an unknown design key on a scripted custom theme (`theme_data_model_class`,
`design.py:135`) needs the theme's script loaded during *validation*, not only at render time, and
is cut to a future scoped `tasks.md` unit.

**Wired.** `bridge.Resolve` looks for `<theme>/init.lua` beside the input file — upstream's
`validate_design` position, because the options must exist before anything reads the effective
tree — and three tests drive it end to end through real files: a script's default reaches the tree,
the document still beats it, and a theme with no script is bit-for-bit what it was.

The three folder/name checks of §1 behavior 3 (a)(b)(c) are wired and ported verbatim
(`design.ValidateTheme`, `design.ValidateCustomThemeFolder`), reached from `models.Validate` via
`internal/schema/models/design/validate.go`, so a bad theme name, a missing folder, or a folder
with no `*.j2.typ` reports upstream's own text rather than rendering happily. The three Lua-only
checks — (d)(e)(f) in the table — are **not surfaced at all**: `themeScript` discards
`luatheme.Run`'s error and falls back silently, exit 0 (§2 behavior 9, corrected). Whether to
surface gopher-lua's own text or invent upstream-shaped wording is still open behind the human
gate.

**No corpus case exercises any of it** — the corpus has no custom theme — so every claim here rests
on unit tests rather than on a differential against upstream. Unblocked — D-002 is approved and no other gate applies. The honest ordering is
behavior 4 before anything else: it is the path all nine built-in themes and all 24 corpus
documents already take, so it is the one that can regress something that currently works.
