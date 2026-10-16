// Command deadset-corpus runs, checks and compares the conformance corpus this
// module embeds.
//
//	deadset-corpus run --language=LANG [--gaps=FILE] [--fixture=NAME ...] [--jobs=N] [--out=FILE] -- ANALYZER [ARG ...]
//	deadset-corpus check
//	deadset-corpus agree FIRST SECOND
//	deadset-corpus cases --product=NAME -- PRODUCT [ARG ...]
//
// run answers every fixture rendered in the language through the analyzer's
// command line and writes the results document; it exits 1 when the corpus
// result is fail. check names every defect of the corpus and exits 1 on one.
// agree compares two results documents and exits 1 on a divergence. cases runs
// every command-line vector naming the product and exits 1 on a failed one.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cplieger/atomicfile/v4"
	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
)

const usage = "usage: deadset-corpus run --language=LANG [--gaps=FILE] [--fixture=NAME ...] [--jobs=N] [--out=FILE] -- ANALYZER [ARG ...]\n" +
	"       deadset-corpus check\n" +
	"       deadset-corpus agree FIRST SECOND\n" +
	"       deadset-corpus cases --product=NAME -- PRODUCT [ARG ...]"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "run":
		return runCorpus(ctx, args[1:], stdout, stderr)
	case "check":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage)
			return 2
		}
		errs := corpus.Check(spec.Corpus)
		for _, err := range errs {
			fmt.Fprintln(stderr, err)
		}
		if len(errs) > 0 {
			return 1
		}
		return 0
	case "agree":
		return agree(args[1:], stdout, stderr)
	case "cases":
		return cases(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}

type names []string

func (n *names) String() string     { return strings.Join(*n, ",") }
func (n *names) Set(v string) error { *n = append(*n, v); return nil }

func runCorpus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("deadset-corpus run", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() { fmt.Fprintln(stderr, usage) }
	language := set.String("language", "", "the language whose renderings the analyzer answers")
	gapsFile := set.String("gaps", "", "the declared gaps the product commits, conformance.json")
	jobs := set.Int("jobs", runtime.NumCPU(), "how many fixtures run at once")
	out := set.String("out", "", "the file the results document is written to; standard output where absent")
	var fixtures names
	set.Var(&fixtures, "fixture", "one fixture to answer; every selected one where absent")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if *language == "" || set.NArg() == 0 {
		set.Usage()
		return 2
	}
	options := corpus.Options{Fixtures: fixtures, Jobs: *jobs}
	if *gapsFile != "" {
		gaps, err := readGaps(*gapsFile)
		if err != nil {
			fmt.Fprintln(stderr, "deadset-corpus:", err)
			return 2
		}
		options.Gaps = gaps
	}
	results, err := corpus.Run(ctx, spec.Corpus, corpus.Analyzer{Command: set.Args(), Language: *language}, options)
	if err != nil {
		fmt.Fprintln(stderr, "deadset-corpus:", err)
		return 3
	}
	data, err := corpus.Encode(results)
	if err != nil {
		fmt.Fprintln(stderr, "deadset-corpus:", err)
		return 3
	}
	if *out == "" {
		_, err = stdout.Write(data)
	} else {
		var absolute string
		if absolute, err = filepath.Abs(*out); err == nil {
			_, err = atomicfile.WriteFile(ctx, absolute, data)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "deadset-corpus:", err)
		return 3
	}
	fmt.Fprintf(stderr, "%s %s: %d fixtures, %d pass, %d gap, %d fail, %d pending\n", results.Product.Name, results.Result,
		results.Totals.Fixtures, results.Totals.Pass, results.Totals.Gap, results.Totals.Fail, results.Totals.Pending)
	if results.Result != corpus.Pass {
		return 1
	}
	return 0
}

func readGaps(path string) ([]corpus.GapRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var gaps corpus.Gaps
	if err := corpus.DecodeStrict(data, &gaps); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return gaps.Gaps, nil
}

func agree(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	var documents [2]corpus.Results
	for i, name := range args {
		data, err := os.ReadFile(name) //nolint:gosec // a results document its invoker names
		if err == nil {
			err = corpus.DecodeStrict(data, &documents[i])
		}
		if err != nil {
			fmt.Fprintf(stderr, "deadset-corpus: %s: %v\n", name, err)
			return 2
		}
	}
	agreement := corpus.Agree(&documents[0], &documents[1])
	for _, excluded := range agreement.Excluded {
		fmt.Fprintln(stdout, excluded)
	}
	for _, divergence := range agreement.Divergences {
		fmt.Fprintln(stdout, divergence)
	}
	fmt.Fprintf(stderr, "%d compared, %d excluded, %d divergent\n", agreement.Compared, len(agreement.Excluded), len(agreement.Divergences))
	if agreement.Compared == 0 {
		fmt.Fprintln(stderr, "deadset-corpus:", errors.New("the two documents share no expectation"))
		return 1
	}
	if len(agreement.Divergences) > 0 {
		return 1
	}
	return 0
}

func cases(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("deadset-corpus cases", flag.ContinueOnError)
	set.SetOutput(stderr)
	set.Usage = func() { fmt.Fprintln(stderr, usage) }
	product := set.String("product", "", "the product the cases name")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if *product == "" || set.NArg() == 0 {
		set.Usage()
		return 2
	}
	results, err := corpus.RunCases(ctx, spec.Vectors, *product, corpus.Analyzer{Command: set.Args()})
	if err != nil {
		fmt.Fprintln(stderr, "deadset-corpus:", err)
		return 3
	}
	failed := false
	for _, result := range results {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", result.Result, result.Case, result.Message)
		failed = failed || result.Result == corpus.Fail
	}
	if failed {
		return 1
	}
	return 0
}
