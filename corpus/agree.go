package corpus

import (
	"fmt"
	"strings"
)

// Divergence is one disagreement between two products over one expectation
// both answered.
type Divergence struct {
	Fixture string
	Symbol  string
	Field   string
	First   ProductAnswer
	Second  ProductAnswer
}

// ProductAnswer is one product's answer to the field under comparison.
type ProductAnswer struct {
	Product string
	Value   string
}

func (d Divergence) String() string {
	return fmt.Sprintf("fixture %s symbol %s: %s is %q for %s and %q for %s",
		d.Fixture, d.Symbol, d.Field, d.First.Value, d.First.Product, d.Second.Value, d.Second.Product)
}

// Exclusion is one expectation both products answered that the comparison left
// out because a product reported it as a gap, a permitted absence rather than a
// different answer, or answered it in a fixture its results mark pending, whose
// rule a product is not yet required to meet.
type Exclusion struct {
	Fixture  string
	Symbol   string
	Products []string
	Pending  bool
}

func (e Exclusion) String() string {
	why := "reported as a gap"
	if e.Pending {
		why = "pending"
	}
	return fmt.Sprintf("fixture %s symbol %s: excluded, %s for %s",
		e.Fixture, e.Symbol, why, strings.Join(e.Products, " and "))
}

// Agreement is the outcome of comparing two products' results. Compared counts
// the expectations both answered, so a run that paired nothing is not read as
// agreement.
type Agreement struct {
	Divergences []Divergence
	Excluded    []Exclusion
	Compared    int
}

type answerKey struct {
	fixture string
	symbol  string
}

// Agree compares two products' results expectation by expectation, in the first
// document's order. Only an expectation both answer is compared, so a fixture
// rendered in one language alone is no disagreement, and one either product
// reports as a gap, or answers in a fixture its results mark pending, is excluded
// and recorded. Where the codes differ the code is the one divergence recorded;
// otherwise the confidence and the suppression are compared independently.
func Agree(a, b *Results) Agreement {
	rows := make(map[answerKey]*ExpectationResult)
	pendingInB := make(map[string]bool)
	for i := range b.Fixtures {
		pendingInB[b.Fixtures[i].Fixture] = b.Fixtures[i].Result == Pending
		for j := range b.Fixtures[i].Expectations {
			row := &b.Fixtures[i].Expectations[j]
			rows[answerKey{fixture: b.Fixtures[i].Fixture, symbol: row.Symbol}] = row
		}
	}
	var out Agreement
	for i := range a.Fixtures {
		fixture := &a.Fixtures[i]
		for j := range fixture.Expectations {
			first := &fixture.Expectations[j]
			second, both := rows[answerKey{fixture: fixture.Fixture, symbol: first.Symbol}]
			if !both {
				continue
			}
			if pending := pendingOf(fixture.Result == Pending, a.Product.Name, pendingInB[fixture.Fixture], b.Product.Name); len(pending) != 0 {
				out.Excluded = append(out.Excluded, Exclusion{Fixture: fixture.Fixture, Symbol: first.Symbol, Products: pending, Pending: true})
				continue
			}
			out.compare(fixture.Fixture, side{first, a.Product.Name}, side{second, b.Product.Name})
		}
	}
	return out
}

// pendingOf names, first product first, each product whose results mark the
// fixture pending.
func pendingOf(inA bool, a string, inB bool, b string) []string {
	var named []string
	if inA {
		named = append(named, a)
	}
	if inB {
		named = append(named, b)
	}
	return named
}

type side struct {
	row     *ExpectationResult
	product string
}

// compare records one expectation both products answered: an exclusion when
// either reports a gap, the code when the codes differ, and otherwise each of
// the confidence and the suppression that differs.
func (out *Agreement) compare(fixture string, a, b side) {
	var gaps []string
	for _, s := range []side{a, b} {
		if s.row.Result == Gap {
			gaps = append(gaps, s.product)
		}
	}
	if len(gaps) != 0 {
		out.Excluded = append(out.Excluded, Exclusion{Fixture: fixture, Symbol: a.row.Symbol, Products: gaps})
		return
	}
	out.Compared++
	at := func(field, x, y string) {
		if x != y {
			out.Divergences = append(out.Divergences, Divergence{
				Fixture: fixture, Symbol: a.row.Symbol, Field: field,
				First:  ProductAnswer{Product: a.product, Value: x},
				Second: ProductAnswer{Product: b.product, Value: y},
			})
		}
	}
	if a.row.Actual.Report != b.row.Actual.Report {
		at("report", a.row.Actual.Report, b.row.Actual.Report)
		return
	}
	at("confidence", orAbsent(a.row.Actual.Confidence), orAbsent(b.row.Actual.Confidence))
	at("suppression", suppressionAnswer(a.row.Suppression), suppressionAnswer(b.row.Suppression))
}

func orAbsent(value string) string {
	if value == "" {
		return "absent"
	}
	return value
}

// suppressionAnswer renders a suppression block so each of its five states reads
// differently in a divergence.
func suppressionAnswer(s *Suppression) string {
	switch {
	case s == nil:
		return "absent"
	case s.Suppressed && s.Stale:
		return "suppressed, with a stale entry"
	case s.Suppressed:
		return "suppressed"
	case s.Stale:
		return "not suppressed, with a stale entry"
	default:
		return "not suppressed"
	}
}
