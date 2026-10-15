package render

import (
	"encoding/json"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// TestVerdictOfEveryMergedVectorIsItsPublishedExitCode pins the verdict over every
// merged report the vectors publish, under the failing severity its caller
// gives, to the exit code the case declares.
func TestVerdictOfEveryMergedVectorIsItsPublishedExitCode(t *testing.T) {
	t.Parallel()

	merged, err := fs.Glob(spec.Vectors, "vectors/merge/*/expected.json")
	if err != nil || len(merged) == 0 {
		t.Fatalf("Setup: glob vectors/merge/*/expected.json = %v, %v, want the merged cases", merged, err)
	}
	for _, expected := range merged {
		dir := path.Dir(expected)
		t.Run(path.Base(dir), func(t *testing.T) {
			t.Parallel()

			body, err := spec.Vectors.ReadFile(path.Join(dir, "expected_exit"))
			if err != nil {
				t.Fatalf("Setup: read %s/expected_exit: %v", dir, err)
			}
			want, err := strconv.Atoi(strings.TrimSpace(string(body)))
			if err != nil {
				t.Fatalf("Setup: %s/expected_exit holds %q, not an exit code", dir, body)
			}

			caller, err := spec.Vectors.ReadFile(path.Join(dir, "caller.json"))
			if err != nil {
				t.Fatalf("Setup: read %s/caller.json: %v", dir, err)
			}
			var facts struct {
				FailOn vocab.Severity `json:"fail_on"`
			}
			if err := json.Unmarshal(caller, &facts); err != nil {
				t.Fatalf("Setup: decode %s/caller.json: %v", dir, err)
			}

			r := decodeVector(t, expected)
			if got := Verdict(&r.Totals, facts.FailOn); got != want {
				t.Errorf("Verdict(%s, %s) = %d, want %d", expected, facts.FailOn, got, want)
			}
		})
	}
}
