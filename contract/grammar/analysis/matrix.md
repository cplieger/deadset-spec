# The build matrix

Rules `matrix.1` to `matrix.6` of [the analysis](../analysis.md). A rule is cited by its id, which is never reused for another rule.

## The derived Go matrix

**matrix.1** A Go matrix that the configuration document does not declare is derived from the build constraints of every Go file under the target root, a test file's as much as any other file's, except a file whose constraint holds only when `ignore` is set, plus the host platform. So a tag only test files name, `race` among them, derives a configuration exactly as a tag any other file names does, and a derived configuration that does not build is dropped, as the setup failures below state. A test file that no configuration of the matrix builds is outside the run, as any other file no configuration builds is, and under a matrix the configuration declares complete it is reported under `DS1601`.

**matrix.2** The `ignore` tag is the toolchain's convention for a file kept out of ordinary package builds unless a configuration explicitly satisfies the tag, and no derived configuration sets it. A file whose build constraint holds only when `ignore` is set, whatever else the constraint names, and that no configuration of the run builds, is outside the analysis: no finding, `DS1601` included under a complete matrix, no warning and no listed type error is about it or about a declaration it holds, and no reference it writes counts. Where its package clause is `main`, such a file is a program run by naming the file, so each declaration of the target that it names is a root. Its names resolve as they do when the toolchain builds that file alone under the host platform, and a name that does not resolve roots nothing. A configuration the configuration document declares may set `ignore`, and a file such a configuration builds is analyzed in it as any file that configuration builds is.

**matrix.3** A derived configuration in which a package of the target has a type error that another configuration of the matrix type-checks without one holds no root for that package's type errors and lists none of them in `type_error_skips`: the configurations that build the package judge it. A package no configuration type-checks keeps the type-error rule in every configuration.

## How configurations combine

**matrix.4** A symbol is reported only when it is dead in every configuration of the run, and each finding names the configurations in which it holds.

**matrix.5** Nor is a symbol reported that a configuration compiles a reference to from a declaration that is not reported, because deleting it would break that configuration's build, and neither is any declaration a reference from one so kept names, until no declaration joins. In the run that counts no test reference, a reference a test file makes does not count for this.

**matrix.6** A finding about a part of a declaration, a parameter, a receiver, a result, a statement, a case or a store, is reported only where every configuration that compiles the part's file reports it, under the same code and at the same position: a configuration that compiles the file and reports nothing there keeps the part.
