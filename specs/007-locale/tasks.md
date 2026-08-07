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

### T4–T24 — one catalog per commit · `[parallel]`
Twenty-one commits, one per language, in `Languages` order. `AGENTS.md` §7 forbids bundling them.
Each turns one row of T3 green.

### T25 — English · `[sequential]`
The base catalog, whose values are defaults in Python rather than a YAML file.

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
