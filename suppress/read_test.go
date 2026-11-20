package suppress

import (
	"errors"
	"testing"
)

// Each entry is read at the brace that opens it, its column in UTF-16 code
// units, with an absent member left absent.
func TestReadIgnoreReadsEachEntryAtItsOpeningBrace(t *testing.T) {
	t.Parallel()

	data := []byte("{\n  \"ignore\": [\n    {\"code\": \"DS1002\", \"symbol\": \"go://x#f\", \"path\": \"a.go\", \"reason\": \"\U0001F600\"}, " +
		"{\"code\": \"DS1002\", \"symbol\": \"ts://@x/a.ts#g\", \"reason\": \"kept\"}\n  ]\n}\n")
	entries, err := ReadIgnore(data)
	if err != nil {
		t.Fatalf("ReadIgnore = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadIgnore read %d entries, want 2", len(entries))
	}
	if got := entries[0].Site; got != (Site{Path: IgnoreFileName, Line: 3, Column: 5}) {
		t.Errorf("the first entry is at %+v, want %s:3:5", got, IgnoreFileName)
	}
	if got := entries[1].Site; got != (Site{Path: IgnoreFileName, Line: 3, Column: 79}) {
		t.Errorf("the second entry is at %+v, want %s:3:79: a surrogate pair counts two units before it", got, IgnoreFileName)
	}
	if entries[1].Path != nil {
		t.Errorf("the second entry's path = %q, want it absent", *entries[1].Path)
	}
}

// An absent document holds no entry.
func TestReadOfAnAbsentDocumentHoldsNothing(t *testing.T) {
	t.Parallel()

	if entries, err := ReadBaseline(nil); err != nil || entries != nil {
		t.Errorf("ReadBaseline(nil) = %v, %v, want no entry and no error", entries, err)
	}
}

// Every defect a document can carry before any finding exists ends the run.
func TestReadRefusesAMalformedDocument(t *testing.T) {
	t.Parallel()

	for name, data := range map[string]string{
		"not JSON":                   `{"ignore": [`,
		"no array":                   `{"description": "x"}`,
		"an undeclared member":       `{"ignore": [], "extra": 1}`,
		"a member written twice":     `{"ignore": [], "ignore": []}`,
		"an entry member twice":      `{"ignore": [{"code": "DS1002", "code": "DS1002", "symbol": "go://x#f"}]}`,
		"an undeclared entry key":    `{"ignore": [{"code": "DS1002", "symbol": "go://x#f", "line": 1}]}`,
		"no code":                    `{"ignore": [{"symbol": "go://x#f"}]}`,
		"a code outside its form":    `{"ignore": [{"code": "DS12", "symbol": "go://x#f"}]}`,
		"no scheme":                  `{"ignore": [{"code": "DS1002", "symbol": "x#f"}]}`,
		"a space outside quotes":     `{"ignore": [{"code": "DS1002", "symbol": "rs://x#a b"}]}`,
		"a space in a scope":         `{"ignore": [{"code": "DS1002", "symbol": "ts://./my file.ts#f"}]}`,
		"lowercase escape digits":    `{"ignore": [{"code": "DS1002", "symbol": "ts://./a%2f.ts#f"}]}`,
		"an unterminated quote":      `{"ignore": [{"code": "DS1002", "symbol": "rs://x#H.'a b"}]}`,
		"a wildcard":                 `{"ignore": [{"code": "DS1002", "symbol": "go://x#*"}]}`,
		"no fragment separator":      `{"ignore": [{"code": "DS1002", "symbol": "rs://x"}]}`,
		"a path climbing out":        `{"ignore": [{"code": "DS1002", "symbol": "go://x#f", "path": "../a.go"}]}`,
		"content after the object":   `{"ignore": []} []`,
		"an entry that is no object": `{"ignore": ["DS1002"]}`,
		"an object after a string":   `{"ignore": ["DS1002", {"code": "DS1002", "symbol": "go://x#f"}]}`,
	} {
		if _, err := ReadIgnore([]byte(data)); !errors.Is(err, ErrMalformed) {
			t.Errorf("ReadIgnore(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

// A record of any scheme is read when it takes the shape every document checks,
// whitespace inside a quoted component and an escape in a path included.
func TestReadAcceptsEveryReferenceInShape(t *testing.T) {
	t.Parallel()

	for _, symbol := range []string{
		"ts://./src/h.ts#Headers.'content type'",
		"ts://./src/my%20file.ts#unused",
		"go://example.com/app/c%2B%2B#Run",
		"rs://crate/a%23b#T.'x y'",
		"go://example.com/app#Best[T]",
	} {
		data := `{"baseline": [{"code": "DS1002", "symbol": "` + symbol + `", "reason": "kept"}]}`
		entries, err := ReadBaseline([]byte(data))
		if err != nil || len(entries) != 1 || entries[0].Symbol != symbol {
			t.Errorf("ReadBaseline(%s) = %v, %v, want the one row naming it", symbol, entries, err)
		}
	}
}
