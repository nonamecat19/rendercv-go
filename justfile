# rendercv-go — see AGENTS.md §8

upstream_dir := "third_party/rendercv"
bin := "bin/rendercv-go"

default:
    @just --list

# --- Setup -------------------------------------------------------------------

# Install everything needed to work on this repo.
setup: submodule
    go mod download
    go install mvdan.cc/gofumpt@latest
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    go install golang.org/x/tools/gopls@latest
    cd {{upstream_dir}} && uv sync --frozen --all-extras

# Initialise / update the pinned upstream submodule.
submodule:
    git submodule update --init --recursive
    @echo "upstream pinned at: $(git -C {{upstream_dir}} rev-parse --short HEAD) ($(git -C {{upstream_dir}} describe --tags))"

# --- Build -------------------------------------------------------------------

build:
    go build -o {{bin}} ./cmd/rendercv-go

clean:
    rm -rf bin dist testdata/.work

# --- Quality -----------------------------------------------------------------

check: fmt-check vet lint

fmt:
    gofumpt -w .

fmt-check:
    @command -v gofumpt >/dev/null || { echo "gofumpt not installed; run 'just setup'"; exit 1; }
    @out=$(gofumpt -l .); if [ -n "$out" ]; then echo "gofumpt would reformat:"; echo "$out"; exit 1; fi

vet:
    go vet ./...

lint:
    golangci-lint run

# --- Tests -------------------------------------------------------------------

test:
    go test ./...

test-coverage:
    go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

# The parity suite. This, and only this, is what "parity" means (AGENTS.md §10.6).
test-parity: build
    go test -tags conformance ./...

# Run one conformance case: `just parity-case classic_full`
parity-case case: build
    go test -tags conformance ./internal/conformance/... -run '{{case}}' -v

# --- Golden fixtures ---------------------------------------------------------

# HUMAN GATE. Changes the parity contract. Use the rendercv-golden-refresh skill.
golden:
    @echo "This regenerates the parity contract. Follow .claude/skills/rendercv-golden-refresh."
    go run ./tools/gengolden

golden-verify:
    go run ./tools/gengolden -verify

# --- Parity probes -----------------------------------------------------------

# Diff our JSON schema against upstream's (contract axis 3).
#
# A tool rather than a `rendercv-go schema` subcommand: axis 2 forbids adding
# commands, and upstream has no such command either. Axis 3's own parenthetical
# allows an equivalent generation path, and this is it.
#
# Red until iteration 7 by design — 209 of upstream's 227 $defs come from the
# design and locale models. See specs/005-json-schema/spec.md §1.
schema-diff:
    diff -u {{upstream_dir}}/schema.json <(go run ./tools/genschema)

# Build the Typst compiler for WASI (D-006). Needs the Rust toolchain and
# `rustup target add wasm32-wasip1`. Pinned to typst 0.14.2 by Cargo.lock, which
# is upstream's line: rendercv_typst/typst.toml declares compiler 0.14.0 and the
# installed typst-py is 0.14.8.
#
# The output is ~29 MB and is NOT committed by this recipe — see
# specs/010-typst-compilation/tasks.md T2, which is human-gated.
typst-wasm:
    cd tools/typstwasm && cargo build --release --target wasm32-wasip1
    @ls -l tools/typstwasm/target/wasm32-wasip1/release/typstwasm.wasm

# Run the vendored Python RenderCV: `just upstream render CV.yaml`
upstream *args:
    cd {{upstream_dir}} && uv run --frozen --all-extras rendercv {{args}}

# Regenerate the locale catalogs from the vendored Python.
# Generated, never hand-edited: 210 mostly non-ASCII strings. See
# tools/localeprobe for what the conformance diff does and does not guarantee.
localeprobe:
    go run ./tools/localeprobe

# Regenerate the pongo2 templates from the vendored Python's Jinja ones.
# Generated, never hand-edited. The transform is semantic, not a copy — see
# tools/gentemplates for what it cannot check.
gentemplates:
    go run ./tools/gentemplates

# Regenerate the design subsystem's data from the vendored Python.
# Generated, never hand-edited. The colour names come from the installed
# pydantic_extra_types rather than from a submodule file — see tools/designprobe.
designprobe:
    go run ./tools/designprobe

# Regenerate the help-page model — usage lines, descriptions and every option
# row — from the vendored typer. Generated, never hand-edited (AGENTS.md §10.1).
# Not a golden: no human gate. The layout is not in here; see specs/012-cli/help.md.
helpprobe:
    go run ./tools/helpprobe

# Regenerate the entry-model field-order fixture from the vendored Python.
# Generated, never hand-edited (AGENTS.md §10.1). Not a golden: no human gate.
entryprobe:
    go run ./tools/entryprobe

# Regenerate the entry-dump fixture: what `model_dump(exclude_none=True)` gives
# for a set of validated entries, which is the bridge's boundary (spec 009).
dumpprobe:
    go run ./tools/dumpprobe

# Regenerate the phone-formatting fixture from the vendored Python's own
# libphonenumber: what `PhoneNumber` stores and how each format prints it.
phoneprobe:
    go run ./tools/phoneprobe

# Regenerate the starter CV's raw blocks — one `cv`, nine `design`, twenty-two
# `locale`, one `settings` — plus the 198-document digest matrix and the
# `cv.name` battery `new`'s generator is tested against. See tools/sampleprobe.
sampleprobe:
    go run ./tools/sampleprobe

# Render every corpus input with the vendored Python and store the `.typ`, `.md`
# and `.html`. This is the parity gate for iterations 9 and 11; see
# tools/docprobe for what it skips.
docprobe:
    go run ./tools/docprobe

# Render the same input with both and diff every artifact.
diff-render input:
    go run ./tools/gengolden -diff {{input}}

# --- Specs -------------------------------------------------------------------

# Scaffold specs/NNN-<name>/{spec,plan,tasks}.md
spec name:
    #!/usr/bin/env bash
    set -euo pipefail
    n=$(printf '%03d' $(( $(ls -d specs/[0-9][0-9][0-9]-* 2>/dev/null | wc -l) )))
    dir="specs/${n}-{{name}}"
    mkdir -p "$dir"
    for f in spec plan tasks; do
      [ -f "$dir/$f.md" ] || printf '# %s %s — %s\n\n**Status:** draft\n**Inherits:** specs/000-parity-contract/spec.md\n' \
        "${f^}" "$n" "{{name}}" > "$dir/$f.md"
    done
    echo "created $dir"

state:
    @cat specs/STATE.md
