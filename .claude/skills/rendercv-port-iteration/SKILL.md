---
name: rendercv-port-iteration
description: Run one full port iteration of the Python-to-Go RenderCV rewrite — investigate upstream, write the spec, land red conformance tests, fan out porters, verify in a fresh context, update the ledger. Use when starting or continuing work on any numbered iteration in specs/STATE.md, or when the user says "port X", "next iteration", or names a subsystem (entry types, themes, locales, templater, CLI).
---

# Port iteration

One iteration = one subsystem = one `specs/NNN-*/` directory = many commits.

Read `AGENTS.md` first if you have not this session. §5 (task graph) and §7 (commits) govern
everything below.

## The shape

```
        ┌─ porter A ─┐
spec ───┼─ porter B ─┼──▶ parity-verifier ──▶ merge ──▶ STATE.md
        └─ porter C ─┘        (fresh context)
```

## Steps

### 1. Pick the iteration
Open `specs/STATE.md`. Take the lowest-numbered row that is not `green`. Do not skip ahead —
the spine is ordered for a reason. Set its status to `spec` and note the date in the log.

### 2. Investigate upstream
Spawn `rendercv-upstream-analyst` with the concrete questions the spec needs answered. Give it
the upstream paths from `AGENTS.md` §2. Insist on `file:line` citations and verbatim strings.

Also point it at the mirrored upstream tests — `third_party/rendercv/tests/<same path>/` — those
encode edge cases you would otherwise miss.

### 3. Write the spec
Spawn `rendercv-spec-writer` with the analyst's report. It produces `spec.md`, then `plan.md`,
then `tasks.md`. Review `tasks.md` yourself before proceeding: every task must be one commit,
and each must be marked `[parallel]` or `[sequential]`.

If the spec proposes a divergence, **stop.** Divergences are human-gated (`AGENTS.md` §5).

### 4. Land the tests red
Add this iteration's corpus cases to `tools/gengolden`, regenerate goldens
(`rendercv-golden-refresh` skill — human-gated), and write the conformance tests behind
`//go:build conformance`. Commit fixtures and tests **before** any implementation. They must
fail for the right reason: assert the failure is "not implemented", not "test is broken".

Set the iteration status to `red`.

### 5. Fan out — but only the leaves
Apply the stop rule. Split **only** tasks marked `[parallel]`: independent entry types, themes,
locales, templates, filters. Spawn one `rendercv-porter` per task, in one message, in parallel.

Everything marked `[sequential]` — anything on the schema → templater → renderer → CLI spine —
**you** do yourself, in this context. Do not spawn agents for it. Multi-agent configurations lose
on sequential work where each step needs the full picture.

Never let more than one agent own a file.

### 6. Verify in a fresh context
Spawn `rendercv-parity-verifier`. It is never one of the porters. Give it the iteration's spec
path and commit range.

If it reports blockers: fix them (or send them back to a porter), then spawn a **new** verifier.
Do not argue with the verifier from the porter's context.

### 7. Merge and record
You own the merge — one owner, always. Then update `specs/STATE.md`:

- iteration status → `green` only if every conformance case for it passes
- the passing-case count
- the axis table, if this iteration made an axis measurable
- the log

Anything cut from the iteration goes in the "Cut scope" section with a reason. An iteration is
never marked done with a failing case (`AGENTS.md` §10.2).

### 8. Stop
Report the iteration summary and stop. Do not roll into the next iteration unasked — the human
decides when to continue, and iteration boundaries are the cheapest place to correct course.

## Anti-patterns

- Spawning agents for the pipeline spine. It degrades results; the stop rule exists because of
  measured evidence, not taste.
- Letting the porter that wrote the code also verify it.
- Regenerating goldens to make a failing test pass. That is deleting the contract, not meeting it.
- One big commit at the end of the iteration.
- Marking `green` with skipped tests.
