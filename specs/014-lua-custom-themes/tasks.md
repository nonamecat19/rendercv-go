# Tasks 014 — closing the Lua custom-theme gaps

Written 2026-08-11, after a read-only investigation established that iteration 14's row had been
counting two other subsystems' findings and that its **real** open set is four items. One
(a non-scalar colour tuple element) is fixed in `e53a321`. These are the rest.

Ordering: T1 is a prerequisite for T2 and T3 — all three need the script's declared shape available
at validation time, which is the control-flow change. T4 is independent.

---

## T1 — load the theme script during validation [spec §5, finding 1/2 prerequisite]

**Owns:** `internal/schema/models/design/validate.go`, `internal/renderer/bridge/model.go`, and a new
file in `internal/schema/models/design` if the loading belongs in its own unit.

Today `<theme>/init.lua` is read only at render time, in `bridge.Resolve`. `design.Validate`
(`validate.go:150-168`) returns `nil` unconditionally for any non-built-in theme once the two folder
checks pass — "its options are its own" — so nothing about a custom theme is ever validated.

Upstream's `theme_data_model_class(**design)` (`design.py:135`) instantiates the script's model
against the whole `design` block at validation time. The port must make the script's declared shape
available at the same point.

**This task changes control flow only.** It must land with no user-visible behavior change: the same
documents validate and render, byte for byte. The two behaviors it unlocks are T2 and T3.

Acceptance: the corpus is unchanged, `just test-parity` is unchanged, and a unit test asserts the
script's declared fields are visible to `Validate` for a scripted theme.

## T2 — value-validate a scripted theme's document values [finding 1]

**After:** T1.

With the script's shape available, run a custom theme's `design` block through the same
`validateModel`/`validateField` machinery the built-in tree uses.

Measured: `design: {theme: mytheme, page: {size: bogus}}` against a scripted theme is
`Input should be 'a4', 'a5', 'us-letter' or 'us-executive'` at `design.page.size`, exit 1 upstream.
The port renders `page-size: "bogus"` into the `.typ` at **exit 0** — a wrong artifact, silently.

Acceptance: that vector matches upstream on exit code and validation record, differentially. A
scripted theme's *own* declared options continue to validate against the script's declaration, not
the built-in tree's.

## T3 — forbid unknown keys on a scripted theme [finding 2]

**After:** T1. Separate commit from T2 — they are one root cause but two user-visible behaviors, and
T2's fix does not imply T3's.

Upstream's generated model inherits the same `extra="forbid"` base as the built-in tree, so an
unrecognised key in a custom theme's `design` block is exit 1. The port accepts it.

Acceptance: an unknown key under a scripted theme's `design` block produces upstream's unknown-field
record at the right location, differentially.

## T4 — report a broken theme script instead of discarding it [finding 3, gate ANSWERED]

**Independent of T1–T3.**

`bridge/model.go:93-111` returns the same `nil, hasScript` for a Lua syntax error, a non-table
return, a shape conflict and a bad declared value as it does for "there is no script at all". The
document renders with base defaults at **exit 0** with no signal of any kind.

**The human gate on wording was taken on 2026-08-11 and answered: exit 1 with a generic message.**
The approved shape is an `Error` panel naming the theme's script path and the reason, e.g.

```
╭─ Error ──────────────────────────────────────────╮
│ The theme script ./mytheme/init.lua could not be │
│ loaded: init.lua:3: '}' expected near 'x'        │
╰──────────────────────────────────────────────────╯
```

exit 1. Upstream's own wording is Python-specific (`SyntaxError`, an import error, a missing class)
and cannot be reproduced; the port matches its **exit code** and its **refusal to render**, and
diverges on the text. That divergence is approved and must be written into `specs/divergences.md` as
part of this task — it is the one place in this tasks file where editing that file is sanctioned,
because the gate has already been taken.

Distinguish the four failure modes in the reason string; do not collapse them. "Script absent"
remains silent and unchanged — that is not a failure.

Acceptance: each of the four failure modes exits 1 with a panel naming the path and a reason; a theme
folder with no script is untouched; and a test pins that "absent" and "broken" take different paths.

---

## Not in this iteration

The `SetMx` 512 MB sandbox bound is **not** a "port succeeds where upstream crashes" case and should
not be filed as one. It is a deliberate bound D-002 exists to impose, against an upstream with no
equivalent and no fixed behavior to diff against. If it earns a `divergences.md` line, that line
should say what it is — a chosen sandbox bound — and it needs its own gate.
