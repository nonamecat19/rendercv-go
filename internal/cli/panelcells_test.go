package cli

import "testing"

// TestPanelMeasuresDisplayCells pins the panel's width measurement against
// Rich's `cell_len` (`rich/cells.py`), which counts **display columns**, not
// codepoints. `panel.go` counted runes, and its own comment predicted the
// failure: "a CJK name in a path would break that, and no golden has one."
//
// It is reachable from an ordinary `new`: `rendercv new "Ольга Ковальчук 李雷"`
// puts the name in the input file's path, and the two wide characters made the
// port pad two columns too far — the box overflowed its own border.
//
// **Every `want` below is captured, never composed**: each is the exact stdout
// of the vendored `rich.panel.Panel(body, title="Get started",
// title_align="left", border_style="bright_black")` at width 80, printed to a
// pipe. Counting the spaces by hand would assert the author's arithmetic
// instead of upstream's output.
//
// The classes swept are the ones where a rune is not a column: East-Asian wide
// (Han, Hangul, kana, fullwidth forms), combining marks, zero-width space,
// zero-width joiner sequences, variation selector 16, astral emoji, and an
// unstripped `\x01`.
//
// **The `\x01` row keeps an upstream quirk.** Rich scores that control
// character 0, so upstream's own line is one column *wider* than the box it
// claims to draw. Matching upstream means reproducing that, not fixing it.
func TestPanelMeasuresDisplayCells(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "cjk name",
			body: "✓ Created your YAML input file: ./Ольга_Ковальчук_李雷_CV.yaml",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ ✓ Created your YAML input file: ./Ольга_Ковальчук_李雷_CV.yaml               │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "wide only",
			body: "李雷 韩梅梅",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ 李雷 韩梅梅                                                                  │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "fullwidth forms",
			body: "Ｆｕｌｌ　ｗｉｄｔｈ",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Ｆｕｌｌ　ｗｉｄｔｈ                                                         │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "hangul",
			body: "한국어 이름",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ 한국어 이름                                                                  │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "kana",
			body: "ありがとう テスト",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ ありがとう テスト                                                            │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "combining marks",
			body: "José Nuñez",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ José Nuñez                                                                   │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "combining on wide",
			body: "Wide中́ mark",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Wide中́ mark                                                                  │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "zero width space",
			body: "Zero\u200bWidth",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Zero\u200bWidth                                                                    │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "unstripped control",
			body: "Ctrl\x01Char",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Ctrl\x01Char                                                                     │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "emoji",
			body: "Emoji 😀 here",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Emoji 😀 here                                                                │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "zwj emoji",
			body: "Family 👨\u200d👩\u200d👧",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Family 👨\u200d👩\u200d👧                                                                    │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "variation selector 16",
			body: "Heart ❤\ufe0f",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Heart ❤\ufe0f                                                                     │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "mixed",
			body: "Mixed 李 Ольга José 😀",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ Mixed 李 Ольга José 😀                                                       │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "wide wrap",
			body: "李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ 李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩 │\n" +
				"│ 梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅李雷韩梅梅                                 │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			t.Setenv("COLUMNS", "80")
			if got := Panel("Get started", []PanelRow{{Text: row.body}}); got != row.want {
				t.Errorf("panel =\n%q\nwant\n%q", got, row.want)
			}
		})
	}
}
