# Port ledger

Living state of the rendercv-go port. **Updated only by the merge owner, after
`rendercv-parity-verifier` reports.** Never edited by a porter as part of a feature commit.

Upstream target: `third_party/rendercv` @ `v2.8` (`2eba248`)

Legend: `—` not started · `spec` spec written · `red` tests written, failing ·
`wip` implementation in progress · `green` all its conformance cases pass

---

## Iterations

| # | Subsystem | Spec | Status | Conformance cases passing |
|---|---|---|---|---|
| 0 | Bootstrap (layout, AGENTS.md, submodule, agents, skills, CI) | — | green | n/a |
| 1 | Conformance harness (corpus, gengolden, helpers) | [001](001-conformance-harness/spec.md) | **audited — FAIL, demoted.** Comparison path sound (6/6 mutations caught); the goldens bake absolute paths and the generation month | n/a (42 cases red by design) |
| 2 | YAML reader + core model (RenderCVModel, CV, Section) | [002](002-yaml-and-core-model/spec.md) | **green — the cut scope is now empty.** Audited 2026-08-11: **all six carried items are closed**, each with the commit that closed it (see the Cut scope section, which is kept as a record rather than a backlog). Nothing regressed | n/a (gated on unit tests, spec §7.2) |
| 3 | Entry types (9) | [003](003-entry-types/spec.md) | **re-audited — FAIL again.** Three of five closures hold; two of my repairs introduced new defects | n/a (gated on unit tests, spec §7.1) |
| 4 | Validation-error parity | [004](004-validation-errors/spec.md) | green | n/a (gated on the 25-record differential, spec §7.3) |
| 5 | JSON Schema generator | [005](005-json-schema/spec.md) | **verified green** — audited and passed, the only iteration besides 9 to do so | n/a (gated on the 18 owned `$defs`, spec §7.1) |
| 6 | Design & themes (9) + the settings schema | [006](006-design-and-themes/spec.md) | **field-order interleave fixed 2026-08-11 (`e25215a`, pinned by `838d01d`) — the one real finding from the prior audit is closed.** `validateModel` used to run `binder.Bind` for shape then a separate per-field loop for value/enum, so all type errors preceded all value errors; now shape errors are grouped by field and interleaved into the declaration-order loop. Measured on `page.size: not-a-size` + `page.top_margin: {}`: `size` then `top_margin`, matching upstream. **New, open, found by a full-repo re-audit 2026-08-11**: a `design:` block with no `theme:` key crashes upstream (`KeyError: 'theme'`, unhandled exception, 10133 B traceback) but the port exits 0 and renders a complete document (880 B success panel) — `validate.go:140-144` deliberately returns no validation error here (see its comment) because producing one would itself be a divergence from an unhandled-exception shape; the fix belongs with iteration 4/12's unhandled-exception handling, not a bare validation message, and needs a scoped decision before it's built | n/a (gated on the 164 `$defs` differential and the override diff, spec §5) |
| 7 | Locale (English + 21 catalogs) | [007](007-locale/spec.md) | **both `phrases` gaps fixed 2026-08-11 (`f82d097`) — closed, a full-repo re-audit confirmed both probes byte-identical.** `binder.ValueModel` used to only check `phrases` was a mapping and never recurse; `ValidatePhrases` now binds the nested `Phrases` model directly, so (a) a wrong-typed `degree_with_area` is `Input should be a valid string.` at exit 1 instead of silent corruption at exit 0, and (b) an unknown key inside `phrases` is `Extra inputs are not permitted` instead of being dropped. `month_names`/`month_abbreviations` were already correctly validated (re-confirmed). The short-month-list panic stays fixed, `monthAt` bounds-checked both directions | n/a (gated on the 45 `$defs` differential and the submodule catalog diff, spec §5) |
| 8 | Templater (pongo2 env, filters, markdown→typst, processors) | [008](008-templater/spec.md) | **all five known `markdown_to_typst` divergences fixed 2026-08-11**, one commit each: backtick width-matching (`6bf7eca`), an image contributing nothing (`19bd16a`), autolinks including decimal-entity mail obfuscation (`024fbde`), raw inline HTML passthrough (`1adbd49`), and an admonition block parsed as one joined unit instead of per-line (`c643181`). Confirmed byte-identical `.typ` end-to-end on all four built-in themes for a synthetic CV covering all five constructs; fixture rows in `internal/renderer/templater/process/testdata/markdown_to_typst.json` grew 101→166, measured from the vendored Python, not hand-written. **New, sixth, unrecorded divergence found while fixing #4**: the raw-HTML stash also covers `ENTITY_RE` (priority 80 in upstream's registry) and a decimal character entity like `&#35;` was being escaped to `\#35;` — same root cause, not yet fixed. **Two gaps left deliberately** (declining rather than misrendering): reference-style links/images have no case (the line-at-a-time Typst path never builds the link-definition map the block pass needs), and `BACKTICK_RE`'s `(?<!\\)` guard is unimplemented | n/a (gated on the 52-fragment Jinja differential and 240 unit cases, spec §7) |
| 9 | Typst renderer (`.typ` emission) + iteration 6's T10 + iteration 8's Wave C | [009](009-typst-renderer/spec.md) | **green** — verified by a fresh context, which returned FAIL on four items; all four fixed and pinned | 24 / 24 |
| 10 | wazero + WASI typst → PDF, then PNG | [010](010-typst-compilation/spec.md) | **gate cleared 2026-08-08; landed and running in the suite.** The compiler, the fonts and `fontawesome` are vendored and embedded (D-007). Every render case now produces a PDF and its PNGs, and `AssertPDF` compares text, page count and geometry. **Not yet verified by a fresh context** | 14 / 14 in the suite |
| 11 | Markdown + HTML renderers | [011](011-markdown-and-html/spec.md) | **both blockers closed; awaiting a fresh context to re-promote.** Raw HTML passes through, and the `"` defect that demoted this iteration is fixed: python-markdown escapes in **three contexts** where goldmark uses one. Measured in an isolated worktree at the commit before the fix, 33 of 75 differential shapes differed; 4 do now, each a different defect (whitespace, emphasis nesting, URL escaping) and each pinned by an inverted assertion. **verified 2026-08-11 — FAIL, stays demoted.** The 71/75 figure is a property of the 75-row fixture, not of the renderer: at least **8 further divergence classes** are reachable end-to-end from an ordinary CV highlight, none in `knownRemainder`, none in `divergences.md` (emphasis-nesting split, image `alt` losing emphasis, the decimal-entity mailto obfuscation, a ``` fence python-markdown has no extension for, list-continuation `<p>` placement, a spaced link URL, a raw block `<div>` in a list item, and a bare `\r`). What *did* check out: all 75 `html.json` rows reproduce byte-for-byte under the submodule's own `markdown.markdown` (0 mismatches, §10.1 satisfied), and all four `knownRemainder` keys still genuinely differ, so the inverted assertions are load-bearing **Re-verified 2026-08-11 after the fixes: all 8 classes closed, and 5 more the fix work found — including the one that matters operationally, a CV field ending in a stray space producing a differing `.html`. `html.json` is 118 rows, all reproducing under the submodule's own `markdown.markdown`. NOT promoted**: three real divergences remain (emphasis nesting, a link destination with an unbracketed space, a block tag in a list item), each reachable from an ordinary CV, and `divergences.md` is human-gated. **Re-audited 2026-08-11, and the framing above is wrong.** All five `knownRemainder` keys were re-run through the submodule's own `markdown.markdown` and **all five still differ** — no vacuity this time. But **none of the three is parity-*impossible*, which is what the human gate is for.** Checked against goldmark v1.8.5's source: emphasis can be taken over wholesale through `parser.WithInlineParsers` at the same trigger characters — a real reimplementation of python-markdown's two regex tree processors, comparable in kind to the hand-written `markdown_to_typst` transform this port already has; the spaced link destination cannot *reuse* goldmark's unexported `linkLabelState`, which the port's comment says accurately, but can be reimplemented from the same bracket-balance rule at `inlinepatterns.py:716-830`; and the block tag in a list item is arguably the cheapest of the three — python-markdown itself solves it with a stash-and-restore **preprocessing pass over the raw string**, which nothing prevents doing before `Convert`. So these are **expensive engineering, not impossibility**, and the gate is meant for the latter (the binary-name case). **They should not be routed to `divergences.md`; iteration 11's spec should carry them as open work.** Also narrower than the phrase suggests: `***bold italic***` and `**_bold italic_**` — how a CV bullet actually writes bold-italic — match goldmark exactly. The divergent shapes are literal `___x___`, an `*em*` reopening around a nested `**strong**`, and intraword `_`. Ranked by reach, the spaced link destination is first, not emphasis | 24 / 24 corpus, **113 / 118 HTML differential**; the 5 pinned by inverted assertions, each confirmed still-differing end-to-end. Three of the previous four `knownRemainder` entries had gone **vacuous** — asserting a difference that no longer existed — and were removed |
| 12 | CLI (`new`, `render`, `create-theme`, overrides, watcher) | [012](012-cli/spec.md) | **re-verified 2026-08-11 — PASS on all five fixes; NOT promoted, one major open.** The blocker, the three gating gaps and the inverted guard are all fixed and mutation-checked by the fresh context that failed them. **N1 stays open and is why this row is not green**: `render --watch <unreadable input>` never returns, where upstream exits 1 — iteration 13's defect, widened by the blocker fix. See the re-verification section below. Original FAIL follows for the record. **FAIL. 1 blocker, 4 majors, 5 minors.** Nine of eleven `gaps.md` findings reproduce as described and are mutation-confirmed to be gated; the full 17-option flag inventory was re-derived from upstream source and matches, and the help geometry holds at 136 lines with 6 differing, all carrying the binary name, at three console widths. **What fails is the gating, not mostly the behavior.** Blocker: click's `ignore_unknown_options` makes an unknown option an ordinary *argument*, so upstream fills `INPUT_FILE_NAME` from it — `render --nope`, `--version`, `-x`, `--helpx` are all **exit 1** upstream (a `FileNotFoundError` on a file of that name) and **exit 2** here, and the same rule mis-assigns the input: `render --typ out.typ CV.yaml` opens `--typ` upstream, `out.typ` here, dropping the real filename into the extras. Not covered by D-011. Majors, each mutation-proven: **G-6 is gated by nothing** (reverting it leaves the whole suite green); **the invented-long-name class is gated by nothing** — re-adding `--nopdf` as a real flag leaves everything green, and that is the class that hid longest; **two of the three override rules are ungated** (the discriminating vectors `--nope -nopdf` and an unknown `--key=value` have no case, while the existing cases pin shapes the mutations do not touch); and **`new_typst_templates`' inverted assertion passes for the wrong reason** — `AssertUnreachable` gets un-rebound stdout and compares file *names* only, so the case is held up by D-009's binary name, not D-008, and a port emitting upstream's Jinja source would not be noticed. Minors: D-011's class is wider than its two named vectors (missing overlay files, a directory as input, every unknown-token vector above); `spec.md` §2 behavior 10 says an unknown override key is a validation error where upstream raises `KeyError` and tracebacks; `gaps.md` is stale in three places, all from iteration 13; `1cfaf15` bundles G-1 and G-2; and `TestUnmappedParserMessageFallsThrough` still skips, voiding itself, in the same area as the still-open bad-indentation location defect | 42 / 42 `TestParity` (8 inverted, one of them for the wrong reason), `just check` 0 issues, `schema-diff` empty, ~120 differential invocations, 13 source mutations | `render` and `new` are wired and their goldens pass, error panels included. `create-theme` is now registered and `--create-typst-templates`/`--create-markdown-templates` write their folders; two of their corpus cases stay red by construction under D-008. `err_not_yaml` is fixed. The five help panels are written and verified, four unreachable by construction under D-010; two more are D-011's unhandled-exception tracebacks | 34 / 42 `TestParity` cases (the suite has 42, not 43 — `manifest.json` was miscounted as a case), **not yet verified by a fresh context** |
| 13 | Parity closeout (sample generator, version, error handler, watcher, packaging) | [013](013-parity-closeout/spec.md) | **verified 2026-08-11 — PASS, 0 blockers.** Verified three times by one fresh context: the initial pass (3 majors, 4 minors), a scoped re-pass over the four fixes (PASS, 3 new minors), and a final scoped pass (PASS, 1 new minor). Findings 5–11 are closed. **The first iteration in this port to pass a fresh context on its first attempt.** Its "2 majors and 4 minors stay open" list was later deleted by `308b6f4`'s trim and lost from this file; **recovered from git history 2026-08-11 and re-verified — all 6 still reproduce, unfixed**, listed here so they aren't lost again: (major) 9 of a 13-commit range fail `go vet` on an intermediate commit — a red *compile*, not a red *assertion*, six commits unbisectable, history not rewritten; (major) `tools/sampleprobe` is still not deleted, and spec §8's own 198-case criterion still depends on the fixture it generates — self-contradictory, **needs a human decision**; (minor) that 198-case criterion is an md5 digest against a captured fixture, not a live differential; (minor) `internal/cli/oserror.go`'s documented OS-Error slack (P-3, human-gated proposal, unwritten); (minor) `panic` was unbanned in package `cli` — **fixed same day, see below**; (minor) `testdata/golden/err_unknown_theme` still bakes this machine's absolute path — same as iteration 1's finding | 42 / 42 `TestParity`, 3468 verbose PASS / 0 FAIL / 1 SKIP under the conformance tag, 7 / 7 behavior-31 differential rows |
| 14 | Lua-scripted custom themes (D-002) + the two folder messages | [014](014-lua-custom-themes/spec.md) | **re-verified 21×, 2026-08-10 — FAIL every pass through the 21st.** Pass 21 fixed a blocker (script default value-checking); 1 major + 1 minor still open, see below. **re-verified 16×, 2026-08-10 — FAIL every pass through the 16th.** Pass 16 found and fixed (`7947929`) the one blocker it turned up: `parseNumericText` applied its bool-word/hex/octal/binary coercion to every colour-tuple element's raw text regardless of the YAML node's actual kind, so a *quoted* string that merely spelled the same token (`colors.name: ["0x10", 0, 0]`) was coerced the same as an unquoted one — upstream's `float(value)` runs on whatever Python object YAML resolved, and a quoted `"0x10"` is a `str`, which `float()` rejects. `parseChannel`/`parseAlpha`/`parseNumericText` now take a `coerce bool`; `validColorNode` sets it from each element's real `yamldoc.Kind`, and the percent branch is always non-coercing (only a string can carry a literal `%`). `ParseColorTuple` (the merge-time caller with no Kind info left by then) keeps the old permissive behavior. Landed with 5 new cases. Two more findings recorded, not fixed: a colour tuple with a nested collection (`[1,2,3,[1]]`) as its 4th element silently drops it as "no alpha" and renders at exit 0, where upstream's `float()` on a `CommentedSeq` is an unhandled `TypeError` — a *port succeeds where upstream crashes* case, the same class as the two behaviors already awaiting the human gate in this file's "Two measured behaviors" section, left alone for the same reason (matching an uncaught Python traceback is D-011's territory, not a validation record this port can produce); and a script's `KindColor` **string** (not tuple) is never `ParseColor`-checked by `ValidateScript`, an asymmetry inside the already-cut "value validation skipped when a script exists" gap rather than a new one. Pass 15's three fixes (chained NaN comparisons, percent-alpha routing, prefix case-sensitivity) confirmed correct and complete on 60+ probes including `.inf`/`.nan` spellings, negative infinity, and whitespace-plus-hex/bool combinations. Pass 15's two out-of-scope findings and the `df4a82a` commit-bundling minor are unchanged. | 0 blockers reproduced by the 16th pass's fix (unverified by a 17th), 2 deferred findings (1 minor), 2 out-of-scope findings, 2 new minors recorded (upstream-crash case, script-string-color asymmetry), 2 known gaps cut forward, 1 coverage gap open, 3 process findings open |
| 15 | Explicit YAML tags | [015](015-yaml-tags/spec.md) | **implemented, not yet verified by a fresh context.** The reader had no case for a tag node at all, so every tagged node became `KindNull`. **62 of a 71-case differential matrix are byte-identical, up from 1**; the 9 remaining are all recorded (D-012's three groups, or upstream `RenderCVInternalError`s tags did not cause). **Six defects the work *exposed* and that predate it are fixed here too** — the acceptance criteria could not pass around them, and three of the six rendered a wrong CV at exit 0. **verified 2026-08-11 — FAIL, demoted. 3 blockers, 6 majors, 2 minors.** The central design bet — a new `KindTagged` that typed fields reject *by not naming it*, enforced by `golangci-lint`'s `exhaustive` check — has a hole: the mechanism only works inside a `switch` the linter sees. `binder.go:479-482`'s `isNonScalar` is a **bool predicate**, so a `TaggedScalar` at a date field routes to `checkScalar` and **renders at exit 0** where upstream emits a 2020-byte table (B1). `social_networks[].network` is the **fifth** shapeless `binder.Field` with a typed upstream annotation, the answer to the open question the previous sweep left (B2). And `boolAsInteger` is a **sixth** `BoolIsTrue` call site that was missed, still deciding truth by `HasPrefix(raw,"t")` under a doc comment iteration 15 itself made false (B3). Majors: `locale.language`'s union-tag Input Value is wrong **on plain YAML**, and `cv.photo` reports a file-existence error where upstream reports a path-type error. Coverage: spec 015 §3.4 claims *every* typed field rejects a tag and tests exactly three — B1 and B2 are two nobody probed **All five findings fixed and independently red-checked 2026-08-11 (B1 `283b448`, B2 `ece367c`, B3 `224cd81`, M1 `a671a11`, M2 `185252d`), each failing on its assertion after a clean build. Still NOT green**, because the *systemic* half of B1 is open: `fitsNoScalarArm` is now an exhaustive switch the linter enforces, but it is **one predicate out of 115** — the rule "any later predicate over Kind belongs in this shape" is a comment, not a constraint, and F2 below is a live consequence | 62 / 71 differential; `TestParity` 42/42 (8 of them inverted, see below); typed-field matrix **34/34**, tag matrix 28/37 |

**Pass 17 (`2e8980c`)** found one finding, fixed: `ValidateCustomThemeFolder` gated on `os.Stat(...).IsDir()`, a stronger predicate than upstream's `pathlib.Path.exists()` (`design.py:74`). A theme name resolving to a **regular file** beside the input exists() upstream and falls through to the `rglob("*.j2.typ")` check, reporting "does not contain any *.j2.typ files" — the port reported "does not exist" instead, the wrong one of the two folder messages this iteration exists to reproduce. Fixed by dropping the `IsDir()` check; only `os.Stat`'s error means the path is missing now. Landed with a case. Pass 17 confirmed pass 16's fix correct on ~50 further probes (quoted-numeric strings that legitimately parse, quoted octal/binary, the alpha position specifically, script/document combinations not previously tried, a cyclic Lua table) and found nothing else standing that wasn't already recorded. **This is the first pass since the chain began where the hunt turned up only one small, already-fixed finding** — see the 17th re-verification log entry for the verifier's own assessment that iteration 14 may be close to closeable.

**Pass 18 fanned out three parallel verifiers across independent slices** — colour/numeric parsing, the Lua sandbox and script-merge machinery, and every other design-tree field kind plus theme-name/folder handling — rather than one more sequential sweep, since they are read-only checks that do not read each other's output (AGENTS.md §5's leaf-fan-out rule). All three found real bugs; all were fixed in this round.

*Colour slice* (`7176a0a`): `formatAlpha` rounded via `math.RoundToEven(c.alpha*100)/100`, which computes on a different, re-rounded number than the original float — `0.075`'s true binary value sits fractionally below it, so Python's `round(0.075, 2)` is `0.07`, but `0.075*100` lands on exactly `7.5` and rounds up to `0.08`; **10 of 10 `.xx5` probes diverged**. Fixed with `strconv.FormatFloat(v, 'f', 2, 64)`, which performs the correctly-rounded decimal conversion on the exact value directly. Also: an alpha that *rounds up* to 1 (`0.996`) is still a Python float and must print `1.0`, not `1` — only an alpha already close to 1 *before* rounding is dropped entirely. And `parseNumericText`'s non-coercing branch was a bare `strconv.ParseFloat`, which accepts Go's hex-float literal syntax (`0x1p-2`) where Python's `float()` raises — a quoted colour-tuple element spelling one validated and then leaked a raw Go slice into the artifact, because `ParseColorTuple`'s coercing path (which routes `0x`-prefixed text to `ParseInt`) disagreed and rejected it. **One finding deferred**: Python's `float()` accepts any Unicode decimal-digit string (`"١٢٣"`), `strconv.ParseFloat` is ASCII-only — inverted exit codes on a colour-tuple element in non-ASCII digits, plausible only in an i18n corpus, recorded rather than built into a Unicode-aware numeric parser for iteration 14's scope.

*Lua slice* (`16850c7`, `e80fcfb`): the sandbox bounded time (`Budget`) and table structure (`maxDepth`) but nothing bounded allocation — `string.rep("x", 3000000000)` is a single Lua instruction, so gopher-lua's between-instruction context check never fires, and Go's runtime kills the whole process with an unrecoverable `fatal error: out of memory` and a ~20KB stack trace, exit 2, from a file the user may have downloaded — precisely the failure class `maxDepth`'s own comment says the sandbox exists to prevent. Fixed with `state.SetMx(512)`, gopher-lua's own memory-limit mechanism (polls total process allocation, calls `os.Exit(3)` directly rather than returning an error `Run` could turn into a panel — still strictly better than the crash it replaces: a bounded exit with a 14-byte message instead of a stack trace). No automated regression test is possible (`SetMx` calls `os.Exit` itself, so triggering it in-process would kill the test binary); verified manually. Separately: a Lua sequence with a hole (`{"education", nil, "publications"}`, keys `{1,3}`) has no clean `1..Len()` run, so `isSequence` correctly refuses it — but it also has no string keys, so it fell through `Options`' string-keyed walk and vanished entirely, silently dropping **both** real elements rather than just the hole, the third occurrence of the silent-list-loss class already fixed in passes 4 and 9/10. Fixed by recovering a numeric-keyed-only table as a sparse sequence (walk every key, sort by index, skip the gap) instead of falling through to an empty map.

*Non-colour design slice* (`5b626f7`, `493a4e7`, `0457dbd`): four bugs, all in `ValidateTheme`/`ValidateCustomThemeFolder` — the theme-name and two-folder-message path that is iteration 14's own stated subject. (1) `customThemeNamePattern`'s `$` didn't allow one trailing newline the way Python's `$` does, so a theme name ending in `\n` (reachable through a block scalar) failed the wrong one of the two messages instead of falling through to the folder checks. (2) `typstDimensionPattern`'s `\d` was ASCII-only where Python's `\d` on a `str` matches any Unicode decimal digit (`"٢cm"`) — fixed with `\p{Nd}`. (3) `hasTemplate` used `filepath.WalkDir`, which `Lstat`s its root, so a theme folder that is itself a symlink to a directory read as "not a directory" and was never descended into, where upstream's `rglob` resolves symlinks by default — fixed with `filepath.EvalSymlinks` before the walk. (4) both folder messages built their path with `filepath.Abs`, which calls `Clean` and collapses `..`, where upstream's `Path.absolute()` only prepends the working directory and leaves the rest of the path exactly as written — a relative input path containing `..` printed a different folder than upstream in both messages; fixed with a non-cleaning `absoluteLikePython`. **One minor deferred**: a theme name longer than `NAME_MAX` makes upstream's `pathlib` raise `OSError` ENAMETOOLONG, caught and printed as a *different*, already-handled `Error` panel upstream's CLI error handler owns — the port instead reads `os.Stat`'s error unconditionally as "missing" and prints the validation-record panel. Distinct from the "port succeeds where upstream crashes" class (both sides *do* handle it, just differently) but out of iteration 14's own scope (CLI/OS-error-handling territory), recorded rather than fixed.

**Pass 19 checked whether the fan-out's nine fixes interact badly with each other and hunted further — FAIL, 2 blockers, both fixed; 6 more findings, 4 acted on.** Blockers: (1) pass 18's `..`-preservation fix (`0457dbd`) was incomplete — `filepath.Dir` (building `relativeTo`) and `filepath.Join` (building the folder path) both clean *before* `absoluteLikePython` ever sees the string, so a **leading** `..` survived (nothing precedes it to cancel against, which is why pass 18's own probe passed) but an **interior** one did not. Fixed (`d80c5bf`) by replacing both call sites with a `pathlib`-style segment parse (drops `.` and empty segments, never resolves `..`) shared between `relativeTo` and the folder join — `os.Stat`/`WalkDir` handle an uncleaned path exactly as well as a cleaned one, so there was no functional reason to clean before the real filesystem calls, only a reason not to when the same string reaches a message. (2) `formatAlpha`'s zero branch used `rounded == 0`, true for `-0.0` too, throwing away the sign; Python's `str(round(-0.0, 2))` is `'-0.0'`. Fixed (`d4fa400`) with `math.Signbit`. Also acted on: the sandboxed-globals gap was **not** actually closed as pass 18 claimed — `sandbox_test.go` still named only 8 of the 17 `blocked` entries, a transcript is not a test; all 17 now have a case (`a87814e`). A misplaced doc comment (`sequenceOf`'s landed on `isNumericKeyedOnly` when pass 18 added the latter) and spec 014 §5's stale "verified nine times" text were both fixed (`7986127`). **Not acted on**: a control character in an Input Value renders raw where Rich expands/strips it — reproduces on `design.page.size` too, so it's panel-renderer territory, not iteration-14-specific, though reachable through `design.theme`; a large sparse Lua table (20M entries) now legitimately reaches pass 18's new 512 MB memory bound and exits 3, which has no upstream counterpart and no `divergences.md` entry — expected consequence of the fix, not a new bug, and `divergences.md` changes are human-gated (AGENTS.md §5); `d80c5bf`, `0457dbd` and one earlier commit each bundle more than one logical unit, a repeat of the already-recorded commit-discipline minor. Full suite green after both fixes: 34/42 `TestParity` unchanged, `just schema-diff` empty. | 0 blockers reproduced by the 19th pass's fixes (unverified by a 20th), sandboxed-globals gap now genuinely closed, 3 minors recorded (panel-renderer control chars, expected memory-bound exit code undocumented pending human gate, repeated commit-bundling), all earlier deferred/out-of-scope items unchanged |

**Pass 20 found 2 blockers (both fixed), 1 major (fixed) and 1 minor (deferred).** Blocker: pass 19's `-0.0` fix (`d4fa400`) applied unconditionally, so a colour alpha spelled as a **plain YAML integer** `-0` — which resolves to a Python `int`, and `float(int)` is always `+0.0`, ints having no negative zero — now wrongly printed `-0.0` too; only a float- or string-sourced `-0.0` should keep its sign. Fixed (`5f05d6e`) with `colorElement.IsPythonInt`, set precisely from `yamldoc.Kind` in `validColorNode` and heuristically (plain-decimal-text shape) in the Kind-blind merge-time `ParseColorTuple`. Major: 2 of the 17 "closed" sandbox test cases from pass 19 (`a87814e`) were vacuous — `dofile("/etc/passwd")` errors because the file isn't valid Lua, not because `dofile` is blocked, and `load("return 1")` errors because gopher-lua's `load` is Lua 5.1's (takes a function, not a string), not because `load` is blocked; both pass with the global left open. Fixed (`22c2100`) with a harmless temp `.lua` file for `dofile` and a well-formed function-argument call for `load`, each confirmed to actually fail when the corresponding name is temporarily removed from `blocked`. **New finding, out of iteration 14's own subsystem, not fixed**: an explicitly-tagged YAML scalar (`!!int 200`, `!!float 0.5`, `!!str Bob`) resolves to nothing in the port's reader — `internal/schema/yamlreader/resolve.go` never sees an unwrapped tag node at all — where ruamel resolves the tag and gives the parsed value its declared type; a tagged colour-tuple element renders wrong or drops silently, and a tagged `cv.name` reaches the Input Value column as `None`. This is reader-level (iteration 2) territory, not design/Lua, but it lands as an artifact divergence in `design.colors` — the area passes 12–20 have been repairing — so it is recorded here rather than in a future iteration's own investigation, for whoever picks up the reader next. **Minor, deferred**: `pathlibParts`'s `absolute` field is a bool, so a POSIX double-leading-slash root (`//tmp/...`, which `pathlib` preserves per POSIX, three-or-more collapses) becomes a single slash — cosmetic, no plausible CV reaches it. Full suite green after both fixes; sandbox test count still 17, now all genuinely load-bearing. | 0 blockers reproduced by the 20th pass's fixes (unverified by a 21st), 1 new out-of-scope finding (YAML tag resolution, iteration 2's), 1 new minor deferred (POSIX `//` root), all earlier deferred/out-of-scope items unchanged |

**Pass 21 found 1 blocker (fixed), 1 major (open) and 1 minor (open).** Blocker: `ValidateScript` shape-checked a theme script's declared defaults but never value-checked them — upstream's `validate_default=True` (`base.py:5`) plus `design.py:135`'s `theme_data_model_class(**design)` means a script declaring `page.size = "bogus"`, `colors.name = true` or `typography.font_family.body = true` is a clean exit-1 panel upstream, where the port let each leak straight into the artifact: bad Typst text, the typst engine dying on a raw `unknown variable: True`, or (worst) a document silently rendering at exit 0 with a font literally named `"True"`. Broader than pass 16's already-recorded minor (script colour *string* vs *tuple* asymmetry) — measured across colour scalar, Typst dimension, literal, bool, string and font-family scalar/nested. Fixed (`123942b`): every scalar surviving the shape gauntlet now runs through `scriptValueMessage`, keyed by the tree's `Kind` and reusing the same validators document-side values already go through (`ParseColor`, `ValidTypstDimension`, `binder.LiteralMessage`, a new `scriptBoolMessage`). **The user-visible outcome is still a whole-script drop, not upstream's panel** — surfacing script diagnostics is spec 014 §2 behavior 9's already-human-gated wording question, left alone; what's closed is the artifact leak, not the exit code (0 with theme defaults, not upstream's 1). One judgement call flagged for the next pass: `KindString`/`KindThemeTag`/`KindTypstDimension` now reject a Lua *number* outright (no int→str coercion, matching pydantic's lax mode), so a script writing `top_margin = 2` now loses its whole script where it previously leaked one field — a behavior change worth re-probing. **Major, not fixed**: the port's `RenderCVUserError` panel is missing upstream's trailing `\n` — invisible to the whole suite because `conformance.Normalize` (and `tools/gengolden`) append `\n` to *both* sides unconditionally before comparing, so the last byte of every golden is unverifiable by construction. Measured on `err_empty_yaml`, which `TestParity` reports green: upstream 553 B, port 552 B, a strict prefix. Validation-error panels (`err_unknown_theme`, `err_not_yaml`) were checked and do *not* have this gap — both sides end without `\n` there — so this is scoped to the `RenderCVUserError` path specifically, CLI/iteration-12 territory. **Minor, not fixed**: STATE.md's own record of the design-crash-with-no-`theme` exit code says 2; measured actual is 1 (rich traceback via typer's excepthook), same for `design: [1]` and `design: {}`. ~200 differential renders otherwise confirmed matching, including all three spec-014 criteria run end-to-end against upstream for the first time. `go test ./...` and `just schema-diff` clean; `just check`'s two pre-existing `typstc` lint issues and `TestParity`'s 8 documented cases unchanged. | 1 blocker fixed (unverified by a 22nd), 1 major fixed (`fa12ea5`), 1 minor fixed (`53412bf`), 1 harness gap open and human-gated, 1 new content divergence recorded, all earlier deferred/out-of-scope items unchanged

**Pass 21's major and minor are both closed; the harness gap behind the major is not, and is human-gated.** The major's root cause is structural rather than cosmetic: upstream prints an error panel through *two* paths, and the port had only one. `@handle_user_errors` (`error_handler.py:11-50`) catches a `RenderCVUserError` that escapes the command and calls `rich.print(Panel(...))`, which **terminates with a newline**; `ProgressPanel.print_user_error`/`print_validation_errors` (`progress_panel.py:119-168`) call `self.update()` on a `rich.live.Live`, whose final render **does not**. Which path an error takes is decided by position: anything raised before `with ProgressPanel(...)` (`render_command.py:231`) — that is, inside `collect_input_file_paths` or `parse_override_arguments` — escapes to the decorator. The asymmetry that makes this measurable rather than guessable is `collect_input_file_paths`' `contextlib.suppress(RenderCVUserValidationError)` (`run_rendercv.py:113`): a YAML *syntax* error is suppressed there, falls through into the Live phase and comes out as a validation panel with no trailing newline, while an *empty* file raises `RenderCVUserError`, is not suppressed, and gets one. The port's `writeLivePanel` trimmed unconditionally. Fixed (`fa12ea5`) with a second writer, `writePrintedPanel`/`failPrintedPanel`, and by making `resolveNamedOverlays` mirror the suppress clause exactly — only a `*schemaerr.UserValidationError` is swallowed. **The fix also corrected a second, separate ordering defect it exposed**: `resolveNamedOverlays` ran *after* `ParseOverrideArguments`, inverting `render_command.py:205` before `:228`, so an empty file passed together with an odd override count reported the override error where upstream reports the empty file (637 bytes against upstream's 553). Four vectors are now byte-identical where they were one byte short or wholly wrong (empty file; `--cv.name=Jane`; `cv.name Jane`; empty file plus an odd override), and five that were already correct — malformed YAML, a validation table, an out-of-range override index, a bad overlay file, and the success panel — are unchanged. **The harness gap stands and is the reason this shipped green for so long**: `conformance.Normalize` (`internal/conformance/conformance.go:241-248`) and `tools/gengolden`'s `normalize` (`main.go:317-324`) both append a trailing newline to *both* sides before comparing, so **the final byte of every golden is unverifiable by construction** — `err_empty_yaml` reported green throughout while being a strict 552-byte prefix of upstream's 553. Un-blinding it means regenerating `testdata/golden`, which is human-gated (AGENTS.md §5), so neither `Normalize` nor any golden was touched; `internal/cli/panelnewline_test.go` pins the trailing bytes of both paths directly instead, and is currently the only thing holding them. **New finding, recorded not fixed**: a malformed YAML (`cv: [`) renders a validation table of 1597 bytes against upstream's 1411, and `--settings` pointed at a bad YAML gives 1504 against 1597 — a *content* divergence inside the table, unrelated to the newline, covered by no golden case, and its own unit.

**Pass 22 fanned out into three parallel read-only slices** — iteration 14's core (Lua/design/theme), the CLI's error and output surface, and the YAML reader plus syntax-error locations — since none reads another's output. **All three returned FAIL: 25 findings, 12 of them blockers.** Every slice confirmed its assigned recent fix holds: `123942b`'s script-value checking survived a 143-case sweep (11 field paths × 13 Lua values, all 10 `Kind`s) with no remaining leak, and its int→str rejection was confirmed correct against upstream's own `validate_default=True` probe; `fa12ea5`'s nine vectors are byte-identical *including the last byte*; and `c82db58`/`4e77e25`'s 13 shapes reproduce, plus 22 more the slice added (escaped quotes, CRLF, BOM, `%YAML`, anchors, unicode, flow-in-block-seq).

**Six blockers fixed in this pass.** (1) `a9739b0`: **a multi-document stream was silently accepted** — `build.go:24` read `Docs[0]` and ignored the rest, so `---\ncv:\n  name: A\n---\nb: 2` rendered A's CV at exit 0 where upstream's composer raises. ruamel's marks are not a failing token's: the context mark is where the *first* document's content began and the problem mark is the *second* document's `---`, which goccy exposes directly; four shapes byte-identical. (2) `1b4360f`: **`new` overwrote an existing input file and reported the opposite** — upstream treats the YAML as one item of the same exists-or-create loop the template folders use (`new_command.py:110-119`), skipping it and printing "Your YAML input file already exists"; the port did an unconditional `os.WriteFile` and hardcoded the "✓ Created" row, so a CV the user had already filled in was destroyed silently. Measured: a pre-existing file stays 0 bytes upstream, was clobbered to 12,097 by the port. **This is the only data-loss defect found in the port so far.** (3) `504c91a`: `fa12ea5` was incomplete — `new` builds **no** `ProgressPanel` at all and is `@handle_user_errors`-decorated, so every one of its errors takes the `rich.print` path unconditionally, but its call site was left on the Live writer; 637/638 and 893/894, now identical. (4) `1b0e464`: **an explicit null document took the wrong path** — upstream's predicate is `yaml.load(...) is None` (`yaml_reader.py:55-57`), so `null`, `~` and `Null` report the 553-byte empty-file `Error` panel; the port keyed on the *absence* of a document and sent them to a 1504-byte validation table. Six spellings now identical. (5) `53084d8`: **`validBoolNode` rejected six numeric spellings upstream accepts and coerces** — pydantic's lax bool takes any int spelling resolving to 0/1 and a float exactly equal to them, so `1.0`, `-0.0`, `1e0`, `0x1`, `0o1`, `0b1`, `+1` all render upstream and exited 1 here, on **every** built-in theme and all 18 `KindBool` fields. Routed through pass 16's kind-aware `parseNumericText`, so a quoted `"0x1"` is still a Python `str` and still fails; `normalizeBools` gained the matching value coercion, so `links.underline: 0o0` reaches the emitter as `false`. (6) `a7101ec`: the wrong pydantic error for a non-boolean number — upstream is `bool_parsing` ("…unable to interpret input."), the port said `bool_type`. Measured rule: a whole number that is not 0/1 is `bool_parsing`, a non-integral float (`1.5`, `1e-7`, `.nan`, `.inf`) stays `bool_type`. Fixed on both the document and the script side.

**Six blockers and the majors remain open**, recorded here rather than rushed: a **tagged YAML node is dropped entirely** (no `*ast.TagNode` case in `buildNode`, so everything tagged becomes `KindNull`) — pass 20 recorded this, but pass 22 measured its true blast radius and it runs **both** ways: a tagged scalar loses its value and prints `None` in the Input Value column, *and* a tagged **collection** breaks documents upstream renders fine (`cv: !!map` errors here, exits 0 upstream). The correct semantics were measured and are asymmetric — ruamel's round-trip loader returns a `TaggedScalar` for `!!str` and for unknown tags (so upstream *rejects* `cv.name: !!str Bob` with "Input should be a valid string" while showing `Bob`), a real `int` for `!!int`, a `ScalarFloat` for `!!float`, and ordinary `CommentedMap`/`CommentedSeq` for `!!map`/`!!seq`. Representing a `TaggedScalar` needs either a new `Kind` (which every `Kind` switch in the schema layer would silently default on) or a `Tagged` flag consulted by the binder; naive unwrapping would *invert* the bug rather than fix it, so it is left for a deliberate decision. Also open: `--quiet` does not silence error output (upstream builds the whole Live on `Console(quiet=quiet)`, so `-q` emits **0 bytes** for a validation failure where the port prints the full 1599-byte table); `new` accepts only the literal name `"John Doe"`; two tab-handling divergences in opposite directions (goccy rejects a tab-indented continuation inside a multi-line double-quoted scalar that ruamel accepts, and accepts a tab in a block sequence that ruamel rejects); a third goccy phrasing for an unterminated flow sequence (`cv: [a\nb: c,`) that no map covers, leaking raw goccy text *and* the wrong location; the duplicate-key location is a span upstream and a single line here, **with a code comment asserting the opposite**; one `ruamelPhrasing` row is dead for the input it names; `TestUnmappedParserMessageFallsThrough` is skipped *because of* the tag defect, so its assertion never runs; and `1e400` resolves to `KindString` here where ruamel gives float `inf`.

**Four items are human-gated and were left alone** (AGENTS.md §5): `divergences.md` entries for the unmapped-parser-message `[line:col]` prefix leaking into user-visible text — which plan 004 §6 made an explicit stop point that was never taken — for `new`'s single-name limitation, for `create-theme` having no `@handle_user_errors` upstream (its errors are a stderr traceback, where the port prints a clean stdout panel), and for a Lua script being unable to declare a `null` default, which makes `templates.education_entry.degree_column` unreachable from a scripted theme. Plus the golden regeneration below.

**All three slices reported the parity suite as non-deterministic — and the cause was the fan-out itself, not the suite.** Two runs at the same commit gave 8 and 22 failures, and slice B saw all 14 render cases fail together on `mkdir rendercv_output: no such file or directory`, which no input can produce upstream. Diagnosed after the slices finished: `caseWorkDir` (`internal/conformance/conformance.go:211-222`) runs every case in a **fixed, shared** path — `testdata/.work/run/<case>` — and `os.RemoveAll`s it first. Case names are unique and only one test binary uses those directories, so the suite does not race *itself*; what raced was three agents sharing one working tree, with slice B running its own differential harness under `testdata/.work/` while the others ran the suite. Re-measured alone afterwards: **five consecutive runs, 8 failures every time** (three of `./internal/conformance/...`, two of the full `./...`), so the suite is deterministic and the reported flakiness was self-inflicted. Every number in this entry was taken that way regardless.

**The underlying fragility is real but cannot be fixed without the human gate.** The shared path is deliberate — `caseWorkDir`'s own comment records why a `t.TempDir` is impossible — and `err_unknown_theme`'s golden bakes the absolute run directory into the bytes being compared, so isolating runs per process would change the path and break that golden. Hardening it therefore means regenerating `testdata/golden`, which is human-gated (AGENTS.md §5). **Until then the operational rule is that the parity suite must not be run concurrently with anything else touching the repository** — including a parallel verifier fan-out, which is how this pass produced three false reports. | 6 blockers fixed (unverified by a 23rd), 6 blockers + 9 majors/minors open, 4 items awaiting the human gate, suite flakiness diagnosed as self-inflicted and the real fragility gated


### Full-repo re-audit, 2026-08-11

A fresh-context `rendercv-parity-verifier` was pointed at the whole non-green set (iterations 1, 3,
6, 7, 8, 11, 12, 14, 15, and pass 22's own open list) rather than one iteration, after three
spot-checked "still open" items from pass 22 turned out to already be fixed in code with no ledger
update. Verdict: **of roughly 30 findings the ledger listed as open, all but 6 no longer
reproduce** — `just check`, `just test`, `just test-parity` and `just schema-diff` all exit 0 on
HEAD. Iterations 6 and 7's real findings (above) were fixed same-day; what's below is what's left.

**Update, same day — a parallel fan-out (6 agents, 3 porters + 3 investigators, none reading
another's output) closed all the small/leaf items from the list above.** Landed:

- **Iteration 8**: all five divergences fixed, plus a sixth found and left open (see iteration 8's
  own row above for the full account).
- **Iteration 14**: both minors fixed. Control character in an Input Value — `844477d`, "strip the
  control codes Rich strips": the transform is Rich's `Text.__init__`→`strip_control_codes`
  (`rich/control.py:9-15,181-192`), which deletes exactly bytes 7/8/11/12/13 and *not* the rest of
  C0 (`\x01`, `\x1b`, `\x7f` pass through raw upstream too — confirmed by probing all of them, a
  blanket-strip fix would have been wrong). Arabic-Indic/Devanagari/fullwidth colour digits —
  `282e672`, "parse a colour tuple's non-ASCII digits": CPython's `PyFloat_FromString` runs a
  decimal-transliteration pre-pass before parsing; the port's `parseNumericText` now does the same,
  using `unicode.Nd`'s stride-1 block structure to get each rune's digit value (no stdlib helper
  exists for this). Both confirmed byte-identical against upstream on their probes.
- **Ledger integrity**: `e25215a` (iteration 6's interleave fix) now has a test (`838d01d`, confirmed
  to fail against the pre-fix code). Iteration 13's deleted "2 majors and 4 minors" were recovered
  from git history (`git show 308b6f4^:specs/STATE.md`) and re-verified against current HEAD — all
  six still reproduce, none were actually fixed, only lost from the ledger. Recorded in iteration
  13's row below rather than duplicated here.
- **New, unprompted**: a full-repo audit for "port crashes where upstream doesn't" turned up
  `internal/cli` had no panic-ban check (`cmd/rendercv-go` has one, `internal/cli` didn't). Adding
  one (`97e17bf`, `c7b6e3f`) found a real live panic — a malformed embedded `help.json` would exit 2
  with a goroutine dump, neither of upstream's two failure shapes. Fixed by returning a zero value
  instead, pinned by a build-time test.

**Still genuinely open, ranked by reach from an ordinary CV:**

1. **Iteration 6** — the missing-`theme`-key crash mismatch. An upstream-analyst traced the full
   shape: unhandled `KeyError` inside an already-open `Live` progress panel (522 B stdout, 9611 B
   stderr traceback, exit 1 — a fifth vector distinct from spec 013's four "crash before any panel
   opens" cases, since this one has nonzero stdout). `specs/013-parity-closeout/spec.md:994-998`
   already drafted the needed generalization (P-1, promoting D-011 from two named goldens to the
   whole unhandled-exception class) but it is **unwritten and human-gated** (`specs/divergences.md`
   changes go through AGENTS.md §5's gate) — no code should land ahead of that decision.
2. **Iteration 11** — emphasis nesting and the spaced link destination. A second upstream-analyst
   pass confirmed both need a hand-written replacement `parser.InlineParser` (goldmark's own
   `linkLabelState`/`processLinkLabel` are unexported, so the link fix can't even reuse goldmark's
   bracket-matching) — sized at "a new file each, comparable to `emphasis.go`'s existing Typst-side
   reimplementation," with real regression risk to the 113 already-passing HTML differential rows.
   Not single-commit; needs its own spec before a porter touches it.
3. **Iteration 1** — goldens still bake this machine's absolute path
   (`testdata/golden/err_unknown_theme/stdout.txt` and two others). Needs a golden regeneration,
   human-gated.
4. **Iteration 14** — two very low-reach residuals noted by the control-char fix: tab expansion
   inside a table cell (upstream expands to the next multiple of 8, the port emits the raw tab —
   `rich/text.py:817-857`), and Rich's own `cell_len` scoring an unstripped zero-width control char
   (`\x01`) as width 0, which makes upstream's own panel one column *over* its stated width for that
   input — matching it means reproducing `cell_len`'s scoring, not obviously worth it.
5. **Iteration 8** — a code block's body isn't `rstrip`'d and doesn't expand tabs the way upstream's
   `Markdown.convert` does before `code_escape` sees it (`"    a &  "` keeps its trailing spaces;
   `"    a\tb"` keeps its raw tab instead of expanding to `tab_length`). Found and deliberately left
   open while fixing the escaping gap next to it (`bbea40b`); every new fixture row was kept free of
   trailing whitespace/tabs so none of them mask it.
6. **Iteration 15** — `fitsNoScalarArm` (`binder.go:520-544`) is still the only Kind predicate
   enforced exhaustively by the linter; the rule that any later Kind predicate belongs in the same
   shape is a comment, not a lint constraint. No new failure found from it, but nothing prevents one.

**Also resolved this session, both from a second wave of 4 parallel porters closing out leads the
first wave found:** the `ENTITY_RE` lead turned out **already fixed** by `1adbd49` itself — verified
with an 18-case probe and a byte-identical end-to-end render, no code change needed. Two real fixes
landed: colour **strings** (not just tuple elements) with non-ASCII digits — `rgb(١٢٣, 2, 3)`,
`hsl(...)`, `rgba(...)` — now parse via `\p{Nd}` regexes and the same transliteration
`282e672` added, confirmed byte-identical on 8 forms (`8a2d560`); and an inline code span's body is
now HTML-escaped for `&`/`<`/`>` the way upstream's `code_escape` does, found while verifying the
`ENTITY_RE` lead and fixed same day (`e54c2a3`, confirmed red-before-green and byte-identical
end-to-end).

**A third wave (2 more verify-first porters) closed both leads the second wave left open.** Indented
code blocks had the same `&`/`<`/`>` escaping gap as inline spans, confirmed reachable (a `highlight`
line starting with 4 spaces) and fixed by reusing the same helper (`bbea40b`) — the admonition path
was checked and correctly does *not* escape, pinned by a named test so a future change can't
regress it silently; left the rstrip/tab-expansion gap next to it open (#5 above) rather than scope
creep. And the Unicode-whitespace lead was confirmed reachable end-to-end (a double-quoted YAML
scalar carries `\v`/`\xa0` through both ruamel and goccy) and real: Go's `\s` is 5 characters,
Python's Unicode-aware `\s` on a `str` pattern is 29, and `\p{White_Space}` isn't usable in Go's RE2
either (unsupported syntax, and the wrong 25-rune set besides) — fixed with an explicit character
class transcribed from CPython's `str.isspace()`, swept against upstream at all 29 runes × 10
positions with zero mismatches (`9aa5ce4`).

**Checked and correctly NOT on this list**: iteration 3's "constructed-entry half of the
discrimination criterion is untested" (spec F5) is not a coverage gap — the test's own comment
explains it is a deliberate cut, because Go's architecture has no equivalent to Python's
already-constructed-model branch (every YAML mapping takes the identical code path in this port);
approximating it would be a test that cannot fail.

## Stretch goals (not gates)

- [ ] PNG pixel-level comparison (depends on the WASI typst font set — see D-006)
- [ ] Public `pkg/rendercv` API frozen and semver'd
- [ ] Cross-compiled release artifacts (linux/darwin/windows × amd64/arm64)

## Cut scope

Anything dropped from an iteration is recorded here with the reason, per `AGENTS.md` §10.2.

### Iteration 2

> **All six are CLOSED as of an audit on 2026-08-11, and this section is now a record, not a
> backlog.** A reader had no way to know that without redoing the audit, which is the point of
> writing it here. Item 1 — coordinate columns — closed by `c986359`, which took both probe
> documents to 0/232 and 0/388. Item 2 — `models.Validate` not calling `cv.Validate` — closed by
> `5e130d1`; the package cycle it was deferred for is resolved and `rendercvmodel.go:113` calls it
> directly. Item 3 — phone formatting — closed by `7f03997` plus `bridge/phone.go:18-38`, landing
> under iteration 4-era work, which moots the spec's self-contradiction about whose item it was.
> Item 4 — T7's no-op regression test — closed by `noalias_corpus_test.go:30`, which walks the whole
> submodule with a `walked < 60` guard against a silent no-op and asserts token-stream identity, the
> stronger of the two statements the item asked for. Item 5 — T10's hand-written scalar corpus —
> closed; `resolve_test.go:19-29` now reads a `tools/scalarprobe`-generated fixture measured through
> upstream's own `read_yaml`. Item 6 — §4.12 tested through an injected stub — closed by two tests
> in `sectionvalidation_test.go` running against the real entry-type registry.

Verified by `rendercv-parity-verifier` in a fresh context. Everything below was carried into
iteration 3's spec as an open item; nothing here was a silent divergence.

1. **Coordinate columns diverge from ruamel in two shapes the T8 fixture cannot see.** A key
   with a null or empty value reports column 1 rather than the key's own indent
   (`internal/schema/yamlreader/build.go`), and a flow-sequence element reports the first value
   token rather than the `[`. Measured against upstream: 33/232 paths differ on
   `examples/John_Doe_ClassicTheme_CV.yaml`, 50/388 on
   `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`. **Lines match
   everywhere**, and spec §6.7 says only line numbers reach users, so nothing user-visible is
   affected yet — but `expected_errors.yaml` is iteration 4's import, so this must be fixed
   there, together with extending the fixture to cover both shapes.
2. **`models.Validate` does not call `cv.Validate`.** `models` owns `ValidationContext` and the
   path types, which `cv` imports, so the edge would cycle. Closing it needs those two types
   moved to a leaf package — a `plan.md` layout change, deferred to iteration 3 rather than
   made unreviewed at the tail of this one. Everything *inside* `cv` is wired (commit
   `fd33d82`).
3. **`phone` formatting (spec §3.49) is not implemented.** `+905419999999` does not serialize to
   `+90-541-999-99-99`; only the `tel:` strip is done. Spec §8 lists this as an acceptance
   criterion while spec §7 assigns phone formatting to iteration 4 — **the spec contradicts
   itself and iteration 4 must resolve which is right.**
4. **T7's no-op regression test over the submodule YAML corpus was never written.** The verifier
   ran it by hand — 64/64 files identical with and without `dealias` — so the transform is
   sound, but nothing in the suite guards it. `noalias_test.go` also asserts tokens where
   `tasks.md` required the parsed tree; the tree-level assertions now live in
   `yamlreader_test.go` (commit `f91da06`).
5. **T10's scalar corpus is hand-written, not generated.** `tools/yamlprobe` still emits only the
   five coordinate documents, so `resolve_test.go` states Go-side expectations rather than
   deriving them from upstream. Behavior was differentially verified as correct.
6. **§4.12's "mixed section" and "entry problems" criteria are tested through an injected
   validator**, because the concrete entry types are iteration 3's. They must be re-checked
   against real types when those land.

Two process failures in this iteration's history, recorded rather than rewritten:

- `1befa1e` bundles T9, T10 and T11 in one commit, against `AGENTS.md` §7.
- T8's coordinate test landed in `65aaa49`, *after* the T9 implementation it was supposed to
  precede, inverting the red-before-green rule of `AGENTS.md` §4.

### Iteration 3

Verified by `rendercv-parity-verifier` in a fresh context, which returned **FAIL** with three
blockers. Two were fixed inside the iteration (commit `9ddd896`); the rest is cut here. Nothing
below is a silent divergence, and none of it belongs in `divergences.md` — every item is an
ordinary bug reproducible in Go, not a place where parity is impossible.

**Fixed rather than cut** (recorded because the failure mode matters): three tests I wrote
asserted the *port's* error codes instead of upstream's, which is why the suite was green while
the codes were wrong. One test stated the correct upstream code in a comment and asserted a
different one on the next line. The diamond caught what self-report would not have.

1. **Entry error ordering does not interleave base and own fields.** The composed binder spec is
   `[base fields] ++ [own fields]`, the reverse of what `class X(BaseWithDates, BaseX)` produces,
   and the `date`/`start_date` validation errors are appended after all bind errors rather than
   emitted at their declared position. Measured against upstream:

   | input | upstream | Go |
   |---|---|---|
   | `ExperienceEntry{company, summary: {a: 1}}` | `position` missing, `summary` string_type | `summary` string_type, `position` missing |
   | `ExperienceEntry{position, highlights: "x"}` | `company` missing, `highlights` list_type | `highlights` list_type, `company` missing |
   | `ExperienceEntry{company, date: 2020-13-01, location: {a: 1}}` | `position`, `date`, `location` | `location`, `position`, `date` |
   | `PublicationEntry{title, authors, doi: bad, journal: {a: 1}}` | `doi` string_pattern_mismatch, `journal` string_type | `journal` string_type, `doi` string_pattern_mismatch |

   The descriptors already carry the right order, so part of the fix is feeding the binder the
   descriptor order. But the date errors must also be *interleaved* at their declared position
   rather than appended, which touches all three base binders plus `publication.go`. Spec 003 §6.3
   already carries a `TODO(iteration-4)` on entry error ordering, and iteration 4 owns error
   ordering, deduplication and message rewriting as its whole subject — so this lands there rather
   than as a rushed refactor at the tail of iteration 3. **Iteration 4 must not build its rewrite
   pipeline before fixing this**, because dedup by `schema_location` is order-sensitive.

2. **A non-scalar `date` or `start_date` produces no error at all.** Upstream produces two union-branch
   errors: `{date: {a: 1}}` on an `ExperienceEntry` gives `('date','int') int_type` and
   `('date','str') string_type`; `start_date` likewise. Go is silent. Reachable through real types
   for the first time in this iteration.

   Iteration 4 detail that makes this cheaper there than here: `unwanted_locations`
   (`pydantic_error_handling.py:24-32`) filters `"int"` and `"str"` out of the location, collapsing
   both rows to `('date',)`, and dedup by `schema_location` (`:167-176`) then merges them into
   **one** user-visible error. So the correct fix is one error at the field, and it needs iteration
   4's filtering and dedup to be expressed faithfully.

3. **Two `specs/003-entry-types/spec.md` §8 claims were inaccurate and are corrected in the record,
   not the code.** (a) §3.18 behavior 42 says iteration 2 "already emits both" custom error types —
   it emitted only `rendercv_other_error`; fixed in `9ddd896`. (b) §8's "every section test iteration 2
   wrote passes against the real registry with no edit to the test" is false: three tests changed
   (`sectionlist_test.go:20-24`, `:81-88`; `sectionvalidation_test.go`'s
   `TestFirstResolvableEntryWins`). The changes are correct — each fixture was one the
   accept-everything stub let pass and that upstream genuinely rejects, and each new expectation was
   measured against the vendored Python first — but the criterion as written cannot be checked, and
   the stale comment claiming otherwise has been corrected.

4. **`Registry.Discriminate` rebuilds the characteristic table on every call**
   (`entries/registry.go:70`). Not a parity issue; performance only. Left as is.

5. **The constructed-entry half of §8's discrimination criterion is not tested.** Upstream asserts
   the seven `(entry_type_name, section_model_name)` pairs twice — once from a raw dict and once
   from a constructed model instance (`tests/schema/models/cv/test_section.py:19-60`). Only the
   raw-mapping half is covered. The first attempt at the second half was a tautology: it validated
   a node and then re-resolved the same node, and both calls take the identical mapping branch, so
   it could not fail unless `Discriminate` were nondeterministic. It was removed rather than left
   as false coverage.

   Upstream's already-a-model branch (`section.py:173-176`) returns `entry.__class__.__name__` and
   has no Go equivalent — a non-mapping, non-string, non-null node falls through to the
   `messageNoType` branch under the `TODO(iteration-4)` in `sectionvalidation.go`. Reproducing it
   depends on iteration 4's §5.14 already-a-model decision, so it lands there.

6. **`messageModelType` is missing the entry type name.** Ours is `Input should be a valid
   dictionary`; upstream's is `Input should be a valid dictionary or instance of EducationEntry` —
   the concrete type is interpolated, and no `error_dictionary.yaml` row rewrites it, so it reaches
   the user raw. Measured on `[{institution, area}, null]`. Reachable from an ordinary document (a
   stray blank list item). Iteration 4 owns it, and it needs the entry type threaded into the
   binder's message rather than a table lookup.

Two process failures in this iteration, recorded rather than rewritten:

- `9ddd896` bundles the wrapper fix, the date-code split, the test corrections and two new
  criterion tests in one commit, against `AGENTS.md` §7. It should have been four.
- `8d131da` is labelled `test:` but ships 17 lines of production `default.go`. The production line
  was the deliberately-empty `Default()` that made the conformance test red, so the red-before-green
  intent held, but the type prefix is wrong.

Verifier items closed inside the iteration: the behavior-43 code table (it had substituted the date
code and `model_type` for `string_pattern_mismatch` and `url_too_long`, so two of the five were
never exercised), discrimination from an already-constructed entry, and the summary-only entry
matching no type end to end.

Not verifiable yet, and not claimed: PDF/PNG, artifact and CLI parity (iterations 10 and 12).

### Iteration 4

**No cut scope.** Every task in `tasks.md` landed, every `TODO(iteration-4)` in the tree is
cleared, and both items cut from iteration 3 plus four of the five carried from iteration 2 are
closed.

The gate is `TestWrongInputDifferential`: upstream's own `wrong_input.yaml` through the whole
port, compared against `expected_errors.yaml` **member by member, in order, with an equal-length
assertion** — all 25 records, all five members including coordinates. It is a port of
`tests/schema/test_pydantic_error_handling.py:19-54` and is the strongest mechanical Axis-4 gate
available.

The parity suite stays at its 42 red cases, which is iteration 1's baseline and not this
iteration's failure: no corpus case can pass until the renderer exists (spec §7.3). No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Bugs found while measuring, none in the task that found them.** Recorded because each was
invisible to the suite that existed at the time:

| Bug | Found by |
|---|---|
| Quoted mapping keys kept their quotes, so `"name": John` was rejected as an unknown key | the dictionary's one quoted scalar |
| `0b101` resolved to a string; ruamel reads it as 5 | the generated scalar corpus |
| Unknown keys were reported before every declared field | measuring the order upstream reports |
| An exact date's range and isoformat failures carried the wrong code (4 of 5 rows) | measuring §4.13's texts |
| `Invalid isoformat string` was missing entirely | the same measurement |
| `PublicationEntry.url` reported after every other field | registering its validator |
| A valueless key's span ended at column 1 whatever the indent | the coordinate differential |
| A flow-collection element started at its first value, not its bracket | the same |

**Two corrections to the spec, both recorded in place rather than silently edited:**

1. §3.2's table gave the reason for step 1 preceding the dictionary as "or `value is not a valid
   phone number` never matches". It does not hold — that message carries no prefix, and
   substitution matches by containment and replaces the whole message, so a prefix can only add a
   match. The ordering is still reproduced;
   `errorpipeline.TestPrefixStripDoesNotChangeWhichRowMatches` now asserts the unobservability, so
   a future row that makes the order matter fails rather than passing silently.
2. §3.13 behavior 46 said the URL length limit is 2083 **characters**. It is UTF-8 **bytes** — the
   check lives in pydantic-core, which is Rust. ASCII hides the difference, so nothing measured
   before was wrong, but a port counting runes would accept URLs upstream rejects.

**Carried forward, with owners:**

- **`emailaddr` is known-incomplete by decision** (spec §7.4). It reproduces 45 measured
  rejections; the library's catalogue is larger. The gap runs toward *accepting* what upstream
  rejects rather than misreporting it — the safer direction, since a plausible-but-wrong message
  passes review while a missing record shows up in the differential. `ErrUnclassified` exists and
  is unreachable, kept for a rule that can be detected but not named.
- **The YAML parser message is option B, scoped to the corpus** (spec §7.5, plan §6). Five
  goccy→ruamel phrasings are mapped and measured; an unmapped failure falls through to goccy's own
  line. **The coordinate half is not covered**: ruamel reports a context mark and a problem mark,
  goccy reports one token, so the corpus case's `line 1 to line 2` comes out as `line 1`. That is
  a decision needing `specs/divergences.md` and the human gate, and this iteration does not
  authorize writing one — see "Open for the human gate" below.
- **Two upstream crashes reassigned to iteration 12** (spec §7.8): a non-mapping entry and a
  list-valued `phone`. Neither is a validation-error-parity question — upstream produces no
  message, only an unhandled exception — so both wait on the CLI's unhandled-failure handling.
  Both carry `TODO(iteration-12)` markers.

**Open for the human gate:** the YAML-syntax coordinate span above. Nothing else.

### Iteration 5

**No cut scope.** Every task landed. The one deferral is written into the spec rather than cut
here: the `$defs` collision suffix (§7.2) is iteration 6's, because it is exercised only by the
per-theme variants and its numbering follows pydantic's emission order — a plausible
implementation would be a wrong answer that reads right, so `DefNameWithSuffix` panics naming the
iteration that owns it.

**Axis 3 is blocked, not failing, and the distinction is the iteration's main finding.**
`schema.json` is a projection of the model tree, so it is complete exactly when the tree is.
Measured over the 227 `$defs`:

| Owner | `$defs` | Share |
|---|---:|---:|
| Iteration 6 — design and themes | 161 | 74.8% |
| Iteration 7 — locale | 45 | 17.0% |
| Iteration 7 — settings | 3 | 3.3% |
| **Iterations 2–4 — what exists** | **18** | **4.8%** |

All 18 are byte-identical to upstream's. `just schema-diff` prints 8,621 lines of difference and
**zero of them are the port's** — every line it emits appears in upstream's file, which is asserted
by `TestPortSchemaInventsNothing`. A green `schema-diff` before iteration 7 would mean the port
invented something.

The alternative — reordering design and locale ahead of the generator — is recorded in spec §7.1
and rejected: the generator is what makes those two iterations mechanically checkable, and
reordering would put the two largest model iterations back to back with no gate between them.

**Four things the differential caught that transcription would not**, each invisible without
comparing real bytes:

| Finding |
|---|
| A required single-arm field **inlines** its arm: `authors` is `{items, type}`, not `{anyOf: [{items, type}]}` |
| `Cv`, `SocialNetwork` and `CustomConnection` have **no `description` key**; every entry type has an explicit `null` — the entry base's `json_schema_extra`, not a docstring difference |
| `CustomConnection.url` is required *and* nullable: in `required`, carrying the null arm, and with **no default** |
| `Cv`'s three scalar-or-list fields carry **no type key at all**, because pydantic cannot express that union |

One rule was derived rather than listed, and it will matter later: pydantic omits a field's title
when the schema is exactly one `$ref` plus `null`. That is why `date` and `start_date` have none
and `end_date` does — the shape, not the field. Iterations 6 and 7 add many more
optional-reference fields, and a list of the four known cases would have missed them silently.

**Two corrections to the parity contract**, both made in place in `specs/000-parity-contract`:

1. Axis 3 named a `rendercv-go schema` subcommand, which Axis 2.1's "no commands added" forbids —
   upstream's CLI has three commands and none is `schema`. The generation path is now named
   explicitly (`go run ./tools/genschema`), so the two axes cannot be read as contradicting.
2. Axis 3 required a trailing newline. Upstream's file has none; its last three bytes are `"\n}`.

**Carried forward:** the absent-set test states the remaining count as a number that must be
changed deliberately, so iterations 6 and 7 cannot close Axis 3 by accident or leave it closed on
paper. Iteration 7 moved it from 209 to 164; iteration 6 takes it to 0.

### Iteration 6

**Axis 3 is closed.** All 227 `$defs` are byte-identical and `just schema-diff` exits 0. The
oracle is `tools/genschema`; the parity suite's own Axis-3 case shells `rendercv-go schema` and
stays red until iteration 12, so "closed" means by the generation path spec 005 §4 named, not by
`just test-parity`.

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with twelve
findings, two of them blockers.** Everything below either was fixed inside the iteration or is cut
here with its reason. None of it belongs in `divergences.md`.

**The blocker worth reading, because the diamond is the only thing that could have caught it.**
The error pipeline reproduced upstream's step 2 — pydantic-core inserts a discriminated union's
resolved branch value as the location's second element, and `parse_plain_pydantic_error` deletes
it. The port never produces that element: `design.Validate` and `locale.Validate` resolve the union
themselves. Deleting anyway removed a **real** key. `design.colors.body` became `design.body`,
which failed to resolve against the document and reached the user as an internal error;
`design.nope` became `design`, which resolved and shipped a wrong location **silently**.

It was unreachable until this iteration emitted the first non-`theme` location under `design`, and
it survived my own tests because they stop at `models.Validate`. The fix deletes step 2 and adds
`TestDesignAndLocaleErrorsSurviveTheWholePipeline`, which runs five documents through `Parse` and
asserts what the user sees — including the colour message reaching dictionary row 13, the first
live producer for a row that has been in the table since iteration 4.

**Four shape checks were missing and are now measured** (`0dbd1e4`): a bool reported nothing at
all, `font_family` accepted a sequence, a non-string colour reported `string_type` instead of
`color_error`, and a non-mapping nested model reported nothing. Each of the four now carries the
code and message upstream gives, including the pair that distinguishes `bool_parsing` from
`bool_type` and the `model_type` text that **names the model**.

**Cut scope, with owners:**

1. **T10, the effective per-theme option tree, was cut to iteration 9** and is **now closed**
   there (`design.Effective`). Nothing validates a default, so the validation walk is the same for
   all nine themes — `TestDesignBlock` shows the same failure under `classic` and `sb2nov`. The
   cost was stated rather than hidden at the time: `WidenFontFamily` and `SnakeCaseSectionTitles`
   had no non-test callers, and spec §5 criterion 4's "must produce the same **model**" was not
   testable. Both are now called by `Effective`, which runs the two coercions where upstream's
   field validators do, and the criterion is met by a seven-document differential against
   upstream's own resolved model.
2. **Wave E — the D-002 Lua custom-theme path and spec §3 behavior 7's two folder messages — is
   iteration 14's**, a new row in the table above. `plan.md` §7 gives the reason: a sandbox bundled
   with 161 `$defs` makes both unreviewable. `design.go` carries the `TODO(iteration-14)` that
   `tasks.md` promised and did not have until the verifier asked for it.
3. **`settings` is still the thin slice of spec 004 §7.9.** The three `$defs` shipped here to close
   Axis 3; unknown-key rejection under `settings` and everything `RenderCommand` describes are
   iteration 12's. `specs/006-design-and-themes/settings.md` says so.

**A process failure the verifier found and I am recording rather than rewriting:**
`settings/schema.go` shipped before any spec covered it, against `AGENTS.md` §4. The retrofit is
`settings.md`, and it marks which of its criteria are open rather than implying the block is
finished.

**Two commit-discipline failures**, also recorded: `ff0c903` bundles T9, T11 and T12, and `58fc1f0`
bundles three `$defs` models with the Axis-3 status claim. Each should have been three commits.

**A design finding worth carrying forward:** a `design` block with **no `theme` key crashes
upstream** — `validate_design` runs in front of the union and does `str(design["theme"])` unguarded
(`design.py:57`), so the shape that gives `locale` a `union_tag_not_found` gives `design` a
`KeyError`. The port stays silent rather than reporting where upstream crashes, joining the two
crashes spec 004 §7.8 sent to iteration 12.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 8

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with three
blockers.** All three are fixed (`718c902`); the rest is cut here or recorded.

**The blocker worth reading is the one my own reasoning hid.** Spec §8 argued that the two
whitespace acceptance criteria could not be checked in this iteration, because only a corpus `.typ`
can exercise the transform and a corpus `.typ` needs iteration 9's model bridge. The second half is
true. **The first is not** — a *fragment* needs only a dictionary — and the verifier said so.

The differential I then wrote found, on its first run, that **Jinja strips one trailing newline
from every template** (`keep_trailing_newline=False`) and pongo2 does not. Every fragment gained a
`\n`, and `Assemble` joins entries with `"\n\n"`, so that is a blank line per entry and per section
on **every** artifact case. Reverting the one transform rule fails 43 of the 52 fragments.

I had written the argument that made it invisible. "Only the end-to-end gate can check this" is a
claim that needs testing rather than asserting, and spec §8 now records the reasoning rather than
the conclusion.

**The other two blockers**, both invisible to 235 passing unit tests:

1. **`escape_typst_characters` phase 1 rescanned the mutated text.** Upstream's `itertools.chain`
   binds both `finditer`s before the loop mutates `string`, so the command pattern never sees a
   math dummy. Rescanning matched `#emph[RENDERCVTYPSTCOMMANDORMATH0]` as one command and leaked
   the **literal dummy name** into the output.
2. **Python's `\b` is Unicode-aware and Go's is ASCII-only**, so any `bold_keywords` entry starting
   or ending with a non-ASCII character never matched — `["Café"]` left `Café au lait` untouched.
   `ats_diacritics` is a corpus case.

**Cut scope, with its owner:**

- **Wave C — the corpus's artifact cases — moves to iteration 9**, together with iteration 6's
  already-cut T10. Turning one green needs the schema-to-renderer model bridge, which is iteration
  9's by two earlier decisions. What moves with it is only what no fragment exercises:
  `Preamble.j2.typ` and `Header.j2.typ`, which read `cv._connections`, `cv._footer` and effective
  design values. All twenty-three of spec §7's criteria are met here.

**Two things the verifier found that are open, and are not blockers:**

- **`render_entry_templates` and `process_date` were not ported** — `tasks.md` T9 marked behaviors
  58–66 done and only the leaf helpers existed. Both are pure string functions needing nothing from
  the renderer, so they were **under-scoped, not blocked**. **Closed at the head of iteration 9**
  (`EntryDate`, `RenderEntryTemplates`), and the orchestrator is what made the other nine
  processors reachable at all: before it, `process.Run` called `RunFields` directly and no theme
  template was ever expanded.
- **Five `markdown_to_typst` divergences**, none declared: an image is dropped upstream and
  rendered here, raw HTML passes through unescaped upstream, an autolink becomes a link upstream, a
  link title is not stripped here, and a doubled backtick differs. All reachable from ordinary CV
  text. **This needs `specs/divergences.md` and the human gate** unless iteration 9 closes them —
  and unlike the gate I invented for the parser choice, these are user-visible.

**One more, measured and left as is:** an **empty** `bold_keywords` entry produces different
garbage on each side, because Go skips an empty regexp match adjacent to a non-empty one and Python
does not. Documented in its own test with the measurement; whether it is worth a divergence entry
is a human call.

**Process failures, recorded rather than rewritten:** `6f11003` bundles the environment, the loader
and seven filters where `AGENTS.md` §7's table asks for one commit per filter plus one for the
environment; `f785d95` bundles the generator, four filters, the embed, a `justfile` recipe and 26
generated templates across 31 files; and two generated fixtures landed in the same commit as the
code they gate, against §7's "fixtures land first, red".

Also: spec §4F behavior 56 says `-%}` appears **never**. It appears nine times. The transform is
correct anyway — pongo2 implements both trims — but the inventory the plan was sized from was
wrong.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 7

**No cut scope.** Every task landed: the ten-field catalog model with both length messages
(T1–T2), the submodule-diff gate (T3), all twenty-two catalogs (T4–T5), the `$defs` collision
numbering (T26), and the 45 locale `$defs` (T27).

T26 landed here rather than in iteration 6, where spec 005 §7.2's panic pointed. Locale's collision
is a flat list of twenty-two; design's is nine themes × their nested models. Same rule, and getting
it visibly right on the easy case means iteration 6 inherits it working. Its difficulty is
invisible in the output — `$defs` sorts its keys, so an alphabetically-assigned suffix produces a
file that *looks* correctly sorted while pairing every model with the wrong number.

**Axis 3 is now blocked on iteration 6 alone.** 63 of 227 `$defs` are the port's and byte-identical;
the 164 absent are the design tree and the three settings models. The absent-set count in
`internal/schema/jsonschema/golden_conformance_test.go` moved from 227−18 to 227−63, and
`just schema-diff` still emits exactly one `+` line — the diff header — so the port invents nothing.

**Two behaviors were measured only while implementing**, both written into spec §3.2 rather than
only here, and both cases where the port's natural answer differs from upstream:

1. **The twelve-element bound is `EnglishLocale`'s alone.** `{language: danish, month_names: [11
   items]}` validates and the same document with `english` does not — the `at.Len(12, 12)` lives in
   `Annotated` metadata that `create_simple_field_spec` strips when it rebuilds each variant's
   field. T2 had applied it to every member, which **rejects documents upstream accepts**, and no
   English fixture could see it. Fixed in `5ded987`; the schema half of the same fact (no
   `minItems`/`maxItems` on the twenty-one variants) had already been forced correct by the `$defs`
   differential, which is how the validation half was found.
2. **`{locale: {language: null}}` is a failure, not an absence** — `union_tag_invalid` with the tag
   reading `'None'`. Two sibling failures came with it, `union_tag_not_found` for a block with no
   `language` and `model_attributes_type` for a non-mapping `locale`.

**Three findings from the verifier, all closed inside the iteration:**

- **`ValidateCatalog` was unreachable.** `rendercvmodel.go` called only `ValidateLanguage`, so the
  extra-key and length rules T1 and T2 shipped existed and no document could reach them. The edge
  and its eight-row differential landed in `7cb5658`. **This is the failure mode to watch for in
  iteration 6**, whose `design` block is wired the same thin way today.
- **`Languages`' order was verified only transitively**, through the `Locale` `$defs` byte match.
  The order decides the `Phrases__N` numbering — the one thing about the locale `$defs` that cannot
  be read off the output, since the keys sort alphabetically whatever number they carry — so a
  submodule bump that reordered the union would have surfaced as forty-five byte failures naming no
  cause. `TestLanguagesAreInUnionOrder` derives the expectation from the glob.
- **The catalog drift check shares a parser with the tool that wrote the data.** `tools/localeprobe`
  and `catalogs_conformance_test.go` both read the YAML with `goccy/go-yaml` and the same struct
  tags, so a goccy defect would land in the data and the expectation alike. The verifier closed it
  out of band with ruamel — 0 differences, field by field, all 21 files — and the tool's head
  comment now states the gap. Nothing in the repo repeats that check.

**One process failure, recorded rather than rewritten:** `b7531dc` bundles T1, T2 and T5 in one
commit, against `AGENTS.md` §7. `tasks.md` lists all three as separate units.

The parity suite stays at its 42 red cases, which is iteration 1's baseline. No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Carried forward:** date formatting is **not** this iteration's despite the row title it used to
carry — spec §4.1 assigns `2020-09` → `Sept 2020` and §4.2 assigns `degree_with_area`'s
substitution to iteration 9, with the renderer.

