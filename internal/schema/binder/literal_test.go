package binder_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
)

// pydantic's `literal_error` text, measured on `PageSize` and on a two-member
// union: `or` before the last member, no serial comma.
func TestLiteralMessage(t *testing.T) {
	tests := []struct {
		name    string
		members []string
		want    string
	}{
		{
			name:    "four members",
			members: []string{"a4", "a5", "us-letter", "us-executive"},
			want:    "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
		},
		{
			// Two members take the `or` branch with no comma at all, which the
			// four-member case cannot distinguish from a serial comma bug.
			name:    "two members",
			members: []string{"left", "right"},
			want:    "Input should be 'left' or 'right'",
		},
		{
			name:    "one member",
			members: []string{"present"},
			want:    "Input should be 'present'",
		},
		{
			name:    "none",
			members: nil,
			want:    "fallback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := binder.LiteralMessage(tc.members, "fallback"); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}
