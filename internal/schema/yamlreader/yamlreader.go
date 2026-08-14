package yamlreader

import (
	"fmt"

	"github.com/nonamecat19/rendercv-go/internal/readtext"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ReadFile reads and parses a YAML input file.
//
// **It performs no existence check and no extension check**, and that is
// upstream's behavior rather than a simplification. `read_yaml` takes a
// `pathlib.Path | str` (`yaml_reader.py:11`); only the `Path` branch checks
// existence (`:33-37`) and the extension whitelist (`:39-47`), and the render
// path's only caller (`schema/rendercv_model_builder.py:85`) passes the file's
// **contents** as a string. So neither check is reachable from the CLI:
// measured, a valid CV named `ok.txt` renders at exit 0, and a missing input
// file reaches the traceback of spec 013 behavior 40, not a message.
//
// Both messages therefore lived here where nothing could surface them and
// nothing could prove them gone. Implementing either reachably would make the
// port stricter than upstream (spec 013 §3.6 behavior 43, §4.11);
// `internal/cli/reachability_test.go` asserts neither string is in the source.
func ReadFile(path string) (*yamldoc.Node, error) {
	// `read_yaml`'s own `Path` branch reads with `read_text`
	// (`yaml_reader.py:49`), so this unreachable twin translates line endings
	// like the reachable boundary in `internal/cli` does.
	raw, err := readtext.File(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ReadString(string(raw))
}

// ReadString parses YAML content that is already in memory.
func ReadString(input string) (*yamldoc.Node, error) {
	doc, err := parse(input)
	if err != nil {
		return nil, err
	}

	// **The predicate is `yaml.load(...) is None`** (`yaml_reader.py:55-57`),
	// not "the file had no content": a document whose whole value is `null`
	// (or `~`, or `Null`) loads to `None` exactly as an empty file does, and
	// upstream reports it the same way. The port keyed on the absence of a
	// document, so an explicit null fell through to the model builder and came
	// back as a validation table instead of the `Error` panel.
	if doc == nil || doc.Kind == yamldoc.KindNull {
		return nil, &schemaerr.UserError{
			Message: "The input file is empty!",
		}
	}

	if doc.Kind == yamldoc.KindString {
		return nil, &schemaerr.InternalError{
			Message: fmt.Sprintf(
				"You probably meant to pass a path to the YAML file, but you passed as a string and RenderCV interpreted it as the contents of the YAML file. Pass the path using `pathlib.Path(%s)`.",
				input,
			),
		}
	}

	return doc, nil
}
