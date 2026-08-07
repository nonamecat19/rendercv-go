# Iteration 9 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

Two inherited items were closed at the head of this iteration, before T1:

- `process_date` / `render_entry_templates` — iteration 8's under-scoped units.
- **T10, the effective per-theme option tree** — iteration 6's, `design.Effective`, 7/7 against
  upstream's own resolved model.

---

| # | Unit | Criterion | State |
|---|---|---|---|
| T1 | `entries.Dump` — declared fields to `map[string]any`, plus the `YearOnly` set | 16 cases against `model_dump(exclude_none=True)`, `tools/dumpprobe` | **done** |
| T2 | `bridge.Sections` — `cv.SectionRecords` to `[]process.Section` | spec §1: the empty section's `TextEntry`, and the title round trip that is not the identity | **done** |
| T3 | `bridge.Connections` — `cv._key_order` to `[]process.Connection` | spec §2, driven by a document whose keys are not in field order | **done** |
| T4 | phone formatting through `nyaruka/phonenumbers` | the three formats over seven numbers, `tools/phoneprobe` | **done** |
| T5 | `bridge.Model` — the whole `process.Model`, design and locale wired in | every field populated, against a hand-checked document | **done** |
| T6 | `typstdoc.Render` — orchestration to a string | renders without error; whitespace is `Assemble`'s | **done** |
| T7 | the first corpus case's `.typ` **byte-identical** | Axis 1's first passing case | **done** |
| T8 | every remaining corpus case, all nine entry types | 21/21 byte-identical, `tools/typprobe` | **done** |

Three units the work turned up that the split did not predict, each its own commit:

| # | Unit | Why |
|---|---|---|
| T1a | `settings.Resolve` | the renderer is the first reader of `bold_keywords`, `pdf_title` and `_resolved_current_date` |
| T1b | `locale.Resolve` plus `ISOCode` / `IsRTL` | the preamble reads two computed properties the catalog did not carry |
| T1c | per-entry `YearOnly` | it sat on the model, so one entry's `start_date: 2000` made **every** entry format as a year |

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

**It did, and it was — twice.** 17 of 21 cases failed on first run, and neither cause was in the
bridge:

1. `render_entry_templates` took `doi_url` as an *input*. It is the publication entry's own
   computed property (`publication.py:79-95`), so nothing supplied it and every publication linked
   to its `url` field instead of to its DOI — with the DOI text already correct beside the wrong
   href, which is what makes it invisible to a reader.
2. `design.Effective` passed a theme's colour through as written. The declared defaults are
   already `as_rgb()`-normalized and the theme YAML is not, so `rgb(0,0,0)` reached the template
   where upstream renders `rgb(0, 0, 0)` — three themes off by two spaces.

Both were in code that shipped green in iterations 6 and 8, with unit tests that passed. The
fixture found them in its first run, which is the same lesson iteration 8's fragment differential
recorded and the reason this one was written before the code that had to satisfy it.
