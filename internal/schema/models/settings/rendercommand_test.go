package settings_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// TestResolveRenderCommand is spec 012 gaps.md G-6.
//
// **The port used to gate generation on the CLI flag alone**, so a document —
// or a `--settings` overlay — asking for no PDF still got a PDF and both PNGs.
// Upstream gates inside each generator on the merged model, which is what makes
// a document's own switches work at all.
func TestResolveRenderCommand(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want settings.RenderCommand
	}{
		{
			name: "no settings block",
			yaml: "cv:\n  name: X\n",
		},
		{
			name: "no render_command block",
			yaml: "settings:\n  current_date: 2025-03-05\n",
		},
		{
			name: "pdf and png off",
			yaml: "settings:\n  render_command:\n    dont_generate_pdf: true\n    dont_generate_png: true\n",
			want: settings.RenderCommand{DontGeneratePDF: true, DontGeneratePNG: true},
		},
		{
			name: "every switch on",
			yaml: "settings:\n  render_command:\n    dont_generate_typst: true\n" +
				"    dont_generate_pdf: true\n    dont_generate_png: true\n" +
				"    dont_generate_markdown: true\n    dont_generate_html: true\n",
			want: settings.RenderCommand{
				DontGenerateTypst: true, DontGeneratePDF: true, DontGeneratePNG: true,
				DontGenerateMarkdown: true, DontGenerateHTML: true,
			},
		},
		{
			// `false` leaves the switch on; a lax-boolean string ("yes") is
			// accepted and takes effect, same as the literal `true`
			// (spec 006 delta §3.4, §9 — `set_bool_words`).
			name: "false leaves it on, a lax-boolean word takes effect",
			yaml: "settings:\n  render_command:\n    dont_generate_pdf: false\n    dont_generate_png: \"yes\"\n",
			want: settings.RenderCommand{DontGeneratePNG: true},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			document, err := yamlreader.ReadString(row.yaml)
			if err != nil {
				t.Fatalf("reading the fixture: %v", err)
			}
			got := settings.Resolve(settingsBlock(document), time.Now()).RenderCommand
			if got != row.want {
				t.Errorf("RenderCommand = %+v, want %+v", got, row.want)
			}
		})
	}
}

// settingsBlock is the document's `settings` mapping, or nil when it has none.
func settingsBlock(document *yamldoc.Node) *yamldoc.Node {
	if document == nil || document.Kind != yamldoc.KindMapping {
		return nil
	}
	for _, item := range document.Items {
		if item.Key == "settings" {
			return item.Value
		}
	}
	return nil
}
