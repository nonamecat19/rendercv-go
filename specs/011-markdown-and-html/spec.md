# Iteration 11 — the Markdown and HTML documents

Behavior of the two remaining text artifacts, extracted from the vendored Python. No Go design
here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/renderer/{markdown.py,html.py}`,
`src/rendercv/renderer/templater/templater.py:50-155`, and
`src/rendercv/renderer/templater/markdown_parser.py:193-202`.

---

## 0. What this iteration is

**Two artifacts, one already 90% built.** Iteration 9 ported `render_full_template` and every
piece it calls; upstream takes `file_type` as a *parameter* (`templater.py:51`), so the Markdown
document is the same function with a different template directory. What is genuinely new is the
Markdown→HTML conversion.

The two artifacts are not independent: **the HTML is generated from the Markdown file's bytes**,
not from the model (`html.py:31-33`), so a wrong `.md` is a wrong `.html` and the order is fixed.

---

## 1. The Markdown document

1. `generate_markdown` (`markdown.py:9-28`) is `render_full_template(model, "markdown")` written to
   the resolved path, in UTF-8.
2. It returns `None` when `settings.render_command.dont_generate_markdown` is set, and **that
   return value is what disables the HTML** (`html.py:28-30`) — the two flags are not independent.
3. The document differs from the Typst one in exactly three places (`templater.py:76-97`):
   - the templates come from `templates/markdown/` with a `.md` extension;
   - **there is no preamble** — the document opens with the header;
   - the string-processor chain is `make_keywords_bold` **only**; `markdown_to_typst` does not run,
     because the output is already Markdown (spec 008 §3).
4. **The theme's `.j2.md` overrides are not searched.** The theme-qualified candidate path is tried
   only for Typst (`templater.py:197-202`), so a user's `classic/Header.j2.md` is ignored where
   `classic/Header.j2.typ` is honored.

## 2. The Markdown header's context is not the Typst header's

5. The Typst header reads `cv._connections`, a list of pre-formatted strings. The Markdown header
   reads the **fields themselves** — `cv.phone`, `cv.email`, `cv.location`, `cv.website`,
   `cv.social_networks` — and formats them inline (`templates/markdown/Header.j2.md`).
6. `cv.phone` is printed with `tel:` stripped and every `-` turned into a space, by two chained
   `replace` filters in the template rather than by a processor.
7. `cv.website` is printed twice: as link text with `https://` and the trailing `/` removed, and as
   the href unchanged.
8. Each social network contributes `network.network`, `network.username` and `network.url` — the
   generated profile URL of spec 004 §3.13, not a cleaned one.
9. **None of these five fields is processed** by `process_model`, which touches only `name`,
   `headline` and the sections (`model_processor.py:88-95`). So a `_` in an email survives
   unescaped, which is correct for Markdown and would be wrong for Typst.

## 3. Markdown → HTML

10. `markdown_to_html` is `markdown.markdown(markdown_string)` (`markdown_parser.py:202`) —
    python-markdown with **no extensions and no configuration**. Its defaults are therefore the
    whole specification: the default `output_format` (`xhtml`), the default extension set (none),
    and the default `tab_length` (4).
11. The input is not arbitrary Markdown. It is the document §1 produced, so the constructs that can
    actually appear are the ones the eight entry templates and the header emit: ATX headings,
    unordered lists, links, `**bold**`, `*italic*`, and paragraphs.
12. `render_html` (`templater.py:130-155`) renders `html/Full.html` with the converted body bound to
    `html_body`, plus the same four model names every fragment gets.
13. **`html_body` is indented by 8** through Jinja's `indent` filter, which does **not** indent the
    first line (spec 008 §4 pins this) — so the body's opening tag sits where the template put it
    and every later line carries eight spaces.
14. The `<html lang="…">` attribute is `locale.language_iso_639_1`, the same table the Typst
    preamble uses.
15. `Full.html` is a single fragment with no theme lookup and no per-section rendering; the whole
    document is one `render_single_template` call.

## 4. Out of scope

**4.1 The CLI flags** `-nomd` and `-nohtml` are iteration 12's. This iteration ends at two strings;
which of them gets written is the render command's decision.

**4.2 `resolve_rendercv_file_path`** — the output paths and their placeholders — is iteration 12's
as well. The corpus's `.md` and `.html` are compared as content here, not as filenames.

**4.3 PDF and PNG** are iteration 10's.

---

## 5. Acceptance criteria

- [ ] §1's Markdown document byte-identical to upstream's `.md` for every corpus case that has an
      input, by the same differential `tools/typprobe` established for the `.typ`.
- [ ] §2's five header fields, driven by a document that supplies all of them.
- [ ] §3's HTML byte-identical for the same cases.
- [ ] §3 behavior 13's indent, which is visible in every case at once and therefore proves nothing
      on its own — it needs a case whose body has more than one line.

## 6. The known hazard

**python-markdown is not goldmark, and this is where that bill comes due.** Iteration 8 ported
python-markdown's *inline* layer for `markdown_to_typst` and recorded five measured divergences in
the process — none of which have a `divergences.md` entry yet, because that file is human-gated.
`markdown_to_html` needs the *block* layer as well: paragraphs, lists, headings, and the
serializer's exact tag and newline placement.

Two routes, and the choice belongs in `plan.md` after measurement, not here:

- port python-markdown's block layer, as iteration 8 did for the inline one;
- use goldmark and normalize, which trades a known-shaped porting job for an unknown-shaped
  difference hunt.

**What decides it is a measurement**: run both over the `.md` documents the corpus actually
produces and count the differing cases. The input is narrow (§3 behavior 11), so the honest answer
may be that neither library's general behavior matters much — but that is a claim to test, not to
assume. Iteration 8's spec §8 made exactly this kind of assumption and hid a real bug behind it.
