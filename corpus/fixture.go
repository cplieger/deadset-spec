package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"golang.org/x/tools/txtar"
)

// Fixture is one fixture of the corpus: its expectation file and the files of
// each rendering it holds, keyed by language.
type Fixture struct {
	Renderings map[string]*Rendering
	Name       string
	Expect     Expectation
}

// Rendering is one language's rendering of a fixture: every file by its path
// from the rendering root, and the manifest among them decoded.
type Rendering struct {
	Files    map[string][]byte
	Language string
	Manifest Manifest
}

var errDuplicateSection = errors.New("txtar section declared twice")

// ReadDocument decodes corpus/corpus.json from a tree holding the corpus.
func ReadDocument(fsys fs.FS) (*Document, error) {
	data, err := fs.ReadFile(fsys, DocumentPath)
	if err != nil {
		return nil, err
	}
	var doc Document
	if err := DecodeStrict(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", DocumentPath, err)
	}
	return &doc, nil
}

// Names is every fixture directory of the corpus, in name order.
func Names(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, FixturesDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// Load reads every fixture of the corpus, in name order, failing on the first
// one that does not read; Check names every defect instead.
func Load(fsys fs.FS) ([]Fixture, error) {
	doc, err := ReadDocument(fsys)
	if err != nil {
		return nil, err
	}
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	fixtures := make([]Fixture, 0, len(names))
	for _, name := range names {
		fixture, err := LoadFixture(fsys, doc, name)
		if err != nil {
			return nil, fmt.Errorf("fixture %s: %w", name, err)
		}
		fixtures = append(fixtures, *fixture)
	}
	return fixtures, nil
}

// LoadFixture reads one fixture: its expectation file strictly, and the rendering
// of every language the expectation file lists in the form the layout states.
func LoadFixture(fsys fs.FS, doc *Document, name string) (*Fixture, error) {
	dir := path.Join(FixturesDir, name)
	data, err := fs.ReadFile(fsys, path.Join(dir, ExpectFile))
	if err != nil {
		return nil, err
	}
	fixture := &Fixture{Name: name, Renderings: map[string]*Rendering{}}
	if err := DecodeStrict(data, &fixture.Expect); err != nil {
		return nil, fmt.Errorf("%s: %w", ExpectFile, err)
	}
	for _, language := range fixture.Expect.Languages {
		rendering, err := readRendering(fsys, dir, language, doc.Layout.Renderings[language])
		if err != nil {
			return nil, fmt.Errorf("rendering %q: %w", language, err)
		}
		fixture.Renderings[language] = rendering
	}
	return fixture, nil
}

// readRendering reads one rendering's files and decodes its manifest.
func readRendering(fsys fs.FS, dir, language, form string) (*Rendering, error) {
	files, err := renderingFiles(fsys, dir, form)
	if err != nil {
		return nil, err
	}
	rendering := &Rendering{Language: language, Files: files}
	data, ok := files[ManifestFile]
	if !ok {
		return nil, fmt.Errorf("no %s", ManifestFile)
	}
	if err := DecodeStrict(data, &rendering.Manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	return rendering, nil
}

// renderingFiles reads one rendering in its form: the files below a directory
// for a form ending in a slash, the sections of a txtar archive otherwise.
func renderingFiles(fsys fs.FS, dir, form string) (map[string][]byte, error) {
	if form == "" {
		return nil, errors.New("the corpus states no rendering form for the language")
	}
	if root, isDir := strings.CutSuffix(form, "/"); isDir {
		root = path.Join(dir, root)
		files := map[string][]byte{}
		err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files[strings.TrimPrefix(p, root+"/")], err = fs.ReadFile(fsys, p)
			return err
		})
		return files, err
	}
	data, err := fs.ReadFile(fsys, path.Join(dir, form))
	if err != nil {
		return nil, err
	}
	archive := txtar.Parse(data)
	files := make(map[string][]byte, len(archive.Files))
	for _, file := range archive.Files {
		if _, dup := files[file.Name]; dup {
			return nil, fmt.Errorf("%w: %q", errDuplicateSection, file.Name)
		}
		files[file.Name] = file.Data
	}
	return files, nil
}

// Sorted is the rendering's file paths in bytewise order.
func (r *Rendering) Sorted() []string {
	names := make([]string, 0, len(r.Files))
	for name := range r.Files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
