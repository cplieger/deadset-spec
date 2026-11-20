package spec_test

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// TestEveryConfigKeysDefaultIsAnInstanceOfItsValue pins the key table's value
// column to its default column: each value schema compiles with
// document.CompileBytes, admits the key's own default, and for a keyed key admits every
// vocabulary value as a member name and the default's every member.
func TestEveryConfigKeysDefaultIsAnInstanceOfItsValue(t *testing.T) {
	for _, key := range vocab.ConfigKeys() {
		if key.Role == vocab.RoleSection {
			continue
		}
		t.Run(strings.ReplaceAll(string(key.Key), ".", "_"), func(t *testing.T) {
			schema, err := document.CompileBytes(string(key.Key), []byte(key.Value))
			if err != nil {
				t.Fatalf("compiling %s's value schema = %v", key.Key, err)
			}
			if key.Default == "" {
				if !key.Required && key.Role == vocab.RoleSetting {
					t.Errorf("%s has no default and is not required, want one of the two", key.Key)
				}
				return
			}
			def, err := document.Decode([]byte(key.Default))
			if err != nil {
				t.Fatalf("decoding %s's default = %v", key.Key, err)
			}
			if key.Role != vocab.RoleKeyed {
				if refused := schema.CheckValue(def); len(refused) != 0 {
					t.Errorf("%s's default %s is refused by its value schema: %v", key.Key, key.Default, refused)
				}
				return
			}
			pattern := regexp.MustCompile(key.KeyPattern)
			for _, name := range key.Values {
				if !pattern.MatchString(name) {
					t.Errorf("%s admits %q, which its key pattern %s refuses", key.Key, name, key.KeyPattern)
				}
			}
			members, _ := def.(map[string]any)
			for name, member := range members {
				if refused := schema.CheckValue(member); len(refused) != 0 {
					t.Errorf("%s's default member %s is refused by its value schema: %v", key.Key, name, refused)
				}
			}
		})
	}
}

// documentCap is the one bound config, scope, ignore, baseline and edges
// documents share.
const documentCap = 16 << 20

func TestOneDocumentCapOf16MiB(t *testing.T) {
	var schema struct {
		MaxBytes int `json:"x-max-bytes"`
	}
	var settings struct {
		MaxBytes int `json:"max_bytes"`
	}
	for path, into := range map[string]any{"contract/config.schema.json": &schema, "contract/settings.json": &settings} {
		data, err := fs.ReadFile(spec.Contract, path)
		if err != nil {
			t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", path, err)
		}
		if err := json.Unmarshal(data, into); err != nil {
			t.Fatalf("Setup: decode %q: %v", path, err)
		}
	}
	for what, got := range map[string]int{
		"config.schema.json x-max-bytes": schema.MaxBytes,
		"settings.json max_bytes":        settings.MaxBytes,
		"vocab.DocumentMaxBytes":         vocab.DocumentMaxBytes,
	} {
		if got != documentCap {
			t.Errorf("%s = %d, want %d (16 MiB)", what, got, documentCap)
		}
	}
}
