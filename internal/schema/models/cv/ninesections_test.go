package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 003 §5.12 — all nine entry types coexist in one CV. Upstream builds one
// section per entry-type name with two entries each and asserts every section has
// two (tests/schema/models/cv/test_cv.py:13-36); this is the same document,
// validated end to end with the real registry and the real per-type validators.
//
// Two entries per section is not decoration: the first decides the section's
// type, so a second entry is what proves the rest are validated against that
// decision rather than each re-discriminated.
func TestNineSectionsOneOfEachType(t *testing.T) {
	const src = `sections:
  text:
    - A first paragraph.
    - A second paragraph.
  one_line:
    - label: Languages
      details: English (native), Spanish (fluent)
    - label: Citizenship
      details: US Citizen
  normal:
    - name: Some Project
      date: '2021'
    - name: Some Award
      start_date: 2020-09
      end_date: present
  experience:
    - company: Microsoft
      position: Software Engineer
      start_date: 2020-09
      end_date: present
    - company: Google
      position: Research Assistant
      date: Fall 2023
  education:
    - institution: Boğaziçi University
      area: Mechanical Engineering
      degree: BS
      start_date: 2015-09
      end_date: 2020-06
    - institution: MIT
      area: Computer Science
  publication:
    - title: Deep Learning for Computer Vision
      authors:
        - J. Doe
        - '***H. Tom***'
      doi: 10.48550/arXiv.2310.03138
      journal: arXiv preprint
    - title: Advances in Quantum Computing
      authors:
        - John Doe
      date: '2020'
  bullet:
    - bullet: Python, JavaScript, C++
    - bullet: Excellent communication skills
  numbered:
    - number: First publication about XYZ
    - number: Patent for ABC technology
  reversed_numbered:
    - reversed_number: Latest research paper
    - reversed_number: Recent patent application
`

	model, errs := cv.Validate(parse(t, src), []string{"cv"}, schemaerr.SourceMain, testOptions())
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}

	records := model.SectionRecords(fixtureRegistry())
	want := []entries.TypeName{
		cv.TextEntry,
		"OneLineEntry",
		"NormalEntry",
		"ExperienceEntry",
		"EducationEntry",
		"PublicationEntry",
		"BulletEntry",
		"NumberedEntry",
		"ReversedNumberedEntry",
	}

	if len(records) != len(want) {
		t.Fatalf("records = %d, want %d — one per section, in input order", len(records), len(want))
	}
	for i, entryType := range want {
		if records[i].EntryType != entryType {
			t.Errorf("section %d (%q) type = %q, want %q",
				i, records[i].Title, records[i].EntryType, entryType)
		}
		if len(records[i].Entries) != 2 {
			t.Errorf("section %d (%q) has %d entries, want 2",
				i, records[i].Title, len(records[i].Entries))
		}
	}

	// Every registered name is exercised, so this cannot silently stop covering a
	// type someone adds later.
	covered := map[entries.TypeName]bool{}
	for _, record := range records {
		covered[record.EntryType] = true
	}
	for _, name := range entries.Default().Names() {
		if !covered[name] {
			t.Errorf("no section exercises %q", name)
		}
	}
}
