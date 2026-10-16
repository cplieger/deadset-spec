package document

import "testing"

func TestARefusalOfAMemberNamesTheMember(t *testing.T) {
	t.Parallel()

	schema, err := CompileBytes("value", []byte(`{
		"type": "object", "additionalProperties": false, "required": ["kind"],
		"properties": {"kind": {"type": "string"}, "codes": {"type": "object", "propertyNames": {"enum": ["DS10"]}}}
	}`))
	if err != nil {
		t.Fatalf("Setup: CompileBytes = %v", err)
	}
	value, err := Decode([]byte(`{"codes": {"DS99": 1}, "extra": true}`))
	if err != nil {
		t.Fatalf("Setup: Decode = %v", err)
	}
	got := map[string]Error{}
	for _, refused := range schema.CheckValue(value) {
		got[refused.Keyword] = Error{Pointer: refused.Pointer, Member: refused.Member}
	}
	want := map[string]Error{
		"propertyNames":        {Pointer: "/codes", Member: "DS99"},
		"additionalProperties": {Pointer: "", Member: "extra"},
		"required":             {Pointer: "", Member: "kind"},
	}
	for keyword, at := range want {
		if got[keyword] != at {
			t.Errorf("CheckValue: the %s refusal is at %+v, want %+v", keyword, got[keyword], at)
		}
	}
}

// entrySchema is a list whose entries take one of two object shapes, the shape
// of a build matrix.
const entrySchema = `{"type": "array", "items": {"oneOf": [
	{"type": "object", "additionalProperties": false, "required": ["id", "os"],
	 "properties": {"id": {"type": "string"}, "os": {"type": "string", "minLength": 1}}},
	{"type": "object", "additionalProperties": false, "required": ["id", "project"],
	 "properties": {"id": {"type": "string"}, "project": {"type": "string"}}}
]}}`

func TestFirstRefusalNamesWhatAReaderFixesFirst(t *testing.T) {
	t.Parallel()

	schema, err := CompileBytes("value", []byte(`{
		"type": "object", "additionalProperties": false, "required": ["kind"],
		"properties": {"kind": {"type": "string"}, "a": {"type": "string", "enum": ["x"]}}
	}`))
	if err != nil {
		t.Fatalf("Setup: CompileBytes = %v", err)
	}
	for _, tc := range []struct {
		name, document string
		want           Error
	}{
		{name: "an undeclared member before a missing one", document: `{"a": "y", "z": 1}`, want: Error{Pointer: "", Keyword: "additionalProperties", Member: "z"}},
		{name: "a missing member before a value inside the object", document: `{"a": "y"}`, want: Error{Pointer: "", Keyword: "required", Member: "kind"}},
		{name: "a value refused", document: `{"kind": "k", "a": "y"}`, want: Error{Pointer: "/a", Keyword: "enum"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value, err := Decode([]byte(tc.document))
			if err != nil {
				t.Fatalf("Setup: Decode = %v", err)
			}
			got := schema.FirstRefusal(value)
			if got == nil || got.Pointer != tc.want.Pointer || got.Keyword != tc.want.Keyword || got.Member != tc.want.Member {
				t.Errorf("FirstRefusal(%s) = %+v, want %+v", tc.document, got, tc.want)
			}
		})
	}
}

func TestFirstRefusalNamesAnUndeclaredMemberBeforeTheObjectsOwnValue(t *testing.T) {
	t.Parallel()

	schema, err := CompileBytes("value", []byte(`{
		"type": "object", "enum": [{"kind": "k"}], "additionalProperties": false,
		"properties": {"kind": {"type": "string"}}
	}`))
	if err != nil {
		t.Fatalf("Setup: CompileBytes = %v", err)
	}
	value, err := Decode([]byte(`{"z": 1}`))
	if err != nil {
		t.Fatalf("Setup: Decode = %v", err)
	}
	if got := schema.FirstRefusal(value); got == nil || got.Keyword != "additionalProperties" || got.Member != "z" {
		t.Errorf("FirstRefusal = %+v, want the undeclared member z", got)
	}
}

func TestFirstRefusalExplainsAOneOfByTheAlternativeTheValueWasWrittenFor(t *testing.T) {
	t.Parallel()

	schema, err := CompileBytes("value", []byte(entrySchema))
	if err != nil {
		t.Fatalf("Setup: CompileBytes = %v", err)
	}
	for _, tc := range []struct {
		name, document string
		want           Error
	}{
		{name: "the alternative declaring the most members", document: `[{"id": "a", "os": ""}]`, want: Error{Pointer: "/0/os", Keyword: "minLength"}},
		{name: "a member it leaves out", document: `[{"id": "a", "os": "linux"}, {"project": "p"}]`, want: Error{Pointer: "/1", Keyword: "required", Member: "id"}},
		{name: "no alternative closer than another", document: `[{"id": "a"}]`, want: Error{Pointer: "/0", Keyword: "oneOf"}},
		{name: "no alternative of the value's type", document: `["a"]`, want: Error{Pointer: "/0", Keyword: "oneOf"}},
		{name: "an instance", document: `[{"id": "a", "project": "p"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value, err := Decode([]byte(tc.document))
			if err != nil {
				t.Fatalf("Setup: Decode = %v", err)
			}
			got := schema.FirstRefusal(value)
			if tc.want.Keyword == "" {
				if got != nil {
					t.Errorf("FirstRefusal(%s) = %+v, want none", tc.document, got)
				}
				return
			}
			if got == nil || got.Pointer != tc.want.Pointer || got.Keyword != tc.want.Keyword || got.Member != tc.want.Member {
				t.Errorf("FirstRefusal(%s) = %+v, want %+v", tc.document, got, tc.want)
			}
		})
	}
}

func TestFirstRefusalExplainsAOneOfAWrapperApplies(t *testing.T) {
	t.Parallel()

	const shapes = `"$defs": {"shapes": {"oneOf": [
		{"type": "object", "additionalProperties": false, "required": ["a"], "properties": {"a": {"type": "string", "minLength": 1}}},
		{"type": "object", "additionalProperties": false, "required": ["b"], "properties": {"b": {"type": "integer"}}}
	]}}`
	for _, tc := range []struct {
		name, schema, document string
		want                   Error
	}{
		{name: "behind a $ref at the root", schema: `{"$ref": "#/$defs/shapes", ` + shapes + `}`, document: `{"a": ""}`, want: Error{Pointer: "/a", Keyword: "minLength"}},
		{name: "behind a $ref of a member", schema: `{"type": "object", "properties": {"m": {"$ref": "#/$defs/shapes"}}, ` + shapes + `}`, document: `{"m": {"a": ""}}`, want: Error{Pointer: "/m/a", Keyword: "minLength"}},
		{name: "in an allOf", schema: `{"allOf": [{"$ref": "#/$defs/shapes"}], ` + shapes + `}`, document: `{"b": "x"}`, want: Error{Pointer: "/b", Keyword: "type"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			schema, err := CompileBytes("value", []byte(tc.schema))
			if err != nil {
				t.Fatalf("Setup: CompileBytes = %v", err)
			}
			value, err := Decode([]byte(tc.document))
			if err != nil {
				t.Fatalf("Setup: Decode = %v", err)
			}
			got := schema.FirstRefusal(value)
			if got == nil || got.Pointer != tc.want.Pointer || got.Keyword != tc.want.Keyword {
				t.Errorf("FirstRefusal(%s) = %+v, want %+v", tc.document, got, tc.want)
			}
		})
	}
}

func TestFirstRefusalReadsAnAlternativeWithTheNodesItApplies(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, schema, document string
		want                   Error
	}{
		{
			name:     "a type behind a $ref",
			schema:   `{"oneOf": [{"$ref": "#/$defs/text"}, {"type": "number", "minimum": 5}], "$defs": {"text": {"type": "string"}}}`,
			document: `3`, want: Error{Keyword: "minimum"},
		},
		{
			name: "members declared in an allOf",
			schema: `{"oneOf": [
				{"allOf": [{"type": "object", "required": ["a"], "properties": {"a": {"type": "string"}, "b": {"type": "integer"}}}]},
				{"type": "object", "required": ["c"], "properties": {"a": {"type": "string"}, "c": {"type": "string"}}}
			]}`,
			document: `{"a": "x", "b": "y"}`, want: Error{Pointer: "/b", Keyword: "type"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			schema, err := CompileBytes("value", []byte(tc.schema))
			if err != nil {
				t.Fatalf("Setup: CompileBytes = %v", err)
			}
			value, err := Decode([]byte(tc.document))
			if err != nil {
				t.Fatalf("Setup: Decode = %v", err)
			}
			got := schema.FirstRefusal(value)
			if got == nil || got.Pointer != tc.want.Pointer || got.Keyword != tc.want.Keyword {
				t.Errorf("FirstRefusal(%s) = %+v, want %+v", tc.document, got, tc.want)
			}
		})
	}
}

func TestFirstRefusalExplainsTheOneOfThatRefusedTheValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		referenced string
		refusals   int
		want       string
	}{
		{"both refuse, the referenced first", `{"type": "number", "minimum": 10}`, 2, "minimum"},
		{"the referenced holds, the node's own refuses", `{"type": "number", "minimum": 0}`, 1, "maximum"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema, err := CompileBytes("value", []byte(`{
				"$ref": "#/$defs/referenced",
				"oneOf": [{"type": "number", "maximum": 0}, {"type": "string"}],
				"$defs": {"referenced": {"oneOf": [`+tc.referenced+`, {"type": "string"}]}}
			}`))
			if err != nil {
				t.Fatalf("Setup: CompileBytes = %v", err)
			}
			value, err := Decode([]byte(`3`))
			if err != nil {
				t.Fatalf("Setup: Decode = %v", err)
			}
			found := schema.CheckValue(value)
			if len(found) != tc.refusals || found[0].Keyword != oneOfKeyword {
				t.Fatalf("Setup: CheckValue(3) = %+v, want %d oneOf refusals", found, tc.refusals)
			}
			if got := schema.FirstRefusal(value); got == nil || got.Keyword != tc.want {
				t.Errorf("FirstRefusal(3) = %+v, want the %s refusal explaining CheckValue's first oneOf", got, tc.want)
			}
		})
	}
}
