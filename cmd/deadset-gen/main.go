// Command deadset-gen writes the files of this module derived from the
// Contract's data, run from the module root by go generate.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/cplieger/atomicfile/v4"
	"github.com/cplieger/deadset-spec/v8/internal/gen"
)

func main() {
	root := flag.String("root", ".", "the module root")
	baseline := flag.Bool("baseline", false, "also rewrite testdata/contract-baseline.json from the documents on disk, the release snapshot")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, *root, *baseline)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "deadset-gen:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, root string, baseline bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	fsys := os.DirFS(root)
	files, err := gen.Generate(fsys)
	if err != nil {
		return err
	}
	if baseline {
		data, err := gen.Baseline(fsys)
		if err != nil {
			return err
		}
		files[gen.BaselinePath] = data
	}
	for _, name := range gen.Paths(files) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := atomicfile.WriteFile(ctx, path, files[name]); err != nil {
			return err
		}
	}
	return nil
}
