package cmdpanel

import "testing"

// The two panels below are real bytes, not a sketch: both were captured from the
// pinned upstream (`third_party/rendercv`, `2eba248`) with the corpus environment
// (`COLUMNS=80 NO_COLOR=1 TERM=dumb`). `readdirOrder` came from a `cp -r` copy of
// `src/` on `PYTHONPATH`, `nameOrder` from the submodule itself. They are the same
// length, byte for byte, and differ only in the order of the three entries.
const readdirOrder = `╭─ Commands ───────────────────────────────────────────────────────────────────╮
│ render        Render a YAML input file. Example: rendercv render             │
│               John_Doe_CV.yaml. Details: rendercv render --help              │
│ new           Generate a YAML input file to get started. Example: rendercv   │
│               new "John Doe". Details: rendercv new --help                   │
│ create-theme  Create a custom theme folder with Typst templates to           │
│               customize. Example: rendercv create-theme customtheme.         │
│               Details: rendercv create-theme --help                          │
╰──────────────────────────────────────────────────────────────────────────────╯
`

const nameOrder = `╭─ Commands ───────────────────────────────────────────────────────────────────╮
│ create-theme  Create a custom theme folder with Typst templates to           │
│               customize. Example: rendercv create-theme customtheme.         │
│               Details: rendercv create-theme --help                          │
│ new           Generate a YAML input file to get started. Example: rendercv   │
│               new "John Doe". Details: rendercv new --help                   │
│ render        Render a YAML input file. Example: rendercv render             │
│               John_Doe_CV.yaml. Details: rendercv render --help              │
╰──────────────────────────────────────────────────────────────────────────────╯
`

// The `Options` panel of the same page. Its order is declared in upstream's
// source, so Sort must not touch it — the shape is otherwise identical to the
// `Commands` panel's.
const optionsPanel = `╭─ Options ────────────────────────────────────────────────────────────────────╮
│ --version             -v        Show the version                             │
│ --install-completion            Install completion for the current shell.    │
│ --show-completion               Show completion for the current shell, to    │
│                                 copy it or customize the installation.       │
│ --help                -h        Show this message and exit.                  │
╰──────────────────────────────────────────────────────────────────────────────╯
`

func TestSort(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a panel in readdir order becomes name order",
			in:   readdirOrder,
			want: nameOrder,
		},
		{
			name: "a panel already in name order is untouched",
			in:   nameOrder,
			want: nameOrder,
		},
		{
			name: "the options panel keeps its declared order",
			in:   optionsPanel,
			want: optionsPanel,
		},
		{
			name: "a whole page sorts only its commands panel",
			in:   optionsPanel + readdirOrder,
			want: optionsPanel + nameOrder,
		},
		{
			name: "output with no panel at all",
			in:   "Render a YAML input file.\n",
			want: "Render a YAML input file.\n",
		},
		{
			name: "an unterminated panel is left alone",
			in:   "╭─ Commands ─╮\n│ render  x  │\n│ new     y  │\n",
			want: "╭─ Commands ─╮\n│ render  x  │\n│ new     y  │\n",
		},
		{
			name: "an empty panel is left alone",
			in:   "╭─ Commands ─╮\n╰────────────╯\n",
			want: "╭─ Commands ─╮\n╰────────────╯\n",
		},
		{
			name: "a body opening with a continuation is left alone",
			in:   "╭─ Commands ─╮\n│    wrapped │\n│ new     y  │\n╰────────────╯\n",
			want: "╭─ Commands ─╮\n│    wrapped │\n│ new     y  │\n╰────────────╯\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sort(tt.in); got != tt.want {
				t.Errorf("Sort() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestSortIsIdempotent pins that the canonical order is a fixed point: the
// generator and the harness both apply it, sometimes to output that has already
// been through it.
func TestSortIsIdempotent(t *testing.T) {
	once := Sort(readdirOrder)
	if twice := Sort(once); twice != once {
		t.Errorf("Sort is not idempotent:\n%s", twice)
	}
}

// TestSortPreservesLength pins the property that makes the canonicalization safe
// to apply to a golden: it moves rows, it never rewrites one.
func TestSortPreservesLength(t *testing.T) {
	if got, want := len(Sort(readdirOrder)), len(readdirOrder); got != want {
		t.Errorf("Sort changed the byte count: %d, want %d", got, want)
	}
}
