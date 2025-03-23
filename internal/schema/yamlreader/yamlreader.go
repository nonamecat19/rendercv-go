package yamlreader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

var acceptedExtensions = map[string]bool{
	".yaml":  true,
	".yml":   true,
	".json":  true,
	".json5": true,
}

func ReadFile(path string) (*yamldoc.Node, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, &schemaerr.UserError{
			Message: fmt.Sprintf("The input file `%s` doesn't exist!", path),
		}
	}

	ext := filepath.Ext(path)
	if !acceptedExtensions[ext] {
		accepted := []string{".yaml", ".yml", ".json", ".json5"}
		return nil, &schemaerr.UserError{
			Message: fmt.Sprintf(
				"The input file should have one of the following extensions: %s. The input file is %s.",
				strings.Join(accepted, ", "),
				filepath.Base(path),
			),
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ReadString(string(raw))
}

func ReadString(input string) (*yamldoc.Node, error) {
	doc, err := parse(input)
	if err != nil {
		return nil, err
	}

	if doc == nil {
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
