# Iteration 6, addendum — the settings schema

Behavior of the `settings` block, extracted from the vendored Python. No Go design here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary source: `src/rendercv/schema/models/settings.py`.

---

## 0. Why this file exists, and what it does not cover

**Written after part of it shipped.** `internal/schema/models/settings/schema.go` landed with
iteration 6's T15 to close Axis 3, against `AGENTS.md` §4's rule that no Go code precedes a spec —
a `rendercv-parity-verifier` finding, recorded here rather than in a commit message that would
scroll away.

The retrofit covers exactly what shipped: the **schema projection** of the four `Settings` fields
and the thirteen `RenderCommand` ones. It does **not** cover their behavior — nothing reads a
`dont_generate_*` flag, resolves an output path, or interprets a placeholder — and §4 says which
iteration does.

**It lives here rather than under a number of its own.** `008` is the templater's and `012` is the
CLI's, both already cited from `STATE.md` and from `TODO` markers in the tree; taking either would
mean renumbering an iteration that other files point at. The work shipped as iteration 6's T15, so
it is documented as iteration 6's addendum, and `spec.md` §5 links here.

---

## 1. The shape

1. `Settings` is a `BaseModelWithoutExtraKeys` with four fields in declaration order:
   `current_date`, `render_command`, `bold_keywords`, `pdf_title`. Every one has a default, so an
   absent `settings` block is legal and so is an empty one.
2. `RenderCommand` is a `BaseModelWithoutExtraKeys` with thirteen: `output_folder`, `design`,
   `locale`, `typst_path`, `pdf_path`, `markdown_path`, `html_path`, `png_path`, and the five
   `dont_generate_*` flags. It mirrors the `render` command's flags, and the settings file is the
   lower-precedence half of that pair — **CLI arguments win**, which its own description says.
3. **`settings` is not a discriminated union**, unlike `design` and `locale`. It keeps every
   element of an error's location, which spec 004 §3.3 behavior 9 already measured and which is
   why it was never in `discriminatedRoots`.

---

## 2. The three value types

4. **`current_date`** is `datetime.date | Literal["today"]` — a union with **no null arm** and a
   `const` arm. Its schema is an `anyOf` of `{format: date, type: string}` and
   `{const: today, type: string}`, and its title is the explicit `Date` rather than the derived
   `Current Date`.
5. **`PlannedPathRelativeToInput`** is a path that need not exist yet; `cv`'s
   `ExistingPathRelativeToInput` must. Their **schemas are identical** — `format: path` carries no
   validation weight — and they are two `$defs` entries because they are two Python types, not
   because they render differently.
6. `render_command.design` and `.locale` are `ExistingPathRelativeToInput`, **nullable, defaulting
   to null**: they name a YAML file carrying that block, which is the overlay mechanism spec 002
   models. The five output paths are `PlannedPathRelativeToInput` with string defaults.

---

## 3. Two things in the schema that are not rules

Both are upstream inconsistencies, reproduced because Axis 3 is a byte contract.

7. **`markdown_path` carries an explicit `title` and the other four output paths do not.** A bare
   `$ref` normally has its title omitted (spec 005 §3.2), and an explicit `Field(title=…)` survives
   the omission — so this is one field written differently from its four siblings. Deriving a rule
   would give four titles or none.
8. **`png_path`'s description reads "Output path for PNG files"**, with no article and a plural,
   while the other four read "the Typst file", "the PDF file" and so on. Interpolating a fixed
   `the ` would be one wrong description of five.

---

## 4. Out of scope

**4.1 Every behavior these defaults drive is iteration 12's.** The `dont_generate_*` flags, the
output-path placeholders, the precedence of CLI arguments over settings, and `current_date`'s
effect on filenames and time spans all belong with the CLI.

**The *validation* of these fields is no longer deferred**, and is not iteration 12's:
[`spec-delta-settings-validation.md`](spec-delta-settings-validation.md) specifies the type of every
field in the tree and the exact record each wrong type produces, measured. A 126-vector sweep found
the whole tree unvalidated — upstream exits 1, the port renders at exit 0 — which is an Axis 2 and
Axis 4 defect, not a missing CLI feature. Placeholders and flag *effects* stay here in §4.1.

**4.2 `current_date`'s validation is already ported**, thinly: spec 004 §7.9 pulled a shape check
forward so the 25-record differential could reach twenty-five. Its message is deliberately the
pipeline's override rather than a pre-substituted one.

**4.3 `bold_keywords` is read by the renderer**, iteration 8's templater, not here.

---

## 5. Acceptance criteria

- [x] All three `$defs` byte-identical: `Settings`, `RenderCommand`, `PlannedPathRelativeToInput`.
      Landed with iteration 6's T15 and gated by the per-`$defs` differential.
- [x] `just schema-diff` exits 0 — these were the last three entries.
- [x] Unknown keys rejected in both models, with spec 004 §4.10's message. Landed since
      (`internal/schema/models/settings/settings.go:84-111`); measured byte-identical to upstream
      for `settings.no_such_key` and `settings.render_command.no_such_key`.
- [ ] The full `RenderCommand` model — its **types** are specified and measured in
      [`spec-delta-settings-validation.md`](spec-delta-settings-validation.md) §2, with the five
      open mechanisms in §3 and the unit breakdown in §12. Its **precedence** against CLI flags is
      iteration 12's; that delta's §9 records what is already correct and what is not.

---

## 6. Status

**Complete for the schema projection, which is what shipped.** Everything else is named in §4 with
the iteration that owns it, and §5 marks which criteria are open rather than implying the block is
finished.
