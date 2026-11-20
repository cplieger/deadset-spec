// Package segment spells and reads the path segments of a stable symbol
// reference, as contract/grammar/symbol-ref.md's Path segments section states.
// It imports no other package of the module.
package segment

import (
	"strings"
	"unicode/utf8"
)

// Set names the bytes one kind of path segment spells as themselves,
// as symbol-ref.md's Path segments section states.
type Set uint8

const (
	// GoElement is one element of a Go import path: ASCII letters, digits, ., _, ~ and -.
	GoElement Set = iota
	// SourcePath is one segment of a TypeScript source path: every byte of a
	// well-formed UTF-8 sequence but those of a character of the unsafe set, of
	// rule 5's whitespace, and of #, %, *, ? and /.
	SourcePath
	// FileOperand is one segment of a file operand before :file: the SourcePath
	// set less ', [, ], \ and :, which open a quoted component, a computed key or
	// a selector in a fragment.
	FileOperand
)

const upperHex = "0123456789ABCDEF"

// allows reports whether the set spells the ASCII byte b as itself.
func (set Set) allows(b byte) bool {
	switch {
	case 'A' <= b && b <= 'Z', 'a' <= b && b <= 'z', '0' <= b && b <= '9':
		return true
	case set == GoElement:
		return strings.IndexByte("._~-", b) >= 0
	case b <= 0x20 || b == 0x7F || strings.IndexByte("#%*?/", b) >= 0:
		return false
	case set == FileOperand:
		return strings.IndexByte(`'[]\:`, b) < 0
	}
	return true
}

// Unsafe reports whether r is in symbol-ref.md's unsafe set: a control
// character, a Bidi_Control character, or the line or paragraph separator.
func Unsafe(r rune) bool {
	for _, span := range unsafeRanges {
		if span[0] <= r && r <= span[1] {
			return true
		}
	}
	return false
}

// spellsRune reports whether a set other than GoElement spells the non-ASCII
// rune r as itself: every rune but one of the unsafe set and the whitespace of
// symbol-ref.md's rule 5, Unicode's White_Space property and U+FEFF.
func spellsRune(r rune) bool {
	switch {
	case r <= 0xA0, 0x2000 <= r && r <= 0x200A, Unsafe(r):
		return false
	}
	switch r {
	case 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return false
	}
	return true
}

// Spell is the canonical spelling of one path segment's bytes: a byte of
// the set as itself, every other byte, and every byte of a sequence that is not
// well-formed UTF-8, as % and two uppercase hexadecimal digits (RFC 3986,
// section 2.1). The spelling is always well-formed UTF-8.
func Spell(set Set, segment string) string {
	var b strings.Builder
	for i := 0; i < len(segment); {
		size, kept := set.keeps(segment[i:])
		if kept {
			b.WriteString(segment[i : i+size])
		} else {
			for j := range size {
				escape(&b, segment[i+j])
			}
		}
		i += size
	}
	return b.String()
}

// keeps is the length of the character or lone byte rest opens with, and
// whether the set spells it as itself.
func (set Set) keeps(rest string) (size int, kept bool) {
	if c := rest[0]; c < utf8.RuneSelf {
		return 1, set.allows(c)
	}
	r, size := utf8.DecodeRuneInString(rest)
	return size, set != GoElement && (r != utf8.RuneError || size > 1) && spellsRune(r)
}

func escape(b *strings.Builder, c byte) {
	b.WriteByte('%')
	b.WriteByte(upperHex[c>>4])
	b.WriteByte(upperHex[c&0x0F])
}

// SpellPath spells each /-separated segment of path by the set and keeps the
// separators.
func SpellPath(set Set, path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = Spell(set, segment)
	}
	return strings.Join(segments, "/")
}

// Read decodes one spelled segment, each % and its two digits to the
// byte they name. canonical is false where the spelling is empty, holds a %
// that opens no escape with uppercase digits, decodes to a /, which no segment
// holds, decodes to . or .., which name no file and would lead a reader that
// joins the decoded segments out of its directory, or is not what Spell writes
// for the decoded bytes: such a spelling names no file, and matching, which
// compares spellings as bytes, never meets it.
func Read(set Set, spelled string) (segment string, canonical bool) {
	if spelled == "" {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(spelled); i++ {
		if spelled[i] != '%' {
			b.WriteByte(spelled[i])
			continue
		}
		if i+2 >= len(spelled) {
			return "", false
		}
		hi, hiOK := upperHexDigit(spelled[i+1])
		lo, loOK := upperHexDigit(spelled[i+2])
		if !hiOK || !loOK {
			return "", false
		}
		b.WriteByte(hi<<4 | lo)
		i += 2
	}
	segment = b.String()
	if segment == "." || segment == ".." {
		return "", false
	}
	return segment, !strings.Contains(segment, "/") && Spell(set, segment) == spelled
}

// upperHexDigit is the value of c as an uppercase hexadecimal digit, the only
// digits an escape is spelled with.
func upperHexDigit(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
