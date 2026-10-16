package document

import "strings"

// Key spells the place e refuses as a configuration refusal names a key: the
// member names of Pointer joined by full stops, an index in brackets after the
// array it indexes, then Member for a refusal about a member; "" is the whole
// document. A segment of ASCII digits alone is an index, and Member is always a
// name. An empty segment or Member is the member named "", so /a//b spells a..b
// and never names the key a.b.
func (e *Error) Key() string {
	var key strings.Builder
	for i, segment := range segments(e.Pointer) {
		if isIndex(segment) {
			key.WriteString("[" + segment + "]")
			continue
		}
		joinMember(&key, i > 0, segment)
	}
	switch e.Keyword {
	case requiredKeyword, additionalKeyword, namesKeyword:
		joinMember(&key, e.Pointer != "", e.Member)
	}
	return key.String()
}

// joinMember writes one member name after the key written so far, after a full
// stop where a name or an index precedes it.
func joinMember(key *strings.Builder, preceded bool, name string) {
	if preceded {
		key.WriteByte('.')
	}
	key.WriteString(name)
}

// isIndex reports whether a JSON Pointer segment is an array index, ASCII
// digits alone.
func isIndex(segment string) bool {
	return segment != "" && strings.Trim(segment, "0123456789") == ""
}
