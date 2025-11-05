# Tasks 015 — Explicit YAML tags

One commit each. Every task lands its own test, confirmed red before the change
(`AGENTS.md` §4 step 4). `go build ./... && go test ./...` green at every commit (§7).

Sequential by construction — T2–T4 all edit `buildNode`'s new case, T5–T7 read the `Kind` it
produces. **No fan-out** (`AGENTS.md` §5, the stop rule).

---

- [x] **T1 — `yamldoc.KindTagged` and `Item.KeyTagged`.**
      Types and doc comments only, no behavior. Appended after `KindSequence` so no existing
      constant moves. Test: `String()`/exhaustiveness-style unit if the package has one,
      otherwise the constant's value is pinned so a later insertion cannot shift it.

- [x] **T2 — a tag on a collection is transparent.**
      `buildNode`'s `*ast.TagNode` case, shape-first: a mapping or sequence inner node builds
      exactly as it would untagged, whatever the tag says (spec §5.1). Closes `cv: !!map`,
      `sections.experience: !!seq`, a `!!map`-tagged entry, a `!!map`-tagged root, and
      `!!str [1,2]`. Test: the tagged and untagged trees are equal.

- [x] **T3 — type-forcing scalar tags.**
      `resolveTag`'s table: `!!int`, `!!float`, `!!bool`, `!!null`, `!!timestamp`
      (plan §2). Includes the YAML-1.1 bool spellings reaching `RenderInput` and
      `design.pythonBoolRepr`, which today read anything but `true` as `False`.
      `!!null` discards its text. Test: table-driven over the eleven rows of spec §3.2 that
      force a kind.

- [x] **T4 — opaque tags.**
      `!!str`, unknown tags, `!!merge`/`!!value`/`!!yaml`, and (as recorded divergences)
      `!!binary`/`!!set`/`!!omap` on a scalar all build `KindTagged` carrying the scalar's text.
      Test: `cv.name: !!str Bob` produces `KindTagged` with `Raw == "Bob"`, and an empty
      `a: !!str` produces `KindTagged` with `Raw == ""`, not a null.

- [x] **T5 — `RenderInput` names `KindTagged`.**
      The switch already falls through to `node.Raw`, which is the right answer; naming the kind
      is what keeps it right. Test: `RenderInput` of a `KindTagged` node is its text.

- [x] **T6 — a tagged theme is never a built-in theme.**
      `ValidateTheme`'s built-in loop gains a `Kind == KindString` guard, so
      `design.theme: !!str classic` takes the custom-theme path as upstream's discriminated union
      does (`design.py:57`). Test: the folder message, not the name message, and not a render.

- [x] **T7 — a tagged mapping key is `Keys should be strings.`**
      One record against the enclosing mapping, the key's text as the input, the field unbound.
      Test: `cv: {!!str name: John Doe}` gives exactly that record at `cv`.

- [ ] **T8 — record the out-of-scope tags. HUMAN GATE.** *(drafted, awaiting approval)*
      `specs/divergences.md` entries for `!!binary`, `!!set`, `!!omap`, and the four
      `ValueError`/`KeyError`/`IndexError` constructor crashes (spec §3.3, D-011 class). Stop for
      approval; do not write the file first (`AGENTS.md` §5).

**Three defects the tag work exposed, all predating it, all fixed as their own commits** —
015's acceptance criteria could not pass around them:

- [x] **T7a — `cv.name`/`headline`/`location` had no declared type**, so `cv.name: 200` rendered a
      CV named `200` at exit 0 (cv.py:32, :36, :40).
- [x] **T7b — an absent `cv.name` is `None`, an empty one is `""`.** The path resolver filters its
      placeholder table on `None` and not on falsiness (`path_resolver.py:76` against `:77-102`),
      so `cv.name: ""` writes `_IN_SNAKE_CASE_CV.typ` while an absent name keeps the literal
      placeholder; and `Preamble.j2.typ:6` interpolates `None` as four letters. The port collapsed
      both onto one Go string and read the raw token besides, naming a file `null_CV.typ`.
- [x] **T7c — `email`, `phone` and `website` ran their format checks on a non-string**, where
      pydantic reports the type failure first.

- [ ] **T9 — verify and update the ledger.**
      Re-run the 25-case differential matrix (24 from spec §3.2 plus the tagged key), then
      `go test ./...`, `just test-parity`, `just schema-diff`, `just check`. `specs/STATE.md`
      updated by the merge owner only, after a **fresh-context** `rendercv-parity-verifier`
      reports — never as part of a feature commit.
