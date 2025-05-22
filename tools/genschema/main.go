// Command genschema writes the JSON Schema of RenderCVModel to stdout.
//
// **A tool rather than a `rendercv-go schema` subcommand**, deliberately. Axis 2
// of the parity contract says "no commands added, none removed, none renamed",
// and upstream's CLI has three commands — `new`, `render`, `create-theme` — none
// of which is `schema`. Upstream generates its `schema.json` from
// `generate_json_schema_file`, outside the CLI, and this is the equivalent
// generation path Axis 3's own parenthetical allows.
//
// Adding the subcommand would have broken Axis 2 to satisfy Axis 3.
package main

import (
	"fmt"
	"os"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
)

func main() {
	text, err := models.SchemaJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "genschema: %v\n", err)
		os.Exit(1)
	}
	// No trailing newline: upstream's file ends at the closing brace, because
	// `json.dumps` does not append one and `write_text` adds nothing
	// (spec 005 §3.4 behavior 15).
	fmt.Print(text)
}
