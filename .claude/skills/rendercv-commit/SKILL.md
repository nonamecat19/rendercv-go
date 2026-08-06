---
name: rendercv-commit
description: Split a dirty working tree into an ordered sequence of single-purpose commits and write their messages. Use before committing anything in this repo, and whenever a change touches more than one logical unit. Enforces AGENTS.md §7 commit discipline.
---

# Commit splitting

This repo forbids bundled commits (`AGENTS.md` §7). A port is only reviewable if each commit is
one thing. This skill turns a dirty tree into that sequence.

## 1. Survey

```bash
git status --short
git diff --stat
git diff --cached --stat
```

## 2. Partition

Group every changed hunk into exactly one logical unit. Split by **purpose**, not by file — one
file can contribute to several commits, and often should.

Mandatory splits:

| If the tree contains… | Then… |
|---|---|
| N entry types / themes / locales / templates | N commits, one each |
| goldens + the code they test | goldens first, in their own commit, landing red |
| a refactor + a feature | two commits, refactor first |
| a fix + unrelated formatting | two commits |
| a submodule bump + anything | bump alone, first, human-gated |
| `specs/STATE.md` + implementation | ledger updates are their own commit, by the merge owner |
| `specs/divergences.md` + anything | divergences alone, human-gated |
| generated files + hand-written files | separate |

Ordering rule: each commit must leave `go build ./... && go test ./...` green on its own. That
usually means types → implementation → wiring → tests-enabled.

## 3. Present the plan before touching git

```
1. test: add golden fixtures for education entry    [testdata/golden/education/**]
2. feat: port EducationEntry model                  [internal/schema/models/cv/entries/education.go, _test.go]
3. feat: port EducationEntry typst template         [internal/renderer/templater/templates/typst/entries/EducationEntry.j2.typ]
4. docs: record education entry in the ledger       [specs/STATE.md]
```

Wait for confirmation. Then stage per commit — `git add <paths>`, or `git add -p` when one file
spans several units.

## 4. Messages

Conventional Commits. **Subject ≤ 50 characters**, imperative, no trailing period.

Types: `feat` `fix` `test` `docs` `refactor` `perf` `chore` `build` `ci`

Body only when the "why" is not obvious from the subject. When present, wrap at 72 and say why,
not what — the diff already says what. Cite the upstream construct when porting:

```
feat: port EducationEntry model

Mirrors src/rendercv/schema/models/cv/entries/education.py. Degree
column handling follows the design.templates flag rather than being
inferred, matching upstream's ordering of optional columns.
```

Every commit ends with:

```
Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: <session URL>
```

## 5. Verify before each commit

```bash
just check && just test
```

Both clean. A commit that breaks the build is worse than a bundled one — it breaks `git bisect`,
which is the only tool that will save this port when a whitespace regression appears eight
iterations later.

## 6. Never

- `git push` — human gate (`AGENTS.md` §5).
- `git commit -a`.
- Amend or rebase a commit that has been pushed.
- Commit inside `third_party/rendercv/`.
- Commit `.claude/settings.local.json` or anything from `/testdata/.work/`.
