package document

import "strings"

// Key spells the place e refuses in v, the value e was found in, as a
// configuration refusal names a key: under, the key of the place v sits at or
// "" for a whole document, then each member name of Pointer after a full stop
// and each index of an array in brackets, then Member for a refusal about a
// member. Whether a segment is an index is read from v, so the member named 1
// spells .1 and never [1]. An empty segment or Member is the member named "",
// so /a//b spells a..b and never names the key a.b.
func (e *Error) Key(under string, v any) string {
	var key strings.Builder
	key.WriteString(under)
	started := under != ""
	for _, segment := range segments(e.Pointer) {
		if _, isArray := v.([]any); isArray {
			key.WriteString("[" + segment + "]")
		} else {
			joinMember(&key, started, segment)
		}
		started = true
		v = valueAt(v, segment)
	}
	switch e.Keyword {
	case requiredKeyword, additionalKeyword, namesKeyword:
		joinMember(&key, started, e.Member)
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
