package design

// The six `Literal` unions of `classic_theme.py:10-24`, each in **declaration**
// order.
//
// Declaration order is contractual twice over: it is the order pydantic writes
// into the schema's `enum` (spec 005 §6 rule 6) and the order
// `binder.LiteralMessage` reads out into `literal_error`. Sorting any of them
// produces a schema that looks tidy and does not match.
//
// They are written here rather than generated, unlike the option tree: six short
// lists that acceptance criteria name individually, which is the same split
// iteration 4 made between the thirteen-row error dictionary and the 210-string
// locale catalogs.
var (
	// Bullets is `Bullet` (`:10`). Eight of the nine are non-ASCII and reach the
	// schema literally, which spec 005 §3.4 covers in the encoder.
	Bullets = []string{"●", "•", "◦", "-", "◆", "★", "■", "—", "○"}

	// BodyAlignments is `BodyAlignment` (`:11`). **The alias is declared and
	// never used** — `typography.alignment` (`:241`) spells the literal out — so
	// it has no `$defs` entry and these members reach the schema inlined.
	BodyAlignments = []string{"left", "justified", "justified-with-no-hyphenation"}

	// Alignments is `Alignment` (`:12`), reached by `header.alignment` and by
	// `typography.date_and_location_column_alignment`.
	Alignments = []string{"left", "center", "right"}

	// SectionTitleTypes is `SectionTitleType` (`:13-22`). **Eight members, not
	// the six spec 006 §2 behavior 5's table claimed** — the table's count was
	// wrong and the line range it cited was right; corrected in the spec.
	SectionTitleTypes = []string{
		"with_partial_line",
		"with_full_line",
		"without_line",
		"moderncv",
		"centered_without_line",
		"centered_with_partial_line",
		"centered_with_centered_partial_line",
		"centered_with_full_line",
	}

	// PhoneNumberFormats is `PhoneNumberFormatType` (`:23`). `E164` is the one
	// member that is not lowercase.
	PhoneNumberFormats = []string{"national", "international", "E164"}

	// PageSizes is `PageSize` (`:24`).
	PageSizes = []string{"a4", "a5", "us-letter", "us-executive"}

	// PhotoPositions is `header.photo_position`'s inline `Literal["left",
	// "right"]` (`:380`). Anonymous, so inlined for the same reason
	// `BodyAlignments` is — the distinction matters to the schema and not to
	// validation.
	PhotoPositions = []string{"left", "right"}
)
