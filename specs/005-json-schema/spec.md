# Iteration 5 — JSON Schema generation

Behavior of `rendercv-go schema`, extracted from the vendored Python. No Go design here; that is
`plan.md`.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary source: `src/rendercv/schema/json_schema_generator.py` (45 lines).
Contract: `specs/000-parity-contract/spec.md` Axis 3 — `rendercv-go schema` diffs empty against
`third_party/rendercv/schema.json`.

---

## 1. The finding that decides this iteration's scope

**The generator is 45 lines. The schema is 405 KB, and 95% of it is models this port has not
written yet.**

Measured over the 227 `$defs`, by the iteration that owns the model each one comes from:

| Owner | `$defs` | Bytes | Share of `$defs` |
|---|---:|---:|---:|
| Iteration 6 — design and themes | 161 | 225,205 | 74.8% |
| Iteration 7 — locale | 45 | 51,266 | 17.0% |
| Iteration 7 — settings | 3 | 10,042 | 3.3% |
| **Iterations 2–4 — cv, entries, the shared types** | **18** | **14,467** | **4.8%** |

The whole file is 404,754 bytes.

1. **Axis 3 cannot close in this iteration**, and no amount of work on the generator changes
   that. `schema.json` is a faithful projection of the model tree, so it is complete exactly when
   the model tree is. The 161 design `$defs` are the nine themes × their nested option models
   (`Colors`, `Typography`, `Header`, …, each emitted once per theme with a `__1`…`__9` collision
   suffix); the 45 locale ones are the twenty-two catalogs plus their `Phrases` models.
2. **The generator is nonetheless portable now**, and separately: its mechanism — the `$defs`
   naming and collision-suffix rules, `$ref` emission, key ordering, the four keys it appends,
   the serialization — is independent of *which* models exist. It can be built and gated against
   the 4.8% that does exist, then produce the other 95% for free as iterations 6 and 7 land.
3. **So this iteration's gate is not `just schema-diff`.** §7 states what it is instead, and §8
   is explicit that a red `schema-diff` at the end of this iteration is the expected result and
   not a failure.

This is a scoping finding, not a divergence: nothing here says parity is unreachable, only that
it is unreachable *in this order*. §7.1 records the alternative and why it was not taken.

---

## 2. What the command does

1. `rendercv-go schema` writes the JSON Schema of `RenderCVModel` to stdout and exits 0.
2. Upstream exposes the same thing two ways: `generate_json_schema()` returns the dictionary and
   `generate_json_schema_file(path)` writes it (`json_schema_generator.py:36-45`). The committed
   `schema.json` at the repository root is that file's output.
3. There is no flag, no argument and no configuration. The schema is a pure function of the model
   tree.

---

## 3. The generator

### 3.1 The four keys it adds

4. `generate_json_schema` subclasses pydantic's `GenerateJsonSchema` and overrides `generate` to
   set four keys after the base implementation has run (`:23-32`), in this order:

   ```python
   json_schema["title"] = "RenderCV"
   json_schema["description"] = __description__
   json_schema["$id"] = "https://raw.githubusercontent.com/rendercv/rendercv/main/schema.json"
   json_schema["$schema"] = "http://json-schema.org/draft-07/schema#"
   ```

5. `__description__` is `Resume builder for academics and engineers` (measured).
6. **`title` is an assignment to an existing key and the other three are insertions.** That is
   directly observable in the top-level key order (§3.2 behavior 8) and it is the whole reason
   that order is not sorted.
7. Nothing else is overridden. Every other key, every `$ref`, every `$defs` entry is stock
   pydantic.

### 3.2 Key order

8. **The top-level object's keys are, in order** (measured):

   ```
   $defs, additionalProperties, properties, required, title, type, description, $id, $schema
   ```

   The first six are sorted (`$` is 0x24, below every letter). The last three are the generator's
   insertions, in the order it makes them. `title` sits in the sorted run because it already
   existed and was overwritten in place.
9. **Every other object's keys are sorted.** Measured on `$defs.Cv`
   (`additionalProperties, properties, title, type`), on a property schema
   (`anyOf, default, examples, title`), and on `$defs` itself, whose 227 keys are in ASCII order.
10. `properties` is the exception that is not an exception: its keys are **field declaration
    order**, not sorted, because they are data rather than schema keywords. Measured on `Cv`:
    `name, headline, location, email, photo, phone, website, social_networks,
    custom_connections, sections` — which is spec 002 §3.44's order.

### 3.3 `$defs` naming and the collision suffix

11. A model is named by its class name when that name is unique across the tree, and by its
    **fully qualified module path with `.` replaced by `__`** when it is not. Measured: 173 of the
    227 entries carry a qualified name, e.g.
    `rendercv__schema__models__cv__entries__education__EducationEntry`.
12. When two *distinct* models would still collide after qualification — the per-theme variants,
    which are generated classes sharing a module — pydantic appends `__1`, `__2`, … in the order
    it first emitted them. Measured: `rendercv__schema__models__design__classic_theme__Colors__1`
    through `__7`, one per theme that carries a `Colors` block.
13. **The suffix numbering is therefore emission-order-dependent, not alphabetical**, and
    reproducing it requires walking the model tree in pydantic's order. This is the single
    hardest thing in the iteration and §7.2 scopes it.

### 3.4 Serialization

14. `json.dumps(schema, indent=2, ensure_ascii=False)` (`:44`). Three consequences, each an
    acceptance criterion:
    - two-space indent;
    - **non-ASCII characters are literal, not escaped** — the file contains 1,269 of them, from
      `Boğaziçi University` in `EducationEntry`'s examples to the accented month names in the
      locale catalogs;
    - Python's separators, so `": "` after a key and `", "` between items.
15. **The file has no trailing newline.** `write_text` writes exactly what `dumps` returned
    (`:45`), and `dumps` does not append one. Measured: the last three bytes are `"\n}`.

---

## 4. Exact strings

### 4.1 Title

```
RenderCV
```

### 4.2 Description — `rendercv/__init__.py`

```
Resume builder for academics and engineers
```

### 4.3 `$id`

```
https://raw.githubusercontent.com/rendercv/rendercv/main/schema.json
```

### 4.4 `$schema`

```
http://json-schema.org/draft-07/schema#
```

---

## 5. Edge cases

16. `required` at the top level is `[]`, not absent — every field of `RenderCVModel` has a
    default (spec 002 §3.28), and pydantic emits the empty list rather than omitting the key.
17. `additionalProperties` is `false` at the top level and on `Cv`, and `true` on the entry
    models, which mirrors the two base classes of spec 002 §3.32. It is emitted explicitly in
    both cases.
18. A model with no docstring emits `"description": null` rather than omitting the key. Measured
    on `EducationEntry`.
19. The schema is **Draft-07 by declaration only**. Pydantic emits 2020-12 constructs (`$defs`,
    `anyOf` with `null` members) and the generator relabels the dialect. Reproducing the label is
    parity; reproducing Draft-07 semantics is not asked for and would be a divergence.

---

## 6. Ordering guarantees

1. Top-level keys: §3.2 behavior 8's exact sequence, which is not sorted.
2. Every other object's keys: sorted, ASCII.
3. `properties`: field declaration order.
4. `$defs`: sorted by key, ASCII — so the collision suffixes sort `__1` before `__2` and
   `Colors__7` before `Connections__1`.
5. Collision suffixes are assigned in **emission** order, which is neither of the above.
6. `enum` members keep the declaration order of the Python `Literal` they come from, not sorted.

---

## 7. Out of scope, and the re-scoping decision

**7.1 Axis 3 closes in iteration 7, not here, and the iterations are not reordered.**

The alternative was to move design and locale ahead of the generator so that Axis 3 could close in
one iteration. It is rejected for two reasons:

- The generator is what makes the design and locale models *checkable*. Written first, it gives
  iterations 6 and 7 a mechanical gate — every theme and every catalog either produces its
  `$defs` entry byte-for-byte or does not — instead of leaving them verified only by their own
  unit tests. That is the same argument spec 004 §7.2 made for fixing the coordinate columns
  before the differential widened.
- Reordering would not reduce the total work, only move it, and it would put the two largest
  model iterations back to back with no gate between them.

So: this iteration builds the generator and gates it on the subtree that exists. `just
schema-diff` stays red until iteration 7, and `specs/STATE.md` records Axis 3 as *blocked on
iterations 6–7* rather than *failing*.

**7.2 The collision-suffix numbering is deferred to iteration 6.**

Behavior 13's rule is only exercised by the per-theme variants, all of which are iteration 6's
models. Building the numbering now would mean building it against models that do not exist and
testing it against nothing. This iteration therefore implements the **naming** rules of behaviors
11 and 12 and leaves the numbering to the iteration that first produces a collision — with a
failing-loudly placeholder rather than a silent wrong answer.

**7.3 The committed `schema.json` file is not written.** Upstream's
`generate_json_schema_file` exists to refresh a checked-in artifact. `rendercv-go` has no such
artifact of its own — the parity target is upstream's — so only the stdout path is ported.

**7.4 `enum` member order for `Literal` types is asserted, not derived.** Pydantic preserves the
order of the Python `Literal`'s arguments. Where the Go port already carries the same list in the
same order — `SocialNetworkName` (spec 004 §4.23), `Languages` (§4.30), the nine theme names — the
schema takes it from there. Where it does not, iteration 6 or 7 supplies it.

---

## 8. Acceptance criteria

**The generator's mechanism**

- [ ] The four added keys, with §4's exact strings, in §3.1 behavior 4's order.
- [ ] The top-level key order of §3.2 behavior 8, exactly, including the fact that it is not
      sorted and that `title` sits in the sorted run.
- [ ] Every other object's keys sorted; `properties` in declaration order.
- [ ] `$defs` sorted by key.
- [ ] Two-space indent, literal non-ASCII, `": "` and `", "` separators, **no trailing newline**.

**The subtree that exists** *(the gate)*

- [ ] The 18 `$defs` owned by iterations 2–4 are byte-identical to upstream's, compared
      individually. Enumerated, because the list is the gate:

      ```
      ArbitraryDate                ListOfEntries
      BulletEntry                  NumberedEntry
      CustomConnection             ReversedNumberedEntry
      Cv                           Section
      ExactDate                    SocialNetwork
      ExistingPathRelativeToInput  SocialNetworkName
      TypstDimension
      rendercv__schema__models__cv__entries__education__EducationEntry
      rendercv__schema__models__cv__entries__experience__ExperienceEntry
      rendercv__schema__models__cv__entries__normal__NormalEntry
      rendercv__schema__models__cv__entries__one_line__OneLineEntry
      rendercv__schema__models__cv__entries__publication__PublicationEntry
      ```

      Two things about that list are worth naming. `TypstDimension` is in it although it is a
      design type, because `Cv` does not reach it and nothing else does either — it is shared and
      already modelled. And **`TextEntry` is absent**: it is `str`, not a model, so it has no
      `$defs` entry at all (spec 003 §3.1).
- [ ] The top-level object minus `$defs` is byte-identical.
- [ ] A test asserts **which** `$defs` are expected to be absent and fails if the list shrinks
      without the corresponding models landing — so iterations 6 and 7 cannot forget to close it.

**Not this iteration**

- [ ] `just schema-diff` is red, and `specs/STATE.md` says so with the reason. A green
      `schema-diff` before iteration 7 would mean the generator is emitting something upstream
      does not have.
