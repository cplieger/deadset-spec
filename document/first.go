package document

import (
	"strconv"
	"strings"
)

// FirstRefusal returns the refusal of v a reader fixes first, nil for an
// instance of s: an object's refusal before any inside it, at one place an
// undeclared member, then a missing one, then any other, else the order of
// [Schema.CheckValue]. A oneOf no alternative matches is explained by the first
// refusal of the alternative v was written for: the one whose type admits it,
// or of several, the one declaring the most of an object's members, then the
// fewest. Where two remain, the refusal is the oneOf's own.
func (s *Schema) FirstRefusal(v any) *Error {
	return s.root.first(v, "")
}

// first is [Schema.FirstRefusal] of the value v at the place at.
func (n *node) first(v any, at string) *Error {
	var found []Error
	n.check(v, at, &found)
	if len(found) == 0 {
		return nil
	}
	first := found[0]
	for _, other := range found[1:] {
		if before(&other, &first) {
			first = other
		}
	}
	if first.Keyword != oneOfKeyword {
		return &first
	}
	held, holders := v, []*node{n}
	for _, segment := range segments(strings.TrimPrefix(first.Pointer, at)) {
		held, holders = below(holders, held, segment)
	}
	if inner := explainOneOf(holders, held, first.Pointer); inner != nil {
		return inner
	}
	return &first
}

// explainOneOf returns the first refusal of the alternative held was written
// for, under a oneOf no alternative matches among the nodes holders apply at
// the place at, or nil where none explains it.
func explainOneOf(holders []*node, held any, at string) *Error {
	for _, holder := range holders {
		for _, applied := range holder.applied(held) {
			if applied.oneOf == nil || countMatching(applied.oneOf, held, at) > 0 {
				continue
			}
			if alternative := closest(applied.oneOf, held); alternative != nil {
				if inner := alternative.first(held, at); inner != nil {
					return inner
				}
			}
		}
	}
	return nil
}

// before reports whether a reader fixes a before b.
func before(a, b *Error) bool {
	if a.Pointer == b.Pointer {
		return rank(a) < rank(b)
	}
	return strings.HasPrefix(b.Pointer, a.Pointer+"/")
}

// rank orders the refusals of one place: an undeclared member, a missing one,
// then any other.
func rank(e *Error) int {
	switch e.Keyword {
	case additionalKeyword, namesKeyword:
		return 0
	case requiredKeyword:
		return 1
	default:
		return 2
	}
}

// applied returns the nodes checked at one place when n is: n, the node its
// $ref names, the members of its allOf, and the branch its if takes for v.
func (n *node) applied(v any) []*node {
	held := []*node{n}
	if n.ref != nil {
		held = append(held, n.ref.applied(v)...)
	}
	for _, each := range n.allOf {
		held = append(held, each.applied(v)...)
	}
	if n.ifNode == nil {
		return held
	}
	branch := n.elseNode
	if n.ifNode.matches(v, "") {
		branch = n.thenNode
	}
	if branch != nil {
		held = append(held, branch.applied(v)...)
	}
	return held
}

// below returns the value one JSON Pointer segment names inside v, and the
// nodes checked there when holders are checked at v.
func below(holders []*node, v any, segment string) (any, []*node) {
	var next []*node
	for _, holder := range holders {
		for _, at := range holder.applied(v) {
			next = append(next, at.member(v, segment)...)
		}
	}
	return valueAt(v, segment), next
}

// member returns the nodes n checks the member or the entry segment names in v
// with.
func (n *node) member(v any, segment string) []*node {
	if _, isArray := v.([]any); isArray {
		if n.items == nil {
			return nil
		}
		return []*node{n.items}
	}
	var held []*node
	declared, named := n.properties[segment]
	if named {
		held = append(held, declared)
	}
	for _, p := range n.patterns {
		if p.pattern.MatchString(segment) {
			named = true
			held = append(held, p.schema)
		}
	}
	if !named && n.additional != nil {
		held = append(held, n.additional)
	}
	return held
}

// valueAt is the value one JSON Pointer segment names inside v, or nil.
func valueAt(v any, segment string) any {
	switch held := v.(type) {
	case map[string]any:
		return held[segment]
	case []any:
		index, err := strconv.Atoi(segment)
		if err != nil || index < 0 || index >= len(held) {
			return nil
		}
		return held[index]
	default:
		return nil
	}
}

// closest returns the alternative a value matching none of them was written
// for, or nil where none or two remain. An alternative counts every node
// applied with it at v: each one's type must admit v, and its members are all
// of theirs.
func closest(alternatives []*node, v any) *node {
	var typed []*node
	for _, each := range alternatives {
		if admitsType(each.applied(v), v) {
			typed = append(typed, each)
		}
	}
	object, isObject := v.(map[string]any)
	if len(typed) == 1 || !isObject {
		return onlyOne(typed)
	}
	return onlyOne(mostDeclaring(typed, object))
}

// admitsType reports whether every type constraint of the nodes admits v.
func admitsType(nodes []*node, v any) bool {
	for _, n := range nodes {
		if len(n.types) > 0 && !hasAnyType(v, n.types) {
			return false
		}
	}
	return true
}

// mostDeclaring returns the alternatives declaring the most of an object's
// members, and of those, the ones declaring the fewest members in all.
func mostDeclaring(alternatives []*node, object map[string]any) []*node {
	var best []*node
	most, fewest := -1, 0
	for _, alternative := range alternatives {
		named, declared := declares(alternative, object)
		switch {
		case named > most || named == most && declared < fewest:
			best, most, fewest = append(best[:0], alternative), named, declared
		case named == most && declared == fewest:
			best = append(best, alternative)
		}
	}
	return best
}

// declares counts the object's members the alternative declares, with every
// node applied with it, and the members it declares in all.
func declares(alternative *node, object map[string]any) (named, declared int) {
	properties := map[string]bool{}
	for _, at := range alternative.applied(object) {
		for name := range at.properties {
			properties[name] = true
		}
	}
	for name := range object {
		if properties[name] {
			named++
		}
	}
	return named, len(properties)
}

func onlyOne(alternatives []*node) *node {
	if len(alternatives) != 1 {
		return nil
	}
	return alternatives[0]
}

// hasAnyType reports whether v is of one of the JSON Schema types.
func hasAnyType(v any, types []string) bool {
	for _, name := range types {
		if hasType(v, name) {
			return true
		}
	}
	return false
}

// segments are the member names and indexes of a JSON Pointer, unescaped.
func segments(pointer string) []string {
	if pointer == "" {
		return nil
	}
	split := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for i, segment := range split {
		split[i] = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
	}
	return split
}
