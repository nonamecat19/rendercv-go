# Iteration 7 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

---

## Wave A — the model

### T1 — the ten-field catalog · `[sequential]`
`internal/schema/models/locale/catalog.go`: the model of spec §2, its defaults, and unknown-key
rejection.
Spec §2, §3 behavior 8.
Tests: the English defaults of behaviors 5 and 7 verbatim, `June`/`July`/`Sept` included; an
unknown key gives spec 004 §4.10's message.

### T2 — the twelve-element constraint · `[sequential]`
Both bounds, both messages, both codes, the count interpolated.
Spec §3 behavior 10. Plan §3, §5 hazard 3.
Tests: 11, 13 and 0 items, each with its own message; the codes reach a record through
`schemaerr.Coded`.

---

## Wave B — the data

### T3 — the catalog data, red · `[sequential]`
`catalogs_conformance_test.go`, `//go:build conformance`. Diffs each of the twenty-one override
files against the Go data, field by field. Declares an empty map so the package compiles. Lands
**red**.
Plan §1. Spec §5.
The test must assert **every** field of every catalog, which `require_all_fields=True` makes
meaningful.

### T4 — generate the twenty-one catalogs · `[sequential]`
**Revised from "one catalog per commit".** `AGENTS.md` §7 forbids bundling twenty-one *features*;
this is one mechanism producing data, and §10.1's rule against hand-writing what a tool can
generate is the stronger one here. 210 strings, most non-ASCII, none proofreadable by eye — Arabic
month abbreviations do not survive human review, so transcribing them would be the port's largest
single transcription risk.

`tools/localeprobe` emits `catalogs_generated.go`; `just localeprobe` reruns it.

**What this costs is written at the tool's head**: T3's diff now compares generated data against
the files it came from, so it cannot fail at generation time. Its value is drift detection after a
submodule bump, not transcription checking — a weaker guarantee than the error dictionary's, whose
thirteen rows were small enough to transcribe and whose diff therefore checked a human copy.

### T5 — English · `[sequential]`
The base catalog, written by hand because its values are Python defaults rather than a YAML file
— which is also why it is the one catalog the diff genuinely checks.

---

## Wave C — the schema

### T26 — the collision suffix · `[sequential]`
`internal/schema/jsonschema`: implement the emission-order numbering spec 005 §7.2 deferred, and
replace `DefNameWithSuffix`'s panic.
Plan §4. Spec 005 §3.3 behaviors 12–13.
Tests: `Phrases__1` through `__22` in `Languages` order, not alphabetical.

### T27 — the 45 `$defs` · `[sequential]`
`locale/schema.go`. Turns 45 rows of the per-`$defs` differential green and shrinks the absent set
from 209 to 164.
Spec §5. Reads T26.
**Update the absent count** in `jsonschema/golden_conformance_test.go` — it is written to fail
until someone does.

### T28 — close the ledger · `[sequential]`
`specs/STATE.md`: iteration 7 green, Axis 3 still blocked on iteration 6 alone.
