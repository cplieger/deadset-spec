package corpus

import (
	"slices"
	"testing"
)

func TestPageLanguagesFollowTheRowKindThePageSelects(t *testing.T) {
	byKind := map[string][]string{"analyzed": {"go", "ts"}, "schema": {"proto"}}
	for page, want := range map[string][]string{
		"references-go": {"go"},
		"schemas":       {"proto"},
		"bindings":      {"go", "ts"},
		"confidence":    {"go", "ts"},
	} {
		if got := pageLanguages(page, byKind); !slices.Equal(got, want) {
			t.Errorf("pageLanguages(%q) = %v, want %v", page, got, want)
		}
	}
}

func TestPerLanguageTargetKindNamesOnlyKindTakingLanguages(t *testing.T) {
	env := &environment{
		languages:           []string{"go", "ts", "proto"},
		targetKindLanguages: []string{"go", "ts"},
		targetKinds:         []string{"application", "library"},
	}
	doc := &Expectation{TargetKind: TargetKind{PerLanguage: map[string]string{"go": "application", "ts": "library"}}}
	if errs := checkVocabularies(doc, env); len(errs) != 0 {
		t.Errorf("checkVocabularies(go and ts kinds, proto row present) = %v, want none", errs)
	}
	doc.TargetKind.PerLanguage["proto"] = "library"
	if errs := checkVocabularies(doc, env); len(errs) != 1 {
		t.Errorf("checkVocabularies(a proto kind) = %v, want one refusal", errs)
	}
}
