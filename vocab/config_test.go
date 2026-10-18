package vocab_test

import (
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

func TestReadsRootReadsAStringWithNoOtherLanguagesPrefix(t *testing.T) {
	goOnly := []vocab.Language{vocab.Go}
	for _, tc := range []struct {
		root string
		want bool
	}{
		{"example.com/app#Main", true},
		{"go://example.com/app#Main", true},
		{"*://example.com/app#Main", true},
		{"g?://example.com/app#Main", true},
		{"ts://app/src/a.ts#main", false},
		{"js://app/src/a.js#main", false},
		{"other://x", false},
		{"://x", false},
	} {
		if got := vocab.ReadsRoot(goOnly, tc.root); got != tc.want {
			t.Errorf("ReadsRoot([go], %q) = %v, want %v", tc.root, got, tc.want)
		}
	}
	if !vocab.ReadsRoot([]vocab.Language{vocab.Go, vocab.TS}, "ts://app/src/a.ts#main") {
		t.Error("ReadsRoot([go ts], a ts:// string) = false, want true")
	}
}
