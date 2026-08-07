# tools/typstwasm

The Typst compiler, built for `wasm32-wasip1` and driven through wazero — `divergences.md` D-006,
iteration 10.

This crate is a thin `typst::World` over WASI preopens. It mirrors upstream's `get_typst_compiler`
(`third_party/rendercv/src/rendercv/renderer/pdf_png.py:154-186`), which configures exactly three
things: a **root** directory, a list of **font folders**, and a **package path**.

```
just typst-wasm      # build; output is ~29 MB and is not committed by the recipe
```

## Flags

| Flag | Meaning |
|---|---|
| `--root` | Typst project root. The input must live under it. Upstream passes the `.typ`'s own directory, which is what makes the photo's base-name reference resolve. |
| `--pkg` | Package path, laid out `<namespace>/<name>/<version>/` — upstream builds this in `get_package_path` (`pdf_png.py:114-146`). |
| `--font-dir` | Repeatable. Upstream passes `rendercv_fonts.paths_to_font_folders` plus a `fonts/` directory beside the input file. |
| `--in` / `--out` | Input `.typ`; output file, or for `png` the prefix. |
| `--format` | `pdf` or `png`. |
| `--ppi` | PNG resolution. Defaults to **144**, which is what upstream gets by passing nothing (`typst/__init__.pyi:119`). |
| `--today` | `YYYY-MM-DD` for `World::today`. Upstream uses the real system date; the conformance suite pins it. |

## Two inputs that are not obvious, and both broke parity when missing

1. **`@preview/fontawesome:0.6.0` is not vendored upstream.** `rendercv_typst/lib.typ:1` imports
   it, and upstream resolves it by **downloading from Typst Universe** into the compiler's own
   cache. Only `rendercv` itself is copied into `package_path`. Without it, compilation fails
   loudly: `file not found (searched at .../preview/fontawesome/0.6.0/typst.toml)`.
2. **`typst_assets::fonts()` is a third font source**, separate from `rendercv_fonts`. The
   `sb2nov` theme asks for New Computer Modern, which only the embedded set has. Without it, 12 of
   the 14 PDF cases still pass — which is the point: the failure is quiet on most inputs.

Folder fonts are loaded **before** the embedded ones, so a folder font wins a name tie. That is
typst-cli's `FontSearcher` order, and it is what makes a user's `fonts/` directory override.
