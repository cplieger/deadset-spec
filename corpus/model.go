// Package corpus reads, checks and runs the conformance corpus: the fixtures,
// the expectation model and the runner every analyzer is answered by. One typed
// model of each document lives here, decoded strictly, so a member a document
// gains fails every reader until this package reads it. A product runs the corpus
// black-box through its command line with Run, the spec checks the fixtures with
// Check, and Agree compares two products' results documents.
package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// The paths of the corpus documents inside the spec module's corpus tree.
const (
	DocumentPath       = "corpus/corpus.json"
	ExpectSchemaPath   = "corpus/expect.schema.json"
	ManifestSchemaPath = "corpus/fixture.schema.json"
	GapsSchemaPath     = "corpus/conformance.schema.json"
	ResultsSchemaPath  = "corpus/conformance-results.schema.json"
	FixturesDir        = "corpus/fixtures"
	ExpectFile         = "expect.json"
	ManifestFile       = "fixture.json"
	TargetDir          = "target"
	InstalledDir       = "installed"
	ReportNone         = "none"
)

// Document is corpus/corpus.json.
type Document struct {
	Runner        map[string]string `json:"runner"`
	Description   string            `json:"description"`
	CorpusVersion string            `json:"corpus_version"`
	Layout        Layout            `json:"layout"`
	Schemas       Schemas           `json:"schemas"`
	Vocabularies  Vocabularies      `json:"vocabularies"`
	SubjectShapes SubjectShapes     `json:"subject_shapes"`
}

// Layout is where the parts of a fixture live. Renderings maps each language to
// its rendering's form: a name ending in a slash is a directory, any other a txtar
// archive.
type Layout struct {
	Renderings          map[string]string `json:"renderings"`
	Description         string            `json:"description"`
	FixtureDirectory    string            `json:"fixture_directory"`
	ExpectationFile     string            `json:"expectation_file"`
	ManifestFile        string            `json:"manifest_file"`
	TargetDirectory     string            `json:"target_directory"`
	ConsumerDirectory   string            `json:"consumer_directory"`
	DependencyDirectory string            `json:"dependency_directory"`
	InstalledDirectory  string            `json:"installed_directory"`
}

// Schemas names the schema each corpus document is an instance of.
type Schemas struct {
	Description     string `json:"description"`
	ExpectationFile string `json:"expectation_file"`
	ManifestFile    string `json:"manifest_file"`
	DeclaredGaps    string `json:"declared_gaps"`
	Results         string `json:"results"`
}

// Vocabularies names the Contract document each expectation field draws from.
type Vocabularies struct {
	Fields      map[string]Vocabulary `json:"fields"`
	Description string                `json:"description"`
}

// Vocabulary is one expectation field's closed vocabulary.
type Vocabulary struct {
	File        string   `json:"file"`
	Field       string   `json:"field"`
	Description string   `json:"description"`
	Also        []string `json:"also"`
}

// SubjectShapes partitions the symbol kinds into parts, dependencies and rows;
// every other kind is a declaration.
type SubjectShapes struct {
	Description string   `json:"description"`
	Part        []string `json:"part"`
	Dependency  []string `json:"dependency"`
	Row         []string `json:"row"`
}

// Expectation is one fixture's expect.json.
type Expectation struct {
	ConfiguredRoots        map[string]string                `json:"configured_roots"`
	ConfiguredDeclarations map[string]ConfiguredDeclaration `json:"configured_declarations"`
	SuppressionsInEffect   *int                             `json:"suppressions_in_effect"`
	SetupFailure           *SetupFailure                    `json:"setup_failure"`
	Severity               map[string]string                `json:"severity"`
	TargetKind             TargetKind                       `json:"target_kind"`
	Name                   string                           `json:"name"`
	Description            string                           `json:"description"`
	Pending                string                           `json:"pending"`
	MinConfidence          string                           `json:"min_confidence"`
	ConsumerTests          string                           `json:"consumer_tests"`
	Languages              []string                         `json:"languages"`
	Rules                  []string                         `json:"rules"`
	Consumers              []string                         `json:"consumers"`
	ClosedWorld            []string                         `json:"closed_world"`
	EdgeEvaluations        []Evaluation                     `json:"edge_evaluations"`
	TypeErrorSkips         []string                         `json:"type_error_skips"`
	Notes                  []Note                           `json:"notes"`
	ConventionsApplied     []Convention                     `json:"conventions_applied"`
	EntryFiles             []string                         `json:"entry_files"`
	DisabledConventions    []string                         `json:"disabled_conventions"`
	Expect                 []Row                            `json:"expect"`
}

// The facts a closed_world member may name.
const (
	WorldConsumers = "consumers"
	WorldMatrix    = "matrix"
	WorldRoots     = "roots"
)

// Note is one notes entry: the note's kind and the directory it names.
type Note struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Convention is one conventions_applied entry.
type Convention struct {
	Name     string `json:"name"`
	Package  string `json:"package"`
	Version  string `json:"version"`
	Manifest string `json:"manifest"`
}

func (c Convention) String() string {
	return fmt.Sprintf("%s (%s %s, %s)", c.Name, c.Package, c.Version, c.Manifest)
}

// SetupFailure is the failure a fixture's run must end with.
type SetupFailure struct {
	Class string   `json:"class"`
	Names []string `json:"names"`
}

// ConfiguredDeclaration binds a logical name to the configuration key the runner
// writes its entry into; Members is a lifecycle contract's member list.
type ConfiguredDeclaration struct {
	Entry   map[string]string `json:"entry"`
	Key     string            `json:"key"`
	Members []string          `json:"members"`
}

// Ref is the subject reference a finding about the entry carries: a symbol
// entry's reference, a module entry's module and name joined by a number sign,
// and a global entry's path after one.
func (d ConfiguredDeclaration) Ref() string {
	if symbol, ok := d.Entry["symbol"]; ok {
		return symbol
	}
	if module, ok := d.Entry["module"]; ok {
		return module + "#" + d.Entry["name"]
	}
	return "#" + d.Entry["global"]
}

// Evaluation is one edge_evaluations entry.
type Evaluation struct {
	Edge   string `json:"edge"`
	Side   string `json:"side"`
	State  string `json:"state"`
	Symbol string `json:"symbol"`
	Report string `json:"report"`
}

// Row is one expectation; a member it omits states nothing and is not compared.
type Row struct {
	Details           *Details   `json:"details"`
	Component         *Component `json:"component"`
	Symbol            string     `json:"symbol"`
	Report            string     `json:"report"`
	SymbolKind        string     `json:"symbol_kind"`
	Confidence        string     `json:"confidence"`
	ReachabilityClass string     `json:"reachability_class"`
	LivenessRelation  string     `json:"liveness_relation"`
	Fixability        string     `json:"fixability"`
	Configurations    []string   `json:"configurations"`
	RetainedBy        []string   `json:"retained_by"`
}

// Details is the details members a row pins, one field per member the
// expectation schema admits, in the schema's member order.
type Details struct {
	NarrowerVisibility string   `json:"narrower_visibility,omitempty"`
	ExcludedBy         string   `json:"excluded_by,omitempty"`
	DependencyClass    string   `json:"dependency_class,omitempty"`
	Replacement        string   `json:"replacement,omitempty"`
	Mechanism          string   `json:"mechanism,omitempty"`
	Edge               string   `json:"edge,omitempty"`
	NameLiteral        string   `json:"name_literal,omitempty"`
	Overlap            []string `json:"overlap,omitempty"`
}

// Named lists the members the details pin, sorted.
func (d *Details) Named() []string {
	if d == nil {
		return nil
	}
	var out []string
	for _, member := range []struct {
		name  string
		named bool
	}{
		{"dependency_class", d.DependencyClass != ""},
		{"edge", d.Edge != ""},
		{"excluded_by", d.ExcludedBy != ""},
		{"mechanism", d.Mechanism != ""},
		{"name_literal", d.NameLiteral != ""},
		{"narrower_visibility", d.NarrowerVisibility != ""},
		{"overlap", len(d.Overlap) > 0},
		{"replacement", d.Replacement != ""},
	} {
		if member.named {
			out = append(out, member.name)
		}
	}
	return out
}

// Component is the component members a row pins. A pointer tells a member the
// row omits apart from a false root or a count of zero.
type Component struct {
	Root           *bool `json:"root,omitempty"`
	SymbolCount    *int  `json:"symbol_count,omitempty"`
	DeletableLines *int  `json:"deletable_lines,omitempty"`
}

// TargetKind is target_kind in either shape target.kind takes: one kind for every
// language, or PerLanguage naming the kind of each.
type TargetKind struct {
	PerLanguage map[string]string
	One         string
}

// UnmarshalJSON reads either shape of target_kind.
func (k *TargetKind) UnmarshalJSON(data []byte) error {
	*k = TargetKind{}
	if err := json.Unmarshal(data, &k.One); err == nil {
		return nil
	}
	return json.Unmarshal(data, &k.PerLanguage)
}

// MarshalJSON writes the shape the kind was read in.
func (k TargetKind) MarshalJSON() ([]byte, error) {
	if k.PerLanguage != nil {
		return json.Marshal(k.PerLanguage)
	}
	return json.Marshal(k.One)
}

// Manifest is a rendering's fixture.json.
type Manifest struct {
	Symbols                map[string]ManifestPosition `json:"symbols"`
	Configurations         []string                    `json:"configurations"`
	ConfigurationsNotBuilt []string                    `json:"configurations_not_built"`
}

// ManifestPosition is one logical name's file and line; Line is a pointer so an
// absent line is told apart from line zero.
type ManifestPosition struct {
	Line *int   `json:"line"`
	File string `json:"file"`
}

// Site is one position as an expectation and a finding are compared at: a path
// from the rendering root and a line.
type Site struct {
	File string
	Line int
}

func (s Site) String() string { return fmt.Sprintf("%s:%d", s.File, s.Line) }

// DecodeStrict decodes one JSON document, refusing a member the target type does
// not declare and anything after the document.
func DecodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("data after the document")
	}
	return nil
}
