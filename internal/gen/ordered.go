package gen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// value is a JSON value that keeps the member order of the document it was read
// from, so a generated file lists members in the order its author wrote them.
type value struct {
	keys   []string
	fields map[string]*value
	items  []*value
	scalar json.RawMessage // a string, number, true, false or null, as written
	kind   byte            // 'o' object, 'a' array, 's' scalar
}

// parse reads one JSON document. A duplicate member is refused, as every
// Contract document refuses one.
func parse(data []byte) (*value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data after the document")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (*value, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		return parseObject(dec)
	case json.Delim('['):
		return parseArray(dec)
	}
	if _, delim := tok.(json.Delim); delim {
		return nil, fmt.Errorf("unexpected delimiter %v", tok)
	}
	raw, err := json.Marshal(tok)
	if err != nil {
		return nil, err
	}
	return &value{kind: 's', scalar: raw}, nil
}

func parseObject(dec *json.Decoder) (*value, error) {
	v := &value{kind: 'o', fields: map[string]*value{}}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		if _, dup := v.fields[key]; dup {
			return nil, fmt.Errorf("duplicate member %q", key)
		}
		child, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		v.keys = append(v.keys, key)
		v.fields[key] = child
	}
	_, err := dec.Token()
	return v, err
}

func parseArray(dec *json.Decoder) (*value, error) {
	v := &value{kind: 'a', items: []*value{}}
	for dec.More() {
		child, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		v.items = append(v.items, child)
	}
	_, err := dec.Token()
	return v, err
}

// get returns a member of an object, or nil.
func (v *value) get(key string) *value {
	if v == nil || v.kind != 'o' {
		return nil
	}
	return v.fields[key]
}

// str is a string scalar's text, or "".
func (v *value) str() string {
	var s string
	if v == nil || v.kind != 's' || json.Unmarshal(v.scalar, &s) != nil {
		return ""
	}
	return s
}

// strs is an array of strings.
func (v *value) strs() []string {
	if v == nil {
		return nil
	}
	out := make([]string, 0, len(v.items))
	for _, it := range v.items {
		out = append(out, it.str())
	}
	return out
}

// set replaces or appends a member.
func (v *value) set(key string, child *value) {
	if _, ok := v.fields[key]; !ok {
		v.keys = append(v.keys, key)
	}
	v.fields[key] = child
}

func strValue(s string) *value {
	raw, _ := json.Marshal(s)
	return &value{kind: 's', scalar: raw}
}

func strsValue(ss []string) *value {
	v := &value{kind: 'a', items: []*value{}}
	for _, s := range ss {
		v.items = append(v.items, strValue(s))
	}
	return v
}

// objValue builds an object from alternating member names and values.
func objValue(pairs ...any) *value {
	v := &value{kind: 'o', fields: map[string]*value{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		name, _ := pairs[i].(string)
		child, _ := pairs[i+1].(*value)
		v.set(name, child)
	}
	return v
}

func arrValue(items ...*value) *value {
	return &value{kind: 'a', items: append([]*value{}, items...)}
}

// encode writes v in the layout every generated Contract file shares: two-space
// indentation, an array of scalars on one line, every object expanded, a final
// newline, and no escaping of <, > and & so text reads as written.
func encode(v *value) []byte {
	var b strings.Builder
	writeValue(&b, v, 0)
	b.WriteByte('\n')
	return []byte(b.String())
}

func writeValue(b *strings.Builder, v *value, depth int) {
	switch v.kind {
	case 's':
		b.Write(unescaped(v.scalar))
	case 'a':
		writeArray(b, v, depth)
	case 'o':
		writeObject(b, v, depth)
	}
}

func writeArray(b *strings.Builder, v *value, depth int) {
	pad := strings.Repeat("  ", depth)
	switch {
	case len(v.items) == 0:
		b.WriteString("[]")
	case allScalars(v.items):
		b.WriteByte('[')
		for i, it := range v.items {
			if i > 0 {
				b.WriteString(", ")
			}
			b.Write(unescaped(it.scalar))
		}
		b.WriteByte(']')
	default:
		b.WriteString("[\n")
		for i, it := range v.items {
			b.WriteString(pad + "  ")
			writeValue(b, it, depth+1)
			if i < len(v.items)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(pad + "]")
	}
}

func writeObject(b *strings.Builder, v *value, depth int) {
	if len(v.keys) == 0 {
		b.WriteString("{}")
		return
	}
	pad := strings.Repeat("  ", depth)
	b.WriteString("{\n")
	for i, k := range v.keys {
		b.WriteString(pad + "  ")
		b.Write(unescaped(mustMarshal(k)))
		b.WriteString(": ")
		writeValue(b, v.fields[k], depth+1)
		if i < len(v.keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(pad + "}")
}

func allScalars(items []*value) bool {
	for _, it := range items {
		if it.kind != 's' {
			return false
		}
	}
	return true
}

func mustMarshal(s string) []byte {
	raw, _ := json.Marshal(s)
	return raw
}

// unescaped undoes encoding/json's HTML escaping of a scalar it wrote.
func unescaped(raw []byte) []byte {
	r := strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&")
	return []byte(r.Replace(string(raw)))
}
