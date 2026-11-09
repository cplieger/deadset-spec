// Command stub stands in for a program a command-line vector's product starts: an
// analyzer, or the toolchain. The conformance runner builds it, copies it to each
// path a case lists in stubs, and writes the case's stub document beside the copy
// as <path>.stub.json. Each invocation runs the first arm of that document whose
// condition holds: the describe arm for the describe verb where the document has
// one, else the first of its arms.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// document is a stub document. Its members are the ones the runner's own copy of
// the type decodes (corpus.stubDocument), which a corpus test holds equal.
type document struct {
	Describe *arm  `json:"describe"`
	Arms     []arm `json:"arms"`
}

// arm is what one invocation does, in this order: print the standard error lines,
// copy each file a flag of writes names to the flag's value, print the stdout file,
// create the touch file, sleep, and exit with exit. Every file name is relative to
// the stub's directory.
type arm struct {
	When     *condition        `json:"when"`
	Identity *identity         `json:"identity"`
	Writes   map[string]string `json:"writes"`
	Stdout   string            `json:"stdout"`
	Touch    string            `json:"touch"`
	Stderr   []string          `json:"stderr"`
	Sleep    int               `json:"sleep_seconds"`
	Exit     int               `json:"exit"`
}

// condition is when an arm runs: an argument --<flag>=... is given, or the scope
// document --scope= names has a path containing scope_contains.
type condition struct {
	Flag          string `json:"flag"`
	ScopeContains string `json:"scope_contains"`
}

// identity replaces the placeholder identity of every file the arm writes with the
// target id of the scope document the invocation names, where from_scope asks for
// it and the document states one, else with default.
type identity struct {
	Default   string `json:"default"`
	FromScope bool   `json:"from_scope"`
}

// placeholder is the identity member a written file carries for identity to fill.
const placeholder = `"identity": "IDENTITY"`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	exe, err := os.Executable()
	if err != nil {
		return refuse("stub: locate the executable: %v", err)
	}
	dir := filepath.Dir(exe)
	data, err := os.ReadFile(strings.TrimSuffix(exe, ".exe") + ".stub.json")
	if err != nil {
		return refuse("stub: read the stub document: %v", err)
	}
	var doc document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return refuse("stub: decode the stub document: %v", err)
	}
	if doc.Describe != nil && len(args) > 0 && args[0] == "describe" {
		return answer(doc.Describe, dir, args, "")
	}
	for i := range doc.Arms {
		if matched, ok := holds(doc.Arms[i].When, args); ok {
			return answer(&doc.Arms[i], dir, args, matched)
		}
	}
	return refuse("stub: no arm answers %q", args)
}

// holds reports whether c holds for args, and the argument a flag condition matched.
func holds(c *condition, args []string) (string, bool) {
	switch {
	case c == nil:
		return "", true
	case c.Flag != "":
		for _, arg := range args {
			if strings.HasPrefix(arg, "--"+c.Flag+"=") {
				return arg, true
			}
		}
		return "", false
	default:
		scope, _ := flagValue(args, "scope")
		return "", strings.Contains(filepath.ToSlash(scope), c.ScopeContains)
	}
}

// answer runs one arm; {arg} in a standard error line is the argument its flag
// condition matched.
func answer(a *arm, dir string, args []string, matched string) int {
	for _, line := range a.Stderr {
		fmt.Fprintln(os.Stderr, strings.ReplaceAll(line, "{arg}", matched))
	}
	if err := a.write(dir, args); err != nil {
		return refuse("stub: %v", err)
	}
	if a.Stdout != "" {
		body, err := os.ReadFile(filepath.Join(dir, a.Stdout))
		if err == nil {
			_, err = os.Stdout.Write(body)
		}
		if err != nil {
			return refuse("stub: print %s: %v", a.Stdout, err)
		}
	}
	if a.Touch != "" {
		if err := os.WriteFile(a.Touch, nil, 0o600); err != nil {
			return refuse("stub: touch %s: %v", a.Touch, err)
		}
	}
	time.Sleep(time.Duration(a.Sleep) * time.Second)
	return a.Exit
}

// write copies each file of writes to the path its flag's argument gives.
func (a *arm) write(dir string, args []string) error {
	for flag, name := range a.Writes {
		to, given := flagValue(args, flag)
		if !given {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if a.Identity != nil {
			body = bytes.ReplaceAll(body, []byte(placeholder), []byte(`"identity": "`+a.Identity.of(args)+`"`))
		}
		if err := os.WriteFile(to, body, 0o600); err != nil { //nolint:gosec // the path the product hands the program it starts
			return err
		}
	}
	return nil
}

// of is the identity the arm writes.
func (id *identity) of(args []string) string {
	scope, given := flagValue(args, "scope")
	if !id.FromScope || !given {
		return id.Default
	}
	var doc struct {
		Target struct {
			ID string `json:"id"`
		} `json:"target"`
	}
	//nolint:gosec // the scope document the product hands the program it starts
	if data, err := os.ReadFile(scope); err == nil && json.Unmarshal(data, &doc) == nil && doc.Target.ID != "" {
		return doc.Target.ID
	}
	return id.Default
}

// flagValue is the value of the argument --<name>=value, and whether args holds one.
func flagValue(args []string, name string) (string, bool) {
	for _, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--"+name+"="); ok {
			return value, true
		}
	}
	return "", false
}

// refuse prints why the stub cannot answer and exits with a code no case expects.
func refuse(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 125
}
