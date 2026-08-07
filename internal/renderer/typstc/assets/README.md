# internal/renderer/typstc/assets

Everything the embedded Typst compiler needs, vendored. `specs/divergences.md` **D-007**.

Nothing here is hand-written or hand-edited. Each file has a recorded provenance below, and
`assets_test.go` pins the ones whose content parity depends on.

| Path | Size | Where it comes from |
|---|---|---|
| `typst.wasm` | 29 MB | `just typst-wasm` — `tools/typstwasm` built for `wasm32-wasip1` |
| `fonts/` | 59 MB | the `rendercv-fonts` package the submodule locks |
| `packages/preview/rendercv/0.3.0/` | 8 KB | `third_party/rendercv/rendercv_typst/` |
| `packages/preview/fontawesome/0.6.0/` | 428 KB | Typst Universe — upstream downloads this at render time |

## `typst.wasm` provenance

```
crate      tools/typstwasm            (typst 0.14.2, pinned by its Cargo.lock)
target     wasm32-wasip1
profile    release — opt-level = "z", lto = true, strip = true
rustc      1.95.0 (59807616e 2026-04-14)
sha256     6a159f83a3ca5b4a42a8ecb13296a79ab5cbeef2dfd9572161f780b11f5de91c
```

Rebuild with `just typst-wasm` and copy the output over this file. The hash is expected to move
with the toolchain; what must not move is the *behavior*, and that is what the PDF differential
in the parity suite measures.

## Why the binary is committed rather than fetched

Measured, not assumed. Resolving fonts from the host instead of vendoring them renders `sb2nov`
in a fallback face and **still passes 12 of the 14 PDF cases** — a quiet failure on 86% of the
corpus. D-007 records the decision and its cost.
