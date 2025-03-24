package cv_test

import (
	"reflect"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
)

func TestSocialNetworkNamesOrder(t *testing.T) {
	want := []cv.SocialNetworkName{
		"LinkedIn",
		"GitHub",
		"GitLab",
		"IMDB",
		"Instagram",
		"ORCID",
		"Mastodon",
		"StackOverflow",
		"ResearchGate",
		"YouTube",
		"Google Scholar",
		"Telegram",
		"WhatsApp",
		"Leetcode",
		"X",
		"Bluesky",
		"Reddit",
	}

	if len(cv.SocialNetworkNames) != 17 {
		t.Fatalf("SocialNetworkNames has %d entries, want 17", len(cv.SocialNetworkNames))
	}

	for i, name := range want {
		t.Run(string(name), func(t *testing.T) {
			if cv.SocialNetworkNames[i] != name {
				t.Errorf("SocialNetworkNames[%d] = %q, want %q", i, cv.SocialNetworkNames[i], name)
			}
		})
	}
}

func TestSocialNetworkRequiredFields(t *testing.T) {
	sn := cv.SocialNetwork{}
	typ := reflect.TypeOf(sn)

	if typ.NumField() != 2 {
		t.Fatalf("SocialNetwork has %d fields, want 2", typ.NumField())
	}

	tests := []struct {
		index int
		name  string
		kind  reflect.Kind
	}{
		{0, "Network", reflect.String},
		{1, "Username", reflect.String},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := typ.Field(tt.index)
			if field.Name != tt.name {
				t.Errorf("field %d = %q, want %q", tt.index, field.Name, tt.name)
			}
			if field.Type.Kind() != tt.kind {
				t.Errorf("field %q kind = %v, want %v", tt.name, field.Type.Kind(), tt.kind)
			}
		})
	}
}

func TestSocialNetworkFieldsRejectsExtraKeys(t *testing.T) {
	want := []string{"network", "username"}

	if len(cv.SocialNetworkFields) != len(want) {
		t.Fatalf("SocialNetworkFields = %v, want %v", cv.SocialNetworkFields, want)
	}
	for i, key := range want {
		if cv.SocialNetworkFields[i] != key {
			t.Errorf("SocialNetworkFields[%d] = %q, want %q", i, cv.SocialNetworkFields[i], key)
		}
	}

	tests := []struct {
		key   string
		allow bool
	}{
		{"network", true},
		{"username", true},
		{"url", false},
		{"icon", false},
		{"extra_field", false},
	}

	allowed := make(map[string]bool, len(cv.SocialNetworkFields))
	for _, f := range cv.SocialNetworkFields {
		allowed[f] = true
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if allowed[tt.key] != tt.allow {
				t.Errorf("allowed[%q] = %v, want %v", tt.key, allowed[tt.key], tt.allow)
			}
		})
	}
}
