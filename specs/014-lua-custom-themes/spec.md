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

- [ ] Behavior 4 first: a theme folder with **no** script resolves to `ClassicTheme` with the theme
      name, and every built-in theme still resolves exactly as it does today — 24/24 documents
      unchanged.
- [ ] A theme whose script **adds** an option: the option validates, appears in the effective tree,
      and reaches a template.
- [ ] A theme whose script **changes a default**: the new default appears where no document
      overrides it, and a document override still wins.
- [ ] The sandbox refuses filesystem and process access, asserted rather than assumed.

## 5. Status

**Not started. Unblocked** — D-002 is approved and no other gate applies. The honest ordering is
behavior 4 before anything else: it is the path all nine built-in themes and all 24 corpus
documents already take, so it is the one that can regress something that currently works.

Behavior 9's error text is the one piece that needs a human before it ships.
