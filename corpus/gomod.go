package corpus

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// The defects of a Go rendering's module wiring.
var (
	ErrNoModule            = errors.New("go rendering module file declares no module path")
	ErrNoRequire           = errors.New("go rendering consumer requires no target module")
	ErrNoReplace           = errors.New("go rendering consumer replaces the target module with no relative path")
	ErrNoDependencyRequire = errors.New("go rendering target requires no dependency module")
	ErrNoDependencyReplace = errors.New("go rendering target replaces the dependency module with no relative path")
	ErrWorkspaceNoTarget   = errors.New("go rendering workspace file lists no ./target use directive")
	ErrWorkspaceUse        = errors.New("go rendering workspace file lists a use directive that is no dependency directory at the rendering root")
)

const (
	goModFile  = "go.mod"
	goWorkFile = "go.work"
)

// goModule is what the checks read from one module file.
type goModule struct {
	replaces map[string]string
	path     string
	requires []string
}

// readGoModule reads a module file's own path, the paths it requires and the new
// path each replace directive names; an absent or unparsable file reads as none.
func readGoModule(name string, data []byte) goModule {
	held := goModule{replaces: map[string]string{}}
	file, err := modfile.Parse(name, data, nil)
	if err != nil {
		return held
	}
	if file.Module != nil {
		held.path = file.Module.Mod.Path
	}
	for _, require := range file.Require {
		held.requires = append(held.requires, require.Mod.Path)
	}
	for _, replace := range file.Replace {
		held.replaces[replace.Old.Path] = replace.New.Path
	}
	return held
}

// topLevelDirectories is every directory at the rendering root, sorted.
func topLevelDirectories(files map[string][]byte) []string {
	held := map[string]bool{}
	for p := range files {
		if dir, _, nested := strings.Cut(p, "/"); nested {
			held[dir] = true
		}
	}
	return slices.Sorted(maps.Keys(held))
}

// checkGoDependencyModules holds a dependency directory, one at the rendering
// root that is neither the target nor a consumer, to a module the target requires
// and replaces with the directory's path, so the rendering loads with no network.
// A dependency the root go.work lists beside the target needs no replace.
func checkGoDependencyModules(consumers []string, files map[string][]byte) []error {
	dependencies := slices.DeleteFunc(topLevelDirectories(files), func(dir string) bool {
		return dir == TargetDir || slices.Contains(consumers, dir)
	})
	workspace, errs := checkGoWorkspace(dependencies, files)
	if len(dependencies) == 0 {
		return errs
	}
	targetMod := path.Join(TargetDir, goModFile)
	target := readGoModule(targetMod, files[targetMod])
	for _, dependency := range dependencies {
		file := path.Join(dependency, goModFile)
		module := readGoModule(file, files[file]).path
		if module == "" {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNoModule, file))
			continue
		}
		if !slices.Contains(target.requires, module) {
			errs = append(errs, fmt.Errorf("%w: %s names no require of %q", ErrNoDependencyRequire, targetMod, module))
		}
		if workspace[dependency] {
			continue
		}
		if want, got := "../"+dependency, target.replaces[module]; got != want {
			errs = append(errs, fmt.Errorf("%w: %s replaces %q with %q, want %q", ErrNoDependencyReplace, targetMod, module, got, want))
		}
	}
	return errs
}

// checkGoWorkspace reads the rendering root's go.work and returns the dependency
// directories it resolves: every one it lists when it lists the target too. A use
// naming anything but the target or a dependency directory is an error, a
// consumer among them, because a workspace member is a dependency.
func checkGoWorkspace(dependencies []string, files map[string][]byte) (map[string]bool, []error) {
	data, held := files[goWorkFile]
	if !held {
		return nil, nil
	}
	var uses []string
	if work, err := modfile.ParseWork(goWorkFile, data, nil); err == nil {
		for _, use := range work.Use {
			uses = append(uses, use.Path)
		}
	}
	listed := map[string]bool{}
	var errs []error
	for _, use := range uses {
		dir := path.Clean(use)
		if path.IsAbs(use) || strings.Contains(dir, "/") || (dir != TargetDir && !slices.Contains(dependencies, dir)) {
			errs = append(errs, fmt.Errorf("%w: %s uses %q", ErrWorkspaceUse, goWorkFile, use))
			continue
		}
		listed[dir] = true
	}
	if !listed[TargetDir] {
		return nil, append(errs, fmt.Errorf("%w: %s", ErrWorkspaceNoTarget, goWorkFile))
	}
	return listed, errs
}

// checkGoConsumerModules holds each consumer module to a require of the target
// module and a replace naming the target's path from the consumer directory.
func checkGoConsumerModules(consumers []string, files map[string][]byte) []error {
	targetMod := path.Join(TargetDir, goModFile)
	module := readGoModule(targetMod, files[targetMod]).path
	if module == "" {
		return []error{fmt.Errorf("%w: %s", ErrNoModule, targetMod)}
	}
	var errs []error
	for _, consumer := range consumers {
		file := path.Join(consumer, goModFile)
		data, held := files[file]
		if !held {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNoModule, file))
			continue
		}
		read := readGoModule(file, data)
		if !slices.Contains(read.requires, module) {
			errs = append(errs, fmt.Errorf("%w: %s names no require of %q", ErrNoRequire, file, module))
		}
		if want, got := "../"+TargetDir, read.replaces[module]; got != want {
			errs = append(errs, fmt.Errorf("%w: %s replaces %q with %q, want %q", ErrNoReplace, file, module, got, want))
		}
	}
	return errs
}
