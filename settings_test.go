package spec_test

import (
	"regexp"
	"strings"
	"testing"

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
