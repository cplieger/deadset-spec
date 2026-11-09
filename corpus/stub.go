package corpus

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// stubSource is the stub program a case's stubs run (stub/main.go), built from
// this copy so a run needs no checkout of the module and no network.
//
//go:embed stub/main.go
var stubSource []byte

// stubDocumentSuffix names the stub document beside a stub's copy of the program,
// which the program reads from its own path.
const stubDocumentSuffix = ".stub.json"

// executableSuffix is what the host's process start needs an executable's name to
// end in: Windows finds a command named without one by PATHEXT, which lists .exe.
var executableSuffix = map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]

// stubDocument is the shape of a stub document, the type stub/main.go decodes it
// into, so a case's document is refused before the product starts it.
type stubDocument struct {
	Describe *stubArm  `json:"describe"`
	Arms     []stubArm `json:"arms"`
}

type stubArm struct {
	When     *stubCondition    `json:"when"`
	Identity *stubIdentity     `json:"identity"`
	Writes   map[string]string `json:"writes"`
	Stdout   string            `json:"stdout"`
	Touch    string            `json:"touch"`
	Stderr   []string          `json:"stderr"`
	Sleep    int               `json:"sleep_seconds"`
	Exit     int               `json:"exit"`
}

type stubCondition struct {
	Flag          string `json:"flag"`
	ScopeContains string `json:"scope_contains"`
}

type stubIdentity struct {
	Default   string `json:"default"`
	FromScope bool   `json:"from_scope"`
}

// checkStub refuses a stub document the program could not run: one it would not
// decode, or one with no arm to answer an invocation.
func checkStub(data []byte) error {
	var doc stubDocument
	if err := DecodeStrict(data, &doc); err != nil {
		return err
	}
	if len(doc.Arms) == 0 {
		return errors.New("the stub document has no arm")
	}
	for _, arm := range doc.Arms {
		if arm.When != nil && (arm.When.Flag == "") == (arm.When.ScopeContains == "") {
			return errors.New("a stub arm's condition names exactly one of flag and scope_contains")
		}
	}
	return nil
}

// stubBuild is the stub program one RunCases call builds on its first case that
// needs it, in a directory removed when the call ends.
type stubBuild struct {
	err  error
	dir  string
	path string
	once sync.Once
}

// program builds the stub program on the first call and returns its path.
func (b *stubBuild) program(ctx context.Context) (string, error) {
	b.once.Do(func() {
		if b.dir, b.err = os.MkdirTemp("", "deadset-stub-"); b.err != nil {
			return
		}
		module := []byte("module deadsetstub\n\ngo 1.22\n")
		if b.err = os.WriteFile(filepath.Join(b.dir, "go.mod"), module, 0o600); b.err != nil {
			return
		}
		if b.err = os.WriteFile(filepath.Join(b.dir, "main.go"), stubSource, 0o600); b.err != nil {
			return
		}
		b.path = filepath.Join(b.dir, "stub"+executableSuffix)
		build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", b.path, ".") //nolint:gosec // into a directory this run created
		build.Dir = b.dir
		build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOTOOLCHAIN=local")
		var out bytes.Buffer
		build.Stdout, build.Stderr = &out, &out
		if err := build.Run(); err != nil {
			b.err = fmt.Errorf("build the stub program: %w: %s", err, out.String())
		}
	})
	return b.path, b.err
}

// remove deletes the directory the program was built in.
func (b *stubBuild) remove() {
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
	}
}

// copyProgram copies the stub program to file, executable.
func copyProgram(program, file string) error {
	in, err := os.Open(program)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
