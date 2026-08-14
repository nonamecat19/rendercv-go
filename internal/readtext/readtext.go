// Package readtext mirrors the one call upstream reads every input document
// with: `pathlib.Path.read_text(encoding="utf-8")`.
//
// The encoding is the visible half of that call and the uninteresting one. The
// half that changes what the parser sees is the **default `newline=None`**:
// `read_text` forwards it to `Path.open` (CPython 3.12.13 `pathlib.py:1022-1028`),
// which is text mode with universal-newline translation enabled, so
// `IncrementalNewlineDecoder.decode` rewrites `\r\n` and then every remaining
// lone `\r` to `\n` (`_pyio.py:1925-1929`; the C accelerator in
// `Modules/_io/textio.c` mirrors it) before the string reaches the caller.
//
// ruamel therefore never sees a carriage return in a document upstream read
// from disk, and neither may the port's parser. Reading raw bytes instead was
// measured as a validation-error divergence: for `cv:\n  name: \rA\n` upstream
// reports `while scanning a simple key.` spanning line 3 to line 4 — the span
// of the *translated* document, which has one more line — and the port reported
// `while parsing a block mapping.` from line 1 to line 3 (spec delta 002-P §6).
package readtext

import (
	"os"
	"strings"
)

// Universal applies Python's universal-newline translation to already-decoded
// text: `\r\n` becomes `\n`, and each remaining lone `\r` becomes `\n`.
//
// The two replacements run in that order, which is `_pyio.py:1926-1929`'s own
// order and the only one that is correct — swapping them would turn `\r\n` into
// two line breaks.
//
// **It is context-free**, exactly as the decoder is: a `\r` inside a quoted
// scalar or a block scalar is translated too, because the translation happens a
// whole layer below YAML. Measured, both sides: `name: "A<CR>B"` folds to
// `A B` because the CR became a line break inside a multi-line double-quoted
// scalar, and a `<CR>` inside a literal block scalar becomes a line break in
// the block's content.
//
// A `\r` escape *written* as `\r` in a double-quoted scalar is untouched: it is
// produced by the YAML scanner, long after the read boundary, and upstream
// carries that carriage return into the rendered artifacts too.
func Universal(text string) string {
	if !strings.ContainsRune(text, '\r') {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

// File reads a file the way upstream's `read_text` does: the bytes, then the
// newline translation.
//
// It deliberately does not decode or validate UTF-8. Upstream's decode is
// strict and raises `UnicodeDecodeError` out of `read_text` before any RenderCV
// code runs, which is the unhandled-traceback class D-011 already covers; the
// port keeps its byte-preserving behavior there rather than inventing a message
// for it (spec delta 002-P §6).
func File(path string) ([]byte, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the caller's own argument
	if err != nil {
		return nil, err
	}
	return []byte(Universal(string(raw))), nil
}
