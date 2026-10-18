package corpus

import (
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8/facts"
	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// factManifest names a consumer and a fallback declared in a generated file,
// a hand-written implementer and the helper only it reaches.
func factManifest() *Manifest {
	line := func(n int) *int { return &n }
	return &Manifest{Symbols: map[string]ManifestPosition{
		"PingRequest": {File: "target/gen/v1/service.pb.go", Line: line(12), Ref: "go://example.com/target/gen/v1#PingRequest"},
		"ServiceFile": {File: "target/src/gen/v1/service.ts", Line: line(1), Ref: "ts://@example/target/src/gen/v1/service.ts#"},
		"server.Ping": {File: "target/main.go", Line: line(15), Ref: "go://example.com/target#server.Ping"},
		"doubled":     {File: "target/main.go", Line: line(19), Ref: "go://example.com/target#doubled"},
	}}
}

// factRecords is a document answering factRows exactly.
func factRecords() *facts.Document {
	finding := func(code, ref string, line int) report.Finding {
		return report.Finding{Code: vocab.Code(code), Position: report.Position{Path: "main.go", Line: line}, Symbol: report.Symbol{Ref: ref}}
	}
	ping := finding("DS1101", "go://example.com/target#server.Ping", 15)
	return &facts.Document{Bindings: []facts.Binding{
		{Role: facts.RoleConsumer, Symbol: "go://example.com/target/gen/v1#PingRequest", Schema: "proto://v1#PingRequest", GeneratedFile: "gen/v1/service.pb.go", State: facts.StateLive},
		{Role: facts.RoleFallback, Symbol: "ts://@example/target/src/gen/v1/service.ts#", GeneratedFile: "src/gen/v1/service.ts", Scope: facts.ScopeTypes, State: facts.StateLive},
		{
			Role: facts.RoleImplementer, Symbol: "go://example.com/target#server.Ping", Schema: "proto://v1#PingService.Ping", State: facts.StateDead,
			Finding: &ping, Carried: []report.Finding{finding("DS1002", "go://example.com/target#doubled", 19)},
		},
	}}
}

func factRows() []FactRow {
	return []FactRow{
		{Symbol: "PingRequest", Role: "consumer", Schema: "proto://v1#PingRequest", State: "live"},
		{Symbol: "ServiceFile", Role: "fallback", State: "live", Scope: "types"},
		{
			Symbol: "server.Ping", Role: "implementer", Schema: "proto://v1#PingService.Ping", State: "dead", Report: "DS1101",
			Carried: []FactFinding{{Symbol: "doubled", Report: "DS1002"}},
		},
	}
}

// TestFactDifferenceComparesTheGeneratedFileScopeAndCarried holds each of the
// three members to the record: a document answering every row passes, and
// one changed in that member alone fails naming it.
func TestFactDifferenceComparesTheGeneratedFileScopeAndCarried(t *testing.T) {
	for i, row := range factRows() {
		if got := factDifference(&row, factManifest(), factRecords()); got != "" {
			t.Errorf("factDifference(row %d of a document answering it) = %q, want none", i, got)
		}
	}
	for _, c := range []struct {
		name, member string
		row          int
		change       func(*facts.Document)
	}{
		{"consumer_generated_file", "generated_file", 0, func(d *facts.Document) { d.Bindings[0].GeneratedFile = "gen/v1/other.pb.go" }},
		{"fallback_generated_file", "generated_file", 1, func(d *facts.Document) { d.Bindings[1].GeneratedFile = "src/gen/v1/other.ts" }},
		{"fallback_scope", "scope", 1, func(d *facts.Document) { d.Bindings[1].Scope = facts.ScopeAll }},
		{"carried_missing", "carried", 2, func(d *facts.Document) { d.Bindings[2].Carried = nil }},
		{"carried_code", "carried", 2, func(d *facts.Document) { d.Bindings[2].Carried[0].Code = "DS1001" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := factRecords()
			c.change(doc)
			rows := factRows()
			if got := factDifference(&rows[c.row], factManifest(), doc); !strings.Contains(got, " "+c.member+" want ") {
				t.Errorf("factDifference(a record changed in %s) = %q, want a difference naming it", c.member, got)
			}
		})
	}
}
