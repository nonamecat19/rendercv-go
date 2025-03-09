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
schema-diff: build
    diff -u {{upstream_dir}}/schema.json <({{bin}} schema)

# Run the vendored Python RenderCV: `just upstream render CV.yaml`
upstream *args:
    cd {{upstream_dir}} && uv run --frozen --all-extras rendercv {{args}}

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
