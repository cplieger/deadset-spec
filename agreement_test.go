package spec_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// productAnswer is one product's answer to the field under comparison.
type productAnswer struct {
	Product string
	Value   string
}

// divergence is one disagreement between two products over one expectation
// both answered: the field that differs and both answers to it.
type divergence struct {
	Fixture string
	Symbol  string
	Field   string
	First   productAnswer
	Second  productAnswer
}

var _ fmt.Stringer = divergence{}

// String names the fixture, the logical symbol and both answers, so a
// divergence is readable without either document.
func (d divergence) String() string {
	return fmt.Sprintf("fixture %s symbol %s: %s is %q for %s and %q for %s",
		d.Fixture, d.Symbol, d.Field, d.First.Value, d.First.Product, d.Second.Value, d.Second.Product)
}

// exclusion is one expectation both products answered that the comparison left
// out, because a product reported it as a gap: a declared gap is a permitted
// absence rather than a different answer, and the products that declared it
// are named so the exclusion is a recorded value and not a silence.
type exclusion struct {
	Fixture  string
	Symbol   string
	Products []string
}

var _ fmt.Stringer = exclusion{}

// String names the fixture, the logical symbol and every product that reported
// the expectation as a gap.
func (e exclusion) String() string {
	return fmt.Sprintf("fixture %s symbol %s: excluded, reported as a gap by %s",
		e.Fixture, e.Symbol, strings.Join(e.Products, " and "))
}

// agreement is the outcome of comparing two products' conformance results.
// Compared counts the expectations the comparison reached, so a run that
// paired nothing is not read as agreement.
type agreement struct {
	Divergences []divergence
	Excluded    []exclusion
	Compared    int
}

// answerKey identifies one expectation across two results documents, which is
// the pair the corpus owns the expected answer for.
type answerKey struct {
	fixture string
	symbol  string
}

// answerSide is one product's row for one expectation.
type answerSide struct {
	Product string
	Row     expectationResult
}

// answerIndex maps every expectation a results document answers to its row.
func answerIndex(doc *resultsDocument) map[answerKey]expectationResult {
	rows := make(map[answerKey]expectationResult)
	for _, fixture := range doc.Fixtures {
		for _, row := range fixture.Expectations {
			rows[answerKey{fixture: fixture.Fixture, symbol: row.Symbol}] = row
		}
	}
	return rows
}

// answerOrAbsent renders an optional answer field as the value the comparison
// compares, so a field one product left out is an answer rather than an empty
// string in a message.
func answerOrAbsent(value string) string {
	if value == "" {
		return "absent"
	}
	return value
}

// suppressionAnswer renders an expectation's suppression block as the value
// the comparison compares: two products agree on suppression behavior exactly
// when these agree. Each of the five states a block can be in reads
// differently, so a divergence names what each product did.
func suppressionAnswer(s *suppressionResult) string {
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

// gapSides names the products that reported the expectation as a gap.
func gapSides(a, b answerSide) []string {
	var products []string
	if a.Row.Result == gapResult {
		products = append(products, a.Product)
	}
	if b.Row.Result == gapResult {
		products = append(products, b.Product)
	}
	return products
}

// compareAnswer compares one expectation's two answers. Where the two products
// report different codes the code is the only divergence recorded: the
// confidence and the suppression behavior of two different findings are not
// comparable, and reporting them would name three differences for one cause.
// Where the codes agree, the confidence and the suppression are compared
// independently, so a product that suppresses a finding another does not is
// reported whatever the confidence says.
func compareAnswer(fixture string, a, b answerSide) []divergence {
	at := func(field, first, second string) []divergence {
		if first == second {
			return nil
		}
		return []divergence{{
			Fixture: fixture,
			Symbol:  a.Row.Symbol,
			Field:   field,
			First:   productAnswer{Product: a.Product, Value: first},
			Second:  productAnswer{Product: b.Product, Value: second},
		}}
	}
	if code := at("report", a.Row.Actual.Report, b.Row.Actual.Report); code != nil {
		return code
	}
	out := at("confidence", answerOrAbsent(a.Row.Actual.Confidence), answerOrAbsent(b.Row.Actual.Confidence))
	return append(out, at("suppression", suppressionAnswer(a.Row.Suppression), suppressionAnswer(b.Row.Suppression))...)
}

// agreementBetween compares two products' conformance results expectation by
// expectation, which is the check neither analyzer can hold because neither
// may depend on the other. Only an expectation both documents answer is
// compared, so a fixture rendered in one language alone is not a
// disagreement, and an expectation either product reports as a gap is excluded
// and recorded instead. The order of the result follows the first document.
func agreementBetween(a, b *resultsDocument) agreement {
	rows := answerIndex(b)
	var out agreement
	for _, fixture := range a.Fixtures {
		for _, row := range fixture.Expectations {
			second, both := rows[answerKey{fixture: fixture.Fixture, symbol: row.Symbol}]
			if !both {
				continue
			}
			first := answerSide{Product: a.Product.Name, Row: row}
			paired := answerSide{Product: b.Product.Name, Row: second}
			if gaps := gapSides(first, paired); len(gaps) != 0 {
				out.Excluded = append(out.Excluded, exclusion{Fixture: fixture.Fixture, Symbol: row.Symbol, Products: gaps})
				continue
			}
			out.Compared++
			out.Divergences = append(out.Divergences, compareAnswer(fixture.Fixture, first, paired)...)
		}
	}
	return out
}

// The two synthetic results documents the cases below compare: one product per
// language, each answering the fixtures its language is rendered in, agreeing
// on every expectation of the fixture both answer. A case plants one
// difference in a copy of the second, so every divergence it reports has one
// cause.
const (
	goProduct = "deadset-go"
	tsProduct = "deadset-ts"

	sharedFixture   = "unused-exported-consumer"
	sharedReported  = "DeadExport"
	sharedUnreached = "UsedByConsumer"

	goOnlyFixture = "interface-satisfaction-conversion"
	tsOnlyFixture = "private-member-unread"
)

const goAnswers = `{
  "corpus_version": "1.0.0",
  "product": {"name": "deadset-go", "version": "1.0.0", "language": "go"},
  "result": "pass",
  "totals": {"fixtures": 2, "pass": 2, "gap": 0, "fail": 0},
  "fixtures": [
    {
      "fixture": "interface-satisfaction-conversion",
      "result": "pass",
      "expectations": [
        {
          "symbol": "DeadExport",
          "result": "pass",
          "actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting"},
          "suppression": {"suppressed": true, "stale": false}
        },
        {
          "symbol": "SatisfiesWriter",
          "result": "pass",
          "actual": {"report": "none", "retained_by": ["interface-satisfaction"]}
        }
      ],
      "unexpected": []
    },
    {
      "fixture": "unused-exported-consumer",
      "result": "pass",
      "expectations": [
        {
          "symbol": "DeadExport",
          "result": "pass",
          "actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting"},
          "suppression": {"suppressed": true, "stale": false}
        },
        {
          "symbol": "UsedByConsumer",
          "result": "pass",
          "actual": {"report": "none"}
        }
      ],
      "unexpected": []
    }
  ]
}`

const tsAnswers = `{
  "corpus_version": "1.0.0",
  "product": {"name": "deadset-ts", "version": "1.0.0", "language": "ts"},
  "result": "pass",
  "totals": {"fixtures": 2, "pass": 2, "gap": 0, "fail": 0},
  "fixtures": [
    {
      "fixture": "private-member-unread",
      "result": "pass",
      "expectations": [
        {
          "symbol": "ReachedByStringIndex",
          "result": "pass",
          "actual": {"report": "none", "retained_by": ["reflective-lookup"]}
        },
        {
          "symbol": "UnreadPrivate",
          "result": "pass",
          "actual": {"report": "DS1301", "confidence": "certain", "reachability_class": "certain"},
          "suppression": {"suppressed": true, "stale": false}
        }
      ],
      "unexpected": []
    },
    {
      "fixture": "unused-exported-consumer",
      "result": "pass",
      "expectations": [
        {
          "symbol": "DeadExport",
          "result": "pass",
          "actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting"},
          "suppression": {"suppressed": true, "stale": false}
        },
        {
          "symbol": "UsedByConsumer",
          "result": "pass",
          "actual": {"report": "none"}
        }
      ],
      "unexpected": []
    }
  ]
}`

// decodeAnswers decodes one synthetic results document, failing the test when
// it carries a field the results shape does not declare.
func decodeAnswers(t *testing.T, name, data string) *resultsDocument {
	t.Helper()
	var doc resultsDocument
	if err := decodeStrict([]byte(data), &doc); err != nil {
		t.Fatalf("Setup: decoding the synthetic %s results: %v", name, err)
	}
	return &doc
}

// answerPair decodes both synthetic documents.
func answerPair(t *testing.T) (first, second *resultsDocument) {
	t.Helper()
	return decodeAnswers(t, goProduct, goAnswers), decodeAnswers(t, tsProduct, tsAnswers)
}

// changeAnswer returns a copy of doc with one expectation row replaced by what
// change makes of a copy of it. Every field a case plants is assigned on the
// copy, and a new suppression block is assigned rather than the pointed-to one
// mutated, so the document it came from is unchanged.
func changeAnswer(doc *resultsDocument, fixture, symbol string, change func(*expectationResult)) *resultsDocument {
	out := *doc
	out.Fixtures = slices.Clone(doc.Fixtures)
	for i := range out.Fixtures {
		if out.Fixtures[i].Fixture != fixture {
			continue
		}
		out.Fixtures[i].Expectations = slices.Clone(out.Fixtures[i].Expectations)
		for j := range out.Fixtures[i].Expectations {
			if out.Fixtures[i].Expectations[j].Symbol == symbol {
				change(&out.Fixtures[i].Expectations[j])
			}
		}
	}
	return &out
}

func TestAgreementSyntheticResultsAreInstancesOfTheResultsSchema(t *testing.T) {
	// The comparison is only worth what its inputs are: a document no product
	// could write would let the checker agree with itself about a shape the
	// corpus does not define.
	documents := []struct {
		product string
		doc     string
	}{
		{product: goProduct, doc: goAnswers},
		{product: tsProduct, doc: tsAnswers},
	}
	for _, tc := range documents {
		t.Run(tc.product, func(t *testing.T) {
			validateAgainst(t, resultsSchemaPath, []byte(tc.doc))
		})
	}
}

func TestAgreementOnTwoProductsAnsweringAlike(t *testing.T) {
	first, second := answerPair(t)
	got := agreementBetween(first, second)

	if len(got.Divergences) != 0 {
		t.Errorf("agreementBetween(%s, %s).Divergences = %v, want none: both answer every shared expectation alike", goProduct, tsProduct, got.Divergences)
	}
	if len(got.Excluded) != 0 {
		t.Errorf("agreementBetween(%s, %s).Excluded = %v, want none: neither product reports a gap", goProduct, tsProduct, got.Excluded)
	}
	// The two expectations of the fixture both languages render, and only
	// those: a fixture one product alone answers is not a shared expectation,
	// so a comparison that paired by symbol alone would count more.
	if got.Compared != 2 {
		t.Errorf("agreementBetween(%s, %s).Compared = %d, want 2, the expectations of %s", goProduct, tsProduct, got.Compared, sharedFixture)
	}
}

func TestAgreementIgnoresAFixtureOneProductAnswers(t *testing.T) {
	first, second := answerPair(t)
	got := agreementBetween(first, second)

	for _, d := range got.Divergences {
		if d.Fixture == goOnlyFixture || d.Fixture == tsOnlyFixture {
			t.Errorf("agreementBetween(%s, %s) reported %v, want no divergence for a fixture one language renders", goProduct, tsProduct, d)
		}
	}
	// The same run the other way round, so the single-language fixture of the
	// first document is the one skipped in one direction and the other's in
	// the other.
	if reverse := agreementBetween(second, first); len(reverse.Divergences) != 0 || reverse.Compared != got.Compared {
		t.Errorf("agreementBetween(%s, %s) = %+v, want no divergence and %d compared", tsProduct, goProduct, reverse, got.Compared)
	}
}

func TestAgreementNamesEveryDivergence(t *testing.T) {
	// Every case plants one difference in the second product's answer and
	// states the divergences the comparison must report for it, both answers
	// included. The fixture and the symbol are the same in every want, so the
	// loop fills them in from the case rather than each want repeating them.
	cases := []struct {
		change func(*expectationResult)
		name   string
		symbol string
		want   []divergence
	}{
		{
			name:   "code",
			symbol: sharedReported,
			change: func(row *expectationResult) {
				row.Actual = answer{Report: reportedNone}
				row.Suppression = nil
			},
			want: []divergence{{
				Field:  "report",
				First:  productAnswer{Product: goProduct, Value: "DS1001"},
				Second: productAnswer{Product: tsProduct, Value: "none"},
			}},
		},
		{
			name:   "confidence",
			symbol: sharedReported,
			change: func(row *expectationResult) { row.Actual.Confidence = "probable" },
			want: []divergence{{
				Field:  "confidence",
				First:  productAnswer{Product: goProduct, Value: "certain"},
				Second: productAnswer{Product: tsProduct, Value: "probable"},
			}},
		},
		{
			name:   "suppression",
			symbol: sharedReported,
			change: func(row *expectationResult) {
				row.Suppression = &suppressionResult{Suppressed: false, Stale: true}
			},
			want: []divergence{{
				Field:  "suppression",
				First:  productAnswer{Product: goProduct, Value: "suppressed"},
				Second: productAnswer{Product: tsProduct, Value: "not suppressed, with a stale entry"},
			}},
		},
		{
			name:   "confidence_and_suppression",
			symbol: sharedReported,
			change: func(row *expectationResult) {
				row.Actual.Confidence = "possible"
				row.Suppression = &suppressionResult{Suppressed: true, Stale: true}
			},
			want: []divergence{
				{
					Field:  "confidence",
					First:  productAnswer{Product: goProduct, Value: "certain"},
					Second: productAnswer{Product: tsProduct, Value: "possible"},
				},
				{
					Field:  "suppression",
					First:  productAnswer{Product: goProduct, Value: "suppressed"},
					Second: productAnswer{Product: tsProduct, Value: "suppressed, with a stale entry"},
				},
			},
		},
		{
			name:   "an_answer_where_the_other_product_reports_nothing",
			symbol: sharedUnreached,
			change: func(row *expectationResult) {
				row.Actual = answer{Report: "DS1001", Confidence: "certain"}
				row.Suppression = &suppressionResult{Suppressed: true}
			},
			want: []divergence{{
				Field:  "report",
				First:  productAnswer{Product: goProduct, Value: "none"},
				Second: productAnswer{Product: tsProduct, Value: "DS1001"},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, second := answerPair(t)
			planted := changeAnswer(second, sharedFixture, tc.symbol, tc.change)
			want := slices.Clone(tc.want)
			for i := range want {
				want[i].Fixture = sharedFixture
				want[i].Symbol = tc.symbol
			}

			got := agreementBetween(first, planted)
			if !slices.Equal(got.Divergences, want) {
				t.Fatalf("agreementBetween(%s, %s with a planted %s divergence).Divergences = %v, want %v", goProduct, tsProduct, tc.name, got.Divergences, want)
			}
			for _, d := range got.Divergences {
				for _, named := range []string{sharedFixture, tc.symbol, d.First.Product, d.First.Value, d.Second.Product, d.Second.Value} {
					if !strings.Contains(d.String(), named) {
						t.Errorf("divergence %q, want it to name %s", d, named)
					}
				}
			}
			if got.Compared != 2 {
				t.Errorf("agreementBetween(planted %s).Compared = %d, want 2: a divergence is compared, not excluded", tc.name, got.Compared)
			}
		})
	}
}

func TestAgreementExcludesADeclaredGap(t *testing.T) {
	// The gap answer a product writes for a capability it declines: the
	// expectation is reported as a gap and the answer says nothing was
	// reported, which is a divergence in code from a product that implements
	// the capability, and must not be reported as one.
	gapAnswer := func(row *expectationResult) {
		row.Result = gapResult
		row.Capability = "DS1001"
		row.Actual = answer{Report: reportedNone}
		row.Suppression = nil
	}

	t.Run("a_gap_on_one_side", func(t *testing.T) {
		first, second := answerPair(t)
		planted := changeAnswer(second, sharedFixture, sharedReported, gapAnswer)

		got := agreementBetween(first, planted)
		if len(got.Divergences) != 0 {
			t.Errorf("agreementBetween(%s, %s declaring a gap) = %v, want no divergence: a gap is a permitted absence", goProduct, tsProduct, got.Divergences)
		}
		if len(got.Excluded) != 1 {
			t.Fatalf("agreementBetween(%s, %s declaring a gap).Excluded = %v, want one record", goProduct, tsProduct, got.Excluded)
		}
		if !slices.Equal(got.Excluded[0].Products, []string{tsProduct}) {
			t.Errorf("Excluded[0].Products = %v, want only %v, the product that declared it", got.Excluded[0].Products, []string{tsProduct})
		}
		for _, want := range []string{sharedFixture, sharedReported, tsProduct} {
			if !strings.Contains(got.Excluded[0].String(), want) {
				t.Errorf("exclusion %q, want it to name %s", got.Excluded[0], want)
			}
		}
		if got.Compared != 1 {
			t.Errorf("agreementBetween(%s, %s declaring a gap).Compared = %d, want 1: the other expectation of %s", goProduct, tsProduct, got.Compared, sharedFixture)
		}
	})

	t.Run("a_gap_on_both_sides", func(t *testing.T) {
		first, second := answerPair(t)
		plantedFirst := changeAnswer(first, sharedFixture, sharedReported, gapAnswer)
		plantedSecond := changeAnswer(second, sharedFixture, sharedReported, gapAnswer)

		got := agreementBetween(plantedFirst, plantedSecond)
		if len(got.Excluded) != 1 {
			t.Fatalf("agreementBetween(two products declaring the same gap).Excluded = %v, want one record", got.Excluded)
		}
		if !slices.Equal(got.Excluded[0].Products, []string{goProduct, tsProduct}) {
			t.Errorf("Excluded[0].Products = %v, want both products named", got.Excluded[0].Products)
		}
	})
}

func TestSuppressionAnswerTellsEveryStateApart(t *testing.T) {
	states := []*suppressionResult{
		nil,
		{Suppressed: true, Stale: false},
		{Suppressed: true, Stale: true},
		{Suppressed: false, Stale: false},
		{Suppressed: false, Stale: true},
	}
	seen := make(map[string]*suppressionResult, len(states))
	for _, s := range states {
		got := suppressionAnswer(s)
		if other, repeated := seen[got]; repeated {
			t.Errorf("suppressionAnswer(%+v) = %q, the value of %+v: two states a product can be in must not compare equal", s, got, other)
		}
		seen[got] = s
	}
}
