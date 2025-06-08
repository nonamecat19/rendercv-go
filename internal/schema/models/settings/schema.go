package settings

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// The three `$defs` the `settings` block owns: `Settings`, `RenderCommand` and
// `PlannedPathRelativeToInput`.
//
// **They belong to no iteration's spec.** Spec 005 §1 counted them with
// iteration 7's locale and iteration 7 turned out not to own them, which left
// three entries between the port and a green Axis 3. They land with iteration 6
// (tasks §T15) rather than waiting for iteration 12's CLI, because the schema is
// a projection of the model and this part of the model is nine defaults and two
// paths — not the render pipeline they describe.
//
// The **behavior** those defaults drive is still iteration 12's: nothing here
// reads a `dont_generate_*` flag or resolves an output path. This is the schema
// projection only, and the differential is what gates it.

// pathPlaceholders is the block every output-path description ends with. It is
// one constant because upstream writes it once and interpolates it five times,
// and five copies in the Go source would be five chances to differ by a
// character the differential would report as five failures.
const pathPlaceholders = "\n\nThe following placeholders can be used:\n\n" +
	"- OUTPUT_FOLDER: The output folder path (e.g., rendercv_output)\n" +
	"- MONTH_NAME: Full name of the month (e.g., January)\n" +
	"- MONTH_ABBREVIATION: Abbreviation of the month (e.g., Jan)\n" +
	"- MONTH: Month as a number (e.g., 1)\n" +
	"- MONTH_IN_TWO_DIGITS: Month as a number in two digits (e.g., 01)\n" +
	"- DAY: Day of the month (e.g., 5)\n" +
	"- DAY_IN_TWO_DIGITS: Day of the month in two digits (e.g., 05)\n" +
	"- YEAR: Year as a number (e.g., 2024)\n" +
	"- YEAR_IN_TWO_DIGITS: Year as a number in two digits (e.g., 24)\n" +
	"- NAME: The name of the CV owner (e.g., John Doe)\n" +
	"- NAME_IN_SNAKE_CASE: The name of the CV owner in snake case (e.g., John_Doe)\n" +
	"- NAME_IN_LOWER_SNAKE_CASE: The name of the CV owner in lower snake case (e.g., john_doe)\n" +
	"- NAME_IN_UPPER_SNAKE_CASE: The name of the CV owner in upper snake case (e.g., JOHN_DOE)\n" +
	"- NAME_IN_KEBAB_CASE: The name of the CV owner in kebab case (e.g., John-Doe)\n" +
	"- NAME_IN_LOWER_KEBAB_CASE: The name of the CV owner in lower kebab case (e.g., john-doe)\n" +
	"- NAME_IN_UPPER_KEBAB_CASE: The name of the CV owner in upper kebab case (e.g., JOHN-DOE)\n"

// datePlaceholders is the shorter list `pdf_title` carries.
const datePlaceholders = "Available placeholders:\n" +
	"- `NAME`: The CV owner's name from `cv.name`\n" +
	"- `CURRENT_DATE`: Formatted date based on `design.templates.single_date`\n" +
	"- `MONTH_NAME`: Full month name (e.g., January)\n" +
	"- `MONTH_ABBREVIATION`: Abbreviated month name (e.g., Jan)\n" +
	"- `MONTH`: Month number (e.g., 1)\n" +
	"- `MONTH_IN_TWO_DIGITS`: Zero-padded month (e.g., 01)\n" +
	"- `DAY`: Day of the month (e.g., 5)\n" +
	"- `DAY_IN_TWO_DIGITS`: Zero-padded day (e.g., 05)\n" +
	"- `YEAR`: Full year (e.g., 2025)\n" +
	"- `YEAR_IN_TWO_DIGITS`: Two-digit year (e.g., 25)\n"

// SchemaDefs is the three entries.
func SchemaDefs() map[string]*jsonschema.Object {
	return map[string]*jsonschema.Object{
		"Settings":                   Schema(),
		"RenderCommand":              RenderCommandSchema(),
		"PlannedPathRelativeToInput": PlannedPathSchema(),
	}
}

// PlannedPathSchema is a path that need not exist yet, unlike `cv`'s
// `ExistingPathRelativeToInput`. Their schemas are identical — `format: path`
// carries no validation weight — and they are two entries because they are two
// Python types.
func PlannedPathSchema() *jsonschema.Object {
	return jsonschema.NewObject().
		Set("format", "path").
		Set("type", "string")
}

// Schema is `Settings`.
//
// `current_date`'s union has **no null arm and a `const` arm**: it is
// `datetime.date | Literal["today"]`, so the two members are a date-formatted
// string and the literal, and the title is the explicit `Date` rather than
// `Current Date`.
func Schema() *jsonschema.Object {
	properties := jsonschema.NewObject().
		Set("current_date", jsonschema.NewObject().
			Set("anyOf", []any{
				jsonschema.NewObject().Set("format", "date").Set("type", "string"),
				jsonschema.NewObject().Set("const", "today").Set("type", "string"),
			}).
			Set("default", "today").
			Set("description", `The date to use as "current date" for filenames, the "last updated"`+
				" label, and time span calculations. Defaults to the actual current date.").
			Set("title", "Date").
			Sort()).
		Set("render_command", jsonschema.Ref("RenderCommand").
			Set("description", "Settings for the `render` command. These correspond to"+
				" command-line arguments. CLI arguments take precedence over these settings.").
			Set("title", "Render Command Settings").
			Sort()).
		Set("bold_keywords", jsonschema.NewObject().
			Set("default", []any{}).
			Set("description", "Keywords to automatically bold in the output.").
			Set("items", jsonschema.NewObject().Set("type", "string")).
			Set("title", "Bold Keywords").
			Set("type", "array").
			Sort()).
		Set("pdf_title", stringField("pdf_title",
			"Title metadata for the PDF document. This appears in browser tabs and"+
				" PDF readers. "+datePlaceholders+
				"\nThe default value is `NAME - CV`.",
			"NAME - CV", "PDF Title"))

	return jsonschema.NewObject().
		Set("additionalProperties", false).
		Set("properties", properties).
		Set("title", "Settings").
		Set("type", "object").
		Sort()
}

// RenderCommandSchema is `RenderCommand`, the `render` flags as settings.
//
// **`render_command.design` and `.locale` are `ExistingPathRelativeToInput`,
// nullable, defaulting to null**, while the five output paths are
// `PlannedPathRelativeToInput` with string defaults — the distinction between a
// file that must already be there and one that will be written.
//
// `markdown_path` carries an explicit `title` and the other four output paths do
// not. That is upstream's inconsistency, not a rule: a bare `$ref` normally has
// its title omitted (spec 005 §3.2), and an explicit `Field(title=…)` survives
// the omission. Deriving it would produce four titles or none.
func RenderCommandSchema() *jsonschema.Object {
	properties := jsonschema.NewObject().
		Set("output_folder", jsonschema.Ref("PlannedPathRelativeToInput").
			Set("default", "rendercv_output").
			Set("description", "Base output folder for all generated files. The default"+
				" value is `rendercv_output`. Referenced as `OUTPUT_FOLDER` in output"+
				" path defaults.\n\n").
			Sort()).
		Set("design", overlayPath("design")).
		Set("locale", overlayPath("locale")).
		Set("typst_path", outputPath("the Typst file", "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ", "")).
		Set("pdf_path", outputPath("the PDF file", "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.pdf", "")).
		Set("markdown_path", outputPath("the Markdown file",
			"OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.md", "Markdown Path")).
		Set("html_path", outputPath("the HTML file", "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.html", "")).
		Set("png_path", outputPath("PNG files", "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.png", "")).
		Set("dont_generate_markdown", boolField("dont_generate_markdown",
			"Skip Markdown generation. This also disables HTML generation."+
				" The default value is `false`.", "Don't Generate Markdown")).
		Set("dont_generate_html", boolField("dont_generate_html",
			"Skip HTML generation. The default value is `false`.", "Don't Generate HTML")).
		Set("dont_generate_typst", boolField("dont_generate_typst",
			"Skip Typst generation. This also disables PDF and PNG generation."+
				" The default value is `false`.", "Don't Generate Typst")).
		Set("dont_generate_pdf", boolField("dont_generate_pdf",
			"Skip PDF generation. The default value is `false`.", "Don't Generate PDF")).
		Set("dont_generate_png", boolField("dont_generate_png",
			"Skip PNG generation. The default value is `false`.", "Don't Generate PNG"))

	return jsonschema.NewObject().
		Set("additionalProperties", false).
		Set("properties", properties).
		Set("title", "RenderCommand").
		Set("type", "object").
		Sort()
}

// overlayPath is `design` or `locale`: a nullable path to a YAML file carrying
// that block, which is the overlay mechanism spec 002 already models.
func overlayPath(block string) *jsonschema.Object {
	return jsonschema.NewObject().
		Set("anyOf", []any{
			jsonschema.Ref("ExistingPathRelativeToInput"),
			jsonschema.NewObject().Set("type", "null"),
		}).
		Set("default", nil).
		Set("description", "Path to a YAML file containing the `"+block+"` field.").
		Sort()
}

func outputPath(what, value, title string) *jsonschema.Object {
	object := jsonschema.Ref("PlannedPathRelativeToInput").
		Set("default", value).
		// `what` carries its own article: four of the five read "the Typst
		// file" and `png_path` reads "PNG files", with no article and a plural.
		// Interpolating a fixed "the " would be one wrong description of five.
		Set("description", "Output path for "+what+", relative to the input YAML"+
			" file. The default value is `"+value+"`."+pathPlaceholders)
	if title != "" {
		object.Set("title", title)
	}
	return object.Sort()
}

func stringField(name, description, value, title string) *jsonschema.Object {
	return jsonschema.NewObject().
		Set("default", value).
		Set("description", description).
		Set("title", titleOr(name, title)).
		Set("type", "string").
		Sort()
}

func boolField(name, description, title string) *jsonschema.Object {
	return jsonschema.NewObject().
		Set("default", false).
		Set("description", description).
		Set("title", titleOr(name, title)).
		Set("type", "boolean").
		Sort()
}

func titleOr(name, title string) string {
	if title != "" {
		return title
	}
	return jsonschema.TitleFor(name)
}
