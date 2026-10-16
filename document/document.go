package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cplieger/jsoncap/v2"
)

// ErrMalformed reports a document that is not one strict JSON value: a syntax
// error, data after the value, or an object naming one member twice.
var ErrMalformed = errors.New("the document is not one strict JSON value")

// Error is one value a document is refused for: the JSON Pointer (RFC 6901) of
// the value, the empty string for the whole document, the keyword or the rule
// that refuses it, and why. Member is the name of the member a required,
// additionalProperties or propertyNames refusal is about, the object being at
// Pointer; it is empty for every other keyword.
type Error struct {
	// oneOf is the node whose oneOf no alternative matches, for a oneOf
	// refusal, so [Schema.FirstRefusal] explains that oneOf and not another
	// one checked at the same place.
	oneOf   *node
	Pointer string
	Keyword string
	Reason  string
	Member  string
}

// Error renders the refusal as one line.
func (e *Error) Error() string {
	at := e.Pointer
	if at == "" {
		at = "the document"
	}
	return fmt.Sprintf("%s: %s: %s", at, e.Keyword, e.Reason)
}

// CheckValue returns what s refuses in v, a value decoded with its numbers
// kept as [json.Number]; none means v is an instance.
func (s *Schema) CheckValue(v any) []Error {
	var found []Error
	s.root.check(v, "", &found)
	return found
}

// Decode reads data as one strict JSON value with its numbers kept exact, the
// value [Schema.CheckValue] reads. A refusal wraps [ErrMalformed].
func Decode(data []byte) (any, error) {
	if err := jsoncap.Preflight(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	v, err := decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return v, nil
}

// Unmarshal reads data as one strict JSON instance of s into v. A document
// that is not strict JSON is refused wrapping [ErrMalformed], and one s
// refuses as the [*Error] [Schema.FirstRefusal] names.
func (s *Schema) Unmarshal(data []byte, v any) error {
	value, err := Decode(data)
	if err != nil {
		return err
	}
	if refused := s.FirstRefusal(value); refused != nil {
		return refused
	}
	return json.Unmarshal(data, v)
}

// Escape spells one member name as a JSON Pointer token.
func Escape(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}
