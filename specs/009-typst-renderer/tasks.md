# Iteration 9 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

Two inherited items were closed at the head of this iteration, before T1:

- `process_date` / `render_entry_templates` — iteration 8's under-scoped units.
- **T10, the effective per-theme option tree** — iteration 6's, `design.Effective`, 7/7 against
  upstream's own resolved model.

---

| # | Unit | Criterion | State |
|---|---|---|---|
| T1 | `entries.Dump` — declared fields to `map[string]any`, plus the `YearOnly` set | one case per entry type, differentially against `model_dump(exclude_none=True)` | |
| T2 | `bridge.Sections` — `cv.SectionRecords` to `[]process.Section` | spec §1: the empty section's `TextEntry`, and the title round trip that is not the identity | |
| T3 | `bridge.Connections` — `cv._key_order` to `[]process.Connection` | spec §2, driven by a document whose keys are not in field order | |
| T4 | phone formatting through `nyaruka/phonenumbers` | the four `phone_number_format` values against upstream's four | |
| T5 | `bridge.Model` — the whole `process.Model`, design and locale wired in | every field populated, against a hand-checked document | |
| T6 | `typstdoc.Render` — orchestration to a string | renders without error; whitespace is `Assemble`'s | |
| T7 | the first corpus case's `.typ` **byte-identical** | Axis 1's first passing case | |
| T8..T15 | one corpus case per remaining entry type | one at a time, never as a bundle | |

---

## Notes carried into the work

**T1's `YearOnly` is not cosmetic.** `date: 2020` and `date: "2020"` produce different output and
the port sees the same text for both, so the tag has to be read at dump time — after that the node
is gone.

**T3 raises `RenderCVInternalError` in four places upstream** (a key in `_key_order` whose value is
None). Those are unreachable from a validated document by construction, since `_key_order` drops
null-valued keys. The port returns an error rather than panicking, and the test that a validated
document never triggers one is part of T3.

**T7 is the gate, not T6.** Everything before it is checkable only against itself; the golden is
the first thing that can find a wrong rewrite in `tools/gentemplates`. Expect the first run to fail
and expect the cause to be somewhere other than this iteration's code.
