package cv_test

import (
	"reflect"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
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

// Spec §3.80 — both fields are required and unknown keys are rejected.
func TestValidateSocialNetwork(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCodes []schemaerr.Code
	}{
		{name: "both fields", input: "network: GitHub\nusername: johndoe\n"},
		{name: "missing username", input: "network: GitHub\n", wantCodes: []schemaerr.Code{binder.CodeMissing}},
		{name: "missing network", input: "username: johndoe\n", wantCodes: []schemaerr.Code{binder.CodeMissing}},
		{name: "both missing", input: "{}\n", wantCodes: []schemaerr.Code{binder.CodeMissing, binder.CodeMissing}},
		{
			name:      "unknown key",
			input:     "network: GitHub\nusername: johndoe\nextra: 1\n",
			wantCodes: []schemaerr.Code{binder.CodeExtraForbidden},
		},
		{
			name:      "unsupported network",
			input:     "network: Friendster\nusername: johndoe\n",
			wantCodes: []schemaerr.Code{cv.CodeLiteral},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, err := yamlreader.ReadString(tc.input)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			_, errs := cv.ValidateSocialNetwork(node, []string{"cv", "social_networks", "0"}, schemaerr.SourceMain)
			if len(errs) != len(tc.wantCodes) {
				t.Fatalf("errs = %+v, want %d", errs, len(tc.wantCodes))
			}
			for i, code := range tc.wantCodes {
				if errs[i].Code != code {
					t.Errorf("errs[%d].Code = %q, want %q", i, errs[i].Code, code)
				}
			}
		})
	}
}

// validUsernames gives each network a username its own rule accepts. Nine
// networks have no rule and take anything; the rest need a real value, and
// listing them here keeps the "every name is accepted" test about the *name*
// rather than about the username.
var validUsernames = map[cv.SocialNetworkName]string{
	"Mastodon": "@johndoe@mastodon.social",
}

// Every one of the seventeen names is accepted.
func TestAllSocialNetworkNamesAccepted(t *testing.T) {
	for _, name := range cv.SocialNetworkNames {
		username := "johndoe"
		if special, ok := validUsernames[name]; ok {
			username = special
		}
		node, err := yamlreader.ReadString("network: \"" + string(name) + "\"\nusername: \"" + username + "\"\n")
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}
		model, errs := cv.ValidateSocialNetwork(node, nil, schemaerr.SourceMain)
		if len(errs) != 0 {
			t.Errorf("%s: errs = %+v, want none", name, errs)
		}
		if model.Network != name {
			t.Errorf("network = %q, want %q", model.Network, name)
		}
	}
}
