# Iteration 11 — tasks

| # | Unit | Criterion | State |
|---|---|---|---|
| T1 | rename `typstdoc` → `document`, `Render` takes a format | the Typst suite unchanged | **done** |
| T2 | `tools/docprobe` captures `.md` and `.html` too | 24 cases × 3 artifacts | **done** |
| T3 | the Markdown document | 24/24 `.md` byte-identical | **done** |
| T4 | `bridge.MarkdownFields` — the header's five contact fields | each field's removal breaks 18–21 cases | **done** |
| T5 | `markdown_to_html` — python-markdown's block layer | 24/24 `.html` byte-identical | **cut, see plan §2** |
| T6 | `render_html` + `Full.html` | gated on T5 | **cut** |

---

## Why T5 and T6 were cut rather than attempted

The library question spec §6 said to measure was measured: goldmark agrees with python-markdown on
**8 of 24** documents, and the 16 disagreements are all loose-versus-tight list structure. That is
a block-layer difference, so the cheap route — goldmark plus a normalizing post-pass — does not
exist.

What remains is a port of python-markdown's block layer, bounded by the narrow input (spec §3
behavior 11) but comparable in size to iteration 8's inline port. Shipping half of it would mean an
iteration marked done with a failing conformance case, which `AGENTS.md` §10.2 forbids.

**The gate is already built**: `expected.html` sits beside every case, so T5 begins with a red test
that costs one loop to enable.
