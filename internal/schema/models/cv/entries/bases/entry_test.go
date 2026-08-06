package bases_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
)

func TestEntryTypeInSnakeCase(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"OneLineEntry", "one_line_entry"},
		{"NormalEntry", "normal_entry"},
		{"ExperienceEntry", "experience_entry"},
		{"EducationEntry", "education_entry"},
		{"PublicationEntry", "publication_entry"},
		{"BulletEntry", "bullet_entry"},
		{"NumberedEntry", "numbered_entry"},
		{"ReversedNumberedEntry", "reversed_numbered_entry"},
		{"TextEntry", "text_entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bases.EntryTypeInSnakeCase(tt.name)
			if got != tt.want {
				t.Errorf("EntryTypeInSnakeCase(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
