package corpus

import (
	"slices"
	"strings"
	"testing"
)

// The two synthetic results documents the cases below compare: one product per
// language, each answering the fixtures its language is rendered in, agreeing
// on every expectation of the fixture both answer. A case plants one
// difference in a copy of the second, so every Divergence it reports has one
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
  "totals": {"fixtures": 2, "pass": 2, "gap": 0, "fail": 0, "pending": 0},
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
  "totals": {"fixtures": 2, "pass": 2, "gap": 0, "fail": 0, "pending": 0},
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
func decodeAnswers(t *testing.T, name, data string) *Results {
	t.Helper()
	var doc Results
	if err := DecodeStrict([]byte(data), &doc); err != nil {
		t.Fatalf("Setup: decoding the synthetic %s results: %v", name, err)
	}
	return &doc
}

// answerPair decodes both synthetic documents.
func answerPair(t *testing.T) (first, second *Results) {
	t.Helper()
	return decodeAnswers(t, goProduct, goAnswers), decodeAnswers(t, tsProduct, tsAnswers)
}

// changeAnswer returns a copy of doc with one expectation row replaced by what
// change makes of a copy of it. Every field a case plants is assigned on the
// copy, and a new suppression block is assigned rather than the pointed-to one
// mutated, so the document it came from is unchanged.
func changeAnswer(doc *Results, fixture, symbol string, change func(*ExpectationResult)) *Results {
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
			if err := Validate(ResultsSchemaPath, []byte(tc.doc)); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestAgreementOnTwoProductsAnsweringAlike(t *testing.T) {
	first, second := answerPair(t)
	got := Agree(first, second)

	if len(got.Divergences) != 0 {
		t.Errorf("Agree(%s, %s).Divergences = %v, want none: both answer every shared expectation alike", goProduct, tsProduct, got.Divergences)
	}
	if len(got.Excluded) != 0 {
		t.Errorf("Agree(%s, %s).Excluded = %v, want none: neither product reports a gap", goProduct, tsProduct, got.Excluded)
	}
	// The two expectations of the fixture both languages render, and only
	// those: a fixture one product alone answers is not a shared expectation,
	// so a comparison that paired by symbol alone would count more.
	if got.Compared != 2 {
		t.Errorf("Agree(%s, %s).Compared = %d, want 2, the expectations of %s", goProduct, tsProduct, got.Compared, sharedFixture)
	}
}

func TestAgreementIgnoresAFixtureOneProductAnswers(t *testing.T) {
	first, second := answerPair(t)
	got := Agree(first, second)

	for _, d := range got.Divergences {
		if d.Fixture == goOnlyFixture || d.Fixture == tsOnlyFixture {
			t.Errorf("Agree(%s, %s) reported %v, want no Divergence for a fixture one language renders", goProduct, tsProduct, d)
		}
	}
	// The same run the other way round, so the single-language fixture of the
	// first document is the one skipped in one direction and the other's in
	// the other.
	if reverse := Agree(second, first); len(reverse.Divergences) != 0 || reverse.Compared != got.Compared {
		t.Errorf("Agree(%s, %s) = %+v, want no Divergence and %d compared", tsProduct, goProduct, reverse, got.Compared)
	}
}

func TestAgreementNamesEveryDivergence(t *testing.T) {
	// Every case plants one difference in the second product's answer and
	// states the divergences the comparison must report for it, both answers
	// included. The fixture and the symbol are the same in every want, so the
	// loop fills them in from the case rather than each want repeating them.
	cases := []struct {
		change func(*ExpectationResult)
		name   string
		symbol string
		want   []Divergence
	}{
		{
			name:   "code",
			symbol: sharedReported,
			change: func(row *ExpectationResult) {
				row.Actual = Answer{Report: ReportNone}
				row.Suppression = nil
			},
			want: []Divergence{{
				Field:  "report",
				First:  ProductAnswer{Product: goProduct, Value: "DS1001"},
				Second: ProductAnswer{Product: tsProduct, Value: "none"},
			}},
		},
		{
			name:   "confidence",
			symbol: sharedReported,
			change: func(row *ExpectationResult) { row.Actual.Confidence = "probable" },
			want: []Divergence{{
				Field:  "confidence",
				First:  ProductAnswer{Product: goProduct, Value: "certain"},
				Second: ProductAnswer{Product: tsProduct, Value: "probable"},
			}},
		},
		{
			name:   "suppression",
			symbol: sharedReported,
			change: func(row *ExpectationResult) {
				row.Suppression = &Suppression{Suppressed: false, Stale: true}
			},
			want: []Divergence{{
				Field:  "suppression",
				First:  ProductAnswer{Product: goProduct, Value: "suppressed"},
				Second: ProductAnswer{Product: tsProduct, Value: "not suppressed, with a stale entry"},
			}},
		},
		{
			name:   "confidence_and_suppression",
			symbol: sharedReported,
			change: func(row *ExpectationResult) {
				row.Actual.Confidence = "possible"
				row.Suppression = &Suppression{Suppressed: true, Stale: true}
			},
			want: []Divergence{
				{
					Field:  "confidence",
					First:  ProductAnswer{Product: goProduct, Value: "certain"},
					Second: ProductAnswer{Product: tsProduct, Value: "possible"},
				},
				{
					Field:  "suppression",
					First:  ProductAnswer{Product: goProduct, Value: "suppressed"},
					Second: ProductAnswer{Product: tsProduct, Value: "suppressed, with a stale entry"},
				},
			},
		},
		{
			name:   "an_answer_where_the_other_product_reports_nothing",
			symbol: sharedUnreached,
			change: func(row *ExpectationResult) {
				row.Actual = Answer{Report: "DS1001", Confidence: "certain"}
				row.Suppression = &Suppression{Suppressed: true}
			},
			want: []Divergence{{
				Field:  "report",
				First:  ProductAnswer{Product: goProduct, Value: "none"},
				Second: ProductAnswer{Product: tsProduct, Value: "DS1001"},
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

			got := Agree(first, planted)
			if !slices.Equal(got.Divergences, want) {
				t.Fatalf("Agree(%s, %s with a planted %s Divergence).Divergences = %v, want %v", goProduct, tsProduct, tc.name, got.Divergences, want)
			}
			for _, d := range got.Divergences {
				for _, named := range []string{sharedFixture, tc.symbol, d.First.Product, d.First.Value, d.Second.Product, d.Second.Value} {
					if !strings.Contains(d.String(), named) {
						t.Errorf("Divergence %q, want it to name %s", d, named)
					}
				}
			}
			if got.Compared != 2 {
				t.Errorf("Agree(planted %s).Compared = %d, want 2: a Divergence is compared, not excluded", tc.name, got.Compared)
			}
		})
	}
}

func TestAgreementExcludesADeclaredGap(t *testing.T) {
	// The gap answer a product writes for a capability it declines: the
	// expectation is reported as a gap and the answer says nothing was
	// reported, which is a Divergence in code from a product that implements
	// the capability, and must not be reported as one.
	gapAnswer := func(row *ExpectationResult) {
		row.Result = Gap
		row.Capability = "DS1001"
		row.Actual = Answer{Report: ReportNone}
		row.Suppression = nil
	}

	t.Run("a_gap_on_one_side", func(t *testing.T) {
		first, second := answerPair(t)
		planted := changeAnswer(second, sharedFixture, sharedReported, gapAnswer)

		got := Agree(first, planted)
		if len(got.Divergences) != 0 {
			t.Errorf("Agree(%s, %s declaring a gap) = %v, want no Divergence: a gap is a permitted absence", goProduct, tsProduct, got.Divergences)
		}
		if len(got.Excluded) != 1 {
			t.Fatalf("Agree(%s, %s declaring a gap).Excluded = %v, want one record", goProduct, tsProduct, got.Excluded)
		}
		if !slices.Equal(got.Excluded[0].Products, []string{tsProduct}) {
			t.Errorf("Excluded[0].Products = %v, want only %v, the product that declared it", got.Excluded[0].Products, []string{tsProduct})
		}
		for _, want := range []string{sharedFixture, sharedReported, tsProduct} {
			if !strings.Contains(got.Excluded[0].String(), want) {
				t.Errorf("Exclusion %q, want it to name %s", got.Excluded[0], want)
			}
		}
		if got.Compared != 1 {
			t.Errorf("Agree(%s, %s declaring a gap).Compared = %d, want 1: the other expectation of %s", goProduct, tsProduct, got.Compared, sharedFixture)
		}
	})

	t.Run("a_gap_on_both_sides", func(t *testing.T) {
		first, second := answerPair(t)
		plantedFirst := changeAnswer(first, sharedFixture, sharedReported, gapAnswer)
		plantedSecond := changeAnswer(second, sharedFixture, sharedReported, gapAnswer)

		got := Agree(plantedFirst, plantedSecond)
		if len(got.Excluded) != 1 {
			t.Fatalf("Agree(two products declaring the same gap).Excluded = %v, want one record", got.Excluded)
		}
		if !slices.Equal(got.Excluded[0].Products, []string{goProduct, tsProduct}) {
			t.Errorf("Excluded[0].Products = %v, want both products named", got.Excluded[0].Products)
		}
	})
}

func TestAgreementExcludesAPendingFixture(t *testing.T) {
	first, second := answerPair(t)
	planted := changeAnswer(second, sharedFixture, sharedReported, func(row *ExpectationResult) {
		row.Result = Fail
		row.Actual = Answer{Report: ReportNone}
		row.Suppression = nil
	})
	for i := range planted.Fixtures {
		if planted.Fixtures[i].Fixture == sharedFixture {
			planted.Fixtures[i].Result = Pending
		}
	}

	got := Agree(first, planted)
	if len(got.Divergences) != 0 {
		t.Errorf("Agree(%s, %s pending %s) = %v, want no Divergence: a pending rule is not yet required", goProduct, tsProduct, sharedFixture, got.Divergences)
	}
	if len(got.Excluded) != 2 || !got.Excluded[0].Pending || !slices.Equal(got.Excluded[0].Products, []string{tsProduct}) {
		t.Fatalf("Agree(%s, %s pending %s).Excluded = %+v, want both expectations excluded as pending for %s", goProduct, tsProduct, sharedFixture, got.Excluded, tsProduct)
	}
	if text := got.Excluded[0].String(); !strings.Contains(text, "pending") || !strings.Contains(text, tsProduct) {
		t.Errorf("Exclusion %q, want it to say pending and name %s", text, tsProduct)
	}
	if got.Compared != 0 {
		t.Errorf("Agree(%s, %s pending %s).Compared = %d, want 0: every shared expectation sits in the pending fixture", goProduct, tsProduct, sharedFixture, got.Compared)
	}
}

func TestSuppressionAnswerTellsEveryStateApart(t *testing.T) {
	states := []*Suppression{
		nil,
		{Suppressed: true, Stale: false},
		{Suppressed: true, Stale: true},
		{Suppressed: false, Stale: false},
		{Suppressed: false, Stale: true},
	}
	seen := make(map[string]*Suppression, len(states))
	for _, s := range states {
		got := suppressionAnswer(s)
		if other, repeated := seen[got]; repeated {
			t.Errorf("suppressionAnswer(%+v) = %q, the value of %+v: two states a product can be in must not compare equal", s, got, other)
		}
		seen[got] = s
	}
}
