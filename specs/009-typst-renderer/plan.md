# Iteration 9 — plan

The Go design for `spec.md`. Packages, types, dependencies, tradeoffs. No behavior claims here that
the spec does not already carry.

---

## 1. Where the code goes

| Upstream | Go | Why there |
|---|---|---|
| pydantic's `model_dump(exclude_none=True)` | `internal/schema/models/cv/entries.Dump` | dumping is the model's own job upstream; the entry package already owns the declared field order the dump must follow |
| `templater/connections.py:60-180` (`parse_connections`) | `internal/renderer/bridge` | it reads `cv._key_order` **and** `design.header.connections`, so it belongs to neither package alone |
| `renderer/model_processor.py`'s caller side | `internal/renderer/bridge` | the assembly of a `process.Model` |
| `templater.py:50-127` (`render_full_template`) | `internal/renderer/typstdoc` | the orchestration that ends at a string |

**A new package rather than growing `templater`.** `process` is deliberately downstream of
`models` (spec 008 plan §4) — it never imports the schema. The bridge is exactly the code that
imports both, so putting it in `templater` would drag the schema into the engine's import graph
and delete that separation. `bridge` imports `models/*` and `process`; `process` still imports
nothing of the schema.

## 2. The entry dump

`process.Entry.Fields` is `map[string]any`, which is already the shape `model_dump` produces. What
the port must supply is the projection from `*yamldoc.Node` to a value:

- a scalar node → its text;
- a sequence node → `[]string` of its elements' text;
- an absent field → **omitted**, which is `exclude_none=True`;
- a field present but written `null` → also omitted; the binder leaves it nil.

**The integer date is the one case where the node's text is not enough.** `date: 2020` dumps as
the *integer* `2020` upstream, and `FormatDateRange` branches on `isinstance(x, int)`
(spec 008 §4C). The port cannot re-derive that from `"2020"` — `date: "2020"` is a string and
formats differently. So `Dump` returns a second value, the `YearOnly` set that
`process.EntryTemplateInput` already declares, read off the node's YAML tag rather than its text.

`Dump` is driven by the same `[]binder.Field` the validator uses, so a field added to an entry
cannot be forgotten by the dump — that is the reason it lives in `entries` and not in `bridge`.

## 3. Phone formatting is a dependency decision

`parse_connections` formats a phone through **libphonenumber** (`phonenumbers.format_number`),
whose output for a given `phone_number_format` is not reproducible by hand — it is a data table of
per-region formatting rules. Go's option is `github.com/nyaruka/phonenumbers`, a generated port of
the same upstream metadata.

**This is a library substitution, not a divergence**, on the same reasoning as goldmark: upstream's
choice is a Python package the port cannot call, both sides are generated from Google's
libphonenumber metadata, and the user-visible output is the formatted number. If the two ever
disagree on a corpus number, *that* is a divergence and gets an entry.

## 4. The photo — spec §4 behavior 15

`download_photo_from_url` is the pipeline's only network access. Design:

```go
type PhotoFetcher func(url string) ([]byte, error)
```

on the render options, defaulting to nil. **Nil means the download is skipped**, and every test
leaves it nil. The CLI (iteration 12) supplies the real fetcher.

This is not a behavior change waiting to be recorded: with a nil fetcher no corpus case differs,
because no corpus case has a URL photo. What it buys is that a conformance run cannot reach the
network — a test that does is worse than one that does not, and the failure mode of the
alternative (a flaky suite that passes offline by accident) is invisible.

## 5. Orchestration

```go
func Render(doc Document, opts Options) (string, error)
```

builds the `process.Model`, calls `process.Run(model, process.FormatTypst)`, renders `Preamble`,
`Header`, and per section `SectionBeginning` / the entry template / `SectionEnding` through
`templater.Environment`, and hands the pieces to `templater.Assemble`.

**`Assemble`'s separators are already pinned** (iteration 8), so this function contributes no
whitespace of its own. If the first golden diffs on whitespace, the bug is in the transform or in
`Assemble`, not here — which is why they were pinned first.

## 6. What this iteration does not build

The Markdown document (iteration 11) reuses every piece above with `FormatMarkdown` and
`typst=false`; the split exists so the first `.typ` golden is not gated on it. The CLI is
iteration 12's; the corpus runs through a test harness here.
