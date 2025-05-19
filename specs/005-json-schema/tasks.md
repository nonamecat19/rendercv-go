# Iteration 5 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

The whole iteration is one owner's: the generator is the pipeline spine's tail, and every task
after T1 reads the previous one's output. Nothing here fans out.

---

## Wave A — the encoder

### T1 — the ordered object · `[sequential]`
`internal/schema/jsonschema/jsonschema.go`: `Object`, `Set`, `Sort`.
Plan §3. Spec §6.
Tests: `Set` overwrites **in place** and does not move the key; `Sort` is ASCII and leaves values
alone; the top-level sequence of spec §3.2 behavior 8 is reproducible by sorting then setting
three more keys — the one case that distinguishes in-place from append-on-overwrite.

### T2 — the serializer, red · `[sequential]`
`internal/schema/jsonschema/marshal.go` and its test. Two-space indent, `": "`, `", "`, literal
non-ASCII, no trailing newline, `null` for a nil value.
Plan §4. Spec §3.4, §5 behavior 18.
Tests: each of the four table rows of plan §4 as its own assertion, **including the `<`, `>`, `&`
row that the current schema cannot exercise** (plan §7 hazard 2); a nested object; an array of
mixed kinds; `null`.

---

## Wave B — the model descriptors

Each adds one `Schema()` and turns its `$defs` row green. They share no file except within a
package, so they are ordered but not entangled.

### T3 — `DefName` and the collision panic · `[sequential]`
Plan §5. Spec §3.3 behaviors 11–13, §7.2.
Tests: a bare name and a qualified one; the panic names iteration 6.

### T4 — the leaf types · `[sequential]`
`ArbitraryDate`, `ExactDate`, `SocialNetworkName`, `TypstDimension`,
`ExistingPathRelativeToInput`. All are one- or two-key objects.
Spec §8. Note `ArbitraryDate` is `integer, string` and `ExactDate` is `string, integer` — the arm
order of spec 004 §3.9b, showing up a second time.

### T5 — the nine entry types · `[sequential]`
`internal/schema/models/cv/entries/schema.go`. Nine `Schema()` functions plus `ListOfEntries`.
`TextEntry` contributes **no** `$defs` entry (spec §8) and `ListOfEntries`' first arm is its
`array of string`.
Spec §8. Reads T3, T4.

### T6 — `SocialNetwork`, `CustomConnection`, `Section` · `[sequential]`
`internal/schema/models/cv/schema.go`.

### T7 — `Cv` · `[sequential]`
Same file. Ten properties in declaration order (spec §3.2 behavior 10).

### T8 — the top level · `[sequential]`
`internal/schema/models/schema.go`: `RenderCVModel`'s object, the four added keys of spec §3.1,
and `$defs` assembled and sorted.
Spec §3.1, §3.2 behavior 8, §4, §5 behaviors 16–17.

---

## Wave C — the gate

### T9 — the per-`$defs` differential · `[sequential]`
`internal/schema/jsonschema/golden_conformance_test.go`, `//go:build conformance`. The
round-trip-both-sides comparison of plan §6, over the eighteen `$defs` of spec §8.
Spec §8 **[diff]**. Plan §6.

### T10 — the absent-set test · `[sequential]`
Same file: every `$defs` key upstream has and the port does not, listed explicitly, failing if the
list shrinks without models landing.
Spec §8, §7.1.

### T11 — the `schema` command · `[sequential]`
`internal/cli`: cobra's `schema` subcommand, writing to stdout, exit 0. This is the port's first
CLI command; iteration 12 owns the rest of the surface, and this task adds **only** this one.
Spec §2.
Tests: the command writes the same bytes the generator produces, and exits 0.

### T12 — record the Axis 3 status · `[sequential]`
`specs/STATE.md`: Axis 3 becomes *blocked on iterations 6–7* with spec §1's table as the reason,
and iteration 5 goes green on its own criteria. `just schema-diff` stays red **by design** and the
ledger says so, so nobody reads it as a regression.
Spec §1, §7.1, §8.

---

## Marker ledger

No `TODO(iteration-5)` exists in the tree today. This iteration adds exactly one, at
`jsonschema.DefName`'s collision branch, cleared by iteration 6.
